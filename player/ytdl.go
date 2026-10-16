package player

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopxl/beep/v2"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/internal/extbin"
	"github.com/xaviiromf/cliamp-x/internal/ytdlcookies"
	"github.com/xaviiromf/cliamp-x/internal/ytdlp"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// pipeBufSize is the buffer size for audio pipe readers (yt-dlp, ffmpeg).
const pipeBufSize = 64 * 1024

// ytdlPipeTimeout limits how long we wait for yt-dlp to produce initial audio.
const ytdlPipeTimeout = 30 * time.Second

// ytdlCauseGrace bounds how long buildYTDLPipeline waits, after the audio pipe
// closes with no data, for yt-dlp or ffmpeg to report why. yt-dlp typically
// exits quickly with a stderr message (bot wall, 404, DRM); this only matters
// when the process is slow to flush and exit.
const ytdlCauseGrace = 3 * time.Second

const ytdlPipelineMaxAttempts = 3

// YTDLPAvailable reports whether yt-dlp is installed and on PATH, or set
// by CLIAMP_YTDLP.
func YTDLPAvailable() bool {
	return extbin.Have("yt-dlp")
}

func appendYTDLCookieArgs(args []string, pageURL string) []string {
	return append(args, ytdlcookies.Args(ytdlcookies.ForURL(pageURL))...)
}

// On Android, yt-dlp with the signed-in cookies uses YouTube web clients
// whose JavaScript challenge costs the bundled QuickJS about 11 s of CPU on
// every run, while without cookies it uses clients that need no challenge.
// There the runs that resolve a signed-in YouTube page for playback go first
// without the cookies. Variables so tests can set them.
var (
	ytdlCookielessFirst = runtime.GOOS == "android"
	// ytdlCookielessTimeout bounds that first run, so a hung one plus the
	// cookie run on a phone (about 16 s) stays within ytdlPipeTimeout.
	ytdlCookielessTimeout = 12 * time.Second
)

// ytdlAudioCap is the highest audio bitrate, in kbps, of the format that a
// yt-dlp start picks, or 0 for the best one. ytdlPremium turns off the
// cookieless first runs, so a Premium account gets its higher formats.
var (
	ytdlAudioCap atomic.Int32
	ytdlPremium  atomic.Bool
)

// SetYTDLQuality caps the audio bitrate of the yt-dlp starts that follow,
// in kbps. 0 picks the best format.
func SetYTDLQuality(kbps int) { ytdlAudioCap.Store(int32(max(0, kbps))) }

// SetYTDLPremium turns the Premium quality on or off. On Android, on makes
// the yt-dlp runs use the signed-in cookies from the first run, at the cost
// of the slower start that the cookieless runs avoid.
func SetYTDLPremium(on bool) { ytdlPremium.Store(on) }

// ytdlCookieless reports whether the yt-dlp runs that resolve pageURL go
// first without its cookies, and with them only when that run fails (sign-in,
// age, members or Premium only): a signed-in YouTube page on Android.
func ytdlCookieless(pageURL string) bool {
	return ytdlCookielessFirst && !ytdlPremium.Load() && ytdlcookies.ForURL(pageURL) != "" &&
		(playlist.IsYouTubeURL(pageURL) || playlist.IsYouTubeMusicURL(pageURL))
}

// ytdlOutput runs yt-dlp with args on pageURL and returns what it prints.
// When ytdlCookieless picks pageURL, a first run without the cookies gets at
// most ytdlCookielessTimeout, and a run with them follows only when it fails.
func ytdlOutput(ctx context.Context, pageURL string, args ...string) ([]byte, error) {
	run := func(ctx context.Context, cookies bool) ([]byte, error) {
		a := args
		if cookies {
			a = appendYTDLCookieArgs(a, pageURL)
		}
		// "--" stops yt-dlp parsing pageURL as a flag. Callers gate on
		// playlist.IsURL, but keep the terminator so a future caller cannot
		// turn a crafted URL into --exec and reach arbitrary command execution.
		cmd := extbin.Command(ctx, "yt-dlp", append(a, "--", pageURL)...)
		// WaitDelay ensures cmd.Output() doesn't hang indefinitely if the
		// process is killed but I/O pipe goroutines haven't drained. Without
		// this, a zombie yt-dlp child keeping stdout open can block Output()
		// forever, which in turn blocks the start and leaves the UI stuck at
		// "Buffering...".
		cmd.WaitDelay = 3 * time.Second
		return cmd.Output()
	}
	if ytdlCookieless(pageURL) {
		first, cancel := context.WithTimeout(ctx, ytdlCookielessTimeout)
		out, err := run(first, false)
		cancel()
		if err == nil || ctx.Err() != nil {
			return out, err
		}
	}
	return run(ctx, true)
}

// ffmpegInstallHint returns a platform-specific install command suggestion.
func ffmpegInstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install ffmpeg"
	case "linux":
		if _, err := exec.LookPath("apt-get"); err == nil {
			return "sudo apt install ffmpeg"
		}
		if _, err := exec.LookPath("pacman"); err == nil {
			return "sudo pacman -S ffmpeg"
		}
		return "see https://ffmpeg.org/download.html"
	case "windows":
		return "winget install ffmpeg"
	default:
		return "see https://ffmpeg.org/download.html"
	}
}

// ytdlPipeStreamer streams PCM audio from a yt-dlp | ffmpeg pipe chain.
// yt-dlp downloads the best audio and writes raw data to stdout; ffmpeg reads
// that via a pipe and converts it to PCM on its stdout, which we consume.
type ytdlPipeStreamer struct {
	pipeReport // total stays 0: the length of a piped page is unknown
	ytdlCmd    *exec.Cmd
	ffmpegCmd  *exec.Cmd
	relay      *memRelay     // the download between yt-dlp and ffmpeg
	pipe       io.ReadCloser // ffmpeg stdout (PCM output)
	reader     *bufio.Reader // buffered reader over pipe
	ytdlErr    <-chan error  // yt-dlp exit error from monitoring goroutine
	ffmpegErr  <-chan error  // ffmpeg exit error from monitoring goroutine
	ytdlDone   <-chan struct{}
	ffmpegDone <-chan struct{}
	formatPath string // where yt-dlp prints the format it picked, or ""
	pcmBuf     []byte
	f32        bool // true = f32le, false = s16le
	closeOnce  sync.Once
}

func (y *ytdlPipeStreamer) Stream(samples [][2]float64) (int, bool) {
	n, ok := streamFromReader(y.reader, samples, &y.pcmBuf, y.f32, y.state)
	y.state.pos.Add(int64(n))
	// On EOF with no frames read, surface why the pipe closed (yt-dlp bot
	// wall, 404, DRM, or undecodable ffmpeg input) instead of a bare EOF.
	if !ok && n == 0 && y.state.err.load() == nil {
		if cause := y.waitCause(0); cause != nil {
			y.state.err.publish(cause)
		}
	}
	return n, ok
}

// waitCause reports why the audio pipe closed without producing audio,
// preferring yt-dlp's reason (bot wall, 404, DRM, region block) over ffmpeg's
// (undecodable input). With d <= 0 it polls without blocking; otherwise it
// waits up to d for a process to report. Returns nil if neither reported an
// error, leaving the caller to surface the bare EOF.
func (y *ytdlPipeStreamer) waitCause(d time.Duration) error {
	if d <= 0 {
		select {
		case e := <-y.ytdlErr:
			if e != nil {
				return e
			}
		default:
		}
		select {
		case e := <-y.ffmpegErr:
			return e
		default:
			return nil
		}
	}
	deadline := time.After(d)
	ytdlDone, ffmpegDone := false, false
	var ffErr error
	for !ytdlDone || !ffmpegDone {
		select {
		case e := <-y.ytdlErr:
			ytdlDone = true
			if e != nil {
				return e
			}
		case e := <-y.ffmpegErr:
			ffmpegDone = true
			ffErr = e
		case <-deadline:
			return ffErr
		}
	}
	return ffErr
}

func (y *ytdlPipeStreamer) Seek(int) error { return nil }

// interrupt releases a blocked PCM read without waiting for yt-dlp or ffmpeg.
// A stalled download otherwise holds Stream, and the speaker lock with it,
// until yt-dlp gets data or gives up. Close reaps both processes later.
func (y *ytdlPipeStreamer) interrupt() {
	_ = y.pipe.Close()
	_ = y.relay.Close()
	if y.ytdlCmd.Process != nil {
		_ = y.ytdlCmd.Process.Kill()
	}
	if y.ffmpegCmd.Process != nil {
		_ = y.ffmpegCmd.Process.Kill()
	}
}

func (y *ytdlPipeStreamer) Close() error {
	y.closeOnce.Do(func() {
		// Kill both processes to stop downloading/decoding.
		if y.ytdlCmd.Process != nil {
			y.ytdlCmd.Process.Kill()
		}
		if y.ffmpegCmd.Process != nil {
			y.ffmpegCmd.Process.Kill()
		}
		y.pipe.Close()
		y.relay.Close()
		if y.formatPath != "" {
			_ = os.Remove(y.formatPath)
		}
		// The monitor goroutines own Wait. Killing both children and waiting
		// for their done signals guarantees Close does not leave zombies.
		if y.ytdlDone != nil {
			<-y.ytdlDone
		}
		if y.ffmpegDone != nil {
			<-y.ffmpegDone
		}
	})
	return nil
}

// pickedFormat reads the format and duration that yt-dlp printed before its
// download, which the first audio byte follows. They are zero when yt-dlp
// printed none.
func (y *ytdlPipeStreamer) pickedFormat() (AudioFormat, time.Duration) {
	if y.formatPath == "" {
		return AudioFormat{}, 0
	}
	data, err := os.ReadFile(y.formatPath)
	if err != nil {
		return AudioFormat{}, 0
	}
	return parseYTDLFormat(string(data))
}

// monitorExit waits for cmd to exit and reports the result on a buffered
// channel: a wrapped error preferring captured stderr over the bare exit code
// on failure, or nil on clean exit. The channel is buffered so the goroutine
// always completes even with no receiver (e.g. after Close kills the process).
func monitorExit(cmd *exec.Cmd, stderr *limitedBuffer, name string) (<-chan error, <-chan struct{}) {
	ch := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := cmd.Wait()
		switch trimmed := strings.TrimSpace(stderr.String()); {
		case err == nil:
			ch <- nil
		case trimmed != "":
			ch <- fmt.Errorf("%s: %w: %s", name, err, trimmed)
		default:
			ch <- fmt.Errorf("%s: %w", name, err)
		}
	}()
	return ch, done
}

// decodeYTDLPipe starts a yt-dlp | ffmpeg pipe chain for the given page URL
// and returns a streaming PCM decoder. If startSec > 0, ffmpeg -ss is used
// to skip to the desired position in the input stream.
func decodeYTDLPipe(pageURL string, sr beep.SampleRate, bitDepth, startSec int) (*ytdlPipeStreamer, beep.Format, error) {
	return decodeYTDLPipeInfo(pageURL, "", "", true, sr, bitDepth, startSec)
}

// ytdlFormat picks the audio format yt-dlp downloads. Prefer direct
// HTTPS/HTTP streams over HLS (m3u8): HLS requires segment downloading and
// muxing which doesn't pipe cleanly to stdout. Live streams (e.g. YouTube
// live) expose no audio-only formats at all, only muxed video+audio over HLS,
// so fall back to "best" as a last resort; the ffmpeg stage outputs PCM audio
// and drops the video.
const ytdlFormat = "bestaudio[protocol=https]/bestaudio[protocol=http]/bestaudio[protocol!=m3u8_native][protocol!=m3u8]/bestaudio/best"

// ytdlFormatSpec is ytdlFormat under the bitrate cap of SetYTDLQuality. A
// page with no direct audio format under the cap falls back to ytdlFormat,
// so a cap never stops a song from playing. "?" keeps the formats whose
// bitrate yt-dlp does not know.
func ytdlFormatSpec() string {
	limit := ytdlAudioCap.Load()
	if limit <= 0 {
		return ytdlFormat
	}
	capped := fmt.Sprintf("bestaudio[abr<=?%d]", limit)
	return capped + "[protocol=https]/" + capped + "[protocol=http]/" + ytdlFormat
}

// decodeYTDLPipeInfo is decodeYTDLPipe that, with infoPath, has yt-dlp load
// the page info prefetched there instead of extracting it again, which
// starts the audio seconds sooner. Without infoPath and with saveInfo, yt-dlp
// writes the page info it extracts to saveInfo + ".info.json". Without
// cookies, yt-dlp runs without the cookie arguments of pageURL.
func decodeYTDLPipeInfo(pageURL, infoPath, saveInfo string, cookies bool, sr beep.SampleRate, bitDepth, startSec int) (*ytdlPipeStreamer, beep.Format, error) {
	if !YTDLPAvailable() {
		return nil, beep.Format{}, fmt.Errorf("yt-dlp is required — install: %s", ytdlp.InstallHint())
	}
	if err := requireFFmpeg(); err != nil {
		return nil, beep.Format{}, err
	}

	// yt-dlp stdout → relay (memory) → ffmpeg stdin.
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, beep.Format{}, fmt.Errorf("os.Pipe: %w", err)
	}
	ytOut, ytIn, err := os.Pipe()
	if err != nil {
		pr.Close()
		pw.Close()
		return nil, beep.Format{}, fmt.Errorf("os.Pipe: %w", err)
	}
	closeRelayPipes := func() {
		ytOut.Close()
		ytIn.Close()
	}

	// Start yt-dlp: download best audio to stdout.
	ytdlArgs := []string{
		"-f", ytdlFormatSpec(),
		"--no-playlist",
		"--quiet",
		"--no-warnings",
		"--socket-timeout", "15",
		"-o", "-",
	}
	// yt-dlp names the format it picked in a file, for the now-playing
	// format line; stdout carries the audio.
	formatPath := ""
	removeFormat := func() {
		if formatPath != "" {
			_ = os.Remove(formatPath)
		}
	}
	if f, err := os.CreateTemp("", "cliamp-x-format-*"); err == nil {
		formatPath = f.Name()
		f.Close()
		ytdlArgs = append(ytdlArgs, "--print-to-file", ytdlFormatTemplate, formatPath)
	}
	if cookies {
		ytdlArgs = appendYTDLCookieArgs(ytdlArgs, pageURL)
	}
	switch {
	case infoPath != "":
		ytdlArgs = append(ytdlArgs, "--load-info-json", infoPath)
	case saveInfo != "":
		ytdlArgs = append(ytdlArgs, "--write-info-json", "-o", "infojson:"+saveInfo, "--", pageURL)
	default:
		ytdlArgs = append(ytdlArgs, "--", pageURL)
	}
	ytdlCmd := extbin.Command(context.Background(), "yt-dlp", ytdlArgs...)
	ytdlCmd.Stdout = ytIn
	var ytdlStderr limitedBuffer
	ytdlCmd.Stderr = &ytdlStderr
	if err := ytdlCmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		closeRelayPipes()
		removeFormat()
		return nil, beep.Format{}, fmt.Errorf("yt-dlp start: %w", err)
	}

	// Start ffmpeg: read from pipe, output PCM to stdout.
	// If startSec > 0, use -ss to seek into the input stream.
	_, _, precision := ffmpegPCMArgs(bitDepth)
	var ffmpegArgs []string
	if startSec > 0 {
		ffmpegArgs = append(ffmpegArgs, "-ss", strconv.Itoa(startSec))
	}
	ffmpegArgs = append(ffmpegArgs, "-i", "pipe:0")
	ffmpegArgs = append(ffmpegArgs, pcmOutputArgs(sr, bitDepth)...)
	ffmpegCmd := extbin.Command(context.Background(), "ffmpeg", ffmpegArgs...)
	ffmpegCmd.Stdin = pr
	var ffmpegStderr limitedBuffer
	ffmpegCmd.Stderr = &ffmpegStderr
	// An owned pipe carries ffmpeg's PCM, like the yt-dlp → ffmpeg hop above.
	// StdoutPipe would tie the read end to cmd.Wait, which monitorExit runs
	// during playback: Wait closes it as soon as ffmpeg exits, dropping PCM
	// still in the pipe and failing the next read instead of reaching EOF.
	pcmR, pcmW, err := os.Pipe()
	if err != nil {
		pw.Close()
		pr.Close()
		closeRelayPipes()
		ytdlCmd.Process.Kill()
		ytdlCmd.Wait()
		removeFormat()
		return nil, beep.Format{}, fmt.Errorf("os.Pipe: %w", err)
	}
	ffmpegCmd.Stdout = pcmW
	if err := ffmpegCmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		pcmR.Close()
		pcmW.Close()
		closeRelayPipes()
		ytdlCmd.Process.Kill()
		ytdlCmd.Wait()
		removeFormat()
		return nil, beep.Format{}, fmt.Errorf("ffmpeg start: %w", err)
	}

	// Close parent's copies of pipe ends. yt-dlp owns ytIn, ffmpeg owns pr
	// (read end) and pcmW (PCM write end), and the relay owns ytOut and pw.
	// If the parent keeps the others open, EOF won't propagate when the
	// owning process exits.
	ytIn.Close()
	pr.Close()
	pcmW.Close()
	relay := newMemRelay()
	relay.run(ytOut, pw)

	// Monitor each process's exit so we can surface why the pipe closed. A
	// process's stderr is only safe to read after Wait() returns, so the
	// capture happens inside monitorExit.
	ytdlErrCh, ytdlDone := monitorExit(ytdlCmd, &ytdlStderr, "yt-dlp")
	ffmpegErrCh, ffmpegDone := monitorExit(ffmpegCmd, &ffmpegStderr, "ffmpeg")

	format := beep.Format{
		SampleRate:  sr,
		NumChannels: 2,
		Precision:   precision,
	}

	return &ytdlPipeStreamer{
		ytdlCmd:    ytdlCmd,
		ffmpegCmd:  ffmpegCmd,
		relay:      relay,
		pipe:       pcmR,
		reader:     bufio.NewReaderSize(pcmR, pipeBufSize),
		ytdlErr:    ytdlErrCh,
		ffmpegErr:  ffmpegErrCh,
		ytdlDone:   ytdlDone,
		ffmpegDone: ffmpegDone,
		formatPath: formatPath,
		pipeReport: pipeReport{state: newPipeStreamState(0)},
		f32:        bitDepth == 32,
	}, format, nil
}

// buildYTDLPipeline creates a trackPipeline for a yt-dlp URL.
// If startSec > 0, playback begins at that offset (seek-by-restart).
func (p *Player) buildYTDLPipeline(pageURL string, startSec int) (*trackPipeline, error) {
	p.streamTitle.Store("")
	begin := time.Now()

	// A start, or a seek, uses the page info prefetched for it, if any.
	info := p.ytdlInfo.get(pageURL)
	if startSec > 0 {
		if info == "" {
			// A resume: extract the page first, so ffmpeg can seek in
			// the audio URL itself.
			p.PrefetchYTDL(pageURL)
			info = p.ytdlInfo.get(pageURL)
		}
		if tp, err := p.ytdlURLPipeline(pageURL, info, startSec, begin); err == nil {
			return tp, nil
		} else if !errors.Is(err, errNoYTDLURL) {
			// The audio URL went stale: extract the page again.
			applog.Info("playback: seek in the audio URL of %q failed, downloading from the start: %v", pageURL, err)
			p.ytdlInfo.drop(pageURL, info)
			info = ""
		}
	}
	cookieless := ytdlCookieless(pageURL)
	for attempt := 1; ; attempt++ {
		saveInfo := ""
		if info == "" {
			saveInfo = p.ytdlInfo.startPath(pageURL)
		}
		decoder, format, err := decodeYTDLPipeInfo(pageURL, info, saveInfo, !cookieless, p.sr, p.bitDepth, startSec)
		if err != nil {
			return nil, err
		}

		wait := ytdlPipeTimeout
		if cookieless {
			wait = ytdlCookielessTimeout
		}
		if err := prefillYTDLPipe(decoder, wait); err != nil {
			if info != "" {
				// The prefetched info went stale: extract the page again,
				// which does not count as a retry.
				p.ytdlInfo.drop(pageURL, info)
				info = ""
				attempt--
				continue
			}
			if cookieless {
				// The page needs the session (sign-in, age, members or
				// Premium only): play it with the cookies, which does not
				// count as a retry either.
				cookieless = false
				attempt--
				continue
			}
			if attempt == ytdlPipelineMaxAttempts || !isTransientYTDL403(err) {
				return nil, err
			}
			continue
		}

		if saveInfo != "" {
			// yt-dlp wrote the page info before the first audio byte; the
			// seeks and restarts of this track load it.
			p.ytdlInfo.add(pageURL, saveInfo+".info.json")
		}
		st := &ytdlStart{begin: begin, prefetched: info != "", offset: startSec, firstPCM: time.Since(begin)}
		if fi, err := os.Stat(decoder.formatPath); err == nil && decoder.formatPath != "" {
			st.extract = fi.ModTime().Sub(begin)
		}
		if fb := decoder.relay.firstByte(); !fb.IsZero() {
			st.firstByte = fb.Sub(begin)
		}

		audio, dur := decoder.pickedFormat()
		// The prefetch reads the pipe away from the speaker callback. A
		// stalled download then plays silence and does not hold the speaker
		// lock, which would freeze every control.
		return p.prefetchNetworkPipeline(&trackPipeline{
			decoder:       decoder,
			stream:        decoder,
			format:        format,
			seekable:      false,
			path:          pageURL,
			ytdlSeek:      true,
			streamOffset:  time.Duration(startSec) * time.Second,
			knownDuration: dur,
			audio:         audio,
			ytdlStart:     st,
		}, true), nil
	}
}

// buildYTDLSource starts the yt-dlp | ffmpeg chain for pageURL at offset
// (whole seconds). The duration yt-dlp printed fills in an unknown
// knownDuration. A play, unlike a preload, logs its start timing.
func (p *Player) buildYTDLSource(pageURL string, knownDuration, offset time.Duration, play bool) (*trackPipeline, error) {
	tp, err := p.buildYTDLPipeline(pageURL, max(0, int(offset.Seconds())))
	if err != nil {
		return nil, err
	}
	if knownDuration > 0 {
		tp.knownDuration = knownDuration
	}
	if play {
		tp.logYTDLStart("start")
	}
	return tp, nil
}

// ytdlStart is where the start of a yt-dlp chain spent its time, each stage
// counted from begin, for the "playback: start" line in cliamp.log.
type ytdlStart struct {
	begin      time.Time // the start began, before any wait for prefetched info
	prefetched bool      // yt-dlp loaded prefetched page info
	direct     bool      // ffmpeg opened the audio URL, see ytdlURLPipeline
	offset     int       // seconds into the track
	extract    time.Duration
	firstByte  time.Duration
	firstPCM   time.Duration
}

func (s *ytdlStart) line(event, track string, audio time.Duration) string {
	info, via := "extracted", "yt-dlp"
	if s.prefetched {
		info = "prefetched"
	}
	if s.direct {
		via = "url"
	}
	return fmt.Sprintf("playback: %s track=%q offset=%ds via=%s info=%s extract=%.2fs firstbyte=%.2fs firstpcm=%.2fs audio=%.2fs",
		event, track, s.offset, via, info, s.extract.Seconds(), s.firstByte.Seconds(), s.firstPCM.Seconds(), audio.Seconds())
}

// logYTDLStart logs the start timing of a yt-dlp pipeline when the speaker
// first takes its audio. event is start or seek.
func (tp *trackPipeline) logYTDLStart(event string) {
	st, path := tp.ytdlStart, tp.path
	if st == nil || tp.livePrefetch == nil {
		return
	}
	tp.livePrefetch.onFirstAudio(func() {
		applog.Info("%s", st.line(event, path, time.Since(st.begin)))
	})
}

// errNoYTDLURL is the error of ytdlURLPipeline for page info that holds no
// direct audio URL, such as an HLS live stream.
var errNoYTDLURL = errors.New("no direct audio URL in the page info")

// ytdlURLPipeline starts a yt-dlp track at startSec by having ffmpeg open the
// audio URL in the page info at infoPath, with the HTTP headers yt-dlp would
// send. ffmpeg seeks there with range requests, so the start waits only for
// the audio at startSec. Through yt-dlp, ffmpeg would read and drop all the
// audio before it: about 1 MB per minute, which on an hour-long mix took
// longer than ytdlPipeTimeout.
func (p *Player) ytdlURLPipeline(pageURL, infoPath string, startSec int, begin time.Time) (*trackPipeline, error) {
	if infoPath == "" {
		return nil, errNoYTDLURL
	}
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, errNoYTDLURL
	}
	var info struct {
		URL      string            `json:"url"`
		Protocol string            `json:"protocol"`
		Headers  map[string]string `json:"http_headers"`
		ACodec   string            `json:"acodec"`
		ABR      float64           `json:"abr"`
		ASR      int               `json:"asr"`
		Duration float64           `json:"duration"`
	}
	if json.Unmarshal(data, &info) != nil || info.URL == "" || (info.Protocol != "https" && info.Protocol != "http") {
		return nil, errNoYTDLURL
	}
	if err := requireFFmpeg(); err != nil {
		return nil, err
	}
	args := []string{"-reconnect", "1", "-reconnect_streamed", "1", "-reconnect_delay_max", "5"}
	if len(info.Headers) > 0 {
		var h strings.Builder
		for k, v := range info.Headers {
			h.WriteString(k + ": " + v + "\r\n")
		}
		args = append(args, "-headers", h.String())
	}
	args = append(args, "-ss", strconv.Itoa(startSec), "-i", info.URL)
	fp, format, err := startFFmpegPipe(args, nil, p.sr, p.bitDepth)
	if err != nil {
		return nil, err
	}
	decoder := &ffmpegPipeStreamer{ffmpegPipe: fp}
	audio, dur := parseYTDLFormat(fmt.Sprintf("%s %g %d %g", cmp.Or(info.ACodec, "NA"), info.ABR, info.ASR, info.Duration))
	if err := decoder.waitForInitialAudio(ytdlPipeTimeout); err != nil {
		decoder.Close()
		return nil, err
	}
	return p.prefetchNetworkPipeline(&trackPipeline{
		decoder:       decoder,
		stream:        decoder,
		format:        format,
		path:          pageURL,
		ytdlSeek:      true,
		streamOffset:  time.Duration(startSec) * time.Second,
		knownDuration: dur,
		audio:         audio,
		ytdlStart:     &ytdlStart{begin: begin, prefetched: true, direct: true, offset: startSec, firstPCM: time.Since(begin)},
	}, true), nil
}

func prefillYTDLPipe(decoder *ytdlPipeStreamer, timeout time.Duration) error {
	// Pre-fill: block until yt-dlp + ffmpeg produce initial audio data.
	// This runs in a tea.Cmd goroutine (not the UI thread), so a chain that
	// fails before its first byte reports why before the track starts. The
	// timeout prevents hanging when yt-dlp is slow to produce output.
	stop := func() { decoder.Close() }
	return peekWithTimeout(decoder.reader, 1, timeout, stop, func(err error) error {
		// The audio pipe closed before producing a byte. Prefer the real
		// cause from yt-dlp (e.g. "Sign in to confirm you're not a bot",
		// "HTTP Error 404", DRM, region block) or ffmpeg over the opaque
		// EOF — the pipe only tells us the upstream closed, not why.
		cause := decoder.waitCause(ytdlCauseGrace)
		decoder.Close()
		if cause != nil {
			return cause
		}
		return fmt.Errorf("waiting for audio data: %w", err)
	})
}

func isTransientYTDL403(err error) bool {
	message := err.Error()
	return strings.Contains(message, "yt-dlp:") && strings.Contains(message, "HTTP Error 403: Forbidden")
}

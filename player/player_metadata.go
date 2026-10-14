package player

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Now-playing titles of internet radio: ICY titles and the metadata
// resolvers that providers register for their streams.

// StreamMetadataResolver matches a stream URL to a now-playing fetcher for
// broadcasters that carry no inline ICY metadata (e.g. NTS, FIP) and instead
// publish the current track via a separate JSON API. It returns a fetch
// function, the poll interval, and ok=false when the URL is not recognized.
type StreamMetadataResolver func(streamURL string) (fetch func(ctx context.Context) (string, error), interval time.Duration, ok bool)

// StreamTitle returns the current ICY stream title (e.g., "Artist - Song").
// Returns "" when no ICY metadata has been received.
func (p *Player) StreamTitle() string {
	v, _ := p.streamTitle.Load().(string)
	return v
}

// setStreamTitle is the ICY onMeta callback, called from the reader goroutine.
func (p *Player) setStreamTitle(title string) {
	p.streamTitle.Store(title)
}

// RegisterStreamMetadataResolver installs a resolver used to pull now-playing
// metadata for streams that lack inline ICY metadata. Pass nil to disable.
func (p *Player) RegisterStreamMetadataResolver(r StreamMetadataResolver) {
	p.mu.Lock()
	p.streamMetaResolver = r
	p.mu.Unlock()
}

// startStreamMetadata (re)starts background now-playing polling for streamURL,
// cancelling any previous poller. It is a no-op unless a registered resolver
// recognizes the URL, so streams that carry ICY metadata (or local files) are
// unaffected. The poller feeds titles through the same path as ICY metadata.
func (p *Player) startStreamMetadata(streamURL string) {
	p.stopStreamMetadata()

	p.mu.Lock()
	resolver := p.streamMetaResolver
	p.mu.Unlock()
	if resolver == nil || streamURL == "" || !isURL(streamURL) {
		return
	}
	fetch, interval, ok := resolver(streamURL)
	if !ok || fetch == nil {
		return
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.metaCancel = cancel
	p.mu.Unlock()

	go p.pollStreamMetadata(ctx, fetch, interval)
}

// stopStreamMetadata cancels the active metadata poller, if any.
func (p *Player) stopStreamMetadata() {
	p.mu.Lock()
	cancel := p.metaCancel
	p.metaCancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// pollStreamMetadata fetches the current title immediately and then on each
// interval tick until ctx is cancelled, publishing non-empty titles via
// setStreamTitle. A title fetched after cancellation is discarded so a stale
// poller cannot clobber the next stream's metadata.
func (p *Player) pollStreamMetadata(ctx context.Context, fetch func(context.Context) (string, error), interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		title, err := fetch(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil && title != "" {
			p.setStreamTitle(title)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// AudioFormat describes the source of the playing track: its codec, its
// bitrate in kbps and its sample rate in Hz. A zero field is unknown.
type AudioFormat struct {
	Codec string
	Kbps  int
	Rate  int
}

// AudioFormat returns the format of the playing track's source.
func (p *Player) AudioFormat() AudioFormat {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current == nil {
		return AudioFormat{}
	}
	return p.current.audio
}

// ytdlFormatTemplate is the --print-to-file template that records the format
// yt-dlp picked. parseYTDLFormat reads it.
const ytdlFormatTemplate = "before_dl:%(acodec)s %(abr)s %(asr)s"

// parseYTDLFormat reads a line of ytdlFormatTemplate, such as
// "opus 129.5 48000". yt-dlp prints NA or none for a field it does not know.
func parseYTDLFormat(line string) AudioFormat {
	var f AudioFormat
	fields := strings.Fields(line)
	if len(fields) != 3 {
		return f
	}
	if c := strings.ToLower(fields[0]); c != "na" && c != "none" {
		// mp4a.40.2 is AAC-LC, mp4a.40.5 HE-AAC.
		if strings.HasPrefix(c, "mp4a") {
			c = "aac"
		}
		f.Codec = c
	}
	if v, err := strconv.ParseFloat(fields[1], 64); err == nil && v > 0 {
		f.Kbps = int(math.Round(v))
	}
	if v, err := strconv.Atoi(fields[2]); err == nil && v > 0 {
		f.Rate = v
	}
	return f
}

// localCodecs names the codec of each local file extension that is not
// itself the codec's name.
var localCodecs = map[string]string{
	".m4a": "aac", ".m4b": "aac", ".mp4": "aac", ".aacp": "aac",
	".ogg": "vorbis", ".oga": "vorbis", ".webm": "opus", ".wav": "pcm",
}

// localAudioFormat describes the local file at path that tp plays: the codec
// from its extension, its mean bitrate from its size, and the sample rate of
// a native decoder. ffmpeg decodes to the output rate, so its source rate
// stays unknown.
func localAudioFormat(path string, tp *trackPipeline) AudioFormat {
	ext := strings.ToLower(filepath.Ext(path))
	f := AudioFormat{Codec: strings.TrimPrefix(ext, ".")}
	if c, ok := localCodecs[ext]; ok {
		f.Codec = c
	}
	if _, ff := tp.decoder.(*localFFmpegStreamer); !ff {
		f.Rate = int(tp.format.SampleRate)
	}
	dur := tp.knownDuration
	if n := tp.decoder.Len(); dur <= 0 && n > 0 && tp.format.SampleRate > 0 {
		dur = tp.format.SampleRate.D(n)
	}
	if info, err := os.Stat(path); err == nil && dur >= time.Second {
		f.Kbps = int(math.Round(float64(info.Size()) * 8 / dur.Seconds() / 1000))
	}
	return f
}

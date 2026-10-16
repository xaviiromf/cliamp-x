package player

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// A prefetched page makes the next start, and the seeks of that track, load
// it instead of extracting the page again, and a start whose prefetched info
// fails extracts it as before.
func TestYTDLPrefetchedInfoServesStartAndSeeks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX process fixtures")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls")
	// The fake yt-dlp logs its arguments. -J prints info; a start with
	// --load-info-json of a file holding "stale" fails like an expired URL.
	writeExecutable(t, filepath.Join(dir, "yt-dlp"), `#!/bin/sh
echo "$@" >> `+log+`
case "$*" in
  *-J*) echo '{"id":"x"}'; exit 0 ;;
  *load-info-json*) f=$(echo "$@" | sed 's/.*--load-info-json \([^ ]*\).*/\1/'); grep -q stale "$f" && { echo "ERROR: HTTP Error 403: Forbidden" >&2; exit 1; } ;;
esac
head -c 4096 /dev/zero
`)
	writeExecutable(t, filepath.Join(dir, "ffmpeg"), "#!/bin/sh\ncat\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	p := &Player{sr: 44100, bitDepth: 16}
	p.RegisterYTDLMatcher(func(string) bool { return true })
	t.Cleanup(p.ytdlInfo.close)
	const url = "https://music.youtube.com/watch?v=abc"

	p.PrefetchYTDL(url)
	p.PrefetchYTDL(url) // a second call for the same page starts nothing
	tp, err := p.buildYTDLPipeline(url, 0)
	if err != nil {
		t.Fatalf("buildYTDLPipeline: %v", err)
	}
	tp.decoder.Close()
	// A seek, as a rewind to the start, restarts yt-dlp from the same info.
	tp, err = p.buildYTDLPipeline(url, 30)
	if err != nil {
		t.Fatalf("buildYTDLPipeline seek: %v", err)
	}
	tp.decoder.Close()
	calls := readLines(t, log)
	if len(calls) != 3 || !strings.Contains(calls[0], "-J") {
		t.Fatalf("yt-dlp calls = %q, want one -J then two starts from the info", calls)
	}
	for _, c := range calls[1:] {
		if !strings.Contains(c, "--load-info-json") || strings.Contains(c, "-- "+url) {
			t.Fatalf("yt-dlp call = %q, want a start from the info", c)
		}
	}

	// A stale prefetch falls back to a normal start.
	_ = os.Remove(log)
	p.PrefetchYTDL(url)
	p.ytdlInfo.mu.Lock()
	e := p.ytdlInfo.entries[url]
	p.ytdlInfo.mu.Unlock()
	select {
	case <-e.done:
	case <-time.After(5 * time.Second):
		t.Fatal("prefetch did not finish")
	}
	if err := os.WriteFile(e.path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	tp, err = p.buildYTDLPipeline(url, 0)
	if err != nil {
		t.Fatalf("buildYTDLPipeline after a stale prefetch: %v", err)
	}
	tp.decoder.Close()
	calls = readLines(t, log)
	if last := calls[len(calls)-1]; !strings.Contains(last, "-- "+url) {
		t.Fatalf("last yt-dlp call = %q, want a normal start from the page", last)
	}
	if _, err := os.Stat(e.path); !os.IsNotExist(err) {
		t.Fatal("the stale info file must be removed")
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// A start that extracts the page saves its info, and a seek or a resume then
// has ffmpeg seek in the audio URL of that info, so it does not wait for
// yt-dlp to download all the audio before the offset. Info with no direct
// URL seeks through yt-dlp as before, and a stale URL extracts the page again.
func TestYTDLSeekOpensTheAudioURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX process fixtures")
	}
	const (
		direct = `{"url":"https://media.example/a","protocol":"https","http_headers":{"User-Agent":"UA"},"acodec":"opus","abr":130,"asr":48000}`
		hls    = `{"url":"https://media.example/a.m3u8","protocol":"m3u8_native"}`
	)
	tests := []struct {
		name   string
		info   string
		ffFail bool // ffmpeg fails to open the audio URL
		start  bool // the track starts at 0 before the seek
		want   []string
	}{
		{name: "seek after a start", info: direct, start: true, want: []string{"start", "url"}},
		{name: "resume extracts the page first", info: direct, want: []string{"info", "url"}},
		{name: "no direct URL seeks through yt-dlp", info: hls, start: true, want: []string{"start", "load"}},
		{name: "stale URL extracts the page again", info: direct, ffFail: true, start: true, want: []string{"start", "url", "start"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			log := filepath.Join(dir, "calls")
			infoFile := filepath.Join(dir, "info")
			if err := os.WriteFile(infoFile, []byte(tt.info), 0o600); err != nil {
				t.Fatal(err)
			}
			writeExecutable(t, filepath.Join(dir, "yt-dlp"), `#!/bin/sh
case "$*" in
  *-J*) echo info >> `+log+`; cat `+infoFile+`; exit 0 ;;
  *load-info-json*) echo load >> `+log+` ;;
  *write-info-json*) echo start >> `+log+`; f=$(echo "$@" | sed 's/.*infojson:\([^ ]*\).*/\1/'); cp `+infoFile+` "$f.info.json" ;;
esac
head -c 4096 /dev/zero
`)
			fail := "head -c 4096 /dev/zero"
			if tt.ffFail {
				fail = "echo 'HTTP error 403 Forbidden' >&2; exit 1"
			}
			writeExecutable(t, filepath.Join(dir, "ffmpeg"), `#!/bin/sh
case "$*" in
  *"-headers User-Agent: UA"*"-ss 600 -i https://media.example/a "*) echo url >> `+log+`; `+fail+` ;;
  *) cat ;;
esac
`)
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

			p := &Player{sr: 44100, bitDepth: 16}
			p.RegisterYTDLMatcher(func(string) bool { return true })
			t.Cleanup(p.ytdlInfo.close)
			const url = "https://music.youtube.com/watch?v=abc"
			if tt.start {
				tp, err := p.buildYTDLPipeline(url, 0)
				if err != nil {
					t.Fatalf("start: %v", err)
				}
				tp.close()
			}
			tp, err := p.buildYTDLPipeline(url, 600)
			if err != nil {
				t.Fatalf("seek: %v", err)
			}
			tp.close()

			if got := readLines(t, log); !slices.Equal(got, tt.want) {
				t.Fatalf("calls = %q, want %q", got, tt.want)
			}
			if tp.streamOffset != 600*time.Second {
				t.Fatalf("stream offset = %v, want 10m0s", tp.streamOffset)
			}
			if want := (AudioFormat{Codec: "opus", Kbps: 130, Rate: 48000}); tt.info == direct && !tt.ffFail && tp.audio != want {
				t.Fatalf("audio = %+v, want %+v", tp.audio, want)
			}
		})
	}
}

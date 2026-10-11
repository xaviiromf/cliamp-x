package player

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// A prefetched page makes the next start load it instead of extracting the
// page again, and a start whose prefetched info fails extracts it as before.
func TestYTDLPrefetchedInfoIsUsedOnce(t *testing.T) {
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
	calls := readLines(t, log)
	if len(calls) != 2 || !strings.Contains(calls[0], "-J") || !strings.Contains(calls[1], "--load-info-json") || strings.Contains(calls[1], "-- "+url) {
		t.Fatalf("yt-dlp calls = %q, want one -J then one start from the info", calls)
	}
	p.ytdlInfo.mu.Lock()
	left := len(p.ytdlInfo.entries)
	p.ytdlInfo.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d prefetched pages left after the start, want 0", left)
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

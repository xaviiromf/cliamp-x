package lyrics

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/fileutil"
)

// The disk cache keeps the lyrics found for each artist and title, so a song
// played again shows its lyrics at once and without a network request. Only
// found lyrics are kept: a song with none is looked up again, so the retry
// key still works after a network failure.
//
// shortcut: entries are never evicted, a few KB per song; add an LRU sweep if
// the folder ever grows past a few tens of MB.
var cache struct {
	mu  sync.Mutex
	dir string
}

// SetCacheDir turns on the disk cache in dir. An empty dir turns it off.
func SetCacheDir(dir string) {
	cache.mu.Lock()
	cache.dir = dir
	cache.mu.Unlock()
}

// cachedLine is one line in a cache file.
type cachedLine struct {
	Ms   int64  `json:"ms,omitempty"`
	Text string `json:"t"`
}

// cachePath returns the cache file of a track, or "" when the cache is off
// or the track has no artist and title.
func cachePath(artist, title string) string {
	cache.mu.Lock()
	dir := cache.dir
	cache.mu.Unlock()
	key := strings.ToLower(strings.TrimSpace(artist)) + "\x00" + strings.ToLower(strings.TrimSpace(title))
	if dir == "" || key == "\x00" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(dir, hex.EncodeToString(sum[:16])+".json")
}

// readCache returns the cached lyrics of a track.
func readCache(artist, title string) ([]Line, bool) {
	path := cachePath(artist, title)
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var stored []cachedLine
	if json.Unmarshal(data, &stored) != nil || len(stored) == 0 {
		return nil, false
	}
	lines := make([]Line, len(stored))
	for i, l := range stored {
		lines[i] = Line{Start: time.Duration(l.Ms) * time.Millisecond, Text: l.Text}
	}
	return lines, true
}

// writeCache keeps found lyrics. A failed write only costs a lookup later.
func writeCache(artist, title string, lines []Line) {
	path := cachePath(artist, title)
	if path == "" || len(lines) == 0 {
		return
	}
	stored := make([]cachedLine, len(lines))
	for i, l := range lines {
		stored[i] = cachedLine{Ms: l.Start.Milliseconds(), Text: l.Text}
	}
	if data, err := json.Marshal(stored); err == nil {
		_ = fileutil.WriteFileAtomic(path, data, 0o600)
	}
}

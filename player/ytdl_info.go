package player

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Extracting a YouTube page takes yt-dlp most of the seconds before the first
// audio byte. PrefetchYTDL extracts the page of the track that plays next
// while the current one plays, so a skip to it starts at once: yt-dlp then
// loads the saved info and only downloads.
const (
	// ytdlInfoTTL is how long prefetched info is used. The stream URLs in
	// it last hours; a stale one only costs a normal start, see
	// buildYTDLPipeline.
	ytdlInfoTTL = 30 * time.Minute
	// ytdlInfoMax bounds the prefetched pages kept at once.
	ytdlInfoMax = 3
	// ytdlInfoTimeout bounds one extraction.
	ytdlInfoTimeout = 30 * time.Second
)

// ytdlInfoCache holds the prefetched page info files, by page URL.
type ytdlInfoCache struct {
	mu      sync.Mutex
	dir     string
	entries map[string]*ytdlInfo
}

// ytdlInfo is one prefetched page. done closes when path holds the info, or
// err says why it does not.
type ytdlInfo struct {
	path   string
	at     time.Time
	done   chan struct{}
	err    error
	cancel context.CancelFunc // stops an extraction that is no longer wanted
}

// PrefetchYTDL starts extracting the page of a yt-dlp track in the
// background, so that a later start of it skips the extraction. A track with
// a downloaded copy, one prefetched already, or not a yt-dlp URL is skipped.
func (p *Player) PrefetchYTDL(pageURL string) {
	if !p.isYTDLURL(pageURL) {
		return
	}
	c := &p.ytdlInfo
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[pageURL]; ok && time.Since(e.at) < ytdlInfoTTL {
		return
	}
	if _, ok := p.localCopy(pageURL); ok || !YTDLPAvailable() {
		return
	}
	if c.dir == "" {
		dir, err := os.MkdirTemp("", "cliamp-x-ytdl-")
		if err != nil {
			return
		}
		c.dir = dir
		c.entries = map[string]*ytdlInfo{}
	}
	for len(c.entries) >= ytdlInfoMax {
		c.evictOldestLocked()
	}
	sum := sha256.Sum256([]byte(pageURL))
	ctx, cancel := context.WithTimeout(context.Background(), ytdlInfoTimeout)
	e := &ytdlInfo{path: filepath.Join(c.dir, hex.EncodeToString(sum[:8])+".json"), at: time.Now(), done: make(chan struct{}), cancel: cancel}
	c.entries[pageURL] = e
	go func() {
		defer close(e.done)
		defer cancel()
		e.err = fetchYTDLInfo(ctx, pageURL, e.path)
	}()
}

// fetchYTDLInfo writes the page info of pageURL to path with yt-dlp -J.
func fetchYTDLInfo(ctx context.Context, pageURL, path string) error {
	args := []string{"-f", ytdlFormat, "--no-playlist", "--quiet", "--no-warnings", "--socket-timeout", "15", "-J"}
	args = appendYTDLCookieArgs(args, pageURL)
	args = append(args, "--", pageURL)
	out, err := exec.CommandContext(ctx, "yt-dlp", args...).Output()
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// take returns the prefetched info file of pageURL and forgets it; the
// caller removes the file. It waits for an extraction still running, which
// is never slower than starting a new one. It returns "" when there is none.
func (c *ytdlInfoCache) take(pageURL string) string {
	c.mu.Lock()
	e, ok := c.entries[pageURL]
	delete(c.entries, pageURL)
	c.mu.Unlock()
	if !ok {
		return ""
	}
	<-e.done
	if e.err != nil || time.Since(e.at) > ytdlInfoTTL {
		_ = os.Remove(e.path)
		return ""
	}
	return e.path
}

// evictOldestLocked drops the oldest entry. The caller holds c.mu.
func (c *ytdlInfoCache) evictOldestLocked() {
	var oldest string
	for url, e := range c.entries {
		if oldest == "" || e.at.Before(c.entries[oldest].at) {
			oldest = url
		}
	}
	e := c.entries[oldest]
	delete(c.entries, oldest)
	e.cancel()
	go func() {
		<-e.done
		_ = os.Remove(e.path)
	}()
}

// close removes the prefetched files.
func (c *ytdlInfoCache) close() {
	c.mu.Lock()
	dir := c.dir
	c.dir, c.entries = "", nil
	c.mu.Unlock()
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

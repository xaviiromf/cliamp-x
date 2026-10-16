package player

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
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
	path := c.pathLocked(pageURL, ".json")
	if path == "" {
		return
	}
	for len(c.entries) >= ytdlInfoMax {
		c.evictOldestLocked()
	}
	ctx, cancel := context.WithTimeout(context.Background(), ytdlInfoTimeout)
	e := &ytdlInfo{path: path, at: time.Now(), done: make(chan struct{}), cancel: cancel}
	c.entries[pageURL] = e
	go func() {
		defer close(e.done)
		defer cancel()
		e.err = fetchYTDLInfo(ctx, pageURL, e.path)
	}()
}

// pathLocked names a file for pageURL with suffix in the cache directory,
// which it creates on first use. It returns "" when it cannot. The caller
// holds c.mu.
func (c *ytdlInfoCache) pathLocked(pageURL, suffix string) string {
	if c.dir == "" {
		dir, err := os.MkdirTemp("", "cliamp-x-ytdl-")
		if err != nil {
			return ""
		}
		c.dir = dir
		c.entries = map[string]*ytdlInfo{}
	}
	sum := sha256.Sum256([]byte(pageURL))
	return filepath.Join(c.dir, hex.EncodeToString(sum[:8])+suffix)
}

// startPath is the -o infojson: template under which a start that extracts
// pageURL has yt-dlp write the page info; yt-dlp adds ".info.json". It
// differs from the prefetch file, which a prefetch may be writing.
func (c *ytdlInfoCache) startPath(pageURL string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.pathLocked(pageURL, "-start")
}

// add keeps the page info that a start wrote at path, so the seeks and
// restarts of pageURL load it instead of extracting the page again. Info
// that the cache already holds for pageURL wins.
func (c *ytdlInfoCache) add(pageURL, path string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[pageURL]; ok || c.dir == "" || filepath.Dir(path) != c.dir {
		_ = os.Remove(path)
		return
	}
	for len(c.entries) >= ytdlInfoMax {
		c.evictOldestLocked()
	}
	done := make(chan struct{})
	close(done)
	c.entries[pageURL] = &ytdlInfo{path: path, at: time.Now(), done: done, cancel: func() {}}
}

// fetchYTDLInfo writes the page info of pageURL to path with yt-dlp -J.
func fetchYTDLInfo(ctx context.Context, pageURL, path string) error {
	out, err := ytdlOutput(ctx, pageURL, "-f", ytdlFormatSpec(), "--no-playlist", "--quiet", "--no-warnings", "--socket-timeout", "15", "-J")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

// get returns the prefetched info file of pageURL, kept for the seeks of
// the track, which restart yt-dlp: without it a rewind waits seconds for a
// new extraction. It waits for an extraction still running, which is never
// slower than starting a new one. It returns "" when there is none.
func (c *ytdlInfoCache) get(pageURL string) string {
	c.mu.Lock()
	e, ok := c.entries[pageURL]
	c.mu.Unlock()
	if !ok {
		return ""
	}
	<-e.done
	if e.err != nil || time.Since(e.at) > ytdlInfoTTL {
		c.drop(pageURL, e.path)
		return ""
	}
	return e.path
}

// drop forgets the stale or failed info file path of pageURL and removes
// it, so the next start extracts the page again.
func (c *ytdlInfoCache) drop(pageURL, path string) {
	c.mu.Lock()
	if e, ok := c.entries[pageURL]; ok && e.path == path {
		delete(c.entries, pageURL)
	}
	c.mu.Unlock()
	_ = os.Remove(path)
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

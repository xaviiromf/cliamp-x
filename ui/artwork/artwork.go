// Package artwork loads cover art and video thumbnails and draws them in the
// terminal with half-block characters: each cell shows two pixels, one in
// the foreground and one in the background colour, so pixels stay about
// square and an image keeps its aspect ratio (square album covers, 16:9 or
// 4:3 video thumbnails).
//
// Loading is asynchronous and cached twice: the downloaded bytes on disk, so
// art survives restarts, and the decoded, downscaled image in memory, so
// drawing a cover costs only the cell loop.
package artwork

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"  // registers GIF decoding
	_ "image/jpeg" // registers JPEG decoding
	_ "image/png"  // registers PNG decoding
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"
)

const (
	// maxPixels is the longest side kept in memory. The terminal shows a
	// cover at most a few dozen cells wide, so more pixels are wasted.
	maxPixels = 160
	// memoryEntries caps the decoded images kept in memory.
	memoryEntries = 400
	// maxDownload caps one image download.
	maxDownload = 4 << 20
	// parallel caps the downloads that run at once.
	parallel = 6
)

// Cache loads images by URL (http, https or a local file path) and keeps them.
// It is safe for concurrent use.
type Cache struct {
	dir    string
	client *http.Client
	sem    chan struct{}

	mu       sync.Mutex
	images   map[string]*entry
	order    []string // least recently stored first
	inflight map[string]bool
	failed   map[string]time.Time

	// kitty graphics: see kitty.go.
	kitty     bool
	kittyBase uint32                 // high byte of this run's image IDs
	kittyNext int                    // last slot taken, 1..kittySlots
	kittySlot [kittySlots + 1]string // URL of the image in each slot
}

type entry struct {
	img image.Image
	// The last drawing of img, kept because the UI redraws many times a
	// second at the same size.
	lines      []string
	cols, rows int
	kitty      kittyState
}

// New returns a cache that keeps downloaded bytes in dir. An empty dir keeps
// images in memory only.
func New(dir string) *Cache {
	return &Cache{
		dir:      dir,
		client:   &http.Client{Timeout: 12 * time.Second},
		sem:      make(chan struct{}, parallel),
		images:   map[string]*entry{},
		inflight: map[string]bool{},
		failed:   map[string]time.Time{},
	}
}

// Get returns the decoded image of url when it is loaded.
func (c *Cache) Get(url string) (image.Image, bool) {
	if c == nil || url == "" {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.images[url]
	if !ok {
		return nil, false
	}
	return e.img, true
}

// Draw returns the lines of the image of url drawn into at most maxCols by
// maxRows cells with its aspect kept, and its size in cells. ok is false
// while the image is not loaded. The drawing is cached per size.
func (c *Cache) Draw(url string, maxCols, maxRows int) (lines []string, cols, rows int, ok bool) {
	if c == nil || url == "" {
		return nil, 0, 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, found := c.images[url]
	if !found {
		return nil, 0, 0, false
	}
	cols, rows = Size(e.img, maxCols, maxRows)
	if e.lines == nil || e.cols != cols || e.rows != rows {
		e.lines, e.cols, e.rows = Render(e.img, cols, rows), cols, rows
	}
	return e.lines, cols, rows, true
}

// Claim marks url as loading and reports whether the caller should load it:
// false when it is loaded, already loading, or failed less than a minute ago.
func (c *Cache) Claim(url string) bool {
	if c == nil || url == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.images[url]; ok || c.inflight[url] {
		return false
	}
	if at, ok := c.failed[url]; ok && time.Since(at) < time.Minute {
		return false
	}
	c.inflight[url] = true
	return true
}

// Load fetches, decodes and stores url. Call it after a successful Claim; it
// blocks, so run it off the UI goroutine.
func (c *Cache) Load(ctx context.Context, url string) error {
	img, err := c.fetch(ctx, url)
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.inflight, url)
	if err != nil {
		c.failed[url] = time.Now()
		return err
	}
	c.images[url] = &entry{img: img}
	c.order = append(c.order, url)
	for len(c.order) > memoryEntries {
		delete(c.images, c.order[0])
		c.order = c.order[1:]
	}
	return nil
}

func (c *Cache) fetch(ctx context.Context, url string) (image.Image, error) {
	return c.decode(ctx, url, maxPixels)
}

// decode reads url and returns its image shrunk to at most limit pixels a
// side.
func (c *Cache) decode(ctx context.Context, url string, limit int) (image.Image, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	data, err := c.readBytes(ctx, url)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("artwork: decode %s: %w", url, err)
	}
	return downscale(img, limit), nil
}

// readBytes returns the image bytes from the disk cache, a local audio
// file's embedded picture, a local image file, or the network.
func (c *Cache) readBytes(ctx context.Context, url string) ([]byte, error) {
	path := strings.TrimPrefix(url, "file://")
	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {
		return readLocal(path)
	}
	cachePath := ""
	if c.dir != "" {
		sum := sha1.Sum([]byte(url))
		cachePath = filepath.Join(c.dir, hex.EncodeToString(sum[:]))
		if data, err := os.ReadFile(cachePath); err == nil && len(data) > 0 {
			return data, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/jpeg,image/png,image/*;q=0.8")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("artwork: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("artwork: %s: HTTP %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownload))
	if err != nil {
		return nil, fmt.Errorf("artwork: %w", err)
	}
	if cachePath != "" {
		if err := os.MkdirAll(c.dir, 0o755); err == nil {
			tmp := cachePath + ".tmp"
			if os.WriteFile(tmp, data, 0o644) == nil {
				_ = os.Rename(tmp, cachePath)
			}
		}
	}
	return data, nil
}

// readLocal reads a local image file, or the picture embedded in an audio
// file's tags.
func readLocal(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if m, err := tag.ReadFrom(f); err == nil && m != nil {
		if pic := m.Picture(); pic != nil && len(pic.Data) > 0 {
			return pic.Data, nil
		}
		return nil, errors.New("artwork: the file has no embedded picture")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(f, maxDownload))
}

// downscale shrinks img so its longest side is at most limit, averaging the
// source pixels that fall in each target pixel.
func downscale(img image.Image, limit int) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || (w <= limit && h <= limit) {
		return img
	}
	scale := float64(limit) / float64(max(w, h))
	return resample(img, max(1, int(float64(w)*scale+0.5)), max(1, int(float64(h)*scale+0.5)))
}

// resample box-filters img to tw by th pixels.
func resample(img image.Image, tw, th int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, tw, th))
	for ty := range th {
		y0 := b.Min.Y + ty*h/th
		y1 := max(y0+1, b.Min.Y+(ty+1)*h/th)
		for tx := range tw {
			x0 := b.Min.X + tx*w/tw
			x1 := max(x0+1, b.Min.X+(tx+1)*w/tw)
			var r, g, bl, n uint64
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					cr, cg, cb, _ := img.At(x, y).RGBA()
					r += uint64(cr)
					g += uint64(cg)
					bl += uint64(cb)
					n++
				}
			}
			i := out.PixOffset(tx, ty)
			out.Pix[i] = uint8(r / n >> 8)
			out.Pix[i+1] = uint8(g / n >> 8)
			out.Pix[i+2] = uint8(bl / n >> 8)
			out.Pix[i+3] = 0xff
		}
	}
	return out
}

// Aspect returns the width over height of img.
func Aspect(img image.Image) float64 {
	b := img.Bounds()
	if b.Dy() == 0 {
		return 1
	}
	return float64(b.Dx()) / float64(b.Dy())
}

// Size returns the cells that img takes when drawn into at most maxCols by
// maxRows cells with its aspect kept.
func Size(img image.Image, maxCols, maxRows int) (cols, rows int) {
	if img == nil || maxCols <= 0 || maxRows <= 0 {
		return 0, 0
	}
	aspect := Aspect(img)
	// A cell holds two square pixels stacked, so a rows-tall box is 2*rows
	// pixels high and as many cells wide as pixels.
	cols = int(float64(2*maxRows)*aspect + 0.5)
	rows = maxRows
	if cols > maxCols {
		cols = maxCols
		rows = max(1, int(float64(maxCols)/aspect/2+0.5))
	}
	return max(1, cols), max(1, rows)
}

// Render draws img in cols by rows cells as lines of half-block glyphs with
// 24-bit colours. Size gives cols and rows that keep the aspect ratio.
func Render(img image.Image, cols, rows int) []string {
	if img == nil || cols <= 0 || rows <= 0 {
		return nil
	}
	px := resample(img, cols, rows*2)
	lines := make([]string, rows)
	var sb strings.Builder
	for r := range rows {
		sb.Reset()
		for c := range cols {
			top := px.PixOffset(c, 2*r)
			bot := px.PixOffset(c, 2*r+1)
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀",
				px.Pix[top], px.Pix[top+1], px.Pix[top+2], px.Pix[bot], px.Pix[bot+1], px.Pix[bot+2])
		}
		sb.WriteString("\x1b[0m")
		lines[r] = sb.String()
	}
	return lines
}

package artwork

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image/png"
	"math/rand/v2"
	"os"
	"strings"
	"time"
)

// The kitty graphics protocol shows a real image instead of half blocks. The
// image is sent to the terminal once, and the UI then prints Unicode
// placeholder cells (U+10EEEE with row and column diacritics, the image ID in
// the foreground colour) where it should appear. The placeholders are plain
// text, so the TUI renderer moves and redraws them like any other cell.
// kitty and Ghostty support it.

const (
	// kittyPixels is the longest side of the image sent to the terminal:
	// sharp at the size the preview pane draws it, and quick to send.
	kittyPixels = 384
	// kittySlots caps the images kept in the terminal at once. A new image
	// takes over the oldest slot, which frees the old image's memory there.
	kittySlots = 32
	// kittyChunk is the base64 payload size of one escape sequence.
	kittyChunk = 4096
)

// kittyDiacritics are the combining marks that number placeholder rows and
// columns, from kitty's rowcolumn-diacritics.txt.
var kittyDiacritics = []rune{
	0x0305, 0x030D, 0x030E, 0x0310, 0x0312, 0x033D, 0x033E, 0x033F, 0x0346, 0x034A,
	0x034B, 0x034C, 0x0350, 0x0351, 0x0352, 0x0357, 0x035B, 0x0363, 0x0364, 0x0365,
	0x0366, 0x0367, 0x0368, 0x0369, 0x036A, 0x036B, 0x036C, 0x036D, 0x036E, 0x036F,
	0x0483, 0x0484, 0x0485, 0x0486, 0x0487, 0x0592, 0x0593, 0x0594, 0x0595, 0x0597,
	0x0598, 0x0599, 0x059C, 0x059D, 0x059E, 0x059F, 0x05A0, 0x05A1, 0x05A8, 0x05A9,
	0x05AB, 0x05AC, 0x05AF, 0x05C4, 0x0610, 0x0611, 0x0612, 0x0613, 0x0614, 0x0615,
	0x0616, 0x0617, 0x0657, 0x0658,
}

// KittyMaxCells is the most rows or columns a kitty image may take.
var KittyMaxCells = len(kittyDiacritics)

// KittySupported reports whether the terminal draws images with kitty's
// Unicode placeholders. tmux and screen need passthrough for it, so they
// get half blocks.
func KittySupported() bool {
	if os.Getenv("TMUX") != "" || os.Getenv("STY") != "" {
		return false
	}
	term := os.Getenv("TERM")
	return term == "xterm-kitty" || os.Getenv("KITTY_WINDOW_ID") != "" ||
		term == "xterm-ghostty" || os.Getenv("TERM_PROGRAM") == "ghostty"
}

// kittyState is the terminal side of one cached image.
type kittyState struct {
	id         uint32 // image ID, 0 when the image is not in the terminal
	cols, rows int    // the placement the terminal has, or is being sent
	shown      bool   // the terminal has the image and placement
	failed     bool   // the image could not be prepared; use half blocks
	lines      []string
}

// EnableKitty makes the cache draw images with the kitty graphics protocol.
// Image IDs share one namespace per terminal, so each run picks its own high
// byte to keep clear of other programs.
func (c *Cache) EnableKitty() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.kitty = true
	c.kittyBase = uint32(1+rand.IntN(254)) << 16
}

// KittyEnabled reports whether EnableKitty was called.
func (c *Cache) KittyEnabled() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.kitty
}

// KittyUpdate returns the work that puts url in the terminal at the size it
// takes in maxCols by maxRows cells, or nil when there is none: the image is
// not loaded yet, already there at that size, or kitty is off. The returned
// function blocks (it reads and encodes the image), so run it off the UI
// goroutine, write the escape sequences it returns, then call KittyShown.
func (c *Cache) KittyUpdate(url string, maxCols, maxRows int) func() string {
	if c == nil || url == "" {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.images[url]
	if !c.kitty || !ok || e.kitty.failed {
		return nil
	}
	cols, rows := Size(e.img, min(maxCols, KittyMaxCells), min(maxRows, KittyMaxCells))
	k := &e.kitty
	if k.id != 0 && k.cols == cols && k.rows == rows {
		return nil
	}
	k.cols, k.rows, k.shown, k.lines = cols, rows, false, nil
	if k.id != 0 {
		id := k.id
		return func() string { return kittyPlace(id, cols, rows) }
	}
	// Take the next slot and drop the image that held it.
	c.kittyNext = c.kittyNext%kittySlots + 1
	if old, ok := c.images[c.kittySlot[c.kittyNext]]; ok && old != e {
		old.kitty = kittyState{}
	}
	c.kittySlot[c.kittyNext] = url
	k.id = c.kittyBase | uint32(c.kittyNext)
	id := k.id
	return func() string {
		data, err := c.kittyPNG(url)
		if err != nil {
			c.mu.Lock()
			if e, ok := c.images[url]; ok {
				e.kitty = kittyState{failed: true}
			}
			c.mu.Unlock()
			return ""
		}
		return kittyTransmit(id, cols, rows, data)
	}
}

// KittyShown records that the terminal received what KittyUpdate sent for
// url, so KittyDraw may print its placeholders.
func (c *Cache) KittyShown(url string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.images[url]; ok && e.kitty.id != 0 {
		e.kitty.shown = true
	}
}

// KittyDraw returns the placeholder lines of url drawn into at most maxCols
// by maxRows cells, and its size in cells. ok is false until the terminal
// has the image at that size; draw half blocks meanwhile.
func (c *Cache) KittyDraw(url string, maxCols, maxRows int) (lines []string, cols, rows int, ok bool) {
	if c == nil || url == "" {
		return nil, 0, 0, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, found := c.images[url]
	if !found || !e.kitty.shown {
		return nil, 0, 0, false
	}
	cols, rows = Size(e.img, min(maxCols, KittyMaxCells), min(maxRows, KittyMaxCells))
	k := &e.kitty
	if k.cols != cols || k.rows != rows {
		return nil, 0, 0, false
	}
	if k.lines == nil {
		k.lines = kittyPlaceholders(k.id, cols, rows)
	}
	return k.lines, cols, rows, true
}

// kittyPNG reads url again at kitty resolution, as PNG: the cache keeps
// only a small copy in memory.
func (c *Cache) kittyPNG(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	img, err := c.decode(ctx, url, kittyPixels)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("artwork: encode %s: %w", url, err)
	}
	return buf.Bytes(), nil
}

// kittyTransmit sends a PNG and makes a virtual placement of cols by rows
// cells for it. q=2 keeps the terminal from answering into the input.
func kittyTransmit(id uint32, cols, rows int, data []byte) string {
	enc := base64.StdEncoding.EncodeToString(data)
	var sb strings.Builder
	for first := true; first || enc != ""; first = false {
		chunk := enc[:min(len(enc), kittyChunk)]
		enc = enc[len(chunk):]
		more := 0
		if enc != "" {
			more = 1
		}
		if first {
			fmt.Fprintf(&sb, "\x1b_Ga=T,U=1,q=2,f=100,i=%d,c=%d,r=%d,m=%d;%s\x1b\\", id, cols, rows, more, chunk)
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", more, chunk)
		}
	}
	return sb.String()
}

// kittyPlace replaces the placement of an image the terminal has.
func kittyPlace(id uint32, cols, rows int) string {
	return fmt.Sprintf("\x1b_Ga=d,d=i,i=%d,q=2\x1b\\\x1b_Ga=p,U=1,q=2,i=%d,c=%d,r=%d\x1b\\", id, id, cols, rows)
}

// kittyPlaceholders returns the rows of placeholder cells for image id. The
// foreground colour carries the 24-bit ID.
func kittyPlaceholders(id uint32, cols, rows int) []string {
	lines := make([]string, rows)
	var sb strings.Builder
	for r := range rows {
		sb.Reset()
		fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%dm", id>>16&0xff, id>>8&0xff, id&0xff)
		for c := range cols {
			sb.WriteRune(0x10EEEE)
			sb.WriteRune(kittyDiacritics[r])
			sb.WriteRune(kittyDiacritics[c])
		}
		sb.WriteString("\x1b[39m")
		lines[r] = sb.String()
	}
	return lines
}

package ui

import (
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/xaviiromf/cliamp-x/ui/artwork"
)

// The cover visualizers draw the cover of the playing song with true-colour
// half blocks (two pixels a cell) and move it with the music. Without a
// cover they show a note.

// SetCover gives the cover visualizers the playing song's cover, or nil.
func (v *Visualizer) SetCover(img image.Image) {
	if img == v.cover {
		return
	}
	v.cover = img
	v.coverPx = nil
}

// coverAt returns the cover resampled to w by h pixels, cached per size.
func (v *Visualizer) coverAt(w, h int) *image.RGBA {
	if v.cover == nil || w <= 0 || h <= 0 {
		return nil
	}
	if v.coverPx != nil && v.coverPx.Bounds().Dx() == w && v.coverPx.Bounds().Dy() == h {
		return v.coverPx
	}
	v.coverPx = artwork.Resample(v.cover, w, h)
	return v.coverPx
}

// noCover is the frame of a cover visualizer while there is no cover.
func (v *Visualizer) noCover() string {
	lines := make([]string, max(1, v.Rows))
	for i := range lines {
		lines[i] = strings.Repeat(" ", v.Cols)
	}
	msg := "♪"
	pad := max(0, (v.Cols-1)/2)
	lines[len(lines)/2] = strings.Repeat(" ", pad) + specWrap(0.4, msg) + strings.Repeat(" ", max(0, v.Cols-pad-1))
	return strings.Join(lines, "\n")
}

// cell is one half-block cell: top and bottom pixel colours; empty cells
// draw a space.
type coverCell struct {
	top, bot [3]uint8
	full     bool
}

// renderCells writes a grid of cells as lines.
func renderCells(cells [][]coverCell) string {
	buf := make([]byte, 0, len(cells)*len(cells[0])*40)
	rgb := func(b []byte, c [3]uint8) []byte {
		b = strconv.AppendUint(b, uint64(c[0]), 10)
		b = append(b, ';')
		b = strconv.AppendUint(b, uint64(c[1]), 10)
		b = append(b, ';')
		return strconv.AppendUint(b, uint64(c[2]), 10)
	}
	for r, row := range cells {
		if r > 0 {
			buf = append(buf, '\n')
		}
		open := false
		for _, c := range row {
			if !c.full {
				if open {
					buf = append(buf, "\x1b[0m"...)
					open = false
				}
				buf = append(buf, ' ')
				continue
			}
			buf = append(buf, "\x1b[38;2;"...)
			buf = rgb(buf, c.top)
			buf = append(buf, "m\x1b[48;2;"...)
			buf = rgb(buf, c.bot)
			buf = append(buf, "m▀"...)
			open = true
		}
		if open {
			buf = append(buf, "\x1b[0m"...)
		}
	}
	return string(buf)
}

func newCells(rows, cols int) [][]coverCell {
	cells := make([][]coverCell, rows)
	for r := range cells {
		cells[r] = make([]coverCell, cols)
	}
	return cells
}

func pixel(img *image.RGBA, x, y int, gain float64) [3]uint8 {
	i := img.PixOffset(x, y)
	f := func(v uint8) uint8 { return uint8(math.Min(255, float64(v)*gain)) }
	return [3]uint8{f(img.Pix[i]), f(img.Pix[i+1]), f(img.Pix[i+2])}
}

// renderCoverPulse draws the cover in the middle, growing and brightening
// with the bass.
func (v *Visualizer) renderCoverPulse(bands []float64) string {
	if v.cover == nil || v.Rows < 1 || v.Cols < 2 {
		return v.noCover()
	}
	bass := bandsBass(bands)
	full := min(v.Cols, 2*v.Rows) // a square cover: N columns, N/2 rows
	side := max(2, int(float64(full)*(0.7+0.3*math.Min(1, bass*1.2))))
	side -= side % 2
	img := v.coverAt(side, side)
	gain := 0.8 + 0.35*bandsAvg(bands)
	cells := newCells(v.Rows, v.Cols)
	x0, y0 := (v.Cols-side)/2, (v.Rows-side/2)/2
	for r := range side / 2 {
		for c := range side {
			y, x := y0+r, x0+c
			if y < 0 || y >= v.Rows || x < 0 || x >= v.Cols {
				continue
			}
			cells[y][x] = coverCell{top: pixel(img, c, 2*r, gain), bot: pixel(img, c, 2*r+1, gain), full: true}
		}
	}
	return renderCells(cells)
}

// renderCoverBars draws spectrum bars that show the cover through them.
func (v *Visualizer) renderCoverBars(bands []float64) string {
	if v.cover == nil || v.Rows < 1 || v.Cols < 2 {
		return v.noCover()
	}
	img := v.coverAt(v.Cols, 2*v.Rows)
	cells := newCells(v.Rows, v.Cols)
	for c := range v.Cols {
		e := bandAt(bands, float64(c)/float64(max(1, v.Cols-1)))
		height := int(math.Round(math.Min(1, e) * float64(2*v.Rows))) // in pixels
		for r := range v.Rows {
			topPx, botPx := 2*r, 2*r+1
			fromBottom := 2*v.Rows - topPx // pixels from the bottom to this cell's top pixel
			if fromBottom-1 > height {
				continue
			}
			cell := coverCell{top: pixel(img, c, topPx, 1.05), bot: pixel(img, c, botPx, 1.05), full: true}
			if fromBottom > height {
				cell.top = [3]uint8{} // only the bottom pixel is inside the bar
			}
			cells[r][c] = cell
		}
	}
	return renderCells(cells)
}

// renderCoverShatter draws the cover as tiles that burst outward with the
// band each tile listens to, and fall back as it calms down.
func (v *Visualizer) renderCoverShatter(bands []float64) string {
	if v.cover == nil || v.Rows < 2 || v.Cols < 4 {
		return v.noCover()
	}
	side := min(v.Cols, 2*v.Rows) * 3 / 4
	side = max(4, side-side%4)
	img := v.coverAt(side, side)
	cells := newCells(v.Rows, v.Cols)
	x0, y0 := (v.Cols-side)/2, (v.Rows-side/2)/2
	const tw, th = 4, 2 // tile size in cells
	cx, cy := float64(side)/2, float64(side/2)/2
	for ty := 0; ty < side/2; ty += th {
		for tx := 0; tx < side; tx += tw {
			// Each tile listens to one band, by its angle around the centre.
			dx, dy := float64(tx)+tw/2-cx, (float64(ty)+th/2-cy)*2
			ang := math.Atan2(dy, dx)
			e := bandAt(bands, (ang+math.Pi)/(2*math.Pi))
			push := math.Pow(math.Min(1, e), 2) * float64(side) * 0.18
			dist := math.Max(1, math.Hypot(dx, dy))
			ox := int(math.Round(dx / dist * push))
			oy := int(math.Round(dy / dist * push / 2))
			for r := ty; r < min(ty+th, side/2); r++ {
				for c := tx; c < min(tx+tw, side); c++ {
					y, x := y0+r+oy, x0+c+ox
					if y < 0 || y >= v.Rows || x < 0 || x >= v.Cols {
						continue
					}
					cells[y][x] = coverCell{top: pixel(img, c, 2*r, 1), bot: pixel(img, c, 2*r+1, 1), full: true}
				}
			}
		}
	}
	return renderCells(cells)
}

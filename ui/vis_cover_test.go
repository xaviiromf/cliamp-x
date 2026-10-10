package ui

import (
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func testCover() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 4), 200, 255})
		}
	}
	return img
}

func TestCoverModesFillTheirBox(t *testing.T) {
	bands := []float64{0.9, 0.7, 0.5, 0.6, 0.8, 0.3, 0.4, 0.2, 0.6, 0.7}
	for _, tt := range []struct {
		name   string
		render func(*Visualizer, []float64) string
	}{
		{"pulse", (*Visualizer).renderCoverPulse},
		{"bars", (*Visualizer).renderCoverBars},
		{"shatter", (*Visualizer).renderCoverShatter},
	} {
		for _, size := range [][2]int{{60, 12}, {150, 40}, {8, 3}} {
			v := NewVisualizer(44100)
			v.Cols, v.Rows = size[0], size[1]
			if out := tt.render(v, bands); strings.Contains(out, "38;2;") {
				t.Fatalf("%s %v: drew the cover without one", tt.name, size)
			}
			v.SetCover(testCover())
			out := tt.render(v, bands)
			lines := strings.Split(out, "\n")
			if len(lines) != v.Rows {
				t.Fatalf("%s %v: %d lines, want %d", tt.name, size, len(lines), v.Rows)
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w != v.Cols {
					t.Fatalf("%s %v line %d: width %d, want %d", tt.name, size, i, w, v.Cols)
				}
			}
			if !strings.Contains(out, "38;2;") {
				t.Fatalf("%s %v: no cover pixels drawn", tt.name, size)
			}
		}
	}
}

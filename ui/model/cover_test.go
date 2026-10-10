package model

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/theme"
)

func hexColor(t *testing.T, h string) color.RGBA {
	t.Helper()
	v, err := strconv.ParseUint(strings.TrimPrefix(h, "#"), 16, 24)
	if err != nil {
		t.Fatalf("bad color %q", h)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}

func TestCoverThemeIsReadable(t *testing.T) {
	// A deep blue cover: the hardest hue to read on a dark background.
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			img.Set(x, y, color.RGBA{10, 20, uint8(140 + x), 255})
		}
	}
	th := coverTheme(img, theme.Theme{Name: "cover"})
	bg := hexColor(t, th.BG)
	for name, c := range map[string]string{"accent": th.Accent, "bright_fg": th.BrightFG, "fg": th.FG, "green": th.Green, "yellow": th.Yellow, "red": th.Red} {
		if r := contrast(hexColor(t, c), bg); r < 4.5 {
			t.Errorf("%s %s contrast %.2f on %s, want at least 4.5", name, c, r, th.BG)
		}
	}
	if err := th.Validate(); err != nil {
		t.Fatalf("cover theme is not a valid theme: %v", err)
	}
}

package model

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/theme"
	"github.com/xaviiromf/cliamp-x/ui/artwork"
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

// coverImg paints a 60x60 image from f.
func coverImg(f func(x, y int) color.RGBA) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, 60, 60))
	for y := range 60 {
		for x := range 60 {
			img.Set(x, y, f(x, y))
		}
	}
	return img
}

func TestCoverThemeFollowsDominantColor(t *testing.T) {
	white, black := color.RGBA{250, 250, 250, 255}, color.RGBA{8, 8, 8, 255}
	red, blue, green := color.RGBA{220, 30, 40, 255}, color.RGBA{30, 60, 220, 255}, color.RGBA{40, 170, 70, 255}
	cases := []struct {
		name      string
		img       image.Image
		light     bool   // the background must be light
		grey      bool   // the background must have no hue
		accentHue string // "red", "blue" or "" for no requirement
		greyText  bool   // accent must be grey too
	}{
		{"all white", coverImg(func(x, y int) color.RGBA { return white }), true, true, "", true},
		{"all black", coverImg(func(x, y int) color.RGBA { return black }), false, true, "", true},
		{"white with red logo", coverImg(func(x, y int) color.RGBA {
			if x >= 24 && x < 36 && y >= 24 && y < 36 {
				return red
			}
			return white
		}), true, true, "red", false},
		{"black with blue logo", coverImg(func(x, y int) color.RGBA {
			if x >= 20 && x < 40 && y >= 24 && y < 36 {
				return blue
			}
			return black
		}), false, true, "blue", false},
		{"red over blue split", coverImg(func(x, y int) color.RGBA {
			if x < 38 {
				return red
			}
			return blue
		}), false, false, "blue", false},
		{"grey gradient", coverImg(func(x, y int) color.RGBA {
			v := uint8(20 + x*3)
			return color.RGBA{v, v, v, 255}
		}), false, true, "", true},
		// A 4:3 YouTube thumbnail: light picture, black bars of 12.5% top and bottom.
		{"letterboxed light photo", coverImg(func(x, y int) color.RGBA {
			switch {
			case y < 8 || y >= 52:
				return black
			case x < 20:
				return color.RGBA{235, 235, 230, 255}
			case x < 40:
				return color.RGBA{240, 200, 200, 255}
			}
			return color.RGBA{200, 215, 240, 255}
		}), true, false, "", false},
		{"photo mix", coverImg(func(x, y int) color.RGBA {
			switch {
			case y < 30:
				return color.RGBA{235, 235, 230, 255}
			case x < 20:
				return green
			case x < 30:
				return red
			}
			return color.RGBA{225, 225, 220, 255}
		}), true, false, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			th := coverTheme(c.img, theme.Theme{Name: "cover"})
			bg := hexColor(t, th.BG)
			if (luminance(bg) > 0.4) != c.light {
				t.Errorf("bg %s: light = %v, want %v", th.BG, !c.light, c.light)
			}
			if _, s, _ := artwork.HSL(bg); c.grey && s > 0.01 {
				t.Errorf("bg %s should be grey, saturation %.2f", th.BG, s)
			}
			h, s, _ := artwork.HSL(hexColor(t, th.Accent))
			switch {
			case c.greyText && s > 0.01:
				t.Errorf("accent %s of a grey cover must be grey", th.Accent)
			case c.accentHue == "red" && !(s > 0.4 && (h < 20 || h > 340)):
				t.Errorf("accent %s must be the red logo", th.Accent)
			case c.accentHue == "blue" && !(s > 0.4 && h > 200 && h < 260):
				t.Errorf("accent %s must be the blue", th.Accent)
			}
			for name, fg := range map[string]string{"accent": th.Accent, "bright_fg": th.BrightFG, "fg": th.FG, "green": th.Green, "yellow": th.Yellow, "red": th.Red} {
				if r := contrast(hexColor(t, fg), bg); r < 4.5 {
					t.Errorf("%s %s contrast %.2f on %s, want at least 4.5", name, fg, r, th.BG)
				}
			}
		})
	}
}

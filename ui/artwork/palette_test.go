package artwork

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestSwatchesOrderByArea(t *testing.T) {
	// Mostly grey with a red and a blue block: grey covers the most.
	img := image.NewRGBA(image.Rect(0, 0, 60, 60))
	for y := range 60 {
		for x := range 60 {
			c := color.RGBA{120, 120, 120, 255}
			switch {
			case x < 20 && y < 30:
				c = color.RGBA{220, 30, 40, 255}
			case x >= 40 && y >= 30:
				c = color.RGBA{30, 60, 220, 255}
			}
			img.Set(x, y, c)
		}
	}
	p := Swatches(img)
	if len(p) != 3 {
		t.Fatalf("swatches = %v, want grey, red and blue", p)
	}
	if _, s, _ := HSL(p[0].Color); s > 0.05 || p[0].Share < 0.6 {
		t.Fatalf("first swatch %v must be the grey that covers 2/3", p[0])
	}
	if p[1].Color.R < 200 && p[1].Color.B < 200 {
		t.Fatalf("red and blue must keep their own swatches, got %v", p)
	}
}

func TestHSLRoundTrip(t *testing.T) {
	for _, c := range []color.RGBA{{255, 0, 0, 255}, {30, 144, 255, 255}, {200, 200, 200, 255}, {12, 80, 40, 255}} {
		h, s, l := HSL(c)
		back := FromHSL(h, s, l)
		if math.Abs(float64(back.R)-float64(c.R)) > 1 || math.Abs(float64(back.G)-float64(c.G)) > 1 || math.Abs(float64(back.B)-float64(c.B)) > 1 {
			t.Fatalf("HSL round trip of %v gave %v", c, back)
		}
	}
}

func TestSwatchesIgnoreLetterbox(t *testing.T) {
	// The picture is four light stripes of about 19% each, so black bars of
	// 25% in all would be the biggest swatch if they counted.
	stripes := []color.RGBA{{240, 240, 240, 255}, {240, 190, 190, 255}, {190, 200, 240, 255}, {240, 235, 180, 255}}
	black := color.RGBA{0, 0, 0, 255}
	tests := []struct {
		name      string
		w, h      int
		top, side int // bar thickness in pixels
		bar       color.RGBA
		wantDark  bool
	}{
		{"black bars top and bottom", 480, 360, 45, 0, black, false},
		{"near-black bars", 480, 360, 45, 0, color.RGBA{14, 12, 10, 255}, false},
		{"pillarbox", 480, 360, 0, 60, black, false},
		{"bars on every side", 480, 360, 40, 40, black, false},
		{"no bars", 480, 360, 0, 0, black, false},
		// A side loses at most a fifth, so a dark cover with a small logo stays dark.
		{"black cover with logo", 480, 360, 150, 150, black, true},
	}
	for _, tc := range tests {
		img := image.NewRGBA(image.Rect(0, 0, tc.w, tc.h))
		for y := range tc.h {
			for x := range tc.w {
				c := stripes[x*4/tc.w]
				if y < tc.top || y >= tc.h-tc.top || x < tc.side || x >= tc.w-tc.side {
					c = tc.bar
				}
				img.Set(x, y, c)
			}
		}
		_, _, l := HSL(Swatches(img)[0].Color)
		if dark := l < 0.2; dark != tc.wantDark {
			t.Errorf("%s: dominant lightness %.2f, dark = %v, want %v", tc.name, l, dark, tc.wantDark)
		}
	}
}

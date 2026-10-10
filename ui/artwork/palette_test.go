package artwork

import (
	"image"
	"image/color"
	"math"
	"testing"
)

func TestPalettePicksVividColors(t *testing.T) {
	// Mostly grey with a red and a blue block: the vivid colors come first.
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
	p := Palette(img, 3)
	if len(p) < 2 {
		t.Fatalf("palette = %v", p)
	}
	for _, c := range p[:2] {
		if _, s, _ := HSL(c); s < 0.5 {
			t.Fatalf("first colors %v must be the vivid ones", p[:2])
		}
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

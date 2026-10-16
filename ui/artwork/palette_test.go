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

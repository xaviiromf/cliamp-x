package artwork

import (
	"image"
	"image/color"
	"math"
	"sort"
)

// Palette returns up to n colors that stand out in img, the most vivid first:
// k-means over a sample of its pixels, then the clusters ranked by how
// saturated and bright they are and how much of the image they cover. Very
// dark, very pale and grey clusters rank last.
func Palette(img image.Image, n int) []color.RGBA {
	if img == nil || n <= 0 {
		return nil
	}
	px := resample(img, 48, 48)
	var pts [][3]float64
	for i := 0; i+3 < len(px.Pix); i += 4 {
		pts = append(pts, [3]float64{float64(px.Pix[i]), float64(px.Pix[i+1]), float64(px.Pix[i+2])})
	}
	k := 8
	centers := make([][3]float64, k)
	for i := range centers {
		centers[i] = pts[i*len(pts)/k]
	}
	counts := make([]int, k)
	for range 12 {
		sums := make([][3]float64, k)
		clear(counts)
		for _, p := range pts {
			best, bestD := 0, math.MaxFloat64
			for c, ctr := range centers {
				d := sq(p[0]-ctr[0]) + sq(p[1]-ctr[1]) + sq(p[2]-ctr[2])
				if d < bestD {
					best, bestD = c, d
				}
			}
			counts[best]++
			for j := range 3 {
				sums[best][j] += p[j]
			}
		}
		for c := range centers {
			if counts[c] > 0 {
				for j := range 3 {
					centers[c][j] = sums[c][j] / float64(counts[c])
				}
			}
		}
	}
	type scored struct {
		c     color.RGBA
		score float64
	}
	var out []scored
	for c, ctr := range centers {
		if counts[c] == 0 {
			continue
		}
		col := color.RGBA{uint8(ctr[0]), uint8(ctr[1]), uint8(ctr[2]), 255}
		_, s, l := HSL(col)
		vivid := s * (1 - math.Abs(l-0.55)*1.6)
		share := float64(counts[c]) / float64(len(pts))
		out = append(out, scored{col, vivid*2 + math.Sqrt(share)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].score > out[j].score })
	var result []color.RGBA
	for _, o := range out {
		// Skip colors too close to one already picked.
		dup := false
		for _, r := range result {
			if sq(float64(r.R)-float64(o.c.R))+sq(float64(r.G)-float64(o.c.G))+sq(float64(r.B)-float64(o.c.B)) < 900 {
				dup = true
				break
			}
		}
		if !dup {
			result = append(result, o.c)
		}
		if len(result) == n {
			break
		}
	}
	return result
}

func sq(x float64) float64 { return x * x }

// HSL returns the hue (0-360), saturation and lightness (0-1) of c.
func HSL(c color.RGBA) (h, s, l float64) {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	default:
		h = (r-g)/d + 4
	}
	return h * 60, s, l
}

// FromHSL builds a color from hue (0-360), saturation and lightness (0-1).
func FromHSL(h, s, l float64) color.RGBA {
	if s == 0 {
		v := uint8(math.Round(l * 255))
		return color.RGBA{v, v, v, 255}
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hue := func(t float64) float64 {
		t = math.Mod(t+1, 1)
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	hn := h / 360
	return color.RGBA{uint8(math.Round(hue(hn+1.0/3) * 255)), uint8(math.Round(hue(hn) * 255)), uint8(math.Round(hue(hn-1.0/3) * 255)), 255}
}

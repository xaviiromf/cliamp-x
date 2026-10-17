package artwork

import (
	"image"
	"image/color"
	"math"
	"sort"
)

// Swatch is a color of an image and the share (0-1) of the image it covers.
type Swatch struct {
	Color color.RGBA
	Share float64
}

// Swatches returns the main colors of img, the one that covers the most area
// first: k-means over a 48x48 sample, seeded with the farthest pixels so a
// small logo keeps its own cluster. White, grey and black count like any
// other color. Clusters closer than 30 in RGB merge into the larger one.
func Swatches(img image.Image) []Swatch {
	if img == nil {
		return nil
	}
	px := resample(trimBars(img), 48, 48)
	var pts [][3]float64
	for i := 0; i+3 < len(px.Pix); i += 4 {
		pts = append(pts, [3]float64{float64(px.Pix[i]), float64(px.Pix[i+1]), float64(px.Pix[i+2])})
	}
	if len(pts) == 0 {
		return nil
	}
	const k = 8
	centers := [][3]float64{pts[0]}
	near := make([]float64, len(pts))
	for i, p := range pts {
		near[i] = dist2(p, pts[0])
	}
	for len(centers) < k {
		far := 0
		for i := range pts {
			if near[i] > near[far] {
				far = i
			}
		}
		if near[far] == 0 {
			break
		}
		centers = append(centers, pts[far])
		for i, p := range pts {
			near[i] = math.Min(near[i], dist2(p, pts[far]))
		}
	}
	counts := make([]int, len(centers))
	for range 12 {
		sums := make([][3]float64, len(centers))
		clear(counts)
		for _, p := range pts {
			best, bestD := 0, math.MaxFloat64
			for c, ctr := range centers {
				if d := dist2(p, ctr); d < bestD {
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
	var all []Swatch
	for c, ctr := range centers {
		if counts[c] > 0 {
			all = append(all, Swatch{color.RGBA{uint8(ctr[0]), uint8(ctr[1]), uint8(ctr[2]), 255}, float64(counts[c]) / float64(len(pts))})
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Share > all[j].Share })
	var out []Swatch
	for _, s := range all {
		merged := false
		for i := range out {
			if Dist(out[i].Color, s.Color) < 30 {
				out[i].Share += s.Share
				merged = true
				break
			}
		}
		if !merged {
			out = append(out, s)
		}
	}
	return out
}

// barMax is the brightest channel a letterbox bar may have, and barSlack how
// far its pixels may differ from each other (JPEG noise).
const (
	barMax   = 24
	barSlack = 16
)

// view is img seen through a smaller rectangle.
type view struct {
	image.Image
	r image.Rectangle
}

func (v view) Bounds() image.Rectangle { return v.r }

// trimBars drops the uniform near-black bars at the edges of img, such as the
// ones of a 4:3 YouTube thumbnail with a 16:9 picture, so they do not count as
// the main color of the cover. A side loses at most a fifth of the image and
// nothing under a sixteenth: a
// dark cover with a small logo stays dark.
func trimBars(img image.Image) image.Image {
	b := img.Bounds()
	// bar reports whether the line from (x, y) in steps of (dx, dy), n pixels, is a bar.
	bar := func(x, y, dx, dy, n int) bool {
		var lo, hi uint32 = 0xff, 0
		for range n {
			r, g, bl, _ := img.At(x, y).RGBA()
			for _, c := range [3]uint32{r >> 8, g >> 8, bl >> 8} {
				lo, hi = min(lo, c), max(hi, c)
			}
			x, y = x+dx, y+dy
		}
		return hi <= barMax && hi-lo <= barSlack
	}
	r := b
	for r.Min.Y < b.Min.Y+b.Dy()/5 && bar(b.Min.X, r.Min.Y, 1, 0, b.Dx()) {
		r.Min.Y++
	}
	for r.Max.Y > b.Max.Y-b.Dy()/5 && bar(b.Min.X, r.Max.Y-1, 1, 0, b.Dx()) {
		r.Max.Y--
	}
	for r.Min.X < b.Min.X+b.Dx()/5 && bar(r.Min.X, b.Min.Y, 0, 1, b.Dy()) {
		r.Min.X++
	}
	for r.Max.X > b.Max.X-b.Dx()/5 && bar(r.Max.X-1, b.Min.Y, 0, 1, b.Dy()) {
		r.Max.X--
	}
	// A thin dark edge is no letterbox and changes no color: leave it.
	if r.Min.Y-b.Min.Y < b.Dy()/16 {
		r.Min.Y = b.Min.Y
	}
	if b.Max.Y-r.Max.Y < b.Dy()/16 {
		r.Max.Y = b.Max.Y
	}
	if r.Min.X-b.Min.X < b.Dx()/16 {
		r.Min.X = b.Min.X
	}
	if b.Max.X-r.Max.X < b.Dx()/16 {
		r.Max.X = b.Max.X
	}
	if r == b {
		return img
	}
	return view{img, r}
}

// Dist is the RGB distance of two colors, 0-441.
func Dist(a, b color.RGBA) float64 {
	return math.Sqrt(dist2([3]float64{float64(a.R), float64(a.G), float64(a.B)}, [3]float64{float64(b.R), float64(b.G), float64(b.B)}))
}

func dist2(a, b [3]float64) float64 { return sq(a[0]-b[0]) + sq(a[1]-b[1]) + sq(a[2]-b[2]) }

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

package ui

import "math"

// renderHelix draws a turning double helix across the panel. The strands
// swell with the overall level, and each rung lights with the band under it.
func (v *Visualizer) renderHelix(bands []float64) string {
	return v.renderDotGrid(&v.xGrid, func(g *brailleGrid, w, h int) {
		mid := float64(h-1) / 2
		avg := bandsAvg(bands)
		amp := math.Max(0.5, mid*(0.35+0.65*math.Min(1, avg*1.3)))
		phase := float64(v.frame) * 0.12
		k := 2 * math.Pi / math.Max(24, float64(w)/2.5)
		rungEvery := max(4, w/24)
		for x := range w {
			s := math.Sin(float64(x)*k + phase)
			y1, y2 := mid+amp*s, mid-amp*s
			front := int8(3)
			back := int8(1)
			if math.Cos(float64(x)*k+phase) < 0 {
				front, back = back, front
			}
			g.set(x, int(math.Round(y1)), front)
			g.set(x, int(math.Round(y2)), back)
			if x%rungEvery == 0 {
				e := bandAt(bands, float64(x)/math.Max(1, float64(w-1)))
				if e > 0.12 {
					gridLine(g, float64(x), y1, float64(x), y2, levelTier(e))
				}
			}
		}
	})
}

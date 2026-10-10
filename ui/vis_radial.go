package ui

import "math"

// renderRadial draws the spectrum as spokes around a ring, mirrored left and
// right, slowly turning. The ring stays round at every panel size.
func (v *Visualizer) renderRadial(bands []float64) string {
	return v.renderDotGrid(&v.xGrid, func(g *brailleGrid, w, h int) {
		cx, cy := float64(w-1)/2, float64(h-1)/2
		maxR := math.Max(1, math.Min(cx, cy))
		avg := bandsAvg(bands)
		inner := maxR * (0.22 + 0.08*bandsBass(bands))
		spokes := max(24, min(360, int(2*math.Pi*maxR/1.6)))
		rot := float64(v.frame) * 0.01
		for i := range spokes {
			frac := float64(i) / float64(spokes)
			mirror := frac * 2
			if mirror > 1 {
				mirror = 2 - mirror
			}
			e := bandAt(bands, mirror)
			outer := inner + (maxR-inner)*(0.08+0.92*math.Min(1, e*0.85+avg*0.15))
			a := frac*2*math.Pi + rot
			ca, sa := math.Cos(a), math.Sin(a)
			for r := inner; r <= outer; r += 0.7 {
				g.set(int(math.Round(cx+r*ca)), int(math.Round(cy+r*sa)), levelTier((r-inner)/math.Max(1, maxR-inner)))
			}
		}
		gridCircle(g, cx, cy, inner*0.9, levelTier(avg))
	})
}

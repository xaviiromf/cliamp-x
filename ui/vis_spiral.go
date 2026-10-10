package ui

import "math"

// renderSpiral draws three spiral arms that turn with the music. Each point
// of an arm reads the band of its distance from the centre, and loud bands
// thicken their stretch of the arm.
func (v *Visualizer) renderSpiral(bands []float64) string {
	return v.renderDotGrid(&v.xGrid, func(g *brailleGrid, w, h int) {
		cx, cy := float64(w-1)/2, float64(h-1)/2
		maxR := math.Max(1, math.Min(cx, cy)*1.15)
		avg := bandsAvg(bands)
		rot := float64(v.frame) * (0.03 + 0.05*avg)
		const arms = 3
		turns := 1.6
		steps := max(60, int(maxR*turns*8))
		for arm := range arms {
			base := float64(arm) * 2 * math.Pi / arms
			for i := range steps {
				t := float64(i) / float64(steps)
				e := bandAt(bands, t)
				a := base + rot + t*turns*2*math.Pi
				r := t * maxR
				x, y := cx+r*math.Cos(a), cy+r*math.Sin(a)
				tier := levelTier(e)
				g.set(int(math.Round(x)), int(math.Round(y)), tier)
				for k := 1; float64(k) <= e*3; k++ {
					off := float64(k) * 0.9
					g.set(int(math.Round(x-off*math.Sin(a))), int(math.Round(y+off*math.Cos(a))), tier)
				}
			}
		}
	})
}

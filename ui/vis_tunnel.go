package ui

import "math"

// tunnelDriver flies through a tunnel of rectangular rings. The rings rush
// past faster when the music is loud, and the bass shakes the vanishing
// point.
type tunnelDriver struct {
	spectrumDriverBase

	phase float64
	grid  brailleGrid
}

func newTunnelDriver() visModeDriver { return &tunnelDriver{} }

func (*tunnelDriver) OnEnter(*Visualizer) {}

func (d *tunnelDriver) Tick(v *Visualizer, ctx VisTickContext) {
	defaultDriverTick(v, ctx, d.AnalysisSpec(v))
	if ctx.OverlayActive {
		return
	}
	d.phase = math.Mod(d.phase+0.015+0.06*bandsAvg(v.SmoothedBands()), 1)
}

func (d *tunnelDriver) Render(v *Visualizer) string {
	bands := v.SmoothedBands()
	return v.renderDotGrid(&d.grid, func(g *brailleGrid, w, h int) {
		bass := bandsBass(bands)
		cx := float64(w-1)/2 + math.Sin(float64(v.frame)*0.07)*bass*float64(w)*0.04
		cy := float64(h-1)/2 + math.Cos(float64(v.frame)*0.05)*bass*float64(h)*0.06
		const rings = 9
		for k := range rings {
			depth := math.Mod(float64(k)/rings+d.phase, 1)
			s := depth * depth // perspective: rings bunch up far away
			if s < 0.01 {
				continue
			}
			hw, hh := s*float64(w)/2, s*float64(h)/2
			e := bandAt(bands, float64(k)/(rings-1))
			tier := levelTier(math.Min(1, depth*0.6+e*0.6))
			x0, x1 := cx-hw, cx+hw
			y0, y1 := cy-hh, cy+hh
			gridLine(g, x0, y0, x1, y0, tier)
			gridLine(g, x0, y1, x1, y1, tier)
			gridLine(g, x0, y0, x0, y1, tier)
			gridLine(g, x1, y0, x1, y1, tier)
		}
		// Corner rails tie the rings together.
		for _, c := range [][2]float64{{0, 0}, {float64(w - 1), 0}, {0, float64(h - 1)}, {float64(w - 1), float64(h - 1)}} {
			gridLine(g, cx, cy, c[0], c[1], 1)
		}
	})
}

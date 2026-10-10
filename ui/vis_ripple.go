package ui

import "math"

type ripple struct{ x, y, r, maxR, strength float64 }

// rippleDriver drops rings on a pond: each beat starts a ring at a random
// spot, and the ring widens and fades.
type rippleDriver struct {
	spectrumDriverBase

	rings    []ripple
	seed     uint64
	prevBass float64
	cooldown int
	grid     brailleGrid
}

func newRippleDriver() visModeDriver { return &rippleDriver{seed: 0xa11ce} }

func (*rippleDriver) OnEnter(*Visualizer) {}

func (d *rippleDriver) Tick(v *Visualizer, ctx VisTickContext) {
	defaultDriverTick(v, ctx, d.AnalysisSpec(v))
	if ctx.OverlayActive {
		return
	}
	bands := v.SmoothedBands()
	bass, avg := bandsBass(bands), bandsAvg(bands)
	w, h := float64(v.columns()*2), float64(v.Rows*4)
	d.cooldown--
	beat := bass > 0.25 && bass > d.prevBass*1.15 && d.cooldown <= 0
	idle := avg > 0.05 && d.cooldown <= -12
	if (beat || idle) && len(d.rings) < 24 {
		size := math.Max(w, h) * (0.25 + 0.5*bass)
		d.rings = append(d.rings, ripple{
			x: rng64(&d.seed) * w, y: rng64(&d.seed) * h,
			maxR: size, strength: math.Min(1, 0.4+bass),
		})
		d.cooldown = 4
	}
	d.prevBass = bass
	kept := d.rings[:0]
	for _, r := range d.rings {
		r.r += 0.6 + 1.4*avg
		if r.r < r.maxR {
			kept = append(kept, r)
		}
	}
	d.rings = kept
}

func (d *rippleDriver) Render(v *Visualizer) string {
	return v.renderDotGrid(&d.grid, func(g *brailleGrid, w, h int) {
		for _, r := range d.rings {
			life := 1 - r.r/r.maxR
			tier := levelTier(life * r.strength)
			gridCircle(g, r.x, r.y, r.r, tier)
			if r.r > 4 {
				gridCircle(g, r.x, r.y, r.r*0.6, levelTier(life*r.strength*0.5))
			}
		}
	})
}

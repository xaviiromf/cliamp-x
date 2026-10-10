package ui

import "math"

type star struct{ x, y, z, px, py float64 }

// starfieldDriver flies through a field of stars. The overall level sets the
// speed, so loud passages streak into warp lines.
type starfieldDriver struct {
	spectrumDriverBase

	stars []star
	seed  uint64
	speed float64
	grid  brailleGrid
}

func newStarfieldDriver() visModeDriver { return &starfieldDriver{seed: 0x5eed} }

func (*starfieldDriver) OnEnter(*Visualizer) {}

func (d *starfieldDriver) respawn(s *star, far bool) {
	s.x = rng64(&d.seed)*2 - 1
	s.y = rng64(&d.seed)*2 - 1
	s.z = 1
	if !far {
		s.z = 0.1 + rng64(&d.seed)*0.9
	}
	s.px, s.py = math.NaN(), math.NaN()
}

func (d *starfieldDriver) Tick(v *Visualizer, ctx VisTickContext) {
	defaultDriverTick(v, ctx, d.AnalysisSpec(v))
	if ctx.OverlayActive {
		return
	}
	want := max(20, min(700, v.columns()*v.Rows/3))
	for len(d.stars) < want {
		var s star
		d.respawn(&s, false)
		d.stars = append(d.stars, s)
	}
	d.stars = d.stars[:want]
	avg := bandsAvg(v.SmoothedBands())
	d.speed = 0.006 + 0.05*avg*avg + 0.01*avg
	for i := range d.stars {
		s := &d.stars[i]
		s.z -= d.speed
		if s.z <= 0.02 {
			d.respawn(s, true)
		}
	}
}

func (d *starfieldDriver) Render(v *Visualizer) string {
	return v.renderDotGrid(&d.grid, func(g *brailleGrid, w, h int) {
		cx, cy := float64(w)/2, float64(h)/2
		scale := math.Max(cx, cy)
		warp := d.speed > 0.02
		for i := range d.stars {
			s := &d.stars[i]
			x := cx + s.x/s.z*scale*0.5
			y := cy + s.y/s.z*scale*0.5
			if x < 0 || y < 0 || x >= float64(w) || y >= float64(h) {
				d.respawn(s, true)
				continue
			}
			tier := levelTier(1 - s.z)
			if warp {
				// The streak runs back toward where the star was a moment ago.
				back := s.z + d.speed*3
				gridLine(g, cx+s.x/back*scale*0.5, cy+s.y/back*scale*0.5, x, y, tier)
			} else {
				g.set(int(x), int(y), tier)
			}
		}
	})
}

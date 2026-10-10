package ui

import "math"

// Helpers shared by the cliamp-x visualizers (radial, spiral, plasma, helix,
// aurora, skyline, tunnel, starfield, ripple, spectrogram). They draw on the
// Braille dot grid, where a dot is about as wide as it is tall, so circles in
// dot space stay round at every panel size, full screen included.

// bandsAvg returns the mean band level.
func bandsAvg(bands []float64) float64 {
	if len(bands) == 0 {
		return 0
	}
	var sum float64
	for _, b := range bands {
		sum += b
	}
	return sum / float64(len(bands))
}

// bandsBass returns the mean level of the lowest third of the bands.
func bandsBass(bands []float64) float64 {
	if len(bands) == 0 {
		return 0
	}
	return bandsAvg(bands[:max(1, len(bands)/3)])
}

// bandAt returns the band level at pos in [0,1], interpolated between
// neighbouring bands.
func bandAt(bands []float64, pos float64) float64 {
	n := len(bands)
	switch n {
	case 0:
		return 0
	case 1:
		return bands[0]
	}
	pos = max(0, min(1, pos)) * float64(n-1)
	i := int(pos)
	if i >= n-1 {
		return bands[n-1]
	}
	frac := pos - float64(i)
	return bands[i]*(1-frac) + bands[i+1]*frac
}

// levelTier maps a 0-1 level to a grid tier: 1 low, 2 mid, 3 high.
func levelTier(level float64) int8 {
	return int8(specTag(level) + 1)
}

// gridLine draws a straight line of dots from (x0,y0) to (x1,y1).
func gridLine(g *brailleGrid, x0, y0, x1, y1 float64, tier int8) {
	steps := int(math.Max(math.Abs(x1-x0), math.Abs(y1-y0))) + 1
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		g.set(int(math.Round(x0+(x1-x0)*t)), int(math.Round(y0+(y1-y0)*t)), tier)
	}
}

// gridCircle draws a circle outline of radius r around (cx,cy).
func gridCircle(g *brailleGrid, cx, cy, r float64, tier int8) {
	if r < 0.5 {
		g.set(int(cx), int(cy), tier)
		return
	}
	steps := max(8, int(2*math.Pi*r))
	for i := range steps {
		a := 2 * math.Pi * float64(i) / float64(steps)
		g.set(int(math.Round(cx+r*math.Cos(a))), int(math.Round(cy+r*math.Sin(a))), tier)
	}
}

// renderDotGrid sizes g to the panel, lets draw fill it with w by h dots and
// returns the rendered frame.
func (v *Visualizer) renderDotGrid(g *brailleGrid, draw func(g *brailleGrid, w, h int)) string {
	rows, cols := v.Rows, v.columns()
	if rows <= 0 || cols <= 0 {
		return ""
	}
	g.keepRunOnEmpty = true
	g.ensure(rows*4, cols*2)
	draw(g, cols*2, rows*4)
	return g.render(rows, cols)
}

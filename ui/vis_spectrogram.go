package ui

import (
	"strings"
	"time"
)

// spectrogramBands is the frequency resolution of the waterfall.
const spectrogramBands = 32

// spectrogramShades runs from silent to loud.
var spectrogramShades = []rune(" ░▒▓█")

// spectrogramDriver draws a scrolling spectrogram: time runs left to right,
// low frequencies sit at the bottom, and each cell's shade and colour show
// how loud that frequency was.
type spectrogramDriver struct {
	history [][]float64 // one column of band levels per step, oldest first
}

func newSpectrogramDriver() visModeDriver { return &spectrogramDriver{} }

func (*spectrogramDriver) AnalysisSpec(*Visualizer) VisAnalysisSpec {
	return spectrumAnalysisSpec(spectrogramBands)
}

func (*spectrogramDriver) TickInterval(_ *Visualizer, ctx VisTickContext) time.Duration {
	return defaultDriverTickInterval(ctx)
}

func (*spectrogramDriver) OnEnter(*Visualizer) {}

func (*spectrogramDriver) OnLeave(*Visualizer) {}

func (d *spectrogramDriver) Tick(v *Visualizer, ctx VisTickContext) {
	defaultDriverTick(v, ctx, d.AnalysisSpec(v))
	if ctx.OverlayActive || !ctx.Playing {
		return
	}
	cols := v.columns()
	if cols <= 0 {
		return
	}
	column := append([]float64(nil), v.SmoothedBands()...)
	d.history = append(d.history, column)
	if len(d.history) > cols {
		d.history = d.history[len(d.history)-cols:]
	}
}

func (d *spectrogramDriver) Render(v *Visualizer) string {
	rows, cols := v.Rows, v.columns()
	if rows <= 0 || cols <= 0 {
		return ""
	}
	lines := make([]string, rows)
	pad := cols - len(d.history)
	for r := range rows {
		// Row 0 is the top, so it shows the highest frequencies.
		pos := 1 - (float64(r)+0.5)/float64(rows)
		var sb, run strings.Builder
		tag := -2
		for c := range cols {
			level := 0.0
			if i := c - pad; i >= 0 && i < len(d.history) {
				level = bandAt(d.history[i], pos)
			}
			idx := min(len(spectrogramShades)-1, int(level*float64(len(spectrogramShades))))
			newTag := specTag(level)
			if idx == 0 {
				newTag = -1
			}
			if newTag != tag {
				flushStyleRun(&sb, &run, tag)
				tag = newTag
			}
			run.WriteRune(spectrogramShades[idx])
		}
		flushStyleRun(&sb, &run, tag)
		lines[r] = sb.String()
	}
	return strings.Join(lines, "\n")
}

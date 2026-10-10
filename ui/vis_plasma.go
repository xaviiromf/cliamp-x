package ui

import (
	"math"
	"strings"
)

// plasmaShades runs from empty to full, in the order a cell brightens.
var plasmaShades = []rune(" .:-=+*#%@")

// renderPlasma draws the classic demoscene plasma: overlapping sine fields
// that flow across the panel. The music sets how bright and busy it is.
func (v *Visualizer) renderPlasma(bands []float64) string {
	rows, cols := v.Rows, v.columns()
	if rows <= 0 || cols <= 0 {
		return ""
	}
	avg := bandsAvg(bands)
	bass := bandsBass(bands)
	t := float64(v.frame) * 0.06
	// A terminal cell is about twice as tall as wide; halve x so blobs stay round.
	sx := 6.0 / math.Max(12, float64(cols))
	sy := 6.0 / math.Max(6, float64(rows)) * 2
	cx, cy := float64(cols)/2*sx, float64(rows)/2*sy
	gain := 0.35 + 0.65*math.Min(1, avg*1.4)
	lines := make([]string, rows)
	for r := range rows {
		var sb, run strings.Builder
		tag := -2
		y := float64(r) * sy
		for c := range cols {
			x := float64(c) * sx
			val := math.Sin(x+t) + math.Sin(y*1.3-t*0.8) + math.Sin((x+y)*0.7+t*0.6) +
				math.Sin(math.Hypot(x-cx, y-cy)*(1.5+bass)-t*1.4)
			level := (val/4 + 1) / 2 * gain
			idx := min(len(plasmaShades)-1, int(level*float64(len(plasmaShades))))
			newTag := specTag(level)
			if newTag != tag {
				flushStyleRun(&sb, &run, tag)
				tag = newTag
			}
			run.WriteRune(plasmaShades[idx])
		}
		flushStyleRun(&sb, &run, tag)
		lines[r] = sb.String()
	}
	return strings.Join(lines, "\n")
}

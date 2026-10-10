package ui

import (
	"strings"
)

// renderSkyline draws a city at night: one building per band slice, as tall
// as the band is loud, with windows that switch on and off.
func (v *Visualizer) renderSkyline(bands []float64) string {
	rows, cols := v.Rows, v.columns()
	if rows <= 0 || cols <= 0 {
		return ""
	}
	type building struct{ start, width, height int }
	var city []building
	for x, i := 0, 0; x < cols; i++ {
		width := 2 + int(scatterHash(i, 3, 11, 1)*5)
		width = min(width, cols-x)
		e := bandAt(bands, float64(x)/float64(max(1, cols-1)))
		base := 0.15 + 0.25*scatterHash(i, 5, 13, 2)
		height := int((base + (1-base)*min(1, e*1.1)) * float64(rows))
		city = append(city, building{x, width, max(1, height)})
		x += width + 1
	}
	grid := make([][]int8, rows) // 0 sky, 1 wall, 2 dark window, 3 lit window
	for r := range grid {
		grid[r] = make([]int8, cols)
	}
	for _, b := range city {
		for r := rows - b.height; r < rows; r++ {
			for c := b.start; c < b.start+b.width && c < cols; c++ {
				cell := int8(1)
				if b.width >= 3 && c > b.start && c < b.start+b.width-1 && r > rows-b.height {
					cell = 2
					if scatterHash(c, r, int(v.frame/20), 4) < 0.45 {
						cell = 3
					}
				}
				grid[r][c] = cell
			}
		}
	}
	lines := make([]string, rows)
	for r := range rows {
		var sb, run strings.Builder
		tag := -2
		for c := range cols {
			var ch rune
			newTag := 0
			switch grid[r][c] {
			case 0:
				ch, newTag = ' ', -1
			case 1:
				ch = '█'
			case 2:
				ch, newTag = '▪', 1
			default:
				ch, newTag = '▪', 2
			}
			if newTag != tag {
				flushStyleRun(&sb, &run, tag)
				tag = newTag
			}
			run.WriteRune(ch)
		}
		flushStyleRun(&sb, &run, tag)
		lines[r] = sb.String()
	}
	return strings.Join(lines, "\n")
}

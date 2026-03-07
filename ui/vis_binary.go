package ui

import "strings"

// renderBinary draws streaming columns of 0s and 1s that scroll at speeds
// proportional to each band's energy. Higher energy produces more 1s (active
// data) and brighter coloring, creating a raw data-stream aesthetic.
func (v *Visualizer) renderBinary(bands [numBands]float64) string {
	height := v.Rows
	lines := make([]string, height)

	for row := range height {
		var sb strings.Builder
		col := 0
		for b := range numBands {
			w := visBandWidth(b)
			for range w {
				energy := bands[b]

				// Scroll speed per column: higher energy = faster data flow.
				speed := max(1, 4-int(energy*3))
				scroll := int(v.frame) / speed

				// Bit value from position hash (time-independent; scroll creates motion).
				h := scatterHash(b, row+scroll, col, 0)
				oneProb := energy*0.6 + 0.15

				var ch byte
				if h < oneProb {
					ch = '1'
				} else {
					ch = '0'
				}

				// 1s on high-energy bands glow bright; 0s stay dim.
				if ch == '1' && energy > 0.4 {
					sb.WriteString(specHighStyle.Render(string(ch)))
				} else if ch == '1' || energy > 0.3 {
					sb.WriteString(specMidStyle.Render(string(ch)))
				} else {
					sb.WriteString(specLowStyle.Render(string(ch)))
				}
				col++
			}
			if b < numBands-1 {
				sb.WriteString(" ")
				col++
			}
		}
		lines[row] = sb.String()
	}
	return strings.Join(lines, "\n")
}

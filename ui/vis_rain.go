package ui

import "strings"

// rainDropChars are the characters used for falling rain streaks.
var rainDropChars = []rune{'│', '│', '┃', '|', '!', ':'}

// rainSplashChars appear briefly when a drop hits the ground.
var rainSplashChars = []rune{'.', '~', '-', ',', '\''}

// renderRain draws downward-falling rain droplets whose density and speed follow
// the beat. Each column has its own drop with a fixed fall speed. Drops splash
// on a ground line at the bottom row. Higher energy activates more columns and
// creates denser rainfall.
func (v *Visualizer) renderRain(bands [numBands]float64) string {
	height := v.Rows
	lines := make([]string, height)

	for row := range height {
		var sb, run strings.Builder
		tag := -1
		col := 0

		for b := range numBands {
			w := visBandWidth(b)
			for range w {
				energy := bands[b]
				seed := uint64(col)*7919 + 104729

				// Column activity: higher energy = more active rain columns.
				if scatterHash(b, 0, col, v.frame/15) > energy*1.8+0.05 {
					if tag != -1 {
						flushStyleRun(&sb, &run, tag)
						tag = -1
					}
					run.WriteByte(' ')
					col++
					continue
				}

				// Fall speed per column: 1-3 frames per row step.
				speed := 1 + int(seed%3)

				// Drop length: 2-4 characters.
				dropLen := 2 + int((seed/7)%3)

				// Cycle: drop falls through visible height + gap before repeating.
				cycleLen := height + dropLen + 3
				offset := int((seed / 13) % uint64(cycleLen))
				pos := (int(v.frame)/speed + offset) % cycleLen

				dist := pos - row
				isSplash := row == height-1 && pos >= height-1 && pos < height+2

				if isSplash {
					ch := rainSplashChars[seed%uint64(len(rainSplashChars))]
					newTag := 0 // dim splash
					if newTag != tag {
						flushStyleRun(&sb, &run, tag)
						tag = newTag
					}
					run.WriteRune(ch)
				} else if dist >= 0 && dist < dropLen {
					ch := rainDropChars[seed%uint64(len(rainDropChars))]
					var newTag int
					switch {
					case dist == 0:
						newTag = 2 // bright head
					case dist == 1:
						newTag = 1 // mid body
					default:
						newTag = 0 // dim tail
					}
					if newTag != tag {
						flushStyleRun(&sb, &run, tag)
						tag = newTag
					}
					run.WriteRune(ch)
				} else {
					if tag != -1 {
						flushStyleRun(&sb, &run, tag)
						tag = -1
					}
					run.WriteByte(' ')
				}
				col++
			}
			if b < numBands-1 {
				if tag != -1 {
					flushStyleRun(&sb, &run, tag)
					tag = -1
				}
				run.WriteByte(' ')
				col++
			}
		}
		flushStyleRun(&sb, &run, tag)
		lines[row] = sb.String()
	}

	return strings.Join(lines, "\n")
}

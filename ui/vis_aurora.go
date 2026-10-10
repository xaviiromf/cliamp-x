package ui

import "math"

// renderAurora draws drifting curtains of light. Each column of the curtain
// hangs from a slowly waving top edge, reaches down as far as its band is
// loud, and shimmers dot by dot.
func (v *Visualizer) renderAurora(bands []float64) string {
	return v.renderDotGrid(&v.xGrid, func(g *brailleGrid, w, h int) {
		t := float64(v.frame) * 0.04
		fh := float64(h)
		for x := range w {
			fx := float64(x)
			wave := (math.Sin(fx*0.045+t) + math.Sin(fx*0.017-t*0.7) + 0.6*math.Sin(fx*0.11+t*1.9)) / 2.6
			top := fh * (0.12 + 0.18*(wave+1)/2)
			e := bandAt(bands, fx/math.Max(1, float64(w-1)))
			length := fh * (0.15 + 0.75*math.Min(1, e*1.1))
			// Curtains hang in folds: every third dot column is a bright
			// strand, its neighbours carry a fainter shimmer.
			strand := 0.25
			if x%3 == 0 {
				strand = 0.75
			}
			for y := int(top); y < int(math.Min(fh, top+length)); y++ {
				depth := (float64(y) - top) / math.Max(1, length)
				fade := (1 - depth) * (1 - depth)
				if scatterHash(x, y, int(v.frame/3), 7) > strand*(0.25+0.75*fade) {
					continue
				}
				g.set(x, y, levelTier(1-depth*0.9))
			}
		}
	})
}

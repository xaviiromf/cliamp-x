package model

import (
	"fmt"
	"time"

	"github.com/xaviiromf/cliamp-x/theme"
)

// The cover theme takes its colors from each song's cover. A new song blends
// the colors of the last cover into the new ones over coverFadeTime instead
// of jumping, like a slow crossfade of the interface.

// coverFadeTime is how long the colors take to blend.
const coverFadeTime = 600 * time.Millisecond

// themeRGB holds the seven colors of a theme, in the order of themeColors.
type themeRGB [7][3]int

// coverFade is a running blend between two cover themes. shown is the theme
// on screen, the start of the next blend. from and to are parsed once.
type coverFade struct {
	shown    theme.Theme
	to       theme.Theme
	from, at themeRGB
	start    time.Time
	active   bool
}

// startCoverFade blends from the colors on screen to next. The first cover
// theme of a run, or a theme whose colors do not parse, shows at once.
func (m *Model) startCoverFade(next theme.Theme, now time.Time) {
	f := &m.coverFade
	from, okFrom := parseThemeRGB(f.shown)
	to, okTo := parseThemeRGB(next)
	if !okFrom || !okTo {
		applyThemeAll(next)
		*f = coverFade{shown: next}
		return
	}
	*f = coverFade{shown: f.shown, to: next, from: from, at: to, start: now, active: true}
}

// themePalette returns the active theme with the colors on screen. The
// cover theme reports the cover it shows or blends to, under its own name.
func (m Model) themePalette() theme.Theme {
	if m.themeIdx < 0 || m.themeIdx >= len(m.themes) {
		return theme.Theme{Name: theme.DefaultName}
	}
	entry := m.themes[m.themeIdx]
	if m.coverThemeActive() {
		cover := m.coverFade.shown
		if m.coverFade.active {
			cover = m.coverFade.to
		}
		if cover.BG != "" {
			cover.Name = entry.Name
			return cover
		}
	}
	return entry
}

// tickCoverFade moves a running blend to its share at now.
func (m *Model) tickCoverFade(now time.Time) {
	f := &m.coverFade
	if !f.active {
		return
	}
	if !m.coverThemeActive() {
		*f = coverFade{}
		return
	}
	p := float64(now.Sub(f.start)) / float64(coverFadeTime)
	if p >= 1 {
		f.shown, f.active = f.to, false
	} else {
		f.shown = blendTheme(f.from, f.at, max(0, p), f.to.Name)
	}
	applyThemeAll(f.shown)
}

// themeColors lists the color fields of a theme.
func themeColors(t *theme.Theme) [7]*string {
	return [7]*string{&t.BG, &t.Accent, &t.BrightFG, &t.FG, &t.Green, &t.Yellow, &t.Red}
}

// parseThemeRGB reads the "#rrggbb" colors of t; false when one does not
// parse.
func parseThemeRGB(t theme.Theme) (themeRGB, bool) {
	var out themeRGB
	for i, c := range themeColors(&t) {
		if _, err := fmt.Sscanf(*c, "#%02x%02x%02x", &out[i][0], &out[i][1], &out[i][2]); err != nil {
			return out, false
		}
	}
	return out, true
}

// blendTheme mixes the colors of a and b, p of the way to b.
func blendTheme(a, b themeRGB, p float64, name string) theme.Theme {
	t := theme.Theme{Name: name}
	for i, c := range themeColors(&t) {
		var mix [3]int
		for ch := range mix {
			mix[ch] = a[i][ch] + int(float64(b[i][ch]-a[i][ch])*p+0.5)
		}
		*c = fmt.Sprintf("#%02x%02x%02x", mix[0], mix[1], mix[2])
	}
	return t
}

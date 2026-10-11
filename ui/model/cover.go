package model

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/theme"
	"github.com/xaviiromf/cliamp-x/ui"
	"github.com/xaviiromf/cliamp-x/ui/artwork"
)

// The cover of the playing song feeds three features: the cover visualizers,
// the "cover" theme, which takes its colors from it, and the cover beside the
// full-screen visualizer.

// coverThemeName is the theme that follows the cover.
const coverThemeName = "cover"

// SetFullscreenCover shows or hides the cover beside the full-screen
// visualizer.
func (m *Model) SetFullscreenCover(on bool) { m.fullVisCoverOff = !on }

// playingCover returns the playing song's cover URL and image, when loaded.
func (m Model) playingCover() (string, image.Image) {
	t, idx := m.currentPlaybackTrack()
	if idx < 0 || m.art == nil {
		return "", nil
	}
	url := trackArtURL(t)
	img, _ := m.art.Get(url)
	return url, img
}

func (m Model) coverThemeActive() bool {
	return m.themeIdx >= 0 && m.themeIdx < len(m.themes) && strings.EqualFold(m.themes[m.themeIdx].Name, coverThemeName)
}

func (m Model) coverVisActive() bool {
	if m.vis == nil {
		return false
	}
	switch m.vis.Mode {
	case ui.VisCoverPulse, ui.VisCoverBars, ui.VisCoverShatter:
		return true
	}
	return false
}

// coverCmds loads the playing song's cover when a feature needs it, hands
// it to the visualizer, recolors the cover theme and sends it to a kitty
// terminal for the full-screen view.
func (m *Model) coverCmds() tea.Cmd {
	if m.art == nil {
		return nil
	}
	wantTheme := m.coverThemeActive()
	if !wantTheme {
		m.coverThemeURL = ""
		m.coverFade = coverFade{}
	}
	if !wantTheme && !m.coverVisActive() && !m.fullVis {
		return nil
	}
	url, img := m.playingCover()
	if url == "" {
		return nil
	}
	if img == nil {
		if m.art.Claim(url) {
			return loadArtworkCmd(m.art, url)
		}
		return nil
	}
	m.vis.SetCover(img)
	if wantTheme && url != m.coverThemeURL {
		m.coverThemeURL = url
		m.startCoverFade(coverTheme(img, m.themes[m.themeIdx]), time.Now())
	}
	if cols, rows := m.fullVisCoverBox(); cols > 0 {
		if send := m.art.KittyUpdate(url, cols, rows); send != nil {
			return func() tea.Msg { return kittyImageMsg{url: url, seq: send()} }
		}
	}
	return nil
}

// coverTheme builds a theme from the cover's colors: the most vivid one is
// the accent, the next ones color the spectrum. Lightness is pinned so text
// stays readable on the dark background, a shade of the accent's hue.
// fallback gives the colors of a
// cover that has none to offer.
func coverTheme(img image.Image, fallback theme.Theme) theme.Theme {
	cols := artwork.Palette(img, 4)
	if len(cols) == 0 {
		return fallback
	}
	h0, s0, _ := artwork.HSL(cols[0])
	pick := func(i int, light float64) color.RGBA {
		if i < len(cols) {
			h, s, _ := artwork.HSL(cols[i])
			if s > 0.12 {
				return artwork.FromHSL(h, clamp(s, 0.45, 0.9), light)
			}
		}
		// Too few vivid colors: turn the accent's hue.
		return artwork.FromHSL(math.Mod(h0+float64(i)*40, 360), clamp(s0, 0.45, 0.85), light)
	}
	hex := func(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
	bg := artwork.FromHSL(h0, 0.25, 0.08)
	readable := func(c color.RGBA) string { return hex(readableOn(c, bg)) }
	return theme.Theme{
		Name:     fallback.Name,
		BG:       hex(bg),
		Accent:   readable(pick(0, 0.64)),
		BrightFG: readable(artwork.FromHSL(h0, 0.12, 0.92)),
		FG:       readable(artwork.FromHSL(h0, 0.1, 0.68)),
		Green:    readable(pick(1, 0.6)),
		Yellow:   readable(pick(2, 0.62)),
		Red:      readable(pick(3, 0.6)),
	}
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// fullVisCoverBox returns the cells of the cover beside the full-screen
// visualizer, or 0, 0 when it does not show: it is off, there is no cover,
// or the terminal is too small to give the visualizer room.
func (m Model) fullVisCoverBox() (cols, rows int) {
	// A cover visualizer already shows the cover.
	if !m.fullVis || m.fullVisCoverOff || m.art == nil || m.coverVisActive() {
		return 0, 0
	}
	if _, img := m.playingCover(); img == nil {
		return 0, 0
	}
	rows = min(m.layout.fullVisualizerRows, 30)
	if rows < 8 || m.layout.panelWidth < 70 {
		return 0, 0
	}
	cols = min(2*rows, m.layout.panelWidth*2/5)
	return cols, cols / 2
}

// renderFullVisCover joins the cover to the left of the visualizer lines.
func (m Model) renderFullVisCover(spectrum string) string {
	cols, rows := m.fullVisCoverBox()
	if cols == 0 {
		return spectrum
	}
	url, img := m.playingCover()
	var picture []string
	picW := cols
	if drawn, c, r, ok := m.art.KittyDraw(url, cols, rows); ok {
		picture, picW = append(make([]string, max(0, (m.layout.fullVisualizerRows-r)/2)), drawn...), c
	} else if img != nil {
		c, r := artwork.Size(img, cols, rows)
		picture, picW = append(make([]string, max(0, (m.layout.fullVisualizerRows-r)/2)), artwork.Render(img, c, r)...), c
	}
	lines := strings.Split(spectrum, "\n")
	for i := range lines {
		pic := ""
		if i < len(picture) {
			pic = picture[i]
		}
		if pic == "" {
			pic = strings.Repeat(" ", picW)
		}
		lines[i] = pic + "  " + lines[i]
	}
	return strings.Join(lines, "\n")
}

// readableOn lightens c until it has a 4.5:1 contrast on bg, the WCAG level
// for text. Dark blues need the most.
func readableOn(c, bg color.RGBA) color.RGBA {
	h, sat, l := artwork.HSL(c)
	for contrast(c, bg) < 4.5 && l < 0.95 {
		l += 0.03
		c = artwork.FromHSL(h, sat, l)
	}
	return c
}

func contrast(a, b color.RGBA) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c color.RGBA) float64 {
	lin := func(v uint8) float64 {
		x := float64(v) / 255
		if x <= 0.04045 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

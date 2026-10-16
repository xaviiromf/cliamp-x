package model

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
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

// coverTheme builds a theme from the cover's colors. The background is the
// color that covers the most area, so a white cover gives a light theme and a
// black one a dark theme (mid tones go to the nearest dark or light shade of
// the same hue). The accent is the most prominent color that stands out from
// it, the next ones color the spectrum, and every text color is moved along
// lightness until it reads on the background. fallback gives the colors of a
// cover that has none to offer.
func coverTheme(img image.Image, fallback theme.Theme) theme.Theme {
	sw := artwork.Swatches(img)
	if len(sw) == 0 {
		return fallback
	}
	dom := sw[0].Color
	bh, bs, bl := artwork.HSL(dom)
	lightBG := bl >= 0.55
	bgL, tint := clamp(bl, 0.04, 0.2), math.Min(bs, 0.5)
	if lightBG {
		bgL, tint = clamp(bl, 0.88, 0.97), math.Min(bs, 0.4)
	}
	bg := artwork.FromHSL(bh, tint, bgL)
	// Shades are given for a dark background and mirrored on a light one.
	shade := func(l float64) float64 {
		if lightBG {
			return 1 - l
		}
		return l
	}
	// Colors that stand out from the main one, most prominent first: area
	// weighted by saturation, so a small red logo beats a larger grey.
	score := func(s artwork.Swatch) float64 {
		_, sat, _ := artwork.HSL(s.Color)
		return s.Share * (0.1 + sat*sat)
	}
	rest := sw[1:]
	sort.SliceStable(rest, func(i, j int) bool { return score(rest[i]) > score(rest[j]) })
	var cols []color.RGBA
	for _, s := range rest {
		if s.Share >= 0.02 && artwork.Dist(s.Color, dom) > 60 {
			cols = append(cols, s.Color)
		}
	}
	h0, s0 := bh, bs
	if len(cols) > 0 {
		h0, s0, _ = artwork.HSL(cols[0])
	}
	// A grey cover keeps grey colors: its hue is meaningless.
	sat := func(s float64) float64 {
		if s <= 0.12 {
			return s
		}
		return clamp(s, 0.45, 0.9)
	}
	pick := func(i int, light float64) color.RGBA {
		if i < len(cols) {
			if h, s, _ := artwork.HSL(cols[i]); s > 0.12 {
				return artwork.FromHSL(h, sat(s), shade(light))
			}
		}
		// Too few vivid colors: turn the accent's hue.
		return artwork.FromHSL(math.Mod(h0+float64(i)*40, 360), sat(s0), shade(light))
	}
	hex := func(c color.RGBA) string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }
	readable := func(c color.RGBA) string { return hex(readableOn(c, bg)) }
	text := math.Min(bs, 0.12)
	return theme.Theme{
		Name:     fallback.Name,
		BG:       hex(bg),
		Accent:   readable(pick(0, 0.64)),
		BrightFG: readable(artwork.FromHSL(bh, text, shade(0.92))),
		FG:       readable(artwork.FromHSL(bh, text, shade(0.68))),
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

// readableOn moves c along lightness, away from bg, until it has a 4.5:1
// contrast on bg, the WCAG level for text. Dark blues on dark need the most.
func readableOn(c, bg color.RGBA) color.RGBA {
	h, sat, l := artwork.HSL(c)
	step := 0.03
	if contrast(color.RGBA{255, 255, 255, 255}, bg) < contrast(color.RGBA{A: 255}, bg) {
		step = -step
	}
	for contrast(c, bg) < 4.5 && l+step >= 0 && l+step <= 1 {
		l += step
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

package model

import (
	"fmt"
	"math"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
)

// eqPanelState is the EQ panel: a preset list, grouped by genre and use, and
// the ten bands drawn as a curve.
type eqPanelState struct {
	visible    bool
	focusBands bool // the arrows move between bands instead of presets
	cursor     int  // row of the preset list
	scroll     int
}

// eqPanelRow is one row of the preset list: a preset index into eqPresets,
// -1 for the saved custom curve, or a group heading.
type eqPanelRow struct {
	preset  int
	heading string
}

// eqPanelRows lists the genre presets, then the utility presets, then the
// custom curve.
func eqPanelRows() []eqPanelRow {
	rows := []eqPanelRow{{preset: -2, heading: L("Genres")}}
	for i, p := range eqPresets {
		if eqGenrePresets[p.Name] {
			rows = append(rows, eqPanelRow{preset: i})
		}
	}
	rows = append(rows, eqPanelRow{preset: -2, heading: L("Utility")})
	for i, p := range eqPresets {
		if !eqGenrePresets[p.Name] {
			rows = append(rows, eqPanelRow{preset: i})
		}
	}
	return append(rows, eqPanelRow{preset: -1})
}

// eqPanelRowOf returns the list row of the active preset.
func (m Model) eqPanelRowOf() int {
	for i, r := range eqPanelRows() {
		if r.heading == "" && r.preset == m.eqPresetIdx {
			return i
		}
	}
	return 1
}

// openEQPanel opens the EQ panel on the active preset.
func (m *Model) openEQPanel() {
	m.eqPanel.visible = true
	m.eqPanel.focusBands = false
	m.eqPanel.cursor = m.eqPanelRowOf()
	m.eqPanel.scroll = 0
	m.eqPanelAdjustScroll()
}

// eqPanelSelect applies the preset of a list row, as a live preview.
func (m *Model) eqPanelSelect(row int) {
	rows := eqPanelRows()
	if row < 0 || row >= len(rows) || rows[row].heading != "" {
		return
	}
	m.setEQBypass(false)
	if rows[row].preset < 0 {
		m.eqPresetIdx = -1
		m.eqCustomLabel = ""
		m.applyEQBands(m.eqCustomBands)
	} else {
		m.eqPresetIdx = rows[row].preset
		m.applyEQPreset()
	}
	m.eqAutoLabel = ""
	m.scheduleEQSave()
}

// eqPanelMove moves the preset cursor by delta, skipping headings.
func (m *Model) eqPanelMove(delta int) {
	rows := eqPanelRows()
	next := m.eqPanel.cursor
	for range rows {
		next = (next + delta + len(rows)) % len(rows)
		if rows[next].heading == "" {
			break
		}
	}
	m.eqPanel.cursor = next
	m.eqPanelSelect(next)
}

// handleEQPanelKey handles the keys of the EQ panel.
func (m *Model) handleEQPanelKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q", "E", "enter":
		m.eqPanel.visible = false
		return nil
	case "tab", "shift+tab":
		m.eqPanel.focusBands = !m.eqPanel.focusBands
	case "up", "k":
		if m.eqPanel.focusBands {
			m.nudgeEQBand(1)
		} else {
			m.eqPanelMove(-1)
		}
	case "down", "j":
		if m.eqPanel.focusBands {
			m.nudgeEQBand(-1)
		} else {
			m.eqPanelMove(1)
		}
	case "left", "h":
		m.eqPanel.focusBands = true
		m.eqCursor = (m.eqCursor + eqBandCount - 1) % eqBandCount
	case "right", "l":
		m.eqPanel.focusBands = true
		m.eqCursor = (m.eqCursor + 1) % eqBandCount
	case "+", "=":
		m.nudgeEQBand(1)
	case "-":
		m.nudgeEQBand(-1)
	case "0":
		m.setEQBypass(false)
		m.setCustomEQBand(m.eqCursor, 0)
		m.eqPanel.cursor = m.eqPanelRowOf()
	case "r":
		m.setEQBypass(false)
		m.SetEQPreset("Flat", nil)
		m.eqAutoLabel = ""
		m.scheduleEQSave()
		m.eqPanel.cursor = m.eqPanelRowOf()
	case "b":
		m.setEQBypass(!m.eqBypass)
		if m.eqBypass {
			m.status.Show(L("EQ bypassed (A/B): press b to compare"), statusTTLMedium)
		} else {
			m.status.Show(L("EQ on"), statusTTLShort)
		}
	case "a":
		m.toggleEQAuto()
	}
	m.eqPanelAdjustScroll()
	return nil
}

// nudgeEQBand moves the selected band by delta dB, which makes the curve a
// custom one.
func (m *Model) nudgeEQBand(delta float64) {
	m.setEQBypass(false)
	bands := m.player.EQBands()
	gain := max(-12, min(12, bands[m.eqCursor]+delta))
	m.setCustomEQBand(m.eqCursor, gain)
	m.eqAutoLabel = ""
	m.eqPanel.cursor = m.eqPanelRowOf()
}

func (m *Model) eqPanelAdjustScroll() {
	visible := max(1, m.effectivePlaylistVisible()-eqCurveRows-3)
	clampScroll(&m.eqPanel.cursor, &m.eqPanel.scroll, len(eqPanelRows()), visible)
}

// toggleEQAuto turns the genre-following EQ on or off and saves the choice.
func (m *Model) toggleEQAuto() {
	m.eqAuto = !m.eqAuto
	if m.eqAuto {
		m.status.Show(L("Auto EQ: the preset follows the genre of each song"), statusTTLMedium)
		track, _ := m.currentPlaybackTrack()
		m.applyAutoEQ(track)
	} else {
		m.eqAutoLabel = ""
		m.status.Show(L("Auto EQ off"), statusTTLMedium)
	}
	_ = m.saveConfigBool("eq_auto", m.eqAuto)
}

// SetEQAuto turns on or off the genre-following EQ.
func (m *Model) SetEQAuto(on bool) { m.eqAuto = on }

// applyAutoEQ switches to the preset of a starting track's genre when the
// auto EQ is on. The choice is not saved: the saved preset stays what the
// user picked.
func (m *Model) applyAutoEQ(track playlist.Track) {
	if !m.eqAuto {
		return
	}
	p, ok := presetForGenre(track.Genre)
	if !ok {
		return
	}
	for i := range eqPresets {
		if eqPresets[i].Name != p.Name {
			continue
		}
		if m.eqPresetIdx != i {
			m.setEQBypass(false)
			m.eqPresetIdx = i
			m.applyEQPreset()
		}
		m.eqAutoLabel = track.Genre
		return
	}
}

// eqCurveRows is the height of the drawn curve: +12 dB to -12 dB in 3 dB
// steps.
const eqCurveRows = 9

func (m Model) eqPanelHeaderLine() string {
	parts := []string{L("Equalizer"), m.EQPresetName()}
	if pre := m.eqPreamp(); pre < -0.05 {
		parts = append(parts, fmt.Sprintf("preamp %+.1f dB", pre))
	}
	if m.eqAuto {
		label := "AUTO"
		if m.eqAutoLabel != "" {
			label += ": " + m.eqAutoLabel
		}
		parts = append(parts, label)
	}
	if m.eqBypass {
		parts = append(parts, "BYPASS")
	}
	return sepHeader(strings.Join(parts, " · "), m.layout.panelWidth)
}

// renderEQPanelBody draws the curve, the band labels and the preset list.
func (m Model) renderEQPanelBody() string {
	budget := m.effectivePlaylistVisible()
	bands := m.player.EQBands()
	if m.eqBypass {
		bands = m.eqBypassBands
	}
	colWidth := 5
	if m.layout.panelWidth >= 80 {
		colWidth = 6
	}
	var lines []string
	for row := range eqCurveRows {
		level := 12 - 3*row // dB at this row
		var b strings.Builder
		b.WriteString(dimStyle.Render(fmt.Sprintf("%+4d ", level)))
		for i, g := range bands {
			cell := strings.Repeat(" ", colWidth-1)
			filled := (level > 0 && g >= float64(level)-1.5) || (level < 0 && g <= float64(level)+1.5)
			switch {
			case level == 0:
				cell = strings.Repeat("─", colWidth-1)
				if math.Abs(g) < 1.5 {
					cell = strings.Repeat("━", colWidth-1)
				}
			case filled:
				cell = strings.Repeat("█", colWidth-1)
			}
			style := eqInactiveStyle
			if filled || (level == 0 && math.Abs(g) < 1.5) {
				style = playlistItemStyle
			}
			if i == m.eqCursor && m.eqPanel.focusBands {
				style = eqActiveStyle
			}
			b.WriteString(style.Render(cell) + " ")
		}
		lines = append(lines, b.String())
	}
	var freqs, gains strings.Builder
	freqs.WriteString("     ")
	gains.WriteString("     ")
	for i, label := range eqBandLabels {
		style := dimStyle
		if i == m.eqCursor {
			style = eqActiveStyle
		}
		freqs.WriteString(style.Render(fmt.Sprintf("%-*s", colWidth, label)))
		gains.WriteString(style.Render(fmt.Sprintf("%-*s", colWidth, fmt.Sprintf("%+.0f", bands[i]))))
	}
	lines = append(lines, freqs.String(), gains.String())
	lines = append(lines, "     "+dimStyle.Render(fmt.Sprintf("%-*s%-*s%s",
		colWidth*3, L("Bass"), colWidth*3, L("Mids"), L("Treble"))))

	listBudget := budget - len(lines)
	if listBudget <= 0 {
		return bodyLines(lines, budget)
	}
	rows := eqPanelRows()
	items := make([]string, len(rows))
	for i, r := range rows {
		switch {
		case r.heading != "":
			items[i] = dimStyle.Render("── " + r.heading)
		case r.preset < 0:
			items[i] = L("Custom")
		default:
			items[i] = eqPresets[r.preset].Name
		}
		if r.heading == "" && r.preset == m.eqPresetIdx {
			items[i] += " " + activeToggle.Render("●")
		}
	}
	cursor := m.eqPanel.cursor
	if m.eqPanel.focusBands {
		cursor = -1
	}
	return strings.Join(lines, "\n") + "\n" + windowList(items, cursor, m.eqPanel.scroll, listBudget)
}

// preampEngine is implemented by engines with an EQ pre-amp stage.
type preampEngine interface {
	SetPreamp(dB float64)
	Preamp() float64
}

// eqPreamp returns the pre-amp gain of the engine, or 0 without one.
func (m Model) eqPreamp() float64 {
	if p, ok := m.player.(preampEngine); ok {
		return p.Preamp()
	}
	return 0
}

// eqHeadroom is the pre-amp cut for a curve: most of the largest boost, so
// a boosted band cannot push a loud master into the soft clipper much.
func eqHeadroom(bands [eqBandCount]float64) float64 {
	peak := 0.0
	for _, g := range bands {
		peak = max(peak, g)
	}
	return -0.7 * peak
}

// setEQBypass switches the EQ off (flat bands, no pre-amp) for an A/B
// comparison, or back on with the curve it had.
func (m *Model) setEQBypass(on bool) {
	if on == m.eqBypass {
		return
	}
	if on {
		m.eqBypassBands = m.player.EQBands()
		m.eqBypass = true
		m.applyEQBands([eqBandCount]float64{})
		return
	}
	m.eqBypass = false
	m.applyEQBands(m.eqBypassBands)
}

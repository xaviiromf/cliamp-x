package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/xaviiromf/cliamp-x/ui"
)

// The frame: View, the sections of the player screen stacked top to bottom,
// and the padding, centering and clipping to the terminal.

// View renders the full TUI frame from the layout that Update keeps current.
// A headless Model renders nothing.
func (m Model) View() tea.View {
	if m.quitting || m.headless || m.renderPaused {
		return tea.NewView("")
	}
	if m.layout.tooSmall() {
		content := fmt.Sprintf("Terminal too small. Resize to at least 40x10 (current: %dx%d).", m.width, m.height)
		// Wrap the message, so a narrow terminal still shows both sizes.
		content = ansi.Wrap(content, max(1, m.width), "")
		view := tea.NewView(ui.FitRect(content, max(1, m.width), max(1, m.height)))
		view.BackgroundColor = ui.ColorBackground
		if ui.ColorBackground != nil {
			view.ForegroundColor = ui.ColorText
		}
		view.AltScreen = true
		return view
	}

	// Every screen renders within the main frame, so frame and center
	// uniformly.
	rendered := m.renderFrame(m.frameContent())

	view := tea.NewView(rendered)
	view.BackgroundColor = ui.ColorBackground
	if ui.ColorBackground != nil {
		view.ForegroundColor = ui.ColorText
	}
	view.AltScreen = true
	view.ReportFocus = true
	view.WindowTitle = currentTerminalTitle(m.termTitle, m.width, m.terminalTitleValues())
	if m.mouseOn {
		view.MouseMode = tea.MouseModeCellMotion
	}
	return view
}

// frameContent renders the rows of the active screen, before the frame pads
// and centers them.
func (m Model) frameContent() string {
	if m.visualizerVisible() {
		m.refreshVisualizerIfPending()
	}
	if m.activeScreen() == screenFullVisualizer {
		return m.renderFullVisualizer()
	}
	// Overlays render in the playlist region (renderMainBody), with their
	// header/help supplied by renderPlaylistHeader / renderHelp. List-heavy
	// tasks collapse to a compact now-playing summary so they can use the
	// reclaimed rows for browsing.
	contentFirst := m.usesContentFirstLayout()
	body := m.renderBodyRegion()
	return strings.Join(m.mainSections(body, true, contentFirst), "\n")
}

// renderFrame pads the content into the terminal frame, centers it and clips
// it to the terminal. It gives what frameStyle, centerFrame and FitRect give
// together, but measures each line once instead of four times: this runs on
// every visualizer frame. A line wider than the panel takes the general path,
// which wraps it.
func (m Model) renderFrame(content string) string {
	l := m.layout
	height := max(1, m.height)
	slow := func() string { return ui.FitRect(m.centerFrame(l.frameStyle().Render(content)), l.frameWidth, height) }
	if l.panelWidth+2*l.paddingH != l.frameWidth || strings.ContainsAny(content, "\t\r") {
		return slow()
	}
	lines := strings.Split(content, "\n")
	widths := make([]int, len(lines))
	for i, line := range lines {
		widths[i] = textWidth(line)
		if widths[i] > l.panelWidth {
			return slow()
		}
	}
	frameH := len(lines) + 2*l.paddingV
	padTop := max(0, (m.height-frameH)/2)
	blank := strings.Repeat(" ", l.frameWidth)
	side := strings.Repeat(" ", l.paddingH)

	var b strings.Builder
	b.Grow(len(content) + (frameH+padTop)*(l.frameWidth+1))
	rows := 0
	emit := func(line string) {
		if rows >= height {
			return
		}
		if rows > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
		rows++
	}
	for range padTop {
		emit("")
	}
	for range l.paddingV {
		emit(blank)
	}
	for i, line := range lines {
		if rows >= height {
			break
		}
		emit(side + line + strings.Repeat(" ", l.panelWidth-widths[i]) + side)
	}
	for range l.paddingV {
		emit(blank)
	}
	return b.String()
}

func trimTrailingEmpty(sections []string) []string {
	for len(sections) > 0 && sections[len(sections)-1] == "" {
		sections = sections[:len(sections)-1]
	}
	return sections
}

// frameHasSlackForTransient reports whether an empty transient row still fits
// in the terminal after rows content rows plus frame padding. Reserving the
// row only when it fits keeps a message from shifting the frame, without
// hiding the message on exactly-full screens (where appearing and idle states
// already share the same top padding).
func (m Model) frameHasSlackForTransient(rows int) bool {
	if m.height <= 0 {
		return true
	}
	return rows+1+2*m.layout.paddingV <= m.height
}

// renderedRows counts the terminal rows that sections paint, including the
// embedded newlines of multi-line sections such as the spectrum and the
// playlist body.
func renderedRows(sections []string) int {
	rows := 0
	for _, s := range sections {
		rows += strings.Count(s, "\n") + 1
	}
	return rows
}

// mainSections builds the stacked rows of the playback screen for the active
// layout tier, ending with the status line and, unless it is hidden, the hint
// bar above it.
func (m Model) mainSections(playlist string, includeTransient, contentFirst bool) []string {
	if m.usesSimplifiedLayout() {
		sections := []string{
			m.renderSimplifiedTrackInfo(),
			m.renderTimeStatus(),
			m.renderSeekBar(),
		}
		sections = trimTrailingEmpty(sections)
		if includeTransient {
			if line := m.renderTransient(); line != "" || m.frameHasSlackForTransient(renderedRows(sections)) {
				sections = append(sections, line)
			}
		}
		return sections
	}

	var sections []string
	if contentFirst {
		if m.layout.tier == layoutMinimal {
			sections = []string{
				m.renderTrackInfo(),
				m.renderTimeStatus(),
				m.renderSeekBar(),
				m.renderPlaylistHeader(),
			}
		} else {
			sections = []string{
				m.renderTitle(),
				m.renderTrackInfo(),
				m.renderTimeStatus(),
			}
			if m.layout.visualizerRows > 0 {
				sections = append(sections, m.renderSpectrum())
			}
			sections = append(sections, m.renderSeekBar(), m.renderPlaylistHeader())
		}
	} else {
		switch m.layout.tier {
		case layoutCompact:
			sections = []string{
				m.renderTitle(),
				m.renderTrackInfo(),
				m.renderTimeStatus(),
			}
			if !m.visualizerDisabled() {
				sections = append(sections, m.renderSpectrum())
			}
			sections = append(sections,
				m.renderSeekBar(),
				m.renderCompactControls(),
				m.renderCompactSource(),
				m.renderPlaylistHeader(),
			)
		case layoutMinimal:
			sections = []string{
				m.renderTrackInfo(),
				m.renderTimeStatus(),
				m.renderSeekBar(),
				m.renderPlaylistHeader(),
			}
		default:
			sections = []string{
				m.renderTitle(),
				m.renderTrackInfo(),
				m.renderTimeStatus(),
				"",
			}
			if !m.visualizerDisabled() {
				sections = append(sections, m.renderSpectrum())
			}
			sections = append(sections, m.renderSeekBar())
			switch {
			case m.layout.twoColumn:
				// EQ, volume, source, and speed move into the settings pane
				// beside the playlist, so only the shared header row is left.
				sections = append(sections, m.renderColumnHeaders())
			case m.layout.closedSettings:
				// Closing the pane keeps the two readouts worth a row of their
				// own and drops the rest; "e" still cycles the EQ preset and
				// "[" / "]" still change speed. With no pane to stop at, the
				// playlist header rule runs the whole frame width.
				sections = append(sections,
					m.renderSourceVolume(),
					fillSeparator(m.renderPlaylistHeader(), m.layout.panelWidth))
			default:
				sections = append(sections, m.renderControls())
				if source := m.renderProviderPill(); source != "" {
					sections = append(sections, source)
				}
				sections = append(sections, m.renderPlaylistHeader())
			}
		}
	}
	if playlist != "" {
		sections = append(sections, playlist)
	}
	if preview := m.renderPreview(); preview != "" {
		sections = append(sections, preview)
	}
	if !m.layout.twoColumn && m.layout.tier != layoutCompact {
		// The two-column body needs no spacer above the hint bar: the settings
		// pane's own blank tail already separates the footer from the columns.
		// Compact chrome also omits it so the speed row stays on-screen.
		sections = append(sections, "")
	}
	if !m.hideHelpBar {
		sections = append(sections, m.renderHelp())
	}
	// The two-column pane carries speed and the download counters, and the
	// closed layout deliberately shows neither.
	if !m.layout.twoColumn && !m.layout.closedSettings {
		sections = append(sections, m.renderBottomStatus())
	}

	sections = trimTrailingEmpty(sections)
	if includeTransient {
		if line := m.renderTransient(); line != "" || m.frameHasSlackForTransient(renderedRows(sections)) {
			sections = append(sections, line)
		}
	}

	return sections
}

func (m Model) renderTransient() string {
	// Fresh status wins over a sticky m.err so confirmations are not masked
	// by an older network error; the error reappears once status expires.
	if text := m.save.activityText(); text != "" {
		return fitRect(feedbackActivityStyle.Render(text), m.layout.panelWidth, 1)
	}
	if m.status.text != "" {
		text := m.status.text
		style := feedbackSuccessStyle
		switch m.status.kind {
		case feedbackActivity:
			style = feedbackActivityStyle
		case feedbackWarning:
			style = feedbackWarningStyle
			text = "WARN: " + text
		case feedbackError:
			style = errorStyle
			text = "ERR: " + text
		}
		return fitRect(style.Render(text), m.layout.panelWidth, 1)
	}
	if m.err != nil {
		return fitRect(errorStyle.Render(fmt.Sprintf("ERR: %s", m.err)), m.layout.panelWidth, 1)
	}
	if n := len(m.logLines); n > 0 {
		return fitRect(dimStyle.Render(m.logLines[n-1].text), m.layout.panelWidth, 1)
	}
	return ""
}

// centerFrame centers a pre-rendered frame in the terminal.
func (m Model) centerFrame(frame string) string {
	frameW := lipgloss.Width(frame)
	frameH := lipgloss.Height(frame)
	padLeft := max(0, (m.width-frameW)/2)
	padTop := max(0, (m.height-frameH)/2)

	if padLeft == 0 {
		return strings.Repeat("\n", padTop) + frame
	}
	// Indent every line by padLeft spaces.
	prefix := strings.Repeat(" ", padLeft)
	lines := strings.Split(frame, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Repeat("\n", padTop) + strings.Join(lines, "\n")
}

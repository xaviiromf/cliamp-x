package model

import (
	"strings"

	"charm.land/lipgloss/v2"

	"cliamp/playlist"
	"cliamp/ui"
)

// renderedLineCount returns how many rendered lines tracks[from..to) would take.
func renderedLineCount(tracks []playlist.Track, from, to int) int {
	end := min(to, len(tracks))
	if end <= from {
		return 0
	}
	return end - from
}

// defaultPlVisible recalculates the natural plVisible for the current terminal
// height (same logic as the window-resize handler, capped at maxPlVisible).
func (m *Model) defaultPlVisible() int {
	saved := m.plVisible
	m.plVisible = 3 // temporary minimal value for measurement
	defer func() { m.plVisible = saved }()
	probe := strings.Join([]string{
		m.renderTitle(), m.renderTrackInfo(), m.renderTimeStatus(), "",
		m.renderSpectrum(), m.renderSeekBar(), "",
		m.renderControls(), "", m.renderPlaylistHeader(),
		"x", "", m.renderHelp(), m.renderBottomStatus(),
	}, "\n")
	fixedLines := lipgloss.Height(ui.FrameStyle.Render(probe)) - 1
	return max(3, min(maxPlVisible, m.height-fixedLines))
}

// adjustScroll ensures plCursor is visible in the playlist view.
// It accounts for album separator lines that reduce the number of
// tracks that fit in the visible window.
func (m *Model) adjustScroll() {
	tracks := m.playlist.Tracks()
	if len(tracks) == 0 {
		return
	}
	visible := m.effectivePlaylistVisible()
	if visible <= 0 {
		return
	}
	m.plScroll = m.playlistScroll(visible)
}

func (m Model) playlistScroll(visible int) int {
	tracks := m.playlist.Tracks()
	scroll := max(0, m.plScroll)
	if scroll >= len(tracks) {
		scroll = max(0, len(tracks)-1)
	}
	if m.plCursor < scroll {
		return m.plCursor
	}
	if m.plCursor-scroll+1 <= visible {
		return scroll
	}
	return m.plCursor - visible + 1
}

func (m Model) mainFrameFixedLines(includeTransient bool) int {
	content := strings.Join(m.mainSections("", includeTransient), "\n")
	return lipgloss.Height(ui.FrameStyle.Render(content))
}

func (m Model) effectivePlaylistVisible() int {
	available := m.height - m.mainFrameFixedLines(true)
	if available <= 0 {
		return 0
	}
	if m.plVisible <= 0 {
		return 0
	}
	return min(m.plVisible, available)
}

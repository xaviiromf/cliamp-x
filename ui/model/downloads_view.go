package model

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

// The downloads screen (Ctrl+O) lists the song that downloads, the ones that
// wait and the ones that failed with why, so a long album download can be
// followed, canceled, or retried after a network failure.

func (m *Model) openDownloads() {
	m.downloads.visible = true
	m.downloads.scroll = 0
}

func (m *Model) handleDownloadsKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q", "ctrl+o", "backspace":
		m.downloads.visible = false
	case "r":
		return m.retryFailedDownloads()
	case "x":
		if m.downloads.running || len(m.downloads.queue) > 0 {
			m.cancelDownloads()
			m.status.Show(L("Downloads canceled"), statusTTLMedium)
		}
	case "down", "j":
		m.downloads.scroll++
	case "up", "k":
		m.downloads.scroll = max(0, m.downloads.scroll-1)
	case "home", "g":
		m.downloads.scroll = 0
	}
	return nil
}

func (m Model) downloadsHeaderLine() string {
	d := m.downloads
	waiting := len(d.queue)
	if d.running {
		waiting++
	}
	return sepHeader(fmt.Sprintf("%s · %d %s · %d %s", L("Downloads"), waiting, L("waiting"), len(d.failedJobs), L("failed")), m.layout.panelWidth)
}

func (m Model) renderDownloadsBody() string {
	d := m.downloads
	width := m.layout.panelWidth - 4
	var lines []string
	if d.running {
		lines = append(lines, trackStyle.Render(fmt.Sprintf("  ↓ %s  %.0f%%", truncate(d.current.Track.DisplayName(), width-8), d.percent)))
	}
	for _, j := range d.queue {
		lines = append(lines, playlistItemStyle.Render("  · "+truncate(j.Track.DisplayName(), width)))
	}
	if len(d.failedJobs) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, labelStyle.Render("  "+L("Failed")+" · r "+L("retries them")))
		for _, f := range d.failedJobs {
			lines = append(lines, errorStyle.Render("  ✗ "+truncate(f.job.Track.DisplayName(), width)))
			lines = append(lines, dimStyle.Render("    "+truncate(strings.TrimSpace(f.err), width-2)))
		}
	}
	if len(lines) == 0 {
		lines = []string{dimStyle.Render("  " + L("Nothing to download. D downloads the highlighted song, album or playlist."))}
	}
	lines = scrolledLines(lines, d.scroll)
	return bodyLines(lines, m.effectivePlaylistVisible())
}

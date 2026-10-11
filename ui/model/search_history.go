package model

import (
	"encoding/json"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/fileutil"
)

// searchHistoryMax bounds the remembered searches.
const searchHistoryMax = 20

// searchHistory keeps the last provider searches, newest first, so Ctrl+F
// can repeat one with the arrow keys.
type searchHistory struct {
	path  string
	items []string
}

// SetSearchHistory loads the remembered searches from path and saves new
// ones there. An empty path keeps them in memory only.
func (m *Model) SetSearchHistory(path string) {
	m.searchHist = searchHistory{path: path}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &m.searchHist.items)
	}
}

// rememberSearch puts query first in the history and returns the command
// that saves it.
func (m *Model) rememberSearch(query string) tea.Cmd {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	items := slices.DeleteFunc(slices.Clone(m.searchHist.items), func(s string) bool { return strings.EqualFold(s, query) })
	items = append([]string{query}, items...)
	if len(items) > searchHistoryMax {
		items = items[:searchHistoryMax]
	}
	m.searchHist.items = items
	path := m.searchHist.path
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		if data, err := json.Marshal(items); err == nil {
			_ = fileutil.WriteFileAtomic(path, data, 0o600)
		}
		return nil
	}
}

// stepSearchHistory moves through the history in the search input: up to
// older searches, down to newer ones and back to an empty query.
func (m *Model) stepSearchHistory(older bool) {
	pos := m.searchOverlay.histPos
	switch {
	case older && pos < len(m.searchHist.items):
		pos++
	case !older && pos > 0:
		pos--
	default:
		return
	}
	m.searchOverlay.histPos = pos
	m.searchOverlay.query = ""
	if pos > 0 {
		m.searchOverlay.query = m.searchHist.items[pos-1]
	}
	m.searchOverlay.err = ""
}

// recentSearchLines lists the history under an empty search input.
func (m Model) recentSearchLines() []string {
	if len(m.searchHist.items) == 0 {
		return nil
	}
	lines := []string{"", labelStyle.Render("  " + L("Recent searches") + "  ↑ ↓")}
	for i, q := range m.searchHist.items {
		lines = append(lines, cursorLine(truncate(q, m.layout.panelWidth-4), i == m.searchOverlay.histPos-1))
	}
	return lines
}

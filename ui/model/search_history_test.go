package model

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSearchHistoryRemembersAndRecalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "search_history.json")
	m := keybindingTestModel()
	m.SetSearchHistory(path)
	for _, q := range []string{"daft punk", "bad bunny", "Daft Punk "} {
		if cmd := m.rememberSearch(q); cmd != nil {
			cmd()
		}
	}
	if got := strings.Join(m.searchHist.items, "|"); got != "Daft Punk|bad bunny" {
		t.Fatalf("history = %q, want the newest first without repeats", got)
	}

	// The next run reads the saved file.
	next := keybindingTestModel()
	next.SetSearchHistory(path)
	next.searchOverlay = searchOverlayState{visible: true, screen: searchOverlayInput}
	next.handleSearchOverlayInputKey(tea.KeyPressMsg{Code: tea.KeyUp})
	next.handleSearchOverlayInputKey(tea.KeyPressMsg{Code: tea.KeyUp})
	if next.searchOverlay.query != "bad bunny" {
		t.Fatalf("Up Up = %q, want the older search", next.searchOverlay.query)
	}
	next.handleSearchOverlayInputKey(tea.KeyPressMsg{Code: tea.KeyUp}) // no older one
	next.handleSearchOverlayInputKey(tea.KeyPressMsg{Code: tea.KeyDown})
	next.handleSearchOverlayInputKey(tea.KeyPressMsg{Code: tea.KeyDown})
	if next.searchOverlay.query != "" || next.searchOverlay.histPos != 0 {
		t.Fatalf("Down back past the newest = %q at %d, want the empty query", next.searchOverlay.query, next.searchOverlay.histPos)
	}

	for i := range searchHistoryMax + 5 {
		m.rememberSearch(fmt.Sprint("q", i))
	}
	if len(m.searchHist.items) != searchHistoryMax {
		t.Fatalf("%d remembered searches, want at most %d", len(m.searchHist.items), searchHistoryMax)
	}
}

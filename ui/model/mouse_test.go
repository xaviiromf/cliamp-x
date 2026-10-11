package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// rowLine returns the frame line that shows playlist row number n.
func rowLine(t *testing.T, m Model, n string) int {
	t.Helper()
	for i, l := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(ansi.Strip(l), " "+n+". ") {
			return i
		}
	}
	t.Fatalf("row %s not on screen", n)
	return -1
}

func TestMouseClickSelectsThenPlays(t *testing.T) {
	m := newLayoutTestModel(100, 40)
	m.SetMouse(true)
	m.focus = focusPlaylist
	m.recomputeLayout()
	if m.View().MouseMode != tea.MouseModeCellMotion {
		t.Fatal("the view must ask for mouse events")
	}
	y := rowLine(t, m, "4")
	m.handleMouseClick(tea.MouseClickMsg{X: 10, Y: y, Button: tea.MouseLeft})
	if m.plCursor != 3 {
		t.Fatalf("cursor = %d after clicking row 4, want 3", m.plCursor)
	}
	eng := m.player.(*playbackFakeEngine)
	before := len(eng.playCalls)
	cmd := m.handleMouseClick(tea.MouseClickMsg{X: 10, Y: y, Button: tea.MouseLeft})
	if cmd == nil && len(eng.playCalls) == before && m.playlist.Index() != 3 {
		t.Fatal("a second click on the selected row must play it")
	}
}

func TestMouseWheelMovesCursor(t *testing.T) {
	m := newLayoutTestModel(100, 40)
	m.focus = focusPlaylist
	m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	if m.plCursor != 2 {
		t.Fatalf("cursor = %d after two wheel steps, want 2", m.plCursor)
	}
	m.handleMouseWheel(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	if m.plCursor != 1 {
		t.Fatalf("cursor = %d after wheel up, want 1", m.plCursor)
	}
}

func TestLineKinds(t *testing.T) {
	if !isSeekBarLine("   ━━━━●──────  ") || isSeekBarLine("   ── Playlist ──") {
		t.Fatal("seek bar detection")
	}
	if !isSeparatorLine("   ── Álbumes ──") || isSeparatorLine("     OK Computer — Radiohead") {
		t.Fatal("separator detection")
	}
	if nearestCursorLine([]string{"  a", "  > b", "  c", "   > d"}, 3) != 3 {
		t.Fatal("nearest cursor line")
	}
}

func TestHitsActionsPill(t *testing.T) {
	line := "  Esc  Back  Enter  Go  F1  Actions  a  Queue"
	start := strings.Index(line, " F1 ")
	for _, tc := range []struct {
		x    int
		want bool
	}{
		{start - 1, false}, {start, true}, {start + 6, true}, {start + 12, false},
	} {
		if got := hitsActionsPill(line, tc.x); got != tc.want {
			t.Errorf("x=%d: got %v, want %v", tc.x, got, tc.want)
		}
	}
	if hitsActionsPill("no hint here", 3) {
		t.Error("a line without the hint must not hit")
	}
}

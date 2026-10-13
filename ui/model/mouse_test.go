package model

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/xaviiromf/cliamp-x/ui/artwork"
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

// A tap on the chrome above or below a list leaves the cursor alone: the
// title, the visualizer, the EQ row and the hint bar are no rows. A tap on
// the F1 hint opens the actions menu.
func TestMouseClickOutsideTheListKeepsTheCursor(t *testing.T) {
	for _, size := range [][2]int{{60, 32}, {110, 30}} {
		m := newLayoutTestModel(size[0], size[1])
		m.SetMouse(true)
		m.plCursor = 2
		m.adjustScroll()
		lines := strings.Split(ansi.Strip(m.View().Content), "\n")
		find := func(substr string) int {
			for i, l := range lines {
				if strings.Contains(l, substr) {
					return i
				}
			}
			t.Fatalf("%dx%d: no line with %q:\n%s", size[0], size[1], substr, strings.Join(lines, "\n"))
			return -1
		}
		for _, target := range []string{"C L I A M P", "EQ ", "Explore music"} {
			if strings.Contains(target, "EQ") && size[0] >= 80 {
				continue // the full tier keeps EQ in the settings pane beside the rows
			}
			y := find(target)
			m.handleMouseClick(tea.MouseClickMsg{X: 5, Y: y, Button: tea.MouseLeft})
			if m.plCursor != 2 || m.keymap.visible {
				t.Fatalf("%dx%d: a click on %q moved the cursor to %d (menu open: %v)", size[0], size[1], target, m.plCursor, m.keymap.visible)
			}
		}
		help := find(" F1 ")
		m.handleMouseClick(tea.MouseClickMsg{X: strings.Index(lines[help], " F1 ") + 1, Y: help, Button: tea.MouseLeft})
		if !m.keymap.visible {
			t.Fatalf("%dx%d: a click on F1 did not open the actions menu", size[0], size[1])
		}
	}
}

// A tap on the preview pane under a menu is on the chrome: the menu cursor
// stays on its row.
func TestMouseClickOnPreviewKeepsTheMenuCursor(t *testing.T) {
	m := newLayoutTestModel(60, 32)
	m.SetMouse(true)
	m.art = artwork.New("")
	m.openRowMenu()
	m.recomputeLayout()
	head, _, _ := strings.Cut(ansi.Strip(m.renderPreview()), "\n")
	if strings.TrimSpace(head) == "" {
		t.Fatal("the row menu at 60x32 shows no preview")
	}
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	for y, line := range lines {
		if strings.TrimSpace(line) != strings.TrimSpace(head) {
			continue
		}
		for _, dy := range []int{0, 2} {
			m.handleMouseClick(tea.MouseClickMsg{X: 8, Y: y + dy, Button: tea.MouseLeft})
			if m.keymap.cursor != 0 || !m.keymap.visible {
				t.Fatalf("a click %d lines into the preview moved the menu cursor to %d (open: %v)", dy, m.keymap.cursor, m.keymap.visible)
			}
		}
		return
	}
	t.Fatalf("no preview header %q on the screen:\n%s", head, strings.Join(lines, "\n"))
}

// A tap on the EQ curve or its band rows is no preset row: the preset under
// the cursor stays, while a tap on another preset selects it.
func TestMouseClickOnEQCurveKeepsThePreset(t *testing.T) {
	m := newLayoutTestModel(60, 32)
	m.SetMouse(true)
	m.handleKey(keyPress('E'))
	m.recomputeLayout()
	m.eqPanelAdjustScroll()
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	cursor := nearestCursorLine(lines, 0)
	if cursor < 0 || !m.eqPanel.visible {
		t.Fatalf("the EQ panel shows no preset cursor:\n%s", strings.Join(lines, "\n"))
	}
	before := m.eqPanel.cursor
	for y, line := range lines {
		if y < cursor && (strings.Contains(line, "16k") || strings.Contains(line, "+12") || isEQBandNames(strings.TrimSpace(line))) {
			m.handleMouseClick(tea.MouseClickMsg{X: 8, Y: y, Button: tea.MouseLeft})
			if m.eqPanel.cursor != before {
				t.Fatalf("a click on %q moved the preset cursor from %d to %d", strings.TrimSpace(line), before, m.eqPanel.cursor)
			}
		}
	}
	// Another preset: the row under the band names, or the one after the
	// cursor when that row is the cursor or a heading.
	target := -1
	for y, line := range lines {
		if isEQBandNames(strings.TrimSpace(line)) {
			target = y + 1
		}
	}
	if target == cursor || isSeparatorLine(lines[target]) {
		target = cursor + 1
	}
	m.handleMouseClick(tea.MouseClickMsg{X: 8, Y: target, Button: tea.MouseLeft})
	if m.eqPanel.cursor == before {
		t.Fatalf("a click on preset row %q did not select it", strings.TrimSpace(lines[target]))
	}
}

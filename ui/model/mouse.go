package model

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// Mouse support. The wheel moves through the list on screen like the arrow
// keys (and changes the volume in the full-screen visualizer). A click on a
// list row moves the cursor there, and a click on the row already selected
// opens or plays it, as Enter does. A click on the seek bar seeks there.
//
// Clicks are placed on the rendered frame itself: the cursor row is the line
// that starts with ">", and list rows are counted from it, skipping the
// "──" separators. Every list draws its cursor that way, so this needs no
// per-screen coordinates.

// SetMouse turns mouse support on or off.
func (m *Model) SetMouse(on bool) { m.mouseOn = on }

func keyPress(code rune) tea.KeyPressMsg { return tea.KeyPressMsg{Code: code} }

// handleMouseWheel scrolls the list on screen.
func (m *Model) handleMouseWheel(msg tea.MouseWheelMsg) tea.Cmd {
	up := msg.Button == tea.MouseWheelUp
	if !up && msg.Button != tea.MouseWheelDown {
		return nil
	}
	if m.activeScreen() == screenFullVisualizer {
		if up {
			m.adjustVolume(1)
		} else {
			m.adjustVolume(-1)
		}
		return nil
	}
	if up {
		return m.handleKey(keyPress(tea.KeyUp))
	}
	return m.handleKey(keyPress(tea.KeyDown))
}

// handleMouseClick moves the cursor to the clicked row, opens the row that
// is already selected, or seeks on the seek bar.
func (m *Model) handleMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	right := msg.Button == tea.MouseRight
	if msg.Button != tea.MouseLeft && !right {
		return nil
	}
	lines := strings.Split(m.View().Content, "\n")
	if msg.Y < 0 || msg.Y >= len(lines) {
		return nil
	}
	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = ansi.Strip(l)
	}
	if isSeekBarLine(plain[msg.Y]) {
		return m.clickSeek(msg.X)
	}
	help := strings.TrimSpace(ansi.Strip(m.renderHelp()))
	onHelp := func(line string) bool { return help != "" && strings.Contains(line, help) }
	if onHelp(plain[msg.Y]) {
		if hitsActionsPill(plain[msg.Y], msg.X) {
			return m.handleKey(keyPress(tea.KeyF1))
		}
		return nil
	}
	cursor := nearestCursorLine(plain, msg.Y)
	if cursor < 0 {
		return nil
	}
	// The seek bar and the playlist header sit above every list, and the
	// preview pane and the hint bar below it, so a click on or past one is on
	// the chrome, not on a row.
	preview := ""
	if m.layout.previewRows > 1 {
		preview, _, _ = strings.Cut(ansi.Strip(m.renderPreview()), "\n")
		preview = strings.TrimSpace(preview)
	}
	for i := min(cursor+1, msg.Y); i <= max(cursor-1, msg.Y); i++ {
		line := strings.TrimSpace(plain[i])
		if isSeekBarLine(plain[i]) || onHelp(plain[i]) || strings.HasPrefix(line, "▸─") || (preview != "" && line == preview) {
			return nil
		}
	}
	if cursor == msg.Y {
		if right {
			m.openRowMenu()
			return nil
		}
		return m.handleKey(keyPress(tea.KeyEnter))
	}
	// Count the list rows between the cursor and the click.
	steps := 0
	lo, hi := min(cursor, msg.Y), max(cursor, msg.Y)
	for i := lo + 1; i <= hi; i++ {
		if !isSeparatorLine(plain[i]) {
			steps++
		}
	}
	if isSeparatorLine(plain[msg.Y]) {
		return nil
	}
	key := tea.KeyDown
	if msg.Y < cursor {
		key = tea.KeyUp
	}
	var cmds []tea.Cmd
	for range steps {
		cmds = append(cmds, m.handleKey(keyPress(key)))
	}
	// A right click selects the row, then offers what can be done with it.
	if right {
		m.openRowMenu()
	}
	return tea.Batch(cmds...)
}

// clickSeek seeks to the position under column x of the seek bar.
func (m *Model) clickSeek(x int) tea.Cmd {
	_, dur := m.playbackClock()
	if dur <= 0 || m.layout.panelWidth <= 0 || !m.player.Seekable() {
		return nil
	}
	left := m.layout.paddingH + max(0, (m.width-m.layout.frameWidth)/2)
	frac := float64(x-left) / float64(m.layout.panelWidth)
	frac = max(0, min(0.999, frac))
	return m.seekAbsolute(time.Duration(frac * float64(dur)))
}

// isSeekBarLine reports whether a frame line is the seek bar: one run of
// bar glyphs with the head.
func isSeekBarLine(line string) bool {
	s := strings.TrimSpace(line)
	if s == "" || !strings.ContainsAny(s, "●━─") {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("●━─", r) {
			return false
		}
	}
	return strings.Contains(s, "●") || strings.Count(s, "━") > 0
}

// isSeparatorLine reports whether a frame line is a section rule, a section
// header of the actions menu, or blank, which is no list row.
func isSeparatorLine(line string) bool {
	s := strings.TrimSpace(line)
	return s == "" || strings.HasPrefix(s, "─") || strings.HasPrefix(s, "▸─") || strings.HasPrefix(s, "— ")
}

// hitsActionsPill reports whether column x of a frame line falls on the
// "F1 Actions" hint of the help bar.
func hitsActionsPill(line string, x int) bool {
	i := strings.Index(line, " F1 ")
	if i < 0 {
		return false
	}
	start := ansi.StringWidth(line[:i])
	end := start + ansi.StringWidth(" F1  "+L("Actions"))
	return x >= start && x < end
}

// nearestCursorLine returns the line of the list cursor (a line starting
// with ">") nearest to y, or -1.
func nearestCursorLine(lines []string, y int) int {
	best := -1
	for i, l := range lines {
		if !strings.HasPrefix(strings.TrimLeft(l, " "), ">") {
			continue
		}
		if best < 0 || abs(i-y) < abs(best-y) {
			best = i
		}
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

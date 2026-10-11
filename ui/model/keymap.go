package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
)

// keymapEntry is a row in the actions menu (F1 / Ctrl+K). Rows with
// `divider = true` are unselectable section headers (e.g. "— Playback —").
type keymapEntry struct {
	key, action string
	divider     bool
	// run is the key that Enter sends to run the entry. When run is empty,
	// Enter shows hint instead.
	run, hint string
	// main marks an entry of the player screen. When the screen under the
	// menu cannot run it, Enter first closes that screen.
	main bool
	// do runs an entry that has no key of its own, such as "Go to the
	// album" in the row menu.
	do func(*Model) tea.Cmd
}

// actionButton is one button of the actions menu: the key that Enter sends,
// with the label of the registry command that key runs unless label is set.
// A key pair such as Left/Right becomes two buttons with their own labels.
type actionButton struct {
	run, keyLabel, label string
	// replaces is the KeyLabel of a registry command this button stands for
	// although run is not one of its keys, such as Nj for the jump prompt.
	replaces string
}

// actionGroups sorts the player commands of the actions menu into sections,
// so someone who does not know the keys can find a command by what it does.
// A player command that no button covers is listed under "Navigation", so a
// new command cannot go missing from the menu.
var actionGroups = []struct {
	name    string
	buttons []actionButton
}{
	{"Playback", []actionButton{
		{run: "space"}, {run: "s"}, {run: ">"}, {run: "<"},
		{run: "left", keyLabel: "Left", label: "Seek back 5 seconds"},
		{run: "right", keyLabel: "Right", label: "Seek forward 5 seconds"},
		{run: "shift+left", keyLabel: "Shift+Left", label: "Seek back a large step"},
		{run: "shift+right", keyLabel: "Shift+Right", label: "Seek forward a large step"},
		{run: "ctrl+j", keyLabel: "Ctrl+J / Nj", label: "Jump to a time or a percentage (7j = 70%)", replaces: "Nj"},
		{run: "T"},
		{run: "]", keyLabel: "]", label: "Speed up (+0.25x)"},
		{run: "[", keyLabel: "[", label: "Speed down (-0.25x)"},
	}},
	{"Queue and playlist", []actionButton{
		{run: "enter"}, {run: "O"}, {run: "c"}, {run: "a"}, {run: "A"}, {run: "z"}, {run: "r"}, {run: "*"}, {run: "x"},
		{run: "shift+up", keyLabel: "Shift+Up", label: "Move track up"},
		{run: "shift+down", keyLabel: "Shift+Down", label: "Move track down"},
		{run: "home", keyLabel: "Home / g", label: "Go to the top of the list"},
		{run: "end", keyLabel: "End / G", label: "Go to the end of the list"},
		{run: "ctrl+z", keyLabel: "Ctrl+Z", label: "Undo the last playlist or queue change"},
		{run: "w"}, {run: "W"}, {run: "p"},
	}},
	{"Search and browse", []actionButton{
		{run: "ctrl+f"}, {run: "/"}, {run: "M"}, {run: "X"}, {run: "N"}, {run: "Y"}, {run: "L"}, {run: "F"},
		{run: "u"}, {run: "o"}, {run: "esc"},
	}},
	{"Library", []actionButton{
		{run: "f"}, {run: "D"}, {run: "ctrl+o"}, {run: "ctrl+s"}, {run: "i"}, {run: "y"}, {run: "S"}, {run: "R"},
	}},
	{"Sound", []actionButton{
		{run: "+", keyLabel: "+", label: "Volume up"},
		{run: "-", keyLabel: "-", label: "Volume down"},
		{run: "e"}, {run: "E"}, {run: "n"}, {run: "m"}, {run: "d"},
	}},
	{"Screen", []actionButton{
		{run: "t"}, {run: "v"}, {run: "ctrl+v"}, {run: "V"},
		{run: "}", keyLabel: "}", label: "Visualizer more sensitive"},
		{run: "{", keyLabel: "{", label: "Visualizer less sensitive"},
		{run: "ctrl+b"}, {run: "ctrl+g"}, {run: "ctrl+h"}, {run: "ctrl+x"}, {run: "ctrl+i"},
	}},
	{"App", []actionButton{{run: "ctrl+c"}}},
}

// id names an entry for the duplicate check and the last-run memory.
func (e keymapEntry) id() string { return e.key + "\x00" + e.action }

// keyCodes holds the named keys that commands and plugins bind.
var keyCodes = map[string]rune{
	"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "space": tea.KeySpace,
	"backspace": tea.KeyBackspace, "delete": tea.KeyDelete, "insert": tea.KeyInsert,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"home": tea.KeyHome, "end": tea.KeyEnd, "pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown,
	"f1": tea.KeyF1, "f2": tea.KeyF2, "f3": tea.KeyF3, "f4": tea.KeyF4, "f5": tea.KeyF5, "f6": tea.KeyF6,
	"f7": tea.KeyF7, "f8": tea.KeyF8, "f9": tea.KeyF9, "f10": tea.KeyF10, "f11": tea.KeyF11, "f12": tea.KeyF12,
}

// keyPressFor builds a key press whose String value is key, such as "a",
// "ctrl+x" or "shift+left". It returns false when it cannot build one.
func keyPressFor(key string) (tea.KeyPressMsg, bool) {
	var msg tea.KeyPressMsg
	name := key
	for {
		mod, rest, found := strings.Cut(name, "+")
		if !found || mod == "" || rest == "" {
			break
		}
		switch mod {
		case "ctrl":
			msg.Mod |= tea.ModCtrl
		case "alt":
			msg.Mod |= tea.ModAlt
		case "shift":
			msg.Mod |= tea.ModShift
		default:
			return tea.KeyPressMsg{}, false
		}
		name = rest
	}
	if code, ok := keyCodes[name]; ok {
		msg.Code = code
		if code == tea.KeySpace && msg.Mod == 0 {
			msg.Text = " "
		}
	} else if r, size := utf8.DecodeRuneInString(name); r != utf8.RuneError && size == len(name) {
		msg.Code = r
		if msg.Mod == 0 {
			msg.Text = name
		}
	} else {
		return tea.KeyPressMsg{}, false
	}
	return msg, msg.String() == key
}

// ReservedKeys returns a fresh copy of every key described by commandRegistry.
// It is handed to the Lua plugin manager at startup so plugins cannot shadow
// a core action or an active text field.
func ReservedKeys() map[string]bool {
	out := make(map[string]bool)
	for _, command := range commandRegistry {
		for _, key := range command.Keys {
			out[key] = true
		}
	}
	return out
}

// buildKeymapEntries builds the actions menu. It starts with the commands of
// the screen that opened it, then lists every player command as a button,
// grouped by actionGroups, then the plugin keys. The result is cached on open
// so navigation (which calls keymapCount many times per frame) is allocation-free.
func (m Model) buildKeymapEntries() []keymapEntry {
	out := make([]keymapEntry, 0, len(commandRegistry)+6)
	seen := make(map[string]bool)
	mode, screen := m.keymapContext()
	add := func(command commandSpec) {
		entry := keymapEntry{key: command.KeyLabel, action: command.label(m)}
		if seen[entry.id()] {
			return
		}
		seen[entry.id()] = true
		switch run := command.runKey(); {
		case run == "":
			entry.hint = "Close the keymap. Then press " + command.KeyLabel + "."
		case command.Mode&mode == 0:
			entry.hint = entry.action + " is not available in " + screen + "."
		default:
			entry.run = run
		}
		out = append(out, entry)
	}

	if mode != commandModeMain {
		out = append(out, keymapEntry{action: "— " + L("This screen") + ": " + L(screen) + " —", divider: true})
		for _, command := range commandRegistry {
			if command.Mode != commandModeAny && (command.Keymap || command.ContextHelp) && command.enabled(m) && command.Mode&mode != 0 {
				add(command)
			}
		}
	}
	covered := map[string]bool{}
	for _, group := range actionGroups {
		var rows []keymapEntry
		for _, b := range group.buttons {
			command, ok := m.findCommand(commandModeMain, b.run)
			if !ok {
				continue
			}
			covered[command.KeyLabel] = true
			if b.replaces != "" {
				covered[b.replaces] = true
			}
			entry := keymapEntry{key: command.KeyLabel, action: command.label(m), run: b.run, main: true}
			if b.label != "" {
				entry.key, entry.action = b.keyLabel, L(b.label)
			}
			if seen[entry.id()] {
				continue
			}
			seen[entry.id()] = true
			rows = append(rows, entry)
		}
		if len(rows) > 0 {
			out = append(out, keymapEntry{action: "— " + L(group.name) + " —", divider: true})
			out = append(out, rows...)
		}
	}
	var rest []commandSpec
	for _, command := range commandRegistry {
		if command.Keymap && command.enabled(m) && !covered[command.KeyLabel] &&
			command.Mode&(commandModeMain|commandModeProvider) != 0 && !command.Help {
			rest = append(rest, command)
		}
	}
	if len(rest) > 0 {
		out = append(out, keymapEntry{action: "— " + L("Navigation") + " —", divider: true})
		for _, command := range rest {
			add(command)
		}
	}
	if m.luaMgr == nil {
		return out
	}
	binds := m.luaMgr.KeyBindings()
	if len(binds) == 0 {
		return out
	}
	out = append(out, keymapEntry{action: "— " + L("Plugins") + " —", divider: true})
	for _, b := range binds {
		label := b.Description
		if b.Plugin != "" {
			label += "  (" + b.Plugin + ")"
		}
		out = append(out, keymapEntry{key: b.Key, action: label, run: b.Key, main: true})
	}
	return out
}

// keymapContext returns the command context that the keymap lists: the one
// under the keymap itself.
func (m Model) keymapContext() (commandMode, string) {
	m.keymap.visible = false
	return m.commandContext()
}

// commandContext returns the command mode and the screen name of what owns
// the keys: the top overlay, or else the focused control of the main screen.
// The help line and the keymap both use it.
func (m Model) commandContext() (commandMode, string) {
	if spec, ok := m.topOverlay(); ok && spec.context != nil {
		return spec.context(&m)
	}

	switch m.focus {
	case focusProvider:
		if m.provSearch.active {
			return commandModeProviderSearch, "Provider Filter"
		}
		return commandModeProvider, "Provider"
	case focusEQ:
		return commandModeEQ, "Equalizer"
	case focusVolume:
		return commandModeVolume, "Volume"
	case focusShuffle:
		return commandModeShuffle, "Shuffle"
	case focusRepeat:
		return commandModeRepeat, "Repeat"
	case focusSpeed:
		return commandModeSpeed, "Speed"
	case focusProvPill:
		return commandModeProviderPill, "Source"
	default:
		return commandModeMain, "Playlist"
	}
}

func (m *Model) keymapCount() int {
	return m.keymap.viewCount(len(m.keymap.entries))
}

// keymapHeaderLine renders the keymap's single-line header for the playlist
// region: the filter prompt while searching/filtered, otherwise a labeled
// separator with the match count.
func (m Model) keymapHeaderLine() string {
	if m.keymap.isFiltered() {
		return m.filterHeader("Filter: Actions", "keymap", m.keymap.filter, fmt.Sprintf("%d/%d", m.keymapCount(), len(m.keymap.entries)))
	}
	title := L("Actions")
	if m.keymap.title != "" {
		title = m.keymap.title
	}
	return sepHeaderN(title, m.keymap.cursor+1, len(m.keymap.entries), m.layout.panelWidth)
}

// keymapMaybeAdjustScroll keeps the cursor visible in the current keymap window.
func (m *Model) keymapMaybeAdjustScroll(visible int) {
	clampScroll(&m.keymap.cursor, &m.keymap.scroll, m.keymapCount(), visible)
}

// openKeymap resets the keymap state and shows it. Snapshots plugin bindings
// once so the render/navigation code doesn't re-query the plugin manager.
func (m *Model) openKeymap() {
	m.showKeymap(m.buildKeymapEntries(), "")
}

// showKeymap opens the menu with entries under title ("" for the actions
// menu), on the entry run last when it is there.
func (m *Model) showKeymap(entries []keymapEntry, title string) {
	m.keymap.filterList = filterList{}
	m.keymap.entries = entries
	m.keymap.title = title
	m.keymap.visible = true
	// Start on the action used last, so a repeated task is one Enter away.
	for i, e := range m.keymap.entries {
		if !e.divider && e.id() == m.keymap.last {
			m.keymap.cursor = i
		}
	}
	m.skipKeymapDivider(1)
	// The keymap now renders in the playlist region; recompute chrome so its
	// header/help are reflected in the visible-row budget, then fit the cursor.
	m.refreshChrome()
	m.applyHeightMode()
	m.keymapMaybeAdjustScroll(m.effectivePlaylistVisible())
}

// closeKeymap hides the keymap, clears its filter state, and restores playlist
// sizing after the inline header and help line are dismissed.
func (m *Model) closeKeymap() {
	m.keymap.visible = false
	m.keymap.clearFilter()
	m.refreshChrome()
	m.applyHeightMode()
	m.adjustScroll()
}

func (m *Model) handleKeymapSearchKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.filterKey(&m.keymap.filterList, "keymap", msg, len(m.keymap.entries), m.updateKeymapFilter) {
		m.keymapMaybeAdjustScroll(m.effectivePlaylistVisible())
	}
	return nil
}

// handleKeymapKey processes key presses while the keymap overlay is open.
func (m *Model) handleKeymapKey(msg tea.KeyPressMsg) tea.Cmd {
	if m.keymap.filtering {
		return m.handleKeymapSearchKey(msg)
	}

	key := msg.String()
	if (key == "up" || key == "k") && m.keymap.filter != "" && m.keymap.cursor == 0 {
		m.keymap.filtering = true
		return nil
	}
	visible := m.effectivePlaylistVisible()
	if stepListCursor(key, &m.keymap.cursor, m.keymapCount(), visible) {
		dir := 1
		if key == "up" || key == "k" {
			dir = -1
		}
		m.skipKeymapDivider(dir)
		m.keymapMaybeAdjustScroll(visible)
		return nil
	}
	switch key {
	case "esc", "ctrl+k", "f1", "?", "q":
		m.closeKeymap()

	case "/":
		m.keymap.beginFilter()
		m.updateKeymapFilter()
		return nil

	case "ctrl+x":
		m.toggleExpandedView()
		m.keymapMaybeAdjustScroll(m.effectivePlaylistVisible())

	case "backspace", "h":
		if m.keymap.filter != "" {
			m.keymap.filter = ""
			m.updateKeymapFilter()
		} else {
			m.closeKeymap()
		}

	case "enter", "l":
		return m.runKeymapEntry()
	}

	return nil
}

// skipKeymapDivider moves the cursor off a section header, one row at a time
// in direction dir (1 down, -1 up), wrapping around the list.
func (m *Model) skipKeymapDivider(dir int) {
	n := m.keymapCount()
	for range n {
		e, ok := m.selectedKeymapEntry()
		if !ok || !e.divider {
			return
		}
		m.keymap.cursor = (m.keymap.cursor + dir + n) % n
	}
}

// selectedKeymapEntry returns the entry under the keymap cursor.
func (m *Model) selectedKeymapEntry() (keymapEntry, bool) {
	idx, ok := m.keymap.rawIndex(m.keymap.cursor, len(m.keymap.entries))
	if !ok {
		return keymapEntry{}, false
	}
	return m.keymap.entries[idx], true
}

// runKeymapEntry runs the selected command. It closes the keymap and sends
// the command key to handleKey, so the command acts as if the user pressed
// the key on the screen that opened the keymap. An entry that one key press
// cannot run keeps the keymap open and shows its hint.
func (m *Model) runKeymapEntry() tea.Cmd {
	entry, ok := m.selectedKeymapEntry()
	if !ok || entry.divider {
		return nil
	}
	if entry.do != nil {
		m.closeKeymap()
		return entry.do(m)
	}
	msg, ok := keyPressFor(entry.run)
	if !ok {
		hint := entry.hint
		if hint == "" {
			hint = "Close the keymap. Then press " + entry.key + "."
		}
		m.status.Warning(hint, statusTTLMedium)
		return nil
	}
	m.keymap.last = entry.id()
	m.closeKeymap()
	var leave tea.Cmd
	if mode, _ := m.commandContext(); entry.main && mode != commandModeMain {
		// Run in place only when the open screen gives the key the same
		// command; a screen that uses the key for something else is left.
		here, ok := m.findCommand(mode, entry.run)
		player, _ := m.findCommand(commandModeMain, entry.run)
		if !ok || here.Label != player.Label {
			leave = m.leaveToPlayer()
		}
	}
	return tea.Batch(leave, m.handleKey(msg))
}

// leaveToPlayer closes the open screens with Esc, one at a time, and gives
// the focus to the playlist, so a player command from the actions menu runs
// as on the player screen. The bound stops a screen that Esc cannot close.
func (m *Model) leaveToPlayer() tea.Cmd {
	var cmds []tea.Cmd
	for range 8 {
		if _, open := m.topOverlay(); !open {
			break
		}
		cmds = append(cmds, m.handleKey(keyPress(tea.KeyEscape)))
	}
	if _, open := m.topOverlay(); !open {
		m.focus = focusPlaylist
	}
	return tea.Batch(cmds...)
}

// updateKeymapFilter rebuilds the filtered indices and clamps the cursor. An
// empty query keeps every entry. A query skips the section dividers.
func (m *Model) updateKeymapFilter() {
	query := strings.ToLower(m.keymap.filter)
	m.keymap.recompute(len(m.keymap.entries), func(i int) bool {
		e := m.keymap.entries[i]
		if query == "" {
			return true
		}
		return !e.divider && (strings.Contains(strings.ToLower(e.key), query) ||
			strings.Contains(strings.ToLower(e.action), query))
	})
}

// renderKeymapList renders the keymap entries for the playlist region while the
// keymap is open. The header and help line are supplied by the main layout
// (renderPlaylistHeader / renderHelp), mirroring renderVisPickerList.
func (m Model) renderKeymapList() string {
	budget := m.effectivePlaylistVisible()
	if budget <= 0 {
		return ""
	}

	visible := shownRows(&m.keymap.filterList, m.keymap.entries)
	if len(visible) == 0 {
		msg := "(empty)"
		if m.keymap.filter != "" {
			msg = "No matches"
		}
		return strings.Join(fitLines([]string{dimStyle.Render("  " + msg)}, budget), "\n")
	}

	width := m.helpWidth()
	if len(visible) > budget {
		width -= scrollbarCols
	}
	lines := make([]string, 0, budget)
	for i := m.keymap.scroll; i < len(visible) && len(lines) < budget; i++ {
		entry := visible[i]
		if entry.divider {
			lines = append(lines, labelStyle.Render("  "+entry.action))
			continue
		}
		lines = append(lines, m.actionButtonLine(entry, !m.keymap.filtering && i == m.keymap.cursor, width))
	}
	lines = padLines(lines, budget, len(lines))
	if len(visible) > budget {
		lines = withScrollbar(lines, width, len(visible), m.keymap.scroll)
	}
	return strings.Join(lines, "\n")
}

// actionButtonLine draws one row of the actions menu as a button: the action
// on the left and its key on the right, as a pill when selected, so the menu
// reads by what a command does and still teaches its key.
func (m Model) actionButtonLine(entry keymapEntry, selected bool, width int) string {
	key := " " + entry.key + " "
	keyW := textWidth(key)
	action := truncate(entry.action, max(1, width-2-keyW-1))
	gap := strings.Repeat(" ", max(1, width-2-textWidth(action)-keyW))
	switch {
	case selected:
		return playlistSelectedStyle.Render("> "+action) + gap + helpKeyStyle.Render(key)
	case m.keymap.filtering:
		return dimStyle.Render("  " + action + gap + key)
	default:
		return playlistItemStyle.Render("  "+action) + gap + dimStyle.Render(key)
	}
}

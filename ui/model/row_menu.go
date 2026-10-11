package model

import tea "charm.land/bubbletea/v2"

// The row menu (O, or a right click on a row) gathers what can be done with
// the highlighted song, album or playlist, so the actions spread over single
// keys (M, X, D, f, w...) are one list away. It is the actions menu with its
// own entries: each runs its key on the screen under the menu.

// rowMenuKeys are the keys the row menu offers, in order, when the screen
// under it runs them.
var rowMenuKeys = []string{"enter", "a", "M", "X", "D", "f", "w", "i"}

// openRowMenu opens the row menu for the highlighted row of the screen that
// has the keys.
func (m *Model) openRowMenu() {
	mode, _ := m.commandContext()
	var entries []keymapEntry
	for _, key := range rowMenuKeys {
		if command, ok := m.findCommand(mode, key); ok {
			entries = append(entries, keymapEntry{key: command.KeyLabel, action: command.label(*m), run: key})
		}
	}
	if _, ok := m.selectedTrackArtistBrowserTarget(); ok {
		entries = append(entries, keymapEntry{action: L("Go to the artist"), do: func(m *Model) tea.Cmd {
			cmd, _ := m.openSelectedTrackArtistBrowser()
			return cmd
		}})
	}
	if _, _, _, ok := m.selectedTrackAlbum(); ok {
		entries = append(entries, keymapEntry{action: L("Go to the album"), do: (*Model).openSelectedTrackAlbum})
	}
	if len(entries) == 0 {
		m.status.Show(L("Nothing to do with this row"), statusTTLShort)
		return
	}
	title := L("Options")
	if track, ok := m.selectedPlaylistTrack(); ok {
		title += ": " + trackViewName(track)
	}
	m.showKeymap(entries, title)
}

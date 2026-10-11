package model

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

// W saves the whole queue as a new playlist in the YouTube Music account.

type saveQueueState struct {
	active bool
	name   string
	err    string
}

type queueSavedMsg struct {
	name    string
	added   int
	skipped int
	err     error
}

// queueSaver is a provider that can create a playlist and fill it.
type queueSaver interface {
	provider.PlaylistCreator
	provider.PlaylistBatchWriter
}

func (m *Model) findQueueSaver() queueSaver {
	s, _ := findCapable(m, func(queueSaver) bool { return true })
	return s
}

func (m *Model) openSaveQueue() {
	if m.playlist == nil || m.playlist.Len() == 0 {
		m.status.Show(L("The queue is empty"), statusTTLMedium)
		return
	}
	if m.findQueueSaver() == nil {
		m.status.Show(L("No provider can create playlists"), statusTTLMedium)
		return
	}
	m.saveQueue = saveQueueState{active: true, name: L("Queue ") + time.Now().Format("2006-01-02 15:04")}
}

func (m *Model) handleSaveQueueKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Code {
	case tea.KeyEscape:
		m.saveQueue = saveQueueState{}
	case tea.KeyEnter:
		name := strings.TrimSpace(m.saveQueue.name)
		if name == "" {
			m.saveQueue.err = L("Enter a name for the playlist.")
			return nil
		}
		saver := m.findQueueSaver()
		if saver == nil {
			return nil
		}
		tracks := append([]playlist.Track(nil), m.playlist.Tracks()...)
		m.saveQueue = saveQueueState{}
		m.status.Activityf(statusTTLLong, "%s %q…", L("Saving the queue to"), name)
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			id, err := saver.CreatePlaylist(ctx, name)
			if err != nil {
				return queueSavedMsg{name: name, err: err}
			}
			added, skipped, err := saver.AddTracksToPlaylist(ctx, id, tracks)
			return queueSavedMsg{name: name, added: added, skipped: skipped, err: err}
		}
	default:
		if m.editText("save-queue", &m.saveQueue.name, msg) {
			m.saveQueue.err = ""
		}
	}
	return nil
}

func (m *Model) handleQueueSaved(msg queueSavedMsg) {
	if msg.err != nil {
		m.status.Errorf(statusTTLDefault, "%s: %s", L("Saving the queue failed"), msg.err)
		return
	}
	text := L("Playlist %q created with %d songs")
	if msg.skipped > 0 {
		text += L(" (%d local skipped)")
		m.status.Successf(statusTTLDefault, text, msg.name, msg.added, msg.skipped)
	} else {
		m.status.Successf(statusTTLDefault, text, msg.name, msg.added)
	}
	for _, pe := range m.providers {
		if r, ok := pe.Provider.(interface{ Refresh() }); ok {
			r.Refresh()
		}
	}
}

func (m Model) saveQueueHeaderLine() string {
	return m.promptHeader("save-queue", L("YouTube Music playlist name"), m.saveQueue.name)
}

func (m Model) renderSaveQueueBody() string {
	lines := []string{dimStyle.Render("  " + L("Enter saves the whole queue as a new private playlist in your account. Esc cancels."))}
	if m.saveQueue.err != "" {
		lines = append(lines, errorStyle.Render("  "+m.saveQueue.err))
	}
	return bodyLines(lines, m.effectivePlaylistVisible())
}

// toggleMark marks or unmarks the queue row under the cursor and moves down.
func (m *Model) toggleMark() {
	t, ok := m.playlist.Track(m.plCursor)
	if !ok {
		return
	}
	if m.plMarks == nil {
		m.plMarks = map[string]bool{}
	}
	if m.plMarks[t.Path] {
		delete(m.plMarks, t.Path)
	} else {
		m.plMarks[t.Path] = true
	}
	if m.plCursor < m.playlist.Len()-1 {
		m.plCursor++
		m.adjustScroll()
	}
	if n := len(m.plMarks); n > 0 {
		m.status.Showf(statusTTLMedium, "%d %s · x %s · Esc %s", n, L("marked"), L("removes them"), L("clears"))
	}
}

// removeMarkedTracks removes every marked row, last first so the indexes of
// the others hold.
func (m *Model) removeMarkedTracks() tea.Cmd {
	var idx []int
	for i, t := range m.playlist.Tracks() {
		if m.plMarks[t.Path] {
			idx = append(idx, i)
		}
	}
	var cmds []tea.Cmd
	removed := 0
	// The first removal records the undo, with the whole queue. A playlist
	// saved on disk can undo one removal only, so a batch there has none.
	undoable := m.writableLoadedPlaylist() == ""
	m.keepPlCursorRow(func() {
		for k := len(idx) - 1; k >= 0; k-- {
			cmd, err := m.removeTrack(idx[k], undoable && k == len(idx)-1)
			if err == nil {
				removed++
			}
			cmds = append(cmds, cmd)
		}
	})
	clear(m.plMarks)
	m.status.Showf(statusTTLMedium, "%d %s", removed, L("songs removed"))
	return tea.Batch(cmds...)
}

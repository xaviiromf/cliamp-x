package model

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

func TestMarkAndRemoveSeveral(t *testing.T) {
	m := newLayoutTestModel(100, 40)
	m.focus = focusPlaylist
	m.plCursor = 1
	m.toggleMark() // marks row 1, moves to 2
	m.plCursor = 4
	m.toggleMark() // marks row 4
	if len(m.plMarks) != 2 {
		t.Fatalf("marks = %d, want 2", len(m.plMarks))
	}
	before := m.playlist.Len()
	m.removeMarkedTracks()
	if m.playlist.Len() != before-2 || len(m.plMarks) != 0 {
		t.Fatalf("length %d (was %d), marks %d; want two rows gone and no marks", m.playlist.Len(), before, len(m.plMarks))
	}
	// Esc clears marks before leaving the playlist.
	m.toggleMark()
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if len(m.plMarks) != 0 || m.focus != focusPlaylist {
		t.Fatal("Esc must clear the marks and keep the focus")
	}
}

type saverProvider struct {
	commandsTestProvider
	created string
	added   []playlist.Track
}

func (s *saverProvider) CreatePlaylist(_ context.Context, name string) (string, error) {
	s.created = name
	return "pl:VLPLnew", nil
}

func (s *saverProvider) AddTracksToPlaylist(_ context.Context, _ string, tracks []playlist.Track) (int, int, error) {
	s.added = tracks
	return len(tracks), 0, nil
}

func TestSaveQueueAsPlaylist(t *testing.T) {
	saver := &saverProvider{commandsTestProvider: commandsTestProvider{name: "YouTube Music"}}
	m := newLayoutTestModel(100, 40)
	m.providers = []provider.Entry{{Key: "ytmusic", Name: "YouTube Music", Provider: saver}}
	m.openSaveQueue()
	if !m.saveQueue.active || m.activeScreen() != screenSaveQueue {
		t.Fatal("W must open the save-queue prompt")
	}
	m.saveQueue.name = "Mix"
	cmd := m.handleSaveQueueKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter must start the save")
	}
	msg := cmd().(queueSavedMsg)
	m.handleQueueSaved(msg)
	if saver.created != "Mix" || len(saver.added) != m.playlist.Len() || msg.added != m.playlist.Len() {
		t.Fatalf("created %q with %d tracks, msg %+v", saver.created, len(saver.added), msg)
	}
}

package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

// albumTestProvider knows the album of every track that names one.
type albumTestProvider struct{ commandsTestProvider }

func (albumTestProvider) AlbumForTrack(t playlist.Track) (provider.AlbumInfo, bool) {
	return provider.AlbumInfo{ID: t.Album, Name: t.Album}, t.Album != ""
}

func (albumTestProvider) AlbumTracks(string) ([]playlist.Track, error) { return nil, nil }

func TestRowMenuOffersTheRowActions(t *testing.T) {
	m := keybindingTestModel()
	prov := albumTestProvider{commandsTestProvider{name: "YouTube"}}
	m.providers = []provider.Entry{{Key: "ytmusic", Name: "YouTube", Provider: prov}}
	m.playlist.Add(playlist.Track{Path: "/a.mp3", Title: "Song", Album: "Album"})
	m.focus = focusPlaylist

	m.handleKey(tea.KeyPressMsg{Code: 'O', Text: "O"})
	if !m.keymap.visible || m.keymap.title == "" {
		t.Fatal("O must open the row menu")
	}
	actions := map[string]bool{}
	for _, e := range m.keymap.entries {
		actions[e.action] = true
	}
	for _, want := range []string{"Play selected track", "Toggle queue (play next)", "Go to the album"} {
		if !actions[want] {
			t.Errorf("row menu misses %q: %+v", want, m.keymap.entries)
		}
	}
	if actions["Go to the artist"] {
		t.Error("the row menu must not offer an artist the providers do not know")
	}

	for i, e := range m.keymap.entries {
		if e.action == "Go to the album" {
			m.keymap.cursor = i
		}
	}
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.keymap.visible || !m.navBrowser.visible || m.navBrowser.screen != navBrowseScreenTracks || m.navBrowser.selAlbum.ID != "Album" {
		t.Fatalf("Go to the album: menu %v, browser %v screen %v album %q", m.keymap.visible, m.navBrowser.visible, m.navBrowser.screen, m.navBrowser.selAlbum.ID)
	}
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.navBrowser.visible {
		t.Fatal("Esc must close an album opened from a song")
	}

	// The actions menu that opens next has its own title again.
	m.openKeymap()
	if m.keymap.title != "" {
		t.Fatalf("actions menu title = %q", m.keymap.title)
	}
}

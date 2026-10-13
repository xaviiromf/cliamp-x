package model

import (
	"path/filepath"
	"testing"

	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
)

func TestNotInterestedHidesTheRow(t *testing.T) {
	t.Run("queue track", func(t *testing.T) {
		m := newLayoutTestModel(80, 24)
		m.SetTaste(taste.NewAt(filepath.Join(t.TempDir(), "taste.json")))
		m.plCursor = 3
		path := m.playlist.Tracks()[3].Path
		before := m.playlist.Len()
		m.rateInterest(taste.NotInterested)
		if m.playlist.Len() != before-1 {
			t.Fatalf("queue length %d, want %d", m.playlist.Len(), before-1)
		}
		if !m.taste.Hidden([]string{path}) {
			t.Fatal("the song is not hidden")
		}
	})
	t.Run("browser album", func(t *testing.T) {
		m := newLayoutTestModel(80, 24)
		m.SetTaste(taste.NewAt(filepath.Join(t.TempDir(), "taste.json")))
		m.navBrowser = navBrowserState{visible: true, mode: navBrowseModeByAlbum, screen: navBrowseScreenList, cursor: 1,
			albums: []provider.AlbumInfo{{ID: "a1", Name: "One"}, {ID: "a2", Name: "♥ Two"}, {ID: "a3", Name: "Three"}}}
		m.rateInterest(taste.NotInterested)
		if len(m.navBrowser.albums) != 2 || m.navBrowser.albums[1].ID != "a3" || m.navBrowser.cursor != 1 {
			t.Fatalf("albums = %+v, cursor %d", m.navBrowser.albums, m.navBrowser.cursor)
		}
		if m.taste.Interest("a2") != taste.NotInterested {
			t.Fatal("rating not stored")
		}
	})
	t.Run("interested keeps the row", func(t *testing.T) {
		m := newLayoutTestModel(80, 24)
		m.SetTaste(taste.NewAt(filepath.Join(t.TempDir(), "taste.json")))
		m.navBrowser = navBrowserState{visible: true, mode: navBrowseModeByArtist, screen: navBrowseScreenList,
			artists: []provider.ArtistInfo{{ID: "UC1", Name: "Band"}}}
		m.rateInterest(taste.Interested)
		if len(m.navBrowser.artists) != 1 || m.taste.Interest("UC1") != taste.Interested {
			t.Fatal("interest changed the list or was not stored")
		}
	})
}

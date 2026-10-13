package ytmusic

import (
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/playlist"
)

const emptyBrowse = `{"contents":{"singleColumnBrowseResultsRenderer":{"tabs":[{"tabRenderer":{"content":{"sectionListRenderer":{"contents":[]}}}}]}}}`

func TestToggleFavoriteSavesAlbumsAndSubscribes(t *testing.T) {
	t.Setenv("CLIAMP_CONFIG_DIR", t.TempDir())
	var calls []string
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any, string) string{
		"browse": func(body map[string]any, _ string) string {
			switch body["browseId"] {
			case "MPREb_x":
				return `{"header":{"musicResponsiveHeaderRenderer":{"title":{"runs":[{"text":"Record"}]}}},"microformat":{"audioPlaylistId":"OLAK5uy_x"}}`
			case "UCband":
				return `{"header":{"musicImmersiveHeaderRenderer":{"title":{"runs":[{"text":"Band"}]}}}}`
			}
			return emptyBrowse
		},
		"like/like": func(body map[string]any, _ string) string {
			calls = append(calls, "save "+str(body, "target", "playlistId"))
			return `{}`
		},
		"like/removelike": func(body map[string]any, _ string) string {
			calls = append(calls, "unsave "+str(body, "target", "playlistId"))
			return `{}`
		},
		"subscription/subscribe": func(body map[string]any, _ string) string {
			calls = append(calls, "subscribe "+str(body, "channelIds", 0))
			return `{}`
		},
		"subscription/unsubscribe": func(body map[string]any, _ string) string {
			calls = append(calls, "unsubscribe "+str(body, "channelIds", 0))
			return `{}`
		},
	}}
	p := New(f.client(testAuth()), Options{})

	for _, want := range []bool{true, false} {
		added, name, err := p.ToggleFavorite("MPREb_x")
		if err != nil || added != want || name != "Record" {
			t.Fatalf("album toggle = %v %q %v, want %v", added, name, err, want)
		}
		added, name, err = p.ToggleFavorite("UCband")
		if err != nil || added != want || name != "Band" {
			t.Fatalf("artist toggle = %v %q %v, want %v", added, name, err, want)
		}
	}
	want := "save OLAK5uy_x,subscribe UCband,unsave OLAK5uy_x,unsubscribe UCband"
	if got := strings.Join(calls, ","); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
	if _, _, err := p.ToggleFavorite("local:al:x|y"); err == nil {
		t.Fatal("a local album was favorited in the account")
	}
}

func TestSongTrackAndPlaceholder(t *testing.T) {
	track := songTrack(Song{VideoID: "abcdefghijk", Title: "T", Artists: []Artist{{"A", "UCa"}, {"B", ""}}, Album: "R", AlbumID: "MPREb_r", Duration: 61})
	if track.Path != WatchURL("abcdefghijk") || track.Artist != "A, B" || track.DurationSecs != 61 || !track.Stream ||
		track.Meta(MetaVideoID) != "abcdefghijk" || track.Meta(MetaArtistID) != "UCa" || track.Meta(MetaAlbumID) != "MPREb_r" {
		t.Fatalf("track = %+v", track)
	}
	ph := placeholder("MPREb_r", "R", "A", "Albums")
	if !ph.IsAlbum() || ph.AlbumID() != "MPREb_r" || ph.Meta(playlist.MetaSection) != "Albums" {
		t.Fatalf("placeholder = %+v", ph)
	}
	if got := trackVideoID(playlist.Track{Path: "https://www.youtube.com/watch?v=abcdefghijk"}); got != "abcdefghijk" {
		t.Fatalf("trackVideoID = %q", got)
	}
}

func TestProviderSignedIn(t *testing.T) {
	for _, tc := range []struct {
		name string
		auth *Auth
		want bool
	}{
		{name: "cookies with a sign-in", auth: testAuth(), want: true},
		{name: "no cookies", auth: &Auth{}},
		{name: "no auth"},
	} {
		if got := New(&Client{auth: tc.auth}, Options{}).SignedIn(); got != tc.want {
			t.Errorf("%s: SignedIn() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

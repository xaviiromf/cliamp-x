package ytmusic

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/taste"
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

// The home cards keep their kind and ID, drop what the taste store hides
// and the liked-songs playlist, and leave out the shelves they empty.
func TestHomeShelves(t *testing.T) {
	t.Setenv("CLIAMP_CONFIG_DIR", t.TempDir())
	song := &Song{VideoID: "abcdefghijk", Title: "Tune", Artists: []Artist{{"Band", "UCband"}}}
	p := New(&Client{auth: &Auth{}}, Options{Taste: taste.NewAt(filepath.Join(t.TempDir(), "taste.json"))})
	p.home, p.homeAt = []Shelf{
		{Title: "Quick picks", Items: []Item{
			{Kind: KindSong, ID: song.VideoID, Title: "Tune", Artist: "Band", Thumbnail: "https://lh3.googleusercontent.com/t", Song: song},
			{Kind: KindAlbum, ID: "MPREb_a", Title: "Record", Subtitle: "Album • Band", Artist: "Band"},
			{Kind: KindPlaylist, ID: "VLLM", Title: "Liked music"},
			{Kind: KindPlaylist, ID: "VLPLx", Title: "Mix"},
			{Kind: KindArtist, ID: "UCband", Title: "Band", Artist: "Band", ArtistID: "UCband"},
		}},
		{Title: "Hidden", Items: []Item{{Kind: KindAlbum, ID: "MPREb_r", Title: "Reggaeton hits"}}},
	}, time.Now()

	shelves, err := p.HomeShelves()
	if err != nil {
		t.Fatal(err)
	}
	if len(shelves) != 1 || shelves[0].Title != "Quick picks" {
		t.Fatalf("shelves = %+v, want only Quick picks", shelves)
	}
	var got []string
	for _, it := range shelves[0].Items {
		got = append(got, it.Kind+" "+it.ID+" "+it.Subtitle)
	}
	want := []string{"song abcdefghijk Band", "album MPREb_a Album • Band", "playlist pl:VLPLx ", "artist UCband Band"}
	if !slices.Equal(got, want) {
		t.Fatalf("items = %q, want %q", got, want)
	}
	if track := shelves[0].Items[0].Track; track == nil || track.Path != WatchURL(song.VideoID) || shelves[0].Items[0].ImageURL != "https://lh3.googleusercontent.com/t" {
		t.Fatalf("song card = %+v", shelves[0].Items[0])
	}
}

// A home that never loaded reports why, so a client can tell offline from
// empty.
func TestHomeShelvesReportsTheFailure(t *testing.T) {
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any, string) string{}}
	if _, err := New(f.client(&Auth{}), Options{}).HomeShelves(); err == nil {
		t.Fatal("a failed home read gave no error")
	}
}

func TestFavoriteState(t *testing.T) {
	p := New(&Client{auth: testAuth()}, Options{})
	p.savedAlbums = map[string]bool{"MPREb_saved": true}
	p.subscribed = map[string]bool{"UCfollowed": true}
	signedOut := New(&Client{auth: &Auth{}}, Options{})
	for _, tc := range []struct {
		name      string
		p         *Provider
		id        string
		on, known bool
	}{
		{name: "saved album", p: p, id: "MPREb_saved", on: true, known: true},
		{name: "other album", p: p, id: "MPREb_other", known: true},
		{name: "followed artist", p: p, id: "UCfollowed", on: true, known: true},
		{name: "playlist", p: p, id: "pl:VLPLx"},
		{name: "signed out", p: signedOut, id: "MPREb_saved"},
	} {
		if on, known := tc.p.FavoriteState(tc.id); on != tc.on || known != tc.known {
			t.Errorf("%s: FavoriteState(%q) = %v, %v; want %v, %v", tc.name, tc.id, on, known, tc.on, tc.known)
		}
	}
}

// An artist page marks its singles and shows its square picture, not the
// wide banner of its header.
func TestArtistAlbumsSinglesAndPicture(t *testing.T) {
	album := func(id, title string) string {
		return `{"musicTwoRowItemRenderer":{"title":{"runs":[{"text":"` + title + `"}]},"navigationEndpoint":{"browseEndpoint":{"browseId":"` + id +
			`","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_ALBUM"}}}}}}`
	}
	shelf := func(items string) string {
		return `{"musicCarouselShelfRenderer":{"header":{"musicCarouselShelfBasicHeaderRenderer":{"title":{"runs":[{"text":"x"}]}}},"contents":[` + items + `]}}`
	}
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any, string) string{
		"browse": func(map[string]any, string) string {
			return `{"header":{"musicImmersiveHeaderRenderer":{"title":{"runs":[{"text":"Band"}]},
			 "thumbnail":{"musicThumbnailRenderer":{"thumbnail":{"thumbnails":[{"url":"https://lh3.googleusercontent.com/b=w1080-h450-p-l90-rj","width":1080,"height":450}]}}}}},
			 "microformat":{"microformatDataRenderer":{"thumbnail":{"thumbnails":[{"url":"https://lh3.googleusercontent.com/b=w544-h544-p-l90-rj","width":544,"height":544}]}}},
			 "contents":{"singleColumnBrowseResultsRenderer":{"tabs":[{"tabRenderer":{"content":{"sectionListRenderer":{"contents":[` +
				shelf(album("MPREb_lp", "LP")) + `,` + shelf(album("MPREb_hit", "Hit")) + `]}}}}]}}}`
		},
	}}
	albums, err := New(f.client(&Auth{}), Options{}).ArtistAlbums("UCband")
	if err != nil {
		t.Fatal(err)
	}
	if len(albums) != 3 || albums[0].ImageURL != "https://lh3.googleusercontent.com/b=w400-h400-p-l90-rj" {
		t.Fatalf("albums = %+v, want the top songs with the square picture, then 2 releases", albums)
	}
	if albums[1].ID != "MPREb_lp" || albums[1].Single || albums[2].ID != "MPREb_hit" || !albums[2].Single {
		t.Fatalf("releases = %+v, want the LP then the single", albums[1:])
	}
}

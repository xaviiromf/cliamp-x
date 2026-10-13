package ytmusic

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
)

// The genre list drops the blocked genres and the ones rated "not
// interested", and ends with the extra genres.
func TestGenresFollowTaste(t *testing.T) {
	t.Setenv("CLIAMP_CONFIG_DIR", t.TempDir())
	store := taste.NewAt(filepath.Join(t.TempDir(), "taste.json"))
	if _, err := store.Rate(prefixGenre+"rock", taste.Item{Kind: taste.Genre, Name: "Rock"}, taste.NotInterested); err != nil {
		t.Fatal(err)
	}
	p := New(&Client{auth: testAuth()}, Options{Taste: store})
	p.cats = []Category{
		{Title: "Urbano", Section: "Géneros", Params: "urb"},
		{Title: "Rock", Section: "Géneros", Params: "rock"},
		{Title: "Jazz", Section: "Géneros", Params: "jazz"},
	}
	p.catsAt = time.Now()

	genres, err := p.Genres()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, g := range genres {
		names = append(names, g.Name)
	}
	if slices.Contains(names, "Urbano") || slices.Contains(names, "Rock") || !slices.Contains(names, "Jazz") {
		t.Fatalf("genres = %v, want Jazz without Urbano and Rock", names)
	}
	if !slices.ContainsFunc(genres, func(g provider.GenreInfo) bool { return g.ID == prefixSearchGenre+"Shoegaze" }) {
		t.Fatal("the extra genres are missing")
	}
}

func TestDislikeTrackSendsDislike(t *testing.T) {
	var got string
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any, string) string{
		"like/dislike": func(body map[string]any, _ string) string {
			got = str(body, "target", "videoId")
			return `{}`
		},
	}}
	p := New(f.client(testAuth()), Options{})
	if err := p.DislikeTrack(context.Background(), playlist.Track{Path: WatchURL("abcdefghijk")}); err != nil {
		t.Fatal(err)
	}
	if got != "abcdefghijk" {
		t.Fatalf("disliked %q", got)
	}
}

func TestPageOf(t *testing.T) {
	s := []int{1, 2, 3, 4, 5}
	for _, tt := range []struct {
		offset, size int
		want         []int
	}{{0, 2, []int{1, 2}}, {4, 2, []int{5}}, {5, 2, nil}, {1, 0, []int{2, 3, 4, 5}}} {
		if got := pageOf(s, tt.offset, tt.size); !slices.Equal(got, tt.want) {
			t.Errorf("pageOf(%d, %d) = %v, want %v", tt.offset, tt.size, got, tt.want)
		}
	}
}

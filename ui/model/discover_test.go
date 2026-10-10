package model

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/xaviiromf/cliamp-x/playlist"
)

func TestCleanSongTitle(t *testing.T) {
	for in, want := range map[string]string{
		"Despacito (feat. Daddy Yankee)":       "Despacito",
		"Sweater Weather [Official Video]":     "Sweater Weather",
		"Blinding Lights - Remastered 2020":    "Blinding Lights",
		"Canción feat. Alguien":                "Canción",
		"  Tití Me Preguntó  ":                 "Tití Me Preguntó",
		"Song (Slowed + Reverb) [Lyrics] - TV": "Song",
	} {
		if got := cleanSongTitle(in); got != want {
			t.Errorf("cleanSongTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsVersionOf(t *testing.T) {
	orig := playlist.Track{Title: "Tití Me Preguntó", Artist: "Bad Bunny", DurationSecs: 240}
	words := significantWords(cleanSongTitle(orig.Title))
	tests := []struct {
		t    playlist.Track
		want bool
	}{
		{playlist.Track{Title: "Titi Me Pregunto (Slowed + Reverb)", DurationSecs: 300}, true},
		{playlist.Track{Title: "TITÍ ME PREGUNTÓ - Remix", Artist: "DJ X", DurationSecs: 200}, true},
		{playlist.Track{Title: "Me Porto Bonito", DurationSecs: 180}, false},
		{playlist.Track{Title: "Tití Me Preguntó (10 hour loop)", DurationSecs: 36000}, false},
	}
	for _, tt := range tests {
		if got := isVersionOf(tt.t, orig, words); got != tt.want {
			t.Errorf("isVersionOf(%q) = %v, want %v", tt.t.Title, got, tt.want)
		}
	}
}

func TestClassifyVersion(t *testing.T) {
	v := remixVariants()
	if got := classifyVersion("Song (Slowed + Reverb)", v, 0); v[got].query != "slowed reverb" {
		t.Errorf("slowed version went to %q", v[got].section)
	}
	if got := classifyVersion("Song (Nightcore)", v, 0); v[got].query != "sped up" {
		t.Errorf("nightcore version went to %q, want the sped-up group", v[got].section)
	}
	if got := classifyVersion("Song", v, 4); got != 4 {
		t.Errorf("an unmarked title must stay in the variant that found it, got %d", got)
	}
}

// fakeSearcher answers every query with the same results. The remix
// finder searches in parallel, so it records queries under a lock.
type fakeSearcher struct {
	mu      sync.Mutex
	results []playlist.Track
	queries []string
}

func (f *fakeSearcher) Name() string                                { return "Fake" }
func (f *fakeSearcher) Playlists() ([]playlist.PlaylistInfo, error) { return nil, nil }
func (f *fakeSearcher) Tracks(string) ([]playlist.Track, error)     { return nil, nil }
func (f *fakeSearcher) SearchTracks(_ context.Context, q string, _ int) ([]playlist.Track, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, q)
	return f.results, nil
}

func TestSearchRemixesGroupsAndFilters(t *testing.T) {
	orig := playlist.Track{Path: "yt:orig", Title: "Sweater Weather", Artist: "The Neighbourhood", DurationSecs: 240}
	s := &fakeSearcher{results: []playlist.Track{
		{Path: "yt:orig", Title: "Sweater Weather"},
		{Path: "yt:1", Title: "Sweater Weather (Slowed + Reverb)", DurationSecs: 290},
		{Path: "yt:2", Title: "Sweater Weather Remix", Artist: "DJ", DurationSecs: 210},
		{Path: "yt:3", Title: "Afraid", DurationSecs: 250},
		{Path: "yt:4", Title: "Sweater Weather (Nightcore)", DurationSecs: 190},
		{Path: "", Title: "Sweater Weather", ProviderMeta: map[string]string{playlist.MetaKind: playlist.MetaKindAlbum}},
	}}
	out, err := searchRemixes(context.Background(), s, orig, cleanSongTitle(orig.Title))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, tr := range out {
		got[tr.Path] = tr.Meta(playlist.MetaSection)
	}
	if len(out) != 3 || got["yt:1"] != "Slowed + reverb" || got["yt:4"] != "Sped up" || got["yt:2"] == "" {
		t.Fatalf("results = %v", got)
	}
	sort.Strings(s.queries)
	if len(s.queries) != len(remixVariants()) || !strings.HasPrefix(s.queries[0], "Sweater Weather ") {
		t.Fatalf("queries = %q", s.queries)
	}
	// Grouped results keep the sections of the overlay contiguous.
	for i := 1; i < len(out); i++ {
		if out[i].Meta(playlist.MetaSection) != out[i-1].Meta(playlist.MetaSection) {
			for j := i + 1; j < len(out); j++ {
				if out[j].Meta(playlist.MetaSection) == out[i-1].Meta(playlist.MetaSection) {
					t.Fatal("results of one section must be contiguous")
				}
			}
		}
	}
}

func TestHandleMoreLikeThisReplacesQueue(t *testing.T) {
	pl := playlist.New()
	pl.Add(playlist.Track{Path: "/old/1.mp3"}, playlist.Track{Path: "/old/2.mp3"})
	eng := &playbackFakeEngine{}
	m := &Model{player: eng, playlist: pl}
	m.requests.discover = 3
	radio := []playlist.Track{{Path: "/new/seed.mp3", Title: "Seed"}, {Path: "/new/a.mp3"}, {Path: "/new/b.mp3"}}
	if cmd := m.handleMoreLikeThis(moreLikeThisMsg{label: "Seed", tracks: radio, gen: 2}); cmd != nil || pl.Len() != 2 {
		t.Fatal("a stale radio must be dropped")
	}
	m.handleMoreLikeThis(moreLikeThisMsg{label: "Seed", tracks: radio, gen: 3})
	if pl.Len() != 3 {
		t.Fatalf("queue length = %d, want the radio", pl.Len())
	}
	if cur, idx := pl.Current(); idx != 0 || cur.Path != "/new/seed.mp3" {
		t.Fatalf("current = %v %d, want the seed first", cur.Path, idx)
	}
	if !strings.Contains(m.status.text, "Seed") {
		t.Fatalf("status = %q", m.status.text)
	}
}

func TestSearchRemixesShortTitleNeedsArtist(t *testing.T) {
	orig := playlist.Track{Path: "yt:orig", Title: "Arsenal", Artist: "Slipknot", DurationSecs: 222}
	s := &fakeSearcher{results: []playlist.Track{
		{Path: "yt:1", Title: "ARSENAL SONG (New Football Anthem)", Artist: "Samoa Wave", DurationSecs: 210},
		{Path: "yt:2", Title: "Arsenal (Slowed)", Artist: "Slipknot", DurationSecs: 260},
		{Path: "yt:3", Title: "Slipknot - Arsenal (DJ Remix)", Artist: "DJ", DurationSecs: 200},
	}}
	out, err := searchRemixes(context.Background(), s, orig, "Arsenal")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].Path == "yt:1" || out[1].Path == "yt:1" {
		t.Fatalf("results = %v, want only versions credited to Slipknot", out)
	}
	if !strings.HasPrefix(s.queries[0], "Slipknot Arsenal ") {
		t.Fatalf("query = %q, want the artist in a short-title search", s.queries[0])
	}
}

func TestMainArtist(t *testing.T) {
	for in, want := range map[string]string{"Bad Bunny, Chencho": "Bad Bunny", "A & B": "A", "Solo": "Solo", "X feat. Y": "X"} {
		if got := mainArtist(in); got != want {
			t.Errorf("mainArtist(%q) = %q, want %q", in, got, want)
		}
	}
}

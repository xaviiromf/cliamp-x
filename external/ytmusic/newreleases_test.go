package ytmusic

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestIsNewYear(t *testing.T) {
	jan := time.Date(2026, time.January, 20, 0, 0, 0, 0, time.UTC)
	oct := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	if !isNewYear(2026, oct) || isNewYear(2025, oct) {
		t.Fatal("in October only this year's releases are new")
	}
	if !isNewYear(2025, jan) || isNewYear(2024, jan) {
		t.Fatal("in January last year's releases are still new")
	}
}

func TestCollectReleases(t *testing.T) {
	now := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC)
	pages := map[string]ArtistPage{
		"UC1": {Name: "Alpha", Singles: []Item{
			{Kind: KindAlbum, ID: "MPREb_a1", Title: "New single", Year: 2026},
			{Kind: KindAlbum, ID: "MPREb_a2", Title: "Old single", Year: 2023},
		}, Albums: []Item{{Kind: KindAlbum, ID: "MPREb_a3", Title: "New album", Year: 2026}}},
		"UC2": {Name: "Beta", Singles: []Item{{Kind: KindAlbum, ID: "MPREb_b1", Title: "Beta single", Year: 2026}}},
	}
	artist := func(_ context.Context, id string) (ArtistPage, error) {
		if p, ok := pages[id]; ok {
			return p, nil
		}
		return ArtistPage{}, errors.New("gone")
	}
	got := collectReleases(context.Background(), []Item{{ID: "UC1"}, {ID: "UC2"}, {ID: "UC3"}}, now, artist)
	var ids []string
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	// Newest on each shelf first; the 2023 single is left out.
	if strings.Join(ids, ",") != "MPREb_a1,MPREb_a3,MPREb_b1" && strings.Join(ids, ",") != "MPREb_a1,MPREb_b1,MPREb_a3" {
		t.Fatalf("releases = %v", ids)
	}
	if !got[0].Single || got[0].Artist != "Alpha" {
		t.Fatalf("first release = %+v", got[0])
	}
}

func TestReleaseRowsMarkUnseen(t *testing.T) {
	p := &Provider{client: &Client{auth: &Auth{}}, words: spanish}
	p.releases.loaded = true
	p.releases.data = releasesFile{Fetched: time.Now(), Releases: []release{
		{ID: "MPREb_x", Title: "X", Artist: "A", Single: true},
		{ID: "MPREb_y", Title: "Y", Artist: "B"},
	}, Seen: map[string]bool{"MPREb_y": true}}
	// Signed out there are no rows.
	if rows := p.releaseRows(); len(rows) != 0 {
		t.Fatalf("signed-out rows = %d", len(rows))
	}
	p.markReleaseSeen("MPREb_x")
	if !p.releases.data.Seen["MPREb_x"] {
		t.Fatal("opening a release must mark it seen")
	}
}

package ytmusic

import (
	"context"
	"errors"
	"testing"

	"github.com/xaviiromf/cliamp-x/lyrics"
	"github.com/xaviiromf/cliamp-x/playlist"
)

func TestTrackLyrics(t *testing.T) {
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any, string) string{
		"next": func(body map[string]any, _ string) string {
			if body["videoId"] != "abcdefghijk" {
				return `{}`
			}
			return `{"contents":{"singleColumnMusicWatchNextResultsRenderer":{"tabbedRenderer":{"watchNextTabbedResultsRenderer":{"tabs":[
				{"tabRenderer":{"endpoint":{"browseEndpoint":{"browseId":"MPLYt_x","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_TRACK_LYRICS"}}}}}}]}}}}}`
		},
		"browse": func(map[string]any, string) string {
			return `{"contents":{"sectionListRenderer":{"contents":[{"musicDescriptionShelfRenderer":{"description":{"runs":[{"text":"line one\nline two"}]},"footer":{"runs":[{"text":"Source: Musixmatch"}]}}}]}}}`
		},
	}}
	p := New(f.client(&Auth{}), Options{})
	lines, err := p.TrackLyrics(context.Background(), playlist.Track{Path: "https://music.youtube.com/watch?v=abcdefghijk"})
	if err != nil || len(lines) != 2 || lines[1].Text != "line two" || lyrics.Synced(lines) {
		t.Fatalf("TrackLyrics = %+v, %v", lines, err)
	}
	if _, err := p.TrackLyrics(context.Background(), playlist.Track{Path: "/music/local.flac"}); !errors.Is(err, lyrics.ErrNotFound) {
		t.Fatalf("a local file must have no YouTube lyrics, got %v", err)
	}
	if _, err := p.TrackLyrics(context.Background(), playlist.Track{Path: "https://music.youtube.com/watch?v=zzzzzzzzzzz"}); !errors.Is(err, lyrics.ErrNotFound) {
		t.Fatalf("a song without lyrics must report ErrNotFound, got %v", err)
	}
}

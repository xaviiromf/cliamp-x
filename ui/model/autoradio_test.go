package model

import (
	"context"
	"testing"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

// relaterProvider suggests fixed related songs for every track.
type relaterProvider struct {
	commandsTestProvider
	related []playlist.Track
	seeds   *[]string
}

func (p relaterProvider) CanRelate(t playlist.Track) bool { return t.Path != "" }

func (p relaterProvider) RelatedTracks(_ context.Context, seed playlist.Track, n int) ([]playlist.Track, error) {
	*p.seeds = append(*p.seeds, seed.Path)
	return p.related, nil
}

var _ provider.Relater = relaterProvider{}

func TestAutoRadioContinuesAfterTheQueue(t *testing.T) {
	var seeds []string
	rel := relaterProvider{
		commandsTestProvider: commandsTestProvider{name: "YouTube"},
		related:              []playlist.Track{{Path: "song-a"}, {Path: "last"}, {Path: "song-b"}},
		seeds:                &seeds,
	}
	m := keybindingTestModel()
	m.providers = append(m.providers, provider.Entry{Key: "yt", Name: "YouTube", Provider: rel})
	m.playlist.Replace([]playlist.Track{{Path: "first"}, {Path: "last"}})
	m.playlist.SetIndex(1)

	// Off: the queue just ends.
	if cmd := m.nextTrack(); cmd != nil {
		t.Fatal("auto radio off still looked up related songs")
	}

	m.SetAutoRadio(true)
	m.playlist.SetIndex(1)
	cmd := m.nextTrack()
	if cmd == nil {
		t.Fatal("no related-songs lookup at the end of the queue")
	}
	msg, ok := cmd().(autoRadioLoadedMsg)
	if !ok {
		t.Fatal("the lookup did not return an autoRadioLoadedMsg")
	}
	if len(seeds) == 0 || seeds[0] != "last" {
		t.Fatalf("seeds = %v, want the last track", seeds)
	}
	m.handleAutoRadioLoaded(msg)
	// "last" is already queued, so only the two new songs are added.
	if m.playlist.Len() != 4 {
		t.Fatalf("playlist has %d tracks, want 4", m.playlist.Len())
	}
	if cur, idx := m.playlist.Current(); idx != 2 || cur.Path != "song-a" {
		t.Fatalf("current = %q at %d, want song-a at 2", cur.Path, idx)
	}

	// A stale answer is dropped.
	m.requests.autoRadio++
	before := m.playlist.Len()
	m.handleAutoRadioLoaded(msg)
	if m.playlist.Len() != before {
		t.Fatal("a stale auto radio answer changed the queue")
	}
}

func TestToggleAutoRadio(t *testing.T) {
	m := keybindingTestModel()
	m.toggleAutoRadio()
	if !m.AutoRadio() {
		t.Fatal("toggle did not turn the auto radio on")
	}
	m.toggleAutoRadio()
	if m.AutoRadio() {
		t.Fatal("toggle did not turn the auto radio off")
	}
}

package model

import (
	"strings"
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/sponsorblock"
	"github.com/xaviiromf/cliamp-x/player"
	"github.com/xaviiromf/cliamp-x/playlist"
)

const sponsorTestURL = "https://music.youtube.com/watch?v=dQw4w9WgXcQ"

func newSponsorModel(t *testing.T, segs []sponsorblock.Segment, pos, dur time.Duration) (*Model, *playbackFakeEngine) {
	t.Helper()
	pl := playlist.New()
	pl.Add(playlist.Track{Path: sponsorTestURL, Title: "Song", DurationSecs: int(dur.Seconds())},
		playlist.Track{Path: "https://music.youtube.com/watch?v=aaaaaaaaaaa", Title: "Next", DurationSecs: 180})
	pl.SetIndex(0)
	eng := &playbackFakeEngine{playing: true, seekable: true, position: pos, duration: dur}
	m := &Model{player: eng, playlist: pl}
	client := sponsorblock.New(nil)
	client.SetCached("dQw4w9WgXcQ", segs)
	m.SetSponsorBlock(client)
	m.cachedPos, m.cachedDur = pos, dur
	return m, eng
}

func TestSponsorSkipsSegment(t *testing.T) {
	m, _ := newSponsorModel(t, []sponsorblock.Segment{{Start: 60, End: 75, Category: "sponsor"}}, 62*time.Second, 200*time.Second)
	m.tickSponsor()
	if !m.sponsor.skipped[sponsorTestURL+"|60.0"] {
		t.Fatal("the segment the playback entered must be skipped")
	}
	if !strings.Contains(m.status.text, "SponsorBlock") {
		t.Fatalf("status = %q, want a SponsorBlock notice", m.status.text)
	}
	// A segment is skipped once per play: seeking back into it plays it.
	m.status.Clear()
	m.tickSponsor()
	if m.status.text != "" {
		t.Fatal("a skipped segment must not be skipped again")
	}
}

func TestSponsorIgnoresOutsideSegments(t *testing.T) {
	m, eng := newSponsorModel(t, []sponsorblock.Segment{{Start: 60, End: 75, Category: "sponsor"}}, 30*time.Second, 200*time.Second)
	m.tickSponsor()
	if len(m.sponsor.skipped) != 0 || len(eng.seekCalls) != 0 {
		t.Fatal("no skip may happen outside a segment")
	}
	// Paused playback never skips.
	eng.paused = true
	m.cachedPos = 65 * time.Second
	m.tickSponsor()
	if len(m.sponsor.skipped) != 0 {
		t.Fatal("no skip may happen while paused")
	}
}

func TestSponsorOutroMovesToNextSong(t *testing.T) {
	m, _ := newSponsorModel(t, []sponsorblock.Segment{{Start: 190, End: 199.5, Category: "outro"}}, 191*time.Second, 200*time.Second)
	m.tickSponsor()
	if _, idx := m.playlist.Current(); idx != 1 {
		t.Fatalf("current index = %d, want the next song after an outro that runs to the end", idx)
	}
}

func TestSponsorStartOffset(t *testing.T) {
	m, _ := newSponsorModel(t, []sponsorblock.Segment{{Start: 0, End: 12.5, Category: "intro"}}, 0, 200*time.Second)
	track, _ := m.playlist.Current()
	if got := m.sponsorStartOffset(track); got != 12500*time.Millisecond {
		t.Fatalf("start offset = %v, want 12.5s", got)
	}
	if !m.sponsor.skipped[sponsorTestURL+"|0.0"] {
		t.Fatal("an intro skipped at start must count as skipped")
	}
	// A whole song marked as one segment is played, not skipped.
	whole, _ := newSponsorModel(t, []sponsorblock.Segment{{Start: 0, End: 200, Category: "music_offtopic"}}, 0, 200*time.Second)
	track, _ = whole.playlist.Current()
	if got := whole.sponsorStartOffset(track); got != 0 {
		t.Fatalf("whole-song segment offset = %v, want 0", got)
	}
}

func TestSameAlbum(t *testing.T) {
	a := playlist.Track{Album: "OK Computer", Artist: "Radiohead"}
	tests := []struct {
		b    playlist.Track
		want bool
	}{
		{playlist.Track{Album: "OK Computer", Artist: "Radiohead"}, true},
		{playlist.Track{Album: "Kid A", Artist: "Radiohead"}, false},
		{playlist.Track{Album: "OK Computer", Artist: "Someone"}, false},
		{playlist.Track{}, false},
	}
	for _, tt := range tests {
		if got := sameAlbum(a, tt.b); got != tt.want {
			t.Errorf("sameAlbum(%v, %v) = %v, want %v", a, tt.b, got, tt.want)
		}
	}
	if sameAlbum(playlist.Track{}, playlist.Track{}) {
		t.Fatal("two songs without an album are not the same album")
	}
}

type fadeEngine struct {
	playbackFakeEngine
	crossfade time.Duration
	mode      player.NormalizeMode
	target    float64
}

func (f *fadeEngine) SetCrossfade(d time.Duration) { f.crossfade = d }
func (f *fadeEngine) SetNormalize(m player.NormalizeMode, t float64) {
	f.mode, f.target = m, t
}
func (f *fadeEngine) Normalize() (player.NormalizeMode, float64) { return f.mode, f.target }

func TestArmCrossfadeSkipsSameAlbum(t *testing.T) {
	pl := playlist.New()
	pl.Add(playlist.Track{Path: "/a/1.flac", Album: "Live", Artist: "X"},
		playlist.Track{Path: "/a/2.flac", Album: "Live", Artist: "X"},
		playlist.Track{Path: "/b/1.flac", Album: "Other", Artist: "Y"})
	pl.SetIndex(0)
	eng := &fadeEngine{}
	m := &Model{player: eng, playlist: pl}
	m.SetCrossfade(6)
	m.armCrossfade(playlist.Track{Path: "/a/2.flac", Album: "Live", Artist: "X"})
	if eng.crossfade != 0 {
		t.Fatalf("same-album crossfade = %v, want 0", eng.crossfade)
	}
	m.armCrossfade(playlist.Track{Path: "/b/1.flac", Album: "Other", Artist: "Y"})
	if eng.crossfade != 6*time.Second {
		t.Fatalf("crossfade = %v, want 6s", eng.crossfade)
	}
}

func TestCycleNormalize(t *testing.T) {
	eng := &fadeEngine{mode: player.NormalizeTrack, target: -14}
	m := &Model{player: eng}
	for _, want := range []player.NormalizeMode{player.NormalizeAlbum, player.NormalizeOff, player.NormalizeTrack} {
		m.cycleNormalize()
		if eng.mode != want || eng.target != -14 {
			t.Fatalf("mode = %v target %v, want %v at -14", eng.mode, eng.target, want)
		}
	}
	if !strings.Contains(m.status.text, "ormaliz") {
		t.Fatalf("status = %q, want the new mode", m.status.text)
	}
}

package model

import (
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/internal/resume"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// Quitting keeps the live playlist, the song that played, its position and
// the play-next queue; a start with nothing to play puts them back.
func TestSessionSurvivesARestart(t *testing.T) {
	yt := "https://music.youtube.com/watch?v=aaaaaaaaaaa"
	tracks := []playlist.Track{
		{Path: "/music/a.mp3", Title: "A"},
		{Path: yt, Title: "YouTube song", Stream: true, DurationSecs: 200},
		{Path: "/music/c.mp3", Title: "C"},
	}
	m := keybindingTestModel()
	eng := m.player.(*playbackFakeEngine)
	m.playlist.Add(tracks...)
	m.playlist.SetIndex(1)
	m.playlist.Queue(2)
	m.playlist.Queue(0)
	eng.playing, eng.position = true, 95e9
	m.setPlaybackTrack(tracks[1])

	m.quit()
	path, secs, _ := m.ResumeState()
	ctx, idx, queue := m.ResumeContext()
	if path != yt || secs != 95 {
		t.Fatalf("resume = %q at %d s, want the YouTube song at 95 s", path, secs)
	}
	if len(ctx) != 3 || idx != 1 || len(queue) != 2 || queue[0] != 2 || queue[1] != 0 {
		t.Fatalf("session = %d tracks, index %d, queue %v", len(ctx), idx, queue)
	}

	next := keybindingTestModel()
	if !next.RestoreSession(resume.State{Path: path, PositionSec: secs, Context: ctx, ContextIndex: idx, Queue: queue}) {
		t.Fatal("RestoreSession did nothing")
	}
	if cur, i := next.playlist.Current(); i != 1 || cur.Path != yt || next.plCursor != 1 {
		t.Fatalf("restored on %q (%d), cursor %d", cur.Path, i, next.plCursor)
	}
	if got := next.playlist.QueueTracks(); len(got) != 2 || got[0].Path != "/music/c.mp3" {
		t.Fatalf("restored queue = %+v", got)
	}
	if next.resume.path != yt || next.resume.secs != 95 {
		t.Fatalf("resume point = %q %d", next.resume.path, next.resume.secs)
	}
	if !strings.Contains(next.status.text, "YouTube song") {
		t.Fatalf("status %q must name the song to continue", next.status.text)
	}
	if next.player.(*playbackFakeEngine).playing {
		t.Fatal("a restored session must not start by itself")
	}

	full := keybindingTestModel()
	full.playlist.Add(playlist.Track{Path: "/music/arg.mp3"})
	if full.RestoreSession(resume.State{Path: path, Context: ctx}) {
		t.Fatal("a start with tracks of its own must not restore the session")
	}
}

func TestStoppedSessionKeepsThePlaylist(t *testing.T) {
	m := keybindingTestModel()
	m.playlist.Add(playlist.Track{Path: "/a.mp3"}, playlist.Track{Path: "/b.mp3"})
	m.playlist.SetIndex(1)
	m.quit()
	path, secs, _ := m.ResumeState()
	ctx, idx, _ := m.ResumeContext()
	if path != "/b.mp3" || secs != 0 || len(ctx) != 2 || idx != 1 {
		t.Fatalf("stopped exit = %q %d s, %d tracks at %d", path, secs, len(ctx), idx)
	}
}

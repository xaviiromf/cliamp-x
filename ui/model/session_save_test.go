package model

import (
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/resume"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// The session must reach the disk while the core runs: the system may kill
// the process, and the exit save then never runs.
func TestTickSessionSave(t *testing.T) {
	a := playlist.Track{Title: "A", Path: "/music/a.flac", DurationSecs: 240}
	b := playlist.Track{Title: "B", Path: "/music/b.flac", DurationSecs: 240}
	t0 := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	type step struct {
		after  time.Duration
		change func(m *Model, e *playbackFakeEngine)
		want   *resume.State // nil: nothing saved at this step
	}
	selectB := func(m *Model, e *playbackFakeEngine) { m.playlist.SetIndex(1); e.position = 5 * time.Second }
	tests := []struct {
		name  string
		steps []step
	}{
		{"first tick saves the song and its position", []step{
			{0, nil, &resume.State{Path: a.Path, PositionSec: 10}},
			{time.Second, nil, nil},
		}},
		{"the position follows every interval", []step{
			{0, nil, &resume.State{Path: a.Path, PositionSec: 10}},
			{sessionSaveInterval - time.Second, nil, nil},
			{sessionSaveInterval, func(_ *Model, e *playbackFakeEngine) { e.position = 50 * time.Second }, &resume.State{Path: a.Path, PositionSec: 50}},
		}},
		{"another song saves at once", []step{
			{0, nil, &resume.State{Path: a.Path, PositionSec: 10}},
			{time.Second, selectB, &resume.State{Path: b.Path, PositionSec: 5, ContextIndex: 1}},
		}},
		{"a pause saves at once, then stays quiet", []step{
			{0, nil, &resume.State{Path: a.Path, PositionSec: 10}},
			{time.Second, func(_ *Model, e *playbackFakeEngine) { e.paused = true }, &resume.State{Path: a.Path, PositionSec: 10}},
			{10 * sessionSaveInterval, nil, nil},
		}},
		{"a track that is still starting waits", []step{
			{0, func(m *Model, _ *playbackFakeEngine) { m.buffering = true }, nil},
			{time.Second, func(m *Model, _ *playbackFakeEngine) { m.buffering = false }, &resume.State{Path: a.Path, PositionSec: 10}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := playlist.New()
			pl.Add(a, b)
			pl.SetIndex(0)
			e := &playbackFakeEngine{playing: true, position: 10 * time.Second}
			m := Model{player: e, playlist: pl}
			var got []resume.State
			m.SetSessionSaver(func(s resume.State) { got = append(got, s) })
			for i, st := range tt.steps {
				if st.change != nil {
					st.change(&m, e)
				}
				got = nil
				m.tickSessionSave(t0.Add(st.after))
				switch {
				case st.want == nil && len(got) != 0:
					t.Fatalf("step %d: saved %+v, want nothing", i, got)
				case st.want != nil && len(got) != 1:
					t.Fatalf("step %d: %d saves, want 1", i, len(got))
				case st.want != nil:
					s := got[0]
					if s.Path != st.want.Path || s.PositionSec != st.want.PositionSec || s.ContextIndex != st.want.ContextIndex || len(s.Context) != 2 {
						t.Fatalf("step %d: saved %+v, want %+v with 2 tracks", i, s, *st.want)
					}
				}
			}
		})
	}
}

// A restored song that has not played keeps its position when the session is saved again.
func TestCaptureExitKeepsRestoredPosition(t *testing.T) {
	a := playlist.Track{Title: "A", Path: "/music/a.flac", DurationSecs: 240}
	pl := playlist.New()
	pl.Add(a)
	pl.SetIndex(0)
	m := Model{player: &playbackFakeEngine{}, playlist: pl}
	m.SetResume(a.Path, 90)
	m.captureExit()
	if path, secs, _ := m.ResumeState(); path != a.Path || secs != 90 {
		t.Fatalf("state = (%q, %d), want (%q, 90)", path, secs, a.Path)
	}
	// Called twice, the play-next queue must not double.
	pl.Queue(0)
	m.captureExit()
	m.captureExit()
	if _, _, q := m.ResumeContext(); len(q) != 1 {
		t.Fatalf("queue = %v, want one entry", q)
	}
}

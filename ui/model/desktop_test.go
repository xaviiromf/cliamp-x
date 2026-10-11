package model

import (
	"sync"
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/playlist"
)

type fakeDesktop struct {
	mu    sync.Mutex
	shown []string
}

func (f *fakeDesktop) Show(summary, body, image string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shown = append(f.shown, summary+"|"+body+"|"+image)
	return nil
}

func (f *fakeDesktop) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.shown)
}

func TestNotifyTrack(t *testing.T) {
	d := &fakeDesktop{}
	m := &Model{}
	m.SetDesktopNotifier(d)
	m.notifyTrack(playlist.Track{Title: "Arsenal", Artist: "Slipknot", Album: "Arsenal", Path: "/a.flac"})
	deadline := time.Now().Add(time.Second)
	for d.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if d.count() != 1 || d.shown[0] != "Arsenal|Slipknot|" {
		t.Fatalf("shown = %v, want the song with its artist (the album repeats the title)", d.shown)
	}
}

func TestNotifyActionNext(t *testing.T) {
	m := newLayoutTestModel(100, 40)
	m.playlist.SetIndex(0)
	m.handleNotifyAction(NotifyActionMsg{Action: "next"})
	if _, idx := m.playlist.Current(); idx != 1 {
		t.Fatalf("index = %d after the Next button, want 1", idx)
	}
}

func TestNotifyActionPauseAndFullScreenSilence(t *testing.T) {
	m := newLayoutTestModel(100, 40)
	eng := m.player.(*playbackFakeEngine)
	eng.playing = true
	m.handleNotifyAction(NotifyActionMsg{Action: "pause"})
	if !eng.paused {
		t.Fatal("the Pause button must pause playback")
	}

	d := &fakeDesktop{}
	m.SetDesktopNotifier(d)
	m.fullVis = true
	m.notifyTrack(playlist.Track{Title: "Song", Path: "/a.flac"})
	time.Sleep(20 * time.Millisecond)
	if d.count() != 0 {
		t.Fatal("no notification over the focused full-screen visualizer")
	}
	m.blurred = true
	m.notifyTrack(playlist.Track{Title: "Song", Path: "/a.flac"})
	deadline := time.Now().Add(time.Second)
	for d.count() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if d.count() != 1 {
		t.Fatal("a terminal without the focus still notifies")
	}
}

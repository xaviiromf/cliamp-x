package model

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/xaviiromf/cliamp-x/playlist"
)

// fadingEngine records the fade levels the sleep timer sets.
type fadingEngine struct {
	*playbackFakeEngine
	fades []float64
}

func (f *fadingEngine) SetFade(level float64) { f.fades = append(f.fades, level) }

func sleepTestModel() (Model, *fadingEngine) {
	eng := &fadingEngine{playbackFakeEngine: &playbackFakeEngine{playing: true}}
	m := keybindingTestModel()
	m.player = eng
	m.playlist.Add(playlist.Track{Path: "/a.mp3", Title: "A"}, playlist.Track{Path: "/b.mp3", Title: "B"})
	m.playlist.SetIndex(0)
	return m, eng
}

func TestSleepTimerCyclesWithT(t *testing.T) {
	m, _ := sleepTestModel()
	want := []string{"15 min", "30 min", "45 min", "60 min", "90 min", "end of this song", "off"}
	for _, w := range want {
		m.handleKey(tea.KeyPressMsg{Code: 'T', Text: "T"})
		if !strings.Contains(m.status.text, w) {
			t.Fatalf("status %q, want it to say %q", m.status.text, w)
		}
	}
	if m.sleep.minutes != 0 {
		t.Fatalf("step = %d after a full cycle, want off", m.sleep.minutes)
	}
}

func TestSleepTimerFadesThenPauses(t *testing.T) {
	m, eng := sleepTestModel()
	m.setSleepMinutes(30)
	now := time.Now()

	m.tickSleep(now.Add(10 * time.Minute))
	if got := eng.fades[len(eng.fades)-1]; got < 1 {
		t.Fatalf("fade %v twenty minutes early, want full volume", got)
	}
	m.tickSleep(m.sleep.at.Add(-sleepFade / 2))
	if got := eng.fades[len(eng.fades)-1]; got < 0.45 || got > 0.55 {
		t.Fatalf("fade %v halfway through the fade-out, want about 0.5", got)
	}
	m.tickSleep(m.sleep.at.Add(time.Second))
	if !eng.paused || m.sleep.minutes != 0 {
		t.Fatalf("paused %v, step %d after the timer ran out", eng.paused, m.sleep.minutes)
	}
	if got := eng.fades[len(eng.fades)-1]; got != 1 {
		t.Fatalf("fade %v after the pause, want the volume restored", got)
	}
	if label := m.sleepLabel(time.Now()); label != "" {
		t.Fatalf("label %q after the pause, want none", label)
	}
}

func TestSleepTimerPausesAtTheEndOfTheSong(t *testing.T) {
	m, eng := sleepTestModel()
	m.setSleepMinutes(-1)
	if m.sleepLabel(time.Now()) != "end" {
		t.Fatalf("label %q, want end", m.sleepLabel(time.Now()))
	}
	m.tickSleep(time.Now())
	if eng.paused {
		t.Fatal("paused during the song")
	}
	m.playlist.SetIndex(1) // the next song starts
	m.tickSleep(time.Now())
	if !eng.paused || m.sleep.minutes != 0 {
		t.Fatal("the next song must start paused")
	}
}

func TestRevealPlayingTrackMovesTheCursor(t *testing.T) {
	m := newLayoutTestModel(80, 24)
	for i := range 200 {
		m.playlist.Add(playlist.Track{Path: "/x" + strings.Repeat("y", i%3) + string(rune('a'+i%26)), Title: "T"})
	}
	m.playlist.SetIndex(150)
	m.player.(*playbackFakeEngine).playing = true
	m.plCursor = 0
	m.adjustScroll()
	m.handleKey(tea.KeyPressMsg{Code: 'c', Text: "c"})
	if m.plCursor != 150 {
		t.Fatalf("cursor = %d, want the playing track 150", m.plCursor)
	}
	if visible := m.effectivePlaylistVisible(); m.plScroll > 150 || m.plScroll+visible <= 150 {
		t.Fatalf("scroll %d with %d rows hides the playing track", m.plScroll, visible)
	}
}

func TestTitleShowsTheModeTagsThatFit(t *testing.T) {
	m := newLayoutTestModel(120, 30)
	m.autoRadio = true
	m.eqAuto = true
	m.setSleepMinutes(25)
	title := ansiStrip(m.renderTitle())
	for _, want := range []string{"RADIO", "AUTO EQ", "SLEEP 25m"} {
		if !strings.Contains(title, want) {
			t.Fatalf("title %q misses %q", title, want)
		}
	}
	narrow := newLayoutTestModel(40, 12)
	narrow.autoRadio, narrow.eqAuto = true, true
	narrow.setSleepMinutes(25)
	if w := textWidth(narrow.renderTitle()); w > narrow.layout.panelWidth {
		t.Fatalf("narrow title is %d wide, more than the panel %d", w, narrow.layout.panelWidth)
	}
}

func ansiStrip(s string) string { return ansi.Strip(s) }

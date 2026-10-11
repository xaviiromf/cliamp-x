package model

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/offline"
	"github.com/xaviiromf/cliamp-x/internal/stats"
	"github.com/xaviiromf/cliamp-x/playlist"
)

func TestQueueDownloadsSkipsWhatIsNotNeeded(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "have.m4a")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := offline.OpenManifest(filepath.Join(dir, "offline.json"))
	_ = manifest.Add("https://music.youtube.com/watch?v=aaaaaaaaaaa", file)
	m := &Model{}
	m.SetOffline(manifest, dir)
	m.downloads.running = true // keep the queue from starting yt-dlp
	jobs := []offline.Job{
		{Track: playlist.Track{Path: "https://music.youtube.com/watch?v=aaaaaaaaaaa"}}, // already downloaded
		{Track: playlist.Track{Path: "/music/local.flac"}},                             // already local
		{Track: playlist.Track{Path: "https://music.youtube.com/watch?v=bbbbbbbbbbb"}},
		{Track: playlist.Track{Path: "https://music.youtube.com/watch?v=bbbbbbbbbbb"}}, // duplicate
	}
	m.queueDownloads("Album", jobs)
	if len(m.downloads.queue) != 1 {
		t.Fatalf("queue = %d, want only the one new song", len(m.downloads.queue))
	}
	if !m.isDownloaded(playlist.Track{Path: "https://www.youtube.com/watch?v=aaaaaaaaaaa"}) {
		t.Fatal("a manifest entry must count as downloaded")
	}
}

func TestHandleDownloadEventCounts(t *testing.T) {
	m := &Model{}
	m.SetOffline(offline.OpenManifest(""), t.TempDir())
	m.downloads.running = true
	m.downloads.current = offline.Job{Track: playlist.Track{Path: "https://youtu.be/ccccccccccc", Title: "Song"}}
	m.downloads.events = make(chan downloadEvent, 1)
	if cmd := m.handleDownloadEvent(downloadEventMsg{ev: downloadEvent{percent: 40}}); cmd == nil || m.downloads.percent != 40 {
		t.Fatal("a progress step must update the percent and keep waiting")
	}
	m.handleDownloadEvent(downloadEventMsg{ev: downloadEvent{final: true, file: "/x.m4a"}})
	if m.downloads.running || !strings.Contains(m.status.text, "1") {
		t.Fatalf("after the last song: running %v, status %q", m.downloads.running, m.status.text)
	}
}

func TestRecordPlayThreshold(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plays.jsonl")
	m := &Model{}
	m.SetStatsLog(stats.Open(path))
	song := playlist.Track{Path: "/a.flac", Title: "A", Artist: "X"}
	m.recordPlay(song, 10*time.Second, 200*time.Second)                               // too short
	m.recordPlay(song, 40*time.Second, 200*time.Second)                               // counts
	m.recordPlay(playlist.Track{Path: "/short.flac"}, 12*time.Second, 20*time.Second) // half of a short song
	deadline := time.Now().Add(2 * time.Second)
	for {
		plays, _ := stats.Open(path).Load()
		if len(plays) == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("plays = %d, want 2", len(plays))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStatsScreen(t *testing.T) {
	m := newLayoutTestModel(120, 50)
	m.SetStatsLog(stats.Open(filepath.Join(t.TempDir(), "plays.jsonl")))
	if cmd := m.openStats(); cmd == nil || !m.stats.visible || m.activeScreen() != screenStats {
		t.Fatal("S must open the statistics screen")
	}
	now := time.Now()
	m.handleStatsLoaded(statsLoadedMsg{plays: []stats.Play{
		{At: now, Key: "a", Title: "Arsenal", Artist: "Slipknot", Secs: 222},
		{At: now.Add(-time.Hour), Key: "a", Title: "Arsenal", Artist: "Slipknot", Secs: 222},
	}})
	m.recomputeLayout()
	body := m.renderStatsBody()
	if !strings.Contains(body, "Slipknot") || !strings.Contains(body, "Arsenal") {
		t.Fatalf("stats body misses the top entries:\n%s", body)
	}
	m.handleStatsKey(tea.KeyPressMsg{Code: tea.KeyRight})
	if m.stats.period != stats.Month {
		t.Fatalf("period = %v, want month after Right", m.stats.period)
	}
	m.handleStatsKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.stats.visible {
		t.Fatal("Esc must close the statistics screen")
	}
}

func TestActivityChart(t *testing.T) {
	rows := activityChart([]float64{0, 2, 4, 8}, 2, 12)
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2", len(rows))
	}
	// 3 cells a day: two bar cells and a gap. The peak fills both rows,
	// half of it fills the bottom row only, a day with nothing stays empty.
	if rows[0] != "         ██ " || rows[1] != "   ▄▄ ██ ██ " {
		t.Fatalf("chart = %q / %q", rows[0], rows[1])
	}
}

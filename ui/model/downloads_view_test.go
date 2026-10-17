package model

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/offline"
	"github.com/xaviiromf/cliamp-x/playlist"
)

func TestDownloadsScreenRetriesFailedSongs(t *testing.T) {
	m := newLayoutTestModel(100, 30)
	m.SetOffline(offline.OpenManifest(filepath.Join(t.TempDir(), "offline.json")), t.TempDir())
	song := playlist.Track{Path: "https://music.youtube.com/watch?v=aaaaaaaaaaa", Title: "Song", Stream: true}

	// A running song that fails is kept for a retry.
	m.downloads.running = true
	m.downloads.current = offline.Job{Track: song}
	m.downloads.events = make(chan downloadEvent, 1)
	m.handleDownloadEvent(downloadEventMsg{ev: downloadEvent{final: true, err: errString("HTTP Error 403")}})
	if len(m.downloads.failedJobs) != 1 {
		t.Fatalf("%d failed jobs, want 1", len(m.downloads.failedJobs))
	}

	m.handleKey(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	if m.activeScreen() != screenDownloads {
		t.Fatal("Ctrl+O must open the downloads screen")
	}
	if body := m.renderDownloadsBody(); !strings.Contains(body, "Song") || !strings.Contains(body, "403") {
		t.Fatalf("downloads body misses the failed song and why:\n%s", body)
	}

	m.downloads.running = true // keep the retry from starting yt-dlp
	m.handleKey(tea.KeyPressMsg{Code: 'r', Text: "r"})
	if len(m.downloads.failedJobs) != 0 || len(m.downloads.queue) != 1 {
		t.Fatalf("after r: %d failed, %d queued; want the song queued again", len(m.downloads.failedJobs), len(m.downloads.queue))
	}

	// x cancels: the song stopped by the cancel is not a failure.
	m.downloads.cancel = func() {}
	m.handleKey(tea.KeyPressMsg{Code: 'x', Text: "x"})
	m.handleDownloadEvent(downloadEventMsg{ev: downloadEvent{final: true, err: context.Canceled}})
	if len(m.downloads.queue) != 0 || len(m.downloads.failedJobs) != 0 || m.downloads.running {
		t.Fatalf("after x: queue %d, failed %d, running %v", len(m.downloads.queue), len(m.downloads.failedJobs), m.downloads.running)
	}
	m.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.downloads.visible {
		t.Fatal("Esc must close the downloads screen")
	}
}

// A finished batch stays readable in download.status (the app shows it in
// Descargas) until the next batch starts.
func TestDownloadsKeepFinishedCount(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		done, fail int
	}{
		{"downloaded", nil, 1, 0},
		{"failed", errString("HTTP Error 403"), 0, 1},
		{"canceled", context.Canceled, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newLayoutTestModel(100, 30)
			m.SetOffline(offline.OpenManifest(filepath.Join(t.TempDir(), "offline.json")), t.TempDir())
			m.downloads.running = true
			m.downloads.current = offline.Job{Track: playlist.Track{Path: "https://music.youtube.com/watch?v=aaaaaaaaaaa", Title: "Song"}}
			m.downloads.events = make(chan downloadEvent, 1)
			m.handleDownloadEvent(downloadEventMsg{ev: downloadEvent{final: true, file: "/x.m4a", err: tt.err}})
			info := m.downloadsInfo()
			if info.Running || info.Done != tt.done || len(info.Failed) != tt.fail {
				t.Fatalf("status = running %v, done %d, failed %d; want done %d, failed %d", info.Running, info.Done, len(info.Failed), tt.done, tt.fail)
			}
		})
	}
}

type errString string

func (e errString) Error() string { return string(e) }

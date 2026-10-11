package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/ui"
)

// BenchmarkView measures one frame of the player screen with a long playlist,
// the cost paid on every visualizer tick.
func BenchmarkView(b *testing.B) {
	m := newLayoutTestModel(160, 50)
	m.playlist = playlist.New()
	for i := range 500 {
		m.playlist.Add(playlist.Track{Path: fmt.Sprintf("/tmp/b-%d.mp3", i), Title: fmt.Sprintf("Canción número %d · versión acústica", i), Artist: "Artista", Album: fmt.Sprintf("Album %d", i/10)})
	}
	m.recomputeLayout()
	b.ReportAllocs()
	for b.Loop() {
		_ = m.View()
	}
}

// TestRenderFrameMatchesLipgloss proves the fast frame path draws exactly
// what the lipgloss frame style, centerFrame and FitRect draw together.
func TestRenderFrameMatchesLipgloss(t *testing.T) {
	type setup struct {
		name string
		w, h int
		pad  [2]int
		open func(*Model)
	}
	var cases []setup
	for _, size := range [][2]int{{40, 10}, {60, 16}, {80, 24}, {120, 40}, {200, 60}, {81, 25}} {
		for _, pad := range [][2]int{{-1, -1}, {0, 0}, {3, 2}} {
			for name, open := range map[string]func(*Model){
				"player":  func(*Model) {},
				"actions": (*Model).openKeymap,
				"queue":   func(m *Model) { m.queue.visible = true },
				"fullvis": func(m *Model) { m.fullVis = true },
				"stats":   func(m *Model) { m.openStats() },
			} {
				cases = append(cases, setup{fmt.Sprintf("%dx%d/pad%v/%s", size[0], size[1], pad, name), size[0], size[1], pad, open})
			}
		}
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newLayoutTestModel(tc.w, tc.h)
			if tc.pad[0] >= 0 {
				m.paddingSet, m.paddingH, m.paddingV = true, tc.pad[0], tc.pad[1]
				m.recomputeLayout()
			}
			tc.open(&m)
			content := m.frameContent()
			want := ui.FitRect(m.centerFrame(m.layout.frameStyle().Render(content)), m.layout.frameWidth, max(1, m.height))
			fast := m.layout.panelWidth+2*m.layout.paddingH == m.layout.frameWidth
			for _, line := range strings.Split(content, "\n") {
				fast = fast && ansi.StringWidth(line) <= m.layout.panelWidth
			}
			if !fast && tc.w >= 80 && tc.name[len(tc.name)-6:] == "player" {
				t.Errorf("the player screen at %dx%d takes the slow frame path", tc.w, tc.h)
			}
			if got := m.renderFrame(content); got != want {
				t.Fatalf("fast frame differs:\n got %q\nwant %q", got, want)
			}
		})
	}
}

// A list longer than its window draws a scroll bar in its last column, and
// no row grows past the panel.
func TestLongPlaylistHasAScrollBar(t *testing.T) {
	m := newLayoutTestModel(90, 30)
	for i := range 80 {
		m.playlist.Add(playlist.Track{Path: fmt.Sprint("/s", i), Title: fmt.Sprint("Song ", i), Album: "Album", DurationSecs: 200})
	}
	m.showAlbumHeaders = true
	m.plCursor = m.playlist.Len() - 1
	m.adjustScroll()
	rows := strings.Split(m.renderPlaylist(), "\n")
	thumbs := 0
	for _, row := range rows {
		if w := textWidth(row); w > m.layout.panelWidth {
			t.Fatalf("row %q is %d wide, panel %d", ansi.Strip(row), w, m.layout.panelWidth)
		}
		plain := ansi.Strip(row)
		if !strings.HasSuffix(plain, "▐") && !strings.HasSuffix(plain, "▕") {
			t.Fatalf("row %q has no scroll bar cell", plain)
		}
		if strings.HasSuffix(plain, "▐") {
			thumbs++
		}
	}
	if thumbs == 0 || !strings.HasSuffix(ansi.Strip(rows[len(rows)-1]), "▐") {
		t.Fatalf("at the end of the list the thumb must reach the bottom: %d thumb rows", thumbs)
	}
}

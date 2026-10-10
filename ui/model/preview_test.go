package model

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/ui/artwork"
)

type previewTestProvider struct{ commandsTestProvider }

func (previewTestProvider) PreviewDetails(context.Context, string) (provider.PreviewDetails, error) {
	return provider.PreviewDetails{Lines: []string{"Album · 2020"}}, nil
}

// newPreviewTestModel returns a model on the album list of the nav browser,
// with the preview pane on.
func newPreviewTestModel(width, height int, albums []provider.AlbumInfo) Model {
	m := newLayoutTestModel(width, height)
	m.art = artwork.New("")
	m.navBrowser.visible = true
	m.navBrowser.mode = navBrowseModeByAlbum
	m.navBrowser.prov = previewTestProvider{commandsTestProvider{name: "Test"}}
	m.navBrowser.albums = albums
	m.recomputeLayout()
	return m
}

func writeTestPNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	path := filepath.Join(t.TempDir(), fmt.Sprintf("art-%dx%d.png", w, h))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestYouTubeVideoID(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"https://music.youtube.com/watch?v=dQw4w9WgXcQ&list=RD", "dQw4w9WgXcQ"},
		{"https://www.youtube.com/watch?v=short", ""},
		{"https://example.com/watch?v=dQw4w9WgXcQ", ""},
		{"/music/song.flac", ""},
	}
	for _, tt := range tests {
		if got := youTubeVideoID(tt.path); got != tt.want {
			t.Errorf("youTubeVideoID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestTidyPreviewText(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		lines     []string
		wantTitle string
		wantLines []string
	}{
		{"single named after its song", "Song", []string{"Artist", "Song", "2:21"}, "Song", []string{"Artist", "2:21"}},
		{"year inside another line", "EP", []string{"Artist", "2023", "Single · 2023"}, "EP", []string{"Artist", "Single · 2023"}},
		{"artist after the row name", "Album · Artist", []string{"EP · 2026", "Artist"}, "Album", []string{"EP · 2026", "Artist"}},
		{"nothing repeated", "Album", []string{"Artist", "10 songs"}, "Album", []string{"Artist", "10 songs"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, lines := tidyPreviewText(tt.title, tt.lines)
			if title != tt.wantTitle || strings.Join(lines, "|") != strings.Join(tt.wantLines, "|") {
				t.Fatalf("tidyPreviewText = %q %q, want %q %q", title, lines, tt.wantTitle, tt.wantLines)
			}
		})
	}
}

func TestPreviewLayout(t *testing.T) {
	albums := []provider.AlbumInfo{{ID: "al1", Name: "Album", Artist: "Artist"}}
	tall := newPreviewTestModel(120, 50, albums)
	if rows := tall.layout.previewRows; rows < previewMinRows || rows > previewMaxRows {
		t.Fatalf("browsing preview rows = %d, want %d..%d", rows, previewMinRows, previewMaxRows)
	}
	if tall.layout.visualizerRows < 1 || tall.layout.bodyRows < 10 {
		t.Fatalf("browsing with a preview: visualizer rows %d, body rows %d; want both kept", tall.layout.visualizerRows, tall.layout.bodyRows)
	}

	if short := newPreviewTestModel(80, 24, albums); short.layout.previewRows != 0 {
		t.Fatalf("80x24 preview rows = %d, want 0: the list needs the rows", short.layout.previewRows)
	}

	off := newPreviewTestModel(120, 50, albums)
	off.SetPreview(false)
	off.recomputeLayout()
	if off.layout.previewRows != 0 {
		t.Fatalf("preview off: rows = %d, want 0", off.layout.previewRows)
	}

	playback := newLayoutTestModel(150, 60)
	playback.SetPreview(true)
	playback.recomputeLayout()
	if playback.layout.previewRows < previewMinRows {
		t.Fatalf("tall playback screen preview rows = %d, want at least %d", playback.layout.previewRows, previewMinRows)
	}
	playback.heightExpanded = true
	playback.recomputeLayout()
	if playback.layout.previewRows != 0 {
		t.Fatalf("expanded playlist preview rows = %d, want 0", playback.layout.previewRows)
	}
}

func TestRenderPreviewKeepsAspect(t *testing.T) {
	tests := []struct {
		name       string
		w, h       int
		wantAspect float64 // cell columns per row, two pixels to a cell
	}{
		{"square cover", 120, 120, 2},
		{"16:9 video thumbnail", 160, 90, 2 * 16.0 / 9},
		{"4:3 video thumbnail", 120, 90, 2 * 4.0 / 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeTestPNG(t, tt.w, tt.h)
			m := newPreviewTestModel(120, 50, []provider.AlbumInfo{{ID: "al1", Name: "Album", ImageURL: path}})
			if !m.art.Claim(path) {
				t.Fatal("Claim of a new image = false")
			}
			if err := m.art.Load(context.Background(), path); err != nil {
				t.Fatal(err)
			}
			out := m.renderPreview()
			lines := strings.Split(out, "\n")
			if len(lines) != m.layout.previewRows {
				t.Fatalf("preview lines = %d, want %d", len(lines), m.layout.previewRows)
			}
			var rows, cols int
			for _, l := range lines[1:] {
				if n := strings.Count(l, "▀"); n > 0 {
					rows++
					cols = n
				}
			}
			got := float64(cols) / float64(rows)
			if got < tt.wantAspect*0.85 || got > tt.wantAspect*1.15 {
				t.Fatalf("image %dx%d cells (%.2f columns per row), want about %.2f", cols, rows, got, tt.wantAspect)
			}
			if !strings.Contains(out, "Album") {
				t.Fatal("preview must show the album name")
			}
		})
	}
}

func TestPreviewCmdsPrefetchesNeighbours(t *testing.T) {
	albums := make([]provider.AlbumInfo, 30)
	for i := range albums {
		albums[i] = provider.AlbumInfo{ID: fmt.Sprintf("al%d", i), Name: fmt.Sprintf("Album %d", i), ImageURL: fmt.Sprintf("/missing/cover-%d.png", i)}
	}
	m := newPreviewTestModel(120, 50, albums)
	m.navBrowser.cursor = 10
	if cmd := m.previewCmds(); cmd == nil {
		t.Fatal("previewCmds = nil, want loads for the rows around the cursor")
	}
	for i, a := range albums {
		near := i >= 10-previewBehind && i <= 10+previewAhead
		// A claimed image is loading, so a second Claim fails.
		if claimable := m.art.Claim(a.ImageURL); claimable == near {
			t.Errorf("row %d: claimed = %v, want %v", i, !claimable, near)
		}
		wantDetails := i >= 10 && i <= 10+previewDetailsAhead
		if m.previewPending[a.ID] != wantDetails {
			t.Errorf("row %d: details requested = %v, want %v", i, m.previewPending[a.ID], wantDetails)
		}
	}

	// The details arrive and join the album's own lines.
	m.handlePreviewDetails(previewDetailsMsg{id: "al10", details: provider.PreviewDetails{Lines: []string{"10 songs"}}})
	it, ok := m.previewTarget()
	if !ok || it.title != "Album 10" || !strings.Contains(strings.Join(it.lines, "|"), "10 songs") {
		t.Fatalf("previewTarget = %+v, %v; want Album 10 with its details", it, ok)
	}
}

func TestTrackArtURL(t *testing.T) {
	tests := []struct {
		name  string
		track playlist.Track
		want  string
	}{
		{"art url", playlist.Track{AlbumArtURL: "https://lh3.example/a", Path: "https://www.youtube.com/watch?v=dQw4w9WgXcQ"}, "https://lh3.example/a"},
		{"youtube video", playlist.Track{Path: "https://www.youtube.com/watch?v=dQw4w9WgXcQ", Stream: true}, "https://i.ytimg.com/vi/dQw4w9WgXcQ/mqdefault.jpg"},
		{"local file", playlist.Track{Path: "/music/a.flac"}, "/music/a.flac"},
		{"radio stream", playlist.Track{Path: "https://radio.example/live", Stream: true}, ""},
	}
	for _, tt := range tests {
		if got := trackArtURL(tt.track); got != tt.want {
			t.Errorf("%s: trackArtURL = %q, want %q", tt.name, got, tt.want)
		}
	}
}

package model

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/bjarneo/cliamp/playlist"
	"github.com/bjarneo/cliamp/theme"
	"github.com/bjarneo/cliamp/ui"
)

func newLayoutTestModel(width, height int) Model {
	player := &playbackFakeEngine{}
	pl := playlist.New()
	for i := range 16 {
		pl.Add(playlist.Track{
			Path:  fmt.Sprintf("/tmp/track-%d.mp3", i),
			Title: "A very long 音楽 track title that must remain inside the terminal",
		})
	}
	m := Model{
		player:   player,
		playlist: pl,
		vis:      ui.NewVisualizer(float64(player.SampleRate())),
		width:    width,
		height:   height,
		focus:    focusPlaylist,
	}
	m.vis.Mode = ui.VisBars
	m.recomputeLayout()
	return m
}

func TestFrameLayoutTiers(t *testing.T) {
	tests := []struct {
		name        string
		width       int
		height      int
		wantTier    layoutTier
		wantVisRows int
	}{
		{name: "too small", width: 39, height: 9, wantTier: layoutTooSmall},
		{name: "minimal", width: 40, height: 10, wantTier: layoutMinimal},
		{name: "compact", width: 56, height: 16, wantTier: layoutCompact, wantVisRows: 2},
		{name: "full", width: 80, height: 24, wantTier: layoutFull, wantVisRows: ui.DefaultVisRows - 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newLayoutTestModel(tt.width, tt.height)
			if m.layout.tier != tt.wantTier {
				t.Fatalf("layout tier = %v, want %v", m.layout.tier, tt.wantTier)
			}
			if tt.wantTier == layoutTooSmall {
				if m.layout.bodyRows != 0 {
					t.Fatalf("body rows = %d, want 0", m.layout.bodyRows)
				}
				return
			}
			if m.layout.bodyRows < 1 {
				t.Fatalf("body rows = %d, want at least one", m.layout.bodyRows)
			}
			if m.vis.Rows != tt.wantVisRows {
				t.Fatalf("visualizer rows = %d, want %d", m.vis.Rows, tt.wantVisRows)
			}
			if m.vis.Cols != m.layout.panelWidth {
				t.Fatalf("visualizer columns = %d, want %d", m.vis.Cols, m.layout.panelWidth)
			}
		})
	}
}

func TestResponsiveViewsFitTerminal(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{39, 9},
		{40, 10},
		{56, 16},
		{80, 20},
		{80, 24},
		{120, 40},
	} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			m := newLayoutTestModel(size.width, size.height)
			m.status.text = "a status message\nthat must not create another row"
			out := m.View().Content
			if got := lipgloss.Height(out); got > size.height {
				t.Fatalf("view height = %d, want <= %d\n%s", got, size.height, out)
			}
			for _, line := range strings.Split(out, "\n") {
				if got := lipgloss.Width(line); got > size.width {
					t.Fatalf("line width = %d, want <= %d: %q", got, size.width, line)
				}
			}
		})
	}
}

func TestResizeClampsActiveOverlayCursor(t *testing.T) {
	m := newLayoutTestModel(120, 40)
	m.themePicker.visible = true
	m.themes = make([]theme.Theme, 40)
	m.themePicker.cursor = 40
	m.themePicker.scroll = 35

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 56, Height: 16})
	m = updated.(Model)
	if m.themePicker.cursor >= len(m.themes)+1 {
		t.Fatalf("theme cursor = %d, want within %d entries", m.themePicker.cursor, len(m.themes)+1)
	}
	if m.themePicker.cursor < m.themePicker.scroll || m.themePicker.cursor >= m.themePicker.scroll+m.themePickerVisible() {
		t.Fatalf("theme cursor %d outside viewport [%d,%d)", m.themePicker.cursor, m.themePicker.scroll, m.themePicker.scroll+m.themePickerVisible())
	}
}

func TestLayoutClampsConfiguredPadding(t *testing.T) {
	previousStyle := ui.FrameStyle
	previousPanelWidth := ui.PanelWidth
	previousPaddingH := ui.PaddingH
	previousPaddingV := ui.VerticalPadding()
	ui.SetPadding(10, 5)
	t.Cleanup(func() {
		ui.SetPadding(previousPaddingH, previousPaddingV)
		ui.FrameStyle = previousStyle
		ui.PanelWidth = previousPanelWidth
	})

	m := newLayoutTestModel(40, 10)
	if m.layout.panelWidth <= 0 {
		t.Fatalf("panel width = %d, want positive", m.layout.panelWidth)
	}
	if got := m.View().Content; lipgloss.Height(got) > 10 {
		t.Fatalf("view height = %d, want <= 10", lipgloss.Height(got))
	}
}

func TestTooSmallLayoutBlocksHiddenMutations(t *testing.T) {
	m := newLayoutTestModel(39, 9)
	before := m.playlist.Len()
	m.handleKey(tea.KeyPressMsg{Text: "x"})
	if got := m.playlist.Len(); got != before {
		t.Fatalf("playlist length = %d after hidden remove, want %d", got, before)
	}
}

func TestTrackInfoScrollsWithinBodyBudget(t *testing.T) {
	m := newLayoutTestModel(40, 10)
	track := m.playlist.Tracks()[0]
	track.Artist = "Artist"
	track.Album = "Album"
	track.Genre = "Genre"
	track.Year = 2026
	track.TrackNumber = 1
	m.playlist.SetTrack(0, track)
	m.showInfo = true

	m.handleKey(tea.KeyPressMsg{Text: "j"})
	if m.infoScroll == 0 {
		t.Fatal("info scroll = 0 after down, want a later metadata row")
	}
	if got := m.renderInfoBody(); !strings.Contains(got, "Artist") {
		t.Fatalf("track info body = %q, want scrolled metadata", got)
	}
}

func TestInlineOverlaysFitResponsiveTerminal(t *testing.T) {
	overlays := []struct {
		name string
		set  func(*Model)
	}{
		{name: "keymap", set: func(m *Model) { m.keymap.visible = true; m.keymap.entries = m.buildKeymapEntries() }},
		{name: "theme", set: func(m *Model) { m.themePicker.visible = true }},
		{name: "visualizer", set: func(m *Model) { m.visPicker.visible = true; m.visPicker.modes = m.vis.AllModeNames() }},
		{name: "device", set: func(m *Model) { m.devicePicker.visible = true }},
		{name: "playlist picker", set: func(m *Model) { m.plPicker.visible = true }},
		{name: "file browser", set: func(m *Model) { m.fileBrowser.visible = true }},
		{name: "provider search", set: func(m *Model) { m.spotSearch.visible = true }},
		{name: "navigation", set: func(m *Model) { m.navBrowser.visible = true }},
		{name: "playlist manager", set: func(m *Model) { m.plManager.visible = true }},
		{name: "queue", set: func(m *Model) { m.queue.visible = true }},
		{name: "info", set: func(m *Model) { m.showInfo = true }},
		{name: "lyrics", set: func(m *Model) { m.lyrics.visible = true }},
		{name: "jump", set: func(m *Model) { m.jumping = true }},
		{name: "url", set: func(m *Model) { m.urlInputting = true }},
		{name: "search", set: func(m *Model) { m.search.active = true }},
		{name: "online search", set: func(m *Model) { m.netSearch.active = true }},
	}

	for _, size := range []struct{ width, height int }{{40, 10}, {56, 16}, {80, 24}} {
		for _, overlay := range overlays {
			t.Run(fmt.Sprintf("%s_%dx%d", overlay.name, size.width, size.height), func(t *testing.T) {
				m := newLayoutTestModel(size.width, size.height)
				overlay.set(&m)
				assertViewFits(t, m.View().Content, size.width, size.height)
			})
		}
	}
}

func assertViewFits(t *testing.T, view string, width, height int) {
	t.Helper()
	if got := lipgloss.Height(view); got > height {
		t.Fatalf("view height = %d, want <= %d\n%s", got, height, view)
	}
	for _, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("line width = %d, want <= %d: %q", got, width, line)
		}
	}
}

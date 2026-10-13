package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/xaviiromf/cliamp-x/lyrics"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/ui/artwork"
)

// smallScreenModel is a player with the data a phone shows: long titles
// with accents, artists, albums, a provider pane and the preview pane.
func smallScreenModel(width, height int) Model {
	m := newLayoutTestModel(width, height)
	m.art = artwork.New("") // the preview pane, as the player has it
	m.playlist = playlist.New()
	for i := range 30 {
		m.playlist.Add(playlist.Track{
			Path:         fmt.Sprintf("/tmp/música/%02d.mp3", i),
			Title:        fmt.Sprintf("Un día de verano en la ciudad de Valparaíso, parte %d", i+1),
			Artist:       "Los Tres y la Orquesta Sinfónica de Concepción",
			Album:        fmt.Sprintf("Grandes éxitos en vivo, volumen %d", i/5+1),
			DurationSecs: 200 + i,
		})
	}
	m.plCursor = 2
	m.provider = commandsTestProvider{name: "Local"}
	m.provPane.lists = []playlist.PlaylistInfo{
		{ID: "fav", Name: "Favoritos con un nombre de lista bastante largo", TrackCount: 120},
		{ID: "hist", Name: "Escuchado recientemente", TrackCount: 5},
	}
	m.status.Error("No se pudo listar los dispositivos: pactl: exit status 1, el servidor no responde", statusTTLDefault)
	m.recomputeLayout()
	return m
}

// Every screen fits the phone sizes: no line wider than the terminal, no
// more lines than its rows, and the actions menu one tap away on the hint
// bar (the full-screen visualizer shows it as "?").
func TestSmallScreensFitTerminal(t *testing.T) {
	sampleLyrics := []lyrics.Line{
		{Text: "Is this the real life? Is this just fantasy?"},
		{Text: "Sends shivers down my spine, body's aching all the time, goodbye everybody"},
	}
	screens := []struct {
		name  string
		keys  []string
		setup func(*Model) // runs after the keys
		want  []string     // text the plan's phone sizes must show whole, translated
	}{
		{name: "queue"},
		{name: "explore", keys: []string{"esc"}},
		{name: "actions menu", keys: []string{"f1"}},
		{name: "actions menu scrolled", keys: []string{"f1", "pgdown", "pgdown"}},
		{name: "row menu", keys: []string{"O"}},
		{name: "filter", keys: []string{"/", "c", "i"}},
		{name: "sleep timer", keys: []string{"T"}},
		{name: "equalizer", keys: []string{"E"}, want: []string{"Flat ●", "16k"}},
		// The panel opens on its last row under the queue layout and still
		// shows five presets.
		{name: "equalizer custom", keys: []string{"E", "+", "esc", "E"}, want: []string{"Loudness", "Small Speakers", "16k"}},
		{name: "statistics", setup: func(m *Model) { m.stats = statsState{visible: true, empty: true} }}, // S needs a play log
		{name: "downloads", keys: []string{"ctrl+o"}},
		{name: "lyrics", keys: []string{"y"}, setup: func(m *Model) { m.lyrics.loading, m.lyrics.err, m.lyrics.lines = false, nil, sampleLyrics }},
		{name: "track info", keys: []string{"i"}},
		{name: "queue manager", keys: []string{"a", "A"}},
		{name: "theme picker", keys: []string{"t"}},
		{name: "visualizer picker", keys: []string{"ctrl+v"}},
		{name: "jump", keys: []string{"ctrl+j"}},
		{name: "load URL", keys: []string{"u"}},
		{name: "file browser", keys: []string{"o"}},
		{name: "save to playlist", setup: func(m *Model) { // w needs the local provider
			m.plPicker = playlistPickerState{visible: true, playlists: m.provPane.lists, title: "Track: Un día de verano en la ciudad de Valparaíso"}
		}},
		{name: "full visualizer", keys: []string{"V"}},
		{name: "volume focus", keys: []string{"tab"}},
		{name: "new playlist", setup: func(m *Model) { m.plManager.visible, m.plManager.screen = true, plMgrScreenNewName },
			want: []string{"Enter creates an empty playlist, then opens the file browser."}},
	}
	sizes := [][2]int{{60, 32}, {110, 30}, {56, 16}, {45, 28}}
	for _, spanish := range []bool{false, true} {
		for _, size := range sizes {
			for _, sc := range screens {
				t.Run(fmt.Sprintf("es=%v/%dx%d/%s", spanish, size[0], size[1], sc.name), func(t *testing.T) {
					spanishUI = spanish
					t.Cleanup(func() { spanishUI = false })
					m := smallScreenModel(size[0], size[1])
					for _, key := range sc.keys {
						msg, ok := keyPressFor(key)
						if !ok {
							t.Fatalf("no key press for %q", key)
						}
						m.handleKey(msg)
						m.recomputeLayout() // as Update does after each message
					}
					if sc.setup != nil {
						sc.setup(&m)
						m.recomputeLayout()
					}
					view := m.View().Content
					lines := strings.Split(view, "\n")
					if len(lines) > size[1] {
						t.Fatalf("%d lines, want <= %d\n%s", len(lines), size[1], ansi.Strip(view))
					}
					for _, line := range lines {
						if w := textWidth(line); w > size[0] {
							t.Fatalf("line %q is %d wide, want <= %d\n%s", ansi.Strip(line), w, size[0], ansi.Strip(view))
						}
					}
					// View clips the frame to the terminal, so the content is
					// what shows an overflow: more rows than the padded frame
					// holds lose the bottom, and a line wider than the panel
					// would wrap inside the frame.
					content := strings.Split(m.frameContent(), "\n")
					if rows := size[1] - 2*m.layout.paddingV; len(content) > rows {
						t.Fatalf("%d content lines, want <= %d\n%s", len(content), rows, ansi.Strip(m.frameContent()))
					}
					for _, line := range content {
						if w := textWidth(line); w > m.layout.panelWidth {
							t.Fatalf("content line %q is %d wide, panel %d", ansi.Strip(line), w, m.layout.panelWidth)
						}
					}
					plain := ansi.Strip(view)
					actions := " F1 "
					if m.fullVis {
						actions = " ? "
					}
					if !m.hideHelpBar && !strings.Contains(plain, actions) {
						t.Errorf("no %q hint on the screen:\n%s", actions, plain)
					}
					// A wrapped text reads whole once the line breaks and the
					// frame padding are one space.
					flat := strings.Join(strings.Fields(plain), " ")
					for _, want := range sc.want {
						if size[1] >= 30 && !strings.Contains(flat, L(want)) {
							t.Errorf("no %q on the screen:\n%s", L(want), plain)
						}
					}
					if sc.name == "row menu" && size[1] >= 30 && m.effectivePlaylistVisible() < len(m.keymap.entries) {
						t.Errorf("row menu shows %d of %d rows", m.effectivePlaylistVisible(), len(m.keymap.entries))
					}
				})
			}
		}
	}
}

// The short hints of a narrow hint bar speak the interface language.
func TestNarrowHintBarIsTranslated(t *testing.T) {
	spanishUI = true
	t.Cleanup(func() { spanishUI = false })
	m := smallScreenModel(45, 28)
	m.openKeymap()
	help := ansi.Strip(m.renderHelp())
	for _, want := range []string{"Esc  Volver", "Enter  Ir", "F1  Ayuda"} {
		if !strings.Contains(help, want) {
			t.Errorf("narrow hint bar %q lacks %q", help, want)
		}
	}
}

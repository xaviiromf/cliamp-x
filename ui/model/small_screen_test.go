package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/xaviiromf/cliamp-x/lyrics"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// smallScreenModel is a player with the data a phone shows: long titles
// with accents, artists, albums, a provider pane and synced lyrics.
func smallScreenModel(width, height int) Model {
	m := newLayoutTestModel(width, height)
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
	m.lyrics.lines = []lyrics.Line{
		{Text: "Sends shivers down my spine, body's aching all the time, goodbye everybody"},
		{Text: "Mama, oo-oo-oo"},
	}
	m.status.Error("No se pudo listar los dispositivos: pactl: exit status 1, el servidor no responde", statusTTLDefault)
	m.recomputeLayout()
	return m
}

// Every screen fits the phone sizes: no line wider than the terminal, no
// more lines than its rows, and the actions menu one tap away on the hint
// bar (the full-screen visualizer shows it as "?").
func TestSmallScreensFitTerminal(t *testing.T) {
	screens := []struct {
		name string
		keys []string
	}{
		{"queue", nil},
		{"explore", []string{"esc"}},
		{"actions menu", []string{"f1"}},
		{"actions menu scrolled", []string{"f1", "pgdown", "pgdown"}},
		{"row menu", []string{"O"}},
		{"filter", []string{"/", "c", "i"}},
		{"sleep timer", []string{"T"}},
		{"equalizer", []string{"E"}},
		{"statistics", []string{"S"}},
		{"downloads", []string{"ctrl+o"}},
		{"lyrics", []string{"y"}},
		{"track info", []string{"i"}},
		{"queue manager", []string{"a", "A"}},
		{"theme picker", []string{"t"}},
		{"visualizer picker", []string{"ctrl+v"}},
		{"jump", []string{"ctrl+j"}},
		{"load URL", []string{"u"}},
		{"file browser", []string{"o"}},
		{"save to playlist", []string{"w"}},
		{"full visualizer", []string{"V"}},
		{"volume focus", []string{"tab"}},
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
					// A line wider than the panel would wrap inside the frame.
					for _, line := range strings.Split(m.frameContent(), "\n") {
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

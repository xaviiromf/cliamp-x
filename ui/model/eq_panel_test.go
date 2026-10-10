package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
)

// preampFakeEngine is the playback fake with an EQ pre-amp stage.
type preampFakeEngine struct {
	playbackFakeEngine
	preamp float64
}

func (f *preampFakeEngine) SetPreamp(db float64) { f.preamp = db }
func (f *preampFakeEngine) Preamp() float64      { return f.preamp }

func eqTestModel() (Model, *preampFakeEngine) {
	m := keybindingTestModel()
	engine := &preampFakeEngine{}
	m.player = engine
	m.eqPresetIdx = -1
	return m, engine
}

func presetIndex(t *testing.T, name string) int {
	t.Helper()
	for i, p := range eqPresets {
		if p.Name == name {
			return i
		}
	}
	t.Fatalf("no preset %q", name)
	return -1
}

func TestPresetForGenre(t *testing.T) {
	for genre, want := range map[string]string{
		"Heavy Metal":      "Metal",
		"Reggaetón":        "Reggaeton",
		"Cumbia villera":   "Cumbia",
		"Hip-Hop/Rap":      "Hip-Hop",
		"Rock alternativo": "Indie",
		"Rock":             "Rock",
		"Electronic":       "Electronic",
		"Música clásica":   "Classical",
		"Lo-fi beats":      "Lo-Fi",
		"Bandas sonoras":   "Soundtrack",
		"K-Pop":            "Pop",
		"Jazz":             "Jazz",
		"Folk":             "Acoustic",
		"Soul":             "R&B",
		"Podcasts":         "Podcast",
	} {
		p, ok := presetForGenre(genre)
		if !ok || p.Name != want {
			t.Errorf("presetForGenre(%q) = %q, %v; want %q", genre, p.Name, ok, want)
		}
	}
	if _, ok := presetForGenre(""); ok {
		t.Error("an empty genre matched a preset")
	}
	if _, ok := presetForGenre("Gregorian chant xyz"); ok {
		t.Error("an unknown genre matched a preset")
	}
	for _, k := range genreKeywords {
		if _, ok := EQPresetByName(k.preset); !ok {
			t.Errorf("keyword preset %q does not exist", k.preset)
		}
	}
}

func TestEQHeadroomFollowsTheBoosts(t *testing.T) {
	m, engine := eqTestModel()
	m.eqPresetIdx = presetIndex(t, "Bass Boost") // peaks at +8
	m.applyEQPreset()
	if want := -0.7 * 8; engine.preamp != want {
		t.Fatalf("preamp = %v, want %v for an 8 dB boost", engine.preamp, want)
	}
	m.SetEQPreset("Flat", nil)
	if engine.preamp != 0 {
		t.Fatalf("preamp = %v, want 0 for a flat curve", engine.preamp)
	}
	m.setCustomEQBand(3, 4)
	if want := -0.7 * 4; engine.preamp != want {
		t.Fatalf("preamp after a band edit = %v, want %v", engine.preamp, want)
	}
}

func TestEQPanelPreviewsPresetsAndSkipsHeadings(t *testing.T) {
	m, engine := eqTestModel()
	m.openEQPanel()
	if !m.eqPanel.visible || eqPanelRows()[m.eqPanel.cursor].heading != "" {
		t.Fatalf("panel cursor on %+v", eqPanelRows()[m.eqPanel.cursor])
	}
	rows := eqPanelRows()
	for range len(rows) + 2 {
		m.handleEQPanelKey(tea.KeyPressMsg{Code: tea.KeyDown})
		row := rows[m.eqPanel.cursor]
		if row.heading != "" {
			t.Fatal("the cursor stopped on a heading")
		}
		if row.preset >= 0 && engine.EQBands() != eqPresets[row.preset].Bands {
			t.Fatalf("preset %q not previewed", eqPresets[row.preset].Name)
		}
	}
	m.handleEQPanelKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.eqPanel.visible {
		t.Fatal("Esc did not close the panel")
	}
}

func TestEQPanelBandEditsAndBypass(t *testing.T) {
	m, engine := eqTestModel()
	m.eqPresetIdx = presetIndex(t, "Rock")
	m.applyEQPreset()
	m.openEQPanel()
	m.handleEQPanelKey(tea.KeyPressMsg{Code: tea.KeyRight})
	m.handleEQPanelKey(tea.KeyPressMsg{Code: tea.KeyUp})
	rock := eqPresets[presetIndex(t, "Rock")].Bands
	if engine.EQBands()[1] != rock[1]+1 || m.eqPresetIdx != -1 {
		t.Fatalf("band edit: bands %v preset %d", engine.EQBands(), m.eqPresetIdx)
	}
	edited := engine.EQBands()

	m.handleEQPanelKey(tea.KeyPressMsg{Text: "b", Code: 'b'})
	if !m.eqBypass || engine.EQBands() != ([eqBandCount]float64{}) || engine.preamp != 0 {
		t.Fatalf("bypass: bands %v preamp %v", engine.EQBands(), engine.preamp)
	}
	m.handleEQPanelKey(tea.KeyPressMsg{Text: "b", Code: 'b'})
	if m.eqBypass || engine.EQBands() != edited {
		t.Fatalf("bypass off: bands %v, want %v", engine.EQBands(), edited)
	}

	m.handleEQPanelKey(tea.KeyPressMsg{Text: "r", Code: 'r'})
	if m.EQPresetName() != "Flat" || engine.EQBands() != ([eqBandCount]float64{}) {
		t.Fatalf("reset: preset %q bands %v", m.EQPresetName(), engine.EQBands())
	}
}

func TestAutoEQFollowsTheGenre(t *testing.T) {
	m, engine := eqTestModel()
	m.applyAutoEQ(playlist.Track{Genre: "Metal"})
	if m.eqPresetIdx != -1 {
		t.Fatal("auto EQ off still changed the preset")
	}
	m.SetEQAuto(true)
	m.applyAutoEQ(playlist.Track{Genre: "Heavy Metal"})
	if m.EQPresetName() != "Metal" || engine.EQBands() != eqPresets[presetIndex(t, "Metal")].Bands || m.eqAutoLabel != "Heavy Metal" {
		t.Fatalf("auto EQ: preset %q label %q", m.EQPresetName(), m.eqAutoLabel)
	}
	// A song without a known genre keeps the current preset.
	m.applyAutoEQ(playlist.Track{Genre: "unknown"})
	if m.EQPresetName() != "Metal" {
		t.Fatalf("unknown genre changed the preset to %q", m.EQPresetName())
	}
}

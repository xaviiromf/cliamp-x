package model

import (
	"testing"

	"github.com/xaviiromf/cliamp-x/ui"
)

func TestVisSensitivityKeys(t *testing.T) {
	m := newLayoutTestModel(120, 40)
	if got := m.vis.Sensitivity(); got != ui.DefaultVisSensitivity {
		t.Fatalf("default sensitivity = %d, want %d", got, ui.DefaultVisSensitivity)
	}
	m.adjustVisSensitivity(1)
	if got := m.vis.Sensitivity(); got != ui.DefaultVisSensitivity+1 {
		t.Fatalf("after } = %d, want %d", got, ui.DefaultVisSensitivity+1)
	}
	for range 20 {
		m.adjustVisSensitivity(-1)
	}
	if got := m.vis.Sensitivity(); got != ui.MinVisSensitivity {
		t.Fatalf("after many { = %d, want %d", got, ui.MinVisSensitivity)
	}
	if m.status.text == "" {
		t.Fatal("changing the sensitivity must show the new level")
	}
}

func TestVisSensitivityInKeymap(t *testing.T) {
	m := newLayoutTestModel(120, 40)
	for _, e := range m.buildKeymapEntries() {
		if e.key == "}" && e.run == "}" {
			return
		}
	}
	t.Fatal("the actions menu must list } for the visualizer sensitivity")
}

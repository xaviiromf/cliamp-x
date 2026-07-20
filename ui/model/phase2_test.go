package model

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestGlobalHelpOpensOverActiveTextInput(t *testing.T) {
	m := Model{search: searchState{active: true, query: "jazz"}}

	m.handleKey(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if !m.keymap.visible {
		t.Fatal("keymap.visible = false after Ctrl+K, want true")
	}
	if !m.search.active || m.search.query != "jazz" {
		t.Fatalf("search state = %+v, want active input preserved", m.search)
	}

	m.handleKey(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if m.keymap.visible {
		t.Fatal("keymap.visible = true after second Ctrl+K, want false")
	}
	if !m.search.active || m.search.query != "jazz" {
		t.Fatalf("search state = %+v after closing help, want active input preserved", m.search)
	}
}

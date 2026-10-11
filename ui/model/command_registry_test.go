package model

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

func TestCommandHelpKeepsEssentialHintsAtMinimumWidth(t *testing.T) {
	m := Model{width: 40, layout: frameLayout{panelWidth: 40}, playlist: playlist.New()}
	m.playlist.Add(playlist.Track{Title: "Track"})
	m.playlist.Queue(0)

	tests := []struct {
		name string
		mode commandMode
		keys []string
	}{
		{name: "main", mode: commandModeMain, keys: []string{"Space", "F1"}},
		{name: "provider", mode: commandModeProvider, keys: []string{"Esc", "Enter", "F1"}},
		{name: "equalizer", mode: commandModeEQ, keys: []string{"Space", "F1"}},
		{name: "speed", mode: commandModeSpeed, keys: []string{"Space", "F1"}},
		{name: "provider pill", mode: commandModeProviderPill, keys: []string{"Esc", "Enter", "F1"}},
		{name: "keymap", mode: commandModeKeymap, keys: []string{"Esc", "Enter", "F1"}},
		{name: "keymap search", mode: commandModeKeymapSearch, keys: []string{"Esc", "Enter", "F1"}},
		{name: "file browser", mode: commandModeFileBrowser, keys: []string{"Esc", "Enter", "F1"}},
		{name: "file search", mode: commandModeFileBrowserSearch, keys: []string{"Esc", "Enter", "F1"}},
		{name: "navigation", mode: commandModeNavBrowser, keys: []string{"Esc", "Enter", "F1"}},
		{name: "navigation search", mode: commandModeNavSearch, keys: []string{"Esc", "Enter", "F1"}},
		{name: "playlist manager", mode: commandModePlaylistManager, keys: []string{"Esc", "Enter", "F1"}},
		{name: "playlist manager input", mode: commandModePlaylistManagerInput, keys: []string{"Esc", "Enter", "F1"}},
		{name: "playlist picker", mode: commandModePlaylistPicker, keys: []string{"Esc", "Enter", "F1"}},
		{name: "playlist picker input", mode: commandModePlaylistPickerInput, keys: []string{"Esc", "Enter", "F1"}},
		{name: "queue", mode: commandModeQueue, keys: []string{"Esc", "d", "F1"}},
		{name: "text input", mode: commandModeSearch, keys: []string{"Esc", "Enter", "F1"}},
		{name: "network search", mode: commandModeNetSearch, keys: []string{"Esc", "Enter", "F1"}},
		{name: "provider search", mode: commandModeSearchOverlay, keys: []string{"Esc", "Enter", "F1"}},
		{name: "jump", mode: commandModeJump, keys: []string{"Esc", "Enter", "F1"}},
		{name: "URL", mode: commandModeURL, keys: []string{"Esc", "Enter", "F1"}},
		{name: "lyrics", mode: commandModeLyrics, keys: []string{"Esc", "r", "F1"}},
		{name: "theme picker", mode: commandModeThemePicker, keys: []string{"Esc", "Enter", "F1"}},
		{name: "visualizer picker", mode: commandModeVisPicker, keys: []string{"Esc", "Enter", "F1"}},
		{name: "device picker", mode: commandModeDevicePicker, keys: []string{"Esc", "Enter", "F1"}},
		{name: "track info", mode: commandModeInfo, keys: []string{"Esc", "F1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			help := m.commandHelp(tt.mode)
			if got := lipgloss.Width(help); got > 40 {
				t.Fatalf("help width = %d, want <= 40: %q", got, help)
			}
			for _, key := range tt.keys {
				if !strings.Contains(help, key) {
					t.Fatalf("help %q does not include %q", help, key)
				}
			}
		})
	}
}

func TestReservedKeysComeFromCommandRegistry(t *testing.T) {
	reserved := ReservedKeys()
	for _, command := range commandRegistry {
		for _, key := range command.Keys {
			if !reserved[key] {
				t.Fatalf("reserved keys omit registry key %q", key)
			}
		}
	}
}

func TestContextHelpAdvertisesProviderBrowsing(t *testing.T) {
	browse := trackArtistBrowseProvider{interactionBrowseProvider{commandsTestProvider{name: "Mixcloud"}}}
	m := keybindingTestModel()
	m.layout.panelWidth = 80
	m.provider = browse
	m.providers = append(m.providers, provider.Entry{Key: "mixcloud", Name: "Mixcloud", Provider: browse})
	m.playlist.Add(playlist.Track{
		Title: "A Show", Artist: "Creator",
		ProviderMeta: map[string]string{"test.creator": "creator"},
	})
	m.focus = focusPlaylist
	if help := m.commandHelp(commandModeMain); !strings.Contains(help, "N") || !strings.Contains(help, "Browse creator") {
		t.Fatalf("playlist help does not advertise creator browse: %q", help)
	}

	m.focus = focusProvider
	paneBrowse := providerPaneBrowseProvider{interactionBrowseProvider{commandsTestProvider{name: "Mixcloud"}}}
	m.provider = paneBrowse
	m.provPane.lists = providerListsWithBrowse(paneBrowse, nil)
	if help := m.commandHelp(commandModeProvider); !strings.Contains(help, "N") || !strings.Contains(help, "Browse provider") || !strings.Contains(help, "Open") {
		t.Fatalf("provider help does not advertise provider browse: %q", help)
	}

	// A browse-capable provider elsewhere in the registry must not advertise
	// its browser while a provider without pane routes is being viewed.
	m.provider = commandsTestProvider{name: "Spotify"}
	if help := m.commandHelp(commandModeProvider); strings.Contains(help, "Browse provider") {
		t.Fatalf("Spotify provider help advertises another provider's browser: %q", help)
	}

	// Outside the provider pane, a highlighted Mixcloud show may still expose
	// its direct creator jump even if another provider is active.
	m.focus = focusPlaylist
	if help := m.commandHelp(commandModeMain); !strings.Contains(help, "Browse creator") {
		t.Fatalf("selected Mixcloud show lost its creator browse action: %q", help)
	}
}

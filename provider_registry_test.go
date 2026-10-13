package main

import (
	"slices"
	"testing"

	"github.com/xaviiromf/cliamp-x/config"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

type stubProvider struct{ name string }

func (s stubProvider) Name() string                                { return s.name }
func (s stubProvider) Playlists() ([]playlist.PlaylistInfo, error) { return nil, nil }
func (s stubProvider) Tracks(string) ([]playlist.Track, error)     { return nil, nil }

// A provider added in its own file joins the list at its order, and one that
// panics while starting is left out without taking the others down.
func TestRegisterProviderFromAFork(t *testing.T) {
	isolateProviderEnv(t)
	saved, savedKeys := slices.Clone(providerFactories), slices.Clone(providerKeys)
	t.Cleanup(func() { providerFactories, providerKeys = saved, savedKeys })

	registerProvider(providerFactory{
		providerKey: providerKey{key: "forkradio", name: "Fork Radio"},
		order:       50,
		shortcut:    "K",
		build: func(env providerEnv) []provider.Entry {
			return []provider.Entry{{Provider: stubProvider{"Fork Radio"}}}
		},
	})
	registerProvider(providerFactory{
		providerKey: providerKey{key: "broken", name: "Broken"},
		order:       60,
		build:       func(providerEnv) []provider.Entry { panic("bad config") },
	})

	set := buildProviders(config.Config{}, false)
	t.Cleanup(set.Close)
	var keys []string
	for _, e := range set.entries {
		keys = append(keys, e.Key)
		if e.Key == "forkradio" && (e.Name != "Fork Radio" || e.Shortcut != "K") {
			t.Errorf("fork entry = %+v, want its name and shortcut filled in", e)
		}
	}
	if want := []string{"ytmusic", "forkradio", "local"}; !slices.Equal(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if got, err := parseProviderKey("ForkRadio"); err != nil || got != "forkradio" {
		t.Fatalf("parseProviderKey = %q, %v", got, err)
	}
}

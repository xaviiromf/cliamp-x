package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"

	"github.com/xaviiromf/cliamp-x/config"
	"github.com/xaviiromf/cliamp-x/favorites"
	"github.com/xaviiromf/cliamp-x/history"
	"github.com/xaviiromf/cliamp-x/player"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/ui/model"
)

// isolateProviderEnv points every config and credential lookup at a new
// directory, so buildProviders reads no real config and starts no provider
// from the environment.
func isolateProviderEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	for _, name := range []string{"CLIAMP_CONFIG_DIR", "XDG_CONFIG_HOME", "YTMUSIC_COOKIES_FROM"} {
		t.Setenv(name, "")
	}
}

func TestBuildProviders(t *testing.T) {
	for _, tt := range []struct {
		name string
		cfg  config.Config
		want []string
	}{
		{name: "empty config", want: []string{"ytmusic", "local"}},
		{name: "ytmusic disabled", cfg: config.Config{YouTubeMusic: config.YouTubeMusicConfig{Disabled: true}}, want: []string{"local"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			isolateProviderEnv(t)
			set := buildProviders(tt.cfg, false)
			t.Cleanup(set.Close)

			var keys []string
			for _, e := range set.entries {
				if e.Provider == nil {
					t.Errorf("entry %q has no provider", e.Key)
				}
				if e.Key != "local" && !slices.ContainsFunc(providerKeys, func(pk providerKey) bool { return pk.key == e.Key }) {
					t.Errorf("entry %q is not in providerKeys, so --provider rejects it", e.Key)
				}
				keys = append(keys, e.Key)
			}
			if !slices.Equal(keys, tt.want) {
				t.Fatalf("keys = %v, want %v", keys, tt.want)
			}
			if set.local == nil {
				t.Fatal("the local provider must be set")
			}
		})
	}
}

// buildProviders makes one favorites store and one history store. The local
// provider lists the virtual playlists from the same stores, so a write
// through the set shows in the local provider at once.
func TestBuildProvidersSharesTheStores(t *testing.T) {
	isolateProviderEnv(t)
	set := buildProviders(config.Config{}, false)
	t.Cleanup(set.Close)
	if set.favorites == nil || set.history == nil {
		t.Fatal("the favorites and history stores must be set")
	}
	track := playlist.Track{Path: "/music/a.mp3", Title: "A"}
	if _, err := set.favorites.Toggle(track); err != nil {
		t.Fatal(err)
	}
	if err := set.history.Record(track, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{favorites.PlaylistName, history.PlaylistName} {
		tracks, err := set.local.Tracks(name)
		if err != nil || len(tracks) != 1 || tracks[0].Path != track.Path {
			t.Errorf("local Tracks(%q) = %+v, %v, want %s", name, tracks, err, track.Path)
		}
	}
}

// With no HOME, XDG_CONFIG_HOME or CLIAMP_CONFIG_DIR, cliamp has no config
// directory and no local provider. The Model then starts without one and
// does not panic.
func TestBuildProvidersWithoutConfigDir(t *testing.T) {
	// APPDATA and USERPROFILE are the config dir fallbacks on Windows.
	for _, name := range []string{"HOME", "CLIAMP_CONFIG_DIR", "XDG_CONFIG_HOME", "APPDATA", "USERPROFILE", "YTMUSIC_COOKIES_FROM"} {
		t.Setenv(name, "")
	}
	set := buildProviders(config.Config{}, false)
	t.Cleanup(set.Close)
	if set.local != nil {
		t.Fatal("local provider set without a config directory")
	}
	for _, e := range set.entries {
		if e.Key == "local" {
			t.Fatal("the provider list has a local entry without a config directory")
		}
	}
	if lp := set.localPlaylists(); lp != nil {
		t.Fatalf("localPlaylists() = %#v, want a nil interface", lp)
	}
	model.New(&player.Player{}, playlist.New(), set.entries, "ytmusic", set.localPlaylists(), set.favorites, set.history, nil, nil, config.SaveFunc{})
}

// fakeStreamer decodes fake: URIs, as Spotify decodes spotify: URIs, and
// counts its Close calls.
type fakeStreamer struct {
	closes int
}

func (*fakeStreamer) Name() string                                { return "Fake" }
func (*fakeStreamer) Playlists() ([]playlist.PlaylistInfo, error) { return nil, nil }
func (*fakeStreamer) Tracks(string) ([]playlist.Track, error)     { return nil, nil }
func (*fakeStreamer) URISchemes() []string                        { return []string{"fake:"} }
func (f *fakeStreamer) Close()                                    { f.closes++ }

func (*fakeStreamer) NewStreamer(string) (beep.StreamSeekCloser, beep.Format, time.Duration, error) {
	return nil, beep.Format{}, 0, errors.New("fake streamer opened")
}

var (
	_ provider.CustomStreamer = (*fakeStreamer)(nil)
	_ provider.Closer         = (*fakeStreamer)(nil)
)

// The player opens the URI schemes of every provider.CustomStreamer with
// that provider, and a shutdown closes every provider.Closer. No provider
// name appears in the wiring.
func TestProviderSetUsesCapabilities(t *testing.T) {
	fake := &fakeStreamer{}
	set := &providerSet{entries: []provider.Entry{
		{Key: "radio", Name: "Radio", Provider: &fakeRadio{}},
		{Key: "fake", Name: "Fake", Provider: fake},
	}}

	engine := &player.Player{}
	set.registerPlayerHooks(engine)
	err := engine.PlayAtForGeneration("fake:track:1", time.Minute, 0, 1)
	if err == nil || !strings.Contains(err.Error(), "fake streamer opened") {
		t.Fatalf("PlayAtForGeneration error = %v, want the error of the fake streamer", err)
	}

	set.Close()
	if fake.closes != 1 {
		t.Fatalf("Close calls = %d, want 1", fake.closes)
	}
}

// A page URL that playlist.IsYTDL accepts opens through yt-dlp.
func TestPlayerHooksSendYTDLPagesToYTDLP(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX shell fixtures")
	}
	bin := t.TempDir()
	for name, body := range map[string]string{
		"yt-dlp": "#!/bin/sh\necho 'ERROR: fake yt-dlp ran' >&2\nexit 1\n",
		"ffmpeg": "#!/bin/sh\ncat >/dev/null\n",
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	// A request that reaches the network fails at once.
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	set := &providerSet{}
	engine := &player.Player{}
	set.registerPlayerHooks(engine)
	for _, op := range []struct {
		name string
		open func(path string) error
	}{
		{name: "play", open: func(path string) error { return engine.PlayAtForGeneration(path, time.Minute, 0, 1) }},
		{name: "preload", open: func(path string) error { return engine.PreloadForGeneration(path, time.Minute, 1) }},
	} {
		t.Run(op.name, func(t *testing.T) {
			err := op.open("https://www.youtube.com/watch?v=abc")
			if err == nil || !strings.Contains(err.Error(), "fake yt-dlp ran") {
				t.Fatalf("open error = %v, want the error of the fake yt-dlp", err)
			}
		})
	}
}

// fakeRadio is a provider with no optional capability.
type fakeRadio struct{}

func (fakeRadio) Name() string                                { return "Radio" }
func (fakeRadio) Playlists() ([]playlist.PlaylistInfo, error) { return nil, nil }
func (fakeRadio) Tracks(string) ([]playlist.Track, error)     { return nil, nil }

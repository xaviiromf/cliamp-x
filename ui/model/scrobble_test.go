package model

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/scrobble"
	"github.com/xaviiromf/cliamp-x/ipc"
	"github.com/xaviiromf/cliamp-x/playlist"
)

func TestScrobbleTrack(t *testing.T) {
	song := playlist.Track{Path: "https://music.youtube.com/watch?v=x", Title: "One More Time", Artist: "Daft Punk", Album: "Discovery", DurationSecs: 320}
	tests := []struct {
		name  string
		track playlist.Track
		dur   time.Duration
		want  scrobble.Track
		ok    bool
	}{
		{"metadata duration", song, 0, scrobble.Track{Artist: "Daft Punk", Title: "One More Time", Album: "Discovery", Duration: 320 * time.Second}, true},
		{"engine duration wins", song, 300 * time.Second, scrobble.Track{Artist: "Daft Punk", Title: "One More Time", Album: "Discovery", Duration: 300 * time.Second}, true},
		{"no artist", playlist.Track{Title: "Untitled"}, time.Minute, scrobble.Track{}, false},
		{"no title", playlist.Track{Artist: "Band"}, time.Minute, scrobble.Track{}, false},
		{"radio stream", playlist.Track{Title: "Live", Artist: "Station", Stream: true}, 0, scrobble.Track{}, false},
	}
	for _, tt := range tests {
		got, ok := scrobbleTrack(tt.track, tt.dur)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: scrobbleTrack = %+v, %v; want %+v, %v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

// Nothing is queued while no service is on, or for a song too short to
// count; the snapshot reports the services that are on.
func TestScrobbleGates(t *testing.T) {
	m, _ := newActionTestModel(t)
	m.reports = &reportQueue{}
	m.scrobbler = scrobble.New(scrobble.Config{LastfmKey: "k", LastfmSecret: "s"}, filepath.Join(t.TempDir(), "queue.json"))
	song := playlist.Track{Title: "Song", Artist: "Band", DurationSecs: 200}
	idle := func() bool {
		m.reports.mu.Lock()
		defer m.reports.mu.Unlock()
		return !m.reports.running && len(m.reports.pending) == 0
	}
	m.scrobbleNowPlaying(song)
	m.scrobbleListen(song, 150*time.Second, 200*time.Second)
	if !idle() {
		t.Fatal("reports queued while Last.fm has no session")
	}
	if lb, lfm := m.scrobbleServices(); lb || lfm {
		t.Fatalf("services = %v, %v; want none", lb, lfm)
	}
	m.scrobbler.SetConfig(scrobble.Config{ListenBrainzToken: "tok"})
	m.scrobbleListen(playlist.Track{Title: "Jingle", Artist: "Band"}, 20*time.Second, 25*time.Second)
	if !idle() {
		t.Fatal("a 25 s song was scrobbled")
	}
	if s := m.runtimeSnapshot(); !s.ScrobbleListenBrainz || s.ScrobbleLastfm {
		t.Fatalf("snapshot services = %v, %v; want ListenBrainz only", s.ScrobbleListenBrainz, s.ScrobbleLastfm)
	}
}

// lastfm.auth needs the API key and secret, a known step, and the token for
// finish; it is unavailable without a scrobbler.
func TestNativeLastfmAuthParams(t *testing.T) {
	keys := scrobble.Config{LastfmKey: "k", LastfmSecret: "s"}
	for _, tc := range []struct {
		name    string
		cfg     *scrobble.Config
		request ipc.Request
		wantErr string
	}{
		{name: "no scrobbler", request: ipc.Request{Name: "start"}, wantErr: ipc.V2ErrorCodeUnavailable},
		{name: "no API key", cfg: &scrobble.Config{}, request: ipc.Request{Name: "start"}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "unknown step", cfg: &keys, request: ipc.Request{Name: "restart"}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "finish without a token", cfg: &keys, request: ipc.Request{Name: "finish", Text: " "}, wantErr: ipc.V2ErrorCodeInvalidParams},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newActionTestModel(t)
			if tc.cfg != nil {
				m.scrobbler = scrobble.New(*tc.cfg, filepath.Join(t.TempDir(), "queue.json"))
			}
			if response := runV2(t, m, "lastfm.auth", tc.request); response.OK || !strings.HasPrefix(response.Error, tc.wantErr) {
				t.Fatalf("lastfm.auth = %+v, want %s", response, tc.wantErr)
			}
		})
	}
}

package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/favorites"
	"github.com/xaviiromf/cliamp-x/internal/offline"
	"github.com/xaviiromf/cliamp-x/internal/playback"
	"github.com/xaviiromf/cliamp-x/internal/stats"
	"github.com/xaviiromf/cliamp-x/ipc"
	"github.com/xaviiromf/cliamp-x/player"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
	"github.com/xaviiromf/cliamp-x/theme"
	"github.com/xaviiromf/cliamp-x/ui"
)

// The tests in this file cover the IPC additions that the native Android UI
// reads.

// normalizingEngine is a player fake with the normalization of the real
// player.
type normalizingEngine struct {
	playbackFakeEngine
	mode player.NormalizeMode
}

func (e *normalizingEngine) SetNormalize(mode player.NormalizeMode, _ float64) { e.mode = mode }
func (e *normalizingEngine) Normalize() (player.NormalizeMode, float64)        { return e.mode, 0 }

// toneEngine fills the audio taps with a sine at freq, or silence at 0.
type toneEngine struct {
	playbackFakeEngine
	freq float64
}

func (e *toneEngine) SamplesInto(dst []float64) int         { return e.fill(dst) }
func (e *toneEngine) WaveformSamplesInto(dst []float64) int { return e.fill(dst) }

func (e *toneEngine) fill(dst []float64) int {
	for i := range dst {
		dst[i] = 0.5 * math.Sin(2*math.Pi*e.freq*float64(i)/44100)
	}
	return len(dst)
}

// snapshotJSON returns the runtime snapshot of m as a client decodes it.
func snapshotJSON(t *testing.T, m *Model) map[string]any {
	t.Helper()
	data, err := json.Marshal(m.runtimeSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// Each snapshot field reaches the JSON under its name, differs at rest,
// and changes the fingerprint, so its change publishes runtime.state.
func TestNativeSnapshotFields(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  func(*Model, *normalizingEngine)
		want map[string]any
	}{
		{name: "buffering", set: func(m *Model, _ *normalizingEngine) { m.buffering = true }, want: map[string]any{"buffering": true}},
		{
			name: "seek in flight",
			set:  func(m *Model, _ *normalizingEngine) { m.seek.active, m.seek.targetPos = true, 90*time.Second },
			want: map[string]any{"seeking": true, "position": 90.0},
		},
		{
			name: "play error",
			set: func(m *Model, _ *normalizingEngine) {
				m.setPlayError("/music/one.flac", errors.New("yt-dlp: ERROR: [youtube] abc: Video unavailable"))
			},
			want: map[string]any{"error": "yt-dlp: ERROR: [youtube] abc: Video unavailable", "error_kind": "unavailable", "error_path": "/music/one.flac"},
		},
		{
			name: "notice",
			set: func(m *Model, _ *normalizingEngine) {
				m.status.ShowAtKind(time.Now(), feedbackWarning, "Careful", statusTTLDefault)
			},
			want: map[string]any{"notice": map[string]any{"text": "Careful", "kind": "warning"}},
		},
		{
			name: "activity notice",
			set:  func(m *Model, _ *normalizingEngine) { m.status.ShowAtKind(time.Now(), feedbackActivity, "Loading", 0) },
			want: map[string]any{"notice": map[string]any{"text": "Loading", "kind": "activity"}},
		},
		{
			name: "sleep timer",
			set: func(m *Model, _ *normalizingEngine) {
				m.sleep = sleepTimer{minutes: 30, at: time.Unix(1700000000, 0)}
			},
			want: map[string]any{"sleep_minutes": 30.0, "sleep_at": 1700000000.0},
		},
		{name: "sleep at the end of the song", set: func(m *Model, _ *normalizingEngine) { m.sleep = sleepTimer{minutes: -1} }, want: map[string]any{"sleep_minutes": -1.0}},
		{name: "auto radio", set: func(m *Model, _ *normalizingEngine) { m.autoRadio = true }, want: map[string]any{"auto_radio": true}},
		{name: "normalize", set: func(_ *Model, e *normalizingEngine) { e.mode = player.NormalizeAlbum }, want: map[string]any{"normalize": "album"}},
		{name: "crossfade", set: func(m *Model, _ *normalizingEngine) { m.crossfade = 4 * time.Second }, want: map[string]any{"crossfade": 4.0}},
		{name: "auto EQ", set: func(m *Model, _ *normalizingEngine) { m.eqAuto = true }, want: map[string]any{"eq_auto": true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &normalizingEngine{mode: player.NormalizeOff}
			engine.playing, engine.position = true, 10*time.Second
			pl := playlist.New()
			pl.Add(playlist.Track{Path: "/music/one.flac", Title: "One"})
			m := Model{player: engine, playlist: pl, vis: ui.NewVisualizer(44100)}
			before, beforePrint := snapshotJSON(t, &m), m.runtimeFingerprint()
			if before["normalize"] != "off" {
				t.Fatalf("normalize at rest = %v, want off", before["normalize"])
			}

			tc.set(&m, engine)
			after := snapshotJSON(t, &m)
			for field, want := range tc.want {
				if reflect.DeepEqual(before[field], want) {
					t.Fatalf("%s = %v before the change, want another value", field, before[field])
				}
				if !reflect.DeepEqual(after[field], want) {
					t.Fatalf("%s = %#v, want %#v", field, after[field], want)
				}
			}
			if m.runtimeFingerprint() == beforePrint {
				t.Fatal("the fingerprint did not change")
			}
		})
	}
}

// An expired status line is no notice.
func TestNativeSnapshotNoticeExpires(t *testing.T) {
	m := Model{player: &playbackFakeEngine{}, playlist: playlist.New()}
	m.status.ShowAtKind(time.Now().Add(-time.Minute), feedbackSuccess, "Old", statusTTLDefault)
	if notice := m.runtimeSnapshot().Notice; notice != nil {
		t.Fatalf("notice = %+v, want none after it expired", notice)
	}
}

// The play error comes only from a failed start: a browse error of the
// hidden provider pane stays out of it. A start that works clears it.
func TestNativeSnapshotPlayError(t *testing.T) {
	stream := "https://music.youtube.com/watch?v=abc"
	prov := commandsTestProvider{name: "YouTube Music"}
	pl := playlist.New()
	pl.Add(playlist.Track{Path: "/music/a.mp3", Title: "A"}, playlist.Track{Path: stream, Title: "B", Stream: true})
	engine := &playbackFakeEngine{playing: true}
	m := Model{player: engine, playlist: pl, provider: prov, vis: ui.NewVisualizer(44100)}
	m.SetIPCBroker(ipc.NewBroker())
	update := func(msg tea.Msg) {
		t.Helper()
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	check := func(step, wantErr, wantKind, wantPath string) {
		t.Helper()
		s := m.runtimeSnapshot()
		if s.Error != wantErr || s.ErrorKind != wantKind || s.ErrorPath != wantPath {
			t.Fatalf("%s: error = %q kind %q path %q, want %q %q %q", step, s.Error, s.ErrorKind, s.ErrorPath, wantErr, wantKind, wantPath)
		}
	}

	update(playlistsLoadedMsg{providerName: prov.Name(), gen: m.requests.provider, err: errors.New("dial tcp: no such host")})
	if m.err == nil {
		t.Fatal("the browse error did not reach the pane")
	}
	check("browse error", "", "", "")

	update(playback.NextMsg{})
	revision := m.runtimeSnapshot().Revision
	failed := errors.New("yt-dlp: ERROR: [youtube] abc: Sign in to confirm you're not a bot")
	update(streamPlayedMsg{path: stream, gen: m.requests.stream, err: failed})
	check("stream start", failed.Error(), "session", stream)
	if m.runtimeSnapshot().Revision == revision {
		t.Fatal("the failed start did not publish runtime.state")
	}

	m.handleYTDLUnpauseReconnect(ytdlUnpauseReconnectMsg{err: errors.New("connection reset by peer"), gen: m.requests.stream, seekGen: m.seek.gen})
	check("unpause reconnect", "connection reset by peer", "network", stream)

	engine.playErr = fmt.Errorf("open: %w", playlist.ErrNeedsAuth)
	m.playIndex(0)
	check("sign-in", engine.playErr.Error(), "session", "/music/a.mp3")

	engine.playErr = nil
	m.playIndex(0)
	check("a start that works", "", "", "")
}

// A seek that restarts a decoder reports its target until it lands, and
// the landing publishes runtime.state.
func TestNativeSnapshotSeekInFlight(t *testing.T) {
	engine := &playbackFakeEngine{playing: true, seekable: true, ytdlSeek: true, position: 10 * time.Second, duration: 200 * time.Second}
	pl := playlist.New()
	pl.Add(playlist.Track{Path: "https://music.youtube.com/watch?v=abc", Title: "A", Stream: true})
	m := Model{player: engine, playlist: pl, vis: ui.NewVisualizer(44100)}
	m.SetIPCBroker(ipc.NewBroker())

	if response := runV2(t, &m, "seek.absolute", ipc.Request{Value: 90}); !response.OK {
		t.Fatalf("seek.absolute = %+v", response)
	}
	s := m.runtimeSnapshot()
	if !s.Seeking || s.Position != 90 {
		t.Fatalf("in flight: seeking %v position %v, want true 90", s.Seeking, s.Position)
	}
	inFlight := s.Revision

	engine.position = 90 * time.Second
	updated, _ := m.Update(seekTickMsg{gen: m.seek.gen, target: 90 * time.Second})
	m = updated.(Model)
	s = m.runtimeSnapshot()
	if s.Seeking || s.Position != 90 || s.Revision == inFlight {
		t.Fatalf("landed: seeking %v position %v revision %d (was %d), want false 90 and a new revision", s.Seeking, s.Position, s.Revision, inFlight)
	}
}

// spectrumBands sends spectrum.get through Update and returns the bands.
func spectrumBands(t *testing.T, m *Model) []float64 {
	t.Helper()
	reply := make(chan V2RequestResult, 1)
	updated, _ := m.Update(V2RequestMsg{Request: ipc.V2Request{Method: "spectrum.get"}, Reply: reply})
	*m = updated.(Model)
	result := <-reply
	var response ipc.Response
	if err := json.Unmarshal(result.Result.Result, &response); err != nil {
		t.Fatal(err)
	}
	return response.Bands
}

// With the drawing off, each spectrum.get analyzes the audio that plays
// now, whatever the hidden layout gives the visualizer, and nothing
// suspends the visualizer between requests.
func TestNativeSpectrumWhileRenderPaused(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
		setup         func(*Model)
	}{
		{name: "normal layout", width: 80, height: 24},
		{name: "provider screen", width: 80, height: 24, setup: func(m *Model) { m.StartInProvider() }},
		{name: "simplified", width: 80, height: 24, setup: func(m *Model) { m.SetSimplified(true) }},
		{name: "50x14", width: 50, height: 14},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CLIAMP_CONFIG_DIR", t.TempDir())
			engine := &toneEngine{freq: 440}
			engine.playing = true
			pl := playlist.New()
			pl.Add(playlist.Track{Path: "/music/one.flac", Title: "One"})
			local := commandsTestProvider{name: "Local"}
			m := New(engine, pl, []provider.Entry{{Key: "local", Name: "Local", Provider: local}}, "", nil, nil, nil, nil, nil, nil)
			m.termTitle.introActive = false
			updated, _ := m.Update(tea.WindowSizeMsg{Width: tc.width, Height: tc.height})
			m = updated.(Model)
			if tc.setup != nil {
				tc.setup(&m)
			}
			if response := runV2(t, &m, "ui.render", ipc.Request{Name: "off"}); !response.OK {
				t.Fatalf("ui.render off = %+v", response)
			}

			// A Suspend resets the analysis clock, so a request within
			// TickAnalyze of the last one would analyze the silence.
			now := time.Now()
			m.tickVisualizerForClient(now)
			want := slices.Clone(m.vis.SmoothedBands())
			if len(want) == 0 || slices.Max(want) <= 0 {
				t.Fatalf("bands = %v, want sound", want)
			}
			engine.freq = 0
			updated, _ = m.Update(tickMsg(now.Add(5 * time.Millisecond)))
			m = updated.(Model)
			updated, _ = m.Update(tea.FocusMsg{})
			m = updated.(Model)
			m.tickVisualizerForClient(now.Add(10 * time.Millisecond))
			if got := m.vis.SmoothedBands(); !slices.Equal(got, want) {
				t.Fatalf("bands after a frame tick and a message = %v, want the smoothing kept at %v", got, want)
			}

			engine.freq = 440
			time.Sleep(ui.TickAnalyze + 5*time.Millisecond)
			first := spectrumBands(t, &m)
			engine.freq = 5000
			time.Sleep(ui.TickAnalyze + 5*time.Millisecond)
			second := spectrumBands(t, &m)
			if len(first) == 0 || slices.Max(first) <= 0 || slices.Equal(first, second) {
				t.Fatalf("spectrum.get bands %v then %v, want sound that changes", first, second)
			}
		})
	}
}

// listAlbumProvider serves the same tracks as a playlist and as an album.
type listAlbumProvider struct {
	commandsTestProvider
	tracks []playlist.Track
}

func (p listAlbumProvider) Tracks(string) ([]playlist.Track, error)      { return p.tracks, nil }
func (p listAlbumProvider) AlbumTracks(string) ([]playlist.Track, error) { return p.tracks, nil }

// provider.load and provider.load_album play the track at index, or the
// first one for an index out of range, and reply with the first page only.
func TestNativeProviderLoadIndex(t *testing.T) {
	tracks := make([]playlist.Track, 250)
	for i := range tracks {
		tracks[i] = playlist.Track{Path: "/music/" + strconv.Itoa(i) + ".mp3", Title: strconv.Itoa(i)}
	}
	prov := listAlbumProvider{commandsTestProvider{name: "Albums"}, tracks}
	for _, tc := range []struct {
		name  string
		op    string
		index int
		want  int
	}{
		{name: "playlist from the start", op: "provider.load", want: 0},
		{name: "playlist at 3", op: "provider.load", index: 3, want: 3},
		{name: "playlist out of range", op: "provider.load", index: 250, want: 0},
		{name: "album at 3", op: "provider.load_album", index: 3, want: 3},
		{name: "album out of range", op: "provider.load_album", index: 999, want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &playbackFakeEngine{}
			m := Model{player: engine, playlist: playlist.New(), providers: []provider.Entry{{Key: "albums", Name: "Albums", Provider: prov}}}
			response := runV2(t, &m, tc.op, ipc.Request{Provider: "albums", Playlist: "Mix", Album: "MPREb_x", Index: tc.index})
			if !response.OK || len(response.Tracks) != 200 || response.Total != 250 || response.Index != tc.want {
				t.Fatalf("response = OK %v, %d tracks of %d, index %d; want 200 of 250 at %d", response.OK, len(response.Tracks), response.Total, response.Index, tc.want)
			}
			if want := tracks[tc.want].Path; len(engine.playCalls) != 1 || engine.playCalls[0] != want {
				t.Fatalf("played %v, want %s", engine.playCalls, want)
			}
		})
	}
}

// queue.replace puts the supplied tracks in the queue and plays the one at
// index. Empty input and a stale revision change nothing.
func TestNativeQueueReplace(t *testing.T) {
	infos := []ipc.TrackInfo{
		{Path: "/music/a.mp3", Title: "A", ProviderMeta: map[string]string{"ytmusic.id": "a"}},
		{Path: "/music/b.mp3", Title: "B"},
		{Path: "/music/c.mp3", Title: "C"},
	}
	for _, tc := range []struct {
		name      string
		params    func(revision uint64) ipc.Request
		wantError string
		wantPlay  string
	}{
		{name: "plays index 2", params: func(uint64) ipc.Request { return ipc.Request{Tracks: infos, Index: 2} }, wantPlay: "/music/c.mp3"},
		{name: "index out of range clamps", params: func(uint64) ipc.Request { return ipc.Request{Tracks: infos, Index: 9} }, wantPlay: "/music/c.mp3"},
		{name: "current revision", params: func(r uint64) ipc.Request { return ipc.Request{Tracks: infos, Index: 1, Revision: r} }, wantPlay: "/music/b.mp3"},
		{name: "empty input", params: func(uint64) ipc.Request { return ipc.Request{} }, wantError: ipc.V2ErrorCodeInvalidParams},
		{name: "too many tracks", params: func(uint64) ipc.Request { return ipc.Request{Tracks: make([]ipc.TrackInfo, 1001)} }, wantError: ipc.V2ErrorCodeInvalidParams},
		{name: "stale revision", params: func(r uint64) ipc.Request { return ipc.Request{Tracks: infos, Revision: r + 1} }, wantError: ipc.V2ErrorCodeConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &playbackFakeEngine{}
			pl := playlist.New()
			pl.Add(playlist.Track{Path: "/music/old.mp3", Title: "Old"})
			m := Model{player: engine, playlist: pl}
			response := runV2(t, &m, "queue.replace", tc.params(pl.Revision()))
			if tc.wantError != "" {
				if response.OK || response.Error != tc.wantError || pl.Len() != 1 || len(engine.playCalls) != 0 {
					t.Fatalf("response = %+v, queue of %d, played %v; want %s and no change", response, pl.Len(), engine.playCalls, tc.wantError)
				}
				return
			}
			if !response.OK || response.Total != 3 || len(response.Tracks) != 3 || response.Tracks[0].ProviderMeta["ytmusic.id"] != "a" {
				t.Fatalf("response = %+v, want the 3 new tracks", response)
			}
			if len(engine.playCalls) != 1 || engine.playCalls[0] != tc.wantPlay || response.Index != m.playlist.Index() {
				t.Fatalf("played %v at index %d, want %s", engine.playCalls, response.Index, tc.wantPlay)
			}
		})
	}
}

// queue.list with the sort upnext lists the tracks after the playing one in
// play order, with their live playlist indexes.
func TestNativeQueueListUpNext(t *testing.T) {
	for _, tc := range []struct {
		name        string
		setup       func(*playlist.Playlist)
		offset      int
		wantIndexes []int
		wantTotal   int
	}{
		{name: "in order", setup: func(p *playlist.Playlist) { p.SetIndex(1) }, wantIndexes: []int{2, 3, 4}, wantTotal: 3},
		{name: "with an offset", setup: func(p *playlist.Playlist) { p.SetIndex(1) }, offset: 1, wantIndexes: []int{3, 4}, wantTotal: 3},
		{name: "last track", setup: func(p *playlist.Playlist) { p.SetIndex(4) }, wantTotal: 0},
		{
			name: "after a play-next track",
			setup: func(p *playlist.Playlist) {
				p.SetIndex(1)
				p.Queue(4)
				p.Next()
			},
			wantIndexes: []int{2, 3, 4}, wantTotal: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pl := playlist.New()
			for i := range 5 {
				pl.Add(playlist.Track{Path: "/music/" + strconv.Itoa(i) + ".mp3"})
			}
			tc.setup(pl)
			m := Model{player: &playbackFakeEngine{}, playlist: pl}
			response := runV2(t, &m, "queue.list", ipc.Request{Sort: "upnext", Offset: tc.offset})
			var got []int
			for _, track := range response.Tracks {
				got = append(got, track.Index)
			}
			if !response.OK || !slices.Equal(got, tc.wantIndexes) || response.Total != tc.wantTotal {
				t.Fatalf("up next = %v of %d, want %v of %d", got, response.Total, tc.wantIndexes, tc.wantTotal)
			}
		})
	}

	// With shuffle on, the list follows the shuffle order.
	pl := playlist.New()
	for i := range 8 {
		pl.Add(playlist.Track{Path: "/music/" + strconv.Itoa(i) + ".mp3"})
	}
	pl.SetIndex(3)
	pl.ToggleShuffle()
	order, _ := pl.OrderWindow(0, pl.Len())
	m := Model{player: &playbackFakeEngine{}, playlist: pl}
	response := runV2(t, &m, "queue.list", ipc.Request{Sort: "upnext", Limit: 3})
	var got []int
	for _, track := range response.Tracks {
		got = append(got, track.Index)
	}
	if !slices.Equal(got, order[1:4]) || response.Total != 7 || response.Index != 3 {
		t.Fatalf("shuffled up next = %v of %d at %d, want %v of 7 at 3", got, response.Total, response.Index, order[1:4])
	}
	if response := runV2(t, &m, "queue.list", ipc.Request{}); len(response.Tracks) != 8 || response.Tracks[0].Index != 0 {
		t.Fatalf("queue.list without a sort = %+v, want the live playlist in track order", response)
	}
}

// eq list names every built-in preset, then Custom, and the active preset.
func TestNativeEQList(t *testing.T) {
	m := Model{player: &playbackFakeEngine{}, playlist: playlist.New(), eqPresetIdx: 1}
	response := runV2(t, &m, "eq", ipc.Request{Name: "list"})
	if !response.OK || len(response.Items) != len(eqPresets)+1 || response.Items[0] != eqPresets[0].Name ||
		response.Items[len(eqPresets)] != "Custom" || response.EQPreset != eqPresets[1].Name {
		t.Fatalf("eq list = %+v", response)
	}
}

// signedInProvider reports a fixed sign-in state.
type signedInProvider struct {
	commandsTestProvider
	signedIn bool
}

func (p signedInProvider) SignedIn() bool { return p.signedIn }

// provider.list reports signed_in only for a provider with an account.
func TestNativeProviderListSignedIn(t *testing.T) {
	m := Model{player: &playbackFakeEngine{}, playlist: playlist.New(), providers: []provider.Entry{
		{Key: "ytmusic", Name: "YouTube Music", Provider: signedInProvider{commandsTestProvider{name: "YouTube Music"}, true}},
		{Key: "out", Name: "Signed out", Provider: signedInProvider{commandsTestProvider{name: "Signed out"}, false}},
		{Key: "local", Name: "Local", Provider: commandsTestProvider{name: "Local"}},
	}}
	response := runV2(t, &m, "provider.list", ipc.Request{})
	if len(response.Providers) != 3 {
		t.Fatalf("providers = %+v", response.Providers)
	}
	for i, want := range []*bool{new(true), new(false), nil} {
		if got := response.Providers[i].SignedIn; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s signed_in = %v, want %v", response.Providers[i].Key, got, want)
		}
	}
}

// The snapshot carries the colors the cover theme shows, and a new cover
// palette publishes. theme list gives every theme with its colors.
func TestNativeThemePalette(t *testing.T) {
	t.Cleanup(func() { applyThemeAll(theme.Default()) })
	red := theme.Theme{Name: "cover", BG: "#100000", Accent: "#ff0000", BrightFG: "#ffffff", FG: "#cccccc", Green: "#00ff00", Yellow: "#ffff00", Red: "#ff0000"}
	blue := red
	blue.BG, blue.Accent = "#000010", "#0000ff"
	dracula := theme.Theme{Name: "Dracula", BG: "#282a36", Accent: "#bd93f9", BrightFG: "#f8f8f2", FG: "#f8f8f2", Green: "#50fa7b", Yellow: "#f1fa8c", Red: "#ff5555"}
	m := Model{player: &playbackFakeEngine{}, playlist: playlist.New(), themes: []theme.Theme{{Name: "Cover"}, dracula}, themeIdx: 0}

	if got := m.runtimeSnapshot().Theme; got.Name != "Cover" || got.BG != "" {
		t.Fatalf("cover theme with no cover = %+v, want the entry", got)
	}
	m.startCoverFade(red, time.Now())
	if got := m.runtimeSnapshot().Theme; got.Name != "Cover" || got.BG != red.BG || got.Accent != red.Accent || got.Red != red.Red {
		t.Fatalf("cover theme = %+v, want the colors of the shown cover", got)
	}
	shown := m.runtimeFingerprint()
	m.startCoverFade(blue, time.Now())
	if got := m.runtimeSnapshot().Theme; !m.coverFade.active || got.Accent != blue.Accent {
		t.Fatalf("fading cover theme = %+v, want the colors it fades to", got)
	}
	if m.runtimeFingerprint() == shown {
		t.Fatal("a new cover palette did not change the fingerprint")
	}

	response := runV2(t, &m, "theme", ipc.Request{Name: "list"})
	want := []ipc.ThemeInfo{{Name: theme.DefaultName}, {Name: "Cover"}, ipcThemeInfo(dracula)}
	if !response.OK || !reflect.DeepEqual(response.Themes, want) || len(response.Items) != 3 {
		t.Fatalf("theme list = %+v, want themes %+v", response, want)
	}
}

// favoriteTestProvider toggles favorites in memory.
type favoriteTestProvider struct {
	commandsTestProvider
	mu  sync.Mutex
	ids map[string]bool
}

func (p *favoriteTestProvider) ToggleFavorite(id string) (bool, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ids[id] = !p.ids[id]
	return p.ids[id], id, nil
}

// playlist.create returns the new ID and provider.favorite the new state.
func TestNativeResultsCarryState(t *testing.T) {
	writable := &writableTestProvider{commandsTestProvider: commandsTestProvider{name: "Writable"}}
	favs := &favoriteTestProvider{commandsTestProvider: commandsTestProvider{name: "Favs"}, ids: map[string]bool{}}
	m := Model{player: &playbackFakeEngine{}, playlist: playlist.New(), providers: []provider.Entry{
		{Key: "writable", Name: "Writable", Provider: writable},
		{Key: "favs", Name: "Favs", Provider: favs},
	}}
	if response := runV2(t, &m, "playlist.create", ipc.Request{Provider: "writable", Playlist: "Road trip"}); !response.OK || response.Playlist != "Road trip" {
		t.Fatalf("playlist.create = %+v, want the new ID", response)
	}
	for _, want := range []bool{true, false} {
		response := runV2(t, &m, "provider.favorite", ipc.Request{Provider: "favs", Playlist: "MPREb_x"})
		if !response.OK || response.Favorite == nil || *response.Favorite != want {
			t.Fatalf("provider.favorite = %+v, want favorite %v", response, want)
		}
	}
}

// genreTestBrowser is a category catalog: the genres, or the moods.
type genreTestBrowser struct{ moods bool }

func (b genreTestBrowser) Genres() ([]provider.GenreInfo, error) {
	if b.moods {
		return []provider.GenreInfo{{ID: "calm", Name: "Calm", Group: "Moods"}}, nil
	}
	return []provider.GenreInfo{{ID: "rock", Name: "Rock", Group: "Genres", Favorite: true}, {ID: "search:Shoegaze", Name: "Shoegaze", Group: "More"}}, nil
}

func (b genreTestBrowser) GenreSortTypes() []provider.SortType {
	return []provider.SortType{{ID: "mix", Label: "Mix"}, {ID: "songs", Label: "Songs"}}
}

func (b genreTestBrowser) GenreTracks(id, sort string) ([]playlist.Track, error) {
	tracks := make([]playlist.Track, 250)
	for i := range tracks {
		tracks[i] = playlist.Track{Path: fmt.Sprintf("/music/%s-%s-%d.mp3", id, sort, i)}
	}
	return tracks, nil
}

// browseTestProvider has every browse capability that the native UI reads.
type browseTestProvider struct {
	commandsTestProvider
	genreTestBrowser
	mu        sync.Mutex
	refreshed int
	pages     [][2]int // the offset and size of each ArtistList call
	genreFavs map[string]bool
}

func (p *browseTestProvider) Tracks(string) ([]playlist.Track, error) { return nil, nil }

func (p *browseTestProvider) HomeShelves() ([]provider.Shelf, error) {
	song := playlist.Track{Path: "/music/fav.mp3", Title: "Fav"}
	return []provider.Shelf{{Title: "Picks", Items: []provider.ShelfItem{
		{Kind: "song", ID: "v1", Title: "Fav", Subtitle: "Band", ImageURL: "song.jpg", Track: &song},
		{Kind: "album", ID: "MPREb_a", Title: "Record", Subtitle: "Band", ImageURL: "album.jpg"},
	}}}, nil
}

func (p *browseTestProvider) GenreBrowserFor(entryID string) provider.GenreBrowser {
	return genreTestBrowser{moods: entryID == "browse:moods"}
}

func (p *browseTestProvider) ToggleGenreFavorite(id string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.genreFavs[id] = !p.genreFavs[id]
	return p.genreFavs[id], nil
}

func (p *browseTestProvider) Artists() ([]provider.ArtistInfo, error) {
	return []provider.ArtistInfo{{ID: "UCa", Name: "♥ Band", ImageURL: "band.jpg"}, {ID: "UCb", Name: "Other"}}, nil
}

func (p *browseTestProvider) ArtistAlbums(string) ([]provider.AlbumInfo, error) {
	return []provider.AlbumInfo{{ID: "MPREb_a", Name: "♥ Record", ImageURL: "album.jpg"}, {ID: "MPREb_s", Name: "Hit · Singles", Single: true}}, nil
}

func (p *browseTestProvider) ArtistList(offset, size int) ([]provider.ArtistInfo, error) {
	p.mu.Lock()
	p.pages = append(p.pages, [2]int{offset, size})
	p.mu.Unlock()
	return []provider.ArtistInfo{{ID: fmt.Sprintf("UC%d", offset), Name: "Found"}}, nil
}

func (p *browseTestProvider) AddTrackToPlaylist(context.Context, string, playlist.Track) error {
	return nil
}

func (p *browseTestProvider) CanAddToPlaylist(list playlist.PlaylistInfo) bool {
	return strings.HasPrefix(list.ID, "pl:")
}

func (p *browseTestProvider) Refresh() {
	p.mu.Lock()
	p.refreshed++
	p.mu.Unlock()
}

func (p *browseTestProvider) PreviewDetails(_ context.Context, id string) (provider.PreviewDetails, error) {
	return provider.PreviewDetails{ImageURL: id + ".jpg", Lines: []string{"", "2024", " ", "Band"}}, nil
}

func (p *browseTestProvider) FavoriteState(id string) (bool, bool) {
	return id == "MPREb_saved", strings.HasPrefix(id, "MPREb_")
}

func newBrowseTestModel() (*Model, *browseTestProvider) {
	prov := &browseTestProvider{
		commandsTestProvider: commandsTestProvider{name: "Browse", lists: []playlist.PlaylistInfo{
			{ID: "pl:mine", Name: "Mine", Section: "Yours", ImageURL: "mine.jpg"},
			{ID: "al:MPREb_n", Name: "● New · Band", Section: "New"},
		}},
		genreFavs: map[string]bool{},
	}
	return &Model{
		player: &playbackFakeEngine{}, playlist: playlist.New(), favSet: map[string]struct{}{"/music/fav.mp3": {}},
		providers: []provider.Entry{{Key: "yt", Name: "Browse", Provider: prov}, {Key: "bare", Name: "Bare", Provider: commandsTestProvider{name: "Bare"}}},
	}, prov
}

// Rows carry their picture and flags, and keep their names for the CLI.
func TestNativeBrowseRowFlags(t *testing.T) {
	m, _ := newBrowseTestModel()
	lists := runV2(t, m, "provider.playlists", ipc.Request{Provider: "yt"})
	want := []ipc.PlaylistInfo{
		{ID: "pl:mine", Name: "Mine", Provider: "yt", Section: "Yours", ImageURL: "mine.jpg", Writable: true},
		{ID: "al:MPREb_n", Name: "● New · Band", Provider: "yt", Section: "New", Unseen: true},
	}
	if !lists.OK || !reflect.DeepEqual(lists.Playlists, want) {
		t.Fatalf("playlists = %+v, want %+v", lists.Playlists, want)
	}
	albums := runV2(t, m, "provider.artist_albums", ipc.Request{Provider: "yt", Artist: "UCa"})
	wantAlbums := []ipc.AlbumInfo{{ID: "MPREb_a", Name: "♥ Record", ImageURL: "album.jpg", Favorite: true}, {ID: "MPREb_s", Name: "Hit · Singles", Single: true}}
	if !albums.OK || !reflect.DeepEqual(albums.Albums, wantAlbums) {
		t.Fatalf("albums = %+v, want %+v", albums.Albums, wantAlbums)
	}
	artists := runV2(t, m, "provider.artists", ipc.Request{Provider: "yt"})
	wantArtists := []ipc.ArtistInfo{{ID: "UCa", Name: "♥ Band", ImageURL: "band.jpg", Favorite: true}, {ID: "UCb", Name: "Other"}}
	if !artists.OK || !reflect.DeepEqual(artists.Artists, wantArtists) || artists.Total != 2 {
		t.Fatalf("artists = %+v, want %+v", artists, wantArtists)
	}
}

// provider.home gives the shelves with each song's track and its ♥.
func TestNativeBrowseHome(t *testing.T) {
	m, _ := newBrowseTestModel()
	response := runV2(t, m, "provider.home", ipc.Request{Provider: "yt"})
	want := []ipc.ShelfInfo{{Title: "Picks", Items: []ipc.ShelfItemInfo{
		{Kind: "song", ID: "v1", Title: "Fav", Subtitle: "Band", ImageURL: "song.jpg", Track: &ipc.TrackInfo{Path: "/music/fav.mp3", Title: "Fav", Bookmark: true}},
		{Kind: "album", ID: "MPREb_a", Title: "Record", Subtitle: "Band", ImageURL: "album.jpg"},
	}}}
	if !response.OK || response.Total != 1 || !reflect.DeepEqual(response.Shelves, want) {
		t.Fatalf("home = %+v, want %+v", response, want)
	}
}

// provider.genres lists the genres, or the catalog of a browse entry, with
// the sorts; genre_tracks pages a genre and genre_favorite toggles its star.
func TestNativeBrowseGenres(t *testing.T) {
	m, prov := newBrowseTestModel()
	genres := runV2(t, m, "provider.genres", ipc.Request{Provider: "yt"})
	want := []ipc.GenreInfo{{ID: "rock", Name: "Rock", Group: "Genres", Favorite: true}, {ID: "search:Shoegaze", Name: "Shoegaze", Group: "More"}}
	if !genres.OK || !reflect.DeepEqual(genres.Genres, want) || genres.Total != 2 || len(genres.Sorts) != 2 || genres.Sorts[0] != (ipc.SortInfo{ID: "mix", Label: "Mix"}) {
		t.Fatalf("genres = %+v, want %+v with the sorts", genres, want)
	}
	moods := runV2(t, m, "provider.genres", ipc.Request{Provider: "yt", Playlist: "browse:moods"})
	if !moods.OK || len(moods.Genres) != 1 || moods.Genres[0].ID != "calm" {
		t.Fatalf("moods = %+v", moods)
	}

	tracks := runV2(t, m, "provider.genre_tracks", ipc.Request{Provider: "yt", Playlist: "rock", Sort: "songs", Offset: 200, Limit: 200})
	if !tracks.OK || tracks.Total != 250 || len(tracks.Tracks) != 50 || tracks.Tracks[0].Path != "/music/rock-songs-200.mp3" {
		t.Fatalf("genre tracks = OK %v, %d of %d", tracks.OK, len(tracks.Tracks), tracks.Total)
	}

	for _, wantOn := range []bool{true, false} {
		response := runV2(t, m, "provider.genre_favorite", ipc.Request{Provider: "yt", Playlist: "rock"})
		if !response.OK || response.Favorite == nil || *response.Favorite != wantOn {
			t.Fatalf("genre_favorite = %+v, want favorite %v", response, wantOn)
		}
	}
	if response := runV2(t, m, "provider.genre_favorite", ipc.Request{Provider: "yt"}); response.OK || len(prov.genreFavs) != 1 {
		t.Fatalf("genre_favorite with no genre = %+v, favorites %v; want a failure", response, prov.genreFavs)
	}
}

// provider.artists with the sort foryou pages the endless list: total is
// the page length, the default page has 40 rows and a page at most 200.
func TestNativeBrowseArtistsForYou(t *testing.T) {
	m, prov := newBrowseTestModel()
	for _, tc := range []struct {
		offset, limit int
		want          [2]int
	}{
		{offset: 0, want: [2]int{0, 40}},
		{offset: 40, limit: 20, want: [2]int{40, 20}},
		{offset: 60, limit: 500, want: [2]int{60, 200}},
	} {
		response := runV2(t, m, "provider.artists", ipc.Request{Provider: "yt", Sort: "foryou", Offset: tc.offset, Limit: tc.limit})
		if !response.OK || response.Total != 1 || len(response.Artists) != 1 {
			t.Fatalf("foryou artists = %+v", response)
		}
		if got := prov.pages[len(prov.pages)-1]; got != tc.want {
			t.Fatalf("ArtistList(%d, %d), want %v", got[0], got[1], tc.want)
		}
	}
}

// provider.refresh drops the caches; provider.preview gives the details and
// the saved state when the provider knows it.
func TestNativeBrowseRefreshAndPreview(t *testing.T) {
	m, prov := newBrowseTestModel()
	if response := runV2(t, m, "provider.refresh", ipc.Request{Provider: "yt"}); !response.OK || prov.refreshed != 1 {
		t.Fatalf("refresh = %+v, refreshed %d", response, prov.refreshed)
	}
	for _, tc := range []struct {
		id       string
		favorite *bool
	}{
		{id: "MPREb_saved", favorite: new(true)},
		{id: "MPREb_other", favorite: new(false)},
		{id: "pl:VLx"},
	} {
		response := runV2(t, m, "provider.preview", ipc.Request{Provider: "yt", Playlist: tc.id})
		want := &ipc.PreviewInfo{ImageURL: tc.id + ".jpg", Lines: []string{"2024", "Band"}, Favorite: tc.favorite}
		if !response.OK || !reflect.DeepEqual(response.Preview, want) {
			t.Fatalf("preview %s = %+v, want %+v", tc.id, response.Preview, want)
		}
	}
}

// Every browse operation is registered and reaches its handler: a provider
// without the capability gets the handler's error, not unavailable.
func TestNativeBrowseRouting(t *testing.T) {
	registry := ipc.DefaultOperationRegistry()
	for _, op := range []string{"provider.home", "provider.genres", "provider.genre_tracks", "provider.genre_favorite", "provider.refresh", "provider.preview"} {
		if _, ok := registry.Lookup(op); !ok {
			t.Errorf("%s is not registered", op)
		}
		m, _ := newBrowseTestModel()
		response := runV2(t, m, op, ipc.Request{Provider: "bare", Playlist: "x"})
		if response.OK || !strings.Contains(response.Error, "provider does not support") {
			t.Errorf("%s on a bare provider = %+v, want the handler's error", op, response)
		}
	}
}

// newActionTestModel has a taste store, a ♥ store, the download manifest
// and a play log in a temp directory, and two queued songs, the first
// playing.
func newActionTestModel(t *testing.T) (*Model, *playbackFakeEngine) {
	t.Helper()
	dir := t.TempDir()
	engine := &playbackFakeEngine{playing: true}
	pl := playlist.New()
	pl.Add(playlist.Track{Path: "/music/one.mp3", Title: "One"}, playlist.Track{Path: "/music/two.mp3", Title: "Two"})
	pl.SetIndex(0)
	m := &Model{player: engine, playlist: pl, favStore: favorites.NewAt(filepath.Join(dir, "favorites.toml"))}
	m.SetTaste(taste.NewAt(filepath.Join(dir, "taste.json")))
	m.SetOffline(offline.OpenManifest(filepath.Join(dir, "offline.json")), filepath.Join(dir, "downloads"))
	m.SetStatsLog(stats.Open(filepath.Join(dir, "plays.jsonl")))
	return m, engine
}

// taste.rate rates each kind, skips the playing song on "not interested",
// forgets a rating with 0 by ID alone, and changes nothing on bad input.
func TestNativeTasteRate(t *testing.T) {
	one := &ipc.TrackInfo{Path: "/music/one.mp3", Title: "One", Artist: "Band"}
	other := &ipc.TrackInfo{Path: "/music/other.mp3", Title: "Other"}
	for _, tc := range []struct {
		name     string
		before   string // an ID rated interested first
		request  ipc.Request
		id       string
		want     int
		wantErr  string
		wantNext bool // the playing song is skipped
	}{
		{name: "song interested", request: ipc.Request{Kind: "song", Track: other, Value: 1}, id: other.Path, want: 1},
		{name: "playing song not interested", request: ipc.Request{Kind: "song", Track: one, Value: -1}, id: one.Path, want: -1, wantNext: true},
		{name: "other song not interested", request: ipc.Request{Kind: "song", Track: other, Value: -1}, id: other.Path, want: -1},
		{name: "album", request: ipc.Request{Kind: "album", Playlist: "MPREb_a", Name: "Record", Value: 1}, id: "MPREb_a", want: 1},
		{name: "artist", request: ipc.Request{Kind: "artist", Playlist: "UCa", Name: "Band", Value: -1}, id: "UCa", want: -1},
		{name: "genre", request: ipc.Request{Kind: "genre", Playlist: "genre:rock", Name: "Rock", Value: 1}, id: "genre:rock", want: 1},
		{name: "undo by id without a track", before: one.Path, request: ipc.Request{Kind: "song", Playlist: one.Path}, id: one.Path},
		{name: "undo by track", before: one.Path, request: ipc.Request{Kind: "song", Track: one}, id: one.Path},
		{name: "undo an album", before: "MPREb_a", request: ipc.Request{Kind: "album", Playlist: "MPREb_a"}, id: "MPREb_a"},
		{name: "value 2", request: ipc.Request{Kind: "album", Playlist: "MPREb_a", Value: 2}, id: "MPREb_a", wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "half a value", request: ipc.Request{Kind: "album", Playlist: "MPREb_a", Value: 0.5}, id: "MPREb_a", wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "a playlist cannot be rated", request: ipc.Request{Kind: "playlist", Playlist: "pl:x", Value: 1}, id: "pl:x", wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "song without a track", request: ipc.Request{Kind: "song", Playlist: one.Path, Value: 1}, id: one.Path, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "album without an id", request: ipc.Request{Kind: "album", Value: 1}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "undo without an id", request: ipc.Request{Kind: "artist"}, wantErr: ipc.V2ErrorCodeInvalidParams},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newActionTestModel(t)
			if tc.before != "" {
				if _, err := m.taste.Rate(tc.before, taste.Item{Kind: taste.Song, Name: "x"}, taste.Interested); err != nil {
					t.Fatal(err)
				}
			}
			response := runV2(t, m, "taste.rate", tc.request)
			if tc.wantErr != "" {
				if response.OK || response.Error != tc.wantErr || len(m.taste.Ratings()) != 0 {
					t.Fatalf("response = %+v, ratings %+v; want %s and no rating", response, m.taste.Ratings(), tc.wantErr)
				}
				return
			}
			if !response.OK || m.taste.Interest(tc.id) != tc.want {
				t.Fatalf("response = %+v, interest in %s = %d; want %d", response, tc.id, m.taste.Interest(tc.id), tc.want)
			}
			if skipped := m.playlist.Index() == 1; skipped != tc.wantNext {
				t.Fatalf("playing index %d, want the song skipped: %v", m.playlist.Index(), tc.wantNext)
			}
			// On a song, interested is the ♥ like.
			if tc.request.Kind == "song" && tc.want != 0 && m.favStore.IsFavorited(tc.id) != (tc.want > 0) {
				t.Fatalf("♥ of %s = %v after %d", tc.id, m.favStore.IsFavorited(tc.id), tc.want)
			}
		})
	}

	m, _ := newActionTestModel(t)
	m.taste = nil
	if response := runV2(t, m, "taste.rate", ipc.Request{Kind: "album", Playlist: "MPREb_a", Value: 1}); response.Error != ipc.V2ErrorCodeUnavailable {
		t.Fatalf("taste.rate without a store = %+v, want unavailable", response)
	}
}

// taste.list gives every rating, "not interested" first.
func TestNativeTasteList(t *testing.T) {
	m, _ := newActionTestModel(t)
	for _, r := range []ipc.Request{
		{Kind: "artist", Playlist: "UCa", Name: "Band", Value: 1},
		{Kind: "song", Track: &ipc.TrackInfo{Path: "/music/other.mp3", Title: "Other", Artist: "Singer"}, Value: -1},
	} {
		if response := runV2(t, m, "taste.rate", r); !response.OK {
			t.Fatalf("taste.rate %+v = %+v", r, response)
		}
	}
	response := runV2(t, m, "taste.list", ipc.Request{})
	if !response.OK || len(response.Ratings) != 2 {
		t.Fatalf("taste.list = %+v", response)
	}
	for i, want := range []ipc.RatingInfo{
		{ID: "/music/other.mp3", Kind: "song", Name: "Singer - Other", Artist: "Singer", Interest: -1}, // a song named by its track, as I and H name it
		{ID: "UCa", Kind: "artist", Name: "Band", Interest: 1},
	} {
		got := response.Ratings[i]
		if _, err := time.Parse(time.RFC3339, got.At); err != nil {
			t.Fatalf("rating %d at %q: %v", i, got.At, err)
		}
		got.At = ""
		if got != want {
			t.Fatalf("rating %d = %+v, want %+v", i, got, want)
		}
	}
}

// radioTestProvider relates every song and item to the same songs.
type radioTestProvider struct {
	commandsTestProvider
	related []playlist.Track
}

func (p radioTestProvider) CanRelate(t playlist.Track) bool { return strings.HasPrefix(t.Path, "yt:") }

func (p radioTestProvider) RelatedTracks(context.Context, playlist.Track, int) ([]playlist.Track, error) {
	return p.related, nil
}

func (p radioTestProvider) RelatedToItem(_ context.Context, id string, _ int) (playlist.Track, []playlist.Track, error) {
	return playlist.Track{Path: "yt:first-of-" + id}, p.related, nil
}

// radio.start replaces the queue with the radio of a song or of an item and
// plays its first song, or fails with the reason.
func TestNativeRadioStart(t *testing.T) {
	related := []playlist.Track{{Path: "yt:a"}, {Path: "yt:b"}}
	for _, tc := range []struct {
		name     string
		related  []playlist.Track
		request  ipc.Request
		wantPlay string
		wantErr  string
	}{
		{name: "track seed", related: related, request: ipc.Request{Track: &ipc.TrackInfo{Path: "yt:seed", Title: "Seed"}}, wantPlay: "yt:seed"},
		{name: "item seed", related: related, request: ipc.Request{Provider: "yt", Playlist: "UCa", Name: "Band"}, wantPlay: "yt:first-of-UCa"},
		{name: "too few songs", request: ipc.Request{Track: &ipc.TrackInfo{Path: "yt:seed"}}, wantErr: "internal_error: No related songs found"},
		{name: "a song no provider relates", related: related, request: ipc.Request{Track: &ipc.TrackInfo{Path: "/music/x.mp3"}}, wantErr: "internal_error: Works only with YouTube Music songs"},
		{name: "no seed", related: related, request: ipc.Request{Provider: "yt"}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "unknown provider", related: related, request: ipc.Request{Provider: "nope", Playlist: "UCa"}, wantErr: ipc.V2ErrorCodeInvalidParams},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, engine := newActionTestModel(t)
			m.providers = []provider.Entry{{Key: "yt", Name: "YT", Provider: radioTestProvider{commandsTestProvider{name: "YT"}, tc.related}}}
			response := runV2(t, m, "radio.start", tc.request)
			if tc.wantErr != "" {
				if response.OK || response.Error != tc.wantErr || m.playlist.Len() != 2 {
					t.Fatalf("response = %+v, queue of %d; want %q and the old queue", response, m.playlist.Len(), tc.wantErr)
				}
				return
			}
			if !response.OK || response.Total != 3 || len(response.Tracks) != 3 || response.Tracks[0].Path != tc.wantPlay {
				t.Fatalf("response = %+v, want the radio of 3 from %s", response, tc.wantPlay)
			}
			if len(engine.playCalls) != 1 || engine.playCalls[0] != tc.wantPlay {
				t.Fatalf("played %v, want %s", engine.playCalls, tc.wantPlay)
			}
		})
	}
}

// discover.remixes gives the versions of a song grouped by section, and
// opens no overlay.
func TestNativeDiscoverRemixes(t *testing.T) {
	m, _ := newActionTestModel(t)
	searcher := &fakeSearcher{results: []playlist.Track{
		{Path: "yt:1", Title: "Sweater Weather (Slowed + Reverb)", DurationSecs: 290},
		{Path: "yt:3", Title: "Afraid", DurationSecs: 250},
	}}
	m.providers = []provider.Entry{{Key: "yt", Name: "Fake", Provider: searcher}}
	track := &ipc.TrackInfo{Path: "yt:orig", Title: "Sweater Weather (Official Video)", Artist: "The Neighbourhood", DurationSecs: 240}
	response := runV2(t, m, "discover.remixes", ipc.Request{Track: track})
	if !response.OK || response.Total != 1 || len(response.Tracks) != 1 || response.Tracks[0].Path != "yt:1" ||
		response.Tracks[0].ProviderMeta[playlist.MetaSection] != "Slowed + reverb" {
		t.Fatalf("remixes = %+v", response)
	}
	if m.searchOverlay.visible || len(searcher.queries) == 0 || !strings.HasPrefix(searcher.queries[0], "Sweater Weather ") {
		t.Fatalf("overlay open %v, queries %q", m.searchOverlay.visible, searcher.queries)
	}
	if response := runV2(t, m, "discover.remixes", ipc.Request{Track: &ipc.TrackInfo{Path: "yt:orig"}}); response.Error != ipc.V2ErrorCodeInvalidParams {
		t.Fatalf("remixes of a song with no title = %+v, want invalid_params", response)
	}
}

// settings.set changes and saves each setting, and refuses any other name
// or value without a change.
func TestNativeSettingsSet(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request ipc.Request
		check   func(*Model, *normalizingEngine) bool
		saved   map[string]string
		wantErr string
	}{
		{name: "auto radio", request: ipc.Request{Name: "auto_radio", Value: 1}, check: func(m *Model, _ *normalizingEngine) bool { return m.autoRadio }, saved: map[string]string{"ytmusic.auto_radio": "true"}},
		{name: "normalize album", request: ipc.Request{Name: "normalize", Value: 2}, check: func(_ *Model, e *normalizingEngine) bool { return e.mode == player.NormalizeAlbum }, saved: map[string]string{"normalize": `"album"`}},
		{name: "normalize off", request: ipc.Request{Name: "normalize"}, check: func(_ *Model, e *normalizingEngine) bool { return e.mode == player.NormalizeOff }, saved: map[string]string{"normalize": `"off"`}},
		{name: "crossfade", request: ipc.Request{Name: "crossfade", Value: 4}, check: func(m *Model, _ *normalizingEngine) bool { return m.crossfade == 4*time.Second }, saved: map[string]string{"crossfade": "4"}},
		{name: "auto EQ", request: ipc.Request{Name: "eq_auto", Value: 1}, check: func(m *Model, _ *normalizingEngine) bool { return m.eqAuto }, saved: map[string]string{"eq_auto": "true"}},
		{name: "mono", request: ipc.Request{Name: "mono", Value: 1}, check: func(_ *Model, e *normalizingEngine) bool { return e.mono }, saved: map[string]string{"mono": "true"}},
		{name: "quality on wifi", request: ipc.Request{Name: "quality_wifi", Text: " Ahorro "}, check: func(m *Model, _ *normalizingEngine) bool { return m.ytQuality.wifi == "ahorro" }, saved: map[string]string{"ytmusic.quality_wifi": `"ahorro"`}},
		{name: "quality on mobile", request: ipc.Request{Name: "quality_mobile", Text: "alta"}, check: func(m *Model, _ *normalizingEngine) bool { return m.ytQuality.mobile == "alta" }, saved: map[string]string{"ytmusic.quality_mobile": `"alta"`}},
		{name: "network is not saved", request: ipc.Request{Name: "network", Text: "mobile"}, check: func(m *Model, _ *normalizingEngine) bool { return m.ytQuality.network == "mobile" }},
		{name: "premium quality", request: ipc.Request{Name: "premium_quality", Text: "on"}, check: func(m *Model, _ *normalizingEngine) bool { return m.ytQuality.premium }, saved: map[string]string{"ytmusic.premium_quality": "true"}},
		{name: "unknown quality", request: ipc.Request{Name: "quality_wifi", Text: "max"}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "unknown network", request: ipc.Request{Name: "network", Text: "ethernet"}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "premium as a number", request: ipc.Request{Name: "premium_quality", Value: 1}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "unknown name", request: ipc.Request{Name: "volume", Value: 1}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "crossfade too long", request: ipc.Request{Name: "crossfade", Value: 13}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "half a second", request: ipc.Request{Name: "crossfade", Value: 1.5}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "negative mode", request: ipc.Request{Name: "normalize", Value: -1}, wantErr: ipc.V2ErrorCodeInvalidParams},
		{name: "switch at 2", request: ipc.Request{Name: "mono", Value: 2}, wantErr: ipc.V2ErrorCodeInvalidParams},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := &normalizingEngine{mode: player.NormalizeTrack}
			saver := &recordingSaver{}
			m := &Model{player: engine, playlist: playlist.New(), configSaver: saver}
			response := runV2(t, m, "settings.set", tc.request)
			if tc.wantErr != "" {
				if response.OK || response.Error != tc.wantErr || len(saver.saved) != 0 || engine.mono || engine.mode != player.NormalizeTrack {
					t.Fatalf("response = %+v, saved %v; want %s and no change", response, saver.saved, tc.wantErr)
				}
				return
			}
			if !response.OK || !tc.check(m, engine) || !reflect.DeepEqual(saver.saved, tc.saved) {
				t.Fatalf("response = %+v, saved %v; want the change and %v saved", response, saver.saved, tc.saved)
			}
		})
	}

	m := &Model{player: &playbackFakeEngine{}, playlist: playlist.New(), configSaver: &recordingSaver{err: errors.New("disk full")}}
	if response := runV2(t, m, "settings.set", ipc.Request{Name: "mono", Value: 1}); response.Error != "internal_error: disk full" {
		t.Fatalf("settings.set with a failing save = %+v, want internal_error", response)
	}
}

// The download operations need offline downloads; download.status gives
// the queue, download.retry queues the failed songs and download.cancel
// empties the queue.
func TestNativeDownloads(t *testing.T) {
	m, _ := newActionTestModel(t)
	m.providers = []provider.Entry{{Key: "yt", Name: "YT", Provider: commandsTestProvider{name: "YT"}}}
	song := func(id string) playlist.Track {
		return playlist.Track{Path: "https://music.youtube.com/watch?v=" + id, Title: id}
	}
	// running stops the queue from starting a real download.
	m.downloads.running, m.downloads.current, m.downloads.percent, m.downloads.done = true, offline.Job{Track: song("cur")}, 42, 1
	m.downloads.queue = []offline.Job{{Track: song("next")}}
	m.downloads.failedJobs = []failedDownload{{job: offline.Job{Track: song("bad")}, err: "HTTP Error 403"}}

	status := runV2(t, m, "download.status", ipc.Request{})
	info := func(id string, index int) ipc.TrackInfo { return ipcTrackInfo(song(id), index, 0, false) }
	current := info("cur", 0)
	want := &ipc.DownloadsInfo{
		Running: true, Current: &current, Percent: 42, Queued: []ipc.TrackInfo{info("next", 0)}, Done: 1,
		Failed: []ipc.DownloadFailure{{Track: info("bad", 0), Error: "HTTP Error 403"}}, Directory: m.downloads.dir,
	}
	if !status.OK || !reflect.DeepEqual(status.Downloads, want) {
		t.Fatalf("download.status = %+v, want %+v", status.Downloads, want)
	}

	if response := runV2(t, m, "download.retry", ipc.Request{}); !response.OK || len(m.downloads.queue) != 2 || len(m.downloads.failedJobs) != 0 {
		t.Fatalf("download.retry = %+v, queue %d, failed %d", response, len(m.downloads.queue), len(m.downloads.failedJobs))
	}
	if response := runV2(t, m, "download", ipc.Request{Track: &ipc.TrackInfo{Path: song("new").Path, Title: "new"}}); !response.OK || len(m.downloads.queue) != 3 {
		t.Fatalf("download of a track = %+v, queue %d", response, len(m.downloads.queue))
	}
	if response := runV2(t, m, "download", ipc.Request{Provider: "yt", Playlist: "MPREb_a", Name: "Record"}); !response.OK {
		t.Fatalf("download of an album = %+v", response)
	}
	for _, bad := range []ipc.Request{{}, {Provider: "yt"}, {Provider: "nope", Playlist: "MPREb_a"}} {
		if response := runV2(t, m, "download", bad); response.Error != ipc.V2ErrorCodeInvalidParams {
			t.Fatalf("download %+v = %+v, want invalid_params", bad, response)
		}
	}
	if response := runV2(t, m, "download.cancel", ipc.Request{}); !response.OK || len(m.downloads.queue) != 0 {
		t.Fatalf("download.cancel = %+v, queue %d", response, len(m.downloads.queue))
	}

	m.downloads.manifest = nil
	for _, op := range []string{"download", "download.status", "download.cancel", "download.retry"} {
		if response := runV2(t, m, op, ipc.Request{Track: &ipc.TrackInfo{Path: song("x").Path}}); response.Error != ipc.V2ErrorCodeUnavailable {
			t.Fatalf("%s without offline downloads = %+v, want unavailable", op, response)
		}
	}
}

// stats sums the play log over the period.
func TestNativeStats(t *testing.T) {
	m, _ := newActionTestModel(t)
	now := time.Now()
	for _, p := range []stats.Play{
		{At: now.Add(-time.Hour), Key: "/a", Title: "A", Artist: "Band", Genre: "Rock", Secs: 120},
		{At: now.Add(-2 * time.Hour), Key: "/a", Title: "A", Artist: "Band", Genre: "Rock", Secs: 60},
		{At: now.AddDate(0, -2, 0), Key: "/b", Title: "B", Artist: "Old", Secs: 60},
	} {
		if err := m.statsLog.Record(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		period string
		plays  int
		songs  int
	}{{"week", 2, 1}, {"month", 2, 1}, {"year", 3, 2}, {"all", 3, 2}} {
		response := runV2(t, m, "stats", ipc.Request{Name: tc.period})
		s := response.Stats
		if !response.OK || s == nil || s.Plays != tc.plays || s.Songs != tc.songs || len(s.Daily) != statsDays || s.Streak < 1 {
			t.Fatalf("stats %s = %+v", tc.period, s)
		}
		if want := (ipc.StatsCount{Name: "A", Detail: "Band", Plays: 2, Secs: 180}); s.TopSongs[0] != want || s.TopArtists[0].Name != "Band" || s.TopGenres[0].Name != "Rock" {
			t.Fatalf("stats %s tops = %+v %+v %+v", tc.period, s.TopSongs, s.TopArtists, s.TopGenres)
		}
	}
	if response := runV2(t, m, "stats", ipc.Request{Name: "decade"}); response.Error != ipc.V2ErrorCodeInvalidParams {
		t.Fatalf("stats of an unknown period = %+v", response)
	}
	m.statsLog = nil
	if response := runV2(t, m, "stats", ipc.Request{Name: "week"}); response.Error != ipc.V2ErrorCodeUnavailable {
		t.Fatalf("stats without a log = %+v", response)
	}
}

// Every batch C operation is registered and reaches its handler.
func TestNativeActionRouting(t *testing.T) {
	registry := ipc.DefaultOperationRegistry()
	for _, op := range []string{"taste.rate", "taste.list", "radio.start", "discover.remixes", "settings.set", "download", "download.status", "download.cancel", "download.retry", "stats"} {
		if _, ok := registry.Lookup(op); !ok {
			t.Errorf("%s is not registered", op)
		}
		m, _ := newActionTestModel(t)
		if response := runV2(t, m, op, ipc.Request{}); strings.HasPrefix(response.Error, ipc.V2ErrorCodeUnavailable) {
			t.Errorf("%s = %+v, want its handler", op, response)
		}
	}
}

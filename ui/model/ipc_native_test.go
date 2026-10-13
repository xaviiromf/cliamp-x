package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/playback"
	"github.com/xaviiromf/cliamp-x/ipc"
	"github.com/xaviiromf/cliamp-x/player"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
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

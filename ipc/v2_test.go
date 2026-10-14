package ipc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestV2JSONEnvelope(t *testing.T) {
	request := V2Request{
		ID:        json.RawMessage(`"request-1"`),
		Method:    "call",
		Operation: "runtime.snapshot",
		Params:    json.RawMessage(`{"detail":"full"}`),
		JobID:     "job-1",
		Topics:    []string{"runtime.state"},
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}

	var decoded V2Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != protocolVersion2 || string(decoded.ID) != `"request-1"` || decoded.Operation != "runtime.snapshot" || decoded.JobID != "job-1" {
		t.Fatalf("decoded request = %#v", decoded)
	}

	response, err := json.Marshal(V2Response{
		ID:       decoded.ID,
		OK:       true,
		Snapshot: &RuntimeSnapshot{State: "playing"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(response, &raw); err != nil {
		t.Fatal(err)
	}
	if string(raw["version"]) != "2" || string(raw["id"]) != `"request-1"` {
		t.Fatalf("response envelope = %s", response)
	}
}

func TestOperationRegistryValidation(t *testing.T) {
	registry := DefaultOperationRegistry()
	tests := []struct {
		name   string
		params string
		want   string
	}{
		{name: "valid", params: `{"value":1.5,"index":0,"batch":[{"path":"song.flac"}]}`},
		{name: "string number", params: `{"value":"1"}`, want: V2ErrorCodeInvalidParams},
		{name: "fractional index", params: `{"index":1.5}`, want: V2ErrorCodeInvalidParams},
		{name: "string index", params: `{"index":"1"}`, want: V2ErrorCodeInvalidParams},
		{name: "negative index", params: `{"index":-1}`, want: V2ErrorCodeInvalidParams},
		{name: "malformed batch", params: `{"batch":{}}`, want: V2ErrorCodeInvalidParams},
		{name: "invalid JSON", params: `{"value":NaN}`, want: V2ErrorCodeInvalidParams},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := registry.Validate("queue.enqueue", json.RawMessage(test.params))
			if got := errorCode(err); got != test.want {
				t.Fatalf("Validate() error code = %q, want %q", got, test.want)
			}
		})
	}
	if got := errorCode(registry.Validate("runtime.missing", nil)); got != V2ErrorCodeUnknownOperation {
		t.Fatalf("unknown operation code = %q", got)
	}
}

// The Android app pauses the drawing with ui.render and the name on or off.
// Like every operation but the two direct reads, it runs as a job.
func TestUIRenderOperation(t *testing.T) {
	registry := DefaultOperationRegistry()
	operation, ok := registry.Lookup("ui.render")
	if !ok || !operation.Async || len(operation.Parameters) != 1 || operation.Parameters[0] != "name" {
		t.Fatalf("ui.render = %+v (registered %v), want an async operation with the name parameter", operation, ok)
	}
	for _, params := range []string{`{"name":"off"}`, `{"name":"on"}`} {
		if err := registry.Validate("ui.render", json.RawMessage(params)); err != nil {
			t.Fatalf("Validate(ui.render, %s) = %+v, want nil", params, err)
		}
	}
}

// The native Android UI reads these parameters from capabilities.
func TestNativeOperationParameters(t *testing.T) {
	registry := DefaultOperationRegistry()
	for name, want := range map[string][]string{
		"queue.replace":       {"tracks", "index", "if_revision"},
		"queue.list":          {"sort", "offset", "limit"},
		"provider.load":       {"provider", "playlist", "index"},
		"provider.load_album": {"provider", "album", "index"},
		// batch B: browse
		"provider.artists":        {"provider", "sort", "offset", "limit"},
		"provider.home":           {"provider"},
		"provider.genres":         {"provider", "playlist"},
		"provider.genre_tracks":   {"provider", "playlist", "sort", "offset", "limit"},
		"provider.genre_favorite": {"provider", "playlist"},
		"provider.refresh":        {"provider"},
		"provider.preview":        {"provider", "playlist"},
		// batch C: actions, library, settings
		"taste.rate":       {"kind", "playlist", "name", "track", "value"},
		"taste.list":       nil,
		"radio.start":      {"provider", "playlist", "name", "track"},
		"discover.remixes": {"track"},
		"settings.set":     {"name", "value", "text"},
		"download":         {"provider", "playlist", "name", "track"},
		"download.status":  nil,
		"download.cancel":  nil,
		"download.retry":   nil,
		"download.auto":    {"limit", "value"},
		"stats":            {"name", "value"},
	} {
		operation, ok := registry.Lookup(name)
		if !ok || !operation.Async || !slices.Equal(operation.Parameters, want) {
			t.Errorf("%s = %+v (registered %v), want an async operation with %v", name, operation, ok, want)
		}
	}
}

// The new snapshot fields keep their JSON names, and a cloned snapshot
// owns its notice.
func TestRuntimeSnapshotNativeFields(t *testing.T) {
	snapshot := RuntimeSnapshot{
		Buffering: true, Seeking: true, Error: "e", ErrorKind: "network", ErrorPath: "/a",
		Notice: &NoticeInfo{Text: "Saved", Kind: "success"}, SleepMinutes: -1, SleepAt: 7,
		AutoRadio: true, Normalize: "track", Crossfade: 3, EQAuto: true,
		QualityWifi: "alta", QualityMobile: "ahorro", Network: "mobile", PremiumQuality: true,
		AudioCodec: "opus", AudioKbps: 130, AudioRate: 48000,
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"buffering", "seeking", "error", "error_kind", "error_path", "notice", "sleep_minutes", "sleep_at", "auto_radio", "normalize", "crossfade", "eq_auto",
		"quality_wifi", "quality_mobile", "network", "premium_quality", "audio_codec", "audio_kbps", "audio_rate"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("snapshot JSON %s has no %s", data, name)
		}
	}
	if string(fields["notice"]) != `{"text":"Saved","kind":"success"}` {
		t.Errorf("notice = %s", fields["notice"])
	}
	clone := cloneSnapshot(&snapshot)
	clone.Notice.Text = "changed"
	if snapshot.Notice.Text != "Saved" {
		t.Error("the clone shares its notice")
	}
}

func TestJobStoreLifecycleCancelAndExpiry(t *testing.T) {
	store := NewJobStore(WithJobTTL(5 * time.Millisecond))
	job, err := store.Create("provider.search")
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := store.Start(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Cancel(job.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("job context was not canceled")
	}

	got, ok := store.Get(job.ID)
	if !ok || got.State != JobCanceled || got.Error == nil || got.Error.Code != V2ErrorCodeCanceled {
		t.Fatalf("canceled job = %#v, present=%v", got, ok)
	}
	select {
	case event := <-store.Events():
		if event.Type != "job.canceled" || event.Job.ID != job.ID || event.Job.State != JobCanceled {
			t.Fatalf("event = %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("missing terminal job event")
	}

	time.Sleep(10 * time.Millisecond)
	if _, ok := store.Get(job.ID); ok {
		t.Fatal("terminal job did not expire")
	}
}

func TestJobStoreSnapshotResult(t *testing.T) {
	store := NewJobStore()
	job, err := store.Create("runtime.snapshot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Start(job.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.SucceedSnapshot(job.ID, RuntimeSnapshot{State: "playing"}); err != nil {
		t.Fatal(err)
	}
	got, ok := store.Get(job.ID)
	if !ok || got.State != JobSucceeded || got.Snapshot == nil || got.Snapshot.State != "playing" {
		t.Fatalf("snapshot job = %#v, present=%v", got, ok)
	}
}

func TestJobStorePreservesFailureDetail(t *testing.T) {
	store := NewJobStore()
	job, err := store.Create("provider.search")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Start(job.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.Fail(job.ID, V2Error{Code: V2ErrorCodeInternal, Message: V2MessageInternal, Detail: "provider timed out"}); err != nil {
		t.Fatal(err)
	}
	completed, ok := store.Get(job.ID)
	if !ok || completed.Error == nil || completed.Error.Detail != "provider timed out" {
		t.Fatalf("job = %#v", completed)
	}
}

func TestServerRoutesV2(t *testing.T) {
	sock := filepath.Join(shortTempDir(t), "cliamp-x.sock")
	server, err := NewServer(sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	server.SetV2Dispatcher(V2DispatcherFunc(func(ctx context.Context, request V2Request) (V2Result, *V2Error) {
		if request.Method != "state.get" {
			t.Fatalf("method = %q", request.Method)
		}
		return V2Result{Snapshot: &RuntimeSnapshot{State: "playing"}}, nil
	}))

	response := sendV2Request(t, sock, V2Request{
		ID:     json.RawMessage(`42`),
		Method: "state.get",
	})
	if !response.OK || response.Version != protocolVersion2 || string(response.ID) != "42" || response.Snapshot == nil || response.Snapshot.State != "playing" {
		t.Fatalf("v2 response = %#v", response)
	}

	unsupported := sendRawV2Request(t, sock, []byte(`{"version":7,"id":"wrong-version"}`))
	if unsupported.OK || string(unsupported.ID) != `"wrong-version"` || errorCode(unsupported.Error) != V2ErrorCodeInvalidVersion {
		t.Fatalf("unsupported response = %#v", unsupported)
	}

}

// The runtime owner matches the read methods exactly, so the server hands it
// the canonical name whatever the case and spaces of the request.
func TestServerCanonicalizesReadMethods(t *testing.T) {
	sock := filepath.Join(shortTempDir(t), "cliamp-x.sock")
	server, err := NewServer(sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	server.SetV2Dispatcher(V2DispatcherFunc(func(_ context.Context, request V2Request) (V2Result, *V2Error) {
		return V2Result{Snapshot: &RuntimeSnapshot{State: request.Method + "|" + request.Operation}}, nil
	}))

	for _, tc := range []struct{ method, operation, want, wantErr string }{
		{method: "state.get", want: "state.get|"},
		{method: "STATE.GET", want: "state.get|"},
		{method: "spectrum.get", want: "spectrum.get|"},
		{method: " spectrum.get ", want: "spectrum.get|"},
		{method: "Spectrum.Get", want: "spectrum.get|"},
		{method: "STATE.GET", operation: "play", wantErr: V2ErrorCodeInvalidRequest},
	} {
		t.Run(tc.method+" "+tc.operation, func(t *testing.T) {
			response := sendV2Request(t, sock, V2Request{ID: json.RawMessage(`1`), Method: tc.method, Operation: tc.operation})
			if tc.wantErr != "" {
				if response.OK || errorCode(response.Error) != tc.wantErr {
					t.Fatalf("response = %#v, want error %q", response, tc.wantErr)
				}
				return
			}
			if !response.OK || response.Snapshot == nil || response.Snapshot.State != tc.want {
				t.Fatalf("response = %#v, want the owner to get %q", response, tc.want)
			}
		})
	}
}

func TestServerCanonicalizesMethodOperation(t *testing.T) {
	sock := filepath.Join(shortTempDir(t), "cliamp-x.sock")
	server, err := NewServer(sock)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	server.SetV2Dispatcher(V2DispatcherFunc(func(_ context.Context, request V2Request) (V2Result, *V2Error) {
		if request.Method != "play" || request.Operation != "play" {
			t.Fatalf("request = %#v", request)
		}
		return V2Result{}, nil
	}))

	response := sendV2Request(t, sock, V2Request{ID: json.RawMessage(`"play"`), Method: "play"})
	if !response.OK {
		t.Fatalf("response = %#v", response)
	}
}

func TestServerRoutesRuntimeSnapshotAliasToV2State(t *testing.T) {
	for _, operation := range []string{"runtime.snapshot", "runtime.status"} {
		t.Run(operation, func(t *testing.T) {
			sock := filepath.Join(shortTempDir(t), "cliamp-x.sock")
			server, err := NewServer(sock)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = server.Close() })
			server.SetV2Dispatcher(V2DispatcherFunc(func(_ context.Context, request V2Request) (V2Result, *V2Error) {
				if request.Method != "state.get" || request.Operation != "" {
					return V2Result{}, &V2Error{Code: V2ErrorCodeInvalidParams, Message: V2MessageInvalidParams, Detail: request.Method + " " + request.Operation}
				}
				return V2Result{Snapshot: &RuntimeSnapshot{State: "paused"}}, nil
			}))

			response := sendV2Request(t, sock, V2Request{ID: json.RawMessage(`"snapshot"`), Method: "operation.submit", Operation: operation})
			if !response.OK || response.Job != nil || response.Snapshot == nil || response.Snapshot.State != "paused" {
				t.Fatalf("response = %#v", response)
			}
		})
	}
}

func TestServerCloseIsIdempotent(t *testing.T) {
	sock := filepath.Join(shortTempDir(t), "cliamp-x.sock")
	server, err := NewServer(sock)
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if err := server.Close(); err != nil {
		t.Fatalf("second Close() = %v", err)
	}
}

func TestV2SubscribeAcknowledgesThenStreamsEvents(t *testing.T) {
	broker := NewBroker()
	sock := filepath.Join(shortTempDir(t), "cliamp-x.sock")
	server, err := NewServerWithBroker(sock, broker)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })

	conn, err := dialSocket(sock, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	request, err := json.Marshal(V2Request{
		ID:     json.RawMessage(`"subscription-1"`),
		Method: "subscribe",
		Topics: []string{"runtime.state"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write(append(request, '\n')); err != nil {
		t.Fatal(err)
	}

	scanner := newFrameScanner(conn)
	if !scanner.Scan() {
		t.Fatalf("read acknowledgement: %v", scanner.Err())
	}
	var acknowledgement V2Response
	if err := json.Unmarshal(scanner.Bytes(), &acknowledgement); err != nil {
		t.Fatal(err)
	}
	if !acknowledgement.OK || acknowledgement.Version != protocolVersion2 || string(acknowledgement.ID) != `"subscription-1"` {
		t.Fatalf("acknowledgement = %#v", acknowledgement)
	}

	if err := broker.Publish("runtime.state", json.RawMessage(`{"state":"playing"}`), false); err != nil {
		t.Fatal(err)
	}
	if !scanner.Scan() {
		t.Fatalf("read event: %v", scanner.Err())
	}
	var event Event
	if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
		t.Fatal(err)
	}
	if event.Event != "runtime.state" {
		t.Fatalf("event = %#v", event)
	}
}

func sendV2Request(t *testing.T, sock string, request V2Request) V2Response {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return sendRawV2Request(t, sock, data)
}

func sendRawV2Request(t *testing.T, sock string, data []byte) V2Response {
	t.Helper()
	conn, err := dialSocket(sock, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write(append(data, '\n')); err != nil {
		t.Fatal(err)
	}
	scanner := newFrameScanner(conn)
	if !scanner.Scan() {
		t.Fatalf("read V2 response: %v", scanner.Err())
	}
	var response V2Response
	if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func errorCode(err *V2Error) string {
	if err == nil {
		return ""
	}
	return err.Code
}

// The browse result shapes keep their JSON names.
func TestBrowseResultFields(t *testing.T) {
	favorite := true
	response := Response{
		OK:       true,
		Shelves:  []ShelfInfo{{Title: "Picks", Items: []ShelfItemInfo{{Kind: "song", ID: "v", Title: "T", Subtitle: "A", ImageURL: "i", Track: &TrackInfo{Path: "p"}}}}},
		Genres:   []GenreInfo{{ID: "g", Name: "Rock", Group: "Genres", Favorite: true}},
		Preview:  &PreviewInfo{ImageURL: "i", Lines: []string{"l"}, Favorite: &favorite},
		Albums:   []AlbumInfo{{ID: "a", Name: "N", ImageURL: "i", Favorite: true, Single: true}},
		Artists:  []ArtistInfo{{ID: "r", Name: "N", ImageURL: "i", Favorite: true}},
		Playlist: "x",
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ok":true,"playlist":"x","artists":[{"id":"r","name":"N","image_url":"i","favorite":true}],` +
		`"albums":[{"id":"a","name":"N","image_url":"i","favorite":true,"single":true}],` +
		`"shelves":[{"title":"Picks","items":[{"kind":"song","id":"v","title":"T","subtitle":"A","image_url":"i","track":{"path":"p"}}]}],` +
		`"genres":[{"id":"g","name":"Rock","group":"Genres","favorite":true}],` +
		`"preview":{"image_url":"i","lines":["l"],"favorite":true}}`
	if string(data) != want {
		t.Fatalf("JSON = %s\nwant   %s", data, want)
	}
	playlist, _ := json.Marshal(PlaylistInfo{ID: "pl:x", Name: "N", Provider: "ytmusic", ImageURL: "i", Writable: true, Unseen: true})
	if want := `{"id":"pl:x","name":"N","provider":"ytmusic","image_url":"i","writable":true,"unseen":true}`; string(playlist) != want {
		t.Fatalf("playlist JSON = %s, want %s", playlist, want)
	}
}

// The batch C result shapes keep their JSON names, and kind reaches the
// request.
func TestActionResultFields(t *testing.T) {
	response := Response{
		OK:      true,
		Ratings: []RatingInfo{{ID: "UCa", Kind: "artist", Name: "Band", Interest: -1, At: "2026-10-13T12:00:00Z"}},
		Downloads: &DownloadsInfo{
			Running: true, Current: &TrackInfo{Path: "c"}, Percent: 42, Queued: []TrackInfo{{Path: "q"}}, Done: 1,
			Failed: []DownloadFailure{{Track: TrackInfo{Path: "f"}, Error: "HTTP Error 403"}}, Directory: "/music",
		},
		Stats: &StatsInfo{
			Plays: 2, Secs: 180, Songs: 1, Artists: 1, TopArtists: []StatsCount{{Name: "Band", Plays: 2, Secs: 180}},
			TopSongs: []StatsCount{{Name: "A", Detail: "Band", Plays: 2, Secs: 180}}, Daily: []float64{0, 3}, Streak: 1,
		},
	}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ok":true,"ratings":[{"id":"UCa","kind":"artist","name":"Band","interest":-1,"at":"2026-10-13T12:00:00Z"}],` +
		`"downloads":{"running":true,"current":{"path":"c"},"percent":42,"queued":[{"path":"q"}],"done":1,` +
		`"failed":[{"track":{"path":"f"},"error":"HTTP Error 403"}],"directory":"/music"},` +
		`"stats":{"plays":2,"secs":180,"songs":1,"artists":1,"top_artists":[{"name":"Band","plays":2,"secs":180}],` +
		`"top_songs":[{"name":"A","detail":"Band","plays":2,"secs":180}],"daily":[0,3],"streak":1}}`
	if string(data) != want {
		t.Fatalf("JSON = %s\nwant   %s", data, want)
	}
	var request Request
	if err := json.Unmarshal([]byte(`{"kind":"song","value":-1}`), &request); err != nil || request.Kind != "song" || request.Value != -1 {
		t.Fatalf("request = %+v, %v", request, err)
	}
}

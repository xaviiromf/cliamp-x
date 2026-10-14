package scrobble

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeServices answers as ListenBrainz on /1/submit-listens and as Last.fm
// on /2.0/. lbStatus is the HTTP status of ListenBrainz, lfmError the
// Last.fm error code (0 answers ok).
type fakeServices struct {
	mu       sync.Mutex
	lbStatus int
	lfmError int
	lb       []map[string]any
	lfm      []url.Values
}

func (f *fakeServices) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/1/submit-listens":
		if r.Header.Get("Authorization") != "Token lb-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var body map[string]any
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &body)
		if f.lbStatus != 0 && f.lbStatus != http.StatusOK {
			w.WriteHeader(f.lbStatus)
			return
		}
		f.lb = append(f.lb, body)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	case "/2.0/":
		_ = r.ParseForm()
		form := r.PostForm
		if f.lfmError != 0 {
			_, _ = io.WriteString(w, `{"error":`+itoa(f.lfmError)+`,"message":"nope"}`)
			return
		}
		if form.Get("api_sig") != wantSignature(form) {
			_, _ = io.WriteString(w, `{"error":13,"message":"Invalid method signature supplied"}`)
			return
		}
		f.lfm = append(f.lfm, form)
		switch form.Get("method") {
		case "auth.getToken":
			_, _ = io.WriteString(w, `{"token":"tok"}`)
		case "auth.getSession":
			_, _ = io.WriteString(w, `{"session":{"name":"user","key":"sess","subscriber":0}}`)
		default:
			_, _ = io.WriteString(w, `{"scrobbles":{"@attr":{"accepted":1,"ignored":0}}}`)
		}
	default:
		http.NotFound(w, r)
	}
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// wantSignature signs form as the Last.fm docs describe, with the secret
// "secret".
func wantSignature(form url.Values) string {
	var keys []string
	for k := range form {
		if k != "format" && k != "api_sig" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + form.Get(k))
	}
	sum := md5.Sum([]byte(b.String() + "secret"))
	return hex.EncodeToString(sum[:])
}

var (
	both     = Config{ListenBrainzToken: "lb-token", LastfmKey: "key", LastfmSecret: "secret", LastfmSession: "sess"}
	song     = Track{Artist: "Daft Punk", Title: "One More Time", Album: "Discovery", Duration: 320 * time.Second}
	playedAt = time.Unix(1_760_000_000, 0)
)

func newTestScrobbler(t *testing.T, cfg Config, f *fakeServices) *Scrobbler {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	s := New(cfg, filepath.Join(t.TempDir(), "scrobble_queue.json"))
	s.listenBrainzURL, s.lastfmURL = srv.URL, srv.URL+"/2.0/"
	return s
}

func queued(t *testing.T, s *Scrobbler) []listen {
	t.Helper()
	data, err := os.ReadFile(s.queuePath)
	if os.IsNotExist(err) {
		return nil
	}
	var q []listen
	if err != nil || json.Unmarshal(data, &q) != nil {
		t.Fatalf("queue file: %v %s", err, data)
	}
	return q
}

// A scrobble reaches each service that is on with its fields, and a failure
// that may pass waits in the queue for the next one; a refusal is dropped.
func TestScrobble(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		lbStatus  int
		lfmError  int
		wantLB    int // listens ListenBrainz took
		wantLfm   int // scrobbles Last.fm took
		wantQueue []string
		wantErr   bool
	}{
		{name: "both services", cfg: both, wantLB: 1, wantLfm: 1},
		{name: "listenbrainz only", cfg: Config{ListenBrainzToken: "lb-token"}, wantLB: 1},
		{name: "lastfm without a session is off", cfg: Config{LastfmKey: "key", LastfmSecret: "secret"}},
		{name: "server error waits", cfg: both, lbStatus: http.StatusServiceUnavailable, wantLfm: 1, wantQueue: []string{ListenBrainz}, wantErr: true},
		{name: "rate limit waits", cfg: both, lbStatus: http.StatusTooManyRequests, wantLfm: 1, wantQueue: []string{ListenBrainz}, wantErr: true},
		{name: "bad token is dropped", cfg: Config{ListenBrainzToken: "wrong"}, wantErr: true},
		{name: "lastfm offline waits", cfg: both, lfmError: 11, wantLB: 1, wantQueue: []string{Lastfm}, wantErr: true},
		{name: "lastfm bad session is dropped", cfg: both, lfmError: 9, wantLB: 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &fakeServices{lbStatus: tt.lbStatus, lfmError: tt.lfmError}
			s := newTestScrobbler(t, tt.cfg, f)
			err := s.Scrobble(context.Background(), song, playedAt)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Scrobble error = %v, want error %v", err, tt.wantErr)
			}
			if len(f.lb) != tt.wantLB || len(f.lfm) != tt.wantLfm {
				t.Fatalf("sent %d to ListenBrainz and %d to Last.fm, want %d and %d", len(f.lb), len(f.lfm), tt.wantLB, tt.wantLfm)
			}
			var services []string
			for _, l := range queued(t, s) {
				services = append(services, l.Service)
			}
			if strings.Join(services, ",") != strings.Join(tt.wantQueue, ",") {
				t.Fatalf("queue = %v, want %v", services, tt.wantQueue)
			}
			if tt.wantLB > 0 {
				body := f.lb[0]
				item := body["payload"].([]any)[0].(map[string]any)
				meta := item["track_metadata"].(map[string]any)
				if body["listen_type"] != "single" || item["listened_at"] != float64(playedAt.Unix()) ||
					meta["artist_name"] != song.Artist || meta["track_name"] != song.Title || meta["release_name"] != song.Album {
					t.Errorf("listenbrainz body = %v", body)
				}
			}
			if tt.wantLfm > 0 {
				form := f.lfm[0]
				if form.Get("method") != "track.scrobble" || form.Get("sk") != "sess" || form.Get("artist") != song.Artist ||
					form.Get("track") != song.Title || form.Get("timestamp") != "1760000000" || form.Get("duration") != "320" {
					t.Errorf("last.fm form = %v", form)
				}
			}
		})
	}
}

// A queued listen goes out first once the service answers again, and a
// listen for a service that was turned off is dropped.
func TestScrobbleQueueDrains(t *testing.T) {
	f := &fakeServices{lbStatus: http.StatusBadGateway}
	s := newTestScrobbler(t, Config{ListenBrainzToken: "lb-token"}, f)
	first := Track{Artist: "A", Title: "First"}
	if err := s.Scrobble(context.Background(), first, playedAt); err == nil {
		t.Fatal("Scrobble with the service down: no error")
	}
	if err := s.Scrobble(context.Background(), song, playedAt.Add(time.Hour)); err == nil || len(queued(t, s)) != 2 {
		t.Fatalf("second scrobble: error %v, queue %d; want both waiting", err, len(queued(t, s)))
	}
	f.lbStatus = http.StatusOK
	if err := s.NowPlaying(context.Background(), song); err != nil {
		t.Fatalf("NowPlaying: %v", err)
	}
	if len(f.lb) != 3 || f.lb[0]["listen_type"] != "playing_now" || len(queued(t, s)) != 0 {
		t.Fatalf("sent %v, queue %d; want now playing, then the two waiting", f.lb, len(queued(t, s)))
	}
	names := []string{}
	for _, body := range f.lb[1:] {
		meta := body["payload"].([]any)[0].(map[string]any)["track_metadata"].(map[string]any)
		names = append(names, meta["track_name"].(string))
	}
	if strings.Join(names, ",") != "First,One More Time" {
		t.Fatalf("drained in order %v", names)
	}
	if _, ok := f.lb[0]["payload"].([]any)[0].(map[string]any)["listened_at"]; ok {
		t.Error("a now-playing listen has listened_at")
	}

	f.lbStatus = http.StatusBadGateway
	_ = s.Scrobble(context.Background(), song, playedAt)
	s.SetConfig(Config{})
	if err := s.Flush(context.Background()); err != nil || len(queued(t, s)) != 0 {
		t.Fatalf("flush with the service off: %v, queue %d; want it dropped", err, len(queued(t, s)))
	}
}

// The Last.fm sign-in returns the page to allow cliamp-x, then trades the
// token for a session key.
func TestLastfmSignIn(t *testing.T) {
	f := &fakeServices{}
	cfg := Config{LastfmKey: "key", LastfmSecret: "secret"}
	s := newTestScrobbler(t, cfg, f)
	token, page, err := s.LastfmToken(context.Background(), cfg)
	if err != nil || token != "tok" || page != "https://www.last.fm/api/auth/?api_key=key&token=tok" {
		t.Fatalf("LastfmToken = %q, %q, %v", token, page, err)
	}
	key, err := s.LastfmSession(context.Background(), cfg, token)
	if err != nil || key != "sess" || f.lfm[1].Get("token") != "tok" || f.lfm[1].Get("sk") != "" {
		t.Fatalf("LastfmSession = %q, %v; form %v", key, err, f.lfm[1])
	}
	f.lfmError = 14
	if _, err := s.LastfmSession(context.Background(), cfg, token); err == nil || !strings.Contains(err.Error(), "error 14") {
		t.Fatalf("LastfmSession of a token not allowed: %v", err)
	}
	if _, _, err := s.LastfmToken(context.Background(), Config{LastfmKey: "key"}); err == nil {
		t.Fatal("LastfmToken without a secret: no error")
	}
}

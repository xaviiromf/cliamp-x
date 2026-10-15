// Package scrobble reports what plays to ListenBrainz and Last.fm: the song
// that starts (now playing) and the song that was listened to (a scrobble).
// A scrobble that fails on a network or server error waits in a queue file
// and goes out with the next one.
package scrobble

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/appmeta"
	"github.com/xaviiromf/cliamp-x/internal/fileutil"
	"github.com/xaviiromf/cliamp-x/internal/httpclient"
)

// The service names of a queued listen.
const (
	ListenBrainz = "listenbrainz"
	Lastfm       = "lastfm"
)

// maxQueue bounds the listens that wait, so an endless outage cannot grow
// the file without end. The oldest go first.
const maxQueue = 1000

// Track is one song as the services take it.
type Track struct {
	Artist   string        `json:"artist"`
	Title    string        `json:"title"`
	Album    string        `json:"album,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
}

// Config holds the credentials. A service with any of its fields empty is
// off.
type Config struct {
	ListenBrainzToken string
	LastfmKey         string // the API key of the user's Last.fm API account
	LastfmSecret      string // its shared secret
	LastfmSession     string // the session key from the sign-in
}

// ListenBrainzOn reports whether listens go to ListenBrainz.
func (c Config) ListenBrainzOn() bool { return c.ListenBrainzToken != "" }

// LastfmOn reports whether scrobbles go to Last.fm.
func (c Config) LastfmOn() bool {
	return c.LastfmKey != "" && c.LastfmSecret != "" && c.LastfmSession != ""
}

// listen is one queued scrobble for one service.
type listen struct {
	Service string    `json:"service"`
	Track   Track     `json:"track"`
	At      time.Time `json:"at"`
}

// Scrobbler sends the reports. It is safe for concurrent use.
type Scrobbler struct {
	mu  sync.Mutex // guards cfg
	cfg Config
	// queueMu serializes the queue file: a Flush holds it while it sends,
	// so Config never waits on the network.
	queueMu   sync.Mutex
	queuePath string
	client    *http.Client
	// The API roots, which tests point at a local server.
	listenBrainzURL string
	lastfmURL       string
}

// New returns a Scrobbler with cfg that keeps failed scrobbles in the file
// at queuePath.
func New(cfg Config, queuePath string) *Scrobbler {
	return &Scrobbler{
		cfg:             cfg,
		queuePath:       queuePath,
		client:          httpclient.NewAPI(15 * time.Second),
		listenBrainzURL: "https://api.listenbrainz.org",
		lastfmURL:       "https://ws.audioscrobbler.com/2.0/",
	}
}

// SetListenBrainzURL points the scrobbler at another ListenBrainz API root;
// tests point it at a local server.
func (s *Scrobbler) SetListenBrainzURL(root string) { s.listenBrainzURL = root }

// SetConfig replaces the credentials.
func (s *Scrobbler) SetConfig(cfg Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

// Config returns the credentials.
func (s *Scrobbler) Config() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// NowPlaying tells each service that t starts, then sends the scrobbles that
// wait. A now-playing report that fails is not kept.
func (s *Scrobbler) NowPlaying(ctx context.Context, t Track) error {
	cfg := s.Config()
	var errs []error
	if cfg.ListenBrainzOn() {
		errs = append(errs, s.listenBrainz(ctx, cfg, "playing_now", t, time.Time{}))
	}
	if cfg.LastfmOn() {
		_, err := s.lastfm(ctx, cfg, "track.updateNowPlaying", trackParams(t, time.Time{}))
		errs = append(errs, err)
	}
	return errors.Join(append(errs, s.Flush(ctx))...)
}

// Scrobble records that t was listened to from at, for each service that
// is on, and sends the queue. A scrobble that fails on a network or server
// error stays in the queue; one that a service refuses is dropped and its
// error returned.
func (s *Scrobbler) Scrobble(ctx context.Context, t Track, at time.Time) error {
	cfg := s.Config()
	var add []listen
	if cfg.ListenBrainzOn() {
		add = append(add, listen{Service: ListenBrainz, Track: t, At: at})
	}
	if cfg.LastfmOn() {
		add = append(add, listen{Service: Lastfm, Track: t, At: at})
	}
	if len(add) == 0 {
		return nil
	}
	s.queueMu.Lock()
	queue, err := s.loadQueue()
	if err == nil {
		err = s.saveQueue(append(queue, add...))
	}
	s.queueMu.Unlock()
	if err != nil {
		return fmt.Errorf("scrobble queue: %w", err)
	}
	return s.Flush(ctx)
}

// Flush sends the queued scrobbles in order. A service stops at its first
// failure that may pass, which keeps that listen and the later ones; a
// listen for a service that is off is dropped.
func (s *Scrobbler) Flush(ctx context.Context) error {
	cfg := s.Config()
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	queue, err := s.loadQueue()
	if err != nil || len(queue) == 0 {
		return err
	}
	var keep []listen
	var errs []error
	stopped := map[string]bool{}
	for _, l := range queue {
		if stopped[l.Service] {
			keep = append(keep, l)
			continue
		}
		var err error
		switch {
		case l.Service == ListenBrainz && cfg.ListenBrainzOn():
			err = s.listenBrainz(ctx, cfg, "single", l.Track, l.At)
		case l.Service == Lastfm && cfg.LastfmOn():
			_, err = s.lastfm(ctx, cfg, "track.scrobble", trackParams(l.Track, l.At))
		}
		if err != nil {
			errs = append(errs, err)
			if retryable(err) {
				stopped[l.Service] = true
				keep = append(keep, l)
			}
		}
	}
	return errors.Join(append(errs, s.saveQueue(keep))...)
}

// loadQueue reads the queue file. The caller holds s.queueMu.
func (s *Scrobbler) loadQueue() ([]listen, error) {
	data, err := os.ReadFile(s.queuePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var queue []listen
	if err := json.Unmarshal(data, &queue); err != nil {
		// A broken file would block every later scrobble; start over.
		return nil, nil
	}
	return queue, nil
}

// saveQueue writes the queue file, or removes it when queue is empty. The
// caller holds s.queueMu.
func (s *Scrobbler) saveQueue(queue []listen) error {
	if len(queue) == 0 {
		if err := os.Remove(s.queuePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if len(queue) > maxQueue {
		queue = queue[len(queue)-maxQueue:]
	}
	data, err := json.Marshal(queue)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.queuePath), 0o755); err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(s.queuePath, data, 0o600)
}

// tempError is a failure that may pass: no connection, a server error or a
// rate limit.
type tempError struct{ err error }

func (e tempError) Error() string { return e.err.Error() }
func (e tempError) Unwrap() error { return e.err }

func retryable(err error) bool {
	var t tempError
	return errors.As(err, &t)
}

// post sends req and returns the body of a 2xx answer. A network error, a
// 5xx or a 429 is a tempError.
func (s *Scrobbler) post(req *http.Request) ([]byte, int, error) {
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, 0, tempError{err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, tempError{err}
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		return body, resp.StatusCode, tempError{fmt.Errorf("HTTP %d", resp.StatusCode)}
	}
	return body, resp.StatusCode, nil
}

// listenBrainz submits one listen of listenType: playing_now or single,
// which takes at.
func (s *Scrobbler) listenBrainz(ctx context.Context, cfg Config, listenType string, t Track, at time.Time) error {
	info := map[string]any{"media_player": "cliamp-x", "submission_client": "cliamp-x", "submission_client_version": appmeta.Version()}
	if t.Duration > 0 {
		info["duration_ms"] = t.Duration.Milliseconds()
	}
	meta := map[string]any{"artist_name": t.Artist, "track_name": t.Title, "additional_info": info}
	if t.Album != "" {
		meta["release_name"] = t.Album
	}
	item := map[string]any{"track_metadata": meta}
	if listenType == "single" {
		item["listened_at"] = at.Unix()
	}
	body, err := json.Marshal(map[string]any{"listen_type": listenType, "payload": []any{item}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.listenBrainzURL+"/1/submit-listens", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+cfg.ListenBrainzToken)
	req.Header.Set("Content-Type", "application/json")
	resp, status, err := s.post(req)
	if err == nil && status/100 != 2 {
		err = fmt.Errorf("HTTP %d: %s", status, strings.TrimSpace(string(resp)))
	}
	if err != nil {
		return fmt.Errorf("listenbrainz %s: %w", listenType, err)
	}
	return nil
}

// ErrInvalidToken is the answer of ValidateListenBrainz to a token that
// ListenBrainz does not know.
var ErrInvalidToken = errors.New("listenbrainz: the token is not valid")

// ValidateListenBrainz asks ListenBrainz whether token belongs to a user.
func (s *Scrobbler) ValidateListenBrainz(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.listenBrainzURL+"/1/validate-token", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+token)
	body, status, err := s.post(req)
	if err == nil && status/100 != 2 {
		err = fmt.Errorf("HTTP %d", status)
	}
	var answer struct {
		Valid bool `json:"valid"`
	}
	if err == nil {
		err = json.Unmarshal(body, &answer)
	}
	if err != nil {
		return fmt.Errorf("listenbrainz validate-token: %w", err)
	}
	if !answer.Valid {
		return ErrInvalidToken
	}
	return nil
}

// trackParams are the Last.fm parameters of t, with the timestamp of a
// scrobble when at is set.
func trackParams(t Track, at time.Time) url.Values {
	v := url.Values{"artist": {t.Artist}, "track": {t.Title}}
	if t.Album != "" {
		v.Set("album", t.Album)
	}
	if t.Duration > 0 {
		v.Set("duration", strconv.Itoa(int(t.Duration.Seconds())))
	}
	if !at.IsZero() {
		v.Set("timestamp", strconv.FormatInt(at.Unix(), 10))
	}
	return v
}

// lastfmTempCodes are the Last.fm errors that may pass: service offline,
// temporarily unavailable, and the rate limit.
var lastfmTempCodes = map[int]bool{11: true, 16: true, 29: true}

// lastfm calls method with params, signed with the user's secret, and
// returns the JSON answer. A session method gets the session key.
func (s *Scrobbler) lastfm(ctx context.Context, cfg Config, method string, params url.Values) (map[string]json.RawMessage, error) {
	params.Set("method", method)
	params.Set("api_key", cfg.LastfmKey)
	if strings.HasPrefix(method, "track.") {
		params.Set("sk", cfg.LastfmSession)
	}
	params.Set("api_sig", lastfmSignature(params, cfg.LastfmSecret))
	params.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.lastfmURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, status, err := s.post(req)
	if err != nil {
		return nil, fmt.Errorf("last.fm %s: %w", method, err)
	}
	var answer map[string]json.RawMessage
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, fmt.Errorf("last.fm %s: HTTP %d: %w", method, status, err)
	}
	if raw, ok := answer["error"]; ok {
		var code int
		_ = json.Unmarshal(raw, &code)
		var message string
		_ = json.Unmarshal(answer["message"], &message)
		err := fmt.Errorf("last.fm %s: error %d: %s", method, code, message)
		if lastfmTempCodes[code] {
			return nil, tempError{err}
		}
		return nil, err
	}
	return answer, nil
}

// lastfmSignature is api_sig: the MD5 of the parameters sorted by name, each
// name followed by its value, and then the secret. format is not signed.
func lastfmSignature(params url.Values, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "format" && k != "callback" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteString(params.Get(k))
	}
	b.WriteString(secret)
	sum := md5.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// LastfmToken starts the Last.fm sign-in with the API key and secret of
// cfg. It returns a token and the page where the user allows cliamp-x to
// scrobble; LastfmSession then trades the token for a session key.
func (s *Scrobbler) LastfmToken(ctx context.Context, cfg Config) (token, authURL string, err error) {
	if cfg.LastfmKey == "" || cfg.LastfmSecret == "" {
		return "", "", errors.New("last.fm: set lastfm_api_key and lastfm_api_secret first")
	}
	answer, err := s.lastfm(ctx, cfg, "auth.getToken", url.Values{})
	if err != nil {
		return "", "", err
	}
	if err := json.Unmarshal(answer["token"], &token); err != nil || token == "" {
		return "", "", errors.New("last.fm auth.getToken: no token in the answer")
	}
	return token, "https://www.last.fm/api/auth/?" + url.Values{"api_key": {cfg.LastfmKey}, "token": {token}}.Encode(), nil
}

// LastfmSession trades a token that the user allowed for a session key.
func (s *Scrobbler) LastfmSession(ctx context.Context, cfg Config, token string) (string, error) {
	if cfg.LastfmKey == "" || cfg.LastfmSecret == "" {
		return "", errors.New("last.fm: set lastfm_api_key and lastfm_api_secret first")
	}
	answer, err := s.lastfm(ctx, cfg, "auth.getSession", url.Values{"token": {token}})
	if err != nil {
		return "", err
	}
	var session struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(answer["session"], &session); err != nil || session.Key == "" {
		return "", errors.New("last.fm auth.getSession: no session in the answer")
	}
	return session.Key, nil
}

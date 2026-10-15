package model

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/internal/scrobble"
	"github.com/xaviiromf/cliamp-x/ipc"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// Scrobbling to ListenBrainz and Last.fm rides on the playback reports: the
// now-playing report at track start and the scrobble when a track that
// played past half its length is left.

// scrobbleMinLength is the shortest song the services take.
const scrobbleMinLength = 30 * time.Second

// scrobbleKeys maps the settings.set names of the credentials, which are
// also their [scrobble] keys, to their Config field.
var scrobbleKeys = map[string]func(*scrobble.Config) *string{
	"listenbrainz_token": func(c *scrobble.Config) *string { return &c.ListenBrainzToken },
	"lastfm_api_key":     func(c *scrobble.Config) *string { return &c.LastfmKey },
	"lastfm_api_secret":  func(c *scrobble.Config) *string { return &c.LastfmSecret },
	"lastfm_session_key": func(c *scrobble.Config) *string { return &c.LastfmSession },
}

// SetScrobbler turns scrobbling on with s.
func (m *Model) SetScrobbler(s *scrobble.Scrobbler) { m.scrobbler = s }

// scrobbleTrack is track as the services take it. A radio stream, or a
// song with no artist or title, is not reported.
func scrobbleTrack(track playlist.Track, dur time.Duration) (scrobble.Track, bool) {
	if track.Stream || track.Artist == "" || track.Title == "" {
		return scrobble.Track{}, false
	}
	if dur <= 0 {
		dur = time.Duration(track.DurationSecs) * time.Second
	}
	return scrobble.Track{Artist: track.Artist, Title: track.Title, Album: track.Album, Duration: dur}, true
}

// scrobbleServices reports which services take reports: ListenBrainz and
// Last.fm.
func (m *Model) scrobbleServices() (listenBrainz, lastfm bool) {
	if m.scrobbler == nil {
		return false, false
	}
	cfg := m.scrobbler.Config()
	return cfg.ListenBrainzOn(), cfg.LastfmOn()
}

// scrobblerOn returns the scrobbler when a service is on.
func (m *Model) scrobblerOn() *scrobble.Scrobbler {
	if m.scrobbler == nil {
		return nil
	}
	if cfg := m.scrobbler.Config(); !cfg.ListenBrainzOn() && !cfg.LastfmOn() {
		return nil
	}
	return m.scrobbler
}

// scrobbleNowPlaying reports that track starts.
func (m *Model) scrobbleNowPlaying(track playlist.Track) {
	s := m.scrobblerOn()
	t, ok := scrobbleTrack(track, m.player.Duration())
	if s == nil || !ok {
		return
	}
	m.queueReport("", func() {
		if err := s.NowPlaying(context.Background(), t); err != nil {
			applog.Warn("scrobble now playing %q: %v", t.Title, err)
		}
	})
}

// scrobbleListen reports that track was listened to for elapsed of dur.
// maybeScrobble has checked that it played past half its length.
func (m *Model) scrobbleListen(track playlist.Track, elapsed, dur time.Duration) {
	s := m.scrobblerOn()
	t, ok := scrobbleTrack(track, dur)
	if s == nil || !ok || t.Duration < scrobbleMinLength {
		return
	}
	at := time.Now().Add(-elapsed)
	m.queueReport("", func() {
		if err := s.Scrobble(context.Background(), t, at); err != nil {
			applog.Warn("scrobble %q: %v", t.Title, err)
		}
	})
}

// setScrobbleSetting serves the credential names of settings.set and saves
// the value in [scrobble]. An empty value turns the service off. It reports
// false for another name or a value with a line break.
func (m *Model) setScrobbleSetting(name, value string) (bool, error) {
	field, ok := scrobbleKeys[name]
	if !ok || m.scrobbler == nil || strings.ContainsAny(value, "\r\n") {
		return false, nil
	}
	cfg := m.scrobbler.Config()
	*field(&cfg) = value
	m.scrobbler.SetConfig(cfg)
	if s, ok := m.configSaver.(sectionStringSaver); ok {
		return true, m.reportConfigSave(s.SaveSectionString("scrobble", name, value))
	}
	return true, nil
}

// checkListenBrainzToken asks ListenBrainz about token off the event loop
// for settings.set; handleV2Response saves it only when it is valid, so a
// mistyped token never shows as connected.
func (m *Model) checkListenBrainzToken(ctx context.Context, jobs *ipc.JobStore, jobID, token string) tea.Cmd {
	s := m.scrobbler
	return func() tea.Msg {
		msg := ipcV2ResponseMsg{Jobs: jobs, JobID: jobID, Operation: "settings.set", Response: ipc.Response{OK: true}, credName: "listenbrainz_token", credValue: token}
		if err := s.ValidateListenBrainz(ctx, token); err != nil {
			msg.Response = ipc.Response{Error: err.Error()}
		}
		return msg
	}
}

// handleV2LastfmAuth runs the Last.fm sign-in off the event loop: start
// returns the page where the user allows cliamp-x and its token, finish
// trades that token, in text, for the session key, which handleV2Response
// saves.
func (m *Model) handleV2LastfmAuth(ctx context.Context, jobs *ipc.JobStore, jobID string, request ipc.Request) tea.Cmd {
	if m.scrobbler == nil {
		m.failV2Job(jobs, jobID, v2UnavailableError())
		return nil
	}
	s, cfg := m.scrobbler, m.scrobbler.Config()
	token := strings.TrimSpace(request.Text)
	if cfg.LastfmKey == "" || cfg.LastfmSecret == "" || (request.Name != "start" && request.Name != "finish") ||
		(request.Name == "finish" && token == "") {
		err := v2InvalidParamsError()
		err.Detail = "set lastfm_api_key and lastfm_api_secret, then call start, then finish with its token in text"
		m.failV2Job(jobs, jobID, err)
		return nil
	}
	return func() tea.Msg {
		msg := ipcV2ResponseMsg{Jobs: jobs, JobID: jobID, Operation: "lastfm.auth", Response: ipc.Response{OK: true}}
		var err error
		if request.Name == "start" {
			msg.Response.Token, msg.Response.URL, err = s.LastfmToken(ctx, cfg)
		} else {
			msg.credName = "lastfm_session_key"
			msg.credValue, err = s.LastfmSession(ctx, cfg, token)
		}
		if err != nil {
			msg.Response = ipc.Response{Error: err.Error()}
		}
		return msg
	}
}

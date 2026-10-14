package model

import (
	"errors"
	"strings"

	"github.com/xaviiromf/cliamp-x/playlist"
)

// setPlayError records why the track at path failed to start, for the
// runtime snapshot. A nil err clears it. Browse errors stay in m.err.
func (m *Model) setPlayError(path string, err error) {
	m.playErr = err
	m.playErrPath = ""
	if err != nil {
		m.playErrPath = path
	}
}

// playErrorKind sorts the raw reason a song failed to start: network,
// session, ytdlp, unavailable, or "" when the reason is not a known one.
func playErrorKind(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, playlist.ErrNeedsAuth) {
		return "session"
	}
	msg := strings.ToLower(err.Error())
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(msg, p) {
				return true
			}
		}
		return false
	}
	switch {
	case has("temporary failure in name resolution", "network is unreachable", "no route to host",
		"connection refused", "timed out", "getaddrinfo", "no such host", "connection reset"):
		return "network"
	case has("not a bot", "sign in to confirm", "confirm your age", "cookies", "login required",
		"members-only", "music premium", "http error 401"):
		return "session"
	case has("unable to extract", "nsig", "unsupported url", "requested format is not available",
		"please report this issue", "player response", "signature", "http error 403"):
		return "ytdlp"
	case has("video unavailable", "video is unavailable", "private video", "has been removed", "not available in your country", "geo"):
		return "unavailable"
	}
	return ""
}

// playErrorHint turns the raw reason a YouTube song failed to start into
// what to do about it, or "" when the reason is not a known one. The raw
// text still goes to the log.
func playErrorHint(err error) string {
	switch playErrorKind(err) {
	case "network":
		return L("No connection to YouTube: check the network")
	case "session":
		return L("The YouTube session may have expired: run cliamp-x login")
	case "ytdlp":
		return L("yt-dlp may be out of date: update it, for example with yt-dlp -U or your package manager")
	case "unavailable":
		return L("This video is unavailable, private or blocked in your country")
	}
	return ""
}

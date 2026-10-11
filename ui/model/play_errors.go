package model

import "strings"

// playErrorHint turns the raw reason a YouTube song failed to start into
// what to do about it, or "" when the reason is not a known one. The raw
// text still goes to the log.
func playErrorHint(err error) string {
	if err == nil {
		return ""
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
		return L("No connection to YouTube: check the network")
	case has("not a bot", "sign in to confirm", "confirm your age", "cookies", "login required",
		"members-only", "music premium", "http error 401"):
		return L("The YouTube session may have expired: run cliamp-x login")
	case has("unable to extract", "nsig", "unsupported url", "requested format is not available",
		"please report this issue", "player response", "signature", "http error 403"):
		return L("yt-dlp may be out of date: update it, for example with yt-dlp -U or your package manager")
	case has("video unavailable", "private video", "has been removed", "not available in your country", "geo"):
		return L("This video is unavailable, private or blocked in your country")
	}
	return ""
}

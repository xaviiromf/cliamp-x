// Package ytdlcookies stores browser cookie sources for yt-dlp-backed hosts.
package ytdlcookies

import (
	"net/url"
	"strings"
	"sync"

	"github.com/xaviiromf/cliamp-x/playlist"
)

var (
	mu     sync.RWMutex
	byHost = make(map[string]string)
)

// SetForHost associates a browser cookie source with a URL host. Passing an
// empty browser removes the association.
func SetForHost(host, browser string) {
	host = playlist.NormalizeHost(host)
	if host == "" {
		return
	}

	mu.Lock()
	defer mu.Unlock()

	browser = strings.TrimSpace(browser)
	if browser == "" {
		delete(byHost, host)
		return
	}
	byHost[host] = browser
}

// ForURL returns the browser cookie source associated with rawURL. yt-dlp
// search prefixes are mapped to the service host they query.
func ForURL(rawURL string) string {
	host := hostForURL(rawURL)
	if host == "" {
		return ""
	}

	mu.RLock()
	defer mu.RUnlock()
	return byHost[host]
}

func hostForURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	lower := strings.ToLower(rawURL)
	switch {
	case strings.HasPrefix(lower, "scsearch"):
		return "soundcloud.com"
	case strings.HasPrefix(lower, "ytsearch"):
		return "youtube.com"
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return playlist.NormalizeHost(u.Hostname())
}

// FilePrefix marks a cookie source that is a cookies.txt file rather than a
// browser, as in "file:/home/me/.config/cliamp-x/ytmusic_cookies.txt".
const FilePrefix = "file:"

// Args returns the yt-dlp arguments for a cookie source: --cookies for a
// file source, --cookies-from-browser for a browser, nothing for "".
func Args(source string) []string {
	source = strings.TrimSpace(source)
	switch {
	case source == "":
		return nil
	case strings.HasPrefix(source, FilePrefix):
		return []string{"--cookies", strings.TrimPrefix(source, FilePrefix)}
	default:
		return []string{"--cookies-from-browser", source}
	}
}

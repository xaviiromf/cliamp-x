package ytmusic

import (
	"context"
	"net/url"
	"regexp"
	"strings"
)

// LinkKind is what a YouTube or YouTube Music link points to.
type LinkKind uint8

const (
	LinkVideo LinkKind = iota + 1
	LinkPlaylist
	LinkAlbum
	LinkArtist
	LinkUnknown // a YouTube page that only the API can resolve, such as @handle
)

// Link is a parsed YouTube or YouTube Music link.
type Link struct {
	Kind       LinkKind
	VideoID    string // the video of a watch link
	PlaylistID string // the list of a playlist or watch link, without VL
	BrowseID   string // album (MPREb_...) or artist channel (UC...)
	Raw        string
}

var videoIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{11}$`)

// WatchURL returns the canonical YouTube Music URL of a video. Every track of
// the provider uses it, so favorites and history match across sources.
func WatchURL(videoID string) string {
	return "https://music.youtube.com/watch?v=" + videoID
}

// VideoIDFromPath returns the video ID of a YouTube or YouTube Music watch
// URL, or "".
func VideoIDFromPath(path string) string {
	link, ok := ParseLink(path)
	if !ok || link.Kind != LinkVideo {
		return ""
	}
	return link.VideoID
}

// ParseLink parses a YouTube, YouTube Music or youtu.be link. It reports
// false for text that is not such a link.
func ParseLink(raw string) (Link, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, " \t\n") {
		return Link{}, false
	}
	if !strings.Contains(raw, "://") {
		lower := strings.ToLower(raw)
		if !strings.HasPrefix(lower, "youtu") && !strings.HasPrefix(lower, "www.youtu") &&
			!strings.HasPrefix(lower, "music.youtube") && !strings.HasPrefix(lower, "m.youtube") {
			return Link{}, false
		}
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Link{}, false
	}
	host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
	q := u.Query()
	link := Link{Raw: raw}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch host {
	case "youtu.be":
		if len(parts) > 0 && videoIDRe.MatchString(parts[0]) {
			link.Kind, link.VideoID, link.PlaylistID = LinkVideo, parts[0], q.Get("list")
			return link, true
		}
		return Link{}, false
	case "youtube.com", "m.youtube.com", "music.youtube.com", "youtube-nocookie.com":
	default:
		return Link{}, false
	}
	switch {
	case len(parts) >= 1 && parts[0] == "watch":
		if v := q.Get("v"); videoIDRe.MatchString(v) {
			link.Kind, link.VideoID, link.PlaylistID = LinkVideo, v, q.Get("list")
			return link, true
		}
		if l := q.Get("list"); l != "" {
			link.Kind, link.PlaylistID = LinkPlaylist, l
			return link, true
		}
	case len(parts) >= 1 && parts[0] == "playlist":
		if l := q.Get("list"); l != "" {
			link.Kind, link.PlaylistID = LinkPlaylist, strings.TrimPrefix(l, "VL")
			return link, true
		}
	case len(parts) >= 2 && (parts[0] == "shorts" || parts[0] == "embed" || parts[0] == "live" || parts[0] == "v"):
		if videoIDRe.MatchString(parts[1]) {
			link.Kind, link.VideoID = LinkVideo, parts[1]
			return link, true
		}
	case len(parts) >= 2 && parts[0] == "browse":
		id := parts[1]
		switch {
		case strings.HasPrefix(id, "MPREb_"):
			link.Kind, link.BrowseID = LinkAlbum, id
		case strings.HasPrefix(id, "VL"):
			link.Kind, link.PlaylistID = LinkPlaylist, strings.TrimPrefix(id, "VL")
		case strings.HasPrefix(id, "UC") || strings.HasPrefix(id, "MPLAUC"):
			link.Kind, link.BrowseID = LinkArtist, channelID(id)
		default:
			link.Kind = LinkUnknown
		}
		return link, true
	case len(parts) >= 2 && parts[0] == "channel":
		link.Kind, link.BrowseID = LinkArtist, parts[1]
		return link, true
	case len(parts) >= 1 && (strings.HasPrefix(parts[0], "@") || parts[0] == "c" || parts[0] == "user"):
		link.Kind = LinkUnknown
		return link, true
	}
	return Link{}, false
}

// resolveLink asks the API what an unknown YouTube link points to, such as
// an artist @handle.
func (c *Client) resolveLink(ctx context.Context, link Link) (Link, error) {
	resp, err := c.call(ctx, "navigation/resolve_url", map[string]any{"url": link.Raw}, "")
	if err != nil {
		return link, err
	}
	ep := obj(resp, "endpoint")
	if be := obj(ep, "browseEndpoint"); be != nil {
		id := str(be, "browseId")
		switch {
		case strings.HasPrefix(id, "MPREb_"):
			link.Kind, link.BrowseID = LinkAlbum, id
		case strings.HasPrefix(id, "VL"):
			link.Kind, link.PlaylistID = LinkPlaylist, strings.TrimPrefix(id, "VL")
		case strings.HasPrefix(id, "UC"):
			link.Kind, link.BrowseID = LinkArtist, id
		}
		return link, nil
	}
	if we := obj(ep, "watchEndpoint"); we != nil {
		link.Kind, link.VideoID, link.PlaylistID = LinkVideo, str(we, "videoId"), str(we, "playlistId")
	}
	return link, nil
}

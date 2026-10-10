// Package sponsorblock looks up the community-marked segments of YouTube
// videos (sponsor reads, intros, outros, non-music parts of music videos)
// on the SponsorBlock API, so the player can skip them.
//
// Lookups use the API's privacy mode: only the first four hex digits of the
// SHA-256 of the video ID are sent, and the matching video is picked from
// the answer locally.
package sponsorblock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/httpclient"
)

// DefaultAPI is the public SponsorBlock server.
const DefaultAPI = "https://sponsor.ajay.app"

// DefaultCategories are the segments worth skipping in music.
var DefaultCategories = []string{"music_offtopic", "intro", "outro", "sponsor", "selfpromo"}

// KnownCategories lists every category the API knows.
var KnownCategories = []string{"sponsor", "selfpromo", "interaction", "intro", "outro", "preview", "music_offtopic", "filler"}

// Segment is one part of a video to skip, in seconds from its start.
type Segment struct {
	Start, End float64
	Category   string
	UUID       string
}

// Client fetches and caches segments. It is safe for concurrent use.
type Client struct {
	API        string
	Categories []string
	HTTP       *http.Client

	mu    sync.Mutex
	cache map[string][]Segment
}

// New returns a client for the public server with the given categories, or
// the defaults when categories is empty.
func New(categories []string) *Client {
	if len(categories) == 0 {
		categories = DefaultCategories
	}
	return &Client{
		API:        DefaultAPI,
		Categories: categories,
		HTTP:       httpclient.NewAPI(10 * time.Second),
		cache:      map[string][]Segment{},
	}
}

// Cached returns the segments of videoID when they were looked up before.
func (c *Client) Cached(videoID string) ([]Segment, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.cache[videoID]
	return s, ok
}

// SetCached stores the segments of videoID as if they were looked up.
func (c *Client) SetCached(videoID string, segs []Segment) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[videoID] = merge(segs)
}

// Segments returns the segments of videoID sorted by start, or none when the
// video has none. Answers are cached for the run, empty ones included.
func (c *Client) Segments(ctx context.Context, videoID string) ([]Segment, error) {
	if s, ok := c.Cached(videoID); ok {
		return s, nil
	}
	segs, err := c.fetch(ctx, videoID)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.cache[videoID] = segs
	c.mu.Unlock()
	return segs, nil
}

type apiVideo struct {
	VideoID  string `json:"videoID"`
	Segments []struct {
		Category   string    `json:"category"`
		ActionType string    `json:"actionType"`
		Segment    []float64 `json:"segment"`
		UUID       string    `json:"UUID"`
	} `json:"segments"`
}

func (c *Client) fetch(ctx context.Context, videoID string) ([]Segment, error) {
	sum := sha256.Sum256([]byte(videoID))
	prefix := hex.EncodeToString(sum[:])[:4]
	cats, _ := json.Marshal(c.Categories)
	q := url.Values{}
	q.Set("categories", string(cats))
	q.Set("actionTypes", `["skip"]`)
	endpoint := strings.TrimRight(c.API, "/") + "/api/skipSegments/" + prefix + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sponsorblock: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // no video with this prefix has segments
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sponsorblock: HTTP %s", resp.Status)
	}
	var videos []apiVideo
	if err := httpclient.ReadJSON(resp.Body, 4<<20, &videos); err != nil {
		return nil, fmt.Errorf("sponsorblock: %w", err)
	}
	var segs []Segment
	for _, v := range videos {
		if v.VideoID != videoID {
			continue
		}
		for _, s := range v.Segments {
			if len(s.Segment) != 2 || s.Segment[1] <= s.Segment[0] {
				continue
			}
			if s.ActionType != "" && s.ActionType != "skip" {
				continue
			}
			if !slices.Contains(c.Categories, s.Category) {
				continue
			}
			segs = append(segs, Segment{Start: s.Segment[0], End: s.Segment[1], Category: s.Category, UUID: s.UUID})
		}
	}
	return merge(segs), nil
}

// merge sorts segments and joins the ones that overlap, so one skip covers
// back-to-back segments.
func merge(segs []Segment) []Segment {
	if len(segs) < 2 {
		return segs
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].Start < segs[j].Start })
	out := segs[:1]
	for _, s := range segs[1:] {
		last := &out[len(out)-1]
		if s.Start <= last.End+0.5 {
			if s.End > last.End {
				last.End = s.End
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

// VideoID returns the YouTube video ID of a watch, youtu.be or shorts URL,
// or "" when path is not one.
func VideoID(path string) string {
	u, err := url.Parse(path)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Host), "www.")
	var id string
	switch {
	case host == "youtu.be":
		id = strings.Trim(u.Path, "/")
	case host == "youtube.com" || host == "music.youtube.com" || host == "m.youtube.com":
		if strings.HasPrefix(u.Path, "/shorts/") {
			id = strings.TrimPrefix(u.Path, "/shorts/")
		} else {
			id = u.Query().Get("v")
		}
	}
	if len(id) != 11 {
		return ""
	}
	return id
}

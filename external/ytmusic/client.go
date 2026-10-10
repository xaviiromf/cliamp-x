package ytmusic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	apiBase = "https://music.youtube.com/youtubei/v1/"
	// defaultClientVersion is used until the live version is read from the
	// music.youtube.com page. InnerTube accepts older versions for months.
	defaultClientVersion = "1.20260930.01.00"
	userAgent            = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36"
	requestTimeout       = 20 * time.Second
	maxResponseBytes     = 32 << 20
)

// ErrSignInRequired reports a request that needs a YouTube sign-in.
var ErrSignInRequired = errors.New("sign in to YouTube Music first: run `cliamp-x login`")

// Client calls the InnerTube API of music.youtube.com.
type Client struct {
	http *http.Client
	auth *Auth
	hl   string
	gl   string
	base string // API base URL; tests point it at a local server

	mu            sync.Mutex
	clientVersion string
	visitorData   string
	versionTried  bool
}

// NewClient returns a client for the given sign-in, language (hl) and
// country (gl). Empty hl and gl come from $LANG.
func NewClient(auth *Auth, hl, gl string) *Client {
	if auth == nil {
		auth = &Auth{}
	}
	lhl, lgl := localeFromEnv()
	if hl == "" {
		hl = lhl
	}
	if gl == "" {
		gl = lgl
	}
	return &Client{
		http:          &http.Client{Timeout: requestTimeout},
		auth:          auth,
		hl:            hl,
		gl:            gl,
		base:          apiBase,
		clientVersion: defaultClientVersion,
	}
}

// localeFromEnv reads the language and country from $LC_ALL, $LC_MESSAGES
// or $LANG, such as es_CL.UTF-8. It falls back to English and the US.
func localeFromEnv() (hl, gl string) {
	hl, gl = "en", "US"
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := os.Getenv(name)
		if v == "" || v == "C" || v == "POSIX" || strings.HasPrefix(v, "C.") {
			continue
		}
		v, _, _ = strings.Cut(v, ".")
		v, _, _ = strings.Cut(v, "@")
		lang, country, _ := strings.Cut(v, "_")
		if len(lang) >= 2 {
			hl = strings.ToLower(lang)
		}
		if len(country) == 2 {
			gl = strings.ToUpper(country)
		}
		return hl, gl
	}
	return hl, gl
}

// SignedIn reports whether the client sends a YouTube sign-in.
func (c *Client) SignedIn() bool { return c.auth.SignedIn() }

// Auth returns the sign-in of the client.
func (c *Client) Auth() *Auth { return c.auth }

var (
	clientVersionRe = regexp.MustCompile(`"INNERTUBE_CLIENT_VERSION":"([0-9.]+)"`)
	visitorDataRe   = regexp.MustCompile(`"VISITOR_DATA":"([^"]+)"`)
)

// refreshClientVersion reads the current client version and visitor data
// from the music.youtube.com page, once per run. A failure keeps the
// defaults.
func (c *Client) refreshClientVersion(ctx context.Context) {
	c.mu.Lock()
	if c.versionTried {
		c.mu.Unlock()
		return
	}
	c.versionTried = true
	c.mu.Unlock()
	if c.base != apiBase {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/", nil)
	if err != nil {
		return
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", c.hl)
	if cookie := c.auth.CookieHeader(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if m := clientVersionRe.FindSubmatch(body); m != nil {
		c.clientVersion = string(m[1])
	}
	if m := visitorDataRe.FindSubmatch(body); m != nil {
		c.visitorData = string(m[1])
	}
}

func (c *Client) versionAndVisitor() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.clientVersion, c.visitorData
}

// requestContext returns the InnerTube "context" object of a request body.
func (c *Client) requestContext() map[string]any {
	version, _ := c.versionAndVisitor()
	return map[string]any{
		"client": map[string]any{
			"clientName":    "WEB_REMIX",
			"clientVersion": version,
			"hl":            c.hl,
			"gl":            c.gl,
		},
		"user": map[string]any{},
	}
}

// call posts body to an InnerTube endpoint, such as "browse", and returns
// the decoded response.
func (c *Client) call(ctx context.Context, endpoint string, body map[string]any, query string) (map[string]any, error) {
	c.refreshClientVersion(ctx)
	if body == nil {
		body = map[string]any{}
	}
	body["context"] = c.requestContext()
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	url := c.base + endpoint + "?prettyPrint=false"
	if query != "" {
		url += "&" + query
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	version, visitor := c.versionAndVisitor()
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept-Language", c.hl)
	req.Header.Set("Origin", origin)
	req.Header.Set("X-Origin", origin)
	req.Header.Set("Referer", origin+"/")
	req.Header.Set("X-Youtube-Client-Name", "67")
	req.Header.Set("X-Youtube-Client-Version", version)
	if visitor != "" {
		req.Header.Set("X-Goog-Visitor-Id", visitor)
	}
	if cookie := c.auth.CookieHeader(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if authz := c.auth.AuthorizationHeader(time.Now()); authz != "" {
		req.Header.Set("Authorization", authz)
		req.Header.Set("X-Goog-AuthUser", "0")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("youtube music %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("youtube music %s: %w", endpoint, err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if !c.auth.SignedIn() {
			return nil, ErrSignInRequired
		}
		return nil, fmt.Errorf("youtube music %s: the sign-in was refused (HTTP %d); run `cliamp-x login` again", endpoint, resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube music %s: HTTP %d: %s", endpoint, resp.StatusCode, apiErrorMessage(data))
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("youtube music %s: decode: %w", endpoint, err)
	}
	return out, nil
}

// apiErrorMessage extracts error.message from an InnerTube error body.
func apiErrorMessage(data []byte) string {
	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &body) == nil && body.Error.Message != "" {
		return body.Error.Message
	}
	s := strings.TrimSpace(string(data))
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

// browse calls the browse endpoint with a browse ID and optional params.
func (c *Client) browse(ctx context.Context, browseID, params string) (map[string]any, error) {
	body := map[string]any{"browseId": browseID}
	if params != "" {
		body["params"] = params
	}
	return c.call(ctx, "browse", body, "")
}

// browseContinuation fetches the next page of a browse result.
func (c *Client) browseContinuation(ctx context.Context, token string) (map[string]any, error) {
	return c.call(ctx, "browse", map[string]any{}, "ctoken="+token+"&continuation="+token+"&type=next")
}

// Package ytmusic is the YouTube Music provider of cliamp-x. It talks to the
// InnerTube API that music.youtube.com itself uses, signed in with the
// cookies of the user's browser, and plays songs through yt-dlp.
package ytmusic

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/internal/appdir"
	"github.com/xaviiromf/cliamp-x/internal/browsercookies"
)

const (
	cookieFileName = "ytmusic_cookies.txt"
	authFileName   = "ytmusic_auth.json"
	origin         = "https://music.youtube.com"
)

// cookieDomains are the cookie domains cliamp-x copies from the browser.
// yt-dlp needs the google.com cookies to refresh a session.
var cookieDomains = []string{"youtube.com", "google.com"}

// authRecord remembers which browser profile holds the sign-in.
type authRecord struct {
	Browser   string    `json:"browser,omitempty"`
	Profile   string    `json:"profile,omitempty"`
	LoggedOut bool      `json:"logged_out,omitempty"`
	Updated   time.Time `json:"updated,omitzero"`
}

// Auth is the YouTube Music sign-in: the cookies copied from the browser.
// A zero Auth is signed out.
type Auth struct {
	mu      sync.RWMutex
	cookies []browsercookies.Cookie
	source  string
}

// CookiePath returns the path of the saved cookie jar. yt-dlp reads it with
// --cookies.
func CookiePath() (string, error) {
	dir, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cookieFileName), nil
}

func authPath() (string, error) {
	dir, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, authFileName), nil
}

func loadRecord() authRecord {
	var rec authRecord
	path, err := authPath()
	if err != nil {
		return rec
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return rec
	}
	_ = json.Unmarshal(data, &rec)
	return rec
}

func saveRecord(rec authRecord) error {
	path, err := authPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	rec.Updated = time.Now().UTC()
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// Login copies the YouTube sign-in of the browser that spec names (yt-dlp
// syntax, "" to detect one) and remembers that profile, so later starts
// refresh the cookies from it without asking.
func Login(spec string) (browsercookies.Source, error) {
	cookies, src, err := browsercookies.Read(spec, cookieDomains, "SAPISID")
	if err != nil {
		return src, err
	}
	path, err := CookiePath()
	if err != nil {
		return src, err
	}
	if err := browsercookies.WriteNetscape(path, cookies); err != nil {
		return src, fmt.Errorf("save cookies: %w", err)
	}
	return src, saveRecord(authRecord{Browser: src.Browser, Profile: src.Profile})
}

// Logout deletes the saved cookies and stops the automatic sign-in until the
// next Login. It does not sign the browser out.
func Logout() error {
	path, err := CookiePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return saveRecord(authRecord{LoggedOut: true})
}

// LoadAuth returns the sign-in for this run. It refreshes the cookies from
// the remembered browser profile, or from spec when the config names a
// browser, so a session the browser renewed carries over. When the browser
// cannot be read it falls back to the saved jar. With no sign-in at all it
// returns a signed-out Auth, which still browses and searches.
func LoadAuth(spec string) *Auth {
	rec := loadRecord()
	if rec.LoggedOut {
		return &Auth{}
	}
	source := spec
	if strings.TrimSpace(source) == "" && rec.Browser != "" {
		source = rec.Browser + ":" + rec.Profile
	}
	path, pathErr := CookiePath()

	cookies, src, err := browsercookies.Read(source, cookieDomains, "SAPISID")
	if err == nil {
		if pathErr == nil {
			if werr := browsercookies.WriteNetscape(path, cookies); werr != nil {
				applog.Warn("ytmusic: save cookies: %v", werr)
			}
		}
		if rec.Browser != src.Browser || rec.Profile != src.Profile {
			_ = saveRecord(authRecord{Browser: src.Browser, Profile: src.Profile})
		}
		applog.Info("ytmusic: signed in from %s", src.Browser)
		return &Auth{cookies: cookies, source: src.Browser}
	}
	applog.Info("ytmusic: browser sign-in unavailable: %v", err)

	if pathErr == nil {
		if saved, rerr := browsercookies.ReadNetscape(path); rerr == nil && hasSignIn(saved) {
			applog.Info("ytmusic: using the saved sign-in")
			return &Auth{cookies: saved, source: "saved"}
		}
	}
	return &Auth{}
}

// NewAuth returns an Auth over the given cookies. Tests use it.
func NewAuth(cookies []browsercookies.Cookie) *Auth {
	return &Auth{cookies: cookies}
}

func hasSignIn(cookies []browsercookies.Cookie) bool {
	for _, c := range cookies {
		if c.Name == "SAPISID" && c.Value != "" {
			return true
		}
	}
	return false
}

// SignedIn reports whether the cookies hold a YouTube sign-in.
func (a *Auth) SignedIn() bool {
	if a == nil {
		return false
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return hasSignIn(a.cookies)
}

// Source names where the sign-in came from, such as "brave-origin".
func (a *Auth) Source() string {
	if a == nil {
		return ""
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.source
}

// cookie returns the value of the youtube.com cookie name.
func (a *Auth) cookie(name string) string {
	for _, c := range a.cookies {
		if c.Name == name && strings.HasSuffix(strings.TrimPrefix(c.Host, "."), "youtube.com") {
			return c.Value
		}
	}
	return ""
}

// CookieHeader returns the Cookie header for music.youtube.com.
func (a *Auth) CookieHeader() string {
	if a == nil {
		return ""
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	now := time.Now().Unix()
	seen := map[string]bool{}
	var parts []string
	for _, c := range a.cookies {
		host := strings.TrimPrefix(c.Host, ".")
		if host != "youtube.com" && host != "music.youtube.com" {
			continue
		}
		if c.Expires != 0 && c.Expires < now {
			continue
		}
		if seen[c.Name] {
			continue
		}
		seen[c.Name] = true
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// AuthorizationHeader returns the SAPISIDHASH Authorization header that
// music.youtube.com sends, or "" when signed out.
func (a *Auth) AuthorizationHeader(now time.Time) string {
	if a == nil {
		return ""
	}
	a.mu.RLock()
	defer a.mu.RUnlock()
	ts := strconv.FormatInt(now.Unix(), 10)
	var parts []string
	for _, h := range []struct{ scheme, cookie string }{
		{"SAPISIDHASH", "SAPISID"},
		{"SAPISID1PHASH", "__Secure-1PAPISID"},
		{"SAPISID3PHASH", "__Secure-3PAPISID"},
	} {
		value := a.cookie(h.cookie)
		if value == "" {
			continue
		}
		parts = append(parts, h.scheme+" "+ts+"_"+sapisidHash(ts, value))
	}
	return strings.Join(parts, " ")
}

// sapisidHash is SHA-1 over "<timestamp> <SAPISID> <origin>".
func sapisidHash(ts, sapisid string) string {
	sum := sha1.Sum([]byte(ts + " " + sapisid + " " + origin))
	return hex.EncodeToString(sum[:])
}

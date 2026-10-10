// Package browsercookies reads the cookies of a signed-in browser profile on
// Linux, so cliamp-x can reuse the YouTube Music session of the browser. It
// reads Chromium-family browsers (Chrome, Chromium, Brave, Brave Origin,
// Edge, Vivaldi, Opera) and Firefox-family browsers (Firefox, LibreWolf,
// Zen, Floorp, Waterfox), including their Flatpak and Snap installs.
package browsercookies

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Cookie is one browser cookie.
type Cookie struct {
	Host     string // domain, with a leading dot for a domain cookie
	Name     string
	Value    string
	Path     string
	Secure   bool
	HTTPOnly bool
	Expires  int64 // Unix seconds, 0 for a session cookie
}

// Source names the browser profile that cookies came from.
type Source struct {
	Browser string // browser key, such as "brave-origin"
	Profile string // profile directory
}

func (s Source) String() string {
	return s.Browser + ":" + s.Profile
}

// ErrNoBrowser reports that no supported browser profile was found.
var ErrNoBrowser = errors.New("no supported browser profile found")

// ErrNotSignedIn reports that the profiles hold no sign-in for the domain.
var ErrNotSignedIn = errors.New("the browser is not signed in to YouTube")

// kind is the cookie store format of a browser family.
type kind uint8

const (
	chromium kind = iota + 1
	firefox
)

// browser describes where one browser keeps its profiles.
type browser struct {
	key   string
	kind  kind
	roots []string // profile roots, relative to $HOME unless absolute
	apps  []string // Secret Service "application" names of the cookie key
}

// browsers lists the supported browsers in auto-detect order.
var browsers = []browser{
	{key: "brave-origin", kind: chromium, apps: []string{"brave"}, roots: []string{
		".config/BraveSoftware/Brave-Origin",
		".config/BraveSoftware/Brave-Origin-Beta",
		".config/BraveSoftware/Brave-Origin-Nightly",
		".config/BraveSoftware/Brave-Origin-Development",
	}},
	{key: "brave", kind: chromium, apps: []string{"brave"}, roots: []string{
		".config/BraveSoftware/Brave-Browser",
		".config/BraveSoftware/Brave-Browser-Beta",
		".config/BraveSoftware/Brave-Browser-Nightly",
		".var/app/com.brave.Browser/config/BraveSoftware/Brave-Browser",
		"snap/brave/current/.config/BraveSoftware/Brave-Browser",
	}},
	{key: "chrome", kind: chromium, apps: []string{"chrome"}, roots: []string{
		".config/google-chrome",
		".config/google-chrome-beta",
		".config/google-chrome-unstable",
		".var/app/com.google.Chrome/config/google-chrome",
	}},
	{key: "chromium", kind: chromium, apps: []string{"chromium"}, roots: []string{
		".config/chromium",
		".var/app/org.chromium.Chromium/config/chromium",
		"snap/chromium/common/chromium",
	}},
	{key: "edge", kind: chromium, apps: []string{"microsoft-edge", "Microsoft Edge"}, roots: []string{
		".config/microsoft-edge",
		".config/microsoft-edge-beta",
		".config/microsoft-edge-dev",
		".var/app/com.microsoft.Edge/config/microsoft-edge",
	}},
	{key: "vivaldi", kind: chromium, apps: []string{"vivaldi"}, roots: []string{
		".config/vivaldi",
		".config/vivaldi-snapshot",
		".var/app/com.vivaldi.Vivaldi/config/vivaldi",
	}},
	{key: "opera", kind: chromium, apps: []string{"opera"}, roots: []string{
		".config/opera",
		".config/opera-beta",
		".var/app/com.opera.Opera/config/opera",
	}},
	{key: "firefox", kind: firefox, roots: []string{
		".mozilla/firefox",
		".config/mozilla/firefox",
		"snap/firefox/common/.mozilla/firefox",
		".var/app/org.mozilla.firefox/.mozilla/firefox",
	}},
	{key: "librewolf", kind: firefox, roots: []string{
		".librewolf",
		".var/app/io.gitlab.librewolf-community/.librewolf",
	}},
	{key: "zen", kind: firefox, roots: []string{
		".zen",
		".var/app/app.zen_browser.zen/.zen",
	}},
	{key: "floorp", kind: firefox, roots: []string{".floorp"}},
	{key: "waterfox", kind: firefox, roots: []string{".waterfox"}},
}

// Browsers returns the supported browser keys in auto-detect order.
func Browsers() []string {
	keys := make([]string, len(browsers))
	for i, b := range browsers {
		keys[i] = b.key
	}
	return keys
}

// aliases maps yt-dlp browser names and common spellings to browser keys.
var aliases = map[string]string{
	"google-chrome":  "chrome",
	"microsoft-edge": "edge",
	"msedge":         "edge",
	"braveorigin":    "brave-origin",
	"brave_origin":   "brave-origin",
	"mozilla":        "firefox",
}

func findBrowser(name string) (browser, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if alias, ok := aliases[name]; ok {
		name = alias
	}
	for _, b := range browsers {
		if b.key == name {
			return b, true
		}
	}
	return browser{}, false
}

// profile is one browser profile with a cookie database.
type profile struct {
	browser browser
	dir     string // profile directory
	db      string // cookie database path
	modTime time.Time
}

// ParseSpec splits a browser spec in yt-dlp syntax, BROWSER[+KEYRING][:PROFILE],
// into the browser name and the profile. The keyring part is ignored: the
// key is found by trying every keyring entry.
func ParseSpec(spec string) (name, profileName string) {
	spec = strings.TrimSpace(spec)
	name, profileName, _ = strings.Cut(spec, ":")
	name, _, _ = strings.Cut(name, "+")
	return strings.TrimSpace(name), strings.TrimSpace(profileName)
}

// Read returns the cookies for the given domains (such as "youtube.com")
// from the browser that spec names. An empty spec tries every supported
// browser and returns the first profile that is signed in, which is the one
// that holds a cookie named signInCookie for the first domain. Read never
// writes to the browser profile.
func Read(spec string, domains []string, signInCookie string) ([]Cookie, Source, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, Source{}, err
	}
	name, profileName := ParseSpec(spec)
	var candidates []profile
	if name == "" {
		for _, b := range browsers {
			candidates = append(candidates, profiles(home, b, "")...)
		}
	} else {
		b, ok := findBrowser(name)
		if !ok {
			return nil, Source{}, fmt.Errorf("unsupported browser %q (supported: %s)", name, strings.Join(Browsers(), ", "))
		}
		candidates = profiles(home, b, profileName)
	}
	if len(candidates) == 0 {
		if name == "" {
			return nil, Source{}, ErrNoBrowser
		}
		return nil, Source{}, fmt.Errorf("%w for %s", ErrNoBrowser, name)
	}
	// The profile used last is the likeliest to hold a live session.
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	var errs []error
	for _, p := range candidates {
		cookies, err := readProfile(p, domains)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s %s: %w", p.browser.key, p.dir, err))
			continue
		}
		if signInCookie != "" && !hasCookie(cookies, domains, signInCookie) {
			errs = append(errs, fmt.Errorf("%s %s: %w", p.browser.key, p.dir, ErrNotSignedIn))
			continue
		}
		return cookies, Source{Browser: p.browser.key, Profile: p.dir}, nil
	}
	if allNotSignedIn(errs) {
		return nil, Source{}, ErrNotSignedIn
	}
	return nil, Source{}, errors.Join(errs...)
}

func allNotSignedIn(errs []error) bool {
	if len(errs) == 0 {
		return false
	}
	for _, err := range errs {
		if !errors.Is(err, ErrNotSignedIn) {
			return false
		}
	}
	return true
}

func readProfile(p profile, domains []string) ([]Cookie, error) {
	switch p.browser.kind {
	case chromium:
		return readChromium(p, domains)
	case firefox:
		return readFirefox(p.db, domains)
	}
	return nil, errors.New("unknown browser kind")
}

// hasCookie reports whether cookies hold a cookie named name for the first
// domain.
func hasCookie(cookies []Cookie, domains []string, name string) bool {
	if len(domains) == 0 {
		return false
	}
	for _, c := range cookies {
		if c.Name == name && c.Value != "" && domainMatch(c.Host, domains[0]) {
			return true
		}
	}
	return false
}

// domainMatch reports whether a cookie host belongs to domain.
func domainMatch(host, domain string) bool {
	host = strings.TrimPrefix(strings.ToLower(host), ".")
	domain = strings.TrimPrefix(strings.ToLower(domain), ".")
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// profiles lists the profiles of b that have a cookie database. A non-empty
// profileName selects one profile by directory name or by absolute path.
func profiles(home string, b browser, profileName string) []profile {
	if profileName != "" {
		dir := expandHome(home, profileName)
		if !filepath.IsAbs(dir) {
			var out []profile
			for _, root := range b.roots {
				out = append(out, profileAt(b, filepath.Join(rootPath(home, root), profileName))...)
			}
			return out
		}
		if out := profileAt(b, dir); len(out) > 0 {
			return out
		}
		// The path may be a profile root, such as a Brave Origin directory.
		return profilesUnder(b, dir)
	}
	var out []profile
	for _, root := range b.roots {
		out = append(out, profilesUnder(b, rootPath(home, root))...)
	}
	return out
}

func rootPath(home, root string) string {
	if filepath.IsAbs(root) {
		return root
	}
	return filepath.Join(home, root)
}

func expandHome(home, p string) string {
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") {
		return filepath.Join(home, p[2:])
	}
	return p
}

// profilesUnder lists the profiles inside one browser root directory.
func profilesUnder(b browser, root string) []profile {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []profile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		out = append(out, profileAt(b, filepath.Join(root, e.Name()))...)
	}
	return out
}

// profileAt returns the profile at dir when it has a cookie database.
func profileAt(b browser, dir string) []profile {
	var names []string
	switch b.kind {
	case chromium:
		names = []string{filepath.Join("Network", "Cookies"), "Cookies"}
	case firefox:
		names = []string{"cookies.sqlite"}
	}
	for _, name := range names {
		db := filepath.Join(dir, name)
		info, err := os.Stat(db)
		if err != nil || info.IsDir() {
			continue
		}
		return []profile{{browser: b, dir: dir, db: db, modTime: info.ModTime()}}
	}
	return nil
}

// WriteNetscape writes cookies to path in the Netscape cookies.txt format
// that yt-dlp --cookies reads. The file is replaced atomically and is only
// readable by the user.
func WriteNetscape(path string, cookies []Cookie) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Netscape HTTP Cookie File\n# Written by cliamp-x. Do not share: it holds your sign-in.\n\n")
	for _, c := range cookies {
		if strings.ContainsAny(c.Name+c.Value+c.Host+c.Path, "\t\n\r") {
			continue
		}
		path := c.Path
		if path == "" {
			path = "/"
		}
		host := c.Host
		if c.HTTPOnly {
			host = "#HttpOnly_" + host
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			host, upperBool(strings.HasPrefix(c.Host, ".")), path, upperBool(c.Secure), c.Expires, c.Name, c.Value)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".cookies-*.txt")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(b.String()); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func upperBool(v bool) string {
	if v {
		return "TRUE"
	}
	return "FALSE"
}

// ReadNetscape reads a Netscape cookies.txt file.
func ReadNetscape(path string) ([]Cookie, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Cookie
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		httpOnly := false
		if strings.HasPrefix(line, "#HttpOnly_") {
			line = strings.TrimPrefix(line, "#HttpOnly_")
			httpOnly = true
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 7 {
			continue
		}
		expires, _ := strconv.ParseInt(f[4], 10, 64)
		out = append(out, Cookie{
			Host: f[0], Path: f[2], Secure: strings.EqualFold(f[3], "TRUE"),
			Expires: expires, Name: f[5], Value: f[6], HTTPOnly: httpOnly,
		})
	}
	return out, sc.Err()
}

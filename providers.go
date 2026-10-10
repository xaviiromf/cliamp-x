package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/config"
	"github.com/xaviiromf/cliamp-x/external/local"
	"github.com/xaviiromf/cliamp-x/external/ytmusic"
	"github.com/xaviiromf/cliamp-x/favorites"
	"github.com/xaviiromf/cliamp-x/history"
	"github.com/xaviiromf/cliamp-x/internal/appdir"
	"github.com/xaviiromf/cliamp-x/internal/ytdlcookies"
	"github.com/xaviiromf/cliamp-x/internal/ytdlp"
	"github.com/xaviiromf/cliamp-x/locallib"
	"github.com/xaviiromf/cliamp-x/player"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/resolve"
)

// providerSet holds the providers of one run, in the order of the provider
// list. The player hooks, the sign-in observers and the shutdown find the
// providers in entries. A provider.CustomStreamer or a provider.Closer needs
// no code here.
type providerSet struct {
	entries []provider.Entry
	// local, favorites and history are nil when the config directory is
	// unavailable. The local provider and the Model share the two stores.
	local     *local.Provider
	favorites *favorites.Store
	history   *history.Store
}

// providerKey is one provider that --provider and the provider config key
// accept.
type providerKey struct {
	key   string // the value of --provider and of the provider config key
	alias string // a short form of key that --provider also accepts
	name  string // the display name in the provider list
	// optional marks a provider that registers only when it is configured.
	// The log names each optional provider that is not configured.
	optional bool
}

// providerKeys lists the providers that --provider accepts, in help order.
// The flag help, the flag check, the provider names and the log of skipped
// providers all come from this table.
var providerKeys = []providerKey{
	{key: "ytmusic", name: "YouTube Music"},
	{key: "local", name: "Local"},
}

// providerName returns the display name of the provider key.
func providerName(key string) string {
	for _, pk := range providerKeys {
		if pk.key == key {
			return pk.name
		}
	}
	return key
}

// providerFlagUsage returns the help text of --provider.
func providerFlagUsage() string {
	var values []string
	for _, pk := range providerKeys {
		values = append(values, pk.key)
		if pk.alias != "" {
			values = append(values, pk.alias)
		}
	}
	return "default provider: " + strings.Join(values, ", ")
}

// parseProviderKey returns the provider key that the --provider value v
// names. It accepts a key or an alias in any case.
func parseProviderKey(v string) (string, error) {
	v = strings.ToLower(v)
	keys := make([]string, len(providerKeys))
	for i, pk := range providerKeys {
		if v == pk.key || (pk.alias != "" && v == pk.alias) {
			return pk.key, nil
		}
		keys[i] = pk.key
	}
	if len(keys) == 1 {
		return "", fmt.Errorf("--provider must be %s (got %q)", keys[0], v)
	}
	last := len(keys) - 1
	return "", fmt.Errorf("--provider must be %s or %s (got %q)", strings.Join(keys[:last], ", "), keys[last], v)
}

// buildProviders creates the providers that cfg enables. The public
// providers are always available. The account providers register when they
// are configured. interactive allows the yt-dlp install prompt.
func buildProviders(cfg config.Config, interactive bool) *providerSet {
	s := &providerSet{}
	s.favorites, s.history = favorites.New(), history.New()
	s.local = local.New(s.favorites, s.history)
	// The virtual Favorites playlist reserves the name of a Favorites.toml
	// playlist. Bookmarks became favorites. Copy the old bookmarks one time.
	if err := s.local.MigrateFavoritesFile(); err != nil {
		applog.Warn("favorites playlist migration: %v", err)
	}
	if added, err := s.local.MigrateBookmarks(); err != nil {
		applog.Warn("bookmark migration: %v", err)
	} else if added > 0 {
		applog.Info("copied %d bookmarks into favorites", added)
	}

	// YouTube Music comes first: it is the view cliamp-x opens on.
	s.entries = append(s.entries, youTubeEntries(cfg, interactive, s.favorites, s.history, localLibrary(cfg.MusicDir))...)
	if s.local != nil {
		s.entries = append(s.entries, provider.Entry{Key: "local", Name: "Local", Provider: s.local})
	} else {
		logProviderSkipped("Local", "local", "config directory unavailable")
	}

	logProviderWiring(s.entries)
	return s
}

// youTubeEntries returns the YouTube Music entry. It returns none when the
// [ytmusic] section sets enabled = false. The sign-in comes from the browser
// cookies that `cliamp-x login` remembered; without one the provider still
// browses and searches. A missing yt-dlp only blocks playback.
func youTubeEntries(cfg config.Config, interactive bool, favs *favorites.Store, hist *history.Store, local *locallib.Library) []provider.Entry {
	if !cfg.YouTubeMusic.IsSet() {
		logYouTubeSkipped("disabled in config")
		return nil
	}
	if !player.YTDLPAvailable() {
		fmt.Fprintf(os.Stderr, "\nYouTube Music requires yt-dlp for audio playback.\n")
		fmt.Fprintf(os.Stderr, "Install command: %s\n\n", ytdlp.InstallHint())
		if offerYTDLPInstall(interactive, os.Stdin, os.Stderr) {
			fmt.Fprintf(os.Stderr, "Installing yt-dlp...\n")
			if err := installYTDLP(); err != nil {
				fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
				fmt.Fprintf(os.Stderr, "Install yt-dlp manually and restart to play songs.\n\n")
			} else {
				fmt.Fprintf(os.Stderr, "yt-dlp installed successfully!\n\n")
			}
		}
	}

	auth := ytmusic.LoadAuth(cfg.YouTubeMusic.CookiesFrom)
	if auth.SignedIn() {
		// yt-dlp plays with the same session, so Premium quality and
		// age-restricted songs work.
		if path, err := ytmusic.CookiePath(); err == nil {
			for _, host := range []string{"youtube.com", "youtu.be", "music.youtube.com"} {
				resolve.SetYTDLCookiesForHost(host, ytdlcookies.FilePrefix+path)
			}
		}
	}
	client := ytmusic.NewClient(auth, cfg.YouTubeMusic.Language, cfg.YouTubeMusic.Region)
	music := ytmusic.New(client, ytmusic.Options{
		Favorites:     favs,
		History:       hist,
		Local:         local,
		ReportHistory: cfg.YouTubeMusic.ReportHistory,
		DataDir:       ytmusicDataDir(),
	})
	return []provider.Entry{
		{Key: "ytmusic", Name: providerName("ytmusic"), Provider: music},
	}
}

// localLibrary returns the index of the local music folder that joins the
// YouTube Music menus: music_dir, or the user's music folder when unset.
// "off" turns it off.
func localLibrary(dir string) *locallib.Library {
	if strings.TrimSpace(dir) == "" {
		dir = locallib.DefaultDir()
	}
	return locallib.New(dir)
}

// Close releases the providers that implement provider.Closer. The three
// YouTube providers share one base, and its close is safe to repeat.
func (s *providerSet) Close() {
	for _, e := range s.entries {
		if c, ok := e.Provider.(provider.Closer); ok {
			c.Close()
		}
	}
}

// localPlaylists returns the local provider for model.New. With no config
// directory it returns a nil interface. A nil *local.Provider in the
// interface would look set to the Model, which then calls it and panics.
func (s *providerSet) localPlaylists() playlist.Provider {
	if s.local == nil {
		return nil
	}
	return s.local
}

// registerPlayerHooks registers with p the stream factories and source
// resolvers of the providers, and the rules that pick the pipeline of a URL.
func (s *providerSet) registerPlayerHooks(p *player.Player) {
	for _, e := range s.entries {
		if cs, ok := e.Provider.(provider.CustomStreamer); ok {
			for _, scheme := range cs.URISchemes() {
				p.RegisterStreamerFactory(scheme, cs.NewStreamer)
			}
		}
	}
	p.RegisterYTDLMatcher(playlist.IsYTDL)
}

// observeAuthURLs sends the sign-in URL of a provider to the Model, so the
// provider pane can show it when no browser opens. YouTube Music signs in
// with browser cookies and opens no sign-in page, so nothing is observed.
func (s *providerSet) observeAuthURLs(func(tea.Msg)) (restore func()) {
	return func() {}
}

// logProviderRegistered records that a provider joined the active set. It
// writes to the log file only, so it never disturbs the TUI. See issue #406.
func logProviderRegistered(name, key string) {
	applog.Info("provider registered: name=%s key=%s", name, key)
}

// logProviderSkipped records why a provider did not register. It writes to
// the log file only, so it never disturbs the TUI. See issue #406.
func logProviderSkipped(name, key, reason string) {
	applog.Info("provider skipped: name=%s key=%s reason=%s", name, key, reason)
}

// logYouTubeSkipped records the skip reason for all three YouTube providers
// (All, video, music), since they register or skip as one group.
func logYouTubeSkipped(reason string) {
	logProviderSkipped(providerName("ytmusic"), "ytmusic", reason)
}

// offerYTDLPInstall asks on in whether to install yt-dlp now. It asks only
// when interactive is true. A bare Enter, y or yes in any case confirms. Any
// other answer, EOF or a read error skips the install. So a start with stdin
// at /dev/null, as under systemd, never installs a package.
func offerYTDLPInstall(interactive bool, in io.Reader, out io.Writer) bool {
	if !interactive {
		return false
	}
	fmt.Fprint(out, "Press Enter to install it now, or type n and press Enter to skip... ")
	answer, err := bufio.NewReader(in).ReadString('\n')
	if err == nil {
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "", "y", "yes":
			return true
		}
	} else {
		// EOF leaves the cursor on the prompt line.
		fmt.Fprintln(out)
	}
	fmt.Fprint(out, "Skipped. YouTube providers are disabled.\n\n")
	return false
}

// installYTDLP attempts to install yt-dlp using the system package manager.
// Returns nil on success. The caller should re-check player.YTDLPAvailable()
// after.
func installYTDLP() error {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err == nil {
			cmd := exec.Command("brew", "install", "yt-dlp")
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		// Fall through to pip
	case "linux":
		if _, err := exec.LookPath("apt-get"); err == nil {
			cmd := exec.Command("sudo", "apt-get", "install", "-y", "yt-dlp")
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
		if _, err := exec.LookPath("pacman"); err == nil {
			cmd := exec.Command("sudo", "pacman", "-S", "--noconfirm", "yt-dlp")
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			return cmd.Run()
		}
	}
	// Fallback: pip/pipx
	if path, err := exec.LookPath("pipx"); err == nil {
		cmd := exec.Command(path, "install", "yt-dlp")
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	if path, err := exec.LookPath("pip3"); err == nil {
		cmd := exec.Command(path, "install", "yt-dlp")
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return fmt.Errorf("no supported package manager found — install manually: https://github.com/yt-dlp/yt-dlp#installation")
}

// isCharDevice reports whether f is a character device, such as a terminal.
// A pipe or a regular file is not. The null device is also a character
// device, so a start with stdin at /dev/null shows the install prompt. The
// EOF that follows skips the install.
func isCharDevice(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// logProviderWiring logs the final provider registry: one line per
// registered provider, plus a skip line for each optional provider absent
// from it. YouTube and Local are not optional: they log their own specific
// skip reasons. See issue #406.
func logProviderWiring(providers []provider.Entry) {
	registered := make(map[string]bool, len(providers))
	for _, p := range providers {
		logProviderRegistered(p.Name, p.Key)
		registered[p.Key] = true
	}
	for _, pk := range providerKeys {
		if pk.optional && !registered[pk.key] {
			logProviderSkipped(pk.name, pk.key, "not configured")
		}
	}
}

// ytmusicDataDir is where the YouTube Music provider keeps its caches.
func ytmusicDataDir() string {
	dir, err := appdir.DataDir()
	if err != nil {
		return ""
	}
	return dir
}

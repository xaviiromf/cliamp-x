package main

import (
	"bufio"
	"cmp"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/config"
	"github.com/xaviiromf/cliamp-x/external/local"
	"github.com/xaviiromf/cliamp-x/favorites"
	"github.com/xaviiromf/cliamp-x/history"
	"github.com/xaviiromf/cliamp-x/locallib"
	"github.com/xaviiromf/cliamp-x/player"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
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
	// taste holds the "interested" and "not interested" ratings that the
	// providers and the Model share.
	taste *taste.Store
	// library is the indexed local music folder, nil when it is off.
	library *locallib.Library
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

// providerEnv is what a provider factory gets: the config and the stores
// that every provider may share. A provider reads its own settings from
// cfg.ProviderSection(key), the [providers.<key>] section of config.toml.
type providerEnv struct {
	cfg config.Config
	// interactive reports whether a terminal can answer a prompt, such as
	// the yt-dlp install offer. It is false in headless mode.
	interactive bool
	favorites   *favorites.Store
	history     *history.Store
	taste       *taste.Store
	library     *locallib.Library // the indexed local music folder
	local       *local.Provider   // nil without a config directory
}

// providerFactory registers one provider. See docs/adding-providers.md.
type providerFactory struct {
	providerKey
	// order places the provider in the provider list, lowest first. The
	// upstream providers use 10 (YouTube Music) and 100 (Local); a fork
	// picks its own number and needs no change to theirs.
	order int
	// shortcut is the key that opens the provider from the player, or "".
	shortcut string
	// build returns the provider's entries, or none when it is not
	// configured. It runs once per start, before the TUI opens.
	build func(env providerEnv) []provider.Entry
}

// providerFactories are the registered providers, in list order.
var providerFactories []providerFactory

// providerKeys lists the providers that --provider accepts, in help order.
// The flag help, the flag check, the provider names and the log of skipped
// providers all come from this table. registerProvider fills it.
var providerKeys []providerKey

// registerProvider adds a provider to every run. Each provider calls it from
// an init function in its own provider_<key>.go file, so adding a provider
// touches no shared file, and a fork keeps its providers through upstream
// updates. A second registration of a key replaces the first.
func registerProvider(f providerFactory) {
	providerFactories = slices.DeleteFunc(providerFactories, func(o providerFactory) bool { return o.key == f.key })
	providerFactories = append(providerFactories, f)
	slices.SortStableFunc(providerFactories, func(a, b providerFactory) int { return cmp.Compare(a.order, b.order) })
	providerKeys = providerKeys[:0]
	for _, pf := range providerFactories {
		providerKeys = append(providerKeys, pf.providerKey)
	}
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

// buildProviders creates the providers that cfg enables, in the order of
// their registration. interactive allows prompts such as the yt-dlp install.
// A provider whose factory panics is left out and logged, so one broken
// provider cannot stop the others.
func buildProviders(cfg config.Config, interactive bool) *providerSet {
	s := &providerSet{}
	s.favorites, s.history, s.taste = favorites.New(), history.New(), taste.New()
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

	s.library = localLibrary(cfg.MusicDir)
	env := providerEnv{
		cfg:         cfg,
		interactive: interactive,
		favorites:   s.favorites,
		history:     s.history,
		taste:       s.taste,
		library:     s.library,
		local:       s.local,
	}
	for _, pf := range providerFactories {
		for _, e := range buildProvider(pf, env) {
			if e.Shortcut == "" {
				e.Shortcut = pf.shortcut
			}
			s.entries = append(s.entries, e)
		}
	}

	logProviderWiring(s.entries)
	return s
}

// buildProvider runs one factory and recovers from its panic.
func buildProvider(pf providerFactory, env providerEnv) (entries []provider.Entry) {
	defer func() {
		if r := recover(); r != nil {
			logProviderSkipped(pf.name, pf.key, fmt.Sprintf("failed to start: %v", r))
			entries = nil
		}
	}()
	for _, e := range pf.build(env) {
		if e.Provider == nil {
			continue
		}
		if e.Key == "" {
			e.Key = pf.key
		}
		if e.Name == "" {
			e.Name = pf.name
		}
		entries = append(entries, e)
	}
	return entries
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

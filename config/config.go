// Package config handles loading user configuration from ~/.config/cliamp/config.toml.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/appdir"
	"github.com/xaviiromf/cliamp-x/internal/fileutil"
)

// maxVisRows caps the configurable visualizer height. The layout shrinks the
// value further when the terminal cannot spare the rows.
const maxVisRows = 40

// Path returns the path of config.toml in the cliamp config directory.
func Path() (string, error) {
	dir, err := appdir.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// parseString unquotes a TOML string value and, if the result is exactly
// $NAME or ${NAME}, replaces it with the value of that environment variable
// (or "" when unset). Mixed values containing other characters are left
// untouched, so literal '$' in passwords is preserved.
func parseString(s string) string {
	s = unquote(s)
	if name, ok := EnvRef(s); ok {
		return os.Getenv(name)
	}
	return s
}

// EnvRef reports whether the unquoted value s is exactly $NAME or ${NAME}.
// Load reads such a value from the environment variable name, so the text
// itself cannot be stored in config.toml.
func EnvRef(s string) (name string, ok bool) {
	if len(s) < 2 || s[0] != '$' {
		return "", false
	}
	name = s[1:]
	if name[0] == '{' {
		if name[len(name)-1] != '}' {
			return "", false
		}
		name = name[1 : len(name)-1]
	}
	if !isEnvName(name) {
		return "", false
	}
	return name, true
}

// unquote removes one pair of matching quotes from s. Inside double quotes it
// decodes \\ and \", the escapes QuoteString writes, and keeps every other
// backslash as typed, so "D:\new" stays a Windows path. Single quotes are
// literal. A # comment after the closing quote is dropped, with or without
// whitespace before it. A value that does not start with a quote is returned
// unchanged, # included.
func unquote(s string) string {
	if len(s) < 2 || (s[0] != '"' && s[0] != '\'') {
		return s
	}
	q := s[0]
	var b strings.Builder
	closed := false
	for i := 1; i < len(s); i++ {
		c := s[i]
		if q == '"' && c == '\\' && i+1 < len(s) && (s[i+1] == '\\' || s[i+1] == '"') {
			i++
			b.WriteByte(s[i])
			continue
		}
		if c == q {
			if afterClose(s[i+1:]) {
				return b.String()
			}
			closed = true
			break
		}
		b.WriteByte(c)
	}
	// The closing quote is missing, as in "Tokyo Night or 'abc". Strip the
	// opening quote and any stray quote at the end, and keep the rest as
	// typed.
	if !closed {
		return strings.TrimRight(s[1:], `"'`)
	}
	// Text follows the closing quote. Strip the outer pair and keep the rest
	// as typed.
	if s[len(s)-1] == q {
		return s[1 : len(s)-1]
	}
	return s
}

// isComment reports whether rest, the text after a value, is empty or a
// # comment that whitespace separates from the value.
func isComment(rest string) bool {
	trimmed := strings.TrimLeft(rest, " \t")
	if trimmed == "" {
		return true
	}
	return len(trimmed) < len(rest) && trimmed[0] == '#'
}

// afterClose reports whether rest, the text after a closing quote or
// bracket, is empty or a # comment. The value cannot hold that #, so the
// comment needs no whitespace before it.
func afterClose(rest string) bool {
	rest = strings.TrimLeft(rest, " \t")
	return rest == "" || rest[0] == '#'
}

var quoteEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

// QuoteString returns s as a double-quoted TOML string that Load reads back
// unchanged. It escapes only \ and ". s must be a single line. The one
// exception is a $NAME or ${NAME} value: Load reads it from the environment.
// Use EnvRef to find such a value.
func QuoteString(s string) string {
	return `"` + quoteEscaper.Replace(s) + `"`
}

func isEnvName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// parseBool reads a bool value. It accepts true and false in any letter case,
// and the other strconv.ParseBool forms such as 1 and 0. ok is false for any
// other value, so the caller keeps the current setting.
func parseBool(val string) (v, ok bool) {
	v, err := strconv.ParseBool(strings.ToLower(scalar(val)))
	return v, err == nil
}

// parseInt reads an integer value. ok is false for any other value.
func parseInt(val string) (int, bool) {
	v, err := strconv.Atoi(scalar(val))
	return v, err == nil
}

// parseFloat reads a finite number value. ok is false for any other value,
// also for NaN and an infinity, which clamp cannot constrain.
func parseFloat(val string) (float64, bool) {
	v, err := strconv.ParseFloat(scalar(val), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// scalar returns the number or bool token at the start of val without its
// trailing # comment. Only the number and bool parsers call it, so a # in an
// unquoted string value is never cut.
func scalar(val string) string {
	if i := strings.IndexAny(val, " \t"); i >= 0 && isComment(val[i:]) {
		return val[:i]
	}
	return val
}

// YouTubeMusicConfig holds settings for the YouTube Music provider. It is on
// unless the section sets enabled = false. The [yt] and [youtube] headers
// count as [ytmusic].
type YouTubeMusicConfig struct {
	Disabled       bool   // true only when user explicitly sets enabled = false
	CookiesFrom    string // browser for the sign-in in yt-dlp syntax, or "" to detect one
	ExpandPlaylist *bool  // nil = default (true), controls whether list= URLs expand the full playlist
	Language       string // hl of the YouTube Music pages, or "" for $LANG
	Region         string // gl of the YouTube Music pages, or "" for $LANG
	AutoRadio      *bool  // nil = default (true): continue with related songs when the queue ends
	ReportHistory  bool   // add played songs to the YouTube Music history
}

// IsSet reports whether the YouTube Music provider is enabled.
func (y YouTubeMusicConfig) IsSet() bool {
	return !y.Disabled
}

// AutoRadioEnabled reports whether the queue continues with related songs.
func (y YouTubeMusicConfig) AutoRadioEnabled() bool {
	return y.AutoRadio == nil || *y.AutoRadio
}

// set applies one key of the [ytmusic] section.
func (y *YouTubeMusicConfig) set(key, val string) {
	switch key {
	case "enabled":
		if v, ok := parseBool(val); ok {
			y.Disabled = !v
		}
	case "cookies_from":
		y.CookiesFrom = strings.TrimSpace(parseString(val))
	case "expand_playlist":
		if v, ok := parseBool(val); ok {
			y.ExpandPlaylist = &v
		}
	case "language":
		y.Language = strings.TrimSpace(parseString(val))
	case "region":
		y.Region = strings.ToUpper(strings.TrimSpace(parseString(val)))
	case "auto_radio":
		if v, ok := parseBool(val); ok {
			y.AutoRadio = &v
		}
	case "report_history":
		if v, ok := parseBool(val); ok {
			y.ReportHistory = v
		}
	}
}

// DownloadsConfig selects the directory for saved audio. Empty uses ~/Music/cliamp-x.
type DownloadsConfig struct {
	Directory string
}

// set applies one key of the [downloads] section.
func (d *DownloadsConfig) set(key, val string) {
	if key == "directory" {
		d.Directory = parseString(val)
	}
}

// Config holds user preferences loaded from the config file.
type Config struct {
	Downloads        DownloadsConfig
	Volume           float64     // dB, clamped at runtime to [VolumeMin, +6]
	VolumeMin        float64     // dB floor, range [-90, 0]; default -50
	VisVolumeLinked  bool        // when true, visualizer bar height follows volume; default true
	EQ               [10]float64 // per-band gain in dB, range [-12, +12]
	EQPreset         string      // preset name, or "" for custom
	EQAuto           bool        // pick the EQ preset from each song's genre
	Repeat           string      // "off", "all", or "one"
	Shuffle          bool
	Mono             bool
	Speed            float64                      // playback speed ratio: 0.25–2.0 (default 1.0)
	AutoPlay         bool                         // start playback automatically on launch (radio streams, CLI tracks)
	SeekStepLarge    int                          // seconds for Shift+Left/Right seek jumps
	Provider         string                       // default provider key, a value that --provider accepts (default "ytmusic")
	Theme            string                       // theme name, or "" for ANSI default
	Visualizer       string                       // visualizer mode name, or "" for default (Bars)
	VisRows          int                          // visualizer height in rows at the full layout tier, or 0 for the built-in default
	SampleRate       int                          // output sample rate: 22050, 44100, 48000, 96000, 192000
	BufferMs         int                          // speaker buffer in milliseconds (50-5000)
	ResampleQuality  int                          // beep resample quality factor (1–4)
	BitDepth         int                          // PCM bit depth for FFmpeg output: 16 or 32
	Simplified       bool                         // simplified playback view: track summary and time strip
	HideHelpBar      bool                         // hide the key-binding hint bar above the status line
	HideSettingsPane bool                         // close the settings pane beside the playlist
	HidePreview      bool                         // hide the artwork preview pane under the lists
	HideFullCover    bool                         // hide the cover beside the full-screen visualizer
	MouseOff         bool                         // true when mouse = false
	Language         string                       // interface language: "es", "en", or "" to follow $LANG
	NotifyOff        bool                         // true when notifications = false
	VisSensitivity   int                          // visualizer sensitivity 1-10, or 0 for the default (7)
	PreviewImages    string                       // preview pictures: "auto", "kitty" or "blocks" (default "auto")
	ShowMetadata     bool                         // expand highlighted-track metadata below settings (default false)
	Expanded         bool                         // start with the playlist expanded (the Ctrl+X state)
	PaddingH         int                          // horizontal padding for the UI frame (default 3)
	PaddingV         int                          // vertical padding for the UI frame (default 1)
	AudioDevice      string                       // preferred audio output device name (empty = system default)
	Playlist         string                       // local TOML playlist name to load on startup
	InitialDirectory string                       // initial directory for the file browser
	MusicDir         string                       // local music folder for the YouTube Music menus: "" = ~/Music, "off" = none
	LyricsOffsetMs   int                          // lyric timestamp adjustment in ms (-10000..10000), applied to all sources
	YouTubeMusic     YouTubeMusicConfig           // optional YouTube Music provider
	SponsorBlock     SponsorBlockConfig           // skipping of marked YouTube segments
	Normalize        string                       // loudness normalization: "track", "album" or "off" (default "track")
	NormalizeTarget  float64                      // normalization target in LUFS, or 0 for the default (-14)
	Crossfade        int                          // seconds a track overlaps the next, 0-12 (default 0)
	Plugins          map[string]map[string]string // per-plugin config from [plugins.*] sections
	LogLevel         string                       // log level: debug, info, warn, error (default "info")
	LowPower         bool                         // reduce CPU by lowering UI cadence and disabling visualization
}

// defaultConfig returns a Config with sensible defaults.
// SampleRate defaults to 0, which means "auto-detect from the system's default
// output device" (see player.DeviceSampleRate). This ensures USB audio devices
// that require a specific rate (commonly 48 kHz) work out of the box.
func defaultConfig() Config {
	return Config{
		VolumeMin:       -50,
		VisVolumeLinked: true,
		Repeat:          "off",
		AutoPlay:        false,
		Speed:           1.0,
		SeekStepLarge:   30,
		SampleRate:      0,
		BufferMs:        250,
		ResampleQuality: 4,
		BitDepth:        16,
		PaddingH:        3,
		PaddingV:        1,
		LogLevel:        "info",
	}
}

// Load reads the config file from ~/.config/cliamp/config.toml.
// Returns defaults if the file does not exist.
func Load() (Config, error) {
	cfg := defaultConfig()

	path, err := Path()
	if err != nil {
		return cfg, nil
	}

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	section := "" // current [section] header, empty = top-level
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Section header: [navidrome], [plex], [plugins.lastfm], etc.
		if name, ok := sectionHeader(line); ok {
			section = canonicalSection(name)
			// Mark providers as enabled when their section exists.
			// Initialize plugin sub-maps for [plugins] and [plugins.*] sections.
			if name, ok := pluginSection(section); ok {
				if cfg.Plugins == nil {
					cfg.Plugins = make(map[string]map[string]string)
				}
				if _, ok := cfg.Plugins[name]; !ok {
					cfg.Plugins[name] = make(map[string]string)
				}
			}
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		switch section {
		case "":
			cfg.setTopLevel(key, val)
		case "downloads":
			cfg.Downloads.set(key, val)
		case "ytmusic":
			cfg.YouTubeMusic.set(key, val)
		case "sponsorblock":
			cfg.SponsorBlock.set(key, val)
		default:
			// Keys of a plugin section go to the plugin. Any other section,
			// such as one left over from cliamp, is ignored.
			if name, ok := pluginSection(section); ok {
				if m, ok := cfg.Plugins[name]; ok {
					m[key] = pluginValue(name, key, val)
				}
			}
		}
	}

	cfg.clamp()
	return cfg, scanner.Err()
}

// sectionHeader returns the name inside a [name] header line. A # comment
// may follow the closing bracket.
func sectionHeader(line string) (string, bool) {
	if !strings.HasPrefix(line, "[") {
		return "", false
	}
	if end := strings.IndexByte(line, ']'); end > 0 && afterClose(line[end+1:]) {
		return line[1:end], true
	}
	if strings.HasSuffix(line, "]") {
		return line[1 : len(line)-1], true
	}
	return "", false
}

// canonicalSection returns the name that Load uses for a section header. It
// ignores letter case, and [yt], [youtube] and [ytmusic] all configure the
// same YouTube providers.
func canonicalSection(name string) string {
	name = strings.ToLower(name)
	switch name {
	case "yt", "youtube":
		return "ytmusic"
	}
	return name
}

// pluginSection returns the plugin name of a [plugins] or [plugins.<name>]
// section. The top-level [plugins] section has the empty name.
func pluginSection(section string) (string, bool) {
	if section == "plugins" {
		return "", true
	}
	return strings.CutPrefix(section, "plugins.")
}

// pluginValue reads a value of the [plugins] section, or of the
// [plugins.<name>] section when plugin is not empty. Plugins get each value
// as a string. cliamp reads three keys itself, so these follow the value
// rules: enabled of a plugin is a bool, and disabled and allowed_binaries of
// [plugins] are lists. Load stores the bool as true or false, and a list in
// square brackets as names that commas separate.
func pluginValue(plugin, key, val string) string {
	switch {
	case plugin != "" && key == "enabled":
		if v, ok := parseBool(val); ok {
			return strconv.FormatBool(v)
		}
	case plugin == "" && (key == "disabled" || key == "allowed_binaries"):
		if strings.HasPrefix(val, "[") {
			return strings.Join(parseStringSlice(val), ",")
		}
	}
	return parseString(val)
}

// setTopLevel applies one top-level key. Keys under an unknown section also
// land here.
func (c *Config) setTopLevel(key, val string) {
	switch key {
	case "volume":
		if v, ok := parseFloat(val); ok {
			c.Volume = v
		}
	case "volume_min":
		if v, ok := parseFloat(val); ok {
			c.VolumeMin = v
		}
	case "vis_volume_linked":
		if v, ok := parseBool(val); ok {
			c.VisVolumeLinked = v
		}
	case "repeat":
		val = parseString(val)
		switch strings.ToLower(val) {
		case "all", "one", "off":
			c.Repeat = strings.ToLower(val)
		}
	case "shuffle":
		if v, ok := parseBool(val); ok {
			c.Shuffle = v
		}
	case "mono":
		if v, ok := parseBool(val); ok {
			c.Mono = v
		}
	case "auto_play":
		if v, ok := parseBool(val); ok {
			c.AutoPlay = v
		}
	case "seek_large_step_sec":
		if v, ok := parseInt(val); ok {
			c.SeekStepLarge = v
		}
	case "lyrics_offset_ms":
		if v, ok := parseInt(val); ok {
			c.LyricsOffsetMs = v
		}
	case "eq":
		c.EQ = parseEQ(val)
	case "eq_preset":
		c.EQPreset = parseString(val)
	case "theme":
		c.Theme = parseString(val)
	case "provider":
		c.Provider = strings.ToLower(parseString(val))
	case "visualizer":
		c.Visualizer = parseString(val)
	case "vis_rows":
		if v, ok := parseInt(val); ok {
			c.VisRows = v
		}
	case "sample_rate":
		if v, ok := parseInt(val); ok {
			c.SampleRate = v
		}
	case "buffer_ms":
		if v, ok := parseInt(val); ok {
			c.BufferMs = v
		}
	case "resample_quality":
		if v, ok := parseInt(val); ok {
			c.ResampleQuality = v
		}
	case "bit_depth":
		if v, ok := parseInt(val); ok {
			c.BitDepth = v
		}
	case "speed":
		if v, ok := parseFloat(val); ok {
			c.Speed = v
		}
	case "simplified":
		if v, ok := parseBool(val); ok {
			c.Simplified = v
		}
	case "hide_help_bar":
		if v, ok := parseBool(val); ok {
			c.HideHelpBar = v
		}
	case "hide_settings_pane":
		if v, ok := parseBool(val); ok {
			c.HideSettingsPane = v
		}
	case "hide_preview":
		if v, ok := parseBool(val); ok {
			c.HidePreview = v
		}
	case "notifications":
		if v, ok := parseBool(val); ok {
			c.NotifyOff = !v
		}
	case "language":
		switch v := strings.ToLower(parseString(val)); v {
		case "es", "en", "":
			c.Language = v
		}
	case "mouse":
		if v, ok := parseBool(val); ok {
			c.MouseOff = !v
		}
	case "hide_fullscreen_cover":
		if v, ok := parseBool(val); ok {
			c.HideFullCover = v
		}
	case "vis_sensitivity":
		if v, ok := parseInt(val); ok {
			c.VisSensitivity = max(1, min(10, v))
		}
	case "normalize":
		switch mode := strings.ToLower(parseString(val)); mode {
		case "track", "album", "off":
			c.Normalize = mode
		case "true", "on":
			c.Normalize = "track"
		case "false":
			c.Normalize = "off"
		}
	case "normalize_target":
		if v, ok := parseFloat(val); ok && v >= -30 && v <= -5 {
			c.NormalizeTarget = v
		}
	case "crossfade":
		if v, ok := parseInt(val); ok {
			c.Crossfade = max(0, min(12, v))
		}
	case "preview_images":
		mode := strings.ToLower(parseString(val))
		switch mode {
		case "auto", "kitty", "blocks":
			c.PreviewImages = mode
		}
	case "show_metadata":
		if v, ok := parseBool(val); ok {
			c.ShowMetadata = v
		}
	case "expanded":
		if v, ok := parseBool(val); ok {
			c.Expanded = v
		}
	case "audio_device":
		c.AudioDevice = parseString(val)
	case "initial_directory":
		c.InitialDirectory = parseString(val)
	case "eq_auto":
		if v, ok := parseBool(val); ok {
			c.EQAuto = v
		}
	case "music_dir":
		c.MusicDir = strings.TrimSpace(parseString(val))
	case "padding_horizontal":
		if v, ok := parseInt(val); ok {
			c.PaddingH = v
		}
	case "padding_vertical":
		if v, ok := parseInt(val); ok {
			c.PaddingV = v
		}
	case "log_level":
		lvl := strings.ToLower(parseString(val))
		switch lvl {
		case "debug", "info", "warn", "warning", "error":
			c.LogLevel = lvl
		}
	case "low_power":
		if v, ok := parseBool(val); ok {
			c.LowPower = v
		}
	}
}

// save updates only the given key in the existing config file, preserving
// all other content, comments, and formatting. If the key doesn't exist,
// it is appended. If no config file exists, one is created with just that key.
// The file always ends with a newline. value is the TOML text of the value.
// The typed savers SaveString, SaveBool, SaveFloat and SaveFloats format it.
func save(key, value string) error {
	if strings.ContainsAny(key+value, "\r\n") {
		return fmt.Errorf("save %s: a key or value holds a line break", key)
	}
	line := fmt.Sprintf("%s = %s", key, value)
	return update(func(data string) string { return editTopLevel(data, key, line) })
}

// saveMu serializes the config saves of this process. The lock file in
// update serializes them across cliamp processes.
var saveMu sync.Mutex

// update reads config.toml, passes its text to edit and writes the result.
// A missing file reads as empty text. A lock file next to config.toml covers
// the read, the edit and the write, so a save from another cliamp process,
// such as cliamp setup next to the TUI, cannot get lost in between.
func update(edit func(data string) string) error {
	path, err := Path()
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	saveMu.Lock()
	defer saveMu.Unlock()
	unlock, err := fileutil.LockFile(path + ".lock")
	if err != nil {
		return err
	}
	defer func() { _ = unlock() }()

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read config: %w", err)
	}
	if err := fileutil.WriteFileAtomic(path, []byte(edit(string(data))), 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// editTopLevel returns data with line written as the top-level key. save
// describes the rules.
func editTopLevel(data, key, line string) string {
	// Scan existing lines and replace the matching key in-place,
	// but only in the top-level scope (before any [section] header).
	// Load uses the last line of a duplicate key, so replace every line.
	lines := strings.Split(data, "\n")
	found := false
	lastKey, header := -1, -1 // indexes of the last top-level key and the first header
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		// Stop searching once we hit a section header — the key
		// belongs in the top-level scope only.
		if _, ok := sectionHeader(trimmed); ok {
			header = i
			break
		}
		k, _, ok := strings.Cut(trimmed, "=")
		if !ok {
			continue
		}
		lastKey = i
		if strings.TrimSpace(k) == key {
			lines[i] = line
			found = true
		}
	}
	switch {
	case found:
	case header >= 0 && lastKey >= 0:
		// Keep top-level keys together.
		lines = slices.Insert(lines, lastKey+1, line)
	case header >= 0:
		// The comment and blank lines above the first header describe the
		// section, so the key goes above them.
		at := header
		for at > 0 {
			if t := strings.TrimSpace(lines[at-1]); t != "" && !strings.HasPrefix(t, "#") {
				break
			}
			at--
		}
		lines = slices.Insert(lines, at, line)
	default:
		// A final newline leaves an empty last element. The key goes
		// before it, so it does not land after a blank line.
		end := len(lines)
		if lines[end-1] == "" {
			end--
		}
		lines = slices.Insert(lines, end, line)
	}
	if lines[len(lines)-1] != "" {
		lines = append(lines, "") // end the file with one newline
	}
	return strings.Join(lines, "\n")
}

// KeyValue is one key line for SaveSection. Value is the TOML text of the
// value, such as QuoteString(s), a number, a bool or a list.
type KeyValue struct {
	Key, Value string
}

// SaveSection writes kv into the [section] block of the config file and
// keeps every other line. It replaces a key in place and adds a missing key
// after the last key of the section. It removes each key in owned that kv
// does not hold, and it keeps all other keys and all comments. Headers match
// the way Load reads them, so section "ytmusic" also edits [youtube] or [yt].
// A missing section is added at the end, and a missing file is created.
func SaveSection(section string, kv []KeyValue, owned []string) error {
	for _, e := range kv {
		if strings.ContainsAny(e.Key+e.Value, "\r\n") {
			return fmt.Errorf("save [%s] %s: a key or value holds a line break", section, e.Key)
		}
	}
	return update(func(data string) string { return editSection(data, section, kv, owned) })
}

// editSection returns data with kv written into [section]. SaveSection
// describes the rules.
func editSection(data, section string, kv []KeyValue, owned []string) string {
	values := make(map[string]string, len(kv))
	for _, e := range kv {
		values[e.Key] = e.Value
	}
	written := make(map[string]bool, len(kv))
	target := canonicalSection(section)

	var lines []string
	if data != "" {
		lines = strings.Split(data, "\n")
	}
	out := make([]string, 0, len(lines)+len(kv)+3)
	inSection := false
	block := 0     // matching blocks seen so far
	insertAt := -1 // missing keys go before out[insertAt]
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if name, ok := sectionHeader(trimmed); ok {
			inSection = canonicalSection(name) == target
			out = append(out, l)
			if inSection {
				block++
				if block == 1 {
					insertAt = len(out)
				}
			}
			continue
		}
		if inSection && !strings.HasPrefix(trimmed, "#") {
			if before, _, ok := strings.Cut(l, "="); ok {
				key := strings.TrimSpace(before)
				if v, ok := values[key]; ok {
					if !strings.HasSuffix(before, " ") && !strings.HasSuffix(before, "\t") {
						before += " "
					}
					l = before + "= " + v
					written[key] = true
				} else if slices.Contains(owned, key) {
					continue
				}
				out = append(out, l)
				if block == 1 {
					insertAt = len(out)
				}
				continue
			}
		}
		out = append(out, l)
	}

	var missing []string
	for _, e := range kv {
		if !written[e.Key] {
			written[e.Key] = true
			missing = append(missing, e.Key+" = "+values[e.Key])
		}
	}
	switch {
	case len(missing) == 0:
	case insertAt >= 0:
		out = slices.Insert(out, insertAt, missing...)
	default:
		for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
			out = out[:len(out)-1]
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, "["+section+"]")
		out = append(out, missing...)
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// PlayerConfig is the subset of player controls needed to apply config.
type PlayerConfig interface {
	SetVolumeMin(db float64)
	SetVolume(db float64)
	SetSpeed(ratio float64)
	SetEQBand(band int, dB float64)
	ToggleMono()
}

// PlaylistConfig is the subset of playlist controls needed to apply config.
type PlaylistConfig interface {
	CycleRepeat()
	ToggleShuffle()
}

// ApplyPlayer applies audio-engine settings from the config.
func (c Config) ApplyPlayer(p PlayerConfig) {
	p.SetVolumeMin(c.VolumeMin)
	p.SetVolume(c.Volume)
	if c.Speed != 0 && c.Speed != 1.0 {
		p.SetSpeed(c.Speed)
	}
	if c.EQPreset == "" || c.EQPreset == "Custom" {
		for i, gain := range c.EQ {
			p.SetEQBand(i, gain)
		}
	}
	if c.Mono {
		p.ToggleMono()
	}
}

// ApplyPlaylist applies playlist-state settings from the config.
func (c Config) ApplyPlaylist(pl PlaylistConfig) {
	switch c.Repeat {
	case "all":
		pl.CycleRepeat() // off -> all
	case "one":
		pl.CycleRepeat() // off -> all
		pl.CycleRepeat() // all -> one
	}
	if c.Shuffle {
		pl.ToggleShuffle()
	}
}

// SeekStepLargeDuration returns the configured Shift+Left/Right seek jump.
func (c Config) SeekStepLargeDuration() time.Duration {
	return time.Duration(c.SeekStepLarge) * time.Second
}

// clamp constrains all Config fields to their valid ranges.
func (c *Config) clamp() {
	c.VolumeMin = max(min(c.VolumeMin, 0), -90)
	c.Volume = max(min(c.Volume, 6), c.VolumeMin)
	if c.Speed < 0.25 || c.Speed > 2.0 {
		c.Speed = 1.0
	}
	c.SeekStepLarge = max(min(c.SeekStepLarge, 600), 6)
	c.LyricsOffsetMs = max(min(c.LyricsOffsetMs, 10000), -10000)
	c.SampleRate = clampSampleRate(c.SampleRate)
	c.BufferMs = max(min(c.BufferMs, 5000), 50)
	c.ResampleQuality = max(min(c.ResampleQuality, 4), 1)
	c.BitDepth = clampBitDepth(c.BitDepth)
	c.PaddingH = max(min(c.PaddingH, 10), 0)
	c.PaddingV = max(min(c.PaddingV, 5), 0)
	if c.VisRows != 0 {
		c.VisRows = max(min(c.VisRows, maxVisRows), 1)
	}
	if c.LowPower {
		c.Visualizer = "none"
	}
}

// nearestAllowed returns the value in allowed closest to v.
// allowed must be non-empty.
func nearestAllowed(v int, allowed []int) int {
	best := allowed[0]
	bestDist := abs(v - best)
	for _, a := range allowed[1:] {
		if d := abs(v - a); d < bestDist {
			best = a
			bestDist = d
		}
	}
	return best
}

// clampSampleRate returns the nearest valid sample rate from the allowed set.
// A value of 0 is preserved as-is to signal "auto-detect" to the player.
func clampSampleRate(v int) int {
	if v == 0 {
		return 0 // auto-detect
	}
	return nearestAllowed(v, []int{22050, 44100, 48000, 96000, 192000})
}

// clampBitDepth returns the nearest valid bit depth (16 or 32).
func clampBitDepth(v int) int {
	if v >= 24 {
		return 32
	}
	return 16
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// listItems returns the items of a list value without the square brackets
// and without a # comment after the closing bracket. The list ends at the
// first ] outside quotes. A value that does not start with [ is returned
// unchanged.
func listItems(val string) string {
	if !strings.HasPrefix(val, "[") {
		return val
	}
	if end := listEnd(val); end > 0 && afterClose(val[end+1:]) {
		return val[1:end]
	}
	return strings.Trim(val, "[]")
}

// listEnd returns the index of the first ] outside quotes in val, or -1. A
// backslash inside double quotes hides the next byte, as in unquote.
func listEnd(val string) int {
	var quote byte
	for i := 1; i < len(val); i++ {
		c := val[i]
		switch {
		case quote == '"' && c == '\\':
			i++
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == ']':
			return i
		}
	}
	return -1
}

// splitItems splits the items of a list on each comma outside quotes. An
// item that starts with a quote ends its quoted text at the matching quote.
// A quote inside an unquoted item is text, as in Jazz's. A backslash inside
// double quotes hides the next byte, as in unquote.
func splitItems(items string) []string {
	var parts []string
	start, atStart := 0, true
	for i := 0; i < len(items); i++ {
		switch c := items[i]; {
		case c == ',':
			parts = append(parts, items[start:i])
			start, atStart = i+1, true
		case c == ' ' || c == '\t':
		case atStart && (c == '"' || c == '\''):
			for i++; i < len(items) && items[i] != c; i++ {
				if c == '"' && items[i] == '\\' {
					i++
				}
			}
			atStart = false
		default:
			atStart = false
		}
	}
	return append(parts, items[start:])
}

// parseStringSlice parses a comma-separated list of strings, optionally
// wrapped in square brackets (e.g. `["Music", "Jazz"]` or `Music, Jazz`).
// Each element is trimmed and unquoted like a single string value.
func parseStringSlice(val string) []string {
	val = listItems(val)
	parts := splitItems(val)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = unquote(strings.TrimSpace(p))
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseEQ parses a TOML-style array like [0, 1.5, -2, ...] into 10 bands.
func parseEQ(val string) [10]float64 {
	var bands [10]float64
	val = listItems(val)
	parts := strings.Split(val, ",")
	for i, p := range parts {
		if i >= 10 {
			break
		}
		// parseFloat drops a comment after the last band of a list without
		// brackets, and it skips a NaN or infinite band.
		if v, ok := parseFloat(strings.TrimSpace(p)); ok {
			bands[i] = max(min(v, 12), -12)
		}
	}
	return bands
}

// SponsorBlockConfig is the [sponsorblock] section.
type SponsorBlockConfig struct {
	Disabled   bool     // true only when the user sets enabled = false
	Categories []string // segment categories to skip, or nil for the defaults
}

func (s *SponsorBlockConfig) set(key, val string) {
	switch key {
	case "enabled":
		if v, ok := parseBool(val); ok {
			s.Disabled = !v
		}
	case "categories":
		s.Categories = parseStringList(val)
	}
}

// parseStringList reads a TOML array of strings such as ["a", "b"]. A bare
// comma-separated list works too.
func parseStringList(val string) []string {
	val = strings.TrimSpace(val)
	if strings.HasPrefix(val, "[") {
		// Drop a # comment after the closing bracket.
		if end := strings.IndexByte(val, ']'); end > 0 {
			val = val[:end+1]
		}
	} else if i := strings.Index(val, " #"); i >= 0 {
		val = val[:i]
	}
	val = strings.TrimSuffix(strings.TrimPrefix(val, "["), "]")
	var out []string
	for _, part := range strings.Split(val, ",") {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part != "" {
			out = append(out, strings.ToLower(part))
		}
	}
	return out
}

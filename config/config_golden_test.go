package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// loadConfigText writes data as config.toml in a temporary HOME and loads it.
func loadConfigText(t *testing.T, data string) Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(os.Getenv("HOME"), ".config", "cliamp-x", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return cfg
}

func readExampleConfig(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "config.toml.example"))
	if err != nil {
		t.Fatalf("read config.toml.example: %v", err)
	}
	// A Windows checkout can turn the line ends into CRLF.
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// exampleSetting matches a commented key or section header in the example.
var exampleSetting = regexp.MustCompile(`(?m)^# (\[[a-z]+\]|[a-z_]+ *=.*)$`)

// uncommentExample turns every commented key and section header of the
// example into a live line, the way a user turns a setting on.
func uncommentExample(s string) string {
	return exampleSetting.ReplaceAllString(s, "$1")
}

// goldenSections sets every key of every provider section. The unquoted
// values that contain # must reach the Config whole.
const goldenSections = `
[youtube]
cookies_from = " brave-origin "
expand_playlist = false
language = "es"
region = "cl"
auto_radio = false
report_history = true
quality_wifi = "Normal"
quality_mobile = "ahorro"
premium_quality = true

[scrobble]
listenbrainz_token = "${CLIAMP_GOLDEN_YT_SECRET}"
lastfm_api_key = " key "
lastfm_api_secret = 'secret'
lastfm_session_key = "sess"

[plugins]
disabled = webhook, discord-rpc

[plugins.lastfm]
api_key = abc#123
callback = https://example.com/cb#frag
note = "quoted # value"
`

// goldenTopLevel sets every top-level key to a value other than its default.
const goldenTopLevel = `
volume = -12.5
volume_min = -60
vis_volume_linked = false
vis_rows = 12
vis_sensitivity = 9
vis_palette = "fire"
repeat = 'ALL'
shuffle = true
mono = true
auto_play = true
seek_large_step_sec = 45
lyrics_offset_ms = -250
eq = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
eq_preset = "Custom"
eq_auto = true
theme = "Tokyo Night"
provider = "Local"
visualizer = "Wave"
sample_rate = 48000
buffer_ms = 500
resample_quality = 2
bit_depth = 32
speed = 1.25
simplified = true
hide_help_bar = true
hide_settings_pane = true
show_metadata = true
expanded = true
audio_device = "USB DAC #2"
initial_directory = ~/Music#Mixes
music_dir = "/srv/music"
music_dirs = ["/srv/music", "/mnt/usb # music"]   # two folders
padding_horizontal = 5
padding_vertical = 2
log_level = "DEBUG"
low_power = false
`

// goldenListComments puts a # comment after each list key. A # inside a
// quoted item stays in the item. A comment after an unquoted last item stays
// part of that item, as for any unquoted string.
const goldenListComments = `
eq = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]   # bass up
`

// TestLoadGolden loads whole config files and compares every Config field,
// so a parser change cannot move a value without a test failure.
func TestLoadGolden(t *testing.T) {
	t.Setenv("CLIAMP_GOLDEN_YT_SECRET", "yt-secret")
	t.Setenv("LISTENBRAINZ_TOKEN", "lb-from-env")
	example := readExampleConfig(t)
	f, tr := false, true

	exampleWant := defaultConfig()
	exampleWant.EQPreset = "Flat"

	sectionsWant := exampleWant
	sectionsWant.YouTubeMusic = YouTubeMusicConfig{
		CookiesFrom:    "brave-origin",
		ExpandPlaylist: &f,
		Language:       "es",
		Region:         "CL",
		AutoRadio:      &f,
		ReportHistory:  true,
		QualityWifi:    "normal",
		QualityMobile:  "ahorro",
		PremiumQuality: true,
	}
	sectionsWant.Scrobble = ScrobbleConfig{ListenBrainzToken: "yt-secret", LastfmAPIKey: "key", LastfmAPISecret: "secret", LastfmSessionKey: "sess"}
	sectionsWant.Plugins = map[string]map[string]string{
		"": {"disabled": "webhook, discord-rpc"},
		"lastfm": {
			"api_key":  "abc#123",
			"callback": "https://example.com/cb#frag",
			"note":     "quoted # value",
		},
	}

	topWant := defaultConfig()
	topWant.Volume = -12.5
	topWant.VolumeMin = -60
	topWant.VisVolumeLinked = false
	topWant.VisRows = 12
	topWant.VisSensitivity = 9
	topWant.VisPalette = "fire"
	topWant.Repeat = "all"
	topWant.Shuffle = true
	topWant.Mono = true
	topWant.AutoPlay = true
	topWant.SeekStepLarge = 45
	topWant.LyricsOffsetMs = -250
	topWant.EQ = [10]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	topWant.EQPreset = "Custom"
	topWant.EQAuto = true
	topWant.Theme = "Tokyo Night"
	topWant.Provider = "local"
	topWant.Visualizer = "Wave"
	topWant.SampleRate = 48000
	topWant.BufferMs = 500
	topWant.ResampleQuality = 2
	topWant.BitDepth = 32
	topWant.Speed = 1.25
	topWant.Simplified = true
	topWant.HideHelpBar = true
	topWant.HideSettingsPane = true
	topWant.ShowMetadata = true
	topWant.Expanded = true
	topWant.AudioDevice = "USB DAC #2"
	topWant.InitialDirectory = "~/Music#Mixes"
	topWant.MusicDir = "/srv/music"
	topWant.MusicDirs = []string{"/srv/music", "/mnt/usb # music"}
	topWant.PaddingH = 5
	topWant.PaddingV = 2
	topWant.LogLevel = "debug"

	// Every setting of the example turned on. The example puts # comments
	// after sample_rate and the other audio numbers.
	allWant := exampleWant
	allWant.VisRows = 7
	allWant.VisSensitivity = 7
	allWant.VisPalette = "theme"
	allWant.InitialDirectory = "~/Music"
	allWant.MusicDir = "~/Music"
	allWant.MusicDirs = []string{"~/Music", "~/Downloads/Music"}
	allWant.AudioDevice = "alsa_output.usb-FiiO_K5_Pro-00.analog-stereo"
	allWant.Provider = "ytmusic"
	allWant.Simplified = true
	allWant.HideHelpBar = true
	allWant.HideSettingsPane = true
	allWant.HidePreview = true
	allWant.HideFullCover = true
	allWant.MouseOff = true
	allWant.Language = "es"
	allWant.NotifyOff = true
	allWant.PreviewImages = "auto"
	allWant.Normalize = "track"
	allWant.NormalizeTarget = -14
	allWant.Crossfade = 6
	allWant.Scrobble = ScrobbleConfig{ListenBrainzToken: "lb-from-env"}
	allWant.SponsorBlock = SponsorBlockConfig{Categories: []string{"music_offtopic", "intro", "outro", "sponsor", "selfpromo"}}
	allWant.Expanded = true
	allWant.Theme = "Tokyo Night"
	allWant.Visualizer = "Bars"
	allWant.YouTubeMusic = YouTubeMusicConfig{
		CookiesFrom:    "brave-origin",
		ExpandPlaylist: &tr,
		Language:       "es",
		Region:         "CL",
		AutoRadio:      &f,
		QualityWifi:    "alta",
		QualityMobile:  "normal",
	}

	listsWant := defaultConfig()
	listsWant.EQ = [10]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		name string
		data string
		want Config
	}{
		{"example as shipped", example, exampleWant},
		{"example with every setting turned on", uncommentExample(example), allWant},
		{"example plus every provider section", example + goldenSections, sectionsWant},
		{"every top-level key", goldenTopLevel, topWant},
		{"comments after lists", goldenListComments, listsWant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := loadConfigText(t, tt.data)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Load() mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

package config

import (
	"reflect"
	"testing"
)

func TestParseBool(t *testing.T) {
	tests := []struct {
		in     string
		want   bool
		wantOK bool
	}{
		{"true", true, true},
		{"True", true, true},
		{"TRUE", true, true},
		{"tRUE", true, true},
		{"1", true, true},
		{"false", false, true},
		{"False", false, true},
		{"FALSE", false, true},
		{"0", false, true},
		{"t", true, true},
		{"F", false, true},
		{"", false, false},
		{"yes", false, false},
		{`"true"`, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := parseBool(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("parseBool(%q) = %v, %v, want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestLoadBoolKeysIgnoreLetterCase checks that every bool key reads its value
// the same way. Before parseBool, shuffle = True read as false while
// expanded = True read as true.
func TestLoadBoolKeysIgnoreLetterCase(t *testing.T) {
	tests := []struct {
		name string
		data string
		got  func(Config) bool
		want bool
	}{
		{"shuffle", "shuffle = True", func(c Config) bool { return c.Shuffle }, true},
		{"mono", "mono = TRUE", func(c Config) bool { return c.Mono }, true},
		{"auto_play", "auto_play = True", func(c Config) bool { return c.AutoPlay }, true},
		{"simplified", "simplified = True", func(c Config) bool { return c.Simplified }, true},
		{"hide_help_bar", "hide_help_bar = True", func(c Config) bool { return c.HideHelpBar }, true},
		{"hide_settings_pane", "hide_settings_pane = True", func(c Config) bool { return c.HideSettingsPane }, true},
		{"show_metadata", "show_metadata = True", func(c Config) bool { return c.ShowMetadata }, true},
		{"expanded", "expanded = True", func(c Config) bool { return c.Expanded }, true},
		{"low_power", "low_power = True", func(c Config) bool { return c.LowPower }, true},
		{"vis_volume_linked", "vis_volume_linked = False", func(c Config) bool { return c.VisVolumeLinked }, false},
		{"invalid keeps default", "vis_volume_linked = maybe", func(c Config) bool { return c.VisVolumeLinked }, true},
		{"report_history", "[ytmusic]\nreport_history = True", func(c Config) bool { return c.YouTubeMusic.ReportHistory }, true},
		{"report_history one", "[ytmusic]\nreport_history = 1", func(c Config) bool { return c.YouTubeMusic.ReportHistory }, true},
		{"auto_radio", "[ytmusic]\nauto_radio = FALSE", func(c Config) bool { return c.YouTubeMusic.AutoRadioEnabled() }, false},
		{"ytmusic enabled", "[ytmusic]\nenabled = False", func(c Config) bool { return c.YouTubeMusic.Disabled }, true},
		{"ytmusic expand_playlist", "[ytmusic]\nexpand_playlist = False", func(c Config) bool {
			return c.YouTubeMusic.ExpandPlaylist != nil && !*c.YouTubeMusic.ExpandPlaylist
		}, true},
		{"ytmusic expand_playlist invalid", "[ytmusic]\nexpand_playlist = maybe", func(c Config) bool {
			return c.YouTubeMusic.ExpandPlaylist == nil
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := loadConfigText(t, tt.data+"\n")
			if got := tt.got(cfg); got != tt.want {
				t.Fatalf("%q: got %v, want %v", tt.data, got, tt.want)
			}
		})
	}
}

func TestQuoteStringRoundTrip(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"plain", `"plain"`},
		{`a\b`, `"a\\b"`},
		{`p"w`, `"p\"w"`},
		{`'x'`, `"'x'"`},
		{`abc\`, `"abc\\"`},
		{`\"`, `"\\\""`},
		{`D:\new`, `"D:\\new"`},
		{"pa#ss word", `"pa#ss word"`},
		{" spaced ", `" spaced "`},
		{"tab\there", "\"tab\there\""},
		{"ünïcode", `"ünïcode"`},
		{"", `""`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := QuoteString(tt.in)
			if got != tt.want {
				t.Fatalf("QuoteString(%q) = %s, want %s", tt.in, got, tt.want)
			}
			if back := unquote(got); back != tt.in {
				t.Fatalf("unquote(%s) = %q, want %q", got, back, tt.in)
			}
		})
	}
}

func TestLoadDecodesQuotedStrings(t *testing.T) {
	cfg := loadConfigText(t, `
[ytmusic]
cookies_from = 'firefox:~/.mozilla'
language = "'es'"
region = "c\\l\"x"

[downloads]
directory = "~/Mus\"ic"
`)
	if got, want := cfg.YouTubeMusic.CookiesFrom, "firefox:~/.mozilla"; got != want {
		t.Errorf("YouTubeMusic.CookiesFrom = %q, want %q", got, want)
	}
	if got, want := cfg.YouTubeMusic.Language, "'es'"; got != want {
		t.Errorf("YouTubeMusic.Language = %q, want %q", got, want)
	}
	if got, want := cfg.YouTubeMusic.Region, `C\L"X`; got != want {
		t.Errorf("YouTubeMusic.Region = %q, want %q", got, want)
	}
	if got, want := cfg.Downloads.Directory, `~/Mus"ic`; got != want {
		t.Errorf("Downloads.Directory = %q, want %q", got, want)
	}
}

func TestScalar(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"-5", "-5"},
		{"-5 # quieter", "-5"},
		{"true\t# on", "true"},
		{"1.25   #faster", "1.25"},
		{"5#x", "5#x"},
		{"5 x", "5 x"},
		{"5 x # y", "5 x # y"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := scalar(tt.in); got != tt.want {
				t.Fatalf("scalar(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestLoadInlineComments checks that a # comment after a quoted string, a
// number or a bool no longer resets the key, and that a # in an unquoted
// string value stays part of the value.
func TestLoadInlineComments(t *testing.T) {
	cfg := loadConfigText(t, `
volume = -5 # quieter
speed = 1.5	# tab before the comment
shuffle = true # on
expanded = True   # any case
repeat = "all" # loop the list
seek_large_step_sec = 10#no space, so the value is invalid

[ytmusic]
cookies_from = "brave" # mine
report_history = true # on
language = es #x

[plugins.lastfm]
api_key = abc #123
`)
	checks := []struct {
		name      string
		got, want any
	}{
		{"Volume", cfg.Volume, -5.0},
		{"Speed", cfg.Speed, 1.5},
		{"Shuffle", cfg.Shuffle, true},
		{"Expanded", cfg.Expanded, true},
		{"Repeat", cfg.Repeat, "all"},
		{"SeekStepLarge", cfg.SeekStepLarge, 30},
		{"YouTubeMusic.CookiesFrom", cfg.YouTubeMusic.CookiesFrom, "brave"},
		{"YouTubeMusic.ReportHistory", cfg.YouTubeMusic.ReportHistory, true},
		{"YouTubeMusic.Language", cfg.YouTubeMusic.Language, "es #x"},
		{"plugins.lastfm.api_key", cfg.Plugins["lastfm"]["api_key"], "abc #123"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %#v, want %#v", c.name, c.got, c.want)
		}
	}
}

func TestSectionHeader(t *testing.T) {
	tests := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"[navidrome]", "navidrome", true},
		{"[plugins.lastfm]", "plugins.lastfm", true},
		{"[navidrome] # my server", "navidrome", true},
		{"[navidrome]\t# my server", "navidrome", true},
		{"[navidrome]# no space", "navidrome", true},
		{"[navidrome]x", "", false},
		{"[[dir]]", "[dir]", true},
		{"navidrome]", "", false},
		{"[navidrome", "", false},
		{"key = [1, 2]", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := sectionHeader(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("sectionHeader(%q) = %q, %v, want %q, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestLoadCommentAfterSectionHeader checks that keys after a commented
// header land in that section and not in the section before it.
func TestLoadCommentAfterSectionHeader(t *testing.T) {
	cfg := loadConfigText(t, `
[downloads]
directory = "/music"

[ytmusic] # my account
cookies_from = "brave"
`)
	if got, want := cfg.YouTubeMusic.CookiesFrom, "brave"; got != want {
		t.Errorf("YouTubeMusic.CookiesFrom = %q, want %q", got, want)
	}
	if got, want := cfg.Downloads.Directory, "/music"; got != want {
		t.Errorf("Downloads.Directory = %q, want %q", got, want)
	}
}

func TestIsComment(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"  ", true},
		{" # note", true},
		{"\t#note", true},
		{"# no space", false},
		{" x # note", false},
		{"x", false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := isComment(tt.in); got != tt.want {
				t.Fatalf("isComment(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestQuotedItemsAndCommentsAfterClose checks that a comma inside a quoted
// list item does not split the item, and that a # comment may follow a
// closing quote or bracket with no whitespace before it.
func TestQuotedItemsAndCommentsAfterClose(t *testing.T) {
	tests := []struct {
		name string
		got  any
		want any
	}{
		{"comment after a double quote", unquote(`"Nord"#x`), "Nord"},
		{"comment after a single quote", unquote(`'Nord'#x`), "Nord"},
		{"text after a quote", unquote(`"Nord"x`), `"Nord"x`},
		{"comma in a quoted item", parseStringSlice(`["a,b", "c"]`), []string{"a,b", "c"}},
		{"comma in a single-quoted item", parseStringSlice(`['a, b', c]`), []string{"a, b", "c"}},
		{"comma in a quoted item without brackets", parseStringSlice(`"a,b", c`), []string{"a,b", "c"}},
		{"escaped quote and comma in an item", parseStringSlice(`["a\",b", c]`), []string{`a",b`, "c"}},
		{"apostrophe in an unquoted item", parseStringSlice(`Jazz's, Rock`), []string{"Jazz's", "Rock"}},
		{"list comment after the bracket", parseStringSlice(`["Music"]#x`), []string{"Music"}},
		{"eq comment after the bracket", parseEQ(`[1,2,3]#c`), [10]float64{1, 2, 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Fatalf("got %#v, want %#v", tt.got, tt.want)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		in     string
		want   float64
		wantOK bool
	}{
		{"-5", -5, true},
		{"1.25 # faster", 1.25, true},
		{"", 0, false},
		{"loud", 0, false},
		{"nan", 0, false},
		{"NaN", 0, false},
		{"inf", 0, false},
		{"-Inf", 0, false},
		{"+infinity", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := parseFloat(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("parseFloat(%q) = %v, %v, want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// TestLoadNonFiniteNumbers checks that NaN and an infinity keep the default.
// clamp lets NaN through, and a saved EQ wrote NaN back to the file.
func TestLoadNonFiniteNumbers(t *testing.T) {
	tests := []struct {
		name string
		data string
		got  func(Config) any
		want any
	}{
		{"volume nan", "volume = nan", func(c Config) any { return c.Volume }, 0.0},
		{"volume inf", "volume = inf", func(c Config) any { return c.Volume }, 0.0},
		{"volume_min nan", "volume_min = nan", func(c Config) any { return c.VolumeMin }, -50.0},
		{"speed nan", "speed = NaN", func(c Config) any { return c.Speed }, 1.0},
		{"eq nan band", "eq = [nan, 1]", func(c Config) any { return c.EQ }, [10]float64{0, 1}},
		{"eq inf band", "eq = [1, -inf, 2]", func(c Config) any { return c.EQ }, [10]float64{1, 0, 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got(loadConfigText(t, tt.data)); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

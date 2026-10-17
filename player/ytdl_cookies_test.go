package player

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/ytdlcookies"
)

func TestAppendYTDLCookieArgsSelectsBrowserByURLHost(t *testing.T) {
	ytdlcookies.SetForHost("mixcloud.com", "firefox")
	ytdlcookies.SetForHost("music.163.com", "chrome")
	t.Cleanup(func() {
		ytdlcookies.SetForHost("mixcloud.com", "")
		ytdlcookies.SetForHost("music.163.com", "")
	})

	tests := []struct {
		url  string
		want []string
	}{
		{
			url:  "https://www.mixcloud.com/creator/show/",
			want: []string{"yt-dlp", "--cookies-from-browser", "firefox"},
		},
		{
			url:  "https://music.163.com/#/song?id=1",
			want: []string{"yt-dlp", "--cookies-from-browser", "chrome"},
		},
		{
			url:  "https://example.com/track",
			want: []string{"yt-dlp"},
		},
	}

	for _, test := range tests {
		got := appendYTDLCookieArgs([]string{"yt-dlp"}, test.url)
		if !slices.Equal(got, test.want) {
			t.Errorf("appendYTDLCookieArgs(%q) = %v, want %v", test.url, got, test.want)
		}
	}
}

// installCookielessFixtures puts a fake yt-dlp and ffmpeg on PATH. yt-dlp logs
// each argv as one line in the returned file. Without --cookies it behaves as
// mode says: "ok" works, "signin" fails like a page that needs the session and
// "hang" never answers. -J prints info, --print duration prints 212 and any
// other run sends audio.
func installCookielessFixtures(t *testing.T, mode string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX process fixtures")
	}
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	writeExecutable(t, filepath.Join(dir, "yt-dlp"), `#!/bin/sh
echo "$*" >> "$YTDL_CALLS"
n=0
for a in "$@"; do
  if [ "$n" = 2 ]; then echo "opus 129.5 48000 212" > "$a"; n=0
  elif [ "$n" = 1 ]; then n=2
  elif [ "$a" = "--print-to-file" ]; then n=1
  fi
done
case "$*" in
  *--cookies*) ;;
  *) case "$YTDL_MODE" in
       signin) echo "ERROR: [youtube] x: Sign in to confirm your age" >&2; exit 1 ;;
       hang) exec sleep 30 ;;
     esac ;;
esac
case "$*" in
  *" -J "*) echo '{"id":"x"}' ;;
  *) head -c 4096 /dev/zero ;;
esac
`)
	writeExecutable(t, filepath.Join(dir, "ffmpeg"), "#!/bin/sh\nexec cat\n")
	t.Setenv("YTDL_CALLS", calls)
	t.Setenv("YTDL_MODE", mode)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return calls
}

// formatFile matches the temporary file of --print-to-file.
var formatFile = regexp.MustCompile(`\S*cliamp-x-format-\S*`)

// infoFile matches the page info template of --write-info-json.
var infoFile = regexp.MustCompile(`infojson:\S*`)

// On Android the runs that resolve a signed-in YouTube page for playback go
// first without the cookies, and with them only when that run fails. Other
// platforms and other hosts keep their cookies on the one run.
func TestYTDLCookielessFirst(t *testing.T) {
	const (
		cookies = "--cookies /cfg/ytmusic_cookies.txt"
		music   = "https://music.youtube.com/watch?v=abc"
		video   = "https://www.youtube.com/watch?v=abc"
		short   = "https://youtu.be/abc"
		cloud   = "https://soundcloud.com/artist/track"
	)
	// The argv before the cookies of each run, and how to start it.
	// tail follows the cookies: a stream start saves the page info it
	// extracts, for the seeks of the track.
	runs := []struct {
		name string
		args string
		tail string
		run  func(t *testing.T, url string)
	}{
		{
			name: "stream",
			args: "-f " + ytdlFormat + " --no-playlist --quiet --no-warnings --socket-timeout 15 -o - --print-to-file " + ytdlFormatTemplate + " FORMAT",
			tail: " --write-info-json -o infojson:INFO",
			run: func(t *testing.T, url string) {
				p := &Player{sr: 44100, bitDepth: 16}
				t.Cleanup(p.ytdlInfo.close)
				tp, err := p.buildYTDLPipeline(url, 0)
				if err != nil {
					t.Fatalf("buildYTDLPipeline: %v", err)
				}
				if want := (AudioFormat{Codec: "opus", Kbps: 130, Rate: 48000}); tp.audio != want {
					t.Errorf("audio = %+v, want %+v", tp.audio, want)
				}
				// The duration comes from the same run: no probe.
				if tp.knownDuration != 212*time.Second {
					t.Errorf("duration = %v, want 3m32s", tp.knownDuration)
				}
				tp.close()
			},
		},
		{
			name: "info prefetch",
			args: "-f " + ytdlFormat + " --no-playlist --quiet --no-warnings --socket-timeout 15 -J",
			run: func(t *testing.T, url string) {
				path := filepath.Join(t.TempDir(), "info.json")
				if err := fetchYTDLInfo(context.Background(), url, path); err != nil {
					t.Fatalf("fetchYTDLInfo: %v", err)
				}
				if b, _ := os.ReadFile(path); !strings.Contains(string(b), `"id"`) {
					t.Fatalf("info file = %q", b)
				}
			},
		},
	}
	tests := []struct {
		name     string
		android  bool
		premium  bool
		signedIn bool
		mode     string
		url      string
		want     []bool // per yt-dlp run: whether it has the cookies
	}{
		{name: "desktop keeps the cookies", signedIn: true, mode: "ok", url: music, want: []bool{true}},
		{name: "desktop session-only page", signedIn: true, mode: "signin", url: music, want: []bool{true}},
		{name: "android YouTube Music without cookies", android: true, signedIn: true, mode: "ok", url: music, want: []bool{false}},
		{name: "android YouTube without cookies", android: true, signedIn: true, mode: "ok", url: video, want: []bool{false}},
		{name: "android youtu.be without cookies", android: true, signedIn: true, mode: "ok", url: short, want: []bool{false}},
		{name: "android falls back on a failed run", android: true, signedIn: true, mode: "signin", url: music, want: []bool{false, true}},
		{name: "android falls back on a hung run", android: true, signedIn: true, mode: "hang", url: music, want: []bool{false, true}},
		{name: "android signed out runs once", android: true, mode: "ok", url: music, want: []bool{false}},
		{name: "android other host keeps its cookies", android: true, signedIn: true, mode: "ok", url: cloud, want: []bool{true}},
		{name: "android premium quality keeps the cookies", android: true, premium: true, signedIn: true, mode: "ok", url: music, want: []bool{true}},
	}
	for _, tt := range tests {
		for _, r := range runs {
			t.Run(tt.name+"/"+r.name, func(t *testing.T) {
				calls := installCookielessFixtures(t, tt.mode)
				first, timeout := ytdlCookielessFirst, ytdlCookielessTimeout
				ytdlCookielessFirst, ytdlCookielessTimeout = tt.android, 300*time.Millisecond
				t.Cleanup(func() { ytdlCookielessFirst, ytdlCookielessTimeout = first, timeout })
				SetYTDLPremium(tt.premium)
				t.Cleanup(func() { SetYTDLPremium(false) })
				if tt.signedIn {
					for _, host := range []string{"youtube.com", "youtu.be", "music.youtube.com", "soundcloud.com"} {
						ytdlcookies.SetForHost(host, ytdlcookies.FilePrefix+"/cfg/ytmusic_cookies.txt")
						t.Cleanup(func() { ytdlcookies.SetForHost(host, "") })
					}
				}

				start := time.Now()
				r.run(t, tt.url)
				if elapsed := time.Since(start); elapsed > 5*time.Second {
					t.Fatalf("took %v, want the fallback within the cookieless bound", elapsed)
				}
				var want []string
				for _, withCookies := range tt.want {
					argv := r.args
					if withCookies {
						argv += " " + cookies
					}
					want = append(want, argv+r.tail+" -- "+tt.url)
				}
				got := readLines(t, calls)
				for i := range got {
					got[i] = formatFile.ReplaceAllString(got[i], "FORMAT")
					got[i] = infoFile.ReplaceAllString(got[i], "infojson:INFO")
				}
				if !slices.Equal(got, want) {
					t.Fatalf("yt-dlp runs:\n got %q\nwant %q", got, want)
				}
			})
		}
	}
}

// A cookieless run that ends because its caller gave up, as when a prefetch is
// evicted, does not start the cookie run.
func TestYTDLOutputNoFallbackAfterTheCallerGivesUp(t *testing.T) {
	calls := installCookielessFixtures(t, "hang")
	first := ytdlCookielessFirst
	ytdlCookielessFirst = true
	t.Cleanup(func() { ytdlCookielessFirst = first })
	ytdlcookies.SetForHost("music.youtube.com", ytdlcookies.FilePrefix+"/cfg/ytmusic_cookies.txt")
	t.Cleanup(func() { ytdlcookies.SetForHost("music.youtube.com", "") })

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if _, err := ytdlOutput(ctx, "https://music.youtube.com/watch?v=abc", "-J"); err == nil {
		t.Fatal("ytdlOutput = nil error, want the cancelled run's error")
	}
	if got := readLines(t, calls); len(got) != 1 || strings.Contains(got[0], "--cookies") {
		t.Fatalf("yt-dlp runs = %q, want only the cookieless one", got)
	}
}

func TestYTDLFormatSpec(t *testing.T) {
	tests := []struct {
		kbps int
		want string
	}{
		{0, ytdlFormat},
		{-5, ytdlFormat},
		{64, "bestaudio[abr<=?64][protocol=https]/bestaudio[abr<=?64][protocol=http]/" + ytdlFormat},
		{128, "bestaudio[abr<=?128][protocol=https]/bestaudio[abr<=?128][protocol=http]/" + ytdlFormat},
	}
	t.Cleanup(func() { SetYTDLQuality(0) })
	for _, tt := range tests {
		SetYTDLQuality(tt.kbps)
		if got := ytdlFormatSpec(); got != tt.want {
			t.Errorf("SetYTDLQuality(%d): ytdlFormatSpec() = %q, want %q", tt.kbps, got, tt.want)
		}
	}
}

func TestParseYTDLFormat(t *testing.T) {
	tests := []struct {
		line string
		want AudioFormat
		dur  time.Duration
	}{
		{"opus 129.478 48000 213\n", AudioFormat{Codec: "opus", Kbps: 129, Rate: 48000}, 213 * time.Second},
		{"mp4a.40.2 130.2 44100 6060.5", AudioFormat{Codec: "aac", Kbps: 130, Rate: 44100}, 6060500 * time.Millisecond},
		{"NA NA NA NA", AudioFormat{}, 0},
		{"none 64 NA NA", AudioFormat{Kbps: 64}, 0},
		{"", AudioFormat{}, 0},
		{"opus 64 48000", AudioFormat{}, 0},
	}
	for _, tt := range tests {
		if got, dur := parseYTDLFormat(tt.line); got != tt.want || dur != tt.dur {
			t.Errorf("parseYTDLFormat(%q) = %+v, %v; want %+v, %v", tt.line, got, dur, tt.want, tt.dur)
		}
	}
}

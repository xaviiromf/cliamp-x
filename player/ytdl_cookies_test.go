package player

import (
	"context"
	"os"
	"path/filepath"
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
case "$*" in
  *--cookies*) ;;
  *) case "$YTDL_MODE" in
       signin) echo "ERROR: [youtube] x: Sign in to confirm your age" >&2; exit 1 ;;
       hang) exec sleep 30 ;;
     esac ;;
esac
case "$*" in
  *" -J "*) echo '{"id":"x"}' ;;
  *"--print duration"*) echo 212 ;;
  *) head -c 4096 /dev/zero ;;
esac
`)
	writeExecutable(t, filepath.Join(dir, "ffmpeg"), "#!/bin/sh\nexec cat\n")
	t.Setenv("YTDL_CALLS", calls)
	t.Setenv("YTDL_MODE", mode)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return calls
}

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
	runs := []struct {
		name string
		args string
		run  func(t *testing.T, url string)
	}{
		{
			name: "stream",
			args: "-f " + ytdlFormat + " --no-playlist --quiet --no-warnings --socket-timeout 15 -o -",
			run: func(t *testing.T, url string) {
				p := &Player{sr: 44100, bitDepth: 16}
				tp, err := p.buildYTDLPipeline(url, 0)
				if err != nil {
					t.Fatalf("buildYTDLPipeline: %v", err)
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
		{
			name: "duration probe",
			args: "--skip-download --no-playlist --socket-timeout 10 --print duration",
			run: func(t *testing.T, url string) {
				if d := probeYTDLDuration(url); d != 212*time.Second {
					t.Fatalf("probeYTDLDuration = %v, want 212s", d)
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
					want = append(want, argv+" -- "+tt.url)
				}
				if got := readLines(t, calls); !slices.Equal(got, want) {
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

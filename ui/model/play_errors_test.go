package model

import (
	"errors"
	"strings"
	"testing"
)

func TestPlayErrorHint(t *testing.T) {
	for _, tc := range []struct{ err, want string }{
		{"yt-dlp: ERROR: [youtube] abc: Sign in to confirm you're not a bot", "cliamp-x login"},
		{"yt-dlp: ERROR: [youtube] abc: nsig extraction failed: You may experience throttling", "yt-dlp -U"},
		{"yt-dlp: ERROR: unable to download webpage: <urlopen error [Errno -3] Temporary failure in name resolution>", "check the network"},
		{"yt-dlp: ERROR: [youtube] abc: Video unavailable", "unavailable"},
		{"ffmpeg: invalid data found when processing input", ""},
	} {
		got := playErrorHint(errors.New(tc.err))
		if tc.want == "" && got != "" || !strings.Contains(got, tc.want) {
			t.Errorf("playErrorHint(%q) = %q, want it to say %q", tc.err, got, tc.want)
		}
	}
	if playErrorHint(nil) != "" {
		t.Error("no error, no hint")
	}
}

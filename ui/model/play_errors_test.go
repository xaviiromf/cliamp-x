package model

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/playlist"
)

func TestPlayErrorHint(t *testing.T) {
	for _, tc := range []struct {
		err        error
		kind, want string
	}{
		{errors.New("yt-dlp: ERROR: [youtube] abc: Sign in to confirm you're not a bot"), "session", "cliamp-x login"},
		{errors.New("yt-dlp: ERROR: [youtube] abc: nsig extraction failed: You may experience throttling"), "ytdlp", "yt-dlp -U"},
		{errors.New("yt-dlp: ERROR: unable to download webpage: <urlopen error [Errno -3] Temporary failure in name resolution>"), "network", "check the network"},
		{errors.New("yt-dlp: ERROR: [youtube] abc: Video unavailable"), "unavailable", "unavailable"},
		{errors.New("yt-dlp: exit status 1: ERROR: [youtube] abc: This video is unavailable"), "unavailable", "unavailable"},
		{fmt.Errorf("open stream: %w", playlist.ErrNeedsAuth), "session", "cliamp-x login"},
		{errors.New("ffmpeg: invalid data found when processing input"), "", ""},
		{nil, "", ""},
	} {
		if got := playErrorKind(tc.err); got != tc.kind {
			t.Errorf("playErrorKind(%v) = %q, want %q", tc.err, got, tc.kind)
		}
		got := playErrorHint(tc.err)
		if tc.want == "" && got != "" || !strings.Contains(got, tc.want) {
			t.Errorf("playErrorHint(%v) = %q, want it to say %q", tc.err, got, tc.want)
		}
	}
}

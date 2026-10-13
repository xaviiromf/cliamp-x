// Package extbin runs the external programs yt-dlp and ffmpeg. By default it
// runs them from PATH. CLIAMP_YTDLP and CLIAMP_FFMPEG replace a program with
// another command line, and CLIAMP_YTDLP_ENV and CLIAMP_FFMPEG_ENV add
// environment entries for it. The Android app sets them, because there yt-dlp
// is Python plus a zip and ffmpeg is a library file.
package extbin

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// overrides maps each program to the variable that replaces it.
var overrides = map[string]string{"yt-dlp": "CLIAMP_YTDLP", "ffmpeg": "CLIAMP_FFMPEG"}

// argv returns the command line that runs name.
// shortcut: an override splits on whitespace with no quoting, as Android app
// paths have no spaces; add quoting when a path needs one.
func argv(name string) []string {
	if key, ok := overrides[name]; ok {
		if v := strings.Fields(os.Getenv(key)); len(v) > 0 {
			return v
		}
	}
	return []string{name}
}

// Env returns the extra environment entries of name: the "K=V" pairs of
// CLIAMP_YTDLP_ENV or CLIAMP_FFMPEG_ENV, separated by the byte 0x1f. It
// drops an entry with no key.
func Env(name string) []string {
	key, ok := overrides[name]
	if !ok {
		return nil
	}
	var env []string
	for _, kv := range strings.Split(os.Getenv(key+"_ENV"), "\x1f") {
		if strings.IndexByte(kv, '=') > 0 {
			env = append(env, kv)
		}
	}
	return env
}

// Have reports whether name can run.
func Have(name string) bool {
	_, err := exec.LookPath(argv(name)[0])
	return err == nil
}

// Command returns the command that runs name with args. With CLIAMP_FFMPEG
// set, yt-dlp gets --ffmpeg-location, as it then finds no ffmpeg on PATH.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	a := argv(name)
	if ff := strings.Fields(os.Getenv("CLIAMP_FFMPEG")); name == "yt-dlp" && len(ff) > 0 {
		a = append(a, "--ffmpeg-location", ff[0])
	}
	cmd := exec.CommandContext(ctx, a[0], append(a[1:], args...)...)
	if env := Env(name); len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	return cmd
}

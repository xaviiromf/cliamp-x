package extbin

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestCommand(t *testing.T) {
	tests := []struct {
		name     string
		program  string
		env      map[string]string
		wantArgs []string // after the program path
		wantPath string   // "" means a PATH lookup of program
		wantEnv  []string // the entries added to os.Environ, nil for none
	}{
		{name: "yt-dlp from PATH", program: "yt-dlp", wantArgs: []string{"yt-dlp", "-J"}},
		{name: "ffmpeg from PATH", program: "ffmpeg", wantArgs: []string{"ffmpeg", "-J"}},
		{
			name:     "yt-dlp override",
			program:  "yt-dlp",
			env:      map[string]string{"CLIAMP_YTDLP": " /app/libpython.so  /app/yt-dlp "},
			wantPath: "/app/libpython.so",
			wantArgs: []string{"/app/libpython.so", "/app/yt-dlp", "-J"},
		},
		{
			name:     "yt-dlp gets the ffmpeg of the override",
			program:  "yt-dlp",
			env:      map[string]string{"CLIAMP_FFMPEG": "/app/libffmpeg.so -nostdin"},
			wantArgs: []string{"yt-dlp", "--ffmpeg-location", "/app/libffmpeg.so", "-J"},
		},
		{
			name:     "ffmpeg override with env",
			program:  "ffmpeg",
			env:      map[string]string{"CLIAMP_FFMPEG": "/app/libffmpeg.so", "CLIAMP_FFMPEG_ENV": "LD_LIBRARY_PATH=/app/lib\x1fA=b=c"},
			wantPath: "/app/libffmpeg.so",
			wantArgs: []string{"/app/libffmpeg.so", "-J"},
			wantEnv:  []string{"LD_LIBRARY_PATH=/app/lib", "A=b=c"},
		},
		{
			name:     "env drops empty and keyless entries",
			program:  "yt-dlp",
			env:      map[string]string{"CLIAMP_YTDLP_ENV": "\x1fPYTHONHOME=/py\x1f=x\x1fnoequals\x1f"},
			wantArgs: []string{"yt-dlp", "-J"},
			wantEnv:  []string{"PYTHONHOME=/py"},
		},
		{
			name:     "blank override falls back to PATH",
			program:  "yt-dlp",
			env:      map[string]string{"CLIAMP_YTDLP": "   "},
			wantArgs: []string{"yt-dlp", "-J"},
		},
		{
			name:     "other programs ignore the overrides",
			program:  "ffprobe",
			env:      map[string]string{"CLIAMP_FFMPEG": "/app/libffmpeg.so", "CLIAMP_FFMPEG_ENV": "A=b"},
			wantArgs: []string{"ffprobe", "-J"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"CLIAMP_YTDLP", "CLIAMP_YTDLP_ENV", "CLIAMP_FFMPEG", "CLIAMP_FFMPEG_ENV"} {
				t.Setenv(key, tt.env[key])
			}
			cmd := Command(context.Background(), tt.program, "-J")
			if !slices.Equal(cmd.Args, tt.wantArgs) {
				t.Errorf("Args = %q, want %q", cmd.Args, tt.wantArgs)
			}
			if tt.wantPath != "" && cmd.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", cmd.Path, tt.wantPath)
			}
			if tt.wantEnv == nil {
				if cmd.Env != nil {
					t.Errorf("Env = %q, want the inherited environment", cmd.Env)
				}
				return
			}
			if added := cmd.Env[len(os.Environ()):]; !slices.Equal(added, tt.wantEnv) {
				t.Errorf("added Env = %q, want %q", added, tt.wantEnv)
			}
		})
	}
}

func TestHave(t *testing.T) {
	dir := t.TempDir()
	python := filepath.Join(dir, "libpython.so")
	if err := os.WriteFile(python, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, override string
		want           bool
	}{
		{name: "not on PATH", want: false},
		{name: "override runs", override: python + " " + filepath.Join(dir, "yt-dlp"), want: true},
		{name: "override missing", override: filepath.Join(dir, "missing"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			t.Setenv("CLIAMP_YTDLP", tt.override)
			if got := Have("yt-dlp"); got != tt.want {
				t.Fatalf("Have(yt-dlp) = %v, want %v", got, tt.want)
			}
		})
	}
}

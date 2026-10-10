package main

import (
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/internal/resume"
)

// cliamp search and cliamp search-sc play the first match of the query. Any
// other arguments pass through as they are.
func TestSearchArgs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    []string
		want    []string
		wantErr string
	}{
		{name: "no arguments"},
		{name: "files", args: []string{"a.mp3", "search"}, want: []string{"a.mp3", "search"}},
		{name: "youtube", args: []string{"search", "never", "gonna"}, want: []string{"ytsearch1:never gonna"}},
		{name: "soundcloud", args: []string{"search-sc", "lofi beats"}, want: []string{"scsearch1:lofi beats"}},
		{name: "no query", args: []string{"search"}, wantErr: "search requires a query string"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := searchArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("searchArgs(%q) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestCheckTerminalOutput(t *testing.T) {
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		readEnd.Close()
		writeEnd.Close()
	})

	if err := checkTerminalOutput(false, writeEnd); err == nil || !strings.Contains(err.Error(), "stdout is not a terminal") {
		t.Fatalf("TUI with pipe output error = %v, want stdout is not a terminal", err)
	}
	if err := checkTerminalOutput(true, writeEnd); err != nil {
		t.Fatalf("headless with pipe output error = %v, want nil", err)
	}
}

// At exit, a track with a position saves it with the playlist name, and a
// track with no position saves nothing.
func TestSaveExitResume(t *testing.T) {
	stream := "https://music.youtube.com/watch?v=abc"
	for _, tt := range []struct {
		name string
		path string
		secs int
		want resume.State
	}{
		{name: "stream", path: stream, secs: 95, want: resume.State{Path: stream, PositionSec: 95, Playlist: "Mix"}},
		{name: "local file", path: "/music/two.mp3", secs: 95, want: resume.State{Path: "/music/two.mp3", PositionSec: 95, Playlist: "Mix"}},
		{name: "no position", path: stream, secs: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CLIAMP_CONFIG_DIR", t.TempDir())
			saveExitResume(tt.path, tt.secs, "Mix")
			if got := resume.Load(); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("saved state = %+v, want %+v", got, tt.want)
			}
		})
	}
}

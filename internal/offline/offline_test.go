package offline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xaviiromf/cliamp-x/playlist"
)

func TestDestination(t *testing.T) {
	folder, base := Destination("/m", Job{Track: playlist.Track{Title: "Song: Part 1?", Artist: "AC/DC", Album: "Back in Black"}, TrackNo: 3})
	if folder != "/m/AC_DC/Back in Black" || base != "03 - Song_ Part 1_" {
		t.Fatalf("Destination = %q, %q", folder, base)
	}
	folder, base = Destination("/m", Job{Track: playlist.Track{Title: "Remix", Artist: ""}})
	if folder != "/m/Unknown artist/Singles" || base != "Remix" {
		t.Fatalf("Destination without album = %q, %q", folder, base)
	}
	if _, base = Destination("/m", Job{Track: playlist.Track{Title: "..", Artist: "A"}}); base != "_" {
		t.Fatalf("a dot-only title must not escape its folder, got %q", base)
	}
}

func TestManifest(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "song.m4a")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "offline.json")
	m := OpenManifest(path)
	if err := m.Add("https://music.youtube.com/watch?v=dQw4w9WgXcQ", file); err != nil {
		t.Fatal(err)
	}
	r := OpenManifest(path)
	if got, ok := r.LocalFile("https://www.youtube.com/watch?v=dQw4w9WgXcQ"); !ok || got != file {
		t.Fatalf("LocalFile = %q, %v", got, ok)
	}
	os.Remove(file)
	if r.Has("https://www.youtube.com/watch?v=dQw4w9WgXcQ") {
		t.Fatal("a deleted file must not count as downloaded")
	}
	if r.Has("/music/local.flac") {
		t.Fatal("a local path has no download")
	}
}

func TestLargeCover(t *testing.T) {
	for in, want := range map[string]string{
		"https://lh3.googleusercontent.com/x=w400-h400-l90-rj": "https://lh3.googleusercontent.com/x=w1200-h1200-l90-rj",
		"https://i.ytimg.com/vi/abc/mqdefault.jpg":             "https://i.ytimg.com/vi/abc/hqdefault.jpg",
		"https://example.com/a.jpg":                            "https://example.com/a.jpg",
	} {
		if got := largeCover(in); got != want {
			t.Errorf("largeCover(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProgressLine(t *testing.T) {
	m := progressLine.FindStringSubmatch("CLIAMP-PROGRESS  42.7%")
	if m == nil || m[1] != "42.7" {
		t.Fatalf("progress line not parsed: %v", m)
	}
}

package locallib

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// writeFiles creates placeholder audio files. They hold no tags, so the
// library falls back to "Artist - Title" file names and folder albums.
func writeFiles(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("not audio"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLibraryGroupsAndSearches(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeFiles(t, root,
		"Signals/Rush - Subdivisions.mp3",
		"Signals/Rush - The Analog Kid.flac",
		"Moving Pictures/Rush - Tom Sawyer.ogg",
		"Loose/Soda Stereo - De Música Ligera.mp3",
		"Loose/notes.txt",
		".hidden/Rush - Secret.mp3",
	)
	lib := New(root)
	if got := len(lib.Tracks()); got != 4 {
		t.Fatalf("tracks = %d, want 4 (no text files, no hidden folders)", got)
	}
	albums := lib.Albums()
	if len(albums) != 3 {
		t.Fatalf("albums = %+v, want 3", albums)
	}
	var signals Album
	for _, a := range albums {
		if a.Name == "Signals" {
			signals = a
		}
	}
	got, ok := lib.Album(signals.Key)
	if !ok || len(got.Tracks) != 2 || got.Artist != "Rush" {
		t.Fatalf("album = %+v ok=%v", got, ok)
	}
	if artists := lib.Artists(); artists["Rush"] != 3 || artists["Soda Stereo"] != 1 {
		t.Fatalf("artists = %v", artists)
	}
	if got := lib.ArtistAlbums("rush"); len(got) != 2 {
		t.Fatalf("ArtistAlbums(rush) = %+v", got)
	}
	if got := lib.Search("rush sawyer", 0); len(got) != 1 || got[0].Title != "Tom Sawyer" {
		t.Fatalf("Search = %+v", got)
	}
	if got := lib.Search("música", 0); len(got) != 1 {
		t.Fatalf("Search(música) = %+v", got)
	}
}

func TestLibraryIndexIsReusedAndRefreshed(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := t.TempDir()
	writeFiles(t, root, "A/X - One.mp3")
	lib := New(root)
	if len(lib.Tracks()) != 1 {
		t.Fatal("first scan")
	}
	if _, err := os.Stat(lib.indexPath); err != nil {
		t.Fatalf("index not saved: %v", err)
	}

	// The scan stays fresh until a refresh.
	writeFiles(t, root, "A/X - Two.mp3")
	if got := len(lib.Tracks()); got != 1 {
		t.Fatalf("before refresh = %d, want 1", got)
	}
	lib.Refresh()
	if got := len(lib.Tracks()); got != 2 {
		t.Fatalf("after refresh = %d, want 2", got)
	}
	// A new instance starts from the saved index and checks the folder.
	again := New(root)
	if got := len(again.Tracks()); got != 2 {
		t.Fatalf("second instance = %d, want 2", got)
	}
}

func TestNewAndNilLibrary(t *testing.T) {
	if New("off") != nil || New(" OFF ") != nil {
		t.Fatal("an off folder must give no library")
	}
	if lib := New(""); lib == nil || len(lib.Dirs()) != 0 {
		t.Fatal("no folder must give an empty library that can take folders")
	}
	var lib *Library
	if len(lib.Tracks()) != 0 || len(lib.Albums()) != 0 || len(lib.Search("x", 1)) != 0 || lib.Dirs() != nil {
		t.Fatal("a nil library must be empty")
	}
	lib.Refresh()
	lib.SetDirs([]string{"/x"})
}

func TestCleanDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"none", nil, []string{}},
		{"empty and spaces dropped", []string{"", "  "}, []string{}},
		{"cleaned", []string{"/a/b/", "/a/./c"}, []string{"/a/b", "/a/c"}},
		{"repeats dropped", []string{"/a", "/a/", " /a"}, []string{"/a"}},
		{"home expanded", []string{"~/Music"}, []string{filepath.Join(home, "Music")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CleanDirs(tt.in); !slices.Equal(got, tt.want) {
				t.Fatalf("CleanDirs(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSetDirsScansEveryFolderOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	a, b := t.TempDir(), t.TempDir()
	writeFiles(t, a, "A/X - One.mp3", "Sub/X - Nested.mp3")
	writeFiles(t, b, "B/Y - Two.mp3", "B/Y - Three.mp3")
	tests := []struct {
		name string
		dirs []string
		want int
	}{
		{"one folder", []string{a}, 2},
		{"two folders", []string{a, b}, 4},
		{"a nested folder counts its songs once", []string{a, filepath.Join(a, "Sub"), b}, 4},
		{"a removed folder drops its songs", []string{b}, 2},
		{"no folder", nil, 0},
		{"a missing folder is skipped", []string{filepath.Join(a, "gone"), b}, 2},
	}
	lib := New(a)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib.SetDirs(tt.dirs)
			if got := len(lib.Tracks()); got != tt.want {
				t.Fatalf("tracks = %d, want %d", got, tt.want)
			}
		})
	}
}

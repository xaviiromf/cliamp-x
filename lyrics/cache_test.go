package lyrics

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLookupCachesFoundLyrics(t *testing.T) {
	SetCacheDir(t.TempDir())
	t.Cleanup(func() { SetCacheDir("") })

	calls := 0
	found := func(context.Context) ([]Line, error) {
		calls++
		return []Line{{Start: 1500 * time.Millisecond, Text: "hola"}, {Start: 3 * time.Second, Text: "mundo"}}, nil
	}
	first, err := Lookup(context.Background(), "", "Artista", "Canción", found)
	if err != nil {
		t.Fatal(err)
	}
	// A cached track needs no source and no network: a failing source and
	// closed APIs prove the second answer comes from the disk.
	serveLyrics(t, lyricsAPI{}, true)
	failing := func(context.Context) ([]Line, error) { calls++; return nil, errors.New("offline") }
	second, err := Lookup(context.Background(), "", " artista ", "CANCIÓN", failing)
	if err != nil || calls != 1 {
		t.Fatalf("second lookup: err %v after %d source calls, want the cache", err, calls)
	}
	if len(second) != len(first) || second[0] != first[0] || second[1] != first[1] {
		t.Fatalf("cached lines %+v, want %+v", second, first)
	}
}

func TestLookupDoesNotCacheMisses(t *testing.T) {
	SetCacheDir(t.TempDir())
	t.Cleanup(func() { SetCacheDir("") })
	serveLyrics(t, lyricsAPI{}, false)

	if _, err := Lookup(context.Background(), "", "Nadie", "Nada"); err == nil {
		t.Fatal("want an error for a track with no lyrics")
	}
	if _, ok := readCache("Nadie", "Nada"); ok {
		t.Fatal("a miss must not be cached, or retrying could never find lyrics")
	}
}

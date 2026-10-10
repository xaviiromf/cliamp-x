package loudstore

import (
	"math"
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loudness.json")
	s := Open(path)
	s.StoreLoudness("https://music.youtube.com/watch?v=dQw4w9WgXcQ", -9.5)
	s.StoreLoudness("https://example.com/stream.mp3", -12)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	r := Open(path)
	// Another URL form of the same video shares the entry.
	if v, ok := r.Loudness("https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=3", false); !ok || v != -9.5 {
		t.Fatalf("video loudness = %v, %v; want -9.5", v, ok)
	}
	if v, ok := r.Loudness("https://example.com/stream.mp3", true); !ok || v != -12 {
		t.Fatalf("url loudness = %v, %v; want -12", v, ok)
	}
	if _, ok := r.Loudness("https://example.com/other.mp3", false); ok {
		t.Fatal("unknown track must not have a loudness")
	}
}

func TestStoreCapsEntries(t *testing.T) {
	s := Open("")
	for i := range maxEntries + 5 {
		s.StoreLoudness(filepath.Join("/x", string(rune('a'+i%26)), string(rune(i))), float64(-i%30))
	}
	if len(s.tracks) > maxEntries || len(s.order) != len(s.tracks) {
		t.Fatalf("entries = %d (order %d), want at most %d", len(s.tracks), len(s.order), maxEntries)
	}
}

func TestParseGain(t *testing.T) {
	for in, want := range map[string]float64{"-7.43 dB": -7.43, "+2.1 dB": 2.1, " -0.5dB ": -0.5, "3": 3} {
		if got, ok := parseGain(in); !ok || math.Abs(got-want) > 1e-9 {
			t.Errorf("parseGain(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "loud", "-99 dB"} {
		if _, ok := parseGain(in); ok {
			t.Errorf("parseGain(%q) accepted", in)
		}
	}
}

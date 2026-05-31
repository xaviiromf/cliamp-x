package qobuz

import "testing"

func TestParseYear(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"2021-05-14", 2021},
		{"1999", 1999},
		{"", 0},
		{"abc", 0},
		{"20", 0},
		{"19xy-01-01", 0},
	}
	for _, tt := range tests {
		if got := parseYear(tt.in); got != tt.want {
			t.Errorf("parseYear(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestTrackArtist(t *testing.T) {
	withPerformer := apiTrack{Performer: apiArtist{Name: "Performer"}}
	if got := trackArtist(withPerformer, nil); got != "Performer" {
		t.Errorf("performer name: got %q want %q", got, "Performer")
	}

	album := &apiAlbum{Artist: apiArtist{Name: "AlbumArtist"}}
	if got := trackArtist(apiTrack{}, album); got != "AlbumArtist" {
		t.Errorf("album fallback: got %q want %q", got, "AlbumArtist")
	}

	if got := trackArtist(apiTrack{}, nil); got != "" {
		t.Errorf("no artist: got %q want empty", got)
	}
}

func TestNewQualityNormalization(t *testing.T) {
	for _, q := range []int{5, 6, 7, 27} {
		if got := New(q).quality; got != q {
			t.Errorf("New(%d).quality = %d, want %d", q, got, q)
		}
	}
	for _, q := range []int{0, 1, 99} {
		if got := New(q).quality; got != 6 {
			t.Errorf("New(%d).quality = %d, want default 6", q, got)
		}
	}
}

package taste

import (
	"path/filepath"
	"testing"
)

func TestHidden(t *testing.T) {
	s := NewAt(filepath.Join(t.TempDir(), "taste.json"))
	tests := []struct {
		name  string
		ids   []string
		texts []string
		want  bool
	}{
		{"blocked genre word", nil, []string{"Reguetón 2024"}, true},
		{"word inside another word", nil, []string{"Trapped in the Closet"}, false},
		{"blocked artist in a list", nil, []string{"Bad Bunny, Jhayco"}, true},
		{"other music", nil, []string{"My Bloody Valentine", "Loveless"}, false},
		{"salsa as a whole word", nil, []string{"Salsa y tropical"}, true},
	}
	for _, tt := range tests {
		if got := s.Hidden(tt.ids, tt.texts...); got != tt.want {
			t.Errorf("%s: Hidden = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestRateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "taste.json")
	s := NewAt(path)
	if _, err := s.Rate("UC1", Item{Kind: Artist, Name: "Some Band"}, NotInterested); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rate("MPREb_1", Item{Kind: Album, Name: "Loveless"}, Interested); err != nil {
		t.Fatal(err)
	}
	s = NewAt(path)
	if !s.Hidden([]string{"UC1"}) || !s.Hidden(nil, "Some Band & Friends") {
		t.Fatal("hidden artist not hidden after reload")
	}
	if got := s.Liked(Album); len(got) != 1 || got[0] != "MPREb_1" {
		t.Fatalf("Liked(Album) = %v", got)
	}
	if got, _ := s.Rate("UC1", Item{}, Neutral); got != Neutral || s.Hidden([]string{"UC1"}, "Some Band") {
		t.Fatal("neutral rating kept the artist hidden")
	}
}

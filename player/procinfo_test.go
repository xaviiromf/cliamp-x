package player

import (
	"strings"
	"testing"
)

func TestStatPolicy(t *testing.T) {
	// 38 fields after the name, then policy (field 41) and more.
	tail := func(policy string) string {
		return strings.Repeat("0 ", 38) + policy + " 0 0 0"
	}
	tests := []struct {
		name, stat string
		want       int
		ok         bool
	}{
		{"other", "123 (AudioTrack) " + tail("0"), 0, true},
		{"fifo", "123 (AudioTrack) " + tail("1"), 1, true},
		{"name with spaces and parens", "123 (a (b) c) " + tail("2"), 2, true},
		{"short", "123 (AudioTrack) S 1 2", 0, false},
		{"no name", "garbage", 0, false},
		{"not a number", "123 (x) " + tail("x"), 0, false},
	}
	for _, tt := range tests {
		got, ok := statPolicy(tt.stat)
		if got != tt.want || ok != tt.ok {
			t.Errorf("%s: statPolicy = %d, %v, want %d, %v", tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

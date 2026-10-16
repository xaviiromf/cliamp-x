package player

import "testing"

func TestParseDeviceSampleRate(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"48000", 48000},
		{"44100", 44100},
		{"8000", 8000},
		{"192000", 192000},
		{"", 0},
		{"abc", 0},
		{"7999", 0},
		{"192001", 0},
		{"-48000", 0},
		{" 48000", 0},
	}
	for _, tt := range tests {
		if got := parseDeviceSampleRate(tt.in); got != tt.want {
			t.Errorf("parseDeviceSampleRate(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

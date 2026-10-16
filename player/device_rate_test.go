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

func TestLightMode(t *testing.T) {
	for in, want := range map[string]bool{"1": true, "": false, "0": false, "true": false, " 1": false} {
		if got := parseLightMode(in); got != want {
			t.Errorf("parseLightMode(%q) = %v, want %v", in, got, want)
		}
	}
	tests := []struct {
		ms    int
		light bool
		want  int
	}{
		{250, false, 250},
		{250, true, 1000},
		{50, true, 1000},
		{2000, true, 2000},
		{2000, false, 2000},
	}
	for _, tt := range tests {
		if got := BufferMsFor(tt.ms, tt.light); got != tt.want {
			t.Errorf("BufferMsFor(%d, %v) = %d, want %d", tt.ms, tt.light, got, tt.want)
		}
	}
}

package qobuz

import (
	"crypto/md5"
	"fmt"
	"testing"
)

func TestMD5Hex(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
		{"abc", "900150983cd24fb0d6963f7d28e17f72"},
	}
	for _, tt := range tests {
		if got := md5hex(tt.in); got != tt.want {
			t.Errorf("md5hex(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestTrackFileURLSignature verifies the request_sig is the md5 of the
// documented raw string layout for track/getFileUrl.
func TestTrackFileURLSignature(t *testing.T) {
	const (
		trackID  = "5966783"
		formatID = 6
		ts       = "1700000000"
		secret   = "deadbeefsecret"
	)
	raw := fmt.Sprintf("trackgetFileUrlformat_id%dintentstreamtrack_id%s%s%s",
		formatID, trackID, ts, secret)
	want := fmt.Sprintf("%x", md5.Sum([]byte(raw)))
	if got := md5hex(raw); got != want {
		t.Fatalf("signature mismatch: got %q want %q", got, want)
	}
}

func TestValidQualities(t *testing.T) {
	for _, q := range []int{5, 6, 7, 27} {
		if !validQualities[q] {
			t.Errorf("expected quality %d to be valid", q)
		}
	}
	for _, q := range []int{0, 1, 4, 8, 100} {
		if validQualities[q] {
			t.Errorf("expected quality %d to be invalid", q)
		}
	}
}

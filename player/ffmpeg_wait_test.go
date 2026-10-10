package player

import (
	"errors"
	"os/exec"
	"runtime"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("http: read on closed response body")
}

// TestFFmpegWaitIgnoresStdinErrorAfterCleanExit covers the race where the
// HTTP body feeding ffmpeg closes as ffmpeg ends: a clean exit is no error,
// while a failed process still reports one.
func TestFFmpegWaitIgnoresStdinErrorAfterCleanExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	for _, tt := range []struct {
		script  string
		wantErr bool
	}{
		{"exit 0", false},
		{"exit 3", true},
	} {
		cmd := exec.Command("sh", "-c", tt.script)
		cmd.Stdin = failingReader{}
		p := newFFmpegProcess(cmd)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		if err := p.wait(); (err != nil) != tt.wantErr {
			t.Fatalf("%s: wait() = %v, want error %v", tt.script, err, tt.wantErr)
		}
	}
}

package player

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

// A paused reader must not stop the download: every byte reaches the relay
// before ffmpeg reads one.
func TestMemRelayBuffersWhileReaderWaits(t *testing.T) {
	srcR, srcW, _ := os.Pipe()
	dstR, dstW, _ := os.Pipe()
	r := newMemRelay()
	r.run(srcR, dstW)

	payload := bytes.Repeat([]byte("audio"), 200_000) // 1 MB, far past a pipe buffer
	done := make(chan struct{})
	go func() {
		_, _ = srcW.Write(payload)
		srcW.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("download blocked on a reader that did not read")
	}
	if !r.resumable() {
		t.Fatal("relay not resumable after a full download")
	}
	got, err := io.ReadAll(dstR)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("read %d bytes, err %v; want %d", len(got), err, len(payload))
	}
}

func TestMemRelayCloseReleasesReader(t *testing.T) {
	r := newMemRelay()
	errc := make(chan error, 1)
	go func() {
		_, err := r.Read(make([]byte, 8))
		errc <- err
	}()
	time.Sleep(10 * time.Millisecond)
	r.Close()
	if err := <-errc; err != io.ErrClosedPipe {
		t.Fatalf("err = %v, want ErrClosedPipe", err)
	}
	if r.resumable() {
		t.Fatal("closed relay reported resumable")
	}
}

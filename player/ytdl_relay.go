package player

import (
	"io"
	"os"
	"sync"
)

// memRelayCap bounds the downloaded bytes that wait for ffmpeg. 256 MiB holds
// hours of YouTube audio; past it yt-dlp waits, as it did before the relay.
const memRelayCap = 256 << 20

// memRelay carries the yt-dlp download to ffmpeg through memory. yt-dlp
// writes as fast as the network allows while a paused ffmpeg reads nothing,
// so the download finishes during a pause and the unpause needs no
// reconnect: the connection never idles until YouTube drops it.
type memRelay struct {
	mu     sync.Mutex
	cond   *sync.Cond
	buf    []byte
	off    int // read position in buf
	eof    bool
	closed bool
}

func newMemRelay() *memRelay {
	r := &memRelay{}
	r.cond = sync.NewCond(&r.mu)
	return r
}

func (r *memRelay) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for len(r.buf)-r.off > memRelayCap && !r.closed {
		r.cond.Wait()
	}
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	r.buf = append(r.buf, p...)
	r.cond.Broadcast()
	return len(p), nil
}

func (r *memRelay) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for r.off == len(r.buf) && !r.eof && !r.closed {
		r.cond.Wait()
	}
	if r.closed {
		return 0, io.ErrClosedPipe
	}
	if r.off == len(r.buf) {
		return 0, io.EOF
	}
	n := copy(p, r.buf[r.off:])
	r.off += n
	// Drop what ffmpeg has read once it is most of the buffer, so a long
	// track does not keep its whole download in memory.
	if r.off > 1<<20 && r.off*2 > len(r.buf) {
		r.buf = append(r.buf[:0], r.buf[r.off:]...)
		r.off = 0
	}
	r.cond.Broadcast()
	return n, nil
}

// closeWrite marks the end of the download.
func (r *memRelay) closeWrite() {
	r.mu.Lock()
	r.eof = true
	r.cond.Broadcast()
	r.mu.Unlock()
}

// Close releases both sides; reads and writes fail from then on.
func (r *memRelay) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	r.closed = true
	r.cond.Broadcast()
	r.mu.Unlock()
	return nil
}

// resumable reports whether the download is not held back by the cap, so a
// long pause can resume from memory.
func (r *memRelay) resumable() bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.closed && (r.eof || len(r.buf)-r.off <= memRelayCap)
}

// run copies src (yt-dlp stdout) into the relay and the relay into dst
// (ffmpeg stdin), closing each end when its side finishes.
func (r *memRelay) run(src, dst *os.File) {
	go func() {
		_, _ = io.Copy(r, src)
		_ = src.Close()
		r.closeWrite()
	}()
	go func() {
		_, _ = io.Copy(dst, r)
		_ = dst.Close()
	}()
}

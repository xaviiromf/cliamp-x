package player

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gopxl/beep/v2"
)

// gaplessStreamer is a beep.Streamer that sequences tracks with zero-gap
// transitions. It sits at the bottom of the audio pipeline and manages
// track sources while the speed/EQ/tap/volume/ctrl chain above it lives forever.
//
// It always returns (len(samples), true) — it never stops the speaker.
// When no audio is available, it fills silence.
type gaplessStreamer struct {
	mu             sync.Mutex
	current        beep.Streamer // active track (decoded + resampled)
	next           beep.Streamer // preloaded next track
	currentVersion uint64        // increments whenever manual playback replaces current
	nextToken      uint64        // identifies the pipeline represented by next
	nextTokenSeq   uint64
	drained        atomic.Bool  // true when current exhausts with no next
	onSwap         func(uint64) // called on the audio goroutine, under the speaker lock, on a valid gapless transition

	// Crossfade. fadeFrames is the configured overlap. remaining reports the
	// frames left in the current track; it runs under the speaker lock. fade
	// and buf belong to the audio goroutine: only Stream touches them.
	fadeFrames atomic.Int64
	remaining  func() (int, bool)
	fade       fadeState
	buf        [][2]float64

	health *playbackHealth // times the reads of the current track
}

// fadeState tracks one crossfade. version ties it to the current track, so
// a manual replacement drops it. Over the overlap the outgoing track falls
// along a cosine and the incoming one rises along a sine, which keeps the
// summed power of two unrelated tracks constant.
type fadeState struct {
	version uint64
	active  bool // both tracks are mixing
	pos     int  // frames of the overlap played
	total   int  // frames in the overlap
	// incoming is true after the outgoing track ended before the overlap
	// did: the new current track keeps rising until pos reaches total.
	incoming bool
}

func fadeGains(pos, total int) (out, in float64) {
	t := 1.0
	if total > 0 {
		t = min(1, float64(pos)/float64(total))
	}
	return math.Cos(t * math.Pi / 2), math.Sin(t * math.Pi / 2)
}

// Stream reads samples from the current track. On exhaustion, it seamlessly
// fills remaining samples from the next track (like beep.Seq). If no next
// track is available, it sets drained=true and fills silence.
func (g *gaplessStreamer) Stream(samples [][2]float64) (int, bool) {
	g.mu.Lock()
	cur := g.current
	next := g.next
	currentVersion := g.currentVersion
	g.mu.Unlock()

	if g.fade.version != currentVersion {
		// A manual replacement ends any crossfade of the old track.
		g.fade = fadeState{version: currentVersion}
	}

	if cur == nil {
		// No active track — fill silence
		clear(samples)
		return len(samples), true
	}

	if next == nil {
		// The preload went away (a queue change): the outgoing track plays
		// on at full level.
		g.fade.active = false
	} else if !g.fade.active && !g.fade.incoming && g.remaining != nil {
		if frames := int(g.fadeFrames.Load()); frames > 0 {
			if rem, ok := g.remaining(); ok {
				if rem <= frames {
					g.fade = fadeState{version: currentVersion, active: true, total: rem}
				} else if pre := rem - frames; pre < len(samples) {
					// The overlap starts inside this buffer: play up to it,
					// then mix the rest.
					g.Stream(samples[:pre])
					g.Stream(samples[pre:])
					return len(samples), true
				}
			}
		}
	}
	if g.fade.active {
		return g.streamCrossfade(samples, cur, next, currentVersion)
	}

	start := time.Now()
	n, ok := cur.Stream(samples)
	g.health.sourceRead(time.Since(start), len(samples))
	if g.fade.incoming {
		g.applyFadeIn(samples[:n])
	}

	if !ok || n < len(samples) {
		// Current track exhausted — try to seamlessly continue with next
		g.mu.Lock()
		if currentVersion != g.currentVersion {
			// A manual replacement won while the old streamer was reading. It
			// owns the next callback; never let the stale read clobber it.
			g.mu.Unlock()
			clear(samples[n:])
			return len(samples), true
		}
		next := g.next
		nextToken := g.nextToken
		g.next = nil
		g.nextToken = 0
		g.current = next
		g.currentVersion++
		swapFn := g.onSwap
		g.mu.Unlock()

		if next != nil {
			// Fill remaining buffer from next track — zero gap
			if n < len(samples) {
				filled, _ := next.Stream(samples[n:])
				n += filled
			}
			// Publish the transition before the UI can observe the new stream as
			// drained. Resource cleanup is dispatched by the callback.
			if swapFn != nil {
				swapFn(nextToken)
			}
			g.drained.Store(false)
		} else {
			// No next track — we've drained
			g.drained.Store(true)
		}
	}

	// Fill any remaining with silence
	for i := n; i < len(samples); i++ {
		samples[i] = [2]float64{}
	}
	return len(samples), true
}

// streamCrossfade mixes the end of cur with the start of next. When the
// overlap is over, or cur runs out first, next becomes the current track.
func (g *gaplessStreamer) streamCrossfade(samples [][2]float64, cur, next beep.Streamer, version uint64) (int, bool) {
	n := len(samples)
	if cap(g.buf) < n {
		g.buf = make([][2]float64, n)
	}
	in := g.buf[:n]
	cn, cok := cur.Stream(samples)
	clear(samples[cn:])
	nn, _ := next.Stream(in)
	clear(in[nn:])
	for i := range n {
		gOut, gIn := fadeGains(g.fade.pos+i, g.fade.total)
		samples[i][0] = samples[i][0]*gOut + in[i][0]*gIn
		samples[i][1] = samples[i][1]*gOut + in[i][1]*gIn
	}
	g.fade.pos += n
	if cok && cn == n && g.fade.pos < g.fade.total {
		return n, true
	}

	// The outgoing track is done; whatever it has left was at zero gain.
	g.mu.Lock()
	if version != g.currentVersion || g.next != next {
		// A replacement or a new preload won the race; it owns the stream.
		g.mu.Unlock()
		g.fade = fadeState{}
		return n, true
	}
	nextToken := g.nextToken
	g.current = next
	g.next = nil
	g.nextToken = 0
	g.currentVersion++
	newVersion := g.currentVersion
	swapFn := g.onSwap
	g.mu.Unlock()
	g.fade = fadeState{version: newVersion, incoming: g.fade.pos < g.fade.total, pos: g.fade.pos, total: g.fade.total}
	if swapFn != nil {
		swapFn(nextToken)
	}
	g.drained.Store(false)
	return n, true
}

// applyFadeIn finishes the rise of a track that took over before its
// crossfade was over.
func (g *gaplessStreamer) applyFadeIn(samples [][2]float64) {
	for i := range samples {
		_, gIn := fadeGains(g.fade.pos+i, g.fade.total)
		samples[i][0] *= gIn
		samples[i][1] *= gIn
	}
	g.fade.pos += len(samples)
	if g.fade.pos >= g.fade.total {
		g.fade.incoming = false
	}
}

// Err always returns nil — errors are handled per-track at decode time.
func (g *gaplessStreamer) Err() error { return nil }

// SetNext preloads the next track's resampled streamer for gapless transition.
func (g *gaplessStreamer) SetNext(s beep.Streamer) uint64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.next = s
	if s == nil {
		g.nextToken = 0
		return 0
	}
	g.nextTokenSeq++
	g.nextToken = g.nextTokenSeq
	return g.nextToken
}

// Replace interrupts the current track and starts a new one immediately.
// Used for manual skip/prev/select operations.
func (g *gaplessStreamer) Replace(s beep.Streamer) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.current = s
	g.next = nil
	g.currentVersion++
	g.nextToken = 0
	g.drained.Store(false)
}

// Clear removes both current and next tracks. The streamer will output
// silence until Replace or SetNext is called.
func (g *gaplessStreamer) Clear() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.current = nil
	g.next = nil
	g.currentVersion++
	g.nextToken = 0
	g.drained.Store(false)
}

// Drained reports whether the current track ended with no next track queued.
func (g *gaplessStreamer) Drained() bool {
	return g.drained.Load()
}

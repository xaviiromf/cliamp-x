package player

import (
	"math"
	"testing"
)

// constStreamer plays a constant level for limit frames.
type constStreamer struct {
	level float64
	n     int
	limit int
}

func (c *constStreamer) Stream(samples [][2]float64) (int, bool) {
	k := 0
	for k < len(samples) && c.n < c.limit {
		samples[k] = [2]float64{c.level, c.level}
		k++
		c.n++
	}
	return k, k > 0
}

func (c *constStreamer) Err() error { return nil }

func TestGaplessCrossfade(t *testing.T) {
	cur := &constStreamer{level: 1, limit: 1000}
	next := &constStreamer{level: 0.5, limit: 10000}
	g := &gaplessStreamer{remaining: func() (int, bool) { return cur.limit - cur.n, true }}
	g.fadeFrames.Store(400)
	g.Replace(cur)
	g.SetNext(next)
	swaps := 0
	g.onSwap = func(uint64) { swaps++ }

	out := drain(t, g, 1400)
	// Before the overlap only the outgoing track plays.
	if out[500][0] != 1 {
		t.Fatalf("frame 500 = %v, want the outgoing track alone", out[500][0])
	}
	// Mid-overlap both are mixed at equal-power gains.
	gOut, gIn := fadeGains(200, 400)
	want := 1*gOut + 0.5*gIn
	if math.Abs(out[800][0]-want) > 0.01 {
		t.Fatalf("frame 800 = %.3f, want %.3f", out[800][0], want)
	}
	// After the overlap only the incoming track plays, at full level.
	if out[1300][0] != 0.5 {
		t.Fatalf("frame 1300 = %v, want the incoming track at full level", out[1300][0])
	}
	if swaps != 1 {
		t.Fatalf("swaps = %d, want 1", swaps)
	}
	// The incoming track was read from the start of the overlap.
	if next.n != 1400-600 {
		t.Fatalf("incoming frames read = %d, want %d", next.n, 800)
	}
}

func TestGaplessCrossfadeEarlyEnd(t *testing.T) {
	// The outgoing track ends 200 frames before its reported duration: the
	// incoming track takes over and keeps rising to full level.
	cur := &constStreamer{level: 1, limit: 800}
	next := &constStreamer{level: 1, limit: 10000}
	g := &gaplessStreamer{remaining: func() (int, bool) { return 1000 - cur.n, true }}
	g.fadeFrames.Store(400)
	g.Replace(cur)
	g.SetNext(next)
	out := drain(t, g, 1400)
	_, gIn := fadeGains(250, 400)
	if math.Abs(out[850][0]-gIn) > 0.01 {
		t.Fatalf("frame 850 = %.3f, want the incoming track rising (%.3f)", out[850][0], gIn)
	}
	if out[1300][0] != 1 {
		t.Fatalf("frame 1300 = %v, want full level", out[1300][0])
	}
}

func TestGaplessNoCrossfadeWithoutNext(t *testing.T) {
	cur := &constStreamer{level: 1, limit: 1000}
	g := &gaplessStreamer{remaining: func() (int, bool) { return cur.limit - cur.n, true }}
	g.fadeFrames.Store(400)
	g.Replace(cur)
	out := drain(t, g, 900)
	if out[899][0] != 1 {
		t.Fatalf("frame 899 = %v, want no fade without a next track", out[899][0])
	}
}

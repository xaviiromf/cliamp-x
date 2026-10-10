package player

import (
	"math"
	"sync/atomic"
	"testing"
)

func TestSoftClip(t *testing.T) {
	for _, x := range []float64{0, 0.5, -0.9, softClipKnee, -softClipKnee} {
		if got := softClip(x); got != x {
			t.Errorf("softClip(%v) = %v, want it unchanged under the knee", x, got)
		}
	}
	prev := softClipKnee
	for _, x := range []float64{0.96, 1.0, 1.5, 3, 100} {
		got := softClip(x)
		if got < prev || got > 1 || got <= softClipKnee {
			t.Errorf("softClip(%v) = %v, want non-decreasing within (knee, 1]", x, got)
		}
		if neg := softClip(-x); neg != -got {
			t.Errorf("softClip(%v) = %v, want the mirror of %v", -x, neg, got)
		}
		prev = got
	}
}

func TestPreampStreamerAppliesGain(t *testing.T) {
	var db atomic.Uint64
	db.Store(math.Float64bits(-6))
	src := &fakeStreamer{count: 4, val: [2]float64{0.5, 0.5}}
	p := newPreampStreamer(src, &db)
	buf := make([][2]float64, 4)
	n, ok := p.Stream(buf)
	if n != 4 || !ok {
		t.Fatalf("Stream = %d %v", n, ok)
	}
	want := 0.5 * math.Pow(10, -6.0/20)
	if math.Abs(buf[0][0]-want) > 1e-9 || math.Abs(buf[3][1]-want) > 1e-9 {
		t.Fatalf("sample = %v, want %v", buf[0][0], want)
	}

	// A boost past full scale is rounded off below 1.
	db.Store(math.Float64bits(12))
	src.count, src.val = 2, [2]float64{0.9, 0.9}
	p.Stream(buf[:2])
	if buf[0][0] > 1 || buf[0][0] <= softClipKnee {
		t.Fatalf("boosted sample = %v, want soft-clipped into (knee, 1]", buf[0][0])
	}
}

func TestPlayerPreampClamp(t *testing.T) {
	p := &Player{}
	p.SetPreamp(-40)
	if p.Preamp() != -24 {
		t.Fatalf("Preamp = %v, want -24", p.Preamp())
	}
	p.SetPreamp(20)
	if p.Preamp() != 12 {
		t.Fatalf("Preamp = %v, want 12", p.Preamp())
	}
}

package player

import (
	"math"
	"sync/atomic"
	"testing"
)

func TestBiquadPassthroughAtZeroDB(t *testing.T) {
	// At 0 dB (or within ±0.1 dB), the biquad should pass through unchanged
	src := &fakeStreamer{val: [2]float64{0.7, -0.3}, count: 64}
	var gain atomic.Uint64
	gain.Store(math.Float64bits(0.0))

	b := newBiquad(src, 1000, 0.707, &gain, 44100)

	samples := make([][2]float64, 64)
	n, _ := b.Stream(samples)

	for i := range n {
		if math.Abs(samples[i][0]-0.7) > 1e-9 {
			t.Errorf("at 0dB: samples[%d][0] = %f, want 0.7", i, samples[i][0])
		}
		if math.Abs(samples[i][1]-(-0.3)) > 1e-9 {
			t.Errorf("at 0dB: samples[%d][1] = %f, want -0.3", i, samples[i][1])
		}
	}
}

func TestBiquadNonZeroGainModifiesSamples(t *testing.T) {
	// Use a sine wave at the filter's center frequency (1000 Hz).
	// A peaking EQ only boosts energy near its center frequency, so a
	// constant (DC) input would pass through unchanged.
	const sr = 44100
	const freq = 1000.0
	const nSamples = 512
	src := &sineStreamerEQ{freq: freq, sr: sr, count: nSamples}
	var gain atomic.Uint64
	gain.Store(math.Float64bits(12.0)) // +12 dB boost

	b := newBiquad(src, freq, 0.707, &gain, sr)

	samples := make([][2]float64, nSamples)
	n, _ := b.Stream(samples)

	// Compare peak amplitude in later samples (after transient settles)
	// against the original sine amplitude of 1.0.
	maxAmp := 0.0
	for i := 256; i < n; i++ {
		if a := math.Abs(samples[i][0]); a > maxAmp {
			maxAmp = a
		}
	}
	// +12 dB should boost amplitude by ~4x; even accounting for filter shape
	// the peak should exceed the original 1.0 amplitude.
	if maxAmp <= 1.05 {
		t.Errorf("biquad at +12dB: max amplitude = %f, expected > 1.05", maxAmp)
	}
}

// sineStreamerEQ generates a sine wave for EQ testing.
type sineStreamerEQ struct {
	freq  float64
	sr    float64
	pos   int
	count int
}

func (s *sineStreamerEQ) Stream(samples [][2]float64) (int, bool) {
	n := min(len(samples), s.count-s.pos)
	if n <= 0 {
		return 0, false
	}
	for i := range n {
		val := math.Sin(2 * math.Pi * s.freq * float64(s.pos+i) / s.sr)
		samples[i] = [2]float64{val, val}
	}
	s.pos += n
	return n, true
}

func (s *sineStreamerEQ) Err() error { return nil }

func TestBiquadCoeffCaching(t *testing.T) {
	var gain atomic.Uint64
	gain.Store(math.Float64bits(3.0))

	b := newBiquad(&fakeStreamer{count: 0}, 1000, 0.707, &gain, 44100)

	b.calcCoeffs(3.0)
	b0First := b.b0
	if !b.inited {
		t.Fatal("inited should be true after calcCoeffs")
	}

	// Same gain should not recompute
	b.calcCoeffs(3.0)
	if b.b0 != b0First {
		t.Error("coefficients should be cached for same gain")
	}

	// Different gain should recompute
	b.calcCoeffs(6.0)
	if b.b0 == b0First {
		t.Error("coefficients should be recomputed for different gain")
	}
	if b.lastGain != 6.0 {
		t.Errorf("lastGain = %f, want 6.0", b.lastGain)
	}
}

func TestBiquadErr(t *testing.T) {
	src := &fakeStreamer{}
	var gain atomic.Uint64

	b := newBiquad(src, 1000, 0.707, &gain, 44100)

	if err := b.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
}

func TestEqFreqs(t *testing.T) {
	// Verify the 10-band EQ center frequencies are in ascending order
	for i := 1; i < len(eqFreqs); i++ {
		if eqFreqs[i] <= eqFreqs[i-1] {
			t.Errorf("eqFreqs[%d] (%f) <= eqFreqs[%d] (%f)", i, eqFreqs[i], i-1, eqFreqs[i-1])
		}
	}

	// Should have exactly 10 bands
	if len(eqFreqs) != 10 {
		t.Errorf("len(eqFreqs) = %d, want 10", len(eqFreqs))
	}
}

package player

import (
	"math"
	"sync"
	"testing"
	"time"
)

// toneStreamer is an endless stereo sine.
type toneStreamer struct {
	freq, amp, rate float64
	n               int
	limit           int // frames before it ends; 0 is endless
}

func (s *toneStreamer) Stream(samples [][2]float64) (int, bool) {
	for i := range samples {
		if s.limit > 0 && s.n >= s.limit {
			return i, i > 0
		}
		v := s.amp * math.Sin(2*math.Pi*s.freq*float64(s.n)/s.rate)
		samples[i] = [2]float64{v, v}
		s.n++
	}
	return len(samples), true
}

func (s *toneStreamer) Err() error { return nil }

func drain(t *testing.T, s interface {
	Stream([][2]float64) (int, bool)
}, frames int) [][2]float64 {
	t.Helper()
	out := make([][2]float64, 0, frames)
	buf := make([][2]float64, 512)
	for len(out) < frames {
		n, ok := s.Stream(buf[:min(512, frames-len(out))])
		out = append(out, buf[:n]...)
		if !ok {
			break
		}
	}
	return out
}

func TestLoudnessMeterSine(t *testing.T) {
	// A 997 Hz sine at -20 dBFS in both channels measures about -20 LUFS:
	// K-weighting adds about 0.7 dB at 1 kHz, the -0.691 offset removes it.
	const rate = 48000
	var m loudnessMeter
	m.init(rate)
	src := &toneStreamer{freq: 997, amp: 0.1, rate: rate}
	m.add(drain(t, src, 10*rate))
	lufs, ok := m.integrated()
	if !ok || math.Abs(lufs-(-20)) > 0.5 {
		t.Fatalf("integrated = %.2f LUFS (%v), want about -20", lufs, ok)
	}
	if got := m.gatedSeconds(); math.Abs(got-9.7) > 0.2 {
		t.Fatalf("gated seconds = %.1f, want about 9.7", got)
	}
}

func TestLoudnessMeterGatesSilence(t *testing.T) {
	const rate = 44100
	var m loudnessMeter
	m.init(rate)
	m.add(make([][2]float64, 5*rate))
	if _, ok := m.integrated(); ok {
		t.Fatal("silence must not produce a loudness")
	}
	// Silence around a tone does not drag its loudness down.
	m.add(drain(t, &toneStreamer{freq: 997, amp: 0.1, rate: rate}, 5*rate))
	m.add(make([][2]float64, 5*rate))
	if lufs, _ := m.integrated(); math.Abs(lufs-(-20)) > 0.6 {
		t.Fatalf("integrated with silence = %.2f, want about -20", lufs)
	}
}

type memLoudness struct {
	mu     sync.Mutex
	known  map[string]float64
	stored map[string]float64
	done   chan struct{}
}

func (m *memLoudness) Loudness(path string, _ bool) (float64, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.known[path]
	return v, ok
}

func (m *memLoudness) StoreLoudness(path string, lufs float64) {
	m.mu.Lock()
	m.stored[path] = lufs
	m.mu.Unlock()
	close(m.done)
}

func rms(frames [][2]float64) float64 {
	var sum float64
	for _, f := range frames {
		sum += f[0] * f[0]
	}
	return math.Sqrt(sum / float64(len(frames)))
}

func TestLevelStreamerKnownLoudness(t *testing.T) {
	// A track known to be at -8 LUFS plays 6 dB lower from the first frame.
	cfg := &normFields{}
	cfg.normMode.Store(int32(NormalizeTrack))
	cfg.normTarget.Store(math.Float64bits(-14))
	store := &memLoudness{known: map[string]float64{"a": -8}, stored: map[string]float64{}, done: make(chan struct{})}
	src := &toneStreamer{freq: 997, amp: 0.4, rate: 44100}
	l := newLevelStreamer(src, 44100, cfg, "a", store)
	out := drain(t, l, 4410)
	want := 0.4 / math.Sqrt2 * math.Pow(10, -6.0/20)
	if got := rms(out); math.Abs(got-want) > want*0.02 {
		t.Fatalf("rms = %.4f, want %.4f (-6 dB)", got, want)
	}
}

func TestLevelStreamerMeasuresAndStores(t *testing.T) {
	// A -20 LUFS tone with no known loudness ends up about 6 dB louder, and
	// its measurement is saved when it ends.
	const rate = 44100
	cfg := &normFields{}
	cfg.normMode.Store(int32(NormalizeTrack))
	cfg.normTarget.Store(math.Float64bits(-14))
	store := &memLoudness{known: map[string]float64{}, stored: map[string]float64{}, done: make(chan struct{})}
	src := &toneStreamer{freq: 997, amp: 0.1, rate: rate, limit: 30 * rate}
	l := newLevelStreamer(src, rate, cfg, "b", store)
	out := drain(t, l, 31*rate)
	tail := out[len(out)-rate:]
	want := 0.1 / math.Sqrt2 * math.Pow(10, 6.0/20)
	if got := rms(tail); math.Abs(got-want) > want*0.06 {
		t.Fatalf("settled rms = %.4f, want about %.4f (+6 dB)", got, want)
	}
	select {
	case <-store.done:
	case <-time.After(2 * time.Second):
		t.Fatal("the measurement was not stored when the track ended")
	}
	if got := store.stored["b"]; math.Abs(got-(-20)) > 0.5 {
		t.Fatalf("stored %.2f LUFS, want about -20", got)
	}
}

func TestLevelStreamerOff(t *testing.T) {
	cfg := &normFields{}
	cfg.normTarget.Store(math.Float64bits(-14))
	store := &memLoudness{known: map[string]float64{"a": -8}, stored: map[string]float64{}, done: make(chan struct{})}
	src := &toneStreamer{freq: 997, amp: 0.4, rate: 44100}
	l := newLevelStreamer(src, 44100, cfg, "a", store)
	if got, want := rms(drain(t, l, 4410)), 0.4/math.Sqrt2; math.Abs(got-want) > want*0.01 {
		t.Fatalf("normalization off changed the level: rms %.4f, want %.4f", got, want)
	}
}

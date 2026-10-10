package player

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/gopxl/beep/v2"
)

// Loudness normalization plays every track at about the same perceived
// level. Each track's stream gets its own gain stage below the gapless
// mixer, so a crossfade mixes two tracks that are already level.
//
// The gain comes from the track's integrated loudness (ITU-R BS.1770, in
// LUFS). A LoudnessStore can supply it before the track starts: from
// ReplayGain tags, or from an earlier play of the same track. Otherwise the
// stage measures the track as it plays and moves the gain towards the
// target as the measurement settles; the integrated value changes less and
// less as the track goes on, so the gain converges to a constant instead of
// pumping like a compressor. A finished measurement goes back to the store.

// NormalizeMode selects what loudness a track is leveled by.
type NormalizeMode int32

const (
	NormalizeOff   NormalizeMode = iota // no gain
	NormalizeTrack                      // each track to the target
	NormalizeAlbum                      // whole albums to the target, keeping their internal balance
)

const (
	// DefaultNormalizeTarget matches the level YouTube and most streaming
	// services play at.
	DefaultNormalizeTarget = -14.0
	// normalizeMaxCut and normalizeMaxBoost bound the gain. Boosts are
	// smaller: the soft clipper catches peaks, but a quiet master pushed far
	// up would still clip often.
	normalizeMaxCut   = -15.0
	normalizeMaxBoost = 6.0
	// normalizeSettle is how much gated audio the measurement needs before
	// it moves the gain.
	normalizeSettle = 3.0 // seconds
	// normalizeRampDBPerSec limits how fast a measured gain changes.
	normalizeRampDBPerSec = 2.0
	// normalizeStoreMin is the gated audio a measurement needs before it is
	// saved for the next play.
	normalizeStoreMin = 20.0 // seconds
)

// LoudnessStore remembers integrated loudness per track path. album asks
// for the album's loudness where the store knows it, and falls back to the
// track's.
type LoudnessStore interface {
	Loudness(path string, album bool) (lufs float64, ok bool)
	StoreLoudness(path string, lufs float64)
}

// normFields groups the Player's normalization state.
type normFields struct {
	normMode   atomic.Int32
	normTarget atomic.Uint64 // LUFS as Float64bits
	loudStore  LoudnessStore // guarded by Player.mu
}

// SetNormalize sets the normalization mode and target loudness in LUFS. It
// applies to the playing tracks at once, with a short ramp.
func (p *Player) SetNormalize(mode NormalizeMode, targetLUFS float64) {
	if math.IsNaN(targetLUFS) || targetLUFS == 0 {
		targetLUFS = DefaultNormalizeTarget
	}
	p.normTarget.Store(math.Float64bits(max(-30, min(-5, targetLUFS))))
	p.normMode.Store(int32(mode))
}

// Normalize returns the normalization mode and target.
func (p *Player) Normalize() (NormalizeMode, float64) {
	return NormalizeMode(p.normMode.Load()), math.Float64frombits(p.normTarget.Load())
}

// SetLoudnessStore sets where known loudness values come from and where
// measurements go. Call it before playback starts.
func (p *Player) SetLoudnessStore(s LoudnessStore) {
	p.mu.Lock()
	p.loudStore = s
	p.mu.Unlock()
}

// levelPipeline puts tp's stream behind a gain stage. inherit carries the
// measurement of the same track over a seek that rebuilt its pipeline.
func (p *Player) levelPipeline(tp *trackPipeline, inherit *levelStreamer) {
	if tp == nil || tp.stream == nil || tp.live {
		return
	}
	p.mu.Lock()
	store := p.loudStore
	p.mu.Unlock()
	l := newLevelStreamer(tp.stream, float64(p.sr), &p.normFields, tp.path, store)
	if inherit != nil {
		l.inherit(inherit)
	}
	tp.level = l
	tp.stream = l
}

// levelStreamer applies the normalization gain to one track and measures
// its loudness.
type levelStreamer struct {
	s     beep.Streamer
	cfg   *normFields
	path  string
	store LoudnessStore

	meter loudnessMeter

	// known is a loudness from the store for knownMode. While it is set the
	// measurement does not move the gain.
	known     float64
	knownMode NormalizeMode
	hasKnown  bool

	gainDB     float64 // applied gain
	sampleRate float64

	storeOnce sync.Once
	mu        sync.Mutex // guards meter and gainDB against finish from close()
}

func newLevelStreamer(s beep.Streamer, sampleRate float64, cfg *normFields, path string, store LoudnessStore) *levelStreamer {
	l := &levelStreamer{s: s, cfg: cfg, path: path, store: store, sampleRate: sampleRate}
	l.meter.init(sampleRate)
	mode, target := l.settings()
	l.lookup(mode)
	// Start at the known gain, so a remembered track never ramps.
	if mode != NormalizeOff && l.hasKnown {
		l.gainDB = clampGain(target - l.known)
	}
	return l
}

func (l *levelStreamer) settings() (NormalizeMode, float64) {
	return NormalizeMode(l.cfg.normMode.Load()), math.Float64frombits(l.cfg.normTarget.Load())
}

// lookup reads the store for mode.
func (l *levelStreamer) lookup(mode NormalizeMode) {
	l.knownMode, l.hasKnown = mode, false
	if l.store == nil || l.path == "" || mode == NormalizeOff {
		return
	}
	if v, ok := l.store.Loudness(l.path, mode == NormalizeAlbum); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
		l.known, l.hasKnown = v, true
	}
}

// inherit takes over the measurement and gain of the stage it replaces.
func (l *levelStreamer) inherit(old *levelStreamer) {
	old.mu.Lock()
	defer old.mu.Unlock()
	l.meter.blocks = append(l.meter.blocks[:0], old.meter.blocks...)
	l.meter.gated = old.meter.gated
	l.gainDB = old.gainDB
	l.known, l.knownMode, l.hasKnown = old.known, old.knownMode, old.hasKnown
}

func clampGain(db float64) float64 {
	return max(normalizeMaxCut, min(normalizeMaxBoost, db))
}

// targetGain returns the gain the stage is heading to.
func (l *levelStreamer) targetGain(mode NormalizeMode, target float64) float64 {
	if mode == NormalizeOff {
		return 0
	}
	if mode != l.knownMode {
		l.lookup(mode)
	}
	if l.hasKnown {
		return clampGain(target - l.known)
	}
	if l.meter.gatedSeconds() < normalizeSettle {
		return l.gainDB
	}
	lufs, ok := l.meter.integrated()
	if !ok {
		return l.gainDB
	}
	return clampGain(target - lufs)
}

func (l *levelStreamer) Stream(samples [][2]float64) (int, bool) {
	n, ok := l.s.Stream(samples)
	if n > 0 {
		l.mu.Lock()
		// Measure before the gain, so the gain never feeds back into it.
		l.meter.add(samples[:n])
		mode, target := l.settings()
		want := l.targetGain(mode, target)
		from, to := l.gainDB, want
		// A measured gain moves slowly. A known gain applies at once, and
		// a mode change ramps the same way.
		if !l.hasKnown || mode == NormalizeOff {
			step := normalizeRampDBPerSec * float64(n) / l.sampleRate
			to = from + max(-step, min(step, want-from))
		}
		l.gainDB = to
		l.mu.Unlock()
		if from != 0 || to != 0 {
			g0, g1 := math.Pow(10, from/20), math.Pow(10, to/20)
			for i := range n {
				g := g0 + (g1-g0)*float64(i+1)/float64(n)
				samples[i][0] *= g
				samples[i][1] *= g
			}
		}
	}
	if !ok || n < len(samples) {
		l.finish(normalizeStoreMin)
	}
	return n, ok
}

func (l *levelStreamer) Err() error { return l.s.Err() }

// finish saves the measurement when it covers at least minSeconds of gated
// audio. A remembered track is not measured again. It saves once per track.
func (l *levelStreamer) finish(minSeconds float64) {
	if l.store == nil || l.path == "" {
		return
	}
	l.mu.Lock()
	known := l.hasKnown && l.knownMode == NormalizeTrack
	enough := l.meter.gatedSeconds() >= minSeconds
	lufs, ok := l.meter.integrated()
	l.mu.Unlock()
	if known || !enough || !ok {
		return
	}
	l.storeOnce.Do(func() { go l.store.StoreLoudness(l.path, lufs) })
}

// loudnessMeter measures integrated loudness after ITU-R BS.1770-4: K-weighted
// mean square over 400 ms blocks that overlap by 75%, with an absolute gate at
// -70 LUFS and a relative gate 10 LU under the ungated level.
type loudnessMeter struct {
	filters [2][2]biquadState // [channel][stage]
	coefs   [2]biquadCoefs

	subFrames int       // frames in a 100 ms sub-block
	subCount  int       // frames in the current sub-block
	subSum    float64   // K-weighted energy of the current sub-block
	recent    []float64 // mean square of the last four sub-blocks
	blocks    []float64 // mean square of each 400 ms block

	gated      int     // blocks over the absolute gate
	cachedN    int     // len(blocks) when cachedLUFS was computed
	cachedLUFS float64 // integrated loudness of the first cachedN blocks
	cachedOK   bool
}

type biquadCoefs struct{ b0, b1, b2, a1, a2 float64 }
type biquadState struct{ x1, x2, y1, y2 float64 }

func (c *biquadCoefs) process(s *biquadState, x float64) float64 {
	y := c.b0*x + c.b1*s.x1 + c.b2*s.x2 - c.a1*s.y1 - c.a2*s.y2
	s.x2, s.x1 = s.x1, x
	s.y2, s.y1 = s.y1, y
	return y
}

// kWeighting returns the two K-weighting stages for a sample rate: a high
// shelf of +4 dB above about 1.5 kHz, then a high pass at about 38 Hz.
func kWeighting(rate float64) [2]biquadCoefs {
	shelf := func(fc, gainDB, q float64) biquadCoefs {
		a := math.Pow(10, gainDB/40)
		w := 2 * math.Pi * fc / rate
		cosw, alpha := math.Cos(w), math.Sin(w)/(2*q)
		sa := 2 * math.Sqrt(a) * alpha
		a0 := (a + 1) - (a-1)*cosw + sa
		return biquadCoefs{
			b0: a * ((a + 1) + (a-1)*cosw + sa) / a0,
			b1: -2 * a * ((a - 1) + (a+1)*cosw) / a0,
			b2: a * ((a + 1) + (a-1)*cosw - sa) / a0,
			a1: 2 * ((a - 1) - (a+1)*cosw) / a0,
			a2: ((a + 1) - (a-1)*cosw - sa) / a0,
		}
	}
	highPass := func(fc, q float64) biquadCoefs {
		w := 2 * math.Pi * fc / rate
		cosw, alpha := math.Cos(w), math.Sin(w)/(2*q)
		a0 := 1 + alpha
		return biquadCoefs{
			b0: (1 + cosw) / 2 / a0,
			b1: -(1 + cosw) / a0,
			b2: (1 + cosw) / 2 / a0,
			a1: -2 * cosw / a0,
			a2: (1 - alpha) / a0,
		}
	}
	return [2]biquadCoefs{shelf(1500, 4, 1/math.Sqrt2), highPass(38, 0.5)}
}

func (m *loudnessMeter) init(rate float64) {
	m.coefs = kWeighting(rate)
	m.subFrames = max(1, int(rate/10))
}

func (m *loudnessMeter) add(samples [][2]float64) {
	for _, s := range samples {
		var e float64
		for ch := range 2 {
			x := m.coefs[0].process(&m.filters[ch][0], s[ch])
			x = m.coefs[1].process(&m.filters[ch][1], x)
			e += x * x
		}
		m.subSum += e
		m.subCount++
		if m.subCount < m.subFrames {
			continue
		}
		m.recent = append(m.recent, m.subSum/float64(m.subFrames))
		m.subSum, m.subCount = 0, 0
		if len(m.recent) > 4 {
			m.recent = m.recent[1:]
		}
		if len(m.recent) == 4 {
			b := (m.recent[0] + m.recent[1] + m.recent[2] + m.recent[3]) / 4
			m.blocks = append(m.blocks, b)
			if blockLoudness(b) > -70 {
				m.gated++
			}
		}
	}
}

// blockLoudness converts a mean square to LUFS.
func blockLoudness(ms float64) float64 {
	if ms <= 0 {
		return math.Inf(-1)
	}
	return -0.691 + 10*math.Log10(ms)
}

// gatedSeconds is how much of the measured audio passes the absolute gate.
// Each block adds 100 ms of new audio.
func (m *loudnessMeter) gatedSeconds() float64 {
	return float64(m.gated) / 10
}

// integrated returns the gated integrated loudness, or false while no block
// passes the gates. It is recomputed only when a block was added.
func (m *loudnessMeter) integrated() (float64, bool) {
	if m.cachedN != len(m.blocks) {
		m.cachedLUFS, m.cachedOK = m.computeIntegrated()
		m.cachedN = len(m.blocks)
	}
	return m.cachedLUFS, m.cachedOK
}

func (m *loudnessMeter) computeIntegrated() (float64, bool) {
	var sum float64
	n := 0
	for _, b := range m.blocks {
		if blockLoudness(b) > -70 {
			sum += b
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	relative := blockLoudness(sum/float64(n)) - 10
	sum, n = 0, 0
	for _, b := range m.blocks {
		if l := blockLoudness(b); l > -70 && l > relative {
			sum += b
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return blockLoudness(sum / float64(n)), true
}

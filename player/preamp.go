package player

import (
	"math"
	"sync/atomic"

	"github.com/gopxl/beep/v2"
)

// softClipKnee is the level above which preampStreamer bends peaks down
// instead of letting them clip at full scale.
const softClipKnee = 0.95

// preampStreamer applies the EQ pre-amp gain and then a soft clipper, so EQ
// boosts that push a loud master past full scale round off instead of
// distorting. Samples under the knee pass through unchanged.
type preampStreamer struct {
	s        beep.Streamer
	db       *atomic.Uint64 // dB stored as Float64bits
	cachedDB float64
	gain     float64
}

func newPreampStreamer(s beep.Streamer, db *atomic.Uint64) *preampStreamer {
	return &preampStreamer{s: s, db: db, cachedDB: math.NaN()}
}

func (p *preampStreamer) Stream(samples [][2]float64) (int, bool) {
	n, ok := p.s.Stream(samples)
	if n == 0 {
		return 0, ok
	}
	if db := math.Float64frombits(p.db.Load()); db != p.cachedDB {
		p.gain = math.Pow(10, db/20)
		p.cachedDB = db
	}
	for i := range n {
		samples[i][0] = softClip(samples[i][0] * p.gain)
		samples[i][1] = softClip(samples[i][1] * p.gain)
	}
	return n, ok
}

func (p *preampStreamer) Err() error { return p.s.Err() }

// softClip maps |x| above the knee smoothly into (knee, 1), so the output
// never reaches full scale. Values under the knee are returned as is.
func softClip(x float64) float64 {
	a := math.Abs(x)
	if a <= softClipKnee {
		return x
	}
	headroom := 1 - softClipKnee
	return math.Copysign(softClipKnee+headroom*math.Tanh((a-softClipKnee)/headroom), x)
}

// SetPreamp sets the EQ pre-amp gain in dB, clamped to [-24, +12].
func (p *Player) SetPreamp(dB float64) {
	p.preamp.Store(math.Float64bits(max(min(dB, 12), -24)))
}

// Preamp returns the EQ pre-amp gain in dB.
func (p *Player) Preamp() float64 {
	return math.Float64frombits(p.preamp.Load())
}

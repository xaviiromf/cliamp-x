package player

import (
	"math"
	"sync"

	"github.com/gopxl/beep/v2"
)

// volumeStreamer applies dB gain and optional mono downmix to an audio stream.
type volumeStreamer struct {
	s          beep.Streamer
	vol        *float64
	mono       *bool
	mu         *sync.Mutex
	cachedDB   float64 // last dB value used to compute cachedGain; starts NaN to force first compute
	cachedGain float64 // precomputed linear gain = 10^(dB/20)
}

func (v *volumeStreamer) Stream(samples [][2]float64) (int, bool) {
	n, ok := v.s.Stream(samples)
	if n == 0 {
		return 0, ok
	}
	v.mu.Lock()
	db := *v.vol
	mono := *v.mono
	v.mu.Unlock()
	// Recompute gain only when volume changes (rare) instead of every Stream() call.
	if db != v.cachedDB {
		v.cachedGain = math.Pow(10, db/20)
		v.cachedDB = db
	}
	gain := v.cachedGain
	for i := range n {
		samples[i][0] *= gain
		samples[i][1] *= gain
		if mono {
			mid := (samples[i][0] + samples[i][1]) / 2
			samples[i][0] = mid
			samples[i][1] = mid
		}
	}
	return n, ok
}

func (v *volumeStreamer) Err() error { return v.s.Err() }

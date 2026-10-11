package player

import (
	"math"
	"time"
)

// The sound controls: volume and its floor, the sleep-timer fade, speed,
// mono, the EQ bands and the crossfade.

// SetVolumeMin sets the minimum volume floor in dB, clamped to [-90, 0].
// If the current volume is below the new floor it is immediately raised to match.
// A NaN floor is ignored.
func (p *Player) SetVolumeMin(db float64) {
	if math.IsNaN(db) {
		return
	}
	newMin := max(min(db, 0), -90)
	p.volMin.Store(math.Float64bits(newMin))
	for {
		cur := p.volume.Load()
		curDB := math.Float64frombits(cur)
		if curDB >= newMin {
			break
		}
		if p.volume.CompareAndSwap(cur, math.Float64bits(newMin)) {
			break
		}
	}
}

// VolumeMin returns the current minimum volume floor in dB.
func (p *Player) VolumeMin() float64 {
	return math.Float64frombits(p.volMin.Load())
}

// SetVolume sets the volume in dB, clamped to [VolumeMin, +6].
// A NaN volume is ignored.
func (p *Player) SetVolume(db float64) {
	if math.IsNaN(db) {
		return
	}
	p.volume.Store(math.Float64bits(max(min(db, 6), p.VolumeMin())))
}

// SetFade scales the output from 1 (full volume) to 0 (silent), for a
// fade-out such as the sleep timer's. Unlike SetVolume it is never saved, so
// quitting in the middle of a fade leaves the volume as it was.
func (p *Player) SetFade(level float64) {
	if math.IsNaN(level) {
		return
	}
	p.fadeCut.Store(math.Float64bits(1 - max(0, min(1, level))))
}

// Volume returns the current volume in dB.
func (p *Player) Volume() float64 {
	return math.Float64frombits(p.volume.Load())
}

// SetSpeed sets the playback speed ratio, clamped to [0.25, 2.0].
// 1.0 is normal speed, 2.0 is double speed, etc. A NaN ratio is ignored.
func (p *Player) SetSpeed(ratio float64) {
	if math.IsNaN(ratio) {
		return
	}
	p.speed.Store(math.Float64bits(max(min(ratio, 2.0), 0.25)))
}

// Speed returns the current playback speed ratio.
func (p *Player) Speed() float64 {
	return math.Float64frombits(p.speed.Load())
}

// ToggleMono switches between stereo and mono (L+R downmix) output.
func (p *Player) ToggleMono() {
	p.mono.Store(!p.mono.Load())
}

// Mono returns true if mono output is enabled.
func (p *Player) Mono() bool {
	return p.mono.Load()
}

// SetEQBand sets a single EQ band's gain in dB, clamped to [-12, +12].
// A NaN gain is ignored.
func (p *Player) SetEQBand(band int, dB float64) {
	if band < 0 || band >= 10 || math.IsNaN(dB) {
		return
	}
	p.eqBands[band].Store(math.Float64bits(max(min(dB, 12), -12)))
}

// EQBands returns a copy of all 10 EQ band gains.
func (p *Player) EQBands() [10]float64 {
	var bands [10]float64
	for i := range 10 {
		bands[i] = math.Float64frombits(p.eqBands[i].Load())
	}
	return bands
}

// SetCrossfade sets how long the end of a track overlaps the start of the
// preloaded next one; 0 plays them back to back. It applies to the next
// transition that has not started yet.
func (p *Player) SetCrossfade(d time.Duration) {
	p.gapless.fadeFrames.Store(int64(p.sr.N(max(0, d))))
}

// Crossfade returns the crossfade length.
func (p *Player) Crossfade() time.Duration {
	return p.sr.D(int(p.gapless.fadeFrames.Load()))
}

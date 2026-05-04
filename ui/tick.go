package ui

import "time"

// Tick intervals: fast for visualizer animation, slow for time/seek display.
const (
	TickAnim    = 16 * time.Millisecond  // ~60 FPS — smooth bar/wave/scope animation cadence
	TickWave    = TickAnim               // ~60 FPS — waveform modes (no FFT)
	TickFast    = 50 * time.Millisecond  // 20 FPS — per-frame-animated spectrum modes
	TickAnalyze = 33 * time.Millisecond  // ~30 Hz — FFT analysis cadence (independent of animation)
	TickSlow    = 200 * time.Millisecond // 5 FPS — visualizer off or overlay
)

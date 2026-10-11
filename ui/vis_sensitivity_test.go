package ui

import (
	"math"
	"testing"
)

func TestSensitivityRaisesBands(t *testing.T) {
	samples := make([]float64, 2048)
	for i := range samples {
		samples[i] = 0.01 * math.Sin(2*math.Pi*440*float64(i)/44100)
	}
	level := func(sens int) float64 {
		v := NewVisualizer(44100)
		v.SetSensitivity(sens)
		var sum float64
		for _, b := range v.Analyze(samples, spectrumAnalysisSpec(16)) {
			sum += b
		}
		return sum
	}
	if low, high := level(MinVisSensitivity), level(MaxVisSensitivity); high <= low {
		t.Fatalf("bands at sensitivity 10 (%.3f) must exceed sensitivity 1 (%.3f)", high, low)
	}
	v := NewVisualizer(44100)
	v.SetSensitivity(99)
	if v.Sensitivity() != MaxVisSensitivity {
		t.Fatalf("SetSensitivity(99) = %d, want %d", v.Sensitivity(), MaxVisSensitivity)
	}
}

// TestSensitivityLevelsMoveApart checks that every level up makes the bars
// follow the beat more: a pulsing tone must move the bars further from frame
// to frame at each level, and the default level must keep its old response.
func TestSensitivityLevelsMoveApart(t *testing.T) {
	spec := NormalizeAnalysisSpec(spectrumAnalysisSpec(16))
	frame := func(n int) []float64 {
		amp := 0.01
		if n%4 == 0 {
			amp = 0.05 // a beat every fourth frame
		}
		s := make([]float64, spec.FFTSize)
		for i := range s {
			x := float64(i) / 44100
			s[i] = amp * (math.Sin(2*math.Pi*110*x) + 0.5*math.Sin(2*math.Pi*2500*x))
		}
		return s
	}
	motion := func(level int) float64 {
		v := NewVisualizer(44100)
		v.SetSensitivity(level)
		var prev []float64
		var sum float64
		for n := range 40 {
			bands := v.Analyze(frame(n), spec)
			for i, b := range bands {
				if prev != nil {
					sum += math.Abs(b - prev[i])
				}
			}
			prev = append(prev[:0], bands...)
		}
		return sum
	}
	moves := map[int]float64{}
	for level := MinVisSensitivity; level <= MaxVisSensitivity; level++ {
		moves[level] = motion(level)
		if level > MinVisSensitivity && moves[level] <= moves[level-1] {
			t.Errorf("level %d moves %.2f, not more than level %d (%.2f)", level, moves[level], level-1, moves[level-1])
		}
	}
	def := moves[DefaultVisSensitivity]
	if moves[MaxVisSensitivity] < 1.5*def || moves[MinVisSensitivity] > 0.5*def {
		t.Errorf("levels 1, %d and 10 move %.2f, %.2f and %.2f: the range is too narrow",
			DefaultVisSensitivity, moves[MinVisSensitivity], def, moves[MaxVisSensitivity])
	}

	v := NewVisualizer(44100)
	if r := v.sensitivityResponse(); r != (visResponse{gainDB: 6, contrast: 1, attack: 0.8, decay: 0.4}) {
		t.Errorf("default response = %+v, want the original one", r)
	}
}

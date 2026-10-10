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

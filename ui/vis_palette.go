package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// VisPalette is a color set for the visualizer alone: the low, mid and high
// spectrum colors. It leaves the rest of the interface on the theme.
type VisPalette struct {
	Name   string
	Colors [3]string // hex, low to high
}

// VisPalettes are the built-in visualizer colors. "theme" follows the theme.
var VisPalettes = []VisPalette{
	{Name: "theme"},
	{Name: "fire", Colors: [3]string{"#ffd000", "#ff6a00", "#ff1744"}},
	{Name: "ocean", Colors: [3]string{"#00e5ff", "#008cff", "#5b3bff"}},
	{Name: "neon", Colors: [3]string{"#39ff14", "#00ffff", "#ff00ff"}},
	{Name: "aurora", Colors: [3]string{"#00ffa3", "#00b3ff", "#b300ff"}},
	{Name: "sunset", Colors: [3]string{"#ffd166", "#ef476f", "#7b2cbf"}},
	{Name: "ice", Colors: [3]string{"#e0fbfc", "#98c1d9", "#3d7ab8"}},
	{Name: "matrix", Colors: [3]string{"#006b1a", "#00c22e", "#7dff8a"}},
	{Name: "sakura", Colors: [3]string{"#ffd6e8", "#ff8fc0", "#ff2d87"}},
	{Name: "amber", Colors: [3]string{"#ffcc00", "#ff9500", "#ff5e00"}},
	{Name: "mono", Colors: [3]string{"#7a7a7a", "#bdbdbd", "#ffffff"}},
}

var (
	visPalette    string         // "" or "theme" follows the theme
	themeSpectrum [3]color.Color // the theme's own spectrum colors
)

// SetVisPalette colors the visualizer with a built-in palette name or three
// comma-separated hex colors ("#112233,#445566,#778899"). An unknown value
// follows the theme. It returns the name now in use.
func SetVisPalette(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if _, ok := visPaletteColors(name); !ok {
		name = "theme"
	}
	visPalette = name
	applyVisPalette()
	return name
}

// VisPaletteName returns the palette in use.
func VisPaletteName() string {
	if visPalette == "" {
		return "theme"
	}
	return visPalette
}

// NextVisPalette returns the built-in palette after the one in use.
func NextVisPalette() string {
	cur := VisPaletteName()
	for i, p := range VisPalettes {
		if p.Name == cur {
			return VisPalettes[(i+1)%len(VisPalettes)].Name
		}
	}
	return VisPalettes[0].Name
}

func visPaletteColors(name string) ([3]color.Color, bool) {
	var out [3]color.Color
	if name == "" || name == "theme" {
		return out, true
	}
	hexes := strings.Split(name, ",")
	if len(hexes) != 3 {
		hexes = nil
		for _, p := range VisPalettes {
			if p.Name == name && p.Name != "theme" {
				hexes = p.Colors[:]
			}
		}
	}
	if len(hexes) != 3 {
		return out, false
	}
	for i, h := range hexes {
		h = strings.TrimSpace(h)
		if len(h) != 7 || h[0] != '#' {
			return out, false
		}
		out[i] = lipgloss.Color(h)
	}
	return out, true
}

// applyVisPalette puts the palette over the theme's spectrum colors.
func applyVisPalette() {
	colors, ok := visPaletteColors(visPalette)
	if !ok || colors[0] == nil {
		colors = themeSpectrum
	}
	SpectrumLow, SpectrumMid, SpectrumHigh = colors[0], colors[1], colors[2]
	refreshSpecANSI()
	refreshRedSectorANSI()
	refreshYinYangANSI()
}

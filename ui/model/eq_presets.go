package model

import "strings"

const eqBandCount = 10

// EQPreset is a named 10-band EQ curve.
type EQPreset struct {
	Name  string
	Bands [eqBandCount]float64
}

// eqGenrePresets are the presets the EQ panel lists under "Genres"; the rest
// are utility curves for a listening situation.
var eqGenrePresets = map[string]bool{
	"Rock": true, "Pop": true, "Jazz": true, "Classical": true, "Electronic": true,
	"Acoustic": true, "Hip-Hop": true, "R&B": true, "Metal": true, "Reggaeton": true,
	"Cumbia": true, "Indie": true, "Lo-Fi": true, "Soundtrack": true, "Blues": true,
}

// eqPresets is the ordered list of built-in EQ presets.
// Bands: 70Hz, 180Hz, 320Hz, 600Hz, 1kHz, 3kHz, 6kHz, 12kHz, 14kHz, 16kHz
var eqPresets = []EQPreset{
	{"Flat", [eqBandCount]float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
	{"Rock", [eqBandCount]float64{5, 4, 2, -1, -2, 2, 4, 5, 5, 5}},
	{"Pop", [eqBandCount]float64{-1, 2, 4, 5, 4, 1, -1, -1, 1, 2}},
	{"Jazz", [eqBandCount]float64{3, 4, 2, 1, -1, -1, 1, 2, 3, 4}},
	{"Classical", [eqBandCount]float64{3, 2, 1, 0, -1, -1, 0, 2, 3, 4}},
	{"Bass Boost", [eqBandCount]float64{8, 6, 4, 2, 0, 0, 0, 0, 0, 0}},
	{"Treble Boost", [eqBandCount]float64{0, 0, 0, 0, 0, 1, 3, 5, 6, 7}},
	{"Vocal", [eqBandCount]float64{-2, -1, 1, 4, 5, 4, 2, 0, -1, -2}},
	{"Electronic", [eqBandCount]float64{6, 4, 1, -1, -2, 1, 3, 4, 5, 6}},
	{"Acoustic", [eqBandCount]float64{3, 3, 2, 0, 1, 2, 3, 3, 2, 1}},
	{"Hip-Hop", [eqBandCount]float64{7, 5, 3, 1, -1, -1, 1, 3, 3, 3}},
	{"R&B", [eqBandCount]float64{4, 6, 3, 1, -1, 1, 2, 2, 1, 0}},
	{"Loudness", [eqBandCount]float64{6, 4, 1, 0, -2, -1, 1, 4, 5, 5}},
	{"Late Night", [eqBandCount]float64{5, 3, 1, 0, -2, -1, 0, 2, 3, 3}},
	{"Podcast", [eqBandCount]float64{-3, -1, 2, 4, 4, 3, 1, -1, -2, -3}},
	{"Small Speakers", [eqBandCount]float64{7, 5, 4, 2, 1, 0, -1, 0, 1, 2}},
	{"Metal", [eqBandCount]float64{4, 3, 1, -2, -3, 0, 3, 5, 5, 4}},
	{"Reggaeton", [eqBandCount]float64{7, 6, 3, 0, -1, 1, 2, 3, 3, 2}},
	{"Cumbia", [eqBandCount]float64{4, 4, 2, 0, 0, 2, 3, 3, 2, 1}},
	{"Indie", [eqBandCount]float64{2, 2, 1, 0, 1, 2, 3, 3, 2, 2}},
	{"Lo-Fi", [eqBandCount]float64{3, 2, 1, 0, -1, -2, -3, -4, -5, -6}},
	{"Soundtrack", [eqBandCount]float64{4, 3, 1, 0, -1, 0, 2, 3, 4, 4}},
	{"Blues", [eqBandCount]float64{3, 3, 1, 0, 1, 2, 2, 1, 0, 0}},
}

// genreKeywords map words of a genre name, in English and Spanish, to the
// preset the auto EQ picks for it. The first match wins, so the more
// specific words come first.
var genreKeywords = []struct {
	words  []string
	preset string
}{
	{[]string{"metal", "hardcore", "thrash", "djent", "screamo"}, "Metal"},
	{[]string{"reggaet", "urbano latino", "dembow", "perreo", "latin trap", "trap latino"}, "Reggaeton"},
	{[]string{"cumbia", "salsa", "bachata", "merengue", "vallenato", "latin", "latina", "latino", "tropical"}, "Cumbia"},
	{[]string{"hip hop", "hip-hop", "hiphop", "rap", "trap", "drill", "grime"}, "Hip-Hop"},
	{[]string{"r&b", "rnb", "soul", "funk", "neo soul"}, "R&B"},
	{[]string{"lo-fi", "lofi", "chill", "ambient", "relax", "sleep", "dormir", "calma", "concentr"}, "Lo-Fi"},
	{[]string{"soundtrack", "banda sonora", "bandas sonoras", "ost", "score", "anime", "videojuego", "video game", "game"}, "Soundtrack"},
	{[]string{"electr", "edm", "house", "techno", "trance", "dubstep", "drum and bass", "dnb", "dance", "synth", "disco"}, "Electronic"},
	{[]string{"blues"}, "Blues"},
	{[]string{"jazz", "swing", "bossa", "bebop"}, "Jazz"},
	{[]string{"clásic", "clasic", "classical", "orquest", "orchestr", "ópera", "opera", "baroque", "barroc", "symphon", "sinfón"}, "Classical"},
	{[]string{"indie", "alternativ", "shoegaze", "post-rock"}, "Indie"},
	{[]string{"punk", "grunge", "rock"}, "Rock"},
	{[]string{"acoustic", "acúst", "acust", "folk", "country", "singer-songwriter", "cantautor"}, "Acoustic"},
	{[]string{"podcast", "talk", "spoken", "audiobook", "audiolibro", "hablado"}, "Podcast"},
	{[]string{"pop", "k-pop", "kpop", "j-pop", "jpop"}, "Pop"},
}

// presetForGenre returns the preset the auto EQ uses for a genre name, or
// false when no keyword matches.
func presetForGenre(genre string) (EQPreset, bool) {
	g := strings.ToLower(strings.TrimSpace(genre))
	if g == "" {
		return EQPreset{}, false
	}
	for _, k := range genreKeywords {
		for _, w := range k.words {
			if strings.Contains(g, w) {
				return EQPresetByName(k.preset)
			}
		}
	}
	return EQPreset{}, false
}

// EQPresetByName looks up a built-in preset by case-insensitive name.
func EQPresetByName(name string) (EQPreset, bool) {
	for _, p := range eqPresets {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return EQPreset{}, false
}

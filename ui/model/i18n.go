package model

import "strings"

// L translates an English interface text to the interface language. Texts
// with no entry, such as names and other dynamic text, come back unchanged.
func L(en string) string {
	if !spanishUI {
		return en
	}
	if es, ok := spanish[en]; ok {
		return es
	}
	return en
}

// SetLanguage picks the interface language: "es", "en", or "" to follow
// $LC_ALL, $LC_MESSAGES or $LANG.
func SetLanguage(lang string) {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "es", "spanish", "español":
		spanishUI = true
	case "en", "english":
		spanishUI = false
	}
}

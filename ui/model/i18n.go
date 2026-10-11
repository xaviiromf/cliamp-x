package model

import (
	"os"
	"strings"
)

// spanishUI reports whether the interface is in Spanish. It starts from the
// locale; SetLanguage overrides it from the config.
var spanishUI = func() bool {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(name); v != "" {
			return strings.HasPrefix(strings.ToLower(v), "es")
		}
	}
	return false
}()

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

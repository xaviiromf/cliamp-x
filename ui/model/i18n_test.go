package model

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var fmtVerb = regexp.MustCompile(`%[-+#0]*[0-9.]*[sdvqfxXtT%]`)

// statusLiteral matches a status call whose text is a string literal.
var statusLiteral = regexp.MustCompile(`status\.(?:Show|Showf|Warning|Warningf|Error|Errorf|Success|Successf|Activity|Activityf)\((?:statusTTL\w+,\s*)?"((?:[^"\\]|\\.)+)"`)

// TestSpanishCatalog checks that the interface has no untranslated key
// label, screen name or literal status message, and that every translation
// keeps the format verbs of its English text, in order.
func TestSpanishCatalog(t *testing.T) {
	need := map[string]string{}
	for _, c := range commandRegistry {
		if c.Label != "" {
			need[c.Label] = "key label"
		}
	}
	for s := screenMain + 1; s <= screenFullVisualizer; s++ {
		if name := s.labelEN(); name != "" {
			need[name] = "screen name"
		}
	}
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range statusLiteral.FindAllStringSubmatch(string(data), -1) {
			text := strings.ReplaceAll(m[1], `\"`, `"`)
			// A text of verbs and symbols only has nothing to translate.
			if strings.TrimFunc(fmtVerb.ReplaceAllString(text, ""), func(r rune) bool { return !isLetter(r) }) == "" {
				continue
			}
			need[text] = "status message in " + f
		}
	}
	var missing []string
	for text, where := range need {
		if _, ok := spanish[text]; !ok {
			missing = append(missing, where+": "+text)
		}
	}
	slices.Sort(missing)
	for _, m := range missing {
		t.Errorf("no Spanish text for %s", m)
	}
	for en, es := range spanish {
		if a, b := fmtVerb.FindAllString(en, -1), fmtVerb.FindAllString(es, -1); !slices.Equal(a, b) {
			t.Errorf("%q and %q have different format verbs: %v vs %v", en, es, a, b)
		}
	}
}

func isLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

func TestLanguageSwitch(t *testing.T) {
	t.Cleanup(func() { spanishUI = false })
	SetLanguage("es")
	if got := L("Settings"); got != "Ajustes" {
		t.Fatalf("L(Settings) in Spanish = %q", got)
	}
	if got := L("some dynamic name"); got != "some dynamic name" {
		t.Fatalf("an unknown text must come back unchanged, got %q", got)
	}
	var m Model
	m.status.Showf(statusTTLShort, "Added %d tracks", 3)
	if m.status.text != "Se agregaron 3 canciones" {
		t.Fatalf("status = %q", m.status.text)
	}
	SetLanguage("en")
	if got := L("Settings"); got != "Settings" {
		t.Fatalf("L(Settings) in English = %q", got)
	}
}

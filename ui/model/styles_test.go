package model

import (
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/xaviiromf/cliamp-x/theme"
)

var truecolorFG = regexp.MustCompile(`\x1b\[38;2;(\d+);(\d+);(\d+)m`)

// renderedFG returns the foreground color lipgloss actually emitted for a
// rendered string, as lowercase CSS hex, so a test can compare it with the
// theme value it came from instead of with a style object.
func renderedFG(t *testing.T, rendered string) string {
	t.Helper()
	match := truecolorFG.FindStringSubmatch(rendered)
	if match == nil {
		t.Fatalf("rendered %q carries no truecolor foreground", rendered)
	}
	rgb := make([]int, 3)
	for i := range rgb {
		value, err := strconv.Atoi(match[i+1])
		if err != nil {
			t.Fatalf("parse color component %d of %q: %v", i, rendered, err)
		}
		rgb[i] = value
	}
	return fmt.Sprintf("#%02x%02x%02x", rgb[0], rgb[1], rgb[2])
}

// TestFavoriteMarksFollowThemeChange covers the favorite markers shown in the
// status bar after a toggle. Switching themes goes through SetTheme, which is
// where the IPC `theme` op and the theme picker both land, so the markers have
// to come out in the new theme's colors without a restart.
func TestFavoriteMarksFollowThemeChange(t *testing.T) {
	before := theme.Theme{
		Name:     "marker-before",
		Accent:   "#88aacc",
		BrightFG: "#ffffff",
		FG:       "#445566",
		Green:    "#88cc88",
		Yellow:   "#ddcc77",
		Red:      "#ee1122",
	}
	after := theme.Theme{
		Name:     "marker-after",
		Accent:   "#aa88cc",
		BrightFG: "#eeeeee",
		FG:       "#cc99aa",
		Green:    "#66bb66",
		Yellow:   "#ccbb66",
		Red:      "#3344ff",
	}

	tests := []struct {
		name       string
		mark       func() string
		wantBefore string
		wantAfter  string
	}{
		{name: "favorited", mark: favAddedMark, wantBefore: before.Red, wantAfter: after.Red},
		{name: "unfavorited", mark: favRemovedMark, wantBefore: before.FG, wantAfter: after.FG},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{themes: []theme.Theme{before, after}}
			t.Cleanup(func() { applyThemeAll(theme.Default()) })

			if !m.SetTheme(before.Name) {
				t.Fatalf("SetTheme(%q) = false, want the theme to be found", before.Name)
			}
			if got := renderedFG(t, tc.mark()); got != tc.wantBefore {
				t.Fatalf("marker = %s under %s, want %s", got, before.Name, tc.wantBefore)
			}

			if !m.SetTheme(after.Name) {
				t.Fatalf("SetTheme(%q) = false, want the theme to be found", after.Name)
			}
			got := renderedFG(t, tc.mark())
			if got == tc.wantBefore {
				t.Fatalf("marker still %s from %s after switching to %s", got, before.Name, after.Name)
			}
			if got != tc.wantAfter {
				t.Fatalf("marker = %s under %s, want %s", got, after.Name, tc.wantAfter)
			}
		})
	}
}

// TestPaintMatchesRender proves the fast paint gives the exact bytes of
// lipgloss Render for every model style under every built-in theme, and
// falls back to Render for texts and styles it does not handle.
func TestPaintMatchesRender(t *testing.T) {
	t.Cleanup(func() { applyThemeAll(theme.Default()) })
	texts := []string{" ", "♥", "> Song", "  12. ", "音楽 wide", "a\nb", "tab\there", "\x1b[1mpre\x1b[m"}
	extra := []lipgloss.Style{
		lipgloss.NewStyle(),
		lipgloss.NewStyle().Width(10),
		lipgloss.NewStyle().Padding(0, 1).Foreground(lipgloss.Color("1")),
		lipgloss.NewStyle().Underline(true),
		lipgloss.NewStyle().Italic(true).Faint(true),
		lipgloss.NewStyle().Border(lipgloss.NormalBorder()),
		lipgloss.NewStyle().BorderLeft(true),
		lipgloss.NewStyle().Margin(0, 2),
	}
	for _, th := range append([]theme.Theme{theme.Default()}, theme.LoadAll()...) {
		applyThemeAll(th)
		styles := append([]lipgloss.Style{
			titleStyle, trackStyle, timeStyle, statusStyle, dimStyle, labelStyle, eqActiveStyle, eqInactiveStyle,
			playlistActiveStyle, playlistItemStyle, playlistSelectedStyle, playlistUnavailableStyle, helpStyle,
			helpKeyStyle, errorStyle, seekFillStyle, seekDimStyle, seekSkipStyle, volBarStyle, activeToggle,
			favMarkerStyle, favRemovedStyle,
		}, extra...)
		for i, st := range styles {
			for _, s := range texts {
				if got, want := paint(st, s), st.Render(s); got != want {
					t.Fatalf("theme %q style %d text %q: paint %q, Render %q", th.Name, i, s, got, want)
				}
			}
		}
	}
}

// TestTextWidthMatchesLipgloss checks the fast width against lipgloss on the
// lines of real frames and on random mixes of the characters it handles and
// the ones it must hand over.
func TestTextWidthMatchesLipgloss(t *testing.T) {
	check := func(s string) {
		t.Helper()
		if got, want := textWidth(s), lipgloss.Width(s); got != want {
			t.Fatalf("textWidth(%q) = %d, lipgloss.Width = %d", s, got, want)
		}
	}
	for _, size := range [][2]int{{80, 24}, {120, 40}, {200, 60}} {
		m := newLayoutTestModel(size[0], size[1])
		for _, open := range []func(*Model){func(*Model) {}, (*Model).openKeymap, func(m *Model) { m.fullVis = true }} {
			open(&m)
			for _, line := range strings.Split(m.View().Content, "\n") {
				check(line)
			}
		}
	}
	pieces := []string{"a", " ", "─", "█", "⣿", "●", "♫", "♥", "…", "·", "ñ", "Ø", "→", "\u00ad", "\u200b", "\u2028", "✓", "❚", "音", "é", "e\u0301", "♥\ufe0f", "👍🏽", "\u200d",
		"\x1b[1;38;5;15m", "\x1b[m", "\x1b]8;;http://x\x07", "\x1b]8;;\x1b\\", "\x1b", "\x1b[", "\t", "\x7f"}
	rng := rand.New(rand.NewPCG(1, 2))
	for range 5000 {
		var b strings.Builder
		for range rng.IntN(12) {
			b.WriteString(pieces[rng.IntN(len(pieces))])
		}
		check(b.String())
	}
}

func TestNarrowRunesAreNarrow(t *testing.T) {
	for r := rune(0); r <= 0x2fff; r++ {
		if !narrowRune(r) {
			continue
		}
		if w := lipgloss.Width(string(r)); w != 1 {
			t.Errorf("narrowRune(%U %q) but lipgloss.Width = %d", r, r, w)
		}
		if w := lipgloss.Width("a" + string(r) + "b"); w != 3 {
			t.Errorf("%U joins its neighbours: width %d", r, w)
		}
	}
}

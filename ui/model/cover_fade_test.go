package model

import (
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/theme"
)

func TestBlendTheme(t *testing.T) {
	black := themeRGB{}
	var white themeRGB
	for i := range white {
		white[i] = [3]int{255, 255, 255}
	}
	if got := blendTheme(black, white, 0.5, "cover"); got.Accent != "#808080" || got.BG != "#808080" || got.Name != "cover" {
		t.Fatalf("half blend = %+v", got)
	}
	if _, ok := parseThemeRGB(theme.Theme{BG: "#000000"}); ok {
		t.Fatal("a theme with missing colors must not parse")
	}
}

func TestCoverFadeBlendsThenSettles(t *testing.T) {
	t.Cleanup(func() { applyThemeAll(theme.Default()) })
	red := theme.Theme{Name: "cover", BG: "#100000", Accent: "#ff0000", BrightFG: "#ffffff", FG: "#cccccc", Green: "#00ff00", Yellow: "#ffff00", Red: "#ff0000"}
	blue := theme.Theme{Name: "cover", BG: "#000010", Accent: "#0000ff", BrightFG: "#ffffff", FG: "#cccccc", Green: "#00ff00", Yellow: "#ffff00", Red: "#ff0000"}
	m := Model{themes: []theme.Theme{{Name: "cover"}}, themeIdx: 0}
	now := time.Now()

	m.startCoverFade(red, now)
	if m.coverFade.active || m.coverFade.shown.Accent != red.Accent {
		t.Fatal("the first cover must show at once")
	}
	m.startCoverFade(blue, now)
	m.tickCoverFade(now.Add(coverFadeTime / 2))
	if got := m.coverFade.shown.Accent; got != "#800080" {
		t.Fatalf("halfway accent = %s, want a mix of red and blue", got)
	}
	m.player = &playbackFakeEngine{}
	if m.isFullyIdle() {
		t.Fatal("a running fade must keep the tick awake")
	}
	m.tickCoverFade(now.Add(coverFadeTime))
	if m.coverFade.active || m.coverFade.shown.Accent != blue.Accent {
		t.Fatalf("after the fade: active %v, accent %s", m.coverFade.active, m.coverFade.shown.Accent)
	}
}

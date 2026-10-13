package config

import "testing"

// A fork's provider reads its own [providers.<key>] section.
func TestProviderSection(t *testing.T) {
	cfg := loadConfigText(t, "[Providers.MyRadio]\nurl = \"https://radio.example.com\"\nuser = me\n\n[ytmusic]\nlanguage = \"es\"\n")
	got := cfg.ProviderSection("myradio")
	if got["url"] != "https://radio.example.com" || got["user"] != "me" {
		t.Fatalf("ProviderSection = %v", got)
	}
	if cfg.ProviderSection("other") != nil {
		t.Fatal("a missing section is not nil")
	}
	if cfg.YouTubeMusic.Language != "es" {
		t.Fatal("the section after it was not read")
	}
}

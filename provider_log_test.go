package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/provider"
)

// readLog opens a fresh log file for the test and returns a func that reads
// back its content. This exercises the same path initLogging uses, so the
// test catches a regression in the log wiring, not just the format strings.
func readLog(t *testing.T) func() string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cliamp.log")
	closeFn, err := applog.Init(path, applog.LevelInfo)
	if err != nil {
		t.Fatalf("applog.Init: %v", err)
	}
	t.Cleanup(func() { _ = closeFn() })
	return func() string {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read log file: %v", err)
		}
		return string(data)
	}
}

func TestLogProviderRegistered(t *testing.T) {
	readBack := readLog(t)

	logProviderRegistered("YouTube Music", "ytmusic")

	got := readBack()
	if !strings.Contains(got, "provider registered") {
		t.Errorf("log missing %q: %s", "provider registered", got)
	}
	if !strings.Contains(got, "name=YouTube Music") {
		t.Errorf("log missing provider name: %s", got)
	}
	if !strings.Contains(got, "key=ytmusic") {
		t.Errorf("log missing provider key: %s", got)
	}
}

func TestLogProviderSkipped(t *testing.T) {
	readBack := readLog(t)

	logProviderSkipped("Spotify", "spotify", "not configured")

	got := readBack()
	if !strings.Contains(got, "provider skipped") {
		t.Errorf("log missing %q: %s", "provider skipped", got)
	}
	if !strings.Contains(got, "key=spotify") {
		t.Errorf("log missing provider key: %s", got)
	}
	if !strings.Contains(got, "reason=not configured") {
		t.Errorf("log missing skip reason: %s", got)
	}
}

func TestLogProviderWiring(t *testing.T) {
	readBack := readLog(t)

	providers := []provider.Entry{
		{Key: "ytmusic", Name: "YouTube Music"},
		{Key: "local", Name: "Local"},
	}
	logProviderWiring(providers)

	got := readBack()
	if strings.Count(got, "provider registered") != len(providers) {
		t.Errorf("expected %d registration lines, got: %s", len(providers), got)
	}
	for _, p := range providers {
		if !strings.Contains(got, "key="+p.Key) {
			t.Errorf("log missing registration for %q: %s", p.Key, got)
		}
	}
	if strings.Contains(got, "reason=not configured") {
		t.Errorf("no optional provider exists, so none may be logged as skipped: %s", got)
	}
}

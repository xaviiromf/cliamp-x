package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// logout and ytmusic reset forget the saved YouTube Music cookies and stop
// the automatic sign-in until the next login.
func TestLogoutCommands(t *testing.T) {
	for _, args := range [][]string{{"cliamp-x", "logout"}, {"cliamp-x", "ytmusic", "reset"}} {
		t.Run(strings.Join(args[1:], " "), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("CLIAMP_CONFIG_DIR", dir)
			jar := filepath.Join(dir, "ytmusic_cookies.txt")
			if err := os.WriteFile(jar, []byte("# Netscape HTTP Cookie File\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := buildApp().Run(t.Context(), args); err != nil {
				t.Fatalf("%v: %v", args, err)
			}
			if _, err := os.Stat(jar); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("cookie jar still exists: stat error = %v", err)
			}
			data, err := os.ReadFile(filepath.Join(dir, "ytmusic_auth.json"))
			if err != nil || !strings.Contains(string(data), `"logged_out": true`) {
				t.Errorf("auth record = %q, %v; want logged_out", data, err)
			}
			// A second logout finds no jar and still succeeds.
			if err := buildApp().Run(t.Context(), args); err != nil {
				t.Errorf("%v without a jar: %v", args, err)
			}
		})
	}
}

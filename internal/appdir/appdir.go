package appdir

import (
	"os"
	"path/filepath"
)

// Dir returns the cliamp configuration directory (~/.config/cliamp).
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "cliamp"), nil
}

// PluginDir returns the cliamp plugin directory (~/.config/cliamp/plugins).
func PluginDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "plugins"), nil
}

// DataDir returns the cliamp data directory (~/.local/share/cliamp), used for
// state that is not user-edited config: plugin stores, downloaded assets, etc.
func DataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "cliamp"), nil
}

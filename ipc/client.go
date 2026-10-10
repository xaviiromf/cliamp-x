package ipc

import (
	"os"
	"path/filepath"

	"github.com/xaviiromf/cliamp-x/internal/appdir"
)

// DefaultSocketPath returns the default IPC socket path (~/.config/cliamp/cliamp.sock).
func DefaultSocketPath() string {
	dir, err := appdir.Dir()
	if err != nil {
		return filepath.Join(os.TempDir(), "cliamp-x.sock")
	}
	return filepath.Join(dir, "cliamp-x.sock")
}

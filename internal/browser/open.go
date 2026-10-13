package browser

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/xaviiromf/cliamp-x/applog"
)

// Open tries to open a URL in the user's default browser. On Android it
// writes the URL to the log and the status bar instead, as the core runs
// with no browser to start.
func Open(u string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", u).Start()
	case "linux":
		return exec.Command("xdg-open", u).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", u).Start()
	case "android":
		applog.UserWarn("open in a browser: %s", u)
		return nil
	default:
		return fmt.Errorf("unsupported platform")
	}
}

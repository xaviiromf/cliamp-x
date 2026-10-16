package player

import (
	"os"
	"strconv"
	"time"
)

// parseDeviceSampleRate reads the output rate the Android app passes in
// CLIAMP_DEVICE_SAMPLE_RATE. It returns 0 for anything but 8000..192000.
// It builds on every platform so its test runs on the host.
func parseDeviceSampleRate(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil || n < 8000 || n > 192000 {
		return 0
	}
	return n
}

// Lightweight mode is the Android app's setting for slow phones. The app sets
// CLIAMP_LIGHT=1 when it starts the core, which then asks for a speaker buffer
// of at least LightBufferMs and starts the next-track prefetch LightPrefetchDelay
// late, so it does not compete with the start of the current song.
const (
	LightBufferMs      = 1000
	LightPrefetchDelay = 8 * time.Second
	lightEnv           = "CLIAMP_LIGHT"
)

// LightMode reports whether the app asked for lightweight mode.
func LightMode() bool { return parseLightMode(os.Getenv(lightEnv)) }

func parseLightMode(s string) bool { return s == "1" }

// BufferMsFor returns the speaker buffer to open: ms, or LightBufferMs when
// lightweight mode needs a bigger one. A buffer the user set higher stays.
func BufferMsFor(ms int, light bool) int {
	if light {
		return max(ms, LightBufferMs)
	}
	return ms
}

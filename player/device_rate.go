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

// OpenSLMinBufferMs is the smallest speaker buffer, in ms, that keeps the
// Android output fed at sample rate sr. oto plays through Oboe's OpenSL ES
// backend, whose feeder thread asks oto's mixer for three stream buffers at a
// time; the mixer hands over what beep's player buffer (half the speaker
// buffer) holds and pads the rest with silence. Without a FAST track (no fast
// mixer on the route, or none free) Oboe makes the stream buffer 8 bursts of
// 20 ms rounded up to 192 frames: 23040 frames a read at 48 kHz against 6000
// in a 250 ms buffer, so the phone played 26% audio and 74% silence. The
// stream opens once per core, so the route at core start decided it. The
// result is rounded up to 50 ms.
func OpenSLMinBufferMs(sr int) int {
	if sr <= 0 {
		return 0
	}
	burst := (sr*20/1000 + 191) / 192 * 192
	read := 3 * 8 * burst
	ms := (2*read*1000 + sr - 1) / sr
	return (ms + 49) / 50 * 50
}

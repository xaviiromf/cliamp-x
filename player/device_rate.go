package player

import "strconv"

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

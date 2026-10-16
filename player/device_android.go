// player/device_android.go — the phone's native output rate from the app.

package player

import "os"

// DeviceSampleRate returns the rate the Android app reads from
// AudioManager.PROPERTY_OUTPUT_SAMPLE_RATE and passes in
// CLIAMP_DEVICE_SAMPLE_RATE, or 0 when it is unset or invalid. Playing at
// that rate avoids a second resample and lets Android grant a FAST track.
func DeviceSampleRate() int {
	return parseDeviceSampleRate(os.Getenv("CLIAMP_DEVICE_SAMPLE_RATE"))
}

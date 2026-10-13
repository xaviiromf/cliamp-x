package main

import (
	"os"
	"time"
)

// Go pins time.Local to UTC on Android and ignores TZ. The app passes the
// system zone ID in TZ, so the clock, the log and the stats use local time.
func init() {
	if tz := os.Getenv("TZ"); tz != "" {
		if loc, err := time.LoadLocation(tz); err == nil {
			time.Local = loc
		}
	}
}

#!/usr/bin/env bash
# Drives the native UI on a device or emulator over adb (UI-SPEC §9.3) and
# saves a screenshot per step. Install the APK first. Usage:
#
#   ANDROID_SERIAL=emulator-5554 android/scripts/ui-smoke.sh [OUT_DIR] [SCENARIO...]
#
# Scenarios: s1 (cold start: clears the app data), s3 (search), s4 (play a
# search song). With none, it runs s3 and s4. The elements are found by
# their Compose test tags, which the app exposes as resource IDs.
set -euo pipefail

OUT=${1:-shots}
shift || true
PKG=io.github.xaviiromf.cliampx
mkdir -p "$OUT"

# node ID: prints the center "x y" of the element tagged ID, or fails.
node() {
	local bounds
	bounds=$(adb exec-out uiautomator dump /dev/tty 2>/dev/null |
		grep -o "resource-id=\"$1\"[^>]*bounds=\"\[[0-9]*,[0-9]*\]\[[0-9]*,[0-9]*\]\"" |
		head -1 | grep -o '\[[0-9]*,[0-9]*\]\[[0-9]*,[0-9]*\]') || return 1
	echo "$bounds" | tr '[],' '   ' | awk '{ print int(($1 + $3) / 2), int(($2 + $4) / 2) }'
}

# tap ID: taps the element. A dump fails while the UI animates (a spinner), so it retries for 10 s.
tap() {
	local xy end=$((SECONDS + 10))
	until xy=$(node "$1"); do
		((SECONDS < end)) || { echo "no element $1" >&2; return 1; }
		sleep 0.5
	done
	# shellcheck disable=SC2086 # xy is "x y"
	adb shell input tap $xy
}

swipe() { adb shell input swipe "$@"; }

shot() { adb exec-out screencap -p >"$OUT/$1.png"; echo "$OUT/$1.png"; }

# waitfor ID [seconds]: waits until the element exists.
waitfor() {
	local end=$((SECONDS + ${2:-20}))
	until node "$1" >/dev/null; do
		((SECONDS < end)) || { echo "timeout waiting for $1" >&2; return 1; }
		sleep 1
	done
}

s1() {
	adb shell pm clear "$PKG" >/dev/null
	adb shell pm grant "$PKG" android.permission.READ_MEDIA_AUDIO
	adb shell pm grant "$PKG" android.permission.POST_NOTIFICATIONS
	adb shell am start -n "$PKG/.MainActivity" >/dev/null
	sleep 2
	shot s1-splash
	waitfor tab_home 90
	shot s1-home
	adb shell run-as "$PKG" cat shared_prefs/updates.xml | grep ytdlp_tag
	adb shell "run-as $PKG head -c 2 no_backup/youtubedl-android/yt-dlp/yt-dlp"
	echo
}

s3() {
	tap tab_search
	sleep 1
	shot s3-search-empty
	adb shell dumpsys input_method | grep -q 'mInputShown=true' && echo "FAIL: the keyboard opened" >&2
	tap search_field
	adb shell input text 'daft%spunk'
	adb shell input keyevent 66
	waitfor row_0 60
	shot s3-results
}

s4() {
	tap row_0
	waitfor mini_player 20
	sleep 8
	shot s4-playing
	adb shell run-as "$PKG" sh -c 'cat files/config/cliamp.log' | tail -3
}

for s in "${@:-s3 s4}"; do
	for one in $s; do "$one"; done
done

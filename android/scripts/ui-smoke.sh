#!/usr/bin/env bash
# Drives the native UI on a device or emulator over adb (UI-SPEC §9.3) and
# saves a screenshot per step. Install the APK first. Usage:
#
#   ANDROID_SERIAL=emulator-5554 android/scripts/ui-smoke.sh [OUT_DIR] [SCENARIO...]
#
# Scenarios: s1 (cold start: clears the app data), s3 (search), s4 (play a
# search song), and, while a song plays, s5 (player gestures), s8 (play
# next from a search row; run s3 first), s14 (EQ drag), s15 (sleep timer)
# and s16 (visualizer). Browse (M3): s2 (Inicio signed out), then after s3
# s9 (album row 2), s10 (artist), s11 (genre and sort), s18 (back to the
# launcher), s19l (Letras open while rotating: no crash), s20 (offline) and
# s23 (radio and remixes; they need the Batch C operations). With none, it
# runs s3 and s4. The elements are found by their Compose test tags, which
# the app exposes as resource IDs.
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
	node search_clear >/dev/null && tap search_clear
	tap search_field
	adb shell input text 'daft%spunk'
	# No Enter: the emulator's keyboard takes it for a word suggestion. The query goes after 450 ms; back hides the keyboard.
	sleep 1
	adb shell input keyevent KEYCODE_BACK
	waitfor tile_artists_0 60
	shot s3-results
	# The songs come after the artist, album and playlist tiles.
	swipe 540 1700 540 500 400
	waitfor row_0 10
}

s4() {
	tap row_0
	waitfor mini_player 20
	sleep 8
	shot s4-playing
	adb shell run-as "$PKG" sh -c 'cat files/config/cliamp.log' | tail -3
}

# drag ID DX DY [ms]: drags from the center of the element.
drag() {
	local x y
	read -r x y < <(node "$1") || { echo "no element $1" >&2; return 1; }
	swipe "$x" "$y" $((x + $2)) $((y + $3)) "${4:-300}"
}

s5() {
	drag mini_player 0 -400 200
	waitfor art 5
	shot s5-player
	drag art -600 0 250
	sleep 2
	shot s5-skipped
	drag art 0 1200 250
	sleep 1
	if node player >/dev/null; then echo "FAIL: the player is still open" >&2; fi
}

s8() {
	drag row_0 0 0 800 # a long-press
	tap menu_play_next
	sleep 1
	tap mini_player
	waitfor chip_queue 5
	shot s8-queue-count
	adb shell input keyevent KEYCODE_BACK
}

s14() {
	tap mini_player
	tap chip_eq
	drag eq_band_0 0 -80 600
	sleep 1
	shot s14-eq
	adb shell input keyevent KEYCODE_BACK
	adb shell input keyevent KEYCODE_BACK
}

s15() {
	tap mini_player
	tap chip_sleep
	tap sleep_15
	sleep 1
	shot s15-sleep
	adb shell input keyevent KEYCODE_BACK
}

s16() {
	tap mini_player
	tap vis_strip
	sleep 1
	tap vis_strip
	sleep 1
	shot s16-a
	sleep 0.2
	shot s16-b
	adb shell input keyevent KEYCODE_BACK
}

# fail MESSAGE: reports a failed check and goes on.
fail() { echo "FAIL: $*" >&2; }

s2() {
	tap tab_home
	waitfor tile_shelf0_0 60
	shot s2-home
	node banner_signin >/dev/null || fail "no sign-in card while signed out"
	! node tile_new_0 >/dev/null || fail "Novedades shows while signed out"
}

# results: Buscar at its root and top, with the results of s3 (a tap on the open tab pops it, then scrolls it up).
results() {
	for _ in 1 2 3; do
		tap tab_search
		sleep 1
	done
	node tile_albums_0 >/dev/null && return
	s3
	tap tab_search # back to the top of the results
	sleep 1
}

s9() {
	results
	tap tile_albums_0
	waitfor row_2 60
	shot s9-album
	tap row_2
	sleep 20
	shot s9-playing
	adb shell input keyevent KEYCODE_BACK
}

s10() {
	results
	tap tile_artists_0
	waitfor row_0 60
	shot s10-artist
	! node detail_follow >/dev/null || fail "Seguir shows while signed out"
	adb shell input keyevent KEYCODE_BACK
}

s11() {
	tap tab_search
	node search_clear >/dev/null && tap search_clear
	waitfor genre_2 30
	tap genre_2
	waitfor row_0 60
	shot s11-genre
	tap detail_play
	tap sort_1
	sleep 8
	shot s11-sorted
	adb shell input keyevent KEYCODE_BACK
}

s18() {
	results
	tap tile_albums_0
	waitfor detail_play 60
	for _ in 1 2 3; do
		adb shell input keyevent KEYCODE_BACK
		sleep 1.5
	done
	adb shell dumpsys activity activities | grep -m1 topResumedActivity | grep -q "$PKG" && fail "back did not reach the launcher"
	adb shell dumpsys media_session | grep -m1 -o 'PlaybackState {state=[A-Z]*'
	adb shell am start -n "$PKG/.MainActivity" >/dev/null
}

s19l() {
	adb logcat -c
	tap mini_player
	tap chip_lyrics
	adb shell settings put system accelerometer_rotation 0
	adb shell settings put system user_rotation 1
	sleep 3
	shot s19l-landscape
	adb shell settings put system user_rotation 0
	sleep 2
	adb logcat -d -b crash | grep -q . && fail "crash: $(adb logcat -d -b crash | grep -m1 Exception)"
	adb shell input keyevent KEYCODE_BACK
}

s20() {
	adb shell svc wifi disable
	adb shell svc data disable
	tap tab_home
	swipe 540 700 540 1700 600
	sleep 2
	shot s20-pull
	tap tab_search
	node search_clear >/dev/null && tap search_clear
	tap search_field
	adb shell input text 'radiohead'
	sleep 8
	adb shell input keyevent KEYCODE_BACK
	shot s20-search
	adb exec-out uiautomator dump /dev/tty | grep -q 'ERR: Sin conexi' || fail "no offline error"
	adb shell svc wifi enable
	adb shell svc data enable
}

s23() {
	results
	swipe 540 1700 540 500 400 # down to the songs
	waitfor row_0 10
	drag row_0 0 0 800 # a long-press
	if ! node menu_radio >/dev/null; then
		echo "SKIP s23: radio.start is not in capabilities (Batch C)"
		adb shell input keyevent KEYCODE_BACK
		return
	fi
	tap menu_radio
	sleep 20
	shot s23-radio
	drag row_0 0 0 800
	tap menu_remixes
	waitfor row_0 60
	shot s23-remixes
}

for s in "${@:-s3 s4}"; do
	for one in $s; do "$one"; done
done

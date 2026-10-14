#!/usr/bin/env bash
# Drives the native UI on a device or emulator over adb (UI-SPEC §9.3) and
# saves a screenshot per step. Install the APK first. Usage:
#
#   ANDROID_SERIAL=emulator-5554 android/scripts/ui-smoke.sh [OUT_DIR] [SCENARIO...]
#
# Scenarios, by the numbers of §9.3. s1 (cold start) and s1b (a song tapped
# during the first yt-dlp update) clear the app data. Search: s3, then s4
# (play a search song). While a song plays: s5 (player gestures), s6 (seek),
# s7 (queue swipe and drag; after s4, whose rows have different titles), s8
# (play next; after s3), s14 (EQ drag), s15 (sleep timer), s16
# (visualizer), s17 (Modo terminal round trips and the screen off in it),
# s19 (rotation: the player stays portrait, Modo terminal turns), s21 (core
# restart), s22 (the media session, also with the screen off) and s13
# (themes and the cover theme's blend, recorded; a YouTube song). Browse: s2
# (Inicio signed out), then after s3 s9 (album row 2), s10 (artist), s11
# (genre and sort), s18 (back to the launcher), s20 (offline) and s23 (radio
# and remixes). Library: s12 (a local song), s24 (download an album, then
# play it offline; after s3), s25 (a rating and its Deshacer; after s3) and
# s26 (statistics). perf runs §9.4 on a release build: frame times, CPU and
# memory. With none, it runs s3 and s4. The elements are found by their
# Compose test tags, which the app exposes as resource IDs. A failed check
# prints FAIL and the run goes on; a missing element ends the run.
set -euo pipefail

OUT=${1:-shots}
shift || true
PKG=io.github.xaviiromf.cliampx
mkdir -p "$OUT"

# bounds ID: prints "x1 y1 x2 y2" of the element tagged ID, or fails.
bounds() {
	adb exec-out uiautomator dump /dev/tty 2>/dev/null |
		grep -o "resource-id=\"$1\"[^>]*bounds=\"\[[0-9]*,[0-9]*\]\[[0-9]*,[0-9]*\]\"" |
		head -1 | grep -o '\[[0-9]*,[0-9]*\]\[[0-9]*,[0-9]*\]' | tr '[],' '   ' | awk 'NF == 4 { print $1, $2, $3, $4 }' | grep .
}

# node ID: prints the center "x y" of the element tagged ID, or fails.
node() { bounds "$1" | awk '{ print int(($1 + $3) / 2), int(($2 + $4) / 2) }'; }

# text ID: prints the first text in the element tagged ID, art placeholders (░) skipped.
# awk reads to the end: an early exit would fail the pipe (pipefail) and end the run.
text() {
	adb exec-out uiautomator dump /dev/tty 2>/dev/null | tr '>' '\n' |
		awk -v id="resource-id=\"$1\"" 'index($0, id) { on = 1 }
			on && !done && match($0, /text="[^"]+"/) { t = substr($0, RSTART + 6, RLENGTH - 7); if (index(t, "░") != 1) { print t; done = 1 } }'
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

# A song tapped while the first yt-dlp update runs waits for it, then plays.
s1b() {
	adb shell pm clear "$PKG" >/dev/null
	adb shell pm grant "$PKG" android.permission.READ_MEDIA_AUDIO
	adb shell pm grant "$PKG" android.permission.POST_NOTIFICATIONS
	adb shell am start -n "$PKG/.MainActivity" >/dev/null
	waitfor tab_search 90
	s3
	tap row_0
	sleep 1
	shot s1b-preparing
	local end=$((SECONDS + 90))
	until playing; do
		((SECONDS < end)) || { fail "the song did not play after the first yt-dlp update"; return; }
		sleep 2
	done
	shot s1b-playing
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
	adb shell run-as "$PKG" cat files/config/cliamp.log | tail -3
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

# share: the player's "01:23 / 03:45" as the played share of the song, in percent.
share() {
	adb exec-out uiautomator dump /dev/tty 2>/dev/null | grep -o 'text="[0-9]*:[0-9]* / [0-9]*:[0-9]*"' | head -1 |
		tr -c '0-9\n' ' ' | awk 'NF == 4 && $3 * 60 + $4 > 0 { print int(100 * ($1 * 60 + $2) / ($3 * 60 + $4)) }'
}

# Seek on a YouTube song: a drag from 10 % to 60 % lands there and stays (no jump back).
s6() {
	tap mini_player
	waitfor seek 10
	local x1 y1 x2 y2 y w
	read -r x1 y1 x2 y2 < <(bounds seek)
	y=$(((y1 + y2) / 2)) w=$((x2 - x1))
	swipe $((x1 + w / 10)) "$y" $((x1 + w * 6 / 10)) "$y" 600
	sleep 0.5
	shot s6-seek
	local at
	at=$(share)
	((${at:-0} >= 55 && ${at:-0} <= 65)) || fail "the seek shows ${at:-?} %, not 60 %"
	sleep 3
	shot s6-seek-3s
	at=$(share)
	((${at:-0} >= 55 && ${at:-0} <= 70)) || fail "the seek went back to ${at:-?} %"
	adb shell input keyevent KEYCODE_BACK
}

# Queue: a swipe to the start removes row 1; the ≡ of row 0 drags it two rows down. Shuffle must be off.
s7() {
	tap mini_player
	tap chip_queue
	waitfor row_2 20
	local before x1 y1 x2 y2
	before=$(text row_1)
	drag row_1 -800 0 300
	sleep 2
	[[ $(text row_1) != "$before" ]] || fail "the swipe left row 1 in place"
	shot s7-removed
	before=$(text row_0)
	read -r x1 y1 x2 y2 < <(bounds row_0)
	# The handle is the row's last 48 dp.
	swipe $((x2 - 60)) $(((y1 + y2) / 2)) $((x2 - 60)) $(((y1 + y2) / 2 + 2 * (y2 - y1))) 1500
	sleep 3
	[[ $(text row_2) == "$before" ]] || fail "the drag did not move row 0 to row 2"
	text status_line | grep -q 'ERR' && fail "a queue edit failed: $(text status_line)"
	shot s7-moved
	adb shell input keyevent KEYCODE_BACK
	adb shell input keyevent KEYCODE_BACK
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

# terminal: whether Modo terminal is on top.
terminal() { adb shell dumpsys activity activities | grep -m1 topResumedActivity | grep -q TerminalActivity; }

# leave: back out of Modo terminal (back at [Cola] closes it).
leave() {
	for _ in 1 2 3; do
		terminal || return 0
		adb shell input keyevent KEYCODE_BACK
		sleep 1
	done
	terminal && fail "back did not leave Modo terminal"
}

# Modo terminal from Ajustes: 5 fast round trips, then the screen off and on inside it; the TUI draws each time.
s17() {
	tap tab_home
	tap settings
	scrollto set_terminal
	for i in 1 2 3 4 5; do
		tap set_terminal
		sleep 1
		terminal || fail "round trip $i: Modo terminal did not open"
		leave
	done
	tap set_terminal
	sleep 2
	shot s17-terminal
	adb shell input keyevent KEYCODE_SLEEP
	sleep 3
	adb shell input keyevent KEYCODE_WAKEUP
	adb shell wm dismiss-keyguard
	sleep 2
	shot s17-terminal-wake
	leave
	adb shell input keyevent KEYCODE_BACK # Ajustes to Inicio
	tap mini_player
	sleep 1
	shot s17-vis-a
	sleep 0.3
	shot s17-vis-b
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

# portrait PNG: whether the screenshot is taller than wide.
portrait() { od -An -j16 -N8 -tu1 "$1" | awk '{ exit !($5 * 16777216 + $6 * 65536 + $7 * 256 + $8 > $1 * 16777216 + $2 * 65536 + $3 * 256 + $4) }'; }

# Rotation turned to landscape: the player (Letras open) stays portrait and Modo terminal turns. Restores the rotation settings.
s19() {
	local acc rot
	acc=$(adb shell settings get system accelerometer_rotation)
	rot=$(adb shell settings get system user_rotation)
	adb logcat -c
	tap mini_player
	tap chip_lyrics
	adb shell settings put system accelerometer_rotation 0
	adb shell settings put system user_rotation 1
	sleep 3
	portrait "$(shot s19-player)" || fail "the player turned to landscape"
	adb shell settings put system user_rotation 0
	adb shell input keyevent KEYCODE_BACK
	tap tab_home
	tap settings
	scrollto set_terminal
	tap set_terminal
	sleep 2
	adb shell settings put system user_rotation 1
	sleep 3
	portrait "$(shot s19-terminal)" && fail "Modo terminal did not turn"
	adb shell settings put system user_rotation 0
	sleep 2
	leave
	adb shell input keyevent KEYCODE_BACK
	for kv in "accelerometer_rotation=$acc" "user_rotation=$rot"; do
		if [[ ${kv#*=} == null ]]; then adb shell settings delete system "${kv%%=*}" >/dev/null; else adb shell settings put system "${kv%%=*}" "${kv#*=}"; fi
	done
	sleep 2
	if adb logcat -d -b crash | grep -q .; then fail "crash: $(adb logcat -d -b crash | grep -m1 Exception)"; fi
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

# Core restart (Cerrar sesión restarts it) with a song loaded: the UI reconnects and follows the core again.
s21() {
	tap tab_home
	tap settings
	scrollto set_signout
	local old pid end=$((SECONDS + 60))
	old=$(adb shell pidof libcliamp.so || true)
	tap set_signout
	sleep 1
	shot s21-reconnect
	until pid=$(adb shell pidof libcliamp.so) && [[ $pid != "$old" ]]; do
		((SECONDS < end)) || { fail "the core did not start again"; return; }
		sleep 1
	done
	sleep 5 # the reconnect and the first state.get
	shot s21-back
	local was=0
	if playing; then was=1; fi
	tap mini_toggle
	end=$((SECONDS + 40)) # from stopped, a YouTube song opens its stream again first
	until if playing; then ((was == 0)); else ((was == 1)); fi; do
		((SECONDS < end)) || { fail "play/pause did not reach the restarted core"; break; }
		sleep 2
	done
	adb shell dumpsys media_session | grep -m1 -o 'PlaybackState {state=[A-Z]*([0-9]*), position=[0-9]*'
	playing || tap mini_toggle
	adb shell input keyevent KEYCODE_BACK
}

# The media session: pause and play from outside, also with the screen off; the position keeps moving.
s22() {
	tap mini_player
	waitfor play_pause 10
	adb shell cmd media_session dispatch pause
	sleep 1
	playing && fail "pause did not stop the song"
	shot s22-paused
	adb shell cmd media_session dispatch play
	sleep 1
	playing || fail "play did not start the song"
	shot s22-playing
	adb shell input keyevent KEYCODE_SLEEP
	sleep 2
	adb shell cmd media_session dispatch pause
	sleep 1
	playing && fail "pause with the screen off did not stop the song"
	adb shell cmd media_session dispatch play
	local a b
	a=$(position)
	sleep 5
	b=$(position)
	# Only forward: the headless emulator's audio runs slower than the clock, so no exact 5 s.
	((b > a)) || fail "the session position stalled with the screen off: $a, then $b"
	adb shell input keyevent KEYCODE_WAKEUP
	adb shell wm dismiss-keyguard
	sleep 1
	shot s22-wake
	adb shell input keyevent KEYCODE_BACK
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

# playing: whether the media session reports a song that plays.
playing() { adb shell dumpsys media_session | grep -m1 -q 'PlaybackState {state=PLAYING'; }

# scrollto ID: swipes the list up until the element shows, at most 6 times.
scrollto() {
	for _ in 1 2 3 4 5 6; do
		node "$1" >/dev/null && return
		swipe 540 1700 540 700 300
		sleep 1
	done
	node "$1" >/dev/null
}

s12() {
	tap tab_library
	tap lib_local
	waitfor row_0 30
	tap row_0
	sleep 6
	shot s12-local
	playing || fail "the local song does not play"
	adb shell input keyevent KEYCODE_BACK
}

s24() {
	results
	tap tile_albums_0
	waitfor detail_download 60
	tap detail_download
	sleep 2
	tap tab_library
	tap lib_chip_downloads
	sleep 8
	shot s24-progress
	local end=$((SECONDS + 600))
	while node downloads_cancel >/dev/null && ((SECONDS < end)); do sleep 5; done
	shot s24-done
	adb shell svc wifi disable
	adb shell svc data disable
	tap downloads_play
	sleep 10
	shot s24-offline
	playing || fail "the downloads do not play offline"
	adb shell svc wifi enable
	adb shell svc data enable
}

s25() {
	results
	swipe 540 1700 540 500 400 # down to the songs
	waitfor row_0 10
	drag row_0 0 0 800 # a long-press
	tap menu_not_interested
	sleep 2
	shot s25-hidden
	tap tab_library
	tap lib_ratings
	waitfor rating_no_0 20
	shot s25-ratings
	tap rating_no_0_undo
	sleep 2
	shot s25-undone
	! node rating_no_0 >/dev/null || fail "Deshacer left the rating"
	adb shell input keyevent KEYCODE_BACK
}

s26() {
	tap tab_library
	tap lib_stats
	for p in week month year all; do
		tap "stats_$p"
		waitfor stats_summary 20 || fail "no summary for $p"
		shot "s26-$p"
	done
	adb shell input keyevent KEYCODE_BACK
}

# Needs a YouTube song that plays (s4) for the cover theme; ends on the Default theme.
s13() {
	tap tab_home
	tap settings
	tap set_theme
	waitfor theme_default 20
	scrollto theme_winamp && tap theme_winamp
	sleep 2
	shot s13-winamp
	scrollto theme_catppuccin-latte || { swipe 540 700 540 1900 300; swipe 540 700 540 1900 300; }
	tap theme_catppuccin-latte
	sleep 2
	shot s13-latte
	tap theme_cover
	sleep 2
	shot s13-cover
	# The next cover's colors blend in over 600 ms (§4.5): the recording shows it frame by frame.
	adb shell screenrecord --time-limit 15 /sdcard/cliampx-blend.mp4 &
	sleep 1
	adb shell input keyevent KEYCODE_MEDIA_NEXT
	wait
	adb pull /sdcard/cliampx-blend.mp4 "$OUT/s13-cover-blend.mp4" >/dev/null 2>&1 && echo "$OUT/s13-cover-blend.mp4"
	adb shell rm /sdcard/cliampx-blend.mp4
	shot s13-cover-next
	swipe 540 700 540 1900 300
	tap theme_default
	adb shell input keyevent KEYCODE_BACK
	adb shell input keyevent KEYCODE_BACK
}

# position: the media session's position now, in ms (its last position plus the time since, at its speed).
position() {
	adb shell dumpsys media_session | grep -m1 -o 'state=PLAYING([0-9]*), position=[0-9]*, buffered position=[0-9]*, speed=[0-9.]*, updated=[0-9]*' |
		tr -c '0-9.\n' ' ' | awk -v now="$(adb shell cat /proc/uptime | awk '{ print int($1 * 1000) }')" '{ print int($2 + (now - $5) * $4) }'
}

# cpu: the app's and the core's CPU, the average of three 10 s samples.
cpu() {
	adb shell top -b -n 3 -d 10 -o PID,%CPU,RES,ARGS | grep -E 'io.github.xaviiromf.cliampx$|libcliamp' |
		awk '{ n[$4] += $2; c[$4]++ } END { for (k in n) printf "  %-60s %5.1f %%\n", k, n[k] / c[k] }'
}

# §9.4 on a release build (assembleRelease, signed with the debug key when there is no keystore)
# while a song plays and the visualizer is Bars. Needs a list of 200 songs or more: Música local, for example
# adb shell 'mkdir -p /sdcard/Music/perf; for i in $(seq 200); do cp /sdcard/Music/a.mp3 /sdcard/Music/perf/$i.mp3; done',
# or PERF_LIST=lib_liked for the liked songs of a signed-in account.
perf() {
	tap tab_home
	tap tab_home # back to the top of Inicio
	adb shell dumpsys gfxinfo "$PKG" reset >/dev/null
	for _ in 1 2 3 4 5; do swipe 540 1800 540 700 150; sleep 0.5; done
	for _ in 1 2 3 4 5; do swipe 540 700 540 1800 150; sleep 0.5; done
	local x y
	read -r x y < <(node tile_shelf0_0)
	for _ in 1 2 3; do swipe 900 "$y" 150 "$y" 200; sleep 0.5; done
	tap tab_library
	tap "${PERF_LIST:-lib_local}"
	waitfor row_0 30
	for _ in 1 2 3 4 5 6 7 8 9 10; do swipe 540 1900 540 400 120; sleep 0.3; done
	for _ in 1 2 3; do
		tap mini_player
		sleep 3
		adb shell input keyevent KEYCODE_BACK
		sleep 1
	done
	echo "frames:"
	adb shell dumpsys gfxinfo "$PKG" | grep -E -m6 'Total frames|Janky frames|percentile'
	adb shell input keyevent KEYCODE_BACK
	tap tab_home
	sleep 3
	echo "CPU, Inicio idle:"
	cpu
	tap mini_player
	sleep 2
	echo "CPU, player with the visualizer:"
	cpu
	adb shell input keyevent KEYCODE_BACK
	adb shell input keyevent KEYCODE_SLEEP
	sleep 3
	echo "CPU, screen off and playing:"
	cpu
	adb shell input keyevent KEYCODE_WAKEUP
	adb shell wm dismiss-keyguard
	adb shell dumpsys meminfo "$PKG" | grep -m1 'TOTAL PSS'
}

for s in "${@:-s3 s4}"; do
	for one in $s; do "$one"; done
done

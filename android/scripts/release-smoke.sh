#!/usr/bin/env bash
# Fresh install of a minified release APK: fails when the core does not start
# or the app logs an init failure that only R8 causes (a release build is not
# debuggable, so no run-as: the checks read logcat and the process list).
#   ANDROID_SERIAL=emulator-5554 android/scripts/release-smoke.sh app-release.apk
set -euo pipefail

APK=$1
PKG=io.github.xaviiromf.cliampx

adb uninstall "$PKG" >/dev/null 2>&1 || true
adb install "$APK" >/dev/null
adb shell pm grant "$PKG" android.permission.READ_MEDIA_AUDIO
adb shell pm grant "$PKG" android.permission.POST_NOTIFICATIONS
adb logcat -c
adb shell am start -n "$PKG/.MainActivity" >/dev/null

# The first start unpacks Python and ffmpeg and updates yt-dlp.
for _ in $(seq 1 90); do
	adb shell pidof libcliamp.so >/dev/null 2>&1 && break
	sleep 1
done
sleep 30

bad=$(adb logcat -d | grep -E 'FATAL EXCEPTION|ExceptionInInitializerError|YoutubeDLException|is not a concrete class|CANNOT LINK EXECUTABLE|failed to initialize' || true)
if [[ -n $bad ]]; then
	echo "release smoke: init failure in logcat:" >&2
	echo "$bad" | head -20 >&2
	exit 1
fi
adb shell pidof libcliamp.so >/dev/null || { echo "release smoke: the core is not running" >&2; exit 1; }
echo "release smoke: ok"

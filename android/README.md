# cliamp-x for Android

The Android shell of cliamp-x: a Kotlin app that runs the Go core in a
terminal view. The design is in `PLAN.md`.

The app is one module, `app/`, package `io.github.xaviiromf.cliampx`:

| File | What it does |
|------|--------------|
| `Core.kt` | Where the core lives and the environment it runs in. Writes the first `config.toml` |
| `CliampService.kt` | Foreground service (`mediaPlayback`). Owns the terminal session of the core, the media session and its notification, audio focus, and the IPC link |
| `MainActivity.kt` | The terminal view, the extra keys, back, pinch zoom, deep links and shared URLs |
| `Ipc.kt` | The V2 IPC client (`docs/remote-control.md`) over the core's Unix socket |
| `SettingsActivity.kt` | Sign-in, cookies.txt import, local music permission, battery, updates, yt-dlp update, quit |
| `Updates.kt` | App updates from the GitHub releases: the check through the core, the APK download, its SHA-256 and the system installer |
| `LoginActivity.kt` | YouTube Music sign-in in a WebView, saved as `ytmusic_cookies.txt` |

## Toolchain

| Tool | Version | Where this machine has it |
|------|---------|---------------------------|
| JDK | 17 | `mise install java@temurin-17`, then `export JAVA_HOME=$(mise where java@temurin-17)` |
| Android SDK | platform 36, build-tools 35.0.0, platform-tools | `~/Android/Sdk` |
| Android NDK | r28c (`28.2.13676358`) | `~/Android/Sdk/ndk/28.2.13676358` |
| Emulator | API 36 `google_apis` x86_64 image | `~/Android/Sdk/emulator` |
| Gradle | 8.14.6, through `./gradlew` (pinned with its SHA-256) | downloaded by the wrapper |

Install the SDK pieces with the command-line tools:

```
sdkmanager "platform-tools" "platforms;android-36" "build-tools;35.0.0" "ndk;28.2.13676358"
```

`android/local.properties` holds `sdk.dir` and stays out of git. Set
`ANDROID_HOME` and `ANDROID_NDK_HOME` in the shell for `make android-core`.

## Build and install

From the repository root, build the Go core into
`app/src/main/jniLibs/<abi>/libcliamp.so` (ignored by git), then the APK:

```
make android-core
cd android && ./gradlew assembleDebug
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

The build fails with "run make android-core" when the core of the ABI is
missing. The APK carries one ABI, `arm64-v8a` by default. The Gradle property
`cliampAbi` picks another one.

`versionName` is `git describe --tags` without the `v`, or the Gradle property
`cliampVersion`. `versionCode` is the commit count of `HEAD`, so a CI checkout
needs the full history (`fetch-depth: 0`).

Tests: `./gradlew testDebugUnitTest` runs the JVM tests of the updater, and
`./gradlew lint` the Android lint.

### Release builds and signing

`./gradlew assembleRelease` signs the APK when `ANDROID_KEYSTORE_FILE` (the
path of the keystore), `ANDROID_KEYSTORE_PASSWORD`, `ANDROID_KEY_ALIAS` and
`ANDROID_KEY_PASSWORD` are set, as `.github/workflows/nightly.yml` does with
the secrets of the same names. Without `ANDROID_KEYSTORE_FILE` it builds
`app-release-unsigned.apk`, which Android does not install. With the file but
a missing password variable, Gradle stops and names the variable.

Android installs an update only when it is signed with the same key as the
installed app. The releases and the nightly are signed with the release key;
local debug builds are signed with `~/.android/debug.keystore`. So a debug
build never updates from a release and a release never updates a debug
build: switching between them needs `adb uninstall io.github.xaviiromf.cliampx` first,
which deletes the app data, the YouTube sign-in included (copy
`files/config` out with `adb shell run-as` before, if it matters). Debug
builds of the same machine update each other. Losing the release key means
no future APK can update the installed ones.

### Updates

Settings has "Buscar actualizaciones", the channel (Estable: tags `v*`;
Nightly: also the `nightly` prerelease) and "Actualizar yt-dlp". The app also
checks once a day when it opens and only offers what it finds. A check runs
`libcliamp.so upgrade --check --json [--prerelease]` (`docs/cli.md`), which
compares versions in Go. The app takes `cliamp-x-<version>-<abi>.apk`
(`<version>` without the `v` and the `+` build part) only from
`https://github.com/xaviiromf/cliamp-x/releases/download/` and only with a
SHA-256 from the release's `checksums.txt`. It downloads it into
`cacheDir/updates`, deletes it when the SHA-256 differs, and opens the system
installer, which first asks to allow installs from cliamp-x. Anonymous GitHub
API access allows 60 requests an hour; a check uses two.

To test the download, the SHA-256 check and the installer without a GitHub
release, debug builds have `DebugUpdateActivity`. Build a second debug APK
(for example with `-PcliampVersion=v9.9.9-test`), then:

```
adb push app-debug.apk /sdcard/Android/data/io.github.xaviiromf.cliampx/files/update.apk
adb shell am start -n io.github.xaviiromf.cliampx/.DebugUpdateActivity --es sha256 $(sha256sum app-debug.apk | cut -d' ' -f1)
adb logcat -s DebugUpdate
```

A wrong `sha256` logs the mismatch and leaves `cache/updates` empty.

### Nightly

`.github/workflows/nightly.yml` runs on every push to `android` and `main`
(and by hand). It builds the arm64 core and the signed release APK, and
replaces the prerelease `nightly`, moving its tag to the commit. The version
is `v<next patch>-nightly.<yyyymmdd>.<commit count>+<short sha>`: after
v2.2.1 that is `v2.2.2-nightly.20261013.1700+abc1234`, newer than v2.2.1 and
older than v2.2.2, and the commit count orders two nightlies of one day as
`versionCode` does. The release is titled with the version, which
`upgrade --check` reads because the tag is not one. It fails without the four
signing secrets instead of publishing an APK that cannot update.

### Emulator

An arm64 Go program crashes under the ARM translation of the x86_64 emulator,
even a hello world, while C programs run. Build the x86_64 core and APK for
the emulator:

```
make android-core ANDROID_ABI=x86_64
cd android && ./gradlew -PcliampAbi=x86_64 assembleDebug
```

## How the app runs the core

`CliampService` starts `<nativeLibraryDir>/libcliamp.so` with no shell in a
pseudo-terminal, as Termux does. Android 10 and later run programs only from
`nativeLibraryDir`, so the core is packaged as a library and the APK sets
`useLegacyPackaging` (extractNativeLibs). The environment:

| Variable | Value |
|----------|-------|
| `HOME` | `<filesDir>/home` |
| `CLIAMP_CONFIG_DIR` | `<filesDir>/config`. It holds `config.toml`, `ytmusic_cookies.txt`, the log and the IPC socket `cliamp-x.sock` |
| `XDG_DATA_HOME`, `XDG_CACHE_HOME`, `TMPDIR` | `<filesDir>/data`, `<cacheDir>`, `<cacheDir>/tmp` |
| `TERM`, `COLORTERM` | `xterm-256color`, `truecolor` |
| `LANG`, `TZ` | From the system locale and time zone |
| `PATH` | `<nativeLibraryDir>` before the system `PATH` |
| `CLIAMP_YTDLP` | `libpython.so` running the yt-dlp zipapp of youtubedl-android, with `--js-runtimes quickjs:<nativeLibraryDir>/libqjs.so` |
| `CLIAMP_FFMPEG` | `<nativeLibraryDir>/libffmpeg.so` |
| `CLIAMP_YTDLP_ENV`, `CLIAMP_FFMPEG_ENV` | `LD_LIBRARY_PATH`, `SSL_CERT_FILE` and, for Python, `PYTHONHOME` and `HOME`, separated by the byte `0x1f` |

The paths copy what `YoutubeDL.kt` and `FFmpeg.kt` of youtubedl-android 0.18.1
do. Before the first start the app calls their `init`, which unpacks Python
and ffmpeg into `noBackupFilesDir/youtubedl-android`. That takes a few seconds
once.

The first start writes this `config.toml`:

```toml
mouse = true
music_dir = "/storage/emulated/0/Music"
```

It leaves `low_power` off so the visualizers keep their full frame rate
while the app is visible. Closing the app leaves the core playing. When the
app goes to the background it sends `ui.render off`, which stops drawing,
and `ui.render on` when it comes back.
Quit, in the notification or in the settings (`⋮`), sends the core SIGTERM so
it saves the resume position, and kills it after 5 seconds.
`cliamp-x://` links and URLs shared to the app run
`libcliamp.so open <link>` once the core listens; a shared URL becomes
`cliamp-x://play?url=<url>`.

Debug the core with its log:

```
adb shell run-as io.github.xaviiromf.cliampx cat files/config/cliamp.log
```

## Licenses

The terminal view (`com.github.termux.termux-app:terminal-view` and
`:terminal-emulator`) is Apache 2.0. youtubedl-android
(`io.github.junkfood02.youtubedl-android`) is GPL-3.0, so the APK as a whole
is distributed under the GPL-3.0. It bundles Python, yt-dlp, QuickJS and
ffmpeg under their own licenses.

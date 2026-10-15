# cliamp-x for iPhone: plan

Branch `iOS`, created from `android` on 2026-10-15. Goal and rules:
`ios/CLAUDE.md`. Design contract: `android/UI-SPEC.md`. Features:
`android/FINAL-PLAN.md` §3.

## 1. Architecture

### How Android does it (the reference)

```
Compose UI ──IPC V2 (Unix socket)──> libcliamp.so (Go core, own process, in a pty)
                                        ├─ yt-dlp  (Python + QuickJS, child process)
                                        └─ ffmpeg  (child process, decodes to PCM)
```

### Why iOS cannot copy it

iOS apps cannot start other processes (`fork`/`exec`/`posix_spawn` are
denied). So on iPhone:

1. The Go core cannot be a separate executable: it must be a **static
   library linked into the app**, running in the app's process.
2. **yt-dlp cannot run as a child process**, and neither can a JavaScript
   runtime binary (QuickJS, deno).
3. **ffmpeg cannot run as a child process.**

### Target architecture

```
SwiftUI app (Swift 6, iOS 17+)
  ├─ Remote.swift ──IPC V2 (Unix socket in the app container)──┐
  └─ CliampCore.start(configDir:) ─────────────────────────────┤
                                                                v
         libcliamp.a (Go core, c-archive, same process, goroutines)
           ├─ YouTube stream resolution: in-process (spike S2)
           └─ decoding: in-process (spike S3)
```

- The core is built with `GOOS=ios GOARCH=arm64 CGO_ENABLED=1
  go build -buildmode=c-archive` into `libcliamp.a` plus its header, and
  linked from SwiftPM (a C/binary target; spike S1 confirms what xtool
  accepts). An exported Go function (in a new `main_ios.go` or
  `cmd/ios/`) starts the same `run(...)` path the desktop and Android use,
  with the config, data and cache directories of the app container passed
  in, in a goroutine.
- The UI keeps talking IPC V2 over the Unix socket, exactly like
  `Remote.kt`. That keeps one contract for both apps and lets the Swift
  client port `Remote.kt` and `Models.kt` almost line by line.
- The core runs **without a terminal**. Today headless mode (`--daemon`)
  removes the `theme`, `vis` and `ui.render` operations, which the native
  UI needs. The smallest core change is an embedded mode: headless, but
  keeping `theme` and `vis` (and `vis.frame`). Write it down in
  `ios/UPSTREAM.md`.
- Audio output: the core plays through `beep` and `oto`, and oto has an
  iOS backend (AudioToolbox). Swift sets `AVAudioSession` to `.playback`
  before starting the core, and `Info.plist` declares
  `UIBackgroundModes: [audio]`, so music keeps playing with the screen
  locked.

## 2. Spikes first (do these before any UI)

Each spike is a small, throwaway-or-keep experiment on a real iPhone,
with a short written result in `ios/SPIKES.md` (what worked, numbers,
the decision). Stop and report to the maintainer if a spike fails all its
options.

### S1. Core in-process

- Build `libcliamp.a` for `ios/arm64` from Linux (the Darwin SDK that
  `xtool setup` installs; find the clang and SDK paths it uses) and link
  it into a minimal xtool app.
- The app starts the core, connects to the socket, sends `state.get` and
  plays a local MP3 bundled in the app.
- Pass: audio plays, `state.get` answers, background playback continues
  with the screen locked for 2 minutes.

### S2. YouTube streams without yt-dlp as a process

Options, in order of preference:

- **A. Go-native.** `go.mod` already has `github.com/kkdai/youtube/v2`
  and, through it, `github.com/dop251/goja` (a pure-Go JavaScript engine).
  Resolve the audio URL in Go: InnerTube player request, signature and
  `n` challenge solved with goja, cookies from `ytmusic_cookies.txt` when
  the cookieless clients fail (the Android rule in `player/ytdl.go`).
  Only for `GOOS=ios` at first.
- **B. Embedded Python.** BeeWare's Python-Apple-support XCFramework runs
  yt-dlp in-process. The JS challenge then needs JavaScriptCore through a
  custom yt-dlp JS provider, because yt-dlp runs runtimes as processes.
- **C. Swift-side resolution** with JavaScriptCore, handing the URL to the
  core with `url.load`.

Pass: a signed-in YouTube Music song and a public video play, start in
under 8 s, and survive a seek. Note what breaks when YouTube changes its
player (A and C need code updates; B needs a yt-dlp update).

### S3. Decoding without ffmpeg as a process

Options:

- **A. AudioToolbox / AVFoundation.** Prefer YouTube's AAC (`m4a`,
  itag 140) on iOS, decode it to PCM with `AVAssetReader` or
  `AudioConverter` through a small cgo or Swift bridge, and feed the core's
  pipeline. Opus stays unsupported until needed.
- **B. FFmpeg libraries** (libavformat, libavcodec) built as static
  libraries for iOS and called from Go with cgo. More work, and LGPL
  linking rules apply.

Pass: AAC and the local formats the core supports today play gapless, and
the 10-band EQ and the visualizer still work.

### S4. The UI shell

A SwiftUI app with the three tabs, the mini-player and the full player
sheet talking to the core over IPC. It proves the client port and the
look (theme palette from `snapshot.theme`, the bundled fonts, glyphs).

## 3. Shared contracts (one source, both apps)

Read these from where they are; generate any iOS copy with a script in
`ios/scripts/`.

| What | Where |
|------|-------|
| Design: screens, tokens, glyphs, gestures, states | `android/UI-SPEC.md` (+ §12 addenda) |
| Feature list | `android/FINAL-PLAN.md` §3 |
| IPC V2 protocol and result shapes | `docs/remote-control.md`, `ipc/protocol.go`, `ipc/v2.go` |
| Client logic to port | `android/.../Remote.kt`, `Models.kt`, `Ytdlp.kt` (watch logic), `Updates.kt` (version compare comes from the core) |
| Themes (77) | `theme/themes/*.toml`, read through the core (`theme list`, `snapshot.theme`) |
| Fonts and their licenses | `android/app/src/main/assets/fonts.json`, `android/app/src/main/res/font/`, `android/app/src/main/assets/licenses/` |
| UI strings (es, en) | `android/app/src/main/res/values/strings.xml` (English) and `values-es/strings.xml` (Spanish). Convert to a String Catalog with a script |
| Backgrounds (17 shaders) | `android/.../ui/Shaders.kt` (AGSL). Port each to a Metal shader used with SwiftUI `.colorEffect` / `.layerEffect`; keep the same names, uniforms and palette mapping |
| Visualizers | The core draws them: `vis.frame` (same as Android) |
| Skins | `android/.../ui/Skins.kt` (the built-in presets) |
| Personalization export format | `class Personalization` in `android/.../ui/Skins.kt` (the JSON must import on both apps) |
| Icons and logo | `android/app/src/main/res/mipmap-anydpi/`, `drawable/` |

## 4. Android to iOS parity

| Android | iOS |
|---------|-----|
| Compose UI | SwiftUI |
| `CliampService` (foreground service, pty) | Core started in-process at launch; `AVAudioSession` `.playback`; `UIBackgroundModes: audio` |
| MediaSession, notification, lock screen | `MPNowPlayingInfoCenter` + `MPRemoteCommandCenter` |
| Audio focus, becoming noisy | `AVAudioSession` interruption and route-change notifications |
| WebView sign-in, cookies.txt export | `WKWebView` + `WKHTTPCookieStore`, written as Netscape `ytmusic_cookies.txt` |
| Import cookies.txt, export/import personalization | `UIDocumentPicker` / `.fileImporter` / `.fileExporter` |
| Local music folder (`READ_MEDIA_AUDIO`) | The app's Documents folder, shown in Files (`UIFileSharingEnabled`, `LSSupportsOpeningDocumentsInPlace`), plus import from Files |
| Modo terminal (TUI in a pty) | Later and optional: SwiftTerm over an in-process pseudo-terminal. Skip in v1 if S1 shows it is hard |
| Android Auto | Out of scope: CarPlay audio apps need an Apple entitlement that sideloaded apps cannot get |
| Home-screen widget | WidgetKit extension, only if xtool supports app extensions (check) |
| Quick Settings tile | Control Center control (iOS 18 `ControlWidget`), same extension condition |
| Launcher shortcuts | Home-screen quick actions (`UIApplicationShortcutItems`) |
| Alternative launcher icons | `UIApplication.setAlternateIconName` |
| Battery optimization screen | Not needed |
| In-app APK updater | No self-update. Settings shows the installed version and links to the GitHub release; optionally an AltStore/SideStore source JSON later |
| yt-dlp self-update | Depends on S2: option B keeps it, options A and C ship updates with the app |
| AGSL shaders | Metal shaders |
| Haptics | `sensoryFeedback` |
| TalkBack, font scale | VoiceOver, Dynamic Type |

## 5. Layout of `ios/`

```
ios/
  CLAUDE.md  AGENTS.md -> CLAUDE.md
  PLAN.md  README.md  SPIKES.md  UPSTREAM.md
  Package.swift        # SwiftPM: app target + CliampCoreLogic (testable, no UI)
  xtool.yml            # bundleID io.github.xaviiromf.cliampx, Info.plist, entitlements
  Sources/CliampX/     # SwiftUI app
  Sources/CliampCoreLogic/  # IPC client, models, snapshot merge (ported from Kotlin, tested)
  Tests/CliampCoreLogicTests/
  Resources/           # fonts, generated strings, shaders, icons
  scripts/             # build-core.sh (libcliamp.a), gen-strings, copy-fonts
```

Create it with `xtool new` and adapt; keep the names above.

## 6. Milestones

| # | Scope | Done when |
|---|-------|-----------|
| I0 | Spikes S1-S4 | `ios/SPIKES.md` has a decision for each, the maintainer agreed |
| I1 | Foundation: core build script, in-process start, `Remote.swift`, models, snapshot merge, theme and fonts, splash, tabs, mini-player, search that plays | A YouTube Music song plays from Buscar on the iPhone, in background too |
| I2 | Player: full player, gestures, seek, queue (swipe, drag), EQ, lyrics, visualizer (`vis.frame`), sleep, speed | UI-SPEC §2.2-2.8 on device |
| I3 | Browse: Inicio, Buscar, album, artist, playlist, genre, Para ti, remixes, action sheets | UI-SPEC §2.1, §2.4, §2.6 |
| I4 | Library and Settings: Biblioteca, downloads, ratings, stats, Wrapped, all of Ajustes (themes, fonts, backgrounds, motion, skins, theme editor, dynamic theme, export/import, quality, Premium, scrobbling, auto cache) | FINAL-PLAN T2-T8 equivalents |
| I5 | iOS integrations: lock screen, remote commands, quick actions, alternate icons, widget and control (if extensions work), onboarding, English, VoiceOver, Dynamic Type, iPad layout | Parity table §4 |
| I6 | Polish and release: performance (Instruments is macOS-only; measure frame pacing with `CADisplayLink` logs), sideload guide in `docs/ios.md`, version shown in Settings | The maintainer installs it with xtool from the branch |

Commit at every working step; each milestone ends with a short report in
`ios/SPIKES.md` or the commit bodies: what works, what was verified on the
device, what is left.

## 7. Known risks

- S2 is the hardest part: YouTube changes its player often. Whatever the
  option, keep the resolution code in one place and test it on device
  with a signed-in account and without one.
- xtool limits: app extensions (widget, control), binary targets for a
  static library, and entitlements must be checked early (S1, I5).
- Free Apple ID signing expires after 7 days and allows few apps; a paid
  developer account (99 USD a year) lasts a year. That is the
  collaborator's choice, not the code's.
- Opus audio: YouTube serves AAC too, so S3 option A starts with AAC.

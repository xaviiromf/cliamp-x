# cliamp-x Android: final stage plan (v1.0)

Status: approved by the user on 2026-10-14 ("quiero que lo hagas todo").
Base: branch `android` at 67fdd748. Integration branch: `android-final`
(worktree `.worktrees/final`). Contract for everything not changed here:
`android/UI-SPEC.md`. CLAUDE.md conventions apply everywhere.

## 0. Checkpoint rules (the session may be interrupted at any time)

- Each task `Tn` works in its own worktree `.worktrees/Tn` on branch
  `final-Tn`, created from `android-final` by the orchestrator or the wave
  integrator. Commit early and often: every commit is a checkpoint.
- The task's LAST implementation commit has the line `Task: Tn done` in its
  body. A fix commit after review has `Task: Tn fixed`.
- Ledger directory: `~/Downloads/android-cmdline/final/ledger/`.
  The reviewer writes `Tn-review.json` (`{"pass":bool,"failures":[...],
  "head":"<sha>"}`). The integrator writes `wave-W.json` when the wave is
  merged into `android-final`.
- Every agent first checks the ledger and `git log` of its branch. Work that
  is already recorded as done is not redone: an implementer whose branch has
  `Task: Tn done` continues only what is missing, a reviewer whose ledger file
  exists returns it, an integrator skips branches already merged
  (`git merge-base --is-ancestor`).
- Never push. Never use bare `git stash`. Never edit outside your worktree.

## 1. Shared resources (15 GB RAM, one emulator, one phone)

- **Build slots (max 2 Gradle builds at once).** Before any `./gradlew` run,
  take a slot: `until mkdir ~/Downloads/android-cmdline/final/locks/build1 2>/dev/null || mkdir ~/Downloads/android-cmdline/final/locks/build2 2>/dev/null; do sleep 20; done`,
  remember which one you got, and `rmdir` it right after the Gradle command
  ends (also on failure). Run Gradle with
  `-Pkotlin.compiler.execution.strategy=in-process` and finish each session
  with `./gradlew --stop`. A slot older than 60 minutes is stale: remove it.
- **Device locks.** `emulator-5554` (x86_64 Android 16, no Google account; arm64
  Go binaries crash there, use `make android-core ANDROID_ABI=x86_64` and
  `-PcliampAbi=x86_64`) and `<phone-serial>` (the user's Samsung A55, arm64,
  signed in to YouTube Music; install only with
  `adb -s <phone-serial> install --user 0 -r`, never uninstall or clear data).
  Take the device lock before installing or driving it:
  `until mkdir ~/Downloads/android-cmdline/final/locks/<serial> 2>/dev/null; do sleep 20; done`
  and release it (`rmdir`) when your device session ends; keep sessions
  short (install, test, screenshot, release). A lock older than 45 minutes
  is stale. Always pass `-s <serial>` to adb.
- **Phone unlock** if it is locked or dark: `input keyevent KEYCODE_WAKEUP`,
  `input swipe 540 1900 540 700 300`, `input text <PIN>`,
  `input keyevent KEYCODE_ENTER`. The PIN is given in your prompt; never
  write it to a file, commit, log, screenshot name or report.
- Screenshots go to `~/Downloads/android-cmdline/final/shots/Tn/`.
- Toolchain: `JAVA_HOME=$(mise where java@temurin-17)`,
  `ANDROID_HOME=$HOME/Android/Sdk`,
  `ANDROID_NDK_HOME=$HOME/Android/Sdk/ndk/28.2.13676358`, Go through
  `mise exec --`.
- Commit rule: `git add -A && mise exec -- make ci`, `mise exec -- make
  android-core`, and from `android/` `./gradlew testDebugUnitTest lint
  assembleDebug` must pass before each commit that touches the code they
  cover (Go-only commits skip Gradle; Android-only commits still run make ci,
  which is quick to fail on nothing).

## 2. Waves

| Wave | Tasks (parallel) | Integrate after |
|------|------------------|-----------------|
| 1 | T1, T2, T3, T4 | yes |
| 2 | T5, T6, T7, T8 | yes |
| 3 | T9, T11 | yes |
| 4 | T10, T12 | yes |
| 5 | T13 acceptance on the phone, then one fix round | merge into `android` |

Ownership inside a wave is listed per task. A task that needs a file owned by
another task of the same wave makes the smallest additive change (new
string entries, a new manifest entry) and expects the integrator to merge.

## 3. Tasks

### T1. Terminal-identical visualizers (wave 1, emulator)
- Replace the 34 native renderers with the core's own visualizer frames, so
  every terminal mode (about 47, including Wave, Scope, Stereo, Binary,
  Cover modes) looks exactly as in Modo terminal.
- Go: new IPC operation `vis.frame` (params `value` = cols? use existing
  fields: `limit` = cols, `offset` = rows, or add fields per CLAUDE.md "To
  add an operation"). It sizes the core visualizer to cols x rows, ticks it
  like `v2BandsResponse` does when render is paused or headless, and returns
  the frame as lines with ANSI SGR colors (`Response.Lines []string`) plus
  the mode name. Headless and paused rendering must work. Table-driven tests,
  docs in `docs/remote-control.md`.
- Android: draw the frame on one Canvas in the selected font, parsing SGR
  truecolor/256/16 colors into the theme palette; a cell grid sized from the
  view and font metrics; poll only while visible and playing, at most 30 fps,
  conflated. Picker lists every mode from `vis list`. Tap cycles all modes.
  Delete the 34 native renderers and their tests, keep the strip and the
  full-player placements. Reduced motion: lower the rate to 10 fps.
- Owns: Go `ipc/`, `ui/model/ipc_runtime.go` (the new op), `ui/` visualizer
  code if needed, Android `Vis.kt`, `ui/Visualizer.kt`, the visualizer
  picker in `ui/Settings.kt`.

### T2. Fonts and shape (wave 1, emulator)
- Font system: JetBrains Mono Nerd Font and Adwaita Mono bundled (Regular and
  Bold, OFL), plus at least 15 downloadable monospace fonts with free
  licenses: Fira Code, Iosevka, Hack, Cascadia Code, Source Code Pro, IBM
  Plex Mono, Ubuntu Mono, Victor Mono, Space Mono, Monaspace Neon, Commit
  Mono, Geist Mono, Departure Mono, VT323, Press Start 2P, Terminus (TTF),
  Inconsolata, Mononoki, Fantasque Sans Mono. A JSON asset pins each
  download URL (official release or the google/fonts repository) and its
  SHA-256; downloads go to app storage, verified, with progress and errors.
  License texts in assets/licenses.
- Font picker in Ajustes with a live preview row; the font applies to the
  whole UI and to the visualizer grid of T1.
- Item 10: text size (4 steps), list density (compacta/normal/amplia),
  corners (cuadradas/redondeadas), haptics on/off.
- Owns: `ui/Theme.kt` typography, new `ui/Fonts.kt`, font assets, the font
  and shape rows of Ajustes.

### T3. Backgrounds and transitions (wave 1, phone)
- At least 15 backgrounds: Sólido (default), CRT, Ondas, Cyberpunk, Retro
  (synthwave sun and grid), Glitch, Matrix, Estrellas, Plasma, Aurora, VHS,
  Tramado (Bayer dither), Tron (grid), Bokeh, Circuito, Topográfico, Grano.
  AGSL `RuntimeShader` on API 33+ with a static palette gradient fallback
  below. Uniforms come from the active theme palette (bg, accent, fg,
  green, yellow, red), so every theme recolors them. Drawn once behind the
  whole app, including the expanded player and sheets; surfaces become
  translucent enough to show it while keeping text contrast (a scrim).
- Intensidad del fondo slider (0-100) and separate intensity for lists vs
  the player (item 2 of the idea list).
- Animation pauses when the app is not visible, with the screen off, and
  slows with reduced motion. Measure `dumpsys gfxinfo` on the phone with
  each background.
- Transitions (item 4): replace abrupt or bouncy motion (springs, fast
  slides, sheet snaps) with eased fades and slides (FastOutSlowIn or
  emphasized easing, 250-400 ms), cross-fade of artwork and palette, and a
  speed setting: Lenta, Normal, Sin animación.
- Owns: new `ui/Backgrounds.kt` and shader sources, `ui/App.kt` root
  composition, motion helpers, the background and motion rows of Ajustes.

### T4. Core features for the app (wave 1, Go only, no device)
- **Quality per network:** `settings.set` names `quality_wifi`,
  `quality_mobile` with values `alta`, `normal`, `ahorro` that map to the
  yt-dlp format selection (for example `ahorro` caps `abr<=64`). The app
  tells the core the current network with `settings.set network wifi|mobile`.
- **Calidad Premium (Android):** `settings.set premium_quality on|off`. On
  turns off the cookieless-first path so a Premium account gets its higher
  quality formats.
- **Now-playing format:** snapshot fields `audio_codec`, `audio_kbps`,
  `audio_rate` for the playing stream.
- **Scrobbling built into the core:** new `internal/scrobble` with
  ListenBrainz (user token) and Last.fm (API key and secret given by the
  user, session from the token flow), fed by the existing track start and
  scrobble reports (`ui/model/notifications.go`, `report_queue.go`), with an
  offline retry queue on disk. Config keys in `config.toml.example` and
  docs; `settings.set` names to set tokens; desktop works too.
- **Wrapped data:** `stats` gets `name:"wrapped"` (a year: top artists,
  songs, albums, genres, minutes, streaks, first and most replayed song).
- **Auto cache support:** an operation the app can call to download the
  liked songs and the recently played ones (reuse `download`), with a limit
  in songs and MB, skipping ones already offline.
- Owns: Go only. Docs in `docs/remote-control.md`, `docs/configuration.md`,
  `docs/youtube-music.md`.

### T5. Skins, theme editor, dynamic theme, export/import (wave 2, emulator)
- Skins: a preset bundles theme, font, background, intensity and visualizer
  mode. At least 8 built in (CRT ámbar, Synthwave, Matrix, Game Boy, Winamp
  clásico, Nord sereno, Papel claro, Vaporwave) and user skins (save
  current, rename, delete).
- Theme editor: edit the palette keys with a color picker and live preview,
  save as a TOML in the core's themes directory (desktop-compatible), then
  select it.
- Dynamic theme: from the cover (the existing cover theme) or from the system
  wallpaper colors (WallpaperManager), generated as a palette.
- Export/import all personalization (skins, user themes, font, background,
  motion, shape) as one JSON file through the system file picker.

### T6. Player experiences and brand (wave 2, phone)
- Full-screen visualizer as the expanded player's background (behind the
  artwork, toggle), using T1 frames.
- Karaoke lyrics: full-screen synced lyrics over the background, current line
  highlighted, auto-scroll, tap a line to seek.
- Stereo mode (item 14): while charging or from the menu, an always-on screen
  with clock, artwork, title and visualizer, night dimming, burn-in shift.
- Notification and lock screen in the theme colors.
- Animated splash with the logo letter by letter.
- Alternative launcher icons (verde, ámbar, synthwave, Game Boy) through
  activity aliases, plus the themed monochrome icon of Android 13+.

### T7. Android integrations and audio settings (wave 2, phone)
- Android Auto: a MediaBrowserService tree (Inicio, Biblioteca, Me gusta,
  Recientes, búsqueda por voz) on the existing MediaSession.
- Home-screen widget with a mini player in the cliamp-x look.
- Quick Settings tile (play/pause) and launcher shortcuts (Buscar, Me gusta,
  Reanudar).
- Share song (YouTube Music link) from the action sheet and the player.
- Resume when headphones or Bluetooth connect (opt-in).
- EQ per output: remember a preset per route (speaker, wired, each Bluetooth
  device) and switch it automatically.
- Ajustes UI for T4: quality per network, Calidad Premium, auto cache (Wi-Fi
  only, limits), downloads Wi-Fi only, scrobbling accounts; network
  monitoring that tells the core; the codec/kbps line in the player.

### T8. Wrapped (wave 2, emulator)
- A "Tu año en cliamp-x" story (swipeable cards) from `stats wrapped`, in the
  theme and font, and export of each card as a PNG to share.

### T9. Onboarding and English (wave 3, emulator)
- First-run onboarding: permissions, sign-in (or skip), pick a skin.
- Full English translation: `values/strings.xml` in English as the default,
  `values-es/` Spanish; the app follows the system language, with an
  override in Ajustes. Fix every hard-coded string.

### T10. Accessibility and tablets (wave 4, emulator)
- TalkBack: content descriptions, roles, state descriptions, focus order,
  custom actions for swipe gestures (skip, remove from queue), live regions
  for now-playing; large fonts up to 200 % without clipping.
- Landscape and tablet layouts (two panes on wide screens), removing the
  portrait lock on large screens only.

### T11. Release quality (wave 3, emulator, no UI overlap with T9)
- About screen: version, GPL-3 for the APK (youtubedl-android), MIT for the
  source, third-party licenses (fonts OFL, termux Apache 2), privacy policy
  (no data collected), link to the source.
- LICENSE: add the user's copyright line for new work, keep Bjarne Øverli's.
  README: state that cliamp-x is an independent project, not official cliamp.
- "Enviar registro": export cliamp.log plus the app's logcat and crash
  buffer to a file and the share sheet; no telemetry.
- R8 minify and resource shrinking for release, with keep rules; verify the
  release build runs.
- Fastlane metadata (es and en: title, short and full description,
  changelog, screenshots from the phone) for IzzyOnDroid and F-Droid, and
  notes for reproducible builds.
- UI tests in CI: a GitHub Actions job running the emulator smoke tests.

### T12. Smoothness on the A55 (wave 4, phone)
- Find the cause of the janky frames (25 % in the player) with a system
  trace (`perfetto` via adb) and gfxinfo, fix it, target under 5 % janky and
  90th percentile at or under 16 ms in the player and lists.

### T13. Final acceptance (wave 5, phone)
- Every feature of this plan and the earlier acceptance checklist on the
  user's phone, then one fix round, then merge into `android`.

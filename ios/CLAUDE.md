# CLAUDE.md for the iOS branch

Rules for any AI agent (Claude Code, Codex, others) working on the `iOS`
branch. `ios/AGENTS.md` is a symlink to this file. The root `CLAUDE.md`
still applies to everything outside `ios/`; read it too.

## The goal

Build the cliamp-x iPhone app: **the same app as the Android one**
(`android/`), with the same screens, features, look and behavior, written
natively for iOS in Swift and SwiftUI, built and installed with
[xtool](https://github.com/xtool-org/xtool) (SwiftPM, no Xcode project).

"The same app" means:

- The same navigation, screens, texts (Spanish and English), gestures,
  themes, fonts, backgrounds, visualizers, skins and settings as Android.
- The same Go core (`libcliamp`) doing the music work: YouTube Music,
  local files, queue, EQ, lyrics, stats, scrobbling. The UI talks to it
  through the same IPC V2 protocol the Android app uses.
- Where iOS has no equivalent (Android Auto, launcher shortcuts, APK
  updates), follow the parity table in `ios/PLAN.md` §4. Never invent a
  replacement that the table does not list without writing it down there
  first.

## Read before you write anything

1. `ios/PLAN.md` (this branch's plan: architecture, spikes, parity table,
   milestones). Do the §2 spikes before any UI work.
2. `android/UI-SPEC.md`: the design contract (screens, tokens, glyphs,
   gestures, states). The Android code is the reference implementation
   when the spec is ambiguous.
3. `android/FINAL-PLAN.md` §3: the full feature list (T1-T12).
4. `docs/remote-control.md`: the IPC V2 protocol, every operation and
   result shape the UI uses.
5. `android/app/src/main/java/io/github/xaviiromf/cliampx/Remote.kt` and
   `Models.kt`: how a client uses IPC (connection, jobs, subscribe,
   snapshot merge, revision floor). Port its logic, not its Kotlin shape.

## Hard rules

- **Never modify `android/`.** It is the reference and it ships from the
  `android` branch. Read it, port it, do not edit it.
- **Go core changes are allowed only when iOS needs them**, and then:
  - put iOS-only code in files with the `_ios.go` suffix, and remember
    that `GOOS=ios` also matches the `darwin` build tag (as `android`
    matches `linux`): a darwin-only file needs `&& !ios` if iOS must not
    build it;
  - keep desktop and Android unchanged: `mise exec -- make ci` and
    `mise exec -- make android-core` must still pass before every commit;
  - list every core change in `ios/UPSTREAM.md` (what, why, files), so the
    maintainer can bring it to `android` and `main`.
- **Shared contracts stay shared.** Do not fork or copy-edit these; read
  them from where they live (§3 of `ios/PLAN.md` lists them): the IPC
  protocol, `theme/themes/*.toml`, `android/app/src/main/assets/fonts.json`
  and its licenses, the strings in `android/app/src/main/res/values*/`
  and the design spec. If the iOS build needs a copy (fonts, strings),
  generate it with a script in `ios/scripts/` from the shared source, never
  by hand.
- **Secrets never enter git:** Apple ID, passwords, certificates (`*.p12`,
  `*.cer`), provisioning profiles (`*.mobileprovision`), xtool auth
  files, YouTube cookies, device UDIDs, personal paths. `ios/.gitignore`
  covers the usual ones; check `git status` before each commit.
- **Do not touch the GitHub Actions workflows** that sign Android
  releases (`release.yml`, `nightly.yml`) or any secret.
- **Bundle ID:** `io.github.xaviiromf.cliampx` (the same as Android).
- **Minimum iOS:** 17.0 (SwiftUI shader effects, Observation).
- **No App Store and no TestFlight.** The app downloads YouTube audio; it
  is installed by sideloading only (xtool, AltStore or SideStore).

## Conventions

- Commits: subject `area: what changed` (50 characters at most, e.g.
  `ios: player screen`), and a body that says what was wrong or missing,
  what the change does and how it was verified. One coherent step per
  commit; commit often.
- Swift: Swift 6 language mode, SwiftUI, Observation (`@Observable`),
  async/await. No third-party dependencies unless `ios/PLAN.md` names
  them; ask the maintainer before adding one.
- Smallest change that fully works; no speculative abstractions; never
  cut validation, security (verify every download hash), error handling
  or accessibility (VoiceOver labels, Dynamic Type).
- Every non-trivial pure function gets a test (`swift test` for the
  `CliampCore` logic target; see `ios/PLAN.md` §5).
- UI text in Spanish and English, from the shared strings.

## Verify before you say it works

- `swift build` / `xtool dev build` must pass.
- Test on a real iPhone with `xtool dev` (the simulator needs macOS; on
  Linux use a device). Look at the result: screenshots or a screen
  recording when a visual feature changed.
- For any core change: `mise exec -- make ci` and
  `mise exec -- make android-core` (needs the Android NDK; if the
  machine has none, say so in the commit body and ask the maintainer to
  run it).
- Report honestly what you could not verify.

## Keeping up with Android

The maintainer keeps working on `android`. Merge it into `iOS` with
`git merge android` (never rebase a shared branch) when new features land,
then port them. Resolve conflicts in favor of `android` for everything
outside `ios/`.

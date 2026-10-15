# cliamp-x for iPhone: getting started

Branch `iOS`. Rules for AI agents: `ios/CLAUDE.md`. Plan: `ios/PLAN.md`.

## What you need

- A Linux (or WSL, or macOS) machine with
  [xtool](https://github.com/xtool-org/xtool) installed. Run `xtool setup`
  once: it asks for `Xcode.xip` (download it from Apple's developer site
  with any Apple ID) and installs the Darwin Swift SDK from it.
- An iPhone with Developer Mode on (Settings → Privacy & Security →
  Developer Mode), connected by USB.
- An Apple ID for signing (`xtool auth`). A free Apple ID works; its
  installs expire after 7 days.
- Go 1.26.9 through mise (`mise.toml` pins it), for the core:
  `mise exec -- go version`.
- Optional, for core changes: the Android NDK, to check that
  `make android-core` still builds (see `android/README.md`).

## Everyday commands

```
cd ios
xtool dev build        # build the app
xtool dev              # build, install and run on the connected iPhone
swift test             # tests of the platform-free logic (CliampCoreLogic)
```

Before the app exists, start with the spikes in `ios/PLAN.md` §2.

## Never commit

Apple ID data, certificates, provisioning profiles, xtool auth files,
YouTube cookies or device identifiers. `ios/.gitignore` lists them; check
`git status` before every commit.

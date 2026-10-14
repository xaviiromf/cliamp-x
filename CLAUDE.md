# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> cliamp-x is a retro terminal music player (Go + Bubbletea). This file tells AI agents where things live, what conventions the codebase uses, and which skills to lean on.

`AGENTS.md` is a symlink to this file. Edit `CLAUDE.md` only.

---

## What cliamp-x is

A fork of [cliamp](https://github.com/bjarneo/cliamp) by Bjarne Øverli: a TUI music player inspired by Winamp, cut down to YouTube Music plus local files, for Linux and Android. It plays YouTube Music with the user's own browser session (audio through yt-dlp, no ads), local files, `ssh://` paths, HTTP streams, HLS and RSS feeds. It has built-in visualizers, a 10-band parametric EQ, synced lyrics, Lua plugins, V2 IPC remote control, a headless mode and MPRIS media controls. The user docs are in Spanish for `README.md` and `docs/youtube-music.md`, English elsewhere.

- Install: `./scripts/install.sh`, or `make build` / `make install`.
- Module: `github.com/xaviiromf/cliamp-x`. Package `main` sits at the repo root.
- Binary: `cliamp-x`. The Lua plugin global stays `cliamp`.
- Entry point: `main.go` → `run(...)`. The TUI and headless mode both start there.
- CLI: `urfave/cli/v3` in `commands.go`.

---

## Architecture map

Package `main` at the repo root:

| File | Responsibility |
|------|----------------|
| `main.go` | `run(...)` for the TUI and headless mode: config, player, playlist, Model, Bubbletea program, IPC server and exit save. It also holds the V2 dispatcher `newV2Dispatcher`, the operation set `v2Operations` and the plugin jobs |
| `providers.go` | The provider registry: `registerProvider`, `providerFactory`, `providerEnv`, `buildProviders` (recovers a factory's panic) and `providerKeys`, which `--provider` reads. The player hooks `registerPlayerHooks` and `isBufferedProviderURL`. The sign-in URL observers and the yt-dlp install prompt |
| `provider_<key>.go` | One file per provider with its `init` that calls `registerProvider`: `provider_ytmusic.go` (order 10, `Y`) and `provider_local.go` (order 100, `L`). A fork adds a provider with a new file only. See `docs/adding-providers.md` |
| `lua_wiring.go` | The Lua state, control and UI providers. `newLuaSender` queues the plugin messages for `prog.Send`, so a plugin never waits on the event loop |
| `commands.go` | The root flags and every subcommand |
| `ipc_client.go` | The V2 client of the subcommands, `cliamp-x open` and `cliamp-x remote`. Each call waits a bounded time |
| `open.go` | `cliamp-x open` for `cliamp-x://` links. It maps each parsed link to a fixed set of operations |
| `waybar.go` | `cliamp-x waybar`: a Waybar custom module that prints one JSON line per playback change from `state.get` and the IPC events |
| `main_darwin.go` | Locks the main goroutine to the main OS thread on macOS with cgo |
| `main_android.go` | Loads the zone in `TZ` at start, because Go pins `time.Local` to UTC on Android |

Each directory below is a Go package:

| Path | Responsibility |
|------|----------------|
| `config/` | Loads `config.toml` with a hand-written parser in `config.go`. CLI overrides are in `flags.go`. `config` is the only config writer: `save` and `SaveSection` in `config.go`, and the typed savers in `saver.go` |
| `player/` | The audio engine: decoding, FFmpeg and yt-dlp pipelines (`ytdl_relay.go` holds the whole yt-dlp download in memory between yt-dlp and ffmpeg, so an unpause after a long pause needs no reconnect: `YTDLResumable`), the buffered provider pipeline, gapless, the 10-band EQ, volume, speed, ICY and the visualizer tap. `player.go` holds the `Player`, playback and preload. `player_seek.go`, `player_effects.go` (volume, the sleep-timer fade, speed, mono, EQ, crossfade), `player_metadata.go` and `player_hooks.go` (what providers register) hold the rest. `ytdl_info.go` prefetches the yt-dlp page info of the next song, so a skip to it starts in about 0.5 s instead of 3 s, and keeps it for that song's seeks and restarts. `audio_device_*.go` holds the platform audio devices |
| `playlist/` | The `Track` model, the queue with shuffle, repeat and play-next, M3U and PLS, the track TOML format, and the base `playlist.Provider` interface in `provider.go` |
| `provider/` | The optional capability interfaces in `interfaces.go`, `Entry`, the `ProviderMeta` keys in `types.go`, and `AddTracks` |
| `external/<name>/` | One package per provider: `ytmusic` (YouTube Music, signed in with browser cookies) and `local` (local playlists, favorites and history). `ytmusic/snapshot.go` keeps home, the library and the genres on disk, shown at once at start and refreshed behind. `ytmusic/discover.go` builds the endless "For you" album list (`AlbumList` sort `foryou`) and artist list (`ArtistList`, `provider.ArtistPager`) from seed artists and their related artists, filters everything through the taste store, holds the 44 search-based extra genres (`search:` IDs) and `DislikeTrack`. `Refresh` restarts discovery with a new random order |
| `taste/` | The "interested" (`I`) and "not interested" (`H`) ratings and the blocked words and artists, in `taste.json` in the config directory. `Hidden` filters recommendations, `Liked` gives discovery seeds. Defaults hide reggaeton, urbano, trap, bachata and salsa |
| `locallib/` | Indexes the tags of the local music folder in SQLite (`modernc.org/sqlite`), so local albums, artists and genres sit in the same menus as the YouTube Music ones. Refreshed incrementally |
| `ui/artwork/` | Cover and thumbnail loading with a disk and memory cache, half-block drawing, the kitty graphics protocol (Unicode placeholders) and the cover palette |
| `internal/sponsorblock/` | SponsorBlock segment lookups (hash-prefix privacy mode) |
| `internal/loudstore/` | Remembered loudness per track for normalization, with ReplayGain tags for local files |
| `internal/offline/` | Offline downloads (yt-dlp + ffmpeg tags and cover) and the manifest that lets the player play a downloaded copy |
| `internal/stats/` | The play log and its summary for the statistics screen |
| `internal/desktopnotify/` | Now-playing desktop notifications over D-Bus with Next and Like actions |
| `favorites/`, `history/` | The ♥ favorites and Recently Played stores in the config directory. `buildProviders` makes one of each and shares them with the local provider and the Model |
| `resolve/` | Turns the user arguments into tracks: files, directories, globs, URLs, M3U, PLS, feeds and yt-dlp playlists |
| `tracksave/` | Saves a downloaded track in the download directory |
| `ui/` | The visualizer engine: `visualizer.go`, the drivers in `vis_driver.go`, the mode table in `vis_registry.go`, one `vis_*.go` file per mode, and the Lua modes in `vis_lua.go`. Also the shared styles, the frame padding and the tick intervals |
| `ui/model/` | The Bubbletea `Model`. See [ui/model by concern](#uimodel-by-concern). This is the biggest package. Start here for UI behavior |
| `luaplugin/` | The Gopher-Lua VMs, the sandbox, the per-plugin event queues, the plugin visualizers and the plugin APIs in `api_*.go` |
| `plugins/` | First-party example plugins. `luaplugin/regression_bundled_test.go` loads each one |
| `pluginmgr/` | `cliamp-x plugins install`, `remove`, `list` and `trust`. Resolves GitHub, GitLab, Codeberg and direct URL sources |
| `ipc/` | The V2 protocol over a Unix socket: the envelope, the operation registry, jobs and the event broker. See [IPC](#ipc) |
| `mediactl/` | Media controls: MPRIS over D-Bus on Linux (bus name `org.mpris.MediaPlayer2.cliamp_x`). The macOS and Windows backends and `service_stub.go` are kept from upstream |
| `lyrics/` | The lyrics lookup: embedded lyrics, LRCLIB and NetEase, with a disk cache of found lyrics in `cache.go` |
| `theme/` | The theme loader and the built-in themes in `themes/` |
| `cmd/` | The bodies of the `playlist`, `history` and `protocol` subcommands |
| `applog/` | The log file `cliamp.log` in the config directory |
| `upgrade/` | Self-update for `cliamp-x upgrade`, and `upgrade --check --json` (`Check`, `Report`) for the Android app's update check. On Android `Run` refuses to replace the binary |
| `internal/` | Private helpers: `appdir` for the config, data and plugin directories, `appmeta` for the version, `authurl` for sign-in URLs, `browser`, `credstore` for credential files, `deeplink` for `cliamp-x://` parsing, `fileutil`, `extbin` (runs yt-dlp and ffmpeg: `CLIAMP_YTDLP` and `CLIAMP_FFMPEG` override the argv, their `_ENV` variables add `K=V` pairs separated by 0x1F, and yt-dlp gets `--ffmpeg-location`), `fuzzy`, `httpclient` for the stream client and `NewAPI`, `netdiag`, `playback` for the shared messages and `Notifier`, `plugintrust`, `resume`, `sshurl`, `tomlutil` for `[[section]]` files, `ytdlcookies` and `ytdlp` |

Other directories: `docs/` holds the user docs, one `.md` per feature. `scripts/install.sh` is the installer. `packaging/` holds the AUR package and the `nfpm.yaml` for deb and rpm. `nix/package.nix` and `flake.nix` build the Nix package. `.github/workflows/` holds `ci.yml` (it also builds the Android core), `release.yml` (it adds the signed APK when the keystore secrets exist), `distros.yml`, which runs the installer in a container of each distribution family, `android.yml` (tests, lint and build of the app when `android/` changes) and `nightly.yml` (the signed nightly APK of `main` and `android`).

`android/` holds the Android app (`io.github.xaviiromf.cliampx`). `CliampService` runs `libcliamp.so`, the Go core built by `make android-core`, in a terminal session and owns the MediaSession. `Remote.kt` is the one IPC V2 client of the native Compose UI in `ui/`, `Ytdlp.kt` keeps yt-dlp current, `Vis.kt` and `ui/Visualizer.kt` draw the core's visualizer frames (`vis.frame`), `TerminalActivity` is Modo terminal (the TUI; the core draws only while it is visible, through `ui.render`). `android/UI-SPEC.md` is the design contract, `android/PLAN.md` the port plan, `android/README.md` the build guide and `docs/android.md` the user guide. On Android the core tries yt-dlp without cookies first (`player/`), and `GOOS=android` also matches the `linux` build tag, so linux-only files carry `&& !android`.

### Runtime flow (read this before touching `main.go`)

1. `main()` sets the version and runs `buildApp()` from `commands.go`. Most subcommands are thin V2 clients in `ipc_client.go`. They talk to the running instance over the socket.
2. `run(overrides, positional, headless, visualizer60FPS)` is the path of the TUI and of headless mode:
   1. Load `config.toml`, apply the CLI overrides and open `cliamp.log`. In headless mode `checkNotRunning` ends the run when another instance serves the socket.
   2. Call `buildProviders` in `providers.go`. Local always registers. YouTube Music registers unless `[ytmusic]` sets `enabled = false`, and needs yt-dlp.
   3. Resolve the positional arguments with `resolve.Args`. Feeds, M3U, PLS and yt-dlp pages go to `Pending` and resolve after start.
   4. Build the start playlist from `--playlist`.
   5. Open the `player.Player` in `newPlayer`. `registerPlayerHooks` adds the stream factories, source resolvers and URL matchers of the providers.
   6. Create the IPC event broker and load the Lua plugins with `luaplugin.New`.
   7. Build the Model with `model.New` and `config.SaveFunc{}`. `configureModel` applies the settings. In headless mode it calls `SetHeadless(true)`.
   8. Create the Bubbletea program with `programOptions`. In both modes `quitOnSignals` turns SIGINT, SIGTERM and SIGHUP into `playback.QuitMsg`, so the exit save runs.
   9. Attach the sign-in URL observers, the media controls in `wireMediaCtl`, and the Lua control and UI providers from `lua_wiring.go`.
   10. Start the IPC server in `startIPC`. In headless mode a failed bind ends the run.
   11. `mediactl.Run` runs the program until it quits. `saveOnExit` then saves the theme and the resume position.

### Headless mode

`cliamp-x --daemon` or `cliamp-x -d` runs the same `model.Model` as the TUI. The program has no renderer and no input. `SetHeadless(true)` makes `View` return nothing and uses the low-power tick. Headless mode loads the Lua plugins, sends now-playing and scrobble reports, records history when a track starts and preloads the next track, as the TUI does. It differs from the TUI in these ways:

- It does not open the provider browser at start.
- It never offers to install yt-dlp.
- `v2Operations` removes the `theme`, `vis` and `ui.render` operations.
- It keeps the saved theme at exit.

`docs/headless.md` is the user guide.

### IPC

The socket is `cliamp-x.sock` in the config directory. `ipc.DefaultSocketPath` returns the path. The protocol is V2 only: one JSON object per line with `"version": 2`. The server answers any other request with `invalid_version`. The operations live in these places:

| Place | What it owns |
|-------|--------------|
| `ipc/v2.go`, `ipc/server.go` | The envelope, `capabilities`, `job.get`, `job.cancel`, `subscribe`, and the direct reads `state.get`, `spectrum.get` and `vis.frame` |
| `ipc/operations.go` | `DefaultOperationRegistry`: the name, the parameters and the parameter checks of each operation |
| `ipc/jobs.go`, `ipc/pubsub.go` | The job store and the event broker |
| `main.go` | `newV2Dispatcher` sends each job to the Model as `model.V2RequestMsg`, and runs `plugin.call` and `plugin.commands` against `luaplugin`. `v2Operations` picks the operations of the runtime |
| `ui/model/ipc_runtime.go` | Playback, queue, play-next, settings, the runtime snapshot and the runtime events |
| `ui/model/ipc_extended.go` | The provider, playlist, library, `url.load`, `save`, `lyrics` and `history` operations |
| `ipc_client.go`, `commands.go`, `open.go` | The CLI side |

To add an operation:

1. Add it to `DefaultOperationRegistry` in `ipc/operations.go`.
2. Put new parameter fields on `ipc.Request` in `ipc/protocol.go` when the existing fields do not fit.
3. Handle it in `handleV2Request` in `ui/model/ipc_runtime.go`, or in `ui/model/ipc_extended.go`. `handleV2Request` sends only `provider.*` and `playlist.*` names to `ipc_extended.go` by itself. For another name, add it to the case that calls `handleV2DeferredRequest`, and add a case to the switch in `handleV2DeferredRequest`. `handleV2Request` fails an unknown name with `unavailable`.
4. For a queue edit, add the name to `v2MutatesLivePlaylist` in `ui/model/ipc_runtime.go`. The revision check then covers it.
5. When headless mode cannot serve the operation, unregister it in `v2Operations` in `main.go`.
6. Add a CLI command in `commands.go` when users need one.
7. Add table-driven tests in `ui/model/`, and in `ipc/` for a protocol change.
8. Document it in `docs/remote-control.md`.

### ui/model by concern

| Concern | Files |
|---------|-------|
| State and setup | `model.go` holds the `Model` struct, the screens, the focus areas and the `ConfigSaver` seam. `state.go` groups the sub-structs. `init.go` holds `New`, the setters, `SetHeadless` and `Init` |
| Update loop | `update.go` is one type switch. After each message it lays out the frame, drops a stale preload, tells the media controls, emits the plugin events and publishes the IPC and plugin state. `update_load.go`, `update_nav.go`, `update_playback.go`, `update_provider.go` and `update_search.go` hold the message bodies. `tick.go` runs the frame tick. `commands.go` holds the `tea.Cmd` constructors and their messages |
| Actions | `actions.go` holds one verb for each user intent that more than one entry point starts: keys, media controls, Lua and IPC. `queue_ops.go` holds the one queue edit rule for keys, IPC and Lua. `playback.go`, `playback_state.go`, `preload.go`, `seek.go` and `audio.go` run playback, the gapless preload, seek, EQ and speed. `eq_presets.go` holds the built-in EQ presets. `ytdl_batch.go` loads a long YouTube playlist in batches |
| Reports | `notifications.go` updates the media controls. At track start it records history and sends the now-playing report. When a track that played past half its length is left, it sends the scrobble. `report_queue.go` keeps the provider reports in order. `favorites.go` copies a ♥ favorite to the service |
| Keys | `keys.go` holds `handleKey` and the global keys. `keys_*.go` hold the keys of each screen. `keys_provider.go` holds the provider pane keys and `keys_screens.go` the small screens. `command_registry.go` lists every key with its mode. It feeds the help bar (at most `maxHelpHints`), the actions menu and `ReservedKeys` for plugins. `keymap.go` is the actions menu (`F1`, `Ctrl+K`, `?`): `actionGroups` sorts every player command into sections, and a player command not in a group falls under "Navigation". `row_menu.go` reuses it as the row menu (`O`, right click) |
| Overlays | `overlays_table.go` holds `overlayStack`. The first open overlay in it renders, gets the keys and gets pasted text. `overlays.go`, `inline_overlays.go`, `inline_overlays_nav.go`, `filebrowser.go`, `pl_picker.go`, `plmgr_append.go`, `lyrics.go`, `metadata.go`, `jump.go` and the `*_subs.go` files implement the overlays. `filter_list.go` holds the cursor, the scroll and the `/` filter of a list overlay. `search.go` filters the playlist for the `/` search |
| View | `view_frame.go` (`View`, the sections and `renderFrame`), `view_header.go` (title, mode tags, clock, seek bar, controls), `view.go` (provider list and playlist), `view_columns.go`, `view_helpers.go`, `view_nav.go`, `view_overlays.go`, `layout.go`, `scroll.go` (also `withScrollbar`), `playlist_rows.go`, `styles.go` and `title.go`. `textinput.go` edits the inline text inputs and draws their cursor. `view_bench_test.go` has `BenchmarkView` and the test that `renderFrame` matches the lipgloss frame byte for byte |
| Providers | `providers.go` switches providers, loads the provider pane and maps the `Y` (YouTube Music) and `L` (Local) shortcuts. `keys_favorites.go` handles the favorite keys. `nav_labels.go` names the browse levels |
| IPC | `ipc_runtime.go` and `ipc_extended.go`. See [IPC](#ipc) |
| Plugins | `plugin_events.go` emits the Lua events after each Update. `plugin_state.go` publishes the `PluginState` snapshot that Lua reads. `plugin_queue.go` applies `PluginQueueMsg` |
| Fork features | `interest.go` (`I`/`H` ratings on the highlighted song, album, artist or genre, never a menu or playlist: pane rows need `provider.RatingKinder`; `P` opens `openRatingsMenu` to undo one; `refreshBrowser` for `F5`/`Ctrl+R` in the browser; `visibleTracks` filters the auto radio), `discover.go` (`M` more like this, `X` remixes), `autoradio.go` (`R`), `downloads.go` (`D` offline), `stats_view.go` (`S`), `queue_save.go` (`W` and the `*` marks), `eq_panel.go` (`E`), `sponsor.go` (SponsorBlock), `desktop.go` (desktop notifications), `mouse.go`, `cover.go` (cover visualizers, cover theme, fullscreen cover), `cover_fade.go` (the cover theme blends between songs) and `preview.go` (the cover pane under a list). `sleep.go` (`T` sleep timer), `downloads_view.go` (`Ctrl+O`), `search_history.go` (`Ctrl+F` history), `play_errors.go` (what to do when a YouTube song fails). `RestoreSession` and `captureSession` in `init.go` keep the playlist, song, position and play-next queue between runs. `audio_extras.go` adapts normalization and crossfade to the test engines; optional engine features (`SetFade`, `PrefetchYTDL`) are small interfaces checked with a type assertion, so the test engines need no stubs |
| Language | `i18n.go` holds `L(en)` and `SetLanguage`. `i18n_es.go` is the Spanish catalog. Every fixed interface text (key labels, screen names, literal status messages, `actionGroups` labels) goes through `L`. `TestSpanishCatalog` in `i18n_test.go` fails `make test` when a key label, screen name, `L("...")` literal, actions menu label or literal status message has no Spanish entry, or when an entry changes its format verbs. A status text built only from verbs and symbols, such as `"%s: %d min"`, still needs an entry |

Overlays are rows in `overlayStack`, not a mode stack. `Update` dispatches with a type switch, not a message-handler interface.

### Provider contract

Every provider implements `playlist.Provider` in `playlist/provider.go`. The optional capabilities are interfaces in `provider/interfaces.go`. The UI finds them with type assertions. Register a provider with `registerProvider` from an `init` in its own `provider_<key>.go`; do not edit `buildProviders`. Its settings come from `[providers.<key>]` through `config.Config.ProviderSection`. `provider.Entry.Shortcut` gives it a player key unless the command registry already uses that key. The default provider key is `ytmusic`; local files are the `local` provider. `docs/adding-providers.md` is the guide for forks.

The main screen (`[Queue]`) lists only what plays now. `Esc` opens the provider pane (`[Explore]`). `Enter` on a browser track replaces the queue from that track (`replaceQueueFromNav`) and returns to the queue. The auto radio is off by default.

### Plugin surface

Lua plugins run in isolated `gopher-lua` VMs. A crash stays in its plugin. Each plugin runs its events and key presses in order from its own queue, and each callback has a timeout. Plugins read the `PluginState` snapshot that the Model publishes. Their controls reach the Model as messages through `lua_wiring.go`. Plugins can register visualizers. When you add or change a plugin API, update both:
1. `luaplugin/api_*.go` and its tests.
2. `docs/plugins.md`, the user-facing reference.

---

## Build, test, and local workflow

`mise.toml` pins Go 1.26.9 and shellcheck 0.10.0. Run the targets through mise to use the pins, for example `mise exec -- make ci`.

| Target | What it does |
|--------|--------------|
| `make build` | `go build -trimpath` with the version ldflags into `./cliamp-x` |
| `make test` | `go test ./...` |
| `make vet` | `go vet ./...` |
| `make lint` | `vet`, then `staticcheck` when it is installed |
| `make staticcheck` | `staticcheck ./...`. It fails when staticcheck is missing |
| `make tools` | Installs the pinned `staticcheck` and `govulncheck` |
| `make fmt` | `gofmt -l -w` on the Go files that git tracks or would track |
| `make fmt-check` | Fails when a Go file that git tracks or would track needs `gofmt` |
| `make tidy-check` | `go mod tidy -diff`. Fails when `go.mod` or `go.sum` is not tidy |
| `make coverage` | Tests with a coverage profile and a summary for each function |
| `make security` | `govulncheck ./...` |
| `make ci` | What CI runs: `fmt-check`, `tidy-check`, `vet`, `staticcheck`, `security`, race tests with coverage, `shellcheck scripts/install.sh` and `git diff --exit-code` |
| `make check` | `fmt`, `vet` and `test` |
| `make install` | Builds and installs the binary into `~/.local/bin` |
| `make clean` | Removes the binary |
| `make android-core` | Builds the Go core for Android into `android/app/src/main/jniLibs/<abi>/libcliamp.so`. Needs `ANDROID_NDK_HOME`; `ANDROID_ABI=x86_64` builds for the emulator, where arm64 Go binaries crash |

One test or one package: `mise exec -- go test ./ui/model -run 'TestSpanishCatalog' -count=1`. Add `-race` for the concurrency tests in `player/` and `luaplugin/`. The visualizer golden tests live in `ui/` and read `ui/testdata`.

Keep these versions in step:

- The `go` line in `go.mod`, the `go` pin in `mise.toml`, and `STATICCHECK_VERSION` in the `Makefile`. CI reads the Go version from `go.mod`.
- The `shellcheck` pin in `mise.toml` and `SHELLCHECK_VERSION` in `.github/workflows/ci.yml`.
- `GOVULNCHECK_VERSION` in the `Makefile` sets the govulncheck that `make tools` installs.

On Linux the build needs the ALSA, FLAC, Vorbis, Ogg and mpg123 headers: `libasound2-dev libflac-dev libvorbis-dev libogg-dev libmpg123-dev` on Debian and Ubuntu. For audio on PipeWire or PulseAudio, install `pipewire-alsa` or `pulseaudio-alsa`. The README troubleshooting section has the details. Runtime tools are `yt-dlp` for YouTube Music and `ffmpeg` for AAC, ALAC, Opus and WMA.

Tests are colocated with sources (`*_test.go`). Favor table-driven tests — the codebase already uses them heavily in `player/`, `playlist/`, `config/`, `ui/model/`, and `luaplugin/`. `config/config_golden_test.go` loads every setting of `config.toml.example`. Other tests check the lists and defaults that `docs/*.md` and `config.toml.example` name, and the anchor links of `docs/*.md` and `README.md`. A docs edit can therefore fail `make test`.

Config lives at `~/.config/cliamp-x/config.toml` (example at `config.toml.example`); plugins at `~/.config/cliamp-x/plugins/`; themes at `~/.config/cliamp-x/themes/`; the YouTube Music cookies at `~/.config/cliamp-x/ytmusic_cookies.txt`. The IPC socket is `~/.config/cliamp-x/cliamp-x.sock`, and the log is `~/.config/cliamp-x/cliamp.log`. Set HOME and every `XDG_*` directory to a temp directory when you run a cliamp-x binary. Build the binary first in the normal environment, for example with `mise exec -- make build`. Do not run `mise exec` or `go` under the isolated HOME. mise then loses its installed tools, and go loses its build cache and module cache. `CLIAMP_CONFIG_DIR` and `XDG_CONFIG_HOME` move only the config directory, which holds the socket and the log. The data directory `~/.local/share/cliamp-x`, the `~/Music/cliamp-x` save directory and `~` in local music paths still follow HOME.

---

## Conventions to follow

- **Package naming:** lowercase, single-word, matches directory. No internal suffix gymnastics — use `internal/` for genuinely private helpers.
- **Error handling:** wrap with `fmt.Errorf("context: %w", err)`. Surface user-facing messages from `main.go` / `run(...)` only.
- **Build tags:** a platform file uses the GOOS suffix `_linux.go`, `_darwin.go` or `_windows.go`. `player/audio_device_macos.go` is the one exception, with `//go:build darwin && !ios`. The file for the other platforms uses `_stub.go`, `_other.go` or `_unix.go`, with a `//go:build` line such as `!windows`. Follow these patterns. Do not invent new conditional-compile styles.
- **Bubbletea messages:** put shared message types in `internal/playback/` so UI code and non-UI callers (Lua, IPC) can both send them via `prog.Send(...)`.
- **Keep `docs/` in sync** on any user-visible change (keybindings, plugin APIs, config keys).
- **Don't add emojis** to code or docs unless the user asks for them.
- **Minimal diffs:** prefer editing in place over rewriting files. No speculative abstractions.
- **Render hot paths:** `View` runs on every visualizer frame. In row and bar code, use `paint(style, s)` instead of `style.Render(s)` and `textWidth(s)` instead of `lipgloss.Width(s)`, both in `ui/model/styles.go`. They give the same bytes and widths (tests check every theme and random text) without the layout and grapheme work. Never call the file system while drawing: for example `offline.Manifest.Recorded`, not `Has`.

---

## Skills to use

When working in this repo, prefer these skills over ad-hoc approaches:

- **`/golang`** — Best practices for production Go (error handling, concurrency, naming, testing patterns). Use for any Go code you write, review, or refactor here. Pair with `everything-claude-code:golang-patterns` and `everything-claude-code:golang-testing` for deeper pattern work.
- **`/simplify`** — Review changed code for reuse, quality, and efficiency, then fix what it finds. Run after non-trivial edits in `player/`, `ui/model/`, or `luaplugin/` — those packages accumulate complexity fastest.
- **Refactoring** — For dead-code cleanup and consolidation, dispatch the `everything-claude-code:refactor-cleaner` agent. For broader architectural restructuring, use `everything-claude-code:architect` first to plan, then execute with narrow edits. Always run `make check` after a refactor — gofmt, vet, and tests all need to pass before you stop.
- **`/go-review`** — For comprehensive idiomatic Go review (concurrency safety, error handling, security) before landing larger changes.
- **`/docs`** — When touching an external library (Bubbletea, Beep, urfave/cli, gopher-lua), look up current docs via Context7 rather than relying on training data.

Golden path for a non-trivial change:
1. Read relevant `docs/*.md` + skim the target package.
2. Plan (optionally via `everything-claude-code:plan`).
3. Implement the narrowest change that works. Add/extend table-driven tests.
4. Run `make check`.
5. Invoke `/simplify` on the diff.
6. If user-visible: update `docs/`.

---

## Where to look first

| Question | Start here |
|----------|-----------|
| How does the player decode X? | `player/decode.go`, `player/ffmpeg.go`, `player/ytdl.go`, `player/pipeline.go` |
| How does the EQ work? | `player/eq.go` and `player/eq_test.go`. The presets are in `ui/model/eq_presets.go` |
| How is the UI laid out? | `ui/model/layout.go`, `ui/model/view.go`, `ui/model/view_columns.go`, `ui/styles.go` |
| How do keybindings work? | `ui/model/command_registry.go`, `ui/model/keys.go`, `ui/model/keys_*.go`, `ui/model/keymap.go`, and the user-facing `docs/keybindings.md` |
| Which overlay gets a key? | `ui/model/overlays_table.go` |
| Where does a key, IPC or Lua action land? | `ui/model/actions.go` and `ui/model/queue_ops.go` |
| How does the YouTube Music provider work? | `external/ytmusic/`, `providers.go`, `docs/youtube-music.md` |
| How does IPC work? | `ipc/v2.go`, `ipc/operations.go`, `ipc/server.go`, `ui/model/ipc_runtime.go`, `docs/remote-control.md` |
| How does headless mode work? | `configureModel`, `programOptions` and `v2Operations` in `main.go`, `SetHeadless` in `ui/model/init.go`, and `docs/headless.md` |
| How do Lua plugins reach the Model? | `lua_wiring.go`, `ui/model/plugin_state.go`, `ui/model/plugin_events.go` |
| How are Lua plugins sandboxed? | `luaplugin/sandbox.go`, the write rules in `luaplugin/api_fs.go`, the queues and timeouts in `luaplugin/hooks.go`, and `internal/plugintrust/` |
| Where are bundled plugins? | `plugins/` |
| What visualizers exist? | `ui/vis_registry.go`, `ui/vis_*.go`, `ui/vis_driver.go` |
| How is configuration resolved and saved? | `config/config.go` for `Load`, `save` and `SaveSection`. `config/flags.go` for the CLI overrides. `config/saver.go` for the typed savers |
| Why does my audio break silently on Linux? | The README troubleshooting section |
| How does the Android app work? | `android/UI-SPEC.md`, `android/README.md`, `android/app/src/main/java/io/github/xaviiromf/cliampx/Remote.kt` and `docs/android.md` |

---

## Things to know about the maintainer's preferences

- Keep responses and commits terse; no trailing "here's what I did" summaries unless asked.
- Bundled PRs for refactors in one area are preferred over many small ones (per feedback memory).
- User-facing changes must update `docs/` in the same change.
- Avoid adding new top-level dependencies casually — the dependency list in `go.mod` is intentional.

## Commits

Every commit must have a description body, not just a subject line.
Subject: `area: what changed` (50 chars or less, e.g. `fix(ui): ...`). Body: what was wrong,
what the fix does, and how it was verified (`make check`: fmt, vet, tests).

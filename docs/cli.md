# CLI Flags

Override a config option for one session without editing `~/.config/cliamp-x/config.toml`. Put flags before or after file and URL arguments.

## Playback

```sh
cliamp-x --vol -5 track.mp3             # startup volume in dB [volume_min, +6]
cliamp-x --shuffle ~/Music              # enable shuffle
cliamp-x --repeat all ~/Music           # repeat mode: off, all, one
cliamp-x --mono track.mp3               # downmix to mono
cliamp-x --no-mono track.mp3            # force stereo
cliamp-x --auto-play ~/Music            # start playback immediately
cliamp-x --playlist "Blade Runner"      # load a local TOML playlist (add --auto-play to start playback)
cliamp-x --provider ytmusic             # open on one provider for this session
```

`cliamp-x` with no file, URL or `--playlist` brings the last session back: the playlist, the song selected in it, the songs queued to play next and, when the song had one, its position. Nothing plays until you press `Space` or `Enter`. cliamp-x saves the session while it plays (when the song, the list or the pause changes, and every 30 seconds for the position), not only at exit, so a crash or a process the system kills brings back the last song and not an old one. YouTube Music songs keep their position too. With no saved session, cliamp-x opens the provider browser.

`--provider` overrides `provider` in config.toml. It accepts the values in [Default Provider](configuration.md#default-provider). Run `cliamp-x --help` to see the list.

## Audio engine

```sh
cliamp-x --sample-rate 48000 track.mp3      # output sample rate (0=auto, 22050, 44100, 48000, 96000, 192000)
cliamp-x --buffer-ms 2000 track.mp3         # speaker buffer in ms (50-5000; useful for unstable radio)
cliamp-x --resample-quality 1 track.mp3     # resample quality factor (1–4)
cliamp-x --bit-depth 32 track.m4a           # PCM bit depth: 16 (default) or 32 (lossless)
cliamp-x --audio-device list                # print the output devices, then exit
cliamp-x --audio-device NAME track.mp3      # use one output device for this session
```

`--audio-device` overrides `audio_device` in config.toml. See [Audio output device](configuration.md#audio-output-device) for the platform rules.

## Appearance

```sh
cliamp-x --simplified ~/Music                  # no visualizer or playlist
cliamp-x --expanded ~/Music                    # start with the playlist expanded (Ctrl+X state)
cliamp-x --eq-preset "Bass Boost" ~/Music
cliamp-x --visualizer Wave ~/Music             # start with one visualizer mode
cliamp-x --visualizer-60fps ~/Music            # smoother visualizer animation (higher CPU use)
```

`--visualizer-60fps` renders a visible visualizer at about 60 FPS during playback. `Wave`, `Scope`, `Heartbeat`, and `Stereo` already use this rate because they draw from audio samples. The flag does not affect overlays or low-power mode. When the terminal reports that it lost the focus, these modes drop to the normal 20 FPS until it gets the focus back. The visualizer keeps moving, because a tiling window manager can still show the window.

## Diagnostics

```sh
cliamp-x --log-level debug                     # raise log verbosity for one session
```

cliamp-x writes logs to `~/.config/cliamp-x/cliamp.log`. Use `debug`, `info` (default), `warn`, or `error`. The file does not grow without bound: at 1 MB cliamp-x renames it to `cliamp.log.1`, replacing the previous one, and starts a new `cliamp.log`.

## Low-power mode

```sh
cliamp-x --low-power track.mp3                 # reduce CPU load for this session
```

This reduces CPU load. It lowers the UI and render rate and sets the visualizer to `none`. Use it on battery power, slow terminals, or SSH sessions. Press `v` in the player to enable a visualizer again.

To make this persistent, set it in `~/.config/cliamp-x/config.toml`:

```toml
low_power = true
```

## Headless daemon mode

```sh
cliamp-x --daemon                              # no TUI, IPC only
cliamp-x --daemon --auto-play --playlist Lofi  # start playing on launch
cliamp-x -d ~/Music --auto-play                # short flag form
```

Run cliamp-x without a UI. It runs the same player as the TUI and listens on the Unix socket that the TUI uses. All `cliamp-x <subcommand>` IPC clients work. Plugins, scrobbling, and Recently Played work as in the TUI. `cliamp-x theme <name>` and `cliamp-x vis <name|next>` return an error. The `list` forms still work. See [Headless Daemon Mode](headless.md) for use cases and configuration examples for Waybar, Hyprland, systemd, and cron.

## YouTube playlists

To play only one video of a YouTube URL with a `list=` parameter, add `--no-expand-playlist`. `--expand-playlist` loads the whole playlist, which is the default. See [YouTube Music](youtube-music.md).

```sh
cliamp-x --no-expand-playlist 'https://www.youtube.com/watch?v=VIDEO&list=PLAYLIST'
```

Press `Ctrl+F` in the player to search YouTube Music. See [YouTube Music](youtube-music.md).

## Updates

```sh
cliamp-x upgrade                # latest stable release
cliamp-x upgrade --prerelease   # newest release, betas and release candidates included
```

Select prerelease updates for each upgrade. `--prerelease` installs the release with the highest SemVer version, stable or prerelease, that has a binary for your platform. It never installs a version older than the one you run. Normal installs and `cliamp-x upgrade` use stable releases only.

`--check` reports the newest release and installs nothing. With `--json` it
prints one JSON object, which the Android app reads:

```sh
cliamp-x upgrade --check --json [--prerelease]
```

```json
{"current":"v2.2.1","latest":"v2.2.2-nightly.20261013.812+abc1234","prerelease":true,"newer":true,
 "assets":[{"name":"cliamp-x-2.2.2-nightly.20261013.812-arm64-v8a.apk","url":"https://github.com/...","sha256":"..."}]}
```

`current` is the version of the binary that runs. `prerelease` tells whether
`latest` has a SemVer prerelease part. `newer` is true when `latest` is a
higher SemVer version than `current`, or when `current` is not a version (a
development build). `assets` lists every file of the release with its public
download URL and the SHA-256 from the release's `checksums.txt`, or `""` when
that file lists none. The Android nightly is the prerelease with the tag
`nightly`. Its release name holds its version,
`v<next patch>-nightly.<yyyymmdd>.<commit count>+<commit>`, which ranks above
the last stable release and below the next one. It has no desktop binaries,
so `cliamp-x upgrade --prerelease` skips it.

`cliamp-x upgrade` downloads the release files through the GitHub API and
checks the SHA-256 in `checksums.txt` before it replaces the binary. While
the repository is private, it needs a GitHub token: it uses `GITHUB_TOKEN` or
`GH_TOKEN` when set, and otherwise asks the GitHub CLI (`gh auth token`). The
token goes only to `api.github.com`. Without one, a private repository answers
404 and the command says to run `gh auth login`.

## General

| Flag | Short | Description |
|------|-------|-------------|
| `--help` | `-h` | Show help and exit |
| `--version` | `-v` | Print version and exit |

## Shell completion

Generate completion scripts for supported shells.

```sh
cliamp-x completion bash
cliamp-x completion zsh
cliamp-x completion fish
cliamp-x completion pwsh
```

Source the generated script directly or install it as your shell documentation describes.

## Mixing flags and files

Put flags before, after, or between positional arguments:

```sh
cliamp-x --shuffle track.mp3 --vol -5
cliamp-x track.mp3 --repeat all --mono ~/Music
```

## Flag reference

| Flag | Type | Default | Range / Values |
|------|------|---------|----------------|
| `--vol` | float | 0 | `volume_min` (default -50) to +6 dB |
| `--shuffle` / `--no-shuffle` | bool | false | |
| `--repeat` | string | off | off, all, one |
| `--mono` / `--no-mono` | bool | false | |
| `--auto-play` / `--no-auto-play` | bool | false | |
| `--simplified` / `--no-simplified` | bool | false | artist/title and time strip; no visualizer or playlist |
| `--help-bar` / `--no-help-bar` | bool | true | show or hide the key-binding hint bar; `F1` or `?` still opens the actions menu |
| `--expanded` / `--no-expanded` | bool | false | start with the playlist expanded (the `Ctrl+X` state) |
| `--provider` | string | ytmusic | provider to open at start: ytmusic |
| `--visualizer` | string | Bars | visualizer mode, such as Bars, Wave or None. See [configuration.md](configuration.md#options) for all modes. |
| `--visualizer-60fps` | bool | false | render a visible visualizer at about 60 FPS |
| `--start-theme` | string | | theme name |
| `--eq-preset` | string | | preset name |
| `--sample-rate` | int | 0 | 0, 22050, 44100, 48000, 96000, 192000. 0 reads the rate of the default output device on macOS and uses 44100 on other platforms. |
| `--buffer-ms` | int | 250 | 50-5000 |
| `--resample-quality` | int | 4 | 1-4 |
| `--bit-depth` | int | 16 | 16, 32 |
| `--audio-device` | string | | a device name from `--audio-device list`. `list` prints the devices and exits. |
| `--playlist` | string | | local TOML playlist name |
| `--log-level` | string | info | debug, info, warn, error |
| `--expand-playlist` / `--no-expand-playlist` | bool | true | expand YouTube and YouTube Music `list=` URLs to the full playlist. `--no-expand-playlist` plays only the video. |
| `--low-power` / `--no-low-power` | bool | false | lower UI cadence; disable visualization |
| `--daemon` / `-d` | bool | false | run headless; IPC only, no TUI |

CLI flags override config file values for the current session only. Persisted boolean options accept matching `--no-*` flags, such as `--no-shuffle` and `--no-low-power`. cliamp-x does not save them.

## Playlist Management

Manage local TOML playlists from the command line without opening the TUI.

```sh
cliamp-x playlist list                          # list playlists with track counts
cliamp-x playlist create "Name"                 # create an empty playlist
cliamp-x playlist create "Name" file1 dir/ ...  # create from files/folders (recursive, skips duplicate paths)
cliamp-x playlist create "Name" --ssh HOST dir/ # create from remote machine via SSH
cliamp-x playlist create "Name" --dir ~/Music   # reference a directory as a [[dir]] source (scanned at load)
cliamp-x playlist add "Name" file1 ...          # append tracks to existing playlist, skipping duplicates
cliamp-x playlist add "Name" --dir ~/Music      # add another directory source
cliamp-x playlist dirs "Name"                   # list directory sources
cliamp-x playlist rename "Old" "New"            # rename a playlist
cliamp-x playlist show "Name"                   # display tracks
cliamp-x playlist show "Name" --json            # machine-readable output
cliamp-x playlist remove "Name" --index 3       # remove track by index
cliamp-x playlist dedupe "Name"                 # remove duplicate paths, keeping the first
cliamp-x playlist sort "Name" --by album         # sort in place
cliamp-x playlist doctor [Name]                  # report missing local files
cliamp-x playlist doctor "Name" --fix            # prune missing local files
cliamp-x playlist export "Name" -o mix.m3u       # export as M3U
cliamp-x playlist export "Name" --format pls     # export as PLS to stdout
cliamp-x playlist import mix.m3u --name "Name"   # import local M3U/M3U8/PLS
cliamp-x playlist favorite "Name" --index 3      # toggle the favorite of a track
cliamp-x playlist favorites                       # list favorite tracks
cliamp-x playlist enrich "Name"                   # probe duration/album 
cliamp-x playlist enrich "Name" --source metadata   # probe duration/album (forces to use the file's metadata as source)
cliamp-x playlist delete "Name"                   # delete entire playlist
```

Sort keys: `track`, `title`, `artist`, `album`, `artist+album`, `path`.

`playlist doctor --fix` prunes only playlist files. It reports a missing favorite and keeps it. To remove a favorite, run `cliamp-x playlist favorite Favorites --index N`.

`--ssh` takes `host`, `user@host`, or `host:port`. `playlist enrich` connects to the port in each `ssh://` track path.

See [playlists.md](playlists.md) for the TOML format. See [ssh-streaming.md](ssh-streaming.md) for remote playback.

## Recently Played

```sh
cliamp-x history                                # show the 50 most recent plays
cliamp-x history --limit 200                    # show the 200 most recent
cliamp-x history --limit 0                      # show all (capped at 200 entries on disk)
cliamp-x history --json                         # machine-readable output
cliamp-x history clear                          # wipe ~/.config/cliamp-x/history.toml
```

cliamp-x adds a track to the history when the track starts. In the TUI, the Local
Playlists provider shows this data in the virtual "Recently Played" entry. See
[history.md](history.md).

## YouTube Music sign-in

```sh
cliamp-x login                                  # copy the YouTube Music session of your browser
cliamp-x login --browser brave-origin           # read the session of one browser
cliamp-x logout                                 # remove the saved cookies
```

See [youtube-music.md](youtube-music.md).

## cliamp-x:// Links

Open a song from a browser, a script, or anything that can open a URL:

```sh
cliamp-x open 'cliamp-x://play?url=https://example.com/s.mp3'    # play a stream
cliamp-x open 'cliamp-x://queue?provider=ytmusic&q=aphex+twin'   # queue a search hit
cliamp-x protocol status                                       # where it is registered
cliamp-x protocol register                                     # register the scheme
cliamp-x protocol unregister                                   # remove it
```

install.sh registers the scheme. See [url-scheme.md](url-scheme.md) for the
URI format and what links cannot do.

## Remote Control (IPC)

Control a running cliamp-x instance from another terminal:

```sh
cliamp-x play / pause / toggle / stop    # playback control
cliamp-x next / prev                     # track navigation
cliamp-x status                          # current state
cliamp-x status --json                   # machine-readable state
cliamp-x volume -5                       # set the volume to -5 dB
cliamp-x remote call volume.adjust --params '{"value":3}'    # raise the volume by 3 dB
cliamp-x seek 30                         # seek relative to current position (seconds)
cliamp-x remote call seek.absolute --params '{"value":90}'   # seek to 90s exactly
cliamp-x load "Playlist Name"            # load a playlist
cliamp-x queue /path/to/file.mp3         # queue a track
cliamp-x shuffle [on|off|toggle]         # toggle or set shuffle
cliamp-x repeat [off|all|one|cycle]      # set or cycle repeat mode
cliamp-x mono [on|off|toggle]            # toggle or set mono output
cliamp-x speed 1.5                       # set playback speed (0.25–2.0)
cliamp-x sleep 30                        # pause in 30 minutes, after a 10 s fade-out
cliamp-x sleep end                       # pause when the playing song ends
cliamp-x sleep off                       # turn the sleep timer off
cliamp-x eq Rock                         # set EQ preset by name
cliamp-x eq --band 0 6.0                 # set EQ band 0 to +6 dB
cliamp-x device list                     # list audio output devices
cliamp-x device "My DAC"                 # switch audio output device
cliamp-x theme list                      # list the themes, terminal colors first
cliamp-x remote state                     # v2 GUI-ready runtime snapshot
cliamp-x remote capabilities              # v2 operation list
cliamp-x remote events runtime.state      # v2 event stream
```

`cliamp-x volume` sets an absolute level. A sign does not make the value relative, so `cliamp-x volume +3` sets the volume to +3 dB. To step the volume up or down, submit `volume.adjust` with `cliamp-x remote call`. A negative value lowers the volume. Both forms clamp the result to the range `volume_min` to +6 dB.

Each command waits up to 30 seconds for its result. `cliamp-x load` waits up to 5 minutes and `cliamp-x plugins call` up to 6 minutes. After that time the command fails with an error, and cliamp-x continues the operation. `cliamp-x remote call --wait` waits until the job ends.

`cliamp-x shuffle`, `cliamp-x repeat` and `cliamp-x mono` accept only the values in brackets, in any letter case. Another value gives an error and changes nothing.

`cliamp-x queue` accepts a relative path. It sends the absolute path of a local file, because the running cliamp-x has its own working directory. A URL goes unchanged. A missing file or a directory gives an error.

`cliamp-x theme list` reads the theme files itself, so it works when cliamp-x is not running. It prints each theme file that it skips on stderr.

See [remote-control.md](remote-control.md) for the protocol specification.

## Waybar

```sh
cliamp-x waybar                          # JSON lines for a Waybar custom module
cliamp-x waybar --format "{icon} {title}" --max-length 30
```

See [Waybar module and desktop notifications](waybar.md).

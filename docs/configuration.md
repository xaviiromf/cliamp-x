# Configuration

YouTube Music needs no setup in `config.toml`: sign in at music.youtube.com in
your browser, then run `cliamp-x login`. See [YouTube Music](youtube-music.md).

## Config directory

cliamp-x searches for its config directory in this order:

- `CLIAMP_CONFIG_DIR`
- `XDG_CONFIG_HOME/cliamp-x`
- `HOME/.config/cliamp-x`

The examples below use `~/.config/cliamp-x`.

For other settings, copy and edit the example config:

```sh
mkdir -p ~/.config/cliamp-x
cp config.toml.example ~/.config/cliamp-x/config.toml
```

## Options

```toml
# Default volume in dB (range: volume_min to 6)
volume = 0

# Minimum volume floor in dB (range: -90 to 0, default: -50)
# Controls how low the volume control can go.
volume_min = -50

# Repeat mode: "off", "all", or "one"
repeat = "off"

# Start with shuffle enabled
shuffle = false

# Start with mono output (L+R downmix)
mono = false

# Audio output device name. Empty uses the system default output.
# Run cliamp-x --audio-device list and copy a name from the second column.
# See "Audio output device" below.
audio_device = ""

# Initial directory for the file browser ('o' key)
initial_directory = "~/Music"

# Local music folder whose albums, artists and genres join the YouTube Music
# menus, marked "· local". Empty uses ~/Music (or XDG_MUSIC_DIR); "off" keeps
# local files out of those menus.
music_dir = ""

# Shift+Left/Right seek jump in seconds
seek_large_step_sec = 30

# EQ preset: "Flat", "Rock", "Pop", "Jazz", "Classical",
#             "Bass Boost", "Treble Boost", "Vocal", "Electronic", "Acoustic",
#             "Hip-Hop", "R&B", "Loudness", "Late Night", "Podcast",
#             "Small Speakers", "Metal", "Reggaeton", "Cumbia", "Indie",
#             "Lo-Fi", "Soundtrack", "Blues"
# Leave empty or "Custom" to use manual eq values below
eq_preset = "Flat"

# Auto EQ: switch to the preset that fits each song's genre (local tags and
# YouTube Music genre mixes). Toggle it with "a" in the EQ panel (E).
# eq_auto = false

# 10-band EQ gains in dB (range: -12 to 12)
# Bands: 70Hz, 180Hz, 320Hz, 600Hz, 1kHz, 3kHz, 6kHz, 12kHz, 14kHz, 16kHz
# Saved Custom curve; applied when eq_preset is "Custom" or empty
eq = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0]

# Manual EQ changes update this curve automatically. Cycling presets with e
# keeps it available, and both values are restored after restart.

# Visualizer mode (leave empty for default Bars)
# Options: Bars, BarsDot, Rain, BarsOutline, Bricks, Columns, ClassicPeak, Wave, Scatter, Flame, Retro, Pulse, Matrix, Binary, Sakura, Firework, Bubbles, Logo, Terrain, Scope, Heartbeat, Butterfly, Ascii, Firefly, Mosaic, Sand, Geyser, ClassicLED, Stereo, Mirror, Omarchy, RedSector, YinYang, Spectrogram, Radial, Starfield, Plasma, Tunnel, Helix, Ripple, Aurora, Skyline, Spiral, None
# Mirror draws tapered Braille bars around a persistent horizontal center axis.
# ClassicPeak uses smooth bars and floating peak caps, with sampling aligned
# to audible playback and adaptive redraws for smooth motion.
# ClassicLED redraws at 30 FPS during playback, like the Winamp LED meter.
# Neighboring bands are averaged into each bar.
# YinYang koi chase each other across a wide strip, leaving on the right and
# coming back on the left. Taller panels have them circle a lily pad whose
# lotus follows recent bass levels, including after silence or time in the strip.
visualizer = "Bars"

# Visualizer volume linking (default: true)
# When true, bar height follows the current volume level (classic behavior).
# Set to false to decouple the visualizer from volume — bars stay visible
# even at very low volume levels.
vis_volume_linked = true

# Visualizer height in rows (default: 7), used at the full layout tier.
# Extra rows are taken from the playlist below, and the layout caps the value
# at what the terminal can spare, always leaving one playlist row. Range 1-40.
# The full screen visualizer (V) is unaffected: it always fills the terminal.
vis_rows = 7

# Visualizer sensitivity, 1 (calm) to 10 (very sensitive); "}" / "{" change it.
vis_sensitivity = 7

# Reduce CPU usage by lowering UI cadence and disabling visualization.
# This has the same effect as starting with --low-power.
low_power = false

# Simplified mode: artist/title and time strip without a visualizer or playlist.
# No visualizer or playback controls are shown.
simplified = false

# Hide the key-binding hint bar above the status line.
hide_help_bar = false
# Start with the playlist expanded, the state Ctrl+X toggles (default: false).
# The simplified playback screen has no playlist, but its provider and overlay
# lists do, and they start expanded too.
expanded = false

# Close the Settings pane beside the playlist (Ctrl+B toggles and saves).
hide_settings_pane = false

# Hide the artwork preview pane under the lists.
hide_preview = false

# Preview pictures: "auto", "kitty" (kitty graphics protocol) or "blocks".
preview_images = "auto"

# Show highlighted-playlist metadata below Settings (Ctrl+I toggles and saves).
show_metadata = false

# Loudness normalization: "track", "album" or "off" ("n" cycles and saves).
normalize = "track"
# Normalization target in LUFS (-30 to -5).
normalize_target = -14

# Seconds the end of a song overlaps the start of the next (0-12, 0 = off).
crossfade = 0

# Empty space around the UI: columns on the left and right (0-10, default 3)
# and rows above and below (0-5, default 1).
padding_horizontal = 3
padding_vertical = 1

# UI theme name (see available themes in ~/.config/cliamp-x/themes/)
theme = "Tokyo Night"

# Log level: "debug", "info", "warn", or "error" (default "info")
# Logs are written to ~/.config/cliamp-x/cliamp.log
log_level = "info"

```

`Stereo` shows separate left and right horizontal LED meters with held peak markers.

## Terminal Layout

cliamp-x adapts its playback screen to the terminal size:

| Terminal size | Layout |
| --- | --- |
| At least `80x24` | Two columns below the seek bar, seven visualizer rows (see `vis_rows`), and detailed source controls |
| At least `56x16` | Compact controls and five visualizer rows |
| At least `40x10` | Minimal playback, list, seek bar, and help layout |
| Smaller than `40x10` | Resize message only |

The tiers use the full terminal size. `padding_horizontal` and
`padding_vertical` set the empty space between the terminal edge and the UI,
in columns and rows. The defaults are 3 and 1. cliamp-x clamps
`padding_horizontal` to 0-10 and `padding_vertical` to 0-5. On a small
terminal, cliamp-x reduces the padding so that the UI keeps at least one column
and one row.

The browsing screens (the provider pane, the browser, search, and the keymap)
keep the visualizer too. They first give the list ten rows, then split what is
left between the [preview pane](#preview-pane) and the visualizer, so a short
terminal shows a shorter visualizer, or none.

At the full tier the playback screen splits below the seek bar: the playlist
fills the left column and a `Settings` pane fills the right one. The pane reads
as a signal chain: the source (`SRC`), then volume (`VOL`) and the EQ preset
with its ten bands, then how the list plays — shuffle (`SHF`), repeat (`RPT`),
and speed (`SPD`) — and last the live network counters for a stream (`NET`).
Shuffle and repeat move out of the playlist header here, which keeps its
counts: queue, favorites, and position. The rows those
controls used to occupy above and below the playlist go to the playlist itself.
The title, track line, time, visualizer, seek bar, and hint bar stay full width.
Narrower terminals, simplified mode, overlays, and list views keep the stacked
single-column layout, with shuffle and repeat back in the header.

In full and compact playback layouts, `Tab` cycles from Playlist through Source,
Volume, EQ, Shuffle, Repeat, and Speed, then returns to Playlist. `Shift+Tab`
reverses the order. Only visible controls participate; Source is skipped when
only one provider is available. See [keybindings](keybindings.md#navigation) for
each control's keys.

The pane is open unless `hide_settings_pane = true`. `Ctrl+B` toggles it and
writes the new value back to that key, so it comes back the way you left it.
With the pane closed the playlist takes the full
frame width and one chrome row is kept above it: the active source on the left
and the volume meter on the right. The EQ readout, the speed indicator, and the
download counters are not drawn — `e` still cycles the EQ preset and `[` / `]`
still change speed, but their readouts are gone, so `Tab` skips the EQ and
speed stops rather than landing on a control you cannot see. Shuffle and repeat
return to the playlist header. The two rows this frees go to the playlist.
Closing Settings also hides Metadata without changing `show_metadata`.

A short body — a tall `vis_rows` leaves the pane few rows — sheds rows by how
readily they are missed rather than by position: the network counters go first,
then the EQ band gains, then shuffle and repeat together. Hidden controls are
also skipped by `Tab` and `Shift+Tab`. Those groups drop whole, so the pane can end
up a row shorter than it was given. A playlist header too narrow for all its
badges drops whole badges off the tail rather than clipping one mid-word.

`simplified = true` replaces the main playback view with the current track
artist/title, time, and seek-progress strip. It hides the visualizer, playback
controls, and playlist. Provider browsing and overlays keep their list-focused
layout. Start one session with `cliamp-x --simplified`.

`hide_help_bar = true` removes the key-binding hint bar above the status line
and gives that row back to the playlist. The full keymap stays available with
`?`. `Ctrl+G` toggles the bar and writes the new value back to this key, so the
bar comes back the way you left it. Use `cliamp-x --no-help-bar` to hide it for
one session, or `cliamp-x --help-bar` to show it despite this setting. Simplified
mode draws neither the hint bar nor a playlist, so it is unaffected by this
setting.

`expanded = true` starts with the playlist at the expanded height, so the list
gets every body row the terminal has left instead of the shorter default, and
`Ctrl+X` is not needed on every launch. The key keeps working and collapses the
view as before. Start one session with `cliamp-x --expanded`, or `--no-expanded`
to start collapsed despite this setting.

List views such as provider browsing, file selection, queues, playlists, search
results, themes, and keybindings use a content-first layout. This layout replaces
the visualizer and detailed controls with a compact now-playing summary. It leaves
more rows for navigation. The visualizer picker keeps its live preview.

### Preview pane

When the terminal has rows to spare, a preview pane appears under the list. It
shows the cover of the highlighted album, the picture of the highlighted
artist or playlist, or the thumbnail of the highlighted song, with a few
details: artist, year, length, number of songs, listeners or plays. Where
nothing is highlighted it shows the song that plays. The picture keeps its
shape, so a plain YouTube video shows as a 16:9 thumbnail and a YouTube Music
album as a square cover. In kitty and Ghostty the pane shows the real image
through the kitty graphics protocol; other terminals, and tmux, get a
lower-detail version drawn with colored half-block characters.
`preview_images` picks the method: `"auto"` (the default), `"kitty"` or
`"blocks"`. Covers load in the background, and the
rows just below and above the cursor load ahead of it, so moving through a
list rarely waits; they are cached in `~/.local/share/cliamp-x/artwork`.
`hide_preview = true` removes the pane.

### Metadata

The read-only `Metadata` section sits below Settings and is hidden by default.
In the main playback view, `Ctrl+I` toggles it and saves the
top-level `show_metadata` preference. The default is `show_metadata = false`,
so the existing layout stays unchanged until you enable it. Metadata is not a
separate Tab stop, and the toggle is inactive while an input field is active.

`Ctrl+I` requires enhanced keyboard reporting to distinguish it from `Tab`.
On terminals that send both keys identically, `Tab` navigation takes precedence;
use lowercase `i` for full info or set `show_metadata = true` in the config.

Metadata describes the **highlighted playlist item**, not necessarily the item
currently playing: a song, a podcast episode or a live stream. Moving the highlight
uses fields already loaded on that track, with no provider or network lookup.
Provider browsers and other list views do not show the section.

Available fields are Title, Artist (Show for podcast episodes), Album, Genre,
Date or Year, Track or Episode, and Length. Album is omitted when it duplicates
the artist/show. A podcast publication Date takes precedence over Year. A
yt-dlp stream that is live shows Type: Live stream. Live now-playing text appears as Playing only when the selected stream
is the one playing. Unknown fields are omitted.

In the full two-column layout, opening Metadata can borrow visualizer rows,
keeping at least one row when the visualizer is enabled. It does not overwrite
`vis_rows`; closing Metadata restores the configured height as space allows.
It never removes the six direct settings (Source, Volume, EQ, Shuffle, Repeat,
Speed) to make room. If there is no room for details after those controls, the
section stays hidden. When fields exceed the section's row budget, it shows
`i: more details`.

The compact Metadata section omits Path. From the playlist, lowercase `i` opens
the full, scrollable info view for the same highlighted item, including Path.
Use `Up`/`Down` or `j`/`k` to scroll, and `i` or `Esc` to close it.

When enabling Metadata in a narrow or simplified layout, with Settings closed,
or with a sidebar too short for details, `Ctrl+I` opens that full info overlay
instead. The preference remains saved so the section appears when you return
to a wide playback layout with enough room and Settings open.

## Playback: normalization, crossfade and SponsorBlock

### Loudness normalization

`normalize` plays every song at about the same perceived level, so a loud
remix does not jump out after a quiet track:

- `"track"` (the default) levels each song to `normalize_target` LUFS
  (default -14, the level YouTube plays at).
- `"album"` keeps the balance inside an album: a local file with a ReplayGain
  album tag is leveled by its album gain.
- `"off"` plays songs as they are.

`n` cycles the three modes while playing and saves the choice. Local files
use their ReplayGain tags (`REPLAYGAIN_TRACK_GAIN` and
`REPLAYGAIN_ALBUM_GAIN`). Any other song, such as a YouTube Music track, is
measured while it plays (ITU-R BS.1770 integrated loudness): the gain moves
gently to the target during the first seconds and settles as the measurement
does, so it never pumps. The measurement is saved in
`~/.local/share/cliamp-x/loudness.json`, and the next play of the song starts
at the right level. Gains are limited to -15 dB and +6 dB, and a soft clipper
keeps boosted peaks from distorting.

### Crossfade

`crossfade = N` overlaps the last N seconds of a song with the first N of the
next one (0-12; 0 turns it off). The outgoing song fades along a cosine curve
and the incoming one along a sine, which keeps the combined level steady.
Consecutive songs of the same album play back to back instead, so live albums
and continuous mixes stay seamless. Crossfading needs the next song ready
early, so the player starts loading it N seconds sooner than usual.

### SponsorBlock

The `[sponsorblock]` section skips the parts of YouTube videos that the
[SponsorBlock](https://sponsor.ajay.app) community marked. It is on by
default:

```toml
[sponsorblock]
enabled = true
categories = ["music_offtopic", "intro", "outro", "sponsor", "selfpromo"]
```

The other categories are `interaction`, `preview` and `filler`. Lookups send
only the first four characters of the SHA-256 hash of the video ID. The
segments of the playing song and of the next one are looked up ahead, so an
intro is skipped by starting the song after it. A segment that runs to the
end of the song moves to the next one, and the status line names each skip.
Marked parts show on the seek bar in the warning color. Each segment is
skipped once per play, so seeking back into it plays it.

## Audio output device

`audio_device` selects the output device when cliamp-x starts. Leave it empty to
use the system default output. To find the device names, run this command:

```sh
cliamp-x --audio-device list
```

The command prints the description and the name of each device, then exits.
A `*` marks the active device. Put the name in `audio_device`:

```toml
audio_device = "alsa_output.usb-FiiO_K5_Pro-00.analog-stereo"
```

- On Linux, cliamp-x sets `PIPEWIRE_NODE` to the name before it opens the audio
  output. PipeWire then sends the cliamp-x stream to that device. Without
  PipeWire, the key has no effect.
- On macOS, cliamp-x makes the device the system default output while it runs.
  It restores the previous default output when it exits. The value can be the
  name or the description.
- On Windows, the key has no effect at startup. The `d` device picker changes
  the system default output instead.

When you select a device with `d` or `cliamp-x device <name>`, the TUI saves it to
`audio_device`. To use a different device for one session, run
`cliamp-x --audio-device <name>`.

## Value syntax

cliamp-x reads a subset of TOML. These rules apply to every key:

- Put a string in double quotes or single quotes. cliamp-x removes one pair of quotes.
- Inside double quotes, write `\\` for a backslash and `\"` for a double quote. cliamp-x keeps every other backslash as you type it, so `"D:\new"` stays a Windows path.
- Single quotes are literal. cliamp-x decodes no escapes inside them.
- An unquoted string also works. cliamp-x keeps all of its text, including a `#`.
- A bool is `true`, `false`, `t` or `f` in any letter case. `1` and `0` also work. cliamp-x ignores any other value and keeps the default.
- A number such as `volume`, `speed` or an `eq` band must be finite. cliamp-x ignores `nan`, `inf` and any other text and keeps the default.
- In a list, a comma inside a quoted item stays part of the item, as in `allowed_binaries = ["my, tool", "ffmpeg"]`.
- To add a comment at the end of a line, put whitespace and then `#`. This works after a quoted string, a number, a bool, a list in square brackets such as `eq` or `allowed_binaries`, or a section header such as `[ytmusic]`. After a closing quote or a closing square bracket, the whitespace is optional. After an unquoted string, the `#` and the text after it stay part of the value. This rule also applies to an unquoted last string item of a list without square brackets, such as `allowed_binaries = ffmpeg, curl # x`.
- A plugin gets each key of its `[plugins.<name>]` section as a string. cliamp-x reads `enabled`, and the `disabled` and `allowed_binaries` lists of `[plugins]`, with these rules.

```toml
volume = -6                       # quieter start
shuffle = True                    # any letter case
eq = [3, 2, 0, 0, 0, 0, 0, 0, 1, 2]  # custom curve
initial_directory = 'D:\Music'    # single quotes keep backslashes

[ytmusic]                         # YouTube Music
cookies_from = "brave-origin"     # browser that holds the sign-in
```

## Secrets from Environment Variables

Set a string value in `config.toml` to `$VAR_NAME` or `${VAR_NAME}` to read it from an environment variable. This keeps passwords, tokens, and client secrets out of the file.

```toml
[plugins.lastfm]
api_key = "${LASTFM_API_KEY}"
```

Rules:

- Interpolation occurs only when the **entire** value is `$NAME` or `${NAME}`. cliamp-x keeps mixed values such as `"p@$$word"` literally. No escaping is required.
- Variable names match `[A-Za-z_][A-Za-z0-9_]*`.
- If the variable is unset, the value is empty (the same as if you had left it blank).
- cliamp-x always reads a whole `$NAME` or `${NAME}` value from the environment. A password such as `$Secret1` cannot be stored as literal text.
- Works for any string field, including plugin config under `[plugins.<name>]`.

## YouTube Music

The YouTube Music provider is on unless `[ytmusic]` sets `enabled = false`.
The `[yt]` and `[youtube]` headers also work. All keys are optional:

```toml
[ytmusic]
# Browser that holds the sign-in, in yt-dlp syntax: brave, brave-origin,
# chrome, chromium, edge, firefox, opera, vivaldi, or a browser:profile path.
# Empty detects the first browser with a YouTube sign-in.
cookies_from = "brave-origin"
# Expand list= URLs to the whole playlist (true) or play one video (false).
expand_playlist = true
# Language and country of the YouTube Music pages (default: from $LANG).
language = "es"
region = "CL"
# When the queue ends, keep playing songs similar to the last one.
auto_radio = true
# Add the songs you play to your YouTube Music history. Off by default.
report_history = false
```

See [YouTube Music](youtube-music.md) for the sign-in and the menus.

## Default Provider

Set the provider that cliamp-x opens at start:

```toml
provider = "ytmusic"
```

Valid values: `ytmusic` (default) and `local`.

You can also override this setting on the CLI: `cliamp-x --provider ytmusic`.

See [audio-quality.md](audio-quality.md) for sample rate, buffer, bit depth, and resample quality settings.

## WSL2 (Windows Subsystem for Linux)

cliamp-x uses ALSA for audio on Linux. WSL2 does not expose ALSA hardware directly. WSLg provides a PulseAudio server that ALSA can use.

If you see `ALSA lib pcm.c: Unknown PCM default`, use these two steps:

**1. Install the ALSA PulseAudio plugin:**

```sh
sudo apt install libasound2-plugins
```

**2. Create `~/.asoundrc` to route ALSA through PulseAudio:**

```sh
cat > ~/.asoundrc << 'EOF'
pcm.default pulse
ctl.default pulse
EOF
```

WSLg must be active. `echo $PULSE_SERVER` should print a path. If it is empty, use Windows 11 with WSLg enabled. Run `wsl --shutdown`, then reopen the terminal.

## ffmpeg (optional)

AAC, ALAC (`.m4a`), Opus, and WMA playback require [ffmpeg](https://ffmpeg.org/):

```sh
# Arch
sudo pacman -S ffmpeg
# Debian/Ubuntu
sudo apt install ffmpeg
# Fedora
sudo dnf install ffmpeg
```

MP3, WAV, FLAC, and OGG work without ffmpeg.

cliamp-x starts a local file that ffmpeg decodes in the background and shows
`Buffering...` until ffmpeg sends audio. The UI stays responsive on a slow
disk or network mount.

## Download directory

By default, TUI `Ctrl+S` saves yt-dlp downloads and temporary audio files in
`~/Music/cliamp-x`. To use another directory, add this setting and restart cliamp-x:

```toml
[downloads]
directory = "/media/usb/CLAPt/Music"
```

An empty value keeps the default. Relative paths are rejected; literal `~` is not
expanded. Missing directories are created. Mount external drives first because
cliamp-x does not check mount status. The IPC `save` operation uses the same
directory, in the TUI and in headless mode. `Ctrl+S` files remain ordinary
local audio files; `D` downloads, below, also play in place of the stream.

### Offline downloads

`D` downloads the highlighted song, album, playlist or artist (its top songs)
for listening without a connection. It works in the provider pane, the
browser, search results and the queue. Songs download one at a time with
yt-dlp, and the status line shows the progress (`↓ 3/12 · Song 45%`).

Each song keeps YouTube's own AAC audio (`.m4a`) when there is one, so it is
not re-encoded, and gets its cover (1200 px for YouTube Music songs), title,
artist, album, year and, for an album, track number. Files land in
`<directory>/<Artist>/<Album>/`, singles in `<Artist>/Singles/`. Without
ffmpeg the files keep the audio but no tags.

A downloaded song plays from its file instead of streaming, also offline;
`~/.local/share/cliamp-x/offline.json` maps the songs to their files. The
preview pane marks downloaded songs with `↓`. When the directory is inside
your music folder (the default is), the downloads also join the local music
menus.

## Listening statistics

`S` opens the statistics of what you listened to in cliamp-x: listening time,
plays, distinct songs and artists, the streak of days in a row, the activity
of the last 30 days, and the top artists, songs and genres. Left and Right
switch between the last week, month, year and all time. A song counts once it
played for 30 seconds, or half of it when it is shorter. The log is
`~/.local/share/cliamp-x/plays.jsonl` and stays on this computer.

## Proxy

cliamp-x reads the proxy variables `HTTP_PROXY`, `HTTPS_PROXY`, `ALL_PROXY` and
`NO_PROXY`. The lowercase names also work. Audio streams, provider API and
sign-in requests, lyrics, feeds, remote
playlists, plugin installs and `cliamp-x upgrade` use them.

```sh
ALL_PROXY=socks5h://127.0.0.1:1080 cliamp-x
HTTPS_PROXY=http://proxy.example:3128 cliamp-x
```

- `HTTP_PROXY` applies to `http://` requests, and `HTTPS_PROXY` applies to
  `https://` requests.
- `ALL_PROXY` applies when the variable for the scheme is not set. Audio
  streams use `ALL_PROXY` only for a `socks5://` or `socks5h://` proxy.
- `NO_PROXY` lists the hosts that cliamp-x connects to directly. cliamp-x does not
  send a request for `localhost` or a loopback address through a proxy.
- API requests accept a user and password in a SOCKS5 proxy URL, such as
  `socks5h://user:pass@proxy.example:1080`. Audio streams refuse a SOCKS5 proxy
  URL with a user and password, because SOCKS5 sends them as clear text.
- yt-dlp reads these variables itself.
- ffmpeg opens HLS streams, and remote streams in a format that cliamp-x does
  not decode itself. ffmpeg reads only the lowercase `http_proxy` and
  `no_proxy` variables. It uses only an `http://` proxy, and it ignores
  `HTTPS_PROXY`, `ALL_PROXY` and SOCKS5 proxies.
- The `cliamp.http` Lua plugin API does not follow these rules.

To send the ffmpeg streams through a proxy too, also set `http_proxy`:

```sh
HTTPS_PROXY=http://proxy.example:3128 http_proxy=http://proxy.example:3128 cliamp-x
```

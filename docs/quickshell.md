# Quickshell Now-Playing Widget (Omarchy)

A notification-sized "now playing" card for [Quickshell](https://quickshell.org), tailored for [Omarchy](https://omarchy.org). Lives in the repo at [`contrib/quickshell/`](../contrib/quickshell).

It pulls live spectrum data from a running cliamp over the IPC socket, draws a Winamp 2-style segmented analyzer, and reads colors directly from the active Omarchy theme so it restyles in lock-step when you swap themes.

> This widget is built for an Omarchy setup. It uses Omarchy's `~/.config/omarchy/current/theme/colors.toml` as the source of truth for its palette. On a non-Omarchy system the panel still works but falls back to the default kanagawa-dragon-ish colors baked into `NowPlaying.qml`.

## What you get

- 320 x 140 card centered along the bottom of every screen
- Full-width Winamp 2-style LED spectrum analyzer with falling peak caps
- Track title + artist, click-to-seek progress bar, time readout
- Pristine vector transport icons (prev / play-pause / next), no font dependency
- Theme follows the active Omarchy theme (background, foreground, accent, color1..color8, selection_background)
- Card border tracks the muted slot of the Omarchy palette (`selection_background`, falling back to `color8`)
- Click the card and press `Esc` or `Q` to dismiss

## Quick start

Make sure cliamp is running first, then either run the widget directly:

```sh
qs -p contrib/quickshell/shell.qml
```

Or install it as a named Quickshell config:

```sh
mkdir -p ~/.config/quickshell
ln -s "$PWD/contrib/quickshell" ~/.config/quickshell/cliamp
qs -c cliamp
```

## Requirements

- Quickshell 0.2+
- A running cliamp (Linux only; the widget needs cliamp's MPRIS service and IPC socket)
- Omarchy, for the theme integration. Without Omarchy the palette stays on the built-in defaults.

## Files

| File | Purpose |
| --- | --- |
| `shell.qml` | Entry point. Per-screen `PanelWindow`, anchors and Esc/Q key binding. |
| `NowPlaying.qml` | The card itself: layout, theme load, MPRIS bindings. |
| `Visualizer.qml` | Winamp 2 LED spectrum analyzer drawn on Canvas2D. |
| `BandStream.qml` | Wraps `cliamp visstream`, exposes the 10-band frames as reactive properties. |
| `MediaIcon.qml` | Resolution-independent transport icons (prev / play / pause / next / stop). |
| `TransportButton.qml` | Hoverable button shell that hosts a `MediaIcon`. |

## Theme mapping

The widget watches `~/.config/omarchy/current/theme/colors.toml` via Quickshell's `FileView` and re-applies live whenever Omarchy rewrites the file (which happens on every `omarchy-theme-set`). The widget tolerates the brief rm/mv window during theme swap and retries automatically.

| Omarchy key | Widget role |
| --- | --- |
| `background` | card background |
| `foreground` | title text |
| `accent` | spectrum bars (low rows), progress fill |
| `color1` | red slot, top rows of the spectrum (loud) |
| `color2` | playing indicator (green slot) |
| `color3` | peak markers, mid rows of the spectrum, hover color |
| `color8` | secondary text (artist, time readout) |
| `selection_background` | card border (falls back to `color8`) |

## Layout

The card has no rounded corners (sharp 90-degree edges) to match a terminal aesthetic. The visualizer is full-width across the top with 10 px breathing room above the bars. Track info, progress bar, and transport buttons follow below in tighter compact spacing.

To reposition the card, edit `shell.qml`:

```qml
PanelWindow {
    anchors { bottom: true; left: true; right: true }
    margins { bottom: 16 }
    implicitHeight: 140
    color: "transparent"

    NowPlaying {
        width: 320
        height: parent.height
        anchors.horizontalCenter: parent.horizontalCenter
    }
}
```

Swap `bottom` for `top` to flip to the top edge, or replace the `horizontalCenter` anchor with `anchors.left` / `anchors.right` to push it into a corner.

## The data path

Two cliamp APIs feed the widget. Both are documented in detail in [Remote Control](remote-control.md), but here is the short version:

### `cliamp status --json`

Returns full state including the active theme. The widget does not currently use this for theme (it reads Omarchy directly), but the `theme` field is available for non-Omarchy consumers:

```json
{
  "ok": true,
  "state": "playing",
  "track": { "title": "...", "artist": "..." },
  "position": 123.4,
  "duration": 240.0,
  "visualizer": "ClassicPeak",
  "theme": {
    "name": "Kanagawa Dragon",
    "accent": "#658594",
    "fg": "#c5c9c5",
    "bright_fg": "",
    "green": "#8a9a7b",
    "yellow": "#c4b28a",
    "red": "#c4746e"
  }
}
```

### `cliamp visstream`

Streams the live 10-band spectrum as newline-delimited JSON, one frame per line, defaulting to 30 fps. Holds a single IPC connection open for the duration:

```sh
cliamp visstream --fps 30
```

Each line:

```json
{"ok":true,"visualizer":"Bars","bands":[0.93,0.81,0.62,0.48,0.31,0.22,0.14,0.09,0.04,0.01]}
```

The widget runs this once and listens to stdout line by line via Quickshell's `SplitParser`. If cliamp isn't running yet, the widget retries the spawn every 2 s until it succeeds. The bands are normalized to [0, 1] in the same shape cliamp uses internally for spectrum visualizers.

## Customizing the visualizer

The analyzer is in `Visualizer.qml`. To tweak the LED look:

```qml
property int segH:   3   // segment height in px
property int segGap: 1   // gap between segments

property color barColor:    "#a9b665"  // bottom 55% of the height
property color accentColor: "#d8a657"  // middle 30% and peak markers
property color warnColor:   "#ea6962"  // top 15%
```

By default these are wired to the Omarchy theme by `NowPlaying.qml` (`barColor` = accent, `accentColor` = `color3`, `warnColor` = `color1`).

## Troubleshooting

**The card appears but the bars are empty.** Confirm cliamp is running and that `cliamp visstream --fps 5` outputs JSON lines in a terminal. If it errors with "cliamp is not running", start cliamp first.

**The card sits on top of my Waybar.** That's intentional: `exclusionMode: ExclusionMode.Ignore` keeps the panel from claiming a strip. Move it to a different edge in `shell.qml` if you want a clean overlay.

**Theme colors are wrong after an Omarchy theme swap.** The widget logs are silenced (`printErrors: false`) and a 400 ms one-shot retry fires after any load failure. If colors still look stale, run `omarchy-theme-set <name>` again — Omarchy's atomic move sometimes races at startup.

**Esc/Q does not exit.** Click anywhere on the card first to grant keyboard focus. The widget uses `WlrLayershell.keyboardFocus: OnDemand`, which is the least-intrusive focus mode on Wayland.

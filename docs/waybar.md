# Waybar module and desktop notifications

## Waybar

`cliamp-x waybar` prints the playing song as a [Waybar](https://github.com/Alexays/Waybar)
custom module: one JSON line each time the playback changes, with the text,
a tooltip (artist, song, album and time), a CSS class (`playing`, `paused` or
`stopped`) and the progress as `percentage`. While cliamp-x is not running the
module stays empty and checks again every two seconds, so Waybar can start it
at login.

Add the module to `~/.config/waybar/config.jsonc` (Omarchy keeps Waybar there
too) and put `"custom/cliamp"` in one of the `modules-*` lists:

```jsonc
"custom/cliamp": {
  "exec": "cliamp-x waybar",
  "return-type": "json",
  "format": "{}",
  "on-click": "cliamp-x toggle",
  "on-click-right": "cliamp-x next",
  "on-click-middle": "cliamp-x prev",
  "on-scroll-up": "cliamp-x remote call volume.adjust --params '{\"value\":2}'",
  "on-scroll-down": "cliamp-x remote call volume.adjust --params '{\"value\":-2}'",
  "escape": true
}
```

Style it in `~/.config/waybar/style.css`:

```css
#custom-cliamp { padding: 0 8px; }
#custom-cliamp.paused { opacity: 0.6; }
#custom-cliamp.stopped { opacity: 0.4; }
```

`--format` sets the text from `{icon}`, `{artist}`, `{title}` and `{album}`
(default `{icon} {artist} - {title}`), and `--max-length` cuts it with an
ellipsis (default 45 characters, 0 for no limit). The module reads the same
events as `cliamp-x remote events runtime.playback`; see
[Remote control](remote-control.md).

## Desktop notifications

Each song that starts shows a notification through the freedesktop
notification service (mako, dunst, swaync, GNOME, KDE...), with the song, the
artist and album, and the cover. A new song replaces the last notification
instead of stacking. When the notification server supports actions, the
notification has **Next** and **Like** buttons: Like gives the playing song a
♥ (a "like" in YouTube Music). `notifications = false` in `config.toml` turns
them off. Headless mode (`cliamp-x --daemon`) shows them too.

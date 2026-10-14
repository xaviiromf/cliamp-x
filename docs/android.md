# Android

cliamp-x runs on Android phones (arm64, Android 10 or later) as an app with a
touch interface: tabs, a mini-player and a full-screen player, in the cliamp-x
look and themes. The same Go core plays the music, so YouTube Music, local
files, the EQ, lyrics and the visualizers work as on the desktop. The terminal
interface is still there as **Modo terminal**.

## Install

Download `cliamp-x-<version>-arm64-v8a.apk` from the
[releases](https://github.com/xaviiromf/cliamp-x/releases) and open it on the
phone. Android asks once to allow installs from that app. Obtainium can track
the releases too.

The first start asks for notifications and music access, then unpacks Python
and ffmpeg for a few seconds and downloads the newest yt-dlp.

## Sign in to YouTube Music

Open **Ajustes → Cuenta → Iniciar sesión** and sign in to Google in the page
that opens. If Google refuses that browser, export `ytmusic_cookies.txt` from
the desktop (`~/.config/cliamp-x/ytmusic_cookies.txt`), copy it to the phone
and use **Importar cookies.txt**. Delete the copy afterwards; it holds your
session.

## Use

| Where | What |
|-------|------|
| Inicio | Recently played, your home shelves, Para ti albums and artists, new releases, genres |
| Buscar | Search (the only place the keyboard opens), recent searches, genre grids |
| Biblioteca | Liked songs, history, local music, your playlists, downloads, ratings, statistics |
| Mini-player | Tap or swipe up to open the player; swipe sideways to skip |
| Player | Swipe the artwork to skip, tap it to switch artwork and visualizer, tap the visualizer to change mode, swipe down to close |
| Rows | Tap to play from there; long-press or `⋮` for actions (play next, queue, like, more like this, go to album or artist, download) |
| Queue | Swipe a row to remove it, drag `≡` to reorder |

The main interface is portrait only. Modo terminal rotates.

## Settings

**Ajustes** holds the theme (all cliamp-x themes, with a live preview), the
visualizer (every mode of the terminal, drawn from the core's own frames), the EQ, sleep
timer, speed, normalization, crossfade, auto radio, the music folder, the
battery setting, updates, the yt-dlp update and Modo terminal.

**Ajustes → Apariencia** also sets the look of the app itself:

- **Fuente**: JetBrains Mono Nerd Font (default) and Adwaita Mono are built
  in. 18 more free monospace fonts download on a tap (Fira Code, Hack,
  Cascadia Code, Source Code Pro, IBM Plex Mono, Ubuntu Mono, Victor Mono,
  Space Mono, Monaspace Neon, Commit Mono, Geist Mono, Departure Mono, VT323,
  Press Start 2P, Terminus, Inconsolata, mononoki, Fantasque Sans Mono). Each
  comes from its official release or the Google Fonts repository and is
  checked against a pinned SHA-256 before use. The font applies to the whole
  interface; the top of the picker previews it. Iosevka is not offered: it
  ships only as archives of 23 MB or more (99 MB for the plain family), over
  the 20 MB per-font download cap.
- **Tamaño del texto**: Pequeño, Normal, Grande or Muy grande, on top of the
  system font size.
- **Densidad de las listas**: Compacta, Normal or Amplia row heights (never
  under 48 dp).
- **Esquinas**: Cuadradas (the cliamp-x look) or Redondeadas.
- **Vibración al mantener pulsado**: on or off.

**Fondo** draws an animated background behind the whole app, the player and
the sheets included, in the colors of the current theme: Sólido (the
default, no animation), CRT, Ondas, Cyberpunk, Retro, Glitch, Matrix,
Estrellas, Plasma, Aurora, VHS, Tramado, Tron, Bokeh, Circuito, Topográfico
and Grano. Two sliders set how much of it shows in the lists and in the
player (0 to 100); the theme's background color stays over it, darker (or
lighter on a light theme) as the slider rises, so text keeps its contrast
(4.5:1, or the theme's own when lower) wherever the background is brightest.
Themes with little contrast to spare show it faintly. It moves at 20 fps
only while the app is on screen, slower with the system's reduced motion,
and needs Android 13 or newer (older versions draw a still gradient of the
theme). A moving background costs battery: each step redraws the whole
screen, about half a CPU core on a Galaxy A55 while the app is in view. **Animaciones** picks
Lenta, Normal or Sin animación for the screen, player page, artwork and
cover-palette transitions; Sin animación also holds the background still.

## Background playback

Music keeps playing with the screen off, with controls in the notification
and on the lock screen. If One UI or another system stops it after a while,
open **Ajustes → Batería** and let cliamp-x run unrestricted (on Samsung: add
it to "Apps que nunca se suspenden").

## Updates

**Ajustes → Buscar actualizaciones** checks the GitHub releases (channel
Estable or Nightly), downloads the APK, checks its SHA-256 and opens the system
installer. Only APKs signed with the same key update each other.

yt-dlp updates itself once a day. YouTube changes often break old yt-dlp
versions; **Ajustes → Actualizar yt-dlp** forces an update.

## Build from source

`android/README.md` has the toolchain, `make android-core` and the Gradle
commands.

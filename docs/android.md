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

The first start shows three short steps while Python and ffmpeg unpack and
the newest yt-dlp downloads behind them:

1. **Permisos** (Permissions): notifications (the music controls), local music, and the
   battery optimization, each with its state and the button that asks.
2. **YouTube Music**: sign in, import a `cookies.txt`, or **Omitir** (Skip).
3. **Elige un skin** (Pick a skin): a tap shows it at once; **Empezar**
   (Start) opens the app.

The app follows the phone's language: English, or Spanish on a phone set to
Spanish. **Ajustes → Sistema → Idioma** (Settings → System → Language in
English) overrides it: **Sistema**, **English** or **Español**. Android 13
and later also list the app in the system's per-app language settings. A
change restarts the core, so its own texts and the YouTube Music shelves
change language too, in the phone's country.

## Sign in to YouTube Music

Open **Ajustes → Cuenta → Iniciar sesión** and sign in to Google in the page
that opens. If Google refuses that browser, export `ytmusic_cookies.txt` from
the desktop (`~/.config/cliamp-x/ytmusic_cookies.txt`), copy it to the phone
and use **Importar cookies.txt**. Delete the copy afterwards; it holds your
session.

**Ajustes → Cuenta → Cerrar sesión y borrar datos** signs out and erases all
cliamp-x data on the phone (sign-in, settings, lists, history, downloads) after
a confirmation. It cannot be undone; the app opens as a fresh install.

## Use

| Where | What |
|-------|------|
| Inicio | Recently played, your home shelves, Para ti albums and artists, new releases, genres |
| Buscar | Search (the only place the keyboard opens), recent searches, genre grids |
| Biblioteca | Liked songs, history, local music, Radio y podcasts, your playlists, downloads, ratings, statistics |
| Mini-player | Tap or swipe up to open the player; swipe sideways to skip |
| Player | Swipe the artwork to skip, tap it to switch artwork and visualizer, tap the visualizer to change mode, swipe down to close. `⋮` holds the full-screen lyrics, the background visualizer and Modo estéreo |
| Lyrics | **[Letras]** shows them in place of the artwork; a tap there opens them full screen (karaoke): the line that sings is highlighted and kept in the middle, and a tap on a synced line seeks to it. `−0.5 s` and `+0.5 s` (in the player view and on the full-screen one) move the lyrics earlier or later, for songs whose timestamps are off; the offset is saved in the core, as on the desktop |
| Rows | Tap to play from there; long-press or `⋮` for actions (play next, queue, like, more like this, go to album or artist, download) |
| Queue | Swipe a row to remove it, drag `≡` to reorder |

**Biblioteca → Radio y podcasts** plays an address you paste: an internet
radio stream, an `.m3u` or `.pls` playlist, or a podcast RSS feed (it must
start with `http://` or `https://`). **[ Reproducir ]** plays a stream or
playlist at once, and the player shows the song the station sends (ICY) with
the station's name, or for a bare stream address its file name, under it. For
a podcast feed it opens the episodes, newest first with date and length, and
a tap plays from that episode on. **[ Guardar ]** keeps the address in a list
on the phone, named after the podcast, the station a `.pls` or `.m3u` names,
or else the address's host; `⋮` on a saved row renames or removes it, and a tap
plays it or opens its episodes. The list needs a core with `url.list`
(see `docs/remote-control.md`).

**Biblioteca → Estadísticas → Tu año en cliamp-x** tells this year (or the
last one) as swipeable cards in the current theme and font: minutes and days
of music, top artists, songs, albums and genres, the longest streak, the first
song of the year and the one replayed most in one day. **Compartir imagen**
under a card shares it as a PNG through the system share sheet. The figures
come from the core's play log (`stats wrapped`), which stays on the phone.

**Biblioteca → Playlists → Local → Importar M3U** makes a local playlist from an
`.m3u` or `.m3u8` file the system file picker opens, named after the file.
URLs (YouTube links too) stay as they are. A file path counts when the file
exists: an absolute one as it is, a relative one from the folder of the M3U
file or from the music folders. The other entries are skipped, and the status
line says how many. A file is read up to 2 MB and 5000 entries. **⋮ →
Exportar M3U** on a playlist writes an `#EXTM3U` file with the system "create
document" picker: local songs by path, YouTube songs by watch URL, each with
its duration and "artist - title".

On a phone the main interface stays in portrait; Modo terminal and Modo
estéreo rotate. On a tablet or an open foldable (600 dp or more on the short
side) it turns too: a landscape player puts the artwork beside the controls,
and from 840 dp wide the player shows two panes, the player and beside it the
queue, or the EQ or full-screen lyrics it opens.

### Accessibility

Every button has a label for TalkBack, in English or Spanish. Toggles say
their state (shuffle on or off, the selected chip, the playing row), the
seek bar and the EQ bands move with TalkBack's adjust gestures or the volume
keys, rules read as headings, and the song that starts and the status line's
messages are announced. The swipes have custom actions in TalkBack's actions
menu: skip (Next, Previous) on the mini-player and the artwork, remove from
the queue or playlist, and Move up or Move down in the queue. Text follows
the system font size up to 200 %: rows grow, the player's chips wrap and its
artwork shrinks to keep every control on screen (the player scrolls when even
that is not enough), the EQ's frequency and gain labels shrink to fit their
column, and a setting keeps its whole label (a long value ends in …).

**Modo estéreo** is an always-on screen for a phone on a stand: the clock,
the artwork, the song and the visualizer. It dims from 22:00 to 07:00 and
moves a few pixels every minute against burn-in; a tap leaves it. Open it
from the player's `⋮`, or let it start by itself while charging: **Ajustes →
Modo estéreo al cargar** opens the system's screen saver settings, where you
pick Modo estéreo.

## Settings

**Ajustes** holds the theme (all cliamp-x themes, with a live preview), the
visualizer (every mode of the terminal, drawn from the core's own frames), the EQ, sleep
timer, speed, normalization, crossfade, auto radio, the music folders, the
battery setting, updates, the yt-dlp update and Modo terminal.

**Ajustes → Apariencia** also sets the look of the app itself:

- **Modo ligero**: Automático (default), Activado or Desactivado, for slow
  phones that play YouTube choppy or whose interface stutters. Automático
  turns it on when Android flags the phone as low RAM, it reports under 4 GiB
  of RAM (a phone sold with 4 GB reports about 3.7 GB), it has no 64-bit ABI,
  or its fastest core is under 2 GHz (a Moto G20 matches the RAM and the
  clock). While on, with nothing else to set: no visualizer behind the
  player and at most 10 frames per second in the other places, the background
  drawn as a plain color, no animations and no cover-color blend between songs
  (as Animaciones → Sin animación), covers decoded at half size, a 1 s audio
  buffer in the core (`buffer_ms` below 1000 is raised to 1000; a higher one
  stays) and the info of the next YouTube song fetched 8 s after the current
  one starts, so it does not compete with it. Your own Fondo, Animaciones and
  background visualizer choices are kept, not changed, and apply again when
  the mode is off. The core reads the buffer when it starts, so changing the
  mode restarts the core (the music stops for a moment). The setting shows
  whether it is on now.
- **Fuente**: JetBrains Mono Nerd Font (default) and Adwaita Mono are built
  in. 18 more free monospace fonts download on a tap (Fira Code, Hack,
  Cascadia Code, Source Code Pro, IBM Plex Mono, Ubuntu Mono, Victor Mono,
  Space Mono, Monaspace Neon, Commit Mono, Geist Mono, Departure Mono, VT323,
  Press Start 2P, Terminus, Inconsolata, mononoki, Fantasque Sans Mono). Each
  comes from its official release or the Google Fonts repository and is
  checked against a pinned SHA-256 before use. The font applies to the whole
  interface and the visualizer grid; the top of the picker previews it.
  Iosevka is not offered: it ships only as archives of 23 MB or more (99 MB
  for the plain family), over the 20 MB per-font download cap.
- **Tamaño del texto**: Pequeño, Normal, Grande or Muy grande, on top of the
  system font size.
- **Densidad de las listas**: Compacta, Normal or Amplia row heights (never
  under 48 dp).
- **Esquinas**: Cuadradas (the cliamp-x look) or Redondeadas.
- **Vibración al mantener pulsado**: on or off.
- **Visualizador de fondo en el reproductor**: the visualizer fills the
  player behind the artwork, dimmed under the text (also in the player's
  `⋮`). The strip under the artwork then hides.
- **Icono de la app**: Predeterminado, Verde, Ámbar, Synthwave or Game Boy.
  Android closes the app when the icon changes (the music keeps
  playing); open it again from the new icon. The launcher may take a few
  seconds to show it, and a home screen shortcut to the old one goes
  away; add it again from the app drawer. On Android 13 and newer every icon also has the themed
  (monochrome) version.

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
screen, about half a CPU core on a Galaxy A55 while the app is in view. It
holds still while a list scrolls. While nothing is touched, the app asks the
screen for 60 Hz with a moving background and in the player, where nothing
moves faster than 30 fps; a touch brings back the full rate. **Animaciones** picks
Lenta, Normal or Sin animación for the screen, player page, artwork and
cover-palette transitions; Sin animación also holds the background still.

**Skins** bundle a theme, a font, a background with its two intensities and a
visualizer mode, applied with one tap: CRT ámbar, Synthwave, Matrix, Game
Boy, Winamp clásico, Nord sereno, Papel claro and Vaporwave are built in.
**Guardar el actual como skin…** keeps the look on screen as a skin of yours;
its ⋮ renames or deletes it. A skin with a font not downloaded yet downloads
the font and uses it when it is here. **Exportar** writes all the
personalization (the look on screen, your skins and themes, text size,
density, corners, vibration and animations) to one JSON file through the
system file picker; **Importar** reads it back (1 MB at most; an entry that
is not valid is left out).

### Backup

**Ajustes → Copia de seguridad completa** keeps your data in one `.zip`, so a
reinstall (or **Cerrar sesión y borrar datos**) does not lose it. **Exportar**
writes the file through the system file picker; **Importar** reads it back.

The zip holds `manifest.json` (`{"format": "cliamp-x-backup", "version": 1,
"session": false}`) and only these files:

| In the zip | What |
|------------|------|
| `config/config.toml` | Settings and the music folders, with every credential line removed (scrobble tokens and keys, passwords) |
| `config/favorites.toml`, `history.toml`, `taste.json`, `ytmusic_genres.json` | Favorites, history, ratings, favorite genres |
| `config/playlists/*.toml`, `config/themes/*.toml` | Local playlists and your themes |
| `data/plays.jsonl`, `data/search_history.json` | The play log (statistics and Wrapped) and the search history |
| `app/personalization.json`, `app/prefs/*.json` | The Skins personalization, saved radio stations and a few app settings |

Downloads, caches, yt-dlp and logs are never included. Last.fm and
ListenBrainz keys are not exported either: sign in again after a restore.
The YouTube Music session (`ytmusic_cookies.txt`, `ytmusic_auth.json`) is out
unless you turn on **Incluir la sesión de YouTube Music**. That file then holds
your cookies, so anyone with it can use your account: keep it private.

**Importar** checks the whole file before it writes anything: a known format
version, only the paths above (no absolute path, no `..`), at most 24 MB per
file, 64 MB and 1000 files in all, and valid UTF-8 and JSON in each file. A
backup with the YouTube Music session in it must say so in its manifest. If
anything fails, nothing changes and the message says why. Otherwise, after
you confirm, the app stops the core, writes the files (each one whole or not
at all), starts the core again and reloads the screens. Files in the backup
replace the ones with the same name; playlists or themes that are not in the
backup stay.

**Tema** also has:

- **Editor de temas**: the seven colors of the theme on screen (bg, accent,
  bright_fg, fg, green, yellow, red) with a hue, saturation and brightness
  picker, a hex field, the contrast of each color with the background, and a
  live preview. **Guardar como…** writes `<name>.toml` to the core's themes
  directory, the same format as `~/.config/cliamp-x/themes/` on the desktop,
  and uses it.
- **Colores de la portada**: the core's cover theme, from the artwork of the
  song playing.
- **Colores del fondo de pantalla**: a dark theme made from the system
  wallpaper's colors, saved as the `wallpaper` theme;
  every text color keeps 4.5:1 contrast with its background.

**Ajustes → Red y descargas** sets the YouTube audio quality separately for
Wi-Fi and mobile data (Alta: the best format, Normal: up to 128 kbps,
Ahorro: up to 64 kbps). The app tells the core which network it uses; a
metered network counts as mobile data. **Calidad Premium** asks for the
256 kbps formats of a Premium account from the first try. **Descargas solo
con Wi-Fi** refuses a download on mobile data. **Caché automática** downloads
your liked songs, then the recently played ones, at most every 12 hours, up
to a number of songs and a size (an estimate of 128 kbps per song); it can
be held to Wi-Fi, and **Llenar ahora** fills it at once. The player shows the
format of the playing stream under the time, such as `OPUS · 160 kbps · 48 kHz`.

**Ajustes → Scrobbling** sends what you play to ListenBrainz (paste your
token from listenbrainz.org/settings; a token ListenBrainz does not accept
is not saved) or Last.fm (paste the key and secret
of your own API account from last.fm/api, tap **Conectar**, allow cliamp-x
in the browser, then tap **Ya lo autoricé**). **Desconectar** removes the
credential. Scrobbles that fail on a network error are sent later.

**Ajustes → Música local** lists the folders the app reads local music from
(`Music` on the phone's storage at first). **+ Carpeta** opens the system
folder picker: pick any folder on the phone's storage or an SD card, such as
`Download/MiMusica`, and tap **Usar esta carpeta**. It joins the list, and
**Biblioteca → Música local** lists the songs of every folder at once; a song
under two of them shows once. **Quitar** next to a folder takes its songs out.
The list stays after a restart. **Biblioteca → Música local** has the same
**+ Carpeta** at the top. Android does not let the picker choose the root of
the storage or `Download` itself; pick a folder inside. A cloud or app folder
(Drive, Recent) has no path on the phone, so the app says so and keeps the
list.

**Todo el teléfono** (Whole phone) replaces the list with every folder that
holds music on the phone, as Android's media index (MediaStore) knows it, and
reads them again at each start and each pull to refresh. While it is on, the
list cannot be edited by hand; turning it off keeps the last list. It needs the
local music permission.

**Ajustes → Sistema → Acerca de** (Settings → System → About) shows the
version, links to the source and to the original cliamp (cliamp-x is an
independent fork, not the official cliamp), the licenses (the APK is
GPL-3.0 because it includes youtubedl-android; the source is MIT), the
privacy policy and every third-party license in full.

Privacy: cliamp-x collects no data. It has no telemetry, analytics, ads or
server of its own. It talks only to the services it uses: YouTube Music,
GitHub (app and yt-dlp updates), the site of each font you download, LRCLIB
and NetEase (lyrics), SponsorBlock, the streams and links you open, and
ListenBrainz or Last.fm when you connect an account.

**Ajustes → Sistema → Enviar registro** (Send log) writes one text file of
about 300 KB at most and opens the share sheet, for a bug report: a header
(app, Android and device, memory, cores, network, battery saver, sample rate
and the settings that matter for playback), the last 1500 lines of
`cliamp.log`, the last 1000 lines of the app's log and its crash log. Nothing
is sent by itself; the file lists what you played, so read it before you send
it. `cliamp.log` itself stays under about 2 MB: at 1 MB the core moves it to
`cliamp.log.1`, replacing the older one, and starts a new file.

## Background playback

Music keeps playing with the screen off, with controls in the notification
and on the lock screen. The notification and the lock screen player take
the theme's colors: they show the cover in the theme's background and accent
(the app keeps the real cover), or the cliamp-x logo for a song with none.
If One UI or another system stops it after a while,
open **Ajustes → Batería** and let cliamp-x run unrestricted (on Samsung: add
it to "Apps que nunca se suspenden").

**Reanudar al conectar auriculares** (Ajustes → Reproducción, off by default)
plays the paused song again when wired, USB or Bluetooth headphones connect.
The equalizer remembers the preset you pick for each output (the speaker,
wired headphones, USB, and each Bluetooth device by name) and switches to it
when that output connects; the EQ screen names the current output. The
automatic EQ by genre and a Personalizado curve are not remembered.

The core plays at the phone's native output rate (48000 Hz on most phones),
which the app reads from Android and passes in `CLIAMP_DEVICE_SAMPLE_RATE`.
Audio is then resampled once at most, and Android can give it a low-latency
track. A `sample_rate` set in `config.toml` still wins.

## Phone and car integrations

- **Widget:** a 4×1 mini player for the home screen with the cover, the song
  and its artist, and previous, play or pause and next, in the theme's
  colors. Tap the song to open the player. With cliamp-x closed, play starts
  it and resumes the last queue.
- **Quick Settings tile:** play or pause, lit while a song plays. Add it from
  the edit view of the Quick Settings panel.
- **Launcher shortcuts:** long-press the icon for **Reanudar**, **Buscar** and
  **Me gusta**.
- **Compartir:** in a YouTube Music song's menu and in the player's ⋯ menu,
  shares the song's music.youtube.com link.
- **Android Auto:** cliamp-x shows as a media app with Inicio (one folder per
  shelf), Biblioteca (your lists), Canciones que te gustan and Escuchado
  recientemente. Voice search ("Ok Google, pon … en cliamp-x") plays the songs
  it finds. Only Android Auto, the Google app and the system media controls
  may browse your library.

## Updates

**Ajustes → Buscar actualizaciones** checks the GitHub releases (channel
Estable or Nightly), downloads the APK, checks its SHA-256 and opens the system
installer. Only APKs signed with the same key update each other.

yt-dlp updates itself once a day. YouTube changes often break old yt-dlp
versions; **Ajustes → Actualizar yt-dlp** forces an update.

## Build from source

`android/README.md` has the toolchain, `make android-core` and the Gradle
commands.

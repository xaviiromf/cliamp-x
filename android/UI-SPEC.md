# cliamp-x for Android: native touch UI, design and implementation spec

**Status:** this is the contract between two implementers:
- the **Go agent**, who adds the IPC operations in the core;
- the **Android agent**, who builds the Kotlin UI.

**Base:** branch `android` at 02e757cb or later.

**What it replaces:** the "UI" row of `android/PLAN.md` §1 (TUI in a Termux view). The TUI stays as **Modo terminal**.

**What the user asked for:**
1. **Songs must play.** The bundled yt-dlp 2025.11.12 fails on today's YouTube. yt-dlp 2026.08.19 with the bundled QuickJS works. The app updates yt-dlp by itself, before the first play and daily after that.
2. **Navigation like Spotify or iTunes:** bottom tabs, a mini-player, a full-screen player, detail pages and long-press menus.
3. **The look of cliamp-x desktop:**
   - its themes;
   - one monospace font, with square corners;
   - section rules `── Label ──`, `[bracket]` values and single-glyph state markers;
   - the block spectrum in green, yellow and red;
   - the spaced logo `C L I A M P - X`.
4. **No keyboard:** buttons and gestures only. The keyboard opens only in the search field and in playlist-name fields.
5. **Keep cliamp-x's performance, customization and UX.** Every feature the desktop reaches with a key gets a touch path:
   - ♥, follow and save;
   - ratings (I/H/P), more like this (M) and remixes (X);
   - downloads (D, Ctrl+O) and statistics (S);
   - EQ, themes, visualizers, sleep timer, speed, normalization, crossfade, auto EQ, auto radio and mono;
   - lyrics, the queue and play-next, and local and YouTube playlists.

---

## 0. Decisions

| Topic | Decision |
|---|---|
| Main UI | Jetpack Compose + Material3, colored only from the cliamp-x theme. It replaces the TerminalView as the launcher screen |
| TUI | Kept as **Modo terminal** (`TerminalActivity`), opened from Ajustes and from the player's ⋮. The extra-keys row exists only there |
| Core | Same process model. The Go core runs in the pty owned by `CliampService`, and the native UI uses V2 IPC only. The core draws (`ui.render on`) only while Modo terminal is visible; otherwise it is `off`. No new render mode |
| yt-dlp | The app downloads the newest yt-dlp release itself, with timeouts, a SHA-256 check and an atomic rename over the file that `CLIAMP_YTDLP` runs. While yt-dlp has never been updated, the first play waits up to 45 s for the update. After that it checks daily |
| Default theme | "Default - Terminal colors" is drawn with the built-in `winamp` palette, the closest match to the green-on-black Termux view the user had |
| Visualizer | The core draws every mode (all 47 and any Lua mode) and `vis.frame` returns its rows with ANSI colors; the app draws them cell by cell on one Canvas in the monospace font (`Vis.kt` parses, `ui/Visualizer.kt` draws). Block, quadrant and sextant glyphs and braille dots are drawn as rectangles so they tile with no gap in any font; the 16 terminal colors of the Default theme map to the theme palette |
| Volume | Android media volume: the hardware keys and the system panel. The core's own volume is not exposed |
| Modo terminal only | Lua plugins, blocked words and artists, `ssh://` paths, `vis_palette`, the lyrics offset and the output device |

---

## 1. Principles

1. **Navigation like Spotify.**
   - Three tabs: **Inicio**, **Buscar**, **Biblioteca**.
   - A persistent **mini-player** above the tabs.
   - A **full-player sheet**: swipe up to open, swipe down to close.
   - **Detail pages** with a big `▶ Reproducir` key pill. Tapping a song plays its list from that song.
   - A long-press, or the visible `⋮`, on any item opens its action sheet.
2. **Touch only.**
   - Every action is a button, a chip, a row tap, a long-press or a gesture.
   - A text field exists only in Buscar and in the name sheet (new, rename, save queue).
   - The Buscar field is **not** focused when the tab opens. The keyboard opens only on a tap in the field.
   - Touch targets are at least 48 dp. Horizontal swipe areas keep 24 dp from the screen edges, which belong to the system back gesture.
3. **The desktop look, translated, never imitated with chrome.**
   - Flat surfaces, square corners, no shadows, one font: JetBrains Mono Regular and Bold.
   - Structure comes from typography: `── Rule ──`, `▸─ Current ──`, `[Valor]`, ` · ` separators, uppercase tags such as `RADIO · FUNDIDO`.
   - Fixed state glyphs:
     - `▶` playing, in green;
     - `♥` favorite, in red;
     - `Q` queued, in accent;
     - `!` unplayable, dim;
     - `●` active or unseen, in accent.
   - The braille spinner `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏` and the seek bar `━━━●───`.
   - Colors come only from the cliamp-x theme and change live.
4. **English and Spanish UI** (FINAL-PLAN T9). All app text is in `values/strings*.xml` in English, the default, and in `values-es/` in Spanish; `StringsTest` fails when the two differ in names or format arguments. The app follows the system language, and Ajustes → Sistema → Idioma overrides it through `Language.kt` (AppCompat per-app locales; the core restarts with the new `LANG`). The Spanish texts reuse the core's Spanish wording:
   - states: "Sonando", "En pausa", "Detenido", "Cargando...";
   - places: "Cola", "Ecualizador", "Letras", "Ajustes", "Estadísticas", "Descargas", "Sonando ahora", "Temporizador";
   - features: "Más como esto", "Me interesa", "No me interesa", "Normalización de volumen", "Radio automática".

   Provider labels (shelf titles, sort labels, genre groups) and notices (`snapshot.notice`) arrive already localized from the core.
5. **Performance and battery.**
   - The core keeps running with drawing off.
   - The native UI works only while it is visible:
     - visualizer frame polling only while a visualizer is on screen and music plays;
     - the position ticker only while in the foreground and playing;
     - nothing polls in the background.
   - Lists are lazy and keyed. JSON is parsed off the main thread into immutable models. Images are decoded at target size and cached.
6. **Smallest change.**
   - No navigation library, image library, DI, ViewModel or serialization library.
   - The Go additions are small handlers over existing functions.
   - The UI hides any element whose operation is missing from `capabilities`, so the two agents can land in any order. There is one temporary fallback, noted in §10.

---

## 2. Navigation map

```
MainActivity (Compose, singleTask, launcher; VIEW cliamp-x:// and SEND text intents as today)
├─ Splash                      until Remote.connected (and, first run, "Preparando yt-dlp…")
├─ Tab Inicio      stack: Home → Album | Artist | Playlist | Genre | ParaTi(albums|artists) | Remixes | Settings → Themes | Visualizers | Eq
├─ Tab Buscar      stack: Search → Album | Artist | Playlist | Genre | Remixes
├─ Tab Biblioteca  stack: Library → Playlist | Album | Artist | Stats | Ratings
│    (any stack may push Album, Artist, Playlist, Genre or Remixes from an action sheet)
├─ Status line     one caption row above the mini-player, only while a notice exists
├─ Mini-player     above the tab bar whenever snapshot.track != null
├─ Player sheet    full-height ModalBottomSheet; pages: Player ⇄ Queue ⇄ Eq
├─ Bottom sheets   item actions, add to playlist, name field, confirm, sleep, speed,
│                  normalization, crossfade
└─ TerminalActivity "Modo terminal" (TUI)
LoginActivity (WebView sign-in, unchanged)
```

### 2.1 Tabs

**Inicio**
- Top bar: logo and `⚙` (opens Ajustes).
- The banner slot (§2.10).
- Carousels, each one shown only when it has items:
  - Escuchado recientemente;
  - one carousel per home shelf;
  - Para ti: álbumes `[Ver todo]`;
  - Para ti: artistas `[Ver todo]`;
  - Novedades de tus artistas;
  - Tus géneros, as chips.

**Buscar**
- The field.
- With an empty query:
  - Recientes: the last 10 queries, as chips;
  - the genre grids: Géneros, Estados de ánimo, Más géneros and Música local, grouped by `group`.
- With a query: the results grouped by section in arrival order (artists, albums, playlists, songs, local).

**Biblioteca**
- Title `▸─ Biblioteca ──` and `[+]` (new playlist).
- Pinned rows: ♥ Canciones que te gustan, Historial, Música local, Tus valoraciones, Estadísticas.
- Chips `[Listas] [Álbumes] [Artistas] [Descargas]`.

**Reselecting** the active tab pops its stack to the root, or scrolls to the top when it is already at the root.

### 2.2 Mini-player (64 dp)

- **Layout:**
  - art 48 dp;
  - title (marquee while playing) over a second line;
  - a `♥` button and a play/pause button;
  - a 2 dp accent progress line along the bottom edge.
- **Second line:**
  - normally the artist;
  - `◌ Cargando...` while buffering;
  - `◌ Preparando yt-dlp…` while a play request waits for the first yt-dlp update;
  - `ERR: <short reason>` in red when the playing track has an error (§2.10).
- **Gestures:**
  - tap, or a drag up of more than 48 dp (or a fling up), opens the player sheet;
  - a horizontal drag past 30 % of the width, or a fling faster than 1000 dp/s, skips: left → `next`, right → `prev`. The bar follows the finger (`translationX`) and springs back.

### 2.3 Full player (sheet, page Player)

- **Top row:** `⌄` (close), `── Sonando ahora ──`, `⋮`.
- **Mode tags,** caption in accent, each shown only when it applies: `RADIO · FUNDIDO · EQ AUTO · APAGADO 23:40`.
- **Art area** (a square, sized in §3.9). It shows one of three views:
  - **Portada**: the cover. When the track has none, a `░` field with a note vector.
  - **Visualizador**: a large visualizer.
  - **Letras**: synced lyrics keep the active line centered in accent bold, the others dim. Plain lyrics just scroll.
- **Art area gestures:**
  - tap toggles Portada ⇄ Visualizador;
  - a horizontal swipe skips (Portada and Visualizador only), with the mini-player rule.
- **Visualizer strip,** 72 dp under the art. It shows only in the Portada view, when the mode is not None and the art is at least 240 dp. A tap cycles through every mode of `vis list` in the core's order (None left out) via `vis {name}`.
- **Title block:**
  - the title (display style, accent bold, marquee) with `♥` at the right;
  - `Artista · Álbum` below. Tapping the artist opens the Artist page when `provider_meta["ytmusic.artist"]` exists; tapping the album opens the Album page when `provider_meta["ytmusic.album"]` exists.
- **Seek bar** with a 48 dp touch height. Under it:
  - left: `01:23 / 03:45`;
  - right, the status: `▶ Sonando`, `‖ En pausa`, `◌ Cargando...`, `■ Detenido`, or `LIVE` for a stream.
- **Transport:** shuffle, prev, the play/pause key pill (72 dp, accent fill), next, and repeat (a "1" badge for repeat one). Every target is at least 56 dp.
- **Chips:** `[Letras] [En cola: n] [EQ] [⏲]`. With a sleep timer set, the last chip reads `[⏲ 23:40]`, or `[⏲ fin]` for end of song.
- **⋮ menu:** the song actions (§2.6), then Velocidad…, Temporizador…, Modo terminal.
- **Closing:** swipe down (only on this page), `⌄`, or system back.
- **Error card:** replaces the art view (§2.10).

### 2.4 Detail pages

**Album.** Opened with an ID that is `MPREb_…`, `al:MPREb_…` (a release row; opening it marks it seen) or `local:al:…`.
- Header:
  - cover 240 dp;
  - the title;
  - the artist (tap opens the Artist page);
  - the preview lines joined by ` · `.
- Buttons:
  - `▶ Reproducir` (key pill);
  - `[⤨]` (shuffle on, then play from a random row);
  - `[♥]` save, only for YouTube albums, with a known state, while signed in;
  - `[↓]` download;
  - `⋮`.
- Rows: number, title, duration, `⋮`.

**Artist.** Opened with a `UC…` or `local:ar:…` ID.
- Header: picture, name, preview lines (listeners, subscribers, top songs).
- Buttons:
  - `[ Seguir ]` / `[ Siguiendo ]`, only with a known state, while signed in;
  - `[ ◉ Radio ]`.
- `── Populares ──`: 5 rows and `[Ver todas]`, which opens a Playlist page for `ar:UC…`.
- Carousels:
  - `── Álbumes ──`;
  - `── Singles y EP ──` (rows with `single`);
  - `── En tu música local ──` (`local:` IDs).

**Playlist.** Any playlist ID: `pl:…`, `fav`, `hist`, `local:all`, `ar:UC…`, `shelf:N`, or a local list name.
- Header: art, name, then `n canciones · h min`.
- Buttons: `▶ Reproducir`, `[⤨]`, `[↓]` (YouTube only).
- Paged rows.
- A local list adds swipe-to-remove on its rows, and `⋮` with Renombrar… and Eliminar.

**Genre.** Opened with a genre ID: category params, `search:<Name>` or `local:genre:<Name>`.
- The name.
- Sort chips from `sorts`, for example `[Mezcla del género] [Canciones populares] …`.
- `▶ Reproducir`, `[★ Fijar en Inicio]` (not for `local:` genres), and the rows.

**Para ti.** An endless grid of albums or artists: `LazyVerticalGrid(GridCells.Adaptive(160.dp))`.

**Remixes y versiones.** The result songs grouped by `provider_meta.section`.

### 2.5 Ajustes (pushed on the Inicio stack)

**Apariencia**
- Tema: the picker. A tap applies the theme at once, and the whole UI, the picker included, recolors. That is the live preview.
- Visualizador: the picker, with a live 120 dp preview while music plays. It lists every mode the core reports in `vis list`, None last, each with its Spanish label (a Lua mode shows its name only).

**Reproducción**
- Ecualizador: opens the Eq screen.
- Velocidad and Temporizador: open sheets.
- Normalización de volumen `[Por canción]`: a sheet with Apagada, Por canción, Por álbum.
- Fundido entre canciones `[Apagado]`: a sheet with 0 to 12 s.
- Toggles: EQ automático por género, Radio automática, Mono. Every value is Spanish: `[Encendido]` or `[Apagado]`.

**Cuenta**
- `YouTube Music · Sesión iniciada` or `No has iniciado sesión`.
- `[ Iniciar sesión ]` (LoginActivity), `[ Importar cookies.txt ]` (SAF), `[ Cerrar sesión ]`.

**Música local**
- The folder path, read-only.
- `[ Permitir música local ]` when the permission is missing.

**Sistema**
- Batería `[ Quitar optimización ]`.
- Actualizaciones `[Estable] [Nightly]`, then `Versión x · [ Buscar actualizaciones ]`.
- `yt-dlp <tag | incluida 2025.11.12> · [ Actualizar yt-dlp ]`.
- Modo terminal ›.
- `[ Salir ]`, in the error color.

### 2.6 Action sheet (long-press or ⋮), per item type

- **Header:** art 48 dp, title, subtitle.
- **Rows:** 56 dp, with a glyph column.
- **Visibility:** an action shows only when its operation is in `capabilities`, its IDs exist, and, for account actions, `signedIn == true`.

| Item | Actions, in order |
|---|---|
| Song (any list, shelf or search row) | Reproducir a continuación · Añadir a una lista… · Me gusta / Quitar de Me gusta (from `bookmark`) · Iniciar radio · Remixes y versiones · Ir al álbum · Ir al artista · Descargar · Me interesa · No me interesa |
| Song in the queue | the Song actions, plus Quitar de la cola |
| Song in a local list | the Song actions, plus Quitar de esta lista |
| Album | Reproducir · Aleatorio · Reproducir a continuación · Guardar en la biblioteca / Quitar de la biblioteca (state from `provider.preview`; hidden until known) · Ir al artista · Iniciar radio · Añadir a una lista… · Descargar · Me interesa · No me interesa |
| Artist | Seguir / Dejar de seguir (state from `provider.preview`) · Iniciar radio · Me interesa · No me interesa |
| Playlist (YouTube) | Reproducir · Aleatorio · Reproducir a continuación · Iniciar radio · Descargar |
| Playlist (local) | Reproducir · Aleatorio · Reproducir a continuación · Renombrar… · Eliminar (confirm sheet) |
| Genre or mood | Reproducir mezcla · Fijar en Inicio / Quitar de Inicio · Iniciar radio (params IDs only, not `search:` or `local:`) · Me interesa · No me interesa |

**"Añadir a una lista…" sheet:**
- `+ Nueva lista…` opens the name sheet with a target, `[YouTube Music] [Local]`.
- `── YouTube Music ──` lists the rows with `writable`.
- `── Local ──` lists the local lists, without the virtual `Favorites` and `Recently Played`.

**"No me interesa"** removes the row from the list on screen at once, and clears the client cache. When the song plays now, the core skips it.

### 2.7 Queue (player-sheet page)

- **Header:**
  - `←`;
  - `▸─ Cola ──`;
  - `⋮` with Guardar como lista… and Vaciar cola;
  - chips `[Aleatorio] [Repetir: Todo]`.
- **Sections:**
  - `── Sonando ──`: one row;
  - `── A continuación (n) ── [Vaciar]`: from `playnext.list`;
  - `── Siguen (n) ──`: the play order after the current song (shuffle aware).
- **Gestures:**
  - tap a row → `queue.play`;
  - swipe end→start removes the row;
  - drag the `≡` handle to reorder. Under shuffle the handle hides in "Siguen", because the core refuses moves there.

### 2.8 Eq (player-sheet page, also pushed from Ajustes)

- Ten vertical block columns, one per band. Dragging a column moves it in 1 dB steps between −12 and +12.
- The frequency row (dim) and the gain row (numeric).
- The `Graves  Medios  Agudos` row, dim.
- `[EQ automático: Off]`.
- `── Preajustes ──`: a wrap of preset chips, with `●` on the active one. "Custom" shows as "Personalizado".

### 2.9 Back behavior

| State | System back |
|---|---|
| A bottom sheet is open | closes it |
| Player sheet on the Queue or Eq page | returns to the Player page |
| Player sheet open | closes the sheet |
| Tab stack has more than one screen | pops one screen; the scroll position of the screen below is kept |
| Buscar or Biblioteca at its root | switches to Inicio |
| Inicio at its root | default handling: the task goes to the background and music keeps playing |
| TerminalActivity | as today; at `[Cola]`/`[Queue]` it calls `finish()`, which returns to the native UI |

Add `android:enableOnBackInvokedCallback="true"` to `<application>`.

### 2.10 States: loading, empty, error, offline, signed out, notices

- **Loading:** braille spinner (accent) plus `Cargando...` (dim), centered. In a carousel, a 136 dp tile holds the spinner.
- **Empty:** a two-space-indented dim line, for example `  No hay canciones.`, plus an action when one exists.
- **Error:** `ERR: <mensaje>` in red, then `[ Reintentar ]`. The message is the job's `error.detail`.
- **Offline:** an error detail that looks like a network failure (§5.7) shows `ERR: Sin conexión a internet.` and `[ Reintentar ]`. Música local and the downloads keep working.
- **Signed out** (`signed_in == false`):
  - Inicio shows the public home and a dismissible card: "Inicia sesión en YouTube Music para ver tus listas, tus me gusta y tus recomendaciones", with `[ Iniciar sesión ]` and `[ Importar cookies.txt ]`;
  - Biblioteca shows the same card in place of the YouTube rows, and keeps the local rows;
  - the account actions (♥ sync, follow, save, add to a YouTube playlist) hide.
- **Core not connected:**
  - first screen: a full-screen splash with `C L I A M P - X` and `⠹ Iniciando cliamp-x…`;
  - the very first launch adds the line `Preparando Python, yt-dlp y ffmpeg (solo la primera vez)…`;
  - later disconnects show the banner `⠹ Reconectando con cliamp-x…`.
- **Play error card.** It shows in the art area, and as the mini-player's second line, only while `snapshot.error != ""` **and** `snapshot.error_path == snapshot.track.path`.

  | `error_kind` | Text | Action |
  |---|---|---|
  | `ytdlp` | "yt-dlp está desactualizado." | `[ Actualizar yt-dlp ]`: forced update, then one retry (§7.8) |
  | `session` | "Tu sesión de YouTube Music caducó." | `[ Iniciar sesión ]` |
  | `network` | "Sin conexión con YouTube." | `[ Reintentar ]` |
  | `unavailable` | "Esta canción no está disponible." | `[ Siguiente ]` |
  | `""` | `snapshot.error` | `[ Reintentar ]` |

  Reintentar = `queue.play {index: snapshot.index}`. Every card also has `[ Siguiente ]`.
- **Status line.** `snapshot.notice` shows as one caption row right above the mini-player:
  - activity: dim;
  - success: green bold;
  - warning: `WARN: ` in yellow bold;
  - error: `ERR: ` in red.

  It hides when the notice leaves the snapshot, and it hides warning and error notices while the play error card shows (the card already says it). App-side errors (`Remote.errors`) use the same row for 4 s.
- **Banner slot (Inicio).** At most one of each, in this order:
  1. reconnecting;
  2. yt-dlp: `⠹ Actualizando yt-dlp…`, then `✓ yt-dlp <tag> listo` (4 s), or `ERR: <msg>` with `[ Reintentar ]`;
  3. app update: `cliamp-x <v> disponible` with `[ Instalar ]` and `[ Ahora no ]`;
  4. sign-in card;
  5. battery card, only after the first song played and while battery optimization is on.

---

## 3. Screens (portrait 1080×2340 @ 420 dpi = 411×891 dp)

**How to read the mockups:**
- One character is about 9 dp.
- The layout must also work from 360 dp width and at the maximum font and display size.
- Glyphs drawn as **vectors** (JetBrains Mono lacks them): ⚙ ⌕ ▤ ⌂ ⤨ ⟳ ⏲ ★ ♫ ⟲ and the transport icons.
- Every other glyph is JBM text.

### 3.1 Splash
```
┌──────────────────────────────────────────────┐
│                                              │
│               C L I A M P - X                │ logo 15sp accent bold
│          ⠹ Iniciando cliamp-x…               │ caption dim, spinner accent
│   Preparando Python, yt-dlp y ffmpeg         │ first launch only
│   (solo la primera vez)…                     │
└──────────────────────────────────────────────┘
```

### 3.2 Inicio
```
┌──────────────────────────────────────────────┐
│ C L I A M P - X                          [⚙] │ top bar 56dp under the status-bar inset
│ ┌──────────────────────────────────────────┐ │
│ │ ⠹ Actualizando yt-dlp…                   │ │ banner slot, surface fill, 0..n banners
│ └──────────────────────────────────────────┘ │
│ ── Escuchado recientemente ───────────────── │ rule 40dp
│ ┌──────┐ ┌──────┐ ┌──────┐ ┌───             │ carousel: tile art 136dp
│ │ art  │ │ art  │ │ art  │ │                │
│ └──────┘ └──────┘ └──────┘ └───             │
│ Título   Título   Título   Tí               │ tile 13sp textPrimary, 1 line
│ Artista  Artista  Artista  Ar               │ label 12sp dim, 1 line
│ ── Selección rápida ──────────────────────── │ one rule + carousel per home shelf
│ ── Para ti: álbumes ─────────── [Ver todo]   │
│ ── Para ti: artistas ────────── [Ver todo]   │ artist tiles are square too
│ ── Novedades de tus artistas ─────────────── │ ● accent in the tile's top-left = unseen
│ ── Tus géneros ───────────────────────────── │
│ [★ Rock] [★ Indie] [★ Lo-fi]                 │ chips
├──────────────────────────────────────────────┤
│ WARN: YouTube Music no guardó el favorito    │ status line 24dp (only with a notice)
├──────────────────────────────────────────────┤
│ [art] Título que se desplaza…     ♥   ‖      │ mini-player 64dp
│       Artista                                │
│━━━━━━━━━━━━━━━━●─────────────────────────────│ 2dp progress
├──────────────────────────────────────────────┤
│     ⌂              ⌕              ▤          │ tab bar 56dp + nav inset
│  [Inicio]        Buscar        Biblioteca    │ active: accent bold in brackets
└──────────────────────────────────────────────┘
```

### 3.3 Buscar, empty query
```
│ ▸─ Buscar ──────────────────────────────────── │
│ ┌────────────────────────────────────────────┐ │
│ │ > ¿Qué quieres escuchar?_               ✕  │ │ field 48dp, surfaceHigh; `>` and `_` accent
│ └────────────────────────────────────────────┘ │
│ ── Recientes ─────────────────────── [Borrar]  │ last 10 queries (prefs), only when non-empty
│ [daft punk] [soda stereo] [lofi]               │
│ ── Géneros ────────────────────────────────── │ group labels come from the core
│ ┌────────────────────┐ ┌────────────────────┐ │ 2-col grid, 64dp tiles, surface fill;
│ │▌Rock              ★│ │▌Pop                │ │ 4dp left bar cycling green/yellow/red/accent;
│ └────────────────────┘ └────────────────────┘ │ ★ (vector) = pinned
│ ── Estados de ánimo ───────────────────────── │ browse:moods
│ ── Más géneros ────────────────────────────── │ search: IDs
│ ── Música local ───────────────────────────── │ local:genre: IDs
```

### 3.4 Buscar, results
```
│ │ > daft punk_                            ✕  │ │
│ ── Artistas ───────────────────────────────── │ carousel of square tiles
│ ── Álbumes ────────────────────────────────── │ carousel
│ ── Listas ─────────────────────────────────── │ carousel
│ ── Canciones ──────────────────────────────── │
│ [art] One More Time                       ⋮    │ track row 64dp
│       Daft Punk · Discovery          5:20      │
│ [art] ▶Around the World                   ⋮    │ playing row: ▶ and title green bold
│       Daft Punk · Homework           7:09      │
│ ── Música local ───────────────────────────── │
```
Section titles are the core's `provider_meta.section` values, in arrival order.

### 3.5 Biblioteca
```
│ ▸─ Biblioteca ─────────────────────────── [+]  │
│ ♥ Canciones que te gustan       312 canciones  │ pinned rows 56dp (♥ text red; others vectors)
│ ⟲ Historial                                    │
│ ♫ Música local                1.204 canciones  │
│ ✓ Tus valoraciones                             │
│ ▤ Estadísticas                                 │
│ [Listas]  Álbumes   Artistas   Descargas       │ chips: active = accent bold in brackets
│ ── YouTube Music ──────────────────────────── │ chip Listas
│ [art] Para correr                     45       │
│ ── Local ──────────────────────────────────── │
│ [art] Mi lista                        12       │
```
- **Álbumes chip:** sort chips from `sorts` without `foryou`, then a 2-column grid with `♥` on saved albums.
- **Artistas chip:** rows with square art, `♥` = followed.
- **Descargas chip:** §3.13.

### 3.6 Album
```
│ ←                                          ⋮  │
│              ┌────────────────┐              │
│              │  cover 240dp   │              │
│              └────────────────┘              │
│ Discovery                                     │ display 22sp accent bold
│ Daft Punk                                     │ body bold textPrimary (tap → artist)
│ Álbum · 2001 · 14 canciones • 61 minutos      │ label dim: preview lines joined by ·
│ ┏━━━━━━━━━━━━━━┓  [⤨]  [♥]  [↓]               │ key pill + bracket buttons 48dp
│ ┃ ▶ Reproducir ┃                              │
│ ┗━━━━━━━━━━━━━━┛                              │
│ ── Canciones ──────────────────────────────── │
│  1  One More Time                  5:20  ⋮    │ row 56dp, number column 3 cells
│  2 ▶Aerodynamic                    3:27  ⋮    │
```

### 3.7 Artist
```
│ ←                                          ⋮  │
│              ┌────────────────┐              │ picture 240dp, square
│              └────────────────┘              │
│ Daft Punk                                     │ display
│ 14,6 M usuarios mensuales · 3,03 M suscript…  │ label dim (a leading "♫ " is dropped)
│ [ Siguiendo ]  [ ◉ Radio ]                    │
│ ── Populares ─────────────────────────────── │
│  1 [art] Get Lucky                     ⋮      │ 5 rows
│ [Ver todas]                                   │
│ ── Álbumes ────────────────────────────────── │ carousel
│ ── Singles y EP ───────────────────────────── │ carousel (single == true)
│ ── En tu música local ─────────────────────── │ carousel (local: IDs), only if present
```

### 3.8 Playlist, Genre, Para ti, Remixes
```
Playlist                          Genre                              Para ti
│ ←                         ⋮ │   │ ←                          ⋮ │   │ ← ▸─ Para ti: álbumes ── │
│ [art 160] Para correr       │   │ Rock                         │   │ ┌──────┐ ┌──────┐        │
│ 45 canciones · 3 h          │   │ [Mezcla del género] Canc…    │   │ │ art  │ │ art  │        │
│ [▶ Reproducir] [⤨] [↓]      │   │ [▶ Reproducir] [★ Fijar]     │   │ └──────┘ └──────┘        │
│ ── Canciones ────────────── │   │ ── Canciones ─────────────── │   │ Título   Título          │
│ [art] Título            ⋮   │   │ [art] Título            ⋮    │   │ Artista  Artista         │
│       Artista · Álbum 3:45  │   │       Artista       3:45     │   │  … endless, ⠹ tile at end│
Remixes: same as Playlist without the header buttons; rules = provider_meta.section groups.
```

### 3.9 Full player, portrait
```
┌──────────────────────────────────────────────┐
│ ⌄            ── Sonando ahora ──          ⋮  │ 48dp
│ RADIO · FUNDIDO · EQ AUTO · APAGADO 23:40    │ caption accent (hidden when empty)
│  ┌────────────────────────────────────────┐  │
│  │                                        │  │ art: square, side = min(width − 48dp,
│  │   Portada / Visualizador / Letras      │  │   height left after the fixed rows)
│  │   ← swipe → skips · tap toggles vis    │  │
│  │                                        │  │
│  └────────────────────────────────────────┘  │
│  ▁▃▅▇█▇▅▃▂▁▃▅▇█▇▅▃▂▁▃▅▇█▇▅▃                  │ visualizer strip 72dp (tap: next mode)
│  Título de la canción que se despla…     ♥   │ display 22sp accent bold marquee; ♥ red/dim
│  Daft Punk · Discovery                        │ body dim; artist and album tappable
│  ━━━━━━━━━━━━━━━━━●────────────────────────── │ seek, 48dp touch area
│  01:23 / 03:45                    ▶ Sonando   │ numeric textPrimary; status green bold
│     ⤨      ◀◀     ┏━━━━━━┓     ▶▶      ⟳      │ 56dp targets; key pill 72dp
│                   ┃  ‖   ┃                    │
│                   ┗━━━━━━┛                    │
│  [Letras] [En cola: 3]  [EQ]    [⏲ 23:40]    │ bracket chips 48dp
└──────────────────────────────────────────────┘
```
**Sizing:**
- Fixed rows ≈ 48 + 16 + 72 + 56 + 48 + 20 + 88 + 48 + 24 padding = 420 dp.
- Art = `min(maxWidth − 48.dp, maxHeight − 420.dp)`, computed in `BoxWithConstraints` inside `WindowInsets.safeDrawing`.
- When the art would be under 240 dp, the strip hides and its 72 dp go to the art.
- At 411×891 dp with gesture navigation, the art is 343 dp.
- On a taller screen the art stops at the width: the height left splits evenly above the art and below the chips, never between the strip and the title.

Error card (replaces the art view):
```
│  ┌────────────────────────────────────────┐  │
│  │ ERR: No se pudo reproducir.            │  │ red
│  │ yt-dlp está desactualizado.            │  │ textPrimary
│  │ ┏━━━━━━━━━━━━━━━━━━━┓                  │  │
│  │ ┃ Actualizar yt-dlp ┃   [ Siguiente ]  │  │
│  │ ┗━━━━━━━━━━━━━━━━━━━┛                  │  │
│  └────────────────────────────────────────┘  │
```

### 3.10 Full player, landscape
This is the only screen that changes layout in landscape. The other screens reflow (grids use `Adaptive`).
```
┌────────────────────────────────────────────────────────────────────────────┐
│ ⌄  ── Sonando ahora ──               RADIO · FUNDIDO                    ⋮ │
│ ┌──────────────────────┐  Título que se desplaza                        ♥  │
│ │                      │  Daft Punk · Discovery                             │
│ │ Portada /            │  ▁▃▅▇█▇▅▃▂▁▃▅▇█▇▅▃▂▁▃▅▇█▇ (56dp)                   │
│ │ Visualizador /       │  ━━━━━━━━━━━━━━━●─────────────────────────────────  │
│ │ Letras               │  01:23 / 03:45                          ▶ Sonando  │
│ │ (square, height-fit) │      ⤨     ◀◀     [ ‖ ]     ▶▶     ⟳               │
│ └──────────────────────┘  [Letras]  [En cola: 3]  [EQ]  [⏲]                 │
└────────────────────────────────────────────────────────────────────────────┘
```

### 3.11 Queue
```
│ ←           ▸─ Cola ─────────────────────   ⋮ │ ⋮: Guardar como lista… · Vaciar cola
│ [Aleatorio]  [Repetir: Todo]                  │
│ ── Sonando ────────────────────────────────── │
│ [art] ▶One More Time                  5:20    │
│ ── A continuación (2) ──────────── [Vaciar]   │
│ [art] Q1 Get Lucky                      ≡     │ swipe ← removes; drag ≡ reorders
│ [art] Q2 Instant Crush                  ≡     │
│ ── Siguen (118) ───────────────────────────── │
│ [art]    Aerodynamic                    ≡     │ ≡ hidden while shuffle is on
```

### 3.12 Action sheet, add to playlist, name field, confirm
```
┌──────────────────────────────────────────┐ ┌──────────────────────────────────────────┐
│ [art] One More Time                      │ │ ── Añadir a una lista ────────────────── │
│       Daft Punk · Discovery              │ │ +  Nueva lista…                          │
│ ──────────────────────────────────────── │ │ ── YouTube Music ─────────────────────── │
│ ▶  Reproducir a continuación             │ │ [art] Para correr                   45   │
│ +  Añadir a una lista…                   │ │ ── Local ─────────────────────────────── │
│ ♥  Me gusta                              │ │       Mi lista                      12   │
│ ◉  Iniciar radio                         │ └──────────────────────────────────────────┘
│ ≈  Remixes y versiones                   │ ┌──────────────────────────────────────────┐
│ ◎  Ir al álbum                           │ │ ── Nueva lista ───────────────────────── │
│ →  Ir al artista                         │ │ > Nombre_                                │ the only other text field
│ ↓  Descargar                             │ │ [YouTube Music]  Local                   │
│ ✓  Me interesa                           │ │ ┏━━━━━━━━┓  [ Cancelar ]                  │
│ ✗  No me interesa                        │ │ ┃ Crear  ┃                              │
└──────────────────────────────────────────┘ │ ┗━━━━━━━━┛                              │
                                              └──────────────────────────────────────────┘
Confirm (Eliminar lista, Vaciar cola): "¿Eliminar «Mi lista»?"  [ Eliminar ] in red · [ Cancelar ]
```

### 3.13 Descargas (Biblioteca chip)
```
│ ── Descargando ────────────────────────────── │
│ ↓ One More Time                        42 %   │
│ ━━━━━━━━━━━●───────────────────────────────── │
│ ── En espera (3) ──────────────────────────── │
│   Aerodynamic                                 │
│ ── Fallidas (1) ─────────────── [Reintentar]  │
│ ✗ Digital Love — HTTP Error 403               │ ✗ red, reason dim
│ [ ▶ Reproducir descargas ]   [ Cancelar ]     │
```

### 3.14 Ajustes, Tema, Visualizador
```
│ ←        ▸─ Ajustes ────────────────────── │ │ ←     ▸─ Tema ────────────────────── │ │ ←  ▸─ Visualizador ───────────────── │
│ ── Apariencia ─────────────────────────── │ │ ● Predeterminado      ■■■■■■■        │ │ ┌──────────────────────────────────┐ │
│ Tema                  [tokyo-night]     › │ │   alucard             ■■■■■■■        │ │ │ live preview 120dp (when playing)│ │
│ Visualizador          [Bars]            › │ │   amber               ■■■■■■■        │ │ └──────────────────────────────────┘ │
│ ── Reproducción ───────────────────────── │ │   …                                  │ │ ● Bars         Barras                │
│ Ecualizador           [Rock]            › │ │   cover  (colores de la portada)     │ │   ClassicPeak  Barras con picos      │
│ Velocidad             [1x]              › │ │ swatches: bg accent bright fg g y r  │ │   Mirror       Barras en espejo      │
│ Temporizador          [Apagado]         › │ │ tap = apply now, UI recolors live    │ │   None         Sin visualizador      │
│ Normalización de volumen [Por canción]  › │                                          │   …                                  │
│ Fundido entre canciones  [Apagado]      › │
│ EQ automático por género [Apagado]        │  tap toggles
│ Radio automática      [Apagado]           │
│ Mono                  [Apagado]           │
│ ── Cuenta ─────────────────────────────── │
│ YouTube Music · Sesión iniciada           │
│ [ Iniciar sesión ] [ Importar cookies.txt ] [ Cerrar sesión ]   (wraps)
│ ── Música local ───────────────────────── │
│ Carpeta  /storage/emulated/0/Music        │
│ ── Sistema ────────────────────────────── │
│ Batería: optimizada        [ Quitar ]     │
│ Actualizaciones   [Estable]  Nightly      │
│ Versión 1.4.0 · [ Buscar actualizaciones ]│
│ yt-dlp 2026.08.19 · [ Actualizar yt-dlp ] │
│ Modo terminal                           › │
│ [ Salir ]                                 │ error color
```

### 3.15 Ecualizador and the small sheets
```
│ ←   ▸─ Ecualizador · Rock ───────────────── │
│ +12 ┤  █  █                       █  █  █  │ 10 columns ≈ 33dp, drag vertically, 1 dB steps
│     │  █  █  █         █   █  █   █  █  █  │ filled cells textPrimary, selected band accent
│   0 ┼──█──█──█──━──━───█───█──█───█──█──█─ │ 0 dB row: ─, or ━ where the band sits at 0
│ -12 ┤                                      │
│     70 180 320 600 1k 3k 6k 12k 14k 16k    │ label dim
│     +5  +4  +2  -1 -2 +2 +4  +5  +5  +5    │ numeric
│     Graves         Medios          Agudos  │ dim
│ [EQ automático: Off]                       │
│ ── Preajustes ──────────────────────────── │
│ [Flat] [●Rock] [Pop] [Jazz] … [Personalizado]  (wrap)
```
```
── Temporizador ─────────────────────────   ── Velocidad ────────────────────   ── Normalización de volumen ──   ── Fundido entre canciones ──
[15 min] [30 min] [45 min] [60 min]         [0.5x] [0.75x] [1x] [1.25x]         ● Apagada                      [Apagado] [2 s] [4 s] [6 s]
[90 min] [Fin de la canción] [Apagar]       [1.5x] [2x]                           Por canción                    [8 s] [10 s] [12 s]
Activo: se pausa a las 23:40                                                      Por álbum
```

### 3.16 Estadísticas and Tus valoraciones
```
│ ←   ▸─ Estadísticas ─────────────────── │  │ ←   ▸─ Tus valoraciones ─────────────── │
│ [Semana]  Mes  Año  Todo                 │  │ ── No me interesa ───────────────────── │
│ 12 h 30 min · 340 reproducciones ·       │  │ Género   Reggaeton          [Deshacer]  │
│ 210 canciones · 5 días seguidos          │  │ ── Me interesa ──────────────────────── │
│ ── Últimos 30 días ───────────────────── │  │ Canción  One More Time      [Deshacer]  │
│ ▁▂▃▅▇█▃▁ ▂▄▆█▇▅▃▁▂▄▆█ (4 rows, accent)   │  │ Artista  Daft Punk          [Deshacer]  │
│ ── Artistas ──────────────────────────── │
│  1. Daft Punk — 42  ██████████████       │ bars accent, counts dim
│ ── Canciones ── … ── Géneros ── …        │
```

**Modo terminal** is the existing TUI screen (TerminalView plus the two extra-keys rows), unchanged.

---

## 4. Design tokens

### 4.1 Colors (from `snapshot.theme`, live)

The palette type is `data class CliampPalette(bg, accent, brightFg, fg, green, yellow, red)`. The derived roles are getters.

| Token | Source | Use |
|---|---|---|
| `bg` | `bg` | window, lists, full player, sheets |
| `textPrimary` | `bright_fg` | titles, numbers, EQ cells |
| `textSecondary` | `fg` | artist · album, durations, rules, brackets, hints, inactive icons |
| `accent` | `accent` | logo, player title, seek fill and head, active chips and tabs, `▸─` rules, spinner, chart bars |
| `onAccent` | computed | black when `accent.luminance() > 0.179`, else white |
| `playing` | `green` | `▶` and the playing row's title, `▶ Sonando`, success notices |
| `favorite`, `error` | `red` | ♥, `ERR:`, `✗`, destructive buttons |
| `warning` | `yellow` | `WARN:` |
| `spectrum` | green / yellow / red | one vertical Brush over the panel height with hard stops, measured from the bottom: green below 30 %, yellow 30–60 %, red above 60 % |
| `surface` | `lerp(bg, brightFg, 0.05)`, at 60 % opacity while a background other than Sólido is on | mini-player, tab bar, banners, genre tiles, sheets |
| `surfaceHigh` | `lerp(bg, brightFg, 0.09)`, with the same opacity as `surface` | search field, pressed chips |
| `pressed` | `accent` at 12 % | press feedback: an instant fill, no ripple |
| `divider` | `fg` at 35 % | 1 dp line between groups, never between rows |
| `scrim` | `#000000` at 60 % | behind sheets |

**Default theme:**
- The core sends "Default - Terminal colors" with a name only. Android draws it with the `winamp` palette:

  | bg | accent | bright_fg | fg | green | yellow | red |
  |---|---|---|---|---|---|---|
  | #000000 | #00ff00 | #ffffff | #969696 | #29ce10 | #d6b521 | #ef3110 |

- The picker labels it "Predeterminado".
- Any theme with a missing hex value falls back to the same palette.

**M3 ColorScheme mapping,** so the Material components inherit the theme:

| M3 role | Token |
|---|---|
| primary | accent |
| onPrimary | onAccent |
| background, surface, surfaceContainer* | bg |
| onBackground, onSurface | brightFg |
| surfaceVariant | surfaceHigh |
| onSurfaceVariant | fg |
| outline | divider |
| error | red |
| secondary | green |
| tertiary | yellow |

**System bars:**
- Edge to edge.
- Dark bar icons when `bg.luminance() > 0.5`. That covers the light themes: alucard, catppuccin-latte, flexoki-light and rose-pine.
- Update the bars in a `SideEffect` on every palette change.

### 4.2 Typography

JetBrains Mono 2.304 (SIL OFL 1.1), Regular and Bold only. Sizes are in sp, so the system font scale applies.

| Role | Size/line | Weight | Color | Use |
|---|---|---|---|---|
| logo | 15/20 | Bold | accent | `C L I A M P - X` (literal spaces) |
| display | 22/28 | Bold | accent | player title, page titles |
| headline | 18/24 | Bold | accent | `▸─ Biblioteca ──` screen rules |
| body | 14/20 | Regular | textPrimary | row titles, settings labels |
| bodyStrong | 14/20 | Bold | playing or accent | playing row, selection, chips that are on |
| tile | 13/18 | Regular | textPrimary | carousel tile titles |
| label | 12/16 | Regular | textSecondary | second lines, durations, `[tags]`, section rules |
| labelCaps | 12/16 | Bold | textPrimary | mode tags |
| numeric | 13/18 | Regular | textPrimary | `01:23 / 03:45` |
| caption | 11/14 | Regular | textSecondary | status line, tab labels, banners |

- Truncate with `…`.
- The player and mini-player titles use `Modifier.basicMarquee(initialDelayMillis = 1600, velocity = 42.dp, spacing = MarqueeSpacing(7 characters))`, only while playing. There is no marquee when `Settings.Global.ANIMATOR_DURATION_SCALE == 0`.

### 4.3 Spacing, sizes, shape, elevation

- **Grid:** 4 dp. Side margins 16 dp. One "cell" is 8.4 dp (one character at 14 sp); the marker columns are one cell wide.
- **Heights:**

  | Element | Height |
  |---|---|
  | top bar | 56 |
  | section rule | 40 |
  | one-line row | 48 |
  | settings row | 56 |
  | track row (48 dp art; the title wraps to 2 lines, the queue's stays 1 line cut in the middle) | 64 min |
  | carousel tile art | 136 |
  | genre tile | 64 |
  | mini-player | 64 |
  | tab bar | 56 + inset |
  | status line | 24 |
  | key pill | 48, or 72 in the player |
  | chip | 48 touch, 32 visual |

- **Shapes:** `RectangleShape` everywhere. Override every size in `MaterialTheme.shapes`, and the sheet shape. The only round element is the seek head `●`.
- **Elevation:** 0. Set `tonalElevation = 0` and draw no shadows.
- **Section rule:**
  - `── Label ` followed by `─` repeated to the width, drawn as text in the label style;
  - the current screen's rule is `▸─ Label ──` in headline accent;
  - a trailing action such as `[Ver todo]` sits at the right end.

### 4.4 Glyphs and icons

**Text glyphs,** verified in JetBrains Mono:
```
> ▶ ◀ ■ ● ○ ◌ ◉ ◎ ◆ ━ ─ │ ┤ ┼ ┏ ┃ ┗ ▸ ✓ ✗ ✕ ↓ ↑ → ← › · … — ‖ ≡ ≈ ⋮ + ×
▁▂▃▄▅▆▇█ ░▒▓ ▀▄ ▕▐ ⌄
```
**Text glyphs from Android's fallback font:** `♥` and the braille block U+2800–28FF (the spinner). The static JetBrains Mono 2.304 TTF lacks them; only the Nerd Font build has them (checked in M1 with `java.awt.Font.canDisplay`).

**Not in JBM,** so drawn as vectors, never as text: `⤨ ⟳ ⟲ ↻ ★ ☆ ⏲ ⏸ ❚ ♫ ♪ ♡ ⚙ ⌕ ▤ ⇄ ▮ ◑`.
- The core sends `★ ` on starred genre rows and `♫ ` in artist preview lines. Strip both (§5.6).
- The paused status text uses `‖`.

**Vector drawables** (24 dp, flat, square joins):
- Existing: `ic_play`, `ic_pause`, `ic_prev`, `ic_next`, `ic_close`.
- Add: `ic_home`, `ic_search`, `ic_library`, `ic_queue`, `ic_more`, `ic_heart`, `ic_heart_outline`, `ic_shuffle`, `ic_repeat`, `ic_repeat_one`, `ic_download`, `ic_eq`, `ic_lyrics`, `ic_timer`, `ic_settings`, `ic_back`, `ic_expand_down`, `ic_drag`, `ic_radio`, `ic_add`, `ic_terminal`, `ic_star`, `ic_check`, `ic_note`, `ic_history`, `ic_stats`.
- Source: Material Symbols **Sharp**, weight 400 (Apache 2.0). If the XML cannot be fetched, draw square-path vectors by hand.
- Tint: textSecondary; accent when active; textPrimary for the transport icons.

**Toggles and buttons:**
- Bracket chip, off: `[` and `]` in textSecondary, the label in accent Regular.
- Bracket chip, on: everything in accent Bold.
- Primary action: a key pill, with accent fill, onAccent Bold text and square corners.
- Secondary: `[ Texto ]` in accent.
- Destructive: text in `error`.

### 4.5 Motion

| Element | Behavior |
|---|---|
| Press | instant `pressed` fill (custom `Indication`), no ripple |
| Screen push and pop | fade in rising 1/40 of the height over 300 ms, the old screen fades out in 200 ms, FastOutSlowIn (`Motion.screen`) |
| Player sheet | M3 drag and its own settle (Material 3 1.4 keeps the sheet motion internal); its pages change like screens in 250 ms |
| Skip swipe | the art follows the finger and eases back in 250 ms |
| Explicit theme change | instant |
| `cover` theme, each new cover palette | the 7 colors animate over 400 ms, FastOutSlowIn |
| Artwork of the player and mini-player | the next song's art cross-fades in over 400 ms; list rows swap at once |
| Ajustes → Animaciones | Lenta stretches every duration above 1.6 times, Sin animación snaps them, closes sheets without the slide and holds the background still |
| Background (Ajustes → Fondo) | an AGSL shader at half resolution behind the whole app, stepped at 20 fps (each step redraws the whole window) while the activity is started, at a quarter speed under reduced motion; paused under the player sheet, which draws its own. A scrim of bg covers it (`scrimOf`): level 100 shows the most that keeps `fg` and `brightFg` at min(4.5:1, their contrast on bg) even over a white (light theme: black) pixel, with the scrim leaned to black (white); lower levels show less and lean only as far as needed |
| Spinner | a braille frame every 100 ms |
| Visualizer | the core's frames arrive at up to 30 fps, one request at a time; the core animates them. On pause or stop the last frame fades out in 300 ms, then drawing stops |
| Reduced motion | `ANIMATOR_DURATION_SCALE == 0` turns off the marquee and the crossfades. The visualizer then polls 10 frames a second |

### 4.6 Live theme flow

- The core is the single source of the theme:
  1. Ajustes → Tema sends `theme {name}`.
  2. The core applies the theme, saves it in `config.toml` and publishes `runtime.state` with the palette (G17).
  3. `Remote.state` updates, `CliampTheme(palette)` recomposes, and every token recolors.
- A theme changed in Modo terminal or by another IPC client recolors the native UI the same way.
- With the `cover` theme, G17 sends the palette the core derives from each new cover. The app animates to it and does not compute a palette of its own.

---

## 5. Data and IPC contract

### 5.1 Transport (protocol V2, `docs/remote-control.md`)

- **Socket:** `Core.socket(ctx)`. One JSON object per line, with `"version":2`.
- **Requests:** one connection per call (`Ipc.call`, soTimeout 5 s).
- **Operation:** `{"version":2,"id":"<n>","method":"operation.submit","operation":op,"params":{…}}` → `job.id`.
  - Then poll `{"version":2,"id":"<n>","method":"job.get","job_id":id}` at 0, 30, 60, 120 and 250 ms, then every 250 ms, until the job is `succeeded`, `failed` or `canceled`, or the timeout:

    | Operation | Timeout |
    |---|---|
    | `provider.*` reads | 60 s |
    | `radio.start`, `discover.remixes` | 45 s |
    | everything else | 30 s |

  - On `succeeded`: return `job.result`, and apply `job.snapshot` (§5.4).
  - On `failed`: throw `IpcError(code, message, detail)`.
  - On caller cancellation or timeout: send `job.cancel {job_id}` in a `NonCancellable` block.
  - The app never waits on `runtime.job` events.
- **Subscription:** a dedicated socket with `{"version":2,"id":"events","method":"subscribe","topics":["runtime.state"]}` and soTimeout 0.
  - The read loop runs on IO and only parses and assigns, so it never blocks.
  - On EOF, an error or a `system.overflow` event: close the socket, back off from 200 ms to 2 s, and reconnect.
  - Cancelling the loop closes the socket.
- **Visualizer frames:** one persistent socket. It sends `{"version":2,"id":"f","method":"vis.frame","params":{"cols":C,"rows":R}}` at most every 33 ms and reads `result.lines` (rows with ANSI SGR colors) and `result.visualizer`.
- **omitempty rules:** a missing number is 0, a missing bool false, a missing string "".
  - `speed == 0` means 1.0.
  - `index` can be −1 for an empty queue.
- **Echoing tracks:** keep each TrackInfo's raw JSON string (`Track.raw`) and send it back unchanged in `track`/`tracks`. `provider_meta` carries the identity.

### 5.2 Connection lifecycle and feature detection

- `CliampService` owns the core. `Remote.start(ctx)` (idempotent) runs the subscription loop for the life of the service, and `Remote.stop()` ends it when the service finishes.
- On every successful (re)subscribe, in this order:
  1. Reset the revision floor (§5.4), so the next snapshot is accepted unconditionally.
  2. Call `state.get` right after the subscribe ack and apply it as `STATE_GET`.
  3. Call `capabilities` → `Remote.ops: Set<String>`.
  4. Call `provider.list` → `Remote.signedIn` (from `signed_in` of `ytmusic`).
  5. Clear the result cache.
  6. Resend the render mode (§7.6).
- UI elements whose operation is missing from `ops` are hidden.

### 5.3 Per-screen contract

`yt` stands for `"provider":"ytmusic"`. "all pages" means the paging loop of §5.5.

#### Global

| Reads | Actions | Cache / refresh |
|---|---|---|
| `runtime.state` events. `state.get` every 5 s while MainActivity is STARTED and `state == "playing"` | — | — |

#### Inicio

Reads:
- `history {limit:30}` → `history[].track`, deduplicated by `path`, at most 20.
- `provider.home {yt}` → `shelves` (G2).
- `provider.albums {yt, sort:"foryou", offset:0, limit:20}`.
- `provider.artists {yt, sort:"foryou", offset:0, limit:20}` (G4).
- `provider.playlists {yt}`, all pages, shared with Biblioteca. Keep the rows with IDs `al:` (Novedades) and `genre:` (Tus géneros).

Each carousel loads when it is composed (inside the LazyColumn).

Actions:

| Tap | Effect |
|---|---|
| Song tile in a shelf | `queue.replace {tracks: the shelf's song tracks, index}` |
| History tile | `queue.replace {tracks: the history tracks, index}` |
| Album / playlist / artist tile | its page |
| Novedades tile | Album page with the `al:` ID |
| Genre chip | Genre page with the ID after `genre:` |

Cache 10 min. Pull to refresh: `provider.refresh {yt}` (G10), then `Remote.invalidate()`, then refetch.

#### Buscar, empty query

- Reads: `provider.genres {yt}` (genres) and `provider.genres {yt, playlist:"browse:moods"}` (G3) → `genres[]` grouped by `group` in arrival order, plus `sorts`.
- Actions: a tile opens the Genre page; a long-press opens the genre sheet.
- Cache: the session.

#### Buscar, query

Reads:
- `provider.search {yt, query, limit:40}`.
- It runs 450 ms after the last change, once the query has at least 2 characters. The IME Search action sends at once, and a new query cancels the previous job.
- Group the rows by `provider_meta.section` in arrival order. A row's kind comes from `provider_meta.albumID`:

  | `albumID` | Kind |
  |---|---|
  | `ar:` prefix | artist |
  | `MPREb_` prefix | album |
  | `pl:` prefix | playlist |
  | missing | song |

Actions:
- A song → `queue.replace {tracks: the song rows of that section, index}`.
- A placeholder → its page with `albumID` (for an artist, strip `ar:`).

Cache: 2 min. The last 10 queries are kept in prefs `ui`.

#### Album

Reads:
- `provider.preview {yt, playlist: id without "al:"}` (G18) → `preview {image_url, lines, favorite?}`.
- Tracks:
  - `MPREb_…` and `local:al:…` → `provider.album_tracks {yt, album:id, limit:200}`;
  - `al:…` → `provider.tracks {yt, playlist:id, limit:200}`.

Actions:

| Action | Operation |
|---|---|
| Row i, or Reproducir (i = 0) | `provider.load_album {yt, album:id, index:i}` (G5); for `al:` IDs, `provider.load {yt, playlist:id, index:i}` |
| ⤨ | `shuffle {name:"on"}`, then load with a random index |
| ♥ | `provider.favorite {yt, playlist:"MPREb_…"}` → `favorite` (G20) |
| ↓ | `download {yt, playlist:id, name:title}` |

Cache 30 min. Pull refetches.

#### Artist

Reads:
- `provider.preview {yt, playlist:"UC…"}`.
- `provider.artist_albums {yt, artist:id}`. Row 0 (`ar:UC…`) is the top-songs entry; drop it from the carousels.
- `provider.album_tracks {yt, album:"ar:UC…", limit:200}` → the top songs.

Actions:

| Action | Operation |
|---|---|
| Seguir | `provider.favorite {yt, playlist:"UC…"}` → `favorite` |
| Radio | `radio.start {yt, playlist:"UC…", name}` (G12) |
| A song | `provider.load {yt, playlist:"ar:UC…", index}` |

Cache 30 min.

#### Playlist

Reads:
- `provider.preview` (only for `pl:` IDs).
- `provider.tracks {provider, playlist, offset, limit:200}`, paged.

Actions:

| Action | Operation |
|---|---|
| Row i, or Reproducir | `provider.load {provider, playlist, index:i}` |
| Local list: remove a row | `playlist.remove {provider:"local", playlist, index}` |
| Local list: rename | `playlist.rename {provider:"local", playlist, new_name}` |
| Local list: delete | `playlist.delete {provider:"local", playlist}` |

Cache 10 min; 1 min for `fav` and `hist`.

#### Genre

Reads: `provider.genre_tracks {yt, playlist:genreID, sort, limit:200}` (G3), with `sorts` from `provider.genres`.

Actions:
- Row i, or Reproducir → `queue.replace {tracks: the rows shown, index:i}`.
- ★ → `provider.genre_favorite {yt, playlist:genreID}` → `favorite`.
- Radio → `radio.start {yt, playlist:"genre:"+genreID, name}`, only for params IDs.

Cache 10 min.

#### Para ti

- Reads: `provider.albums {yt, sort:"foryou", offset, limit:40}` and `provider.artists {yt, sort:"foryou", offset, limit:40}`.
- A tile opens its page.
- The list is kept while the tab lives.

#### Biblioteca

Reads:
- `provider.playlists {yt}`, all pages. The library is the rows whose `section == rows[0].section` (`fav`, `hist`, `local:all`, `pl:…`).
- `provider.playlists {provider:"local"}`, without `Favorites` and `Recently Played`.
- `provider.albums {yt, sort:"top"|"alpha"|"artist"|"year", offset, limit:100}`.
- `provider.artists {yt}`, all pages.

Actions:
- Pinned rows open the Playlist page with `fav`, `hist` or `local:all`.
- `[+]` opens the name sheet, then `playlist.create {provider, playlist:name}` → `playlist` (the new ID, G20).

Cache 10 min. Pull: `provider.refresh`.

#### Descargas

- Reads: `download.status` (G15), polled every 1 s while the chip is visible and `running`.
- Actions:
  - `download.cancel`, `download.retry`;
  - Reproducir descargas → `url.load {path: downloads.directory, play:true}`.

#### Valoraciones

- Reads: `taste.list` → `ratings[]` (G11).
- Actions: Deshacer → `taste.rate {kind, playlist:id, value:0}`.
- No cache.

#### Estadísticas

- Reads: `stats {name:"week"|"month"|"year"|"all"}` (G16).
- Actions: the period chips.
- No cache.

#### Player

Reads:
- The snapshot.
- `lyrics {}` when the Letras view opens, and on each track change while it stays open. Cache by `track.path`.
- The visualizer frame stream (§5.4).

Actions:

| Action | Operation |
|---|---|
| Transport | `toggle`, `next`, `prev` |
| Seek | `seek.absolute {value}` |
| Shuffle | `shuffle {name:"toggle"}` |
| Repeat | `repeat {name:"cycle"}` |
| ♥ | `playlist.bookmark {provider:"ytmusic", track}` → `favorite` (G20) |
| Visualizer | `vis {name}` |
| Sleep timer | `sleep {value}` |
| Speed | `speed {value}` |

#### Queue

Reads:
- `playnext.list {limit:200}`.
- `queue.list {sort:"upnext", offset, limit:200}` (G22).
- Refetch, debounced 100 ms, when `playlist_revision` or `play_next_total` changes while the page is visible.

Actions:
- `queue.play {index, if_revision}`;
- `queue.remove {index, if_revision}`;
- `playnext.remove {index}`;
- `queue.move {index, to, if_revision}` (adjacent swaps, §7.7);
- `playnext.move {index, to}`;
- `playnext.clear`, `queue.clear`;
- Guardar como lista → `playlist.create`, then `playlist.add_many {provider, playlist:id, tracks}` (from `queue.list` pages).

#### Eq

- Reads: `eq {name:"list"}` → `items`, `eq_preset` (G19). The bands come from `snapshot.eq_bands`.
- Actions:
  - `eq {name}`;
  - `eq {band:i, value:dB}` through a conflated sender: at most one in flight, the latest value wins, sent during the drag and on release;
  - `settings.set {name:"eq_auto", value:0|1}`.

#### Ajustes

- Reads:
  - snapshot fields: `theme`, `visualizer`, `speed`, `sleep_*`, `normalize`, `crossfade`, `eq_auto`, `auto_radio`, `mono`;
  - `theme {name:"list"}` → `themes[]` (G17).
- Actions: `theme {name}`, `vis {name}`, `speed {value}`, `sleep {value}`, `settings.set {name, value}` (G14) for `normalize`, `crossfade`, `eq_auto`, `auto_radio` and `mono`.

#### Item sheet

Actions:
- `track.queue {track}`;
- `radio.start {track}` or `{provider, playlist, name}`;
- `discover.remixes {track}` → `tracks` (G13);
- `download {track}` or `{provider, playlist, name}`;
- `taste.rate {kind, playlist?, name, track?, value:±1}`;
- `playlist.add {provider, playlist, track}` and `playlist.add_many`.

After `taste.rate`, `provider.favorite`, `playlist.bookmark` or any `playlist.*` edit: `Remote.invalidate()`.

### 5.4 Live state

**Snapshot merge,** a pure function `Snapshots.merge(cur, new, source, floor)`, tested.

- **Sources:**

  | Source | Comes from |
  |---|---|
  | `EVENT` | a `runtime.state` event without `retained` |
  | `RETAINED` | an event with `"retained":true` |
  | `STATE_GET` | a `state.get` reply |
  | `PLAYBACK_JOB` | the snapshot of a job whose operation is one of: `play`, `pause`, `toggle`, `stop`, `next`, `prev`, `seek`, `seek.absolute`, `queue.play`, `queue.replace`, `track.play`, `provider.load`, `provider.load_album`, `radio.start`, `url.load`, `speed` |
  | `OTHER_JOB` | the snapshot of any other job |

- **Revision:** a snapshot with `revision < floor` is ignored. The floor becomes the accepted revision. A (re)subscribe resets the floor to 0, because the revision restarts at 0 in every core process.
- **Position:** `position` and `receivedAt = SystemClock.elapsedRealtime()` are taken from `EVENT`, `STATE_GET` and `PLAYBACK_JOB`, and from any source when there is no current snapshot or the track path changed. Otherwise the current position and `receivedAt` are kept, because a retained event or an unrelated job snapshot can carry a minutes-old position. Every other field is always taken.

**Position clock,** a pure function `PlaybackClock.positionAt(snap, now)`:
- While `state == "playing" && !buffering && !seeking`: `position + (now − receivedAt)/1000 × speed`, clamped to `duration` when `duration > 0`.
- Otherwise `position`.

The ticker is a `delay(250)` loop local to the seek bar and the time text, running only while playing. The bar draws in `drawBehind`. The text uses `derivedStateOf { pos.toInt() }`.

**Seek:**
- While dragging, the bar shows the local value.
- After release, it keeps that value until a snapshot arrives whose position is within 2 s of the target (G7 reports the target while `seeking`), or 5 s pass.
- **Queue:** reacts to `playlist_revision` and `play_next_total`.
- **Notices and errors:** §2.10, from the G7 fields.

**Visualizer frames:** a cold, conflated `Flow<VisFrame>` on the view's cell grid, collected only while all of these hold:
1. MainActivity is STARTED.
2. A visualizer is visible: the player strip, the big view, or the Visualizador preview.
3. `state == "playing"`.

Each frame goes into a state holder read **only** inside the Canvas draw lambda.
- The grid comes from the view and the font: rows of about 12 dp fill the height exactly, the columns are whole cells of the font's advance, centered.
- A new grid restarts the flow; the core keeps that grid while it draws nothing, so the stateful modes keep their animation.

### 5.5 Paging

| Operations | Contract |
|---|---|
| `provider.playlists`, `provider.tracks`, `provider.album_tracks`, `provider.artist_albums`, `provider.artists` (no sort), `queue.list`, `playnext.list` | `total` is the full length. Load pages of `limit:200`, and fetch the next page when the last visible row is within 20 of the loaded end. "All pages" loops until `offset ≥ total`. The absolute row index is `offset + i` |
| `provider.albums` and `provider.artists` with `sort:"foryou"` | `total` is the page length. The next offset is `offset + limit` (not + returned), and an empty page ends the list |
| `provider.search` | never paged |

The `foryou` contract has a known gap. `shortcut: a page cut short by the discovery round cap leaves a gap that the next offset skips; acceptable for an endless random list; upgrade if users notice missing rows (have G4 return next).`

### 5.6 Row names and flags

- Names stay exactly as the core sends them. The flags come from G1 booleans and ID prefixes. `RowName` strips, for display only:
  - the prefixes `♥ `, `● `, `★ `;
  - the suffix ` · local`;
  - when `single` is true, the text after the last ` · `.
- Release rows (`al:`) are named `Title · Artist[ · Single]`. The tile title is the text before the first ` · `, and the subtitle is the rest.
- In artist preview lines, a leading `♫ ` is dropped.
- A local row is one whose ID starts with `local:`.

### 5.7 Errors

**By `IpcError.code`:**

| Code | Handling |
|---|---|
| connect failure, `unavailable` | the reconnect banner |
| `conflict` on a queue edit | refetch silently |
| `invalid_params` | log it and show `ERR: <operation>` (this is a bug) |
| `internal_error` | show `detail` |

**Network-looking** means `detail` contains, in a lowercase compare, any of: `no such host`, `timeout`, `timed out`, `network is unreachable`, `connection refused`, `connection reset`, `i/o timeout`, `temporary failure in name resolution`. Show "Sin conexión a internet."

A `detail` containing `sign in` or `sign-in` shows the session card.

---

## 6. Go changes (contract for the Go agent)

**For every item, follow CLAUDE.md "To add an operation":**
1. Register it in `DefaultOperationRegistry` (`ipc/operations.go`) with its parameters.
2. Add the Request and Response fields in `ipc/protocol.go`, and the snapshot fields in `ipc/v2.go`.
3. Route it:
   - `provider.*` and `playlist.*` reach `handleIPCLibrary` by themselves; add a `case` there.
   - `queue.replace` joins the `queue*` case list of `handleV2Request` and `v2MutatesLivePlaylist`.
   - `taste.rate`, `taste.list`, `settings.set`, `download.status`, `download.cancel` and `download.retry` are synchronous cases in `handleV2Request`.
   - `radio.start`, `discover.remixes`, `download` and `stats` use the reply-channel pattern: add them to the case that calls `handleV2DeferredRequest`, which finishes through `waitV2ResponseCmd`.
4. Add table-driven tests in `ui/model/ipc_native_test.go` (new), `ipc/` tests for the registry and protocol changes, and `external/ytmusic` tests for the provider changes.
5. Document each item in `docs/remote-control.md`: the operations table, the snapshot fields and every new result shape.

**Other rules:**
- IPC error details are English, or existing `L()` strings. Any new `L()` text needs its Spanish entry in `i18n_es.go`, or `TestSpanishCatalog` fails.
- Headless mode serves every new operation. `ui.render` stays unregistered there.
- Before each commit, run `mise exec -- make check`; before each batch is done, run `mise exec -- make ci`.
- Commit subjects use `area: what changed`, with a body.

**Protocol summary:**
- **New Request field:** only `Kind string `json:"kind,omitempty"``. Everything else reuses existing fields: `playlist` carries item IDs, `name` names and enums, `value` numbers, and `sort`, `track`, `tracks`, `index`, `offset`, `limit` keep their meaning.
- **New Response fields** (all omitempty): `Shelves []ShelfInfo`, `Genres []GenreInfo`, `Favorite *bool`, `Ratings []RatingInfo`, `Downloads *DownloadsInfo`, `Stats *StatsInfo`, `Themes []ThemeInfo`, `Preview *PreviewInfo`.
- **New fields on existing types:**
  - `PlaylistInfo`: `image_url`, `writable`, `unseen`;
  - `AlbumInfo`: `image_url`, `favorite`, `single`;
  - `ArtistInfo`: `image_url`, `favorite`;
  - `ProviderInfo`: `signed_in *bool`.

### Batch A: player, playback-fix UX, queue

**G7 · Snapshot fields.**

Add to `ipc.RuntimeSnapshot` (all omitempty):
- `buffering bool`;
- `seeking bool`;
- `error`, `error_kind`, `error_path` (strings);
- `notice *NoticeInfo{text, kind}`;
- `sleep_minutes int`, `sleep_at int64`;
- `auto_radio bool`;
- `normalize string`;
- `crossfade int`;
- `eq_auto bool`.

How each one is filled:

| Field | Value |
|---|---|
| `buffering` | `m.buffering` |
| `seeking` | `m.seek.active`. While it is true, `position = m.seek.targetPos.Seconds()` (`runtimeSnapshot` today reports the pre-seek position until the yt-dlp seek lands, and no event follows) |
| `error`, `error_kind`, `error_path` | **only from a new `playErr error` / `playErrPath string` on the Model, never from `m.err`**. `m.err` also holds browse errors of the hidden TUI pane, with "Ctrl+R to retry" hints |
| `notice` | `m.status` when its text is not empty and `!Expired(now)`. `kind` is `activity`, `success`, `warning` or `error`, from `feedbackKind` |
| `sleep_minutes` | `m.sleep.minutes` (−1 means end of song) |
| `sleep_at` | `m.sleep.at.Unix()` when minutes > 0 |
| `auto_radio` | `m.autoRadio` |
| `normalize` | `normalizeModeName` of the `normalizer` interface: `off`, `track` or `album`; "" without it |
| `crossfade` | `int(m.crossfade / time.Second)` |
| `eq_auto` | `m.eqAuto` |

The play error:
- Add `func (m *Model) setPlayError(path string, err error)` in `play_errors.go`. Call it at these sites:

  | Site | Call |
  |---|---|
  | The `PlayAt` failure in `playback.go`, including the `ErrNeedsAuth` branch | set |
  | `playback.go` where an async start begins (`m.err = nil` before `playStreamCmd`) | clear |
  | The synchronous success | clear |
  | `handleStreamPlayed` (`update_playback.go`): the error branches, including `ErrNeedsAuth` | set |
  | `handleStreamPlayed`: success | clear |
  | `handleYTDLUnpauseReconnect`: error | set |
  | `handleYTDLUnpauseReconnect`: success | clear |

- Split `playErrorHint` into `playErrorKind(err) string` and `playErrorHint`, which switches on the kind. `playErrorKind` returns:
  - `session` for `errors.Is(err, playlist.ErrNeedsAuth)`;
  - otherwise `network`, `session`, `ytdlp`, `unavailable` or "", from today's substring tables.

Fingerprint: add to `ipcRuntimeFingerprint` and `runtimeFingerprint()` the fields `seeking`, the play error text and path, the notice text and kind, `sleepMinutes`, `sleepAt`, `autoRadio`, `normalize`, `crossfade` and `eqAuto`. Then a landing seek, an expiring notice or a play error publishes `runtime.state`.

Tests:
- each field reaches the snapshot;
- each one changes the fingerprint;
- the `playErrorKind` table;
- a provider-pane load error leaves `error` empty;
- a seek in flight reports the target and `seeking`, and its landing changes the fingerprint.

**G9 · Spectrum while the screen is not drawn.**
1. In `v2BandsResponse`: `if m.headless { m.tickVisualizer(now) } else if m.renderPaused { m.tickVisualizerForClient(now) }`.
2. Add, in `ui/model/tick.go`:
   ```go
   // tickVisualizerForClient analyzes the audio that plays now for a
   // spectrum client while nothing is drawn. The hidden layout may give the
   // visualizer no rows (simplified mode, the minimal tier), so the analysis
   // gets at least one row and a column count.
   func (m *Model) tickVisualizerForClient(now time.Time) {
       if m.vis == nil || m.player == nil || m.vis.Mode == ui.VisNone {
           return
       }
       m.vis.Rows = max(1, m.vis.Rows)
       if m.vis.Cols <= 0 {
           m.vis.Cols = 80
       }
       ctx := m.visualizerTickContext(now)
       ctx.Playing = m.player.IsPlaying() && !m.player.IsPaused()
       ctx.Paused = m.player.IsPlaying() && m.player.IsPaused()
       ctx.OverlayActive = false
       m.vis.Tick(ctx)
   }
   ```
3. While `renderPaused` (and not headless), neither path may call `vis.Suspend()`. Suspend resets the frame timing and ticks the driver with `OverlayActive`, which breaks the smoothing between spectrum requests.
   - `tickVisualizer` returns at once when `m.renderPaused && !m.headless`. The spectrum client drives the analysis.
   - `maybeRequestVisualizerRefresh` returns at once when `m.renderPaused`.
4. `previewCmds` returns nil when `m.renderPaused`. Nobody sees the preview pane, so this saves network and CPU. `coverCmds` stays ungated: the cover theme needs it.
5. No new render mode and no tick change.
6. Tests:
   - render paused, with a playing fake player, `spectrum.get` returns non-empty `bands` that change between calls, in each of: the normal layout, the provider screen focused, `simplified = true`, and a 50×14 window;
   - the bands keep their smoothing across two calls (no Suspend).

**G5 · Start index for list loads.**
- In `handleIPCProviderLoad`: `start := 0; if i := result.request.Index; i > 0 && i < len(result.tracks) { start = i }`. Then `SetIndex(start)` and `plCursor = start`.
- The reply becomes `resp := m.v2PlaylistResponse(); resp.Playlist = result.request.Playlist`, so at most 200 tracks are returned instead of the whole list.
- Add `"index"` to the parameters of `provider.load` and `provider.load_album`.
- Test: index 3 plays track 3; an out-of-range index plays 0; the reply has at most 200 tracks.

**G6 · `queue.replace {tracks, index, if_revision}`.**
- In `keys_nav.go`, split `replaceQueueFromNav`:
  ```go
  // replaceQueueAt replaces the queue with tracks and plays from start. Keys
  // and IPC share it.
  func (m *Model) replaceQueueAt(tracks []playlist.Track, start int) tea.Cmd {
      // the current body of replaceQueueFromNav without the focus, nav and
      // status lines, with SetIndex(start), plCursor = start, plScroll = 0
  }
  ```
  `replaceQueueFromNav` keeps its focus, nav and status lines and calls `replaceQueueAt(tracks, 0)`.
- `queue.replace`:
  - empty `tracks`, or more than 1000 → `invalid_params`;
  - `index` is clamped into range;
  - convert with `ipcTrackFromInfo`;
  - reply `v2PlaylistResponse()`.
- Register `tracks`, `index`, `if_revision`. Add it to the queue case list and to `v2MutatesLivePlaylist`.
- Tests: success with index 2, empty input, a stale revision → `conflict`.

**G22 · `queue.list {sort:"upnext", offset, limit}`.**
- `pos := m.playlist.OrderPosition(m.playlist.Index())`, then `idx, tracks := m.playlist.OrderWindow(pos+1+offset, min(limit,200))`.
- Each item is `ipcTrackInfo(track, idx[i], QueuePosition(idx[i]), fav)`, so `index` stays the real index.
- `total = Len() − pos − 1`.
- Other `sort` values keep today's behavior. Register `sort`.
- Test with and without shuffle.

**G19 · `eq {name:"list"}`.** In `handleV2EQ`, before the preset branch: `items` = the names of `eqPresets` plus "Custom", and `eq_preset = m.EQPresetName()`.

**G21 · Sign-in state.**
- Add `func (p *Provider) SignedIn() bool { return p.client.SignedIn() }` in `external/ytmusic/provider.go`. It does no I/O.
- In `provider.list`, when the provider implements `interface{ SignedIn() bool }`, set `ProviderInfo.SignedIn = &v`.

**G17 · Theme palette and list.**
- Add `func (m Model) themePalette() theme.Theme`:
  - it starts from the active entry;
  - when `coverThemeActive()`, it takes `m.coverFade.to` while `m.coverFade.active`, else `m.coverFade.shown`;
  - when that theme's `BG` is not empty, it keeps the entry's name and returns those colors.
- `runtimeSnapshot` builds `ThemeInfo` from it.
- Add `themeBG` and `themeAccent` to the fingerprint, so each new cover palette publishes `runtime.state`.
- `theme {name:"list"}` also returns `themes: []ThemeInfo`: Default first with its name only, then every `m.themes` entry with its colors.
- Test:
  - with the cover theme and `shown` set, the snapshot carries `shown`'s colors;
  - changing `to` while a fade is active changes the fingerprint;
  - `list` returns `themes`.

**G20 · Results that carry state.**

| Operation | New result field |
|---|---|
| `playlist.create` | `playlist`: the ID that `CreatePlaylist` returns (ytmusic already returns `pl:VL…`; local returns the name) |
| `provider.favorite` | `favorite: &added` |
| `playlist.bookmark` | `favorite: m.playlistTrackFavorited(track)` after the toggle |

### Batch B: browse

**G1 · Artwork and flags on rows.**
- `ipcProviderPlaylistInfos`:
  - `ImageURL: list.ImageURL`;
  - `Unseen: strings.HasPrefix(list.Name, "● ")`;
  - `Writable`: the provider is a `provider.PlaylistWriter` and (it is not a `provider.PlaylistTargetFilter` or `CanAddToPlaylist(list)`).
- `ipcAlbumInfos`: `ImageURL`, `Favorite: strings.HasPrefix(album.Name, "♥ ")`, `Single: album.Single`.
- Add `Single bool` to `provider.AlbumInfo` (`provider/types.go`), and set it in the ytmusic singles loop of the artist albums (`provider.go`, where it appends `" · " + p.words.singles`).
- The `provider.artists` mapping: `ImageURL`, and `Favorite` from the `♥ ` prefix.
- Names stay unchanged, for the CLI and the TUI.

**G2 · `provider.home {provider}`.**
- In `provider/types.go`:
  ```go
  type ShelfItem struct{ Kind, ID, Title, Subtitle, ImageURL string; Track *playlist.Track }
  type Shelf struct{ Title string; Items []ShelfItem }
  ```
- In `provider/interfaces.go`: `type HomeLoader interface{ HomeShelves() ([]Shelf, error) }`.
- ytmusic `HomeShelves` builds the shelves from `p.cachedHome()`, item by item:

  | Source item | Kind | ID |
  |---|---|---|
  | KindSong, KindVideo | `song` | the videoId; `Track = &songTrack(*it.Song)` |
  | KindAlbum | `album` | `MPREb_…` |
  | KindPlaylist | `playlist` | `"pl:"+it.ID` |
  | KindArtist | `artist` | `UC…` |

  - `Subtitle` is `it.Subtitle`, or `it.Artist` when it is empty. `ImageURL` is `it.Thumbnail`.
  - Drop items where `p.hidden([]string{it.Title, it.Artist}, it.ID)`, the `LM` playlist, songs that `p.visibleTracks` drops, and empty shelves.
- Run it in a `tea.Cmd`. Song tracks carry ♥ through `trackFavoriteLookup`.
- Result `{ok, shelves:[{title, items:[{kind, id, title, subtitle, image_url, track?}]}], total:<shelf count>}`.
- Without HomeLoader: "provider does not support home".

**G3 · Genres.**
- `provider.genres {provider, playlist?}`:
  - the browser is `entry.Provider.(provider.GenreBrowser)`;
  - when the provider is a `provider.GenreBrowseRouter` and `playlist != ""`, use `GenreBrowserFor(playlist)` (`browse:moods` gives moods);
  - result `{ok, genres:[{id, name, group, favorite}], sorts: GenreSortTypes(), total}`.
- `provider.genre_tracks {provider, playlist:genreID, sort, offset, limit}` → `GenreTracks(id, sort)` in a Cmd → `{ok, tracks, total}` (`ipcPage`, max 200). It handles params, `search:` and `local:genre:` IDs.
- `provider.genre_favorite {provider, playlist:genreID}` → `GenreFavoriteToggler.ToggleGenreFavorite` → `{ok, favorite}`.

**G4 · "For you" artists.** In `provider.artists`, when `sort == "foryou"` and the provider is a `provider.ArtistPager`, return `ArtistList(offset, limit)` (default 40, max 200) in the Cmd → `{ok, artists, total: len(page)}`. Register `sort`.

**G10 · `provider.refresh {provider}`.** In a Cmd, call `playlist.Refresher.Refresh()` → `{ok}`. Without it: "provider does not support refresh".

**G18 · `provider.preview {provider, playlist:id}`.**
- In a Cmd with a 15 s timeout, call `provider.PreviewLoader.PreviewDetails(ctx, id)` → `preview:{image_url, lines}`.
- Also add the state of the save and follow buttons:
  - new optional interface `provider.FavoriteStater { FavoriteState(id string) (on, known bool) }`;
  - ytmusic implements it for `MPREb_…` (via `savedAlbums`, loading `allAlbums()` when nil) and `UC…` (via `subscribed`, loading `Artists()` when nil), under `p.mu` as `ToggleFavorite` does;
  - it returns `known = false` when signed out or for other IDs;
  - when known, set `preview.favorite = &on`.
- Test: a saved album gives `favorite:true`; a signed-out provider gives no `favorite`.

### Batch C: actions, library, settings

**G11 · Ratings.**
- `taste.rate {kind, playlist?, name, track?, value}`:
  - `value` must be −1, 0 or 1.
  - **value 0:** `id = playlist`, or `track.path` when `track` is set; it must not be empty. Call `m.taste.Rate(id, taste.Item{Kind: kind}, taste.Neutral)`. No `track` is needed for any kind; this is the Deshacer path.
  - **±1:** `kind` must be song, album, artist or genre.
    - A song needs `track`, and `id = track.Path`. Other kinds need `playlist`.
    - Build `interestTarget{kind, id, name, track}`.
    - For −1 on the song that plays now, set `hide = func(m *Model) tea.Cmd { return m.skipNext() }`.
    - Return `m.applyInterest(target, value)`, which also does the ♥ like and the YouTube dislike.
  - Result `{ok}`; anything invalid → `invalid_params`.
- `taste.list` → `{ok, ratings:[{id, kind, name, artist, interest, at(RFC3339)}]}` from `m.taste.Ratings()`.
- Tests: ±1 for each kind; 0 without a track; the bad cases.

**G12 · `radio.start {track}` or `{provider, playlist:id, name}`.**
- Extract `moreLikeThisFrom(seed discoverSeed, reply chan<- ipc.Response) tea.Cmd` from `moreLikeThis`. `moreLikeThis` calls it with `discoverTarget()` and a nil reply.
- `moreLikeThisMsg` carries `reply`. In `handleMoreLikeThis`, with a reply set:
  - every early return replies `{ok:false, error}`, using the existing `L()` texts such as "No related songs found";
  - a superseded `gen` replies "superseded";
  - success replies `v2PlaylistResponse()` after the replace.
- The seed comes from `{track}` (`ipcTrackFromInfo`), or from `{prov: the entry's provider, itemID: playlist, label: name}`.

**G13 · `discover.remixes {track}`.**
- In a Cmd: take the searcher as `findRemixes` does, then `searchRemixes(ctx, searcher, track, cleanSongTitle(track.Title))` (the title itself when that is empty), with `discoverTimeout`.
- Result `{ok, tracks, total}`. `provider_meta.section` holds the group.
- No overlay opens.

**G14 · `settings.set {name, value}`.**
- Refactor each toggle into a setter that the key path and IPC share:
  - `setAutoRadio(on)`, used by `toggleAutoRadio`;
  - `setNormalizeMode(mode)`, used by `cycleNormalize`;
  - `setEQAuto(on)`, used by `toggleEQAuto`;
  - new `setCrossfade(sec)`: `SetCrossfade(sec)` plus `saveConfigFloat("crossfade", float64(sec), 0)`.

| `name` | `value` | Effect |
|---|---|---|
| `auto_radio` | 0 or 1 | `setAutoRadio` (saves `[ytmusic] auto_radio`) |
| `normalize` | 0 off, 1 track, 2 album | `setNormalizeMode` (saves `normalize`) |
| `crossfade` | 0 to 12 (whole seconds) | `setCrossfade` (saves `crossfade`) |
| `eq_auto` | 0 or 1 | `setEQAuto`, which runs `applyAutoEQ` when on (saves `eq_auto`) |
| `mono` | 0 or 1 | set the player mono, and `saveConfigBool("mono", on)` |

- Result `{ok}`. The state comes back in the snapshot (G7). Any other name or value → `invalid_params`.

**G15 · Downloads.**
- Refactor `downloadHighlighted` into `downloadSeed(seed discoverSeed) tea.Cmd`.
- `download {track}` or `{provider, playlist:id, name}`:
  - with no manifest → `unavailable`;
  - otherwise `{ok}` once the jobs are queued or resolving.
- `download.status` → `{ok, downloads:{running, current?:TrackInfo, percent, queued:[TrackInfo], done, failed:[{track, error}], directory}}`. `directory` comes from `tracksave.Directory(m.downloads.dir)`.
- `download.cancel` → `cancelDownloads()`; `download.retry` → `retryFailedDownloads()`.

**G16 · `stats {name:"week|month|year|all"}`.**
- In a Cmd: `m.statsLog.Load()`, then `stats.Summarize(plays, period, time.Now(), 10, 30)`.
- Result `{ok, stats:{plays, secs, songs, artists, top_artists:[{name, detail, plays, secs}], top_songs, top_genres, daily:[30 numbers, minutes per day, oldest→today], streak}}`.
- Use an `ipc.StatsInfo` mirror type, so `internal/stats` gets no JSON tags.

**Not done, with the reason:**

| Item | Reason |
|---|---|
| Big `runtime.job` events | nothing reads results from events; the app polls `job.get` |
| A `native` render mode, or a faster tick under render off | the spectrum comes from `spectrum.get`; track advance and sleep already run at the 500 ms tick |
| `sign_in_required` | covered by `error_kind:"session"` and `signed_in` |
| Multi-track `track.queue` | the client loops |
| Real move | drag sends adjacent swaps |
| Full artist page op | assembled from `artist_albums`, top songs and `preview` |
| Filtered search | not needed |
| Blocked words | Modo terminal |
| Preamp volume | not needed; Android volume |

---

## 7. Android architecture

### 7.1 Files (package `io.github.xaviiromf.cliampx`, module `app`)

| File | Status | Responsibility |
|---|---|---|
| `Core.kt` | edit | Guard `YoutubeDL.init` + `FFmpeg.init` with an `AtomicBoolean`, set after the first success, so they run once per process |
| `Ytdlp.kt` | new, about 130 lines | The yt-dlp updater, its status flow and the pure policy (§7.8) |
| `Ipc.kt` | edit | Add `job(id)` and `cancel(id)`. Add `class SpectrumStream(socket)`: one persistent connection, `next(): FloatArray`, `close()`, soTimeout 2 s |
| `Remote.kt` | new | The process-wide object: subscription, `state`, `connected`, `ops`, `signedIn`, `quitting`, `errors`, `call`/`fire`, the play gate, the result cache, render serialization, `visFrames()`, the error watcher. Typed helpers per operation live here (`home()`, `search(q)`, `queueReplace(tracks, i)` …). There is no separate Api file |
| `Models.kt` | new | Immutable `@Immutable` data classes and pure parsers: `Snapshot`, `Track(raw)`, `ThemeInfo`, `Notice`, `Row`, `Album`, `Artist`, `Genre`, `Shelf`, `ShelfItem`, `Lyric`, `HistoryEntry`, `Downloads`, `Stats`, `Rating`, `Preview`. Also `RowName`, `searchSections`, `Snapshots.merge`, `PlaybackClock` |
| `CliampService.kt` | edit | §7.11 |
| `MainActivity.kt` | replace | `ComponentActivity`. Details below this table |
| `TerminalActivity.kt` | rename of today's MainActivity | Keeps the TerminalView, the extra keys, pinch font, the binding, `attach`/`onScreenUpdated`, and `padForSystemBars`. Removes permissions, deep links and `offerUpdate`. `onStart`/`onStop` call `Remote.terminalVisible(true/false)` in place of `service.render`. Back at `[Cola]`/`[Queue]` → `finish()`. Manifest: not exported; same `configChanges`; `windowSoftInputMode="adjustResize\|stateAlwaysHidden"` |
| `LoginActivity.kt` | edit | Add `companion fun signOut(ctx)`: delete `ytmusic_cookies.txt` and `ytmusic_auth.json`, `CookieManager.getInstance().removeAllCookies(null)`, then `CliampService.send(ctx, ACTION_RESTART)`. Move `importCookies(ctx, uri)` here from SettingsActivity |
| `Updates.kt` | keep | One edit: `useCaches = false` on the APK download connection |
| `SettingsActivity.kt` | keep in M1–M4, delete in M5 | In M1, add a "Modo terminal" button, and make its "Actualizar yt-dlp" call `Ytdlp.update(ctx, force = true)` |
| `ui/Theme.kt` | new | `CliampPalette`, `paletteOf(ThemeInfo?)`, the winamp fallback, derived tokens, `LocalPalette`, `CliampTheme {}` (M3 colorScheme, typography, shapes, no-ripple indication, system bars) |
| `ui/Widgets.kt` | new | `SectionRule`, `ScreenRule`, `BracketChip`, `KeyPill`, `IconBtn`, `TrackRow`, `Tile`, `Spinner`, `StatusLine`, `Banner`, `EmptyState`, `ErrorState`, `SeekBar`, `NameSheet`, `ConfirmSheet` |
| `ui/Art.kt` | new | The image loader (§7.4) |
| `ui/Nav.kt` | new | `Tab`, `Screen`, the process-level `Nav` object |
| `ui/App.kt` | new | Scaffold: tab content, status line, mini-player, tab bar, player-sheet host, sheet host, splash, banners |
| `ui/Home.kt`, `ui/Search.kt`, `ui/Library.kt`, `ui/Detail.kt` | new | Inicio, Buscar, Biblioteca; Album, Artist, Playlist, Genre, ParaTi, Remixes |
| `ui/Player.kt`, `ui/Queue.kt`, `ui/Visualizer.kt`, `ui/Eq.kt` | new | The player sheet pages and the Canvas renderers |
| `ui/Settings.kt` | new (M5) | Ajustes, the Tema and Visualizador pickers, the speed, sleep, normalization and crossfade sheets |
| `ui/Menus.kt` | new | Action sheets and the add-to-playlist sheet |
| `ui/Stats.kt` | new | Estadísticas and Valoraciones |

The new `MainActivity` does this:
- `enableEdgeToEdge()` and `volumeControlStream = AudioManager.STREAM_MUSIC`.
- The one-time permission request (`asked_permissions`), moved from today's MainActivity.
- `startForegroundService(EXTRA_LINK)` for VIEW and SEND intents.
- `Modifier.semantics { testTagsAsResourceId = true }` on the root.
- `setContent { CliampTheme { App() } }`.
- It finishes when `Remote.quitting` becomes true.
- `EXTRA_OPEN_PLAYER` opens the sheet.
- No service binding.

**Resources and other files:**
- `res/font/jetbrains_mono_regular.ttf` and `jetbrains_mono_bold.ttf`;
- `assets/licenses/JetBrainsMono-OFL.txt`;
- the new `res/drawable/ic_*.xml`;
- the strings in `values/strings.xml`;
- tests in `app/src/test/java/io/github/xaviiromf/cliampx/`;
- `android/scripts/ui-smoke.sh`.

**Manifest:**
- add `TerminalActivity`;
- add `android:enableOnBackInvokedCallback="true"` on `<application>`;
- MainActivity keeps its intent filters and `configChanges`, with `windowSoftInputMode="adjustResize"`;
- remove `SettingsActivity` in M5.

### 7.2 Remote (`object`, about 300 lines)

**Lifecycle:**
- `start(ctx)` is idempotent and is called from `CliampService.onCreate`.
- `scope = CoroutineScope(SupervisorJob() + Dispatchers.Default)`.
- `stop()` cancels the subscription loop.

**Flows:**

| Flow | Type |
|---|---|
| `state` | `StateFlow<Snapshot?>` |
| `connected` | `StateFlow<Boolean>` |
| `ops` | `StateFlow<Set<String>>` |
| `signedIn` | `StateFlow<Boolean?>` |
| `quitting` | `StateFlow<Boolean>` |
| `errors` | `SharedFlow<String>` (replay 0, buffer 8) |

**Calls:**
- `suspend fun call(op, params = JSONObject(), timeoutMs = default(op)): JSONObject` runs on `Dispatchers.IO` (§5.1). For play operations it first awaits the play gate (§7.8).
- `fun fire(op, params)` is `scope.launch { runCatching { call(op, params) }.onFailure { errors.tryEmit(msg) } }`.

**Cache:** `suspend fun <T> cached(key, ttlMs, load)` keeps an in-memory map of value and time. `invalidate()` clears it all.

**Render:** `fun terminalVisible(on: Boolean)` (§7.6).

**Visualizer frames:** `fun visFrames(cols, rows, periodMs): Flow<VisFrame>` is a conflated flow on IO over `FrameStream`, one request at most every periodMs (33, or 100 under reduced motion), parsed on IO. It reconnects after 500 ms on an error.

**Threading:** all socket I/O and JSON parsing run on IO. Composables collect with `collectAsStateWithLifecycle()`. No `JSONObject` reaches a composable.

### 7.3 Navigation (hand-written, about 70 lines)

- `sealed interface Screen`, each with a stable `key`:
  - `Home`, `Search`, `Library`;
  - `Album(id, title, art, subtitle)`, `Artist(id, name, art)`, `Playlist(provider, id, title, art)`, `Genre(id, title)`, `ParaTi(kind)`, `Remixes(trackRaw, title)`;
  - `Settings`, `Themes`, `Visualizers`, `Eq`, `Stats`, `Ratings`.
- `object Nav` holds `tab` and one `mutableStateListOf<Screen>` per tab. It lives in the process, so it survives an activity recreation (a font-scale change recreates the activity). It has `push`, `pop` and `select(tab)`; a reselect pops to the root.
- **Hosting:** `AnimatedContent(stack.last(), crossfade 150 ms)` inside `rememberSaveableStateHolder().SaveableStateProvider(screen.key)`, so each screen's `LazyListState` survives push, pop and recreation. The `BackHandler`s follow the §2.9 order.
- **Player sheet:**
  - `ModalBottomSheet(sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true), dragHandle = null, shape = RectangleShape, containerColor = bg, contentWindowInsets = { WindowInsets(0) }, sheetGesturesEnabled = page == PlayerPage.Player)`;
  - the pages are a local `mutableStateOf(PlayerPage.Player)`;
  - sheet gestures are off on Queue and Eq, so the list scroll, the reorder drag and the EQ drags never fight the sheet.

### 7.4 Images (`ui/Art.kt`, about 90 lines, no library)

1. Install `HttpResponseCache.install(File(cacheDir, "http"), 32 MB)` once. The APK and yt-dlp downloads set `useCaches = false`.
2. **URL rewrite,** a pure tested function:
   - googleusercontent and ggpht: `=w\d+-h\d+` → `=w{px}-h{px}`;
   - `i.ytimg.com/vi/<id>/mqdefault.jpg` → `hqdefault.jpg` when `px > 320`.
3. Decode with `ImageDecoder.createSource(ByteBuffer)`, `setTargetSize` (keep the aspect; center-crop when drawing) and `ALLOCATOR_HARDWARE`.
4. **Local tracks** (no URL, the path is a file): `MediaMetadataRetriever().apply { setDataSource(path) }.embeddedPicture`, decoded the same way.
5. Keep an `LruCache<String, ImageBitmap>` sized in bytes (1/8 of `maxMemory`), an in-flight `Map<String, Deferred>` and a `Semaphore(4)`.
6. `@Composable fun rememberArt(url: String?, px: Int): ImageBitmap?` uses `produceState`, so it cancels when the row leaves.
7. **Placeholder:** a `░` texture in `fg` at 25 % with a centered `ic_note`.
8. `suspend fun software(url, px): Bitmap?` decodes with `ALLOCATOR_SOFTWARE`, for the MediaSession.

Sizes: rows 48 dp (126 px), tiles 136 dp (357 px), the player at full width (1080 px).

### 7.5 Gestures

| Where | Gesture | Implementation | Effect |
|---|---|---|---|
| Mini-player | tap | `clickable` | open the player sheet |
| Mini-player | drag up > 48 dp or fling up | `draggable(Orientation.Vertical)` | open the player sheet |
| Mini-player, player art (Portada and Visualizador) | horizontal drag > 30 % width or fling > 1000 dp/s | `pointerInput { detectHorizontalDragGestures }` + `graphicsLayer.translationX`, spring back; the art keeps 24 dp from the screen edges | `next` (left), `prev` (right) |
| Player art | tap | `clickable` | Portada ⇄ Visualizador |
| Visualizer strip | tap | `clickable` | `vis {name: next mode of vis list}` |
| Player sheet (Player page) | swipe down, back | M3 | close |
| Seek bar | drag, tap | `pointerInput` (`detectHorizontalDragGestures` + `detectTapGestures`) | `seek.absolute` on release (§5.4) |
| Rows, tiles | tap / long-press / ⋮ | `combinedClickable(onLongClick)` + `HapticFeedbackType.LongPress`; ⋮ is a 48 dp `IconButton` | play / action sheet |
| Queue rows | swipe end→start | `SwipeToDismissBox(enableDismissFromStartToEnd = false)` | `queue.remove` or `playnext.remove` |
| Queue rows | drag `≡` | `detectDragGestures` on the handle over `LazyListState.layoutInfo` | reorder (§7.7) |
| Inicio, Biblioteca, detail pages, Para ti | pull | `PullToRefreshBox` | §5.3 refresh |
| EQ column | vertical drag, tap | `pointerInput` per column, 1 dB steps | conflated `eq {band, value}` |
| Lists | scroll near the end | `snapshotFlow { lastVisibleIndex }` | the next page |

### 7.6 Render arbitration (serialized)

- The desired mode is `"on"` while TerminalActivity is STARTED, else `"off"`.
- One consumer coroutine sends it, and it never sends two at once:
  ```kotlin
  private val wantTerminal = MutableStateFlow(false)
  private var applied: String? = null   // reset to null on every (re)connect
  // loop in scope:
  while (true) {
      val want = if (wantTerminal.value) "on" else "off"
      if (want != applied) runCatching { call("ui.render", JSONObject().put("name", want)) }.onSuccess { applied = want }
      wantTerminal.first { (if (it) "on" else "off") != applied }   // also wakes on reconnect
  }
  ```
- Each send awaits its job, so a fast `on → off → on` (screen off and on, or leaving and entering Modo terminal) always ends in the last state.
- The core draws for 1–2 s at start, until the first `off` arrives.
- **Never forward touch input to the pty** from the native UI.

### 7.7 Queue reorder with swaps

- During the drag, reorder the local list only.
- On drop with `from != to`, send `|to − from|` adjacent `queue.move {index:i, to:i±1, if_revision}` calls in sequence. Each one passes the `playlist_revision` from the previous job's snapshot.
- On `conflict` or any failure: refetch.
- The play-next section does the same with `playnext.move`.
- Under shuffle, the handle hides in "Siguiente".

### 7.8 yt-dlp auto-update (the "no song plays" fix)

**Why not youtubedl-android's updater:**
- `updateYoutubeDL` reads the GitHub API with no timeout while holding the library's monitor.
- It deletes the yt-dlp directory before it copies the new file.
- The API has a rate limit of 60 requests an hour.

**`object Ytdlp`** does the update itself:
- **Target file:** `File(File(ctx.noBackupFilesDir, YoutubeDL.baseName), "${YoutubeDL.ytdlpDirName}/${YoutubeDL.ytdlpBin}")`, the same path `Core.env` puts in `CLIAMP_YTDLP`. The core runs yt-dlp fresh for each play, so a replaced file takes effect on the next play with no restart. `YoutubeDL.init` copies the bundled 2025.11.12 copy only when the file is missing, so the update survives app restarts and APK updates.
- **Prefs:** the phase 4 `Updates.prefs(ctx)`, with keys `ytdlp_tag` (null means never updated, which is the bundled 2025.11.12) and `ytdlp_checked`.
- **Status:** `status: StateFlow<Status>`, with `sealed interface Status { Idle; Updating; Updated(tag); UpToDate(tag); Failed(msg) }`, and `finishedAt` (elapsedRealtime of the last Updated or UpToDate).

**`suspend fun update(ctx, force: Boolean): Status`:**
- It is guarded by a `Mutex`. When busy, it waits for the running update and returns its result.
- Steps:
  1. If `!force` and `!Updates.due(ytdlp_checked, now)`, return the current status.
  2. Write `ytdlp_checked = now`, then set `Updating`.
  3. On IO, inside `withTimeout(120_000)`:
     1. The repository is `yt-dlp/yt-dlp-nightly-builds` when `Updates.prefs.getBoolean(Updates.NIGHTLY)`, else `yt-dlp/yt-dlp`.
     2. GET `https://github.com/<repo>/releases/latest` with `instanceFollowRedirects = false`, connect and read timeouts of 15 s, and `useCaches = false`. The tag is the `Location` header after `/tag/`.
     3. If the tag equals `ytdlp_tag` and the target exists, the result is `UpToDate(tag)`.
     4. Download `https://github.com/<repo>/releases/download/<tag>/SHA2-256SUMS` and take the hash on the line that ends in ` yt-dlp`.
     5. Download `…/releases/download/<tag>/yt-dlp` into `<target>.part`, in the same directory. Cap it at 64 MB, with `ensureActive()` in the copy loop. Check that it starts with `#!` and that `Updates.sha256(part) == hash`.
     6. `part.renameTo(target)`. The rename is atomic, so a running yt-dlp keeps its old inode. Then `ytdlp_tag = tag`, and the result is `Updated(tag)`.
     - A failure gives `Failed(Updates.message(ctx, e))`, and the `.part` file is deleted.

**Triggers:**

1. **Start.** In `CliampService.start()`, after `Core.prepare` and the core start: `Remote.scope.launch { Ytdlp.update(ctx, force = tag == null) }`. The core itself is never held back.
2. **Play gate.** `Remote.call` checks the play operations: `play`, `toggle`, `next`, `prev`, `queue.play`, `queue.replace`, `track.play`, `provider.load`, `provider.load_album`, `radio.start`, and `url.load` with `play`.
   - While `tag == null && status is Updating`, it first awaits `withTimeoutOrNull(45_000) { status.first { it !is Updating } }`.
   - The mini-player, or the splash when nothing played yet, shows `◌ Preparando yt-dlp…`.
   - Then it sends the request, whatever the outcome.
3. **Error watcher** (in Remote, on each applied snapshot). Let `e = snapshot.error` and `p = snapshot.error_path`. It acts only when `e != ""`, `p` is a YouTube URL (`youtube.com`, `youtu.be` or `music.youtube.com`), `p == snapshot.track.path` and `p !in retried`:
   - `status is Updating` → wait for it to end. If it ends Updated or UpToDate, retry.
   - Else, if Updated or UpToDate finished less than 60 s ago → retry now. This covers a play spawned during the swap.
   - Else, if `tag == null` → `update(force = true)`, at most once a minute. If it ends Updated or UpToDate, retry.
   - Else, if `error_kind == "ytdlp"` and `ytdlp_checked` is more than 1 h old → `update(force = true)`. If it ends Updated, retry.
   - Else: no automatic action; the card shows.

   "Retry" means: if `state.value.track.path == p`, add `p` to `retried` and call `queue.play {index: snapshot.index}`. `retried` lives for the process.
4. **Manual.** The card's `[ Actualizar yt-dlp ]`, and Ajustes, call `update(force = true)`. Then, when the playing track's error is still set, they retry once, even after UpToDate. The button is disabled while `Updating`.

**UI:**
- The Inicio banner: `⠹ Actualizando yt-dlp…`, then `✓ yt-dlp <tag> listo` (4 s), or `ERR: <msg>` with `[ Reintentar ]`.
- Ajustes shows `yt-dlp <ytdlp_tag ?: "incluida 2025.11.12">`.
- The state is indeterminate; there is no progress bar.

### 7.9 First run, permissions, sign-in

1. **Launch:** a fresh install shows the onboarding (`ui/Onboarding.kt`, FINAL-PLAN T9) while the core starts behind it: permissions (notifications, local music, battery), the YouTube Music sign-in or Omitir, and a skin, which previews live once the core is up and applies at `[Empezar]`. Only a fresh install (Python not unpacked yet) gets it, until `[Empezar]` (prefs `ui` `onboarding`); an update keeps the one-time POST_NOTIFICATIONS + READ_MEDIA_AUDIO request. The splash shows until `Remote.connected`.
2. **First run:** the forced yt-dlp update runs. If the user taps a song meanwhile, it waits behind the gate.
3. **Inicio cards:** sign-in (dismissible; the dismissal is stored in prefs `ui`); battery; app update.
   - The app-update banner replaces phase 4's daily `AlertDialog`. It runs `Updates.check` when `Updates.due`, on IO, once per launch.
   - `[ Instalar ]` opens SettingsActivity with `EXTRA_UPDATE` (M1–M4), or the Ajustes screen, which then downloads and installs (M5).
4. **Sign-in:** LoginActivity, or the cookie import. Both restart the core. `Remote` reconnects, resets the revision floor, clears the cache and refreshes `signedIn`.
5. **Sign-out:** `LoginActivity.signOut`.


### 7.10 Ajustes and phase 4

- `ui/Settings.kt` (M5) ports every SettingsActivity row and calls the same code:
  - `Updates.check`, `download`, `install`, `message`, `offline`, `prefs`, `NIGHTLY`;
  - `LoginActivity.importCookies`;
  - `ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS`;
  - quit.
- The "allow installs" round trip uses `LifecycleResumeEffect` in place of `onResume`.
- Reuse phase 4's strings.
- Delete `SettingsActivity.kt` and its manifest entry. `EXTRA_UPDATE` becomes `Nav.push(Settings(install = true))`.
- `DebugUpdateActivity` (in `src/debug`) stays.

### 7.11 CliampService and the MediaSession

- `onCreate` calls `Remote.start(this)`. `followState`/`onState` are replaced with collecting `Remote.state` on Main into `publish()`:
  ```kotlin
  .filterNotNull()
  .distinctUntilChanged { a, b ->
      a.track?.path == b.track?.path && a.track?.title == b.track?.title && a.state == b.state &&
      a.speed == b.speed && a.duration == b.duration && a.seekable == b.seekable &&
      abs(PlaybackClock.positionAt(a, now) - PlaybackClock.positionAt(b, now)) < 2
  }
  ```
  This avoids re-posting the notification and the 512 px art on every 5 s `state.get`.
- `PlaybackState.setState(state, position, speed = snapshot.speed, receivedAt)`.
- The art comes from `Art.software(url, 512)`.
- `op(...)` and `render(...)` go through Remote (`Remote.fire`, `Remote.terminalVisible`).
- `activity: MainActivity?` becomes `terminal: TerminalActivity?`.
- The notification content intent opens `MainActivity` with `EXTRA_OPEN_PLAYER`.
- `quit()` sets `Remote.quitting = true` before stopping the core.
- **Unchanged:**
  - audio focus, the noisy receiver and the wake lock;
  - the single SIGTERM;
  - `open(link)`;
  - the actions.

---

## 8. Dependencies

The dependency set is checked against AGP 8.13.2, Kotlin 2.4.21 and compileSdk 36.

**Gradle files:**
- `android/build.gradle.kts`: `id("org.jetbrains.kotlin.plugin.compose") version "2.4.21" apply false`. It must equal the Kotlin version.
- `app/build.gradle.kts`: `plugins { id("org.jetbrains.kotlin.plugin.compose") }` and `buildFeatures { compose = true }`.

**Implementation:**
- `platform("androidx.compose:compose-bom:2026.06.01")`: the last BOM on Compose UI/foundation 1.11.4 (compileSdk 36). 2026.08.00 and later need AGP 9.2.
- `androidx.compose.material3:material3`: 1.4.0, from the BOM.
- `androidx.activity:activity-compose:1.13.0`.
- `androidx.lifecycle:lifecycle-runtime-compose:2.10.0`.

**Kept:**
- `androidx.core:core-ktx:1.18.0`;
- `androidx.appcompat:appcompat:1.8.0` (TerminalActivity, LoginActivity, SettingsActivity until M5);
- `androidx.media:media:1.8.0`;
- `org.jetbrains.kotlinx:kotlinx-coroutines-android:1.11.0`;
- termux `terminal-view` and `terminal-emulator:v0.118.3`;
- youtubedl-android `library` and `ffmpeg:0.18.1`, for Python, ffmpeg and the bundled yt-dlp; its updater is no longer called.

**Tests:** `junit:junit:4.13.2` and `org.json:json:20260814`.

**Version pin comment:** "AGP 8.13.2 / compileSdk 36 ceiling: do not bump the BOM past 2026.06.01, activity past 1.13.x, or lifecycle past 2.10.x."

**Not used:** Coil, a navigation library, ViewModel, DI, kotlinx.serialization, material-icons. R8 stays off; Compose's baseline profiles come through profileinstaller.

**Assets, not Maven:**
- JetBrains Mono 2.304 Regular and Bold TTF, from `JetBrainsMono-2.304.zip` (`fonts/ttf/`), OFL 1.1, about 0.6 MB;
- Material Symbols Sharp 24 px vector XML, Apache 2.0.

---

## 9. Test plan

### 9.1 Go (the Go agent writes these; reviewers rerun them)

```
mise exec -- go test ./ui/model -run 'Native|IPC|Spectrum|Snapshot|Theme|Queue' -count=1
mise exec -- go test ./ipc ./external/ytmusic ./provider . -count=1
mise exec -- make check && mise exec -- make ci
```

Cases, at least:

| Item | Cases |
|---|---|
| G7 | each field reaches the snapshot and changes the fingerprint; the `playErrorKind` table, including `ErrNeedsAuth`; **a provider-pane load error leaves `error` empty**; a seek in flight reports the target and `seeking`, and its landing publishes |
| G9 | non-empty, changing bands with render paused: normal layout, provider screen, simplified mode, 50×14; no Suspend between calls |
| G5 | index 0, 3 and out of range; the reply has ≤ 200 tracks |
| G6 | success, empty input, stale revision → `conflict` |
| G22 | with and without shuffle |
| G19, G21 | basic cases |
| G17 | cover palette in the snapshot and the fingerprint; `themes` list |
| G20 | `playlist.create` id, `provider.favorite` and `playlist.bookmark` favorite |
| G1 | flags from the prefixes; `single` |
| G2 | kinds, hidden filter, LM dropped, empty shelves dropped |
| G3, G4, G10 | basic cases |
| G18 | `favorite` known and unknown |
| G11 | ±1 per kind; **0 without a track**; bad input |
| G12 | track seed, item seed, too few songs |
| G13 | basic case |
| G14 | each name, and bad values |
| G15 | no manifest → `unavailable`; the status shape |
| G16 | basic case |
| Routing | every new name is in `DefaultOperationRegistry` and reachable (not `unavailable`) |

### 9.2 Kotlin JVM (`cd android && ./gradlew testDebugUnitTest`)

| Test | Covers |
|---|---|
| `ModelsTest` | snapshot omitempty defaults (missing `volume` and `index`; `speed` 0 → 1.0; the `shuffle` pointer); the G7 fields; `Track.raw` round-trips exactly; parsers for shelves, genres, albums, artists, downloads, stats, ratings, themes and preview, with fixtures pasted from real core output |
| `SnapshotMergeTest` | the floor resets on reconnect, so a lower revision after a core restart is accepted; a retained event keeps the old position; `OTHER_JOB` keeps the position; `PLAYBACK_JOB` takes it; a track change takes it |
| `PlaybackClockTest` | playing, paused, buffering, seeking, speed 1.5, clamping |
| `RowNameTest` | `♥ `, `● `, `★ `, ` · local`, `single`, release `Title · Artist · Single` |
| `SearchSectionsTest` | grouping and the kind from the `albumID` prefix |
| `ArtUrlTest` | `=w400-h400` → `=w1080-h1080` keeping the suffix; mqdefault → hqdefault only above 320 |
| `PaletteTest` | hex parse; onAccent threshold 0.179; `surface` lerp; Default and missing values → winamp |
| `RenderSenderTest` | with a fake caller that delays: rapid on/off/on flips end in the last state, never two sends in flight, and a resend after a reconnect |
| `PagingTest` | whole-list next offset; foryou `offset + limit`; end on an empty page |
| `ErrorMapTest` | network-looking details; sign-in details; `error_kind` → card text and action |
| `YtdlpPolicyTest` | the error-watcher table of §7.8 (Updating, finished < 60 s, tag null, ytdlp kind > 1 h, nothing); one retry per path; the gate only while `tag == null && Updating`; the tag parsed from a `Location` header; the SUMS line parse |
| `GlyphTest` | loads `res/font/jetbrains_mono_regular.ttf` with `java.awt.Font.createFont` and asserts `canDisplay` for every glyph the code uses as text (one shared constant list) |

### 9.3 Emulator scenarios (emulator-5554: x86_64, Android 16, no Google account)

**What the emulator can and cannot test:**
- Testable: local files, search, public playback, the public home, genres, album, artist and playlist pages, radio, remixes, downloads, the queue, EQ, themes, visualizers, Modo terminal and the MediaSession.
- **Not testable** (they need sign-in): the library's YouTube rows, likes sync, follow and save, Para ti (needs library seeds), Novedades. There, assert that these elements are **hidden**.

**Build:**
```
make android-core ANDROID_ABI=x86_64
cd android && ./gradlew assembleDebug -PcliampAbi=x86_64        (flows)
cd android && ./gradlew assembleRelease -PcliampAbi=x86_64      (performance; sign with the debug key if no keystore)
adb install -r app/build/outputs/apk/debug/app-debug.apk
```

**Setup:**
```
adb shell pm grant io.github.xaviiromf.cliampx android.permission.READ_MEDIA_AUDIO
adb shell pm grant io.github.xaviiromf.cliampx android.permission.POST_NOTIFICATIONS
adb push test.mp3 /sdcard/Music/
adb shell settings put global animator_duration_scale 0      (and window and transition scales)
```

**Driver:** `android/scripts/ui-smoke.sh`, with these helpers:
- `node ID`: `adb exec-out uiautomator dump /dev/tty`, then grep `resource-id="ID"` and parse its bounds;
- `tap ID`;
- `swipe x1 y1 x2 y2 ms`;
- `shot NAME`: `adb exec-out screencap -p > $OUT/NAME.png`;
- `waitfor ID [s]`.

`uiautomator dump` needs an idle UI: dump with the player sheet closed or the visualizer on None. Screenshots work at any time.

**Test tags:**
- shell: `tab_home`, `tab_search`, `tab_library`, `mini_player`, `mini_toggle`, `status_line`, `banner_<kind>`;
- player: `player`, `player_close`, `player_more`, `art`, `vis_strip`, `seek`, `play_pause`, `next`, `prev`, `shuffle`, `repeat`, `fav`, `chip_lyrics`, `chip_queue`, `chip_eq`, `chip_sleep`, `error_card`, `error_action`;
- search: `search_field`, `search_clear`;
- lists: `row_<i>`, `row_more_<i>`, `tile_<shelf>_<i>`, `menu_<action>`;
- detail pages: `detail_play`, `detail_shuffle`, `detail_save`, `detail_follow`;
- settings: `settings`, `set_theme`, `theme_<name>`, `set_vis`, `vis_<Name>`, `set_eq`, `eq_band_<n>`, `eq_preset_<name>`, `set_sleep`, `sleep_<n>`, `set_terminal`, `set_signout`;
- library: `lib_chip_<lists|albums|artists|downloads>`, `lib_liked`, `lib_history`, `lib_local`, `lib_stats`, `lib_ratings`.

| # | Scenario | Steps | Pass |
|---|---|---|---|
| S1 | Cold start | `pm clear`, grant, launch | splash, then Inicio within 90 s; `banner_ytdlp` shows Actualizando, then listo; `run-as io.github.xaviiromf.cliampx cat shared_prefs/updates.xml` has `ytdlp_tag`; the target file starts with `#!` |
| S1b | Tap during the first update | `pm clear`, launch, search at once and `tap row_0` while the banner reads Actualizando | mini-player shows "Preparando yt-dlp…", then the song plays with no further action |
| S2 | Inicio signed out | screenshot | public home shelves and the sign-in card; Novedades and account actions hidden |
| S3 | Search | `tap tab_search` (keyboard must stay closed); `tap search_field`; `input text 'daft%spunk'`; `keyevent 66` | grouped results screenshot |
| S4 | Play a search song | `tap row_0` | `mini_player` within 20 s; status Sonando |
| S5 | Player gestures | swipe up on the mini-player; swipe left on `art`; screenshot; swipe down | the sheet opens, the title changes, the sheet closes |
| S6 | Seek on a YouTube song | drag `seek` from 10 % to 60 % | the time reads about 60 % at once and stays there (no jump back) after 3 s |
| S7 | Queue | `chip_queue`; swipe `row_1` left; drag a handle 2 rows | row removed; order changed; no conflict loop |
| S8 | Context menu | long-press a search song → `menu_play_next` | `[En cola: n+1]` in the player |
| S9 | Album from row 3 | open an album result → `tap row_2` | the mini-player title equals row 2's title; the queue starts at it |
| S10 | Artist | open an artist result | header, Populares, carousels; `detail_follow` hidden (signed out); Radio starts a queue |
| S11 | Genre | Buscar empty → a genre tile → `detail_play`; switch the sort chip | it plays; the list changes |
| S12 | Local file | Biblioteca → `lib_local` → `row_0` | plays test.mp3; local art or the placeholder |
| S13 | Themes | Ajustes → Tema → `theme_winamp`, then `theme_catppuccin-latte`, then `theme_cover` while playing | black/green UI; a light UI with dark status icons; colors that change on the next song |
| S14 | EQ | `chip_eq`; drag `eq_band_0` up 3 steps | preset becomes Personalizado; label +3 |
| S15 | Sleep | `chip_sleep` → `sleep_15` | `APAGADO hh:mm` tag and the `[⏲ …]` chip |
| S16 | Visualizer | player open and playing; `tap vis_strip` twice; two screenshots 200 ms apart | the mode changes; the bars differ between the shots |
| S17 | Modo terminal | Ajustes → `set_terminal`; back at `[Cola]`; then 5 fast round trips, plus `keyevent KEYCODE_SLEEP` / `KEYCODE_WAKEUP` while in Modo terminal | the TUI always draws when shown; the native visualizer moves again after returning |
| S18 | Back | Album → back → back → back | Buscar → Inicio → launcher on top (`dumpsys activity activities`); music keeps playing |
| S19 | Rotation | `settings put system accelerometer_rotation 0; settings put system user_rotation 1` with the player open | the §3.10 layout; nothing clipped |
| S20 | Offline | `svc wifi disable; svc data disable`; pull on Inicio | `ERR: Sin conexión a internet.` + Reintentar; Música local still plays |
| S21 | Core restart | Ajustes → `set_signout` (restarts the core) while a song is loaded | the `Reconectando` banner, then: the title, play/pause and position follow `state.get` again; tapping play works; `dumpsys media_session` state matches |
| S22 | Notification, lock screen, screen off | `cmd media_session dispatch pause` / `play`; seek from the notification; `keyevent KEYCODE_SLEEP`, a seek via `cmd media_session` if available, then `dumpsys media_session` | the UI follows within 1 s; after the seek the session position stays at the target with the screen off |
| S23 | Radio and remixes | long-press a song → `menu_radio`; then `menu_remixes` | a new queue of more than 2 songs; the Remixes page with section rules |
| S24 | Downloads | album → `detail_download`; Biblioteca → `lib_chip_downloads` | progress, then done; Reproducir descargas plays with network off |
| S25 | Ratings | long-press a song → `menu_not_interested`; Biblioteca → `lib_ratings` → Deshacer | the row disappears at once; Deshacer removes the rating |
| S26 | Stats | Biblioteca → `lib_stats`; tap each period | summary and chart render |

### 9.4 Performance (release build)

- **Frames:** run `adb shell dumpsys gfxinfo io.github.xaviiromf.cliampx reset`. Then scroll Inicio (5 swipes), scroll a carousel, scroll a 200-row playlist, and open and close the player 3 times with the visualizer on. Then run `dumpsys gfxinfo io.github.xaviiromf.cliampx`.
  - A55: < 5 %, 90th percentile ≤ 16 ms. The frame numbers are judged on the A55 only.
  - Emulator: not judged. Its SwiftShader renderer misses the frame budget for any app (the system Settings app also shows about 40 % janky frames there), so its numbers say nothing about this app.
  - Acceptance run on the A55 (120 Hz): 25 % janky frames, 90th percentile 18 ms, short of the target. A later pass found no cheap hot spot: the visualizer, the seek line and the mini-player progress already redraw without recomposing, the title marquees stop after 3 runs and the art decodes off the main thread. The next step is a system trace on the A55 of the player open and close.
- **CPU:** `adb shell top -b -n 3 -d 10 -o PID,%CPU,RES,ARGS | grep -E 'cliampx|libcliamp'`.

  | Situation | App | Core |
  |---|---|---|
  | screen off, playing | ≈ 0–1 % | ≤ 5 % |
  | foreground, player with visualizer (A55) | ≤ 15 % | ≤ 12 % |
  | foreground, Inicio idle | ≈ 0 % | at the screen-off level |

- **Memory:** `dumpsys meminfo io.github.xaviiromf.cliampx`. TOTAL PSS < 250 MB after browsing 10 screens.
- **Spectrum stops:** close the player and confirm the core CPU drops back to the Inicio level.

### 9.5 Phone checklist (the user's Galaxy A55, Android 16, signed in, release build)

1. **Fresh install:** the yt-dlp banner shows; a song tapped right away plays as soon as the update ends (within 45 s); later songs start within 10 s.
2. **Inicio:** personal shelves, Para ti albums and artists, Novedades with `●`. Pull to refresh changes the Para ti order.
3. **Biblioteca:**
   - Listas, the liked-songs count, Álbumes (♥ first), Artistas (♥ = followed);
   - a local list can be created, renamed and deleted;
   - a song added to a YouTube playlist appears on music.youtube.com.
4. **♥ and follow:**
   - ♥ a song, and it shows in the YouTube Music likes;
   - open an artist you **already follow** from Buscar: the button reads "Siguiendo";
   - follow and unfollow; save an album.
5. **Ratings:** Me interesa / No me interesa: the row hides; Tus valoraciones → Deshacer brings it back.
6. **Discovery:** radio from a song, an album, an artist and a genre; Remixes y versiones lists the groups.
7. **Offline:** download an album, turn on airplane mode, and Descargas → Reproducir descargas plays it.
8. **Lock screen and notification:** play, pause, next, seek. The position stays right, also after a seek with the screen off.
9. **Audio routing:** unplugging the headphones, or disconnecting Bluetooth, pauses.
10. **Long playback:** 30 min with the screen off and no cuts, after the battery card is done.
11. **Cover theme:** it recolors per song with a smooth blend.
12. **Themes:** every theme is legible; the light themes have dark status icons.
13. **Accessibility sizes:** at maximum font size and display size, with 3-button navigation, the player controls all fit and nothing is clipped.
14. **One UI gesture navigation:** predictive back works; edge swipes do not skip songs.
15. **Modo terminal:** the round trip works, also with the screen turned off inside it.
16. **Account:** Cerrar sesión, then Iniciar sesión again; the library comes back.
17. **Settings that survive Salir and reopen:** theme, visualizer, EQ preset and bands, speed, normalization, crossfade, auto EQ, auto radio, mono, shuffle and repeat. The sleep timer does not survive. Auto radio continues at the end of the queue.
18. **TalkBack:** every icon button reads a Spanish label.

---

## 10. Work split and ownership

**Ownership (no shared files):**
- **Go agent:** everything outside `android/`: `ipc/`, `ui/model/`, `provider/`, `external/ytmusic/`, `main.go`, `docs/remote-control.md`, and `ui/model/i18n_es.go` only if a new `L()` text is unavoidable.
- **Android agent:** everything under `android/`, including `android/README.md` (the file table, the new UI, the smoke script) and `android/PLAN.md` (mark the §1 UI row superseded and link this spec).
- The Android agent builds `libcliamp.so` itself (`make android-core`, x86_64 and arm64-v8a) at the start of each milestone, from the `android` branch with the latest Go batch merged. It never edits Go files.

**Phase 4 is already merged** (02e757cb). The Android agent ports its pieces as follows:
- `MainActivity.offerUpdate` → the Inicio banner (M1);
- the SettingsActivity rows → Compose Ajustes (M5);
- the "Actualizar yt-dlp" button → `Ytdlp.update(force = true)` (M1).

**Go agent:** one bundled commit series per batch. Run `make check` per commit and `make ci` per batch, with the docs in the same change. After each batch, report the new operation names, so the Android agent can check `capabilities`.
1. **Batch A:** G7, G9, G5, G6, G22, G19, G21, G17, G20.
2. **Batch B:** G1, G2, G3, G4, G10, G18.
3. **Batch C:** G11, G12, G13, G14, G15, G16.

**Android agent milestones.** Each one builds, passes `testDebugUnitTest` and `lint`, and runs its scenarios with screenshots.

| Milestone | Scope | Needs (Go) | Scenarios |
|---|---|---|---|
| **M1 Foundation and the playback fix** | Compose Gradle setup and pins; fonts and icons; `Models.kt` and its tests; `Ipc.kt` additions; `Remote.kt` (merge, gate, render sender, error watcher); `Ytdlp.kt` and its triggers; the Core init guard; the Service on Remote (MediaSession speed and seek fix, distinct publish); today's MainActivity renamed to TerminalActivity; the new Compose MainActivity with theme, Nav, App shell, tab bar, splash, banners (yt-dlp, update, reconnecting), status line, a basic mini-player, and **Buscar with song results that play**; SettingsActivity gets "Modo terminal" | none. Search play uses `queue.replace` when present, else `track.play` — `shortcut: the only fallback; remove once Batch A is merged` | S1, S1b, S3, S4, S17, S21, S22 |
| **M2 Player** | Full mini-player; the full player (art views, swipe, seek, transport, ♥, chips, mode tags, error card); the Bars visualizer and the spectrum flow; lyrics; the Queue page (swipe, drag); the Eq page; the sleep and speed sheets | Batch A | S5–S8, S14–S16 |
| **M3 Browse** | Inicio (all sections, pull to refresh, cards), Buscar (history, genre grids, placeholders), Album, Artist, Playlist, Genre, Para ti, Remixes; the item action sheet and add-to-playlist; `Art.kt` | Batch B (G12 and G13 rows hide until C) | S2, S9–S11, S18, S20, S23 |
| **M4 Library** | Biblioteca chips and pinned rows; local playlist create, rename, delete and remove; Descargas; Tus valoraciones; Estadísticas; Guardar cola como lista | Batch C | S12, S24–S26 |
| **M5 Settings** | Complete Ajustes (§2.5): the theme picker (live), the visualizer picker, normalization, crossfade, the toggles, account sign-in and sign-out, music folder, battery, phase 4 updates, yt-dlp, Modo terminal, Salir. Delete SettingsActivity | Batch A + C | S13 |
| **M6 Polish** | The ClassicPeak and Mirror renderers; the landscape player; the cover-theme animation; reduced motion; the empty and error state sweep; TalkBack labels; the §9.4 performance pass; the complete `ui-smoke.sh`; README and PLAN updates; then the §9.5 checklist handed to the user | — | S19, all of §9.4 |

**Integration rule:** the UI hides every element whose operation is not yet in `capabilities`. There are no other fallbacks.

**Deferred by the user (M6):** the landscape player (§3.10) and the TalkBack labels (§9.5 item 18). The ClassicPeak and Mirror renderers are done, with 32 more native modes (§0). MainActivity is locked to portrait, and Modo terminal still turns; S19 checks both.

**Performance acceptance (M6):** the §9.4 frame numbers are judged on the A55 only; the emulator cannot meet any frame budget (§9.4). The A55 run (25 % janky, 90th percentile 18 ms) stays open until a trace there names the hot spot.

---

## 11. Open risks

See `open_risks`. The top three:
1. The yt-dlp download path: GitHub redirects, rate limits and the SUMS file, plus Python compatibility of new yt-dlp releases.
2. The error-kind classification of the real failure text.
3. Compose smoothness on the A55 with a 30 fps visualizer.
4. The items the user deferred in §10 stay open: landscape, TalkBack.

**Not covered:** nothing here was built or run.
- The JBM glyph coverage was checked against the installed JetBrainsMono **Nerd Font**, a superset; `GlyphTest` confirms it against the static TTF.
- `sheetGesturesEnabled` on `ModalBottomSheet` in material3 1.4.0 is assumed.

---

## 12. Orchestrator addenda (binding; they override the sections above where they differ)

### 12.1 Sharp artwork (user report: "las carátulas se ven muy pixeladas")

The core stores YouTube Music song art as `i.ytimg.com/vi/<id>/hqdefault.jpg?sqp=…`
(480x360, a 16:9 frame letterboxed into 4:3). A centered square crop of it
is about 270 px, which the full player stretches to 1080 px. That is the
pixelation the user sees. Rules:

1. **Android, `Art.kt` URL rewrite** (pure, unit-tested, `px` = target pixels):
   - `googleusercontent.com` and `ggpht.com`: replace a trailing `=w<n>-h<n>…`
     or `=s<n>…` size spec with `=w<px>-h<px>-l90-rj`, `px` capped at 1200.
   - `i.ytimg.com/vi/<id>/<any>.jpg` (and `vi_webp`): drop the query. For
     `px <= 320` use `mqdefault.jpg`. Otherwise try `maxresdefault.jpg`
     (1280x720), then `sddefault.jpg`, then `hqdefault.jpg`: a 404, or the
     120x90 grey placeholder YouTube returns for a missing maxres, moves to
     the next candidate. Cache the winning variant per video ID.
   - Crop: a 16:9 image (maxres, mqdefault) crops to its central square. A
     4:3 ytimg image (sd, hq) first drops the letterbox bars (the central
     16:9 band), then crops the central square.
   - Decode at the target size of the view (full player = screen width in
     px), never upscale a smaller decode in the view.
2. **MediaSession / notification / lock screen** use the same rewrite at
   512 px.
3. **Go (core):** where `external/ytmusic` picks a thumbnail for a track,
   album, playlist or artist, prefer a square `googleusercontent` album art
   when the InnerTube response has one, else the largest thumbnail. Keep the
   URL as returned (the app rewrites the size). Table-driven test.
4. **Verify** on the user's phone with a screenshot of the full player for a
   YouTube Music song: the art must be sharp at full width (compare a crop
   of the screenshot; no visible blocks).

### 12.2 yt-dlp speed on the phone

On the A55, an extraction with yt-dlp 2026.08.19 + QuickJS took 16 s (11.5 s
CPU in the JS solver). yt-dlp caches the solved player under its cache
directory. (Correction: 2026.08.19 caches only the signature functions; its
cache of the preprocessed player is off, `_ENABLE_PREPROCESSED_PLAYER_CACHE
= False` in `jsc/_builtin/ejs.py`. The app turns it on with a yt-dlp plugin,
see `android/README.md`.) Add `XDG_CACHE_HOME=<cacheDir>/yt-dlp-cache` (or
`--cache-dir`) to the yt-dlp environment so the cache persists, then
measure the first and the second play of different songs on the phone
(the second should skip the solver work). Keep the core's next-track info
prefetch (`player/ytdl_info.go`) working on Android so a skip starts fast.
Report the measured times.

### 12.3 Devices

- `emulator-5554`: x86_64 Android 16 emulator, no Google account. Use the
  x86_64 core and `-PcliampAbi=x86_64` APKs. Arm64 Go binaries segfault
  under its ARM translation.
- `<phone-serial>`: the user's Samsung A55 (arm64, Android 16, One UI, has a
  a second Android user). It has the debug-signed app installed and signed
  in to YouTube Music with imported cookies. Only the stages that the
  orchestrator names may use it. Install with
  `adb -s <phone-serial> install --user 0 -r <arm64 debug apk>` (the `-r`
  update keeps the sign-in; never uninstall, never clear data, never touch
  other apps or settings, restore rotation and any setting you change).

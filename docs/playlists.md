# Playlists

cliamp-x supports local **TOML playlists** that you manage in the TUI or CLI. It also loads **M3U/M3U8/PLS playlists** from files and URLs.

## Loading a Saved Playlist

Select the **Local** source to manage playlists. It shows the playlist
directly with its full add, remove, rename and edit actions — there is no
separate read-only Local list. `Esc` there leaves the playlist for the live
queue; `Esc` from the queue returns to the playlist.

In the playlist (`p`):

- `Enter` opens a playlist, and `p` on its track screen plays it from the top.
  Both **replace** what is loaded, which clears the queue.
- `A` **appends** instead. On the list screen it adds the whole playlist to the
  end of the current one; on a track screen it adds the marked tracks, or the
  highlighted one when nothing is marked. Playback and the queue are left
  alone.

`A` starts playback only when the playlist was empty to begin with.

## Saving the Current List

Press `p` for the playlist, then `w` on the list screen to save
everything currently loaded. The picker offers your saved playlists plus a
**New playlist** row.

- `Enter` adds the tracks to the end of the chosen playlist, skipping any it
  already holds.
- `p` adds them to the start instead. A track already in the playlist moves to
  the front rather than being skipped.

From the main view, `w` writes only the highlighted track. In the playlist's
track screen, `Space` marks tracks and `w` copies the marked ones.

## M3U and PLS Playlists

Load local or remote `.m3u`, `.m3u8`, and `.pls` files:

```sh
cliamp-x ~/radio-stations.m3u
cliamp-x http://radio.example.com/streams.m3u
cliamp-x ~/music.m3u https://example.com/live.m3u   # mix local + remote
cliamp-x ~/radio.pls
```

### EXTINF Metadata

cliamp-x reads titles and durations from `#EXTINF` lines:

```m3u
#EXTM3U
#EXTINF:180,Radio Station 1
http://station-1.com/stream
#EXTINF:-1,Radio Station 2
http://station-2.com/stream/hd
```

Entries without `#EXTINF` also work. cliamp-x uses the file name or URL as the title.

### Relative Paths

cliamp-x resolves paths in a local M3U file from the M3U file directory:

```m3u
#EXTINF:240,My Song
../Music/song.mp3
#EXTINF:-1,Live Stream
http://example.com/live
```

If `radio.m3u` is in `~/playlists/`, cliamp-x resolves `../Music/song.mp3` to `~/Music/song.mp3`.

On Android, **Biblioteca → Importar M3U** and **Exportar M3U** in a playlist's menu move lists to and from these files; see [Android](android.md).

### Edge Cases Handled

- UTF-8 BOM (common in Windows-created files)
- `\r\n` line endings
- Missing `#EXTM3U` header
- Mixed local and remote entries in the same file
- Other `#` directives (silently skipped)

---

## Local TOML Playlists

Create and manage playlists as `.toml` files in `~/.config/cliamp-x/playlists/`. The folder follows cliamp-x config directory resolution: `CLIAMP_CONFIG_DIR`, then `XDG_CONFIG_HOME/cliamp-x`, then `HOME/.config/cliamp-x`.

### File Format

Each playlist has one `.toml` file. cliamp-x automatically reads every file in the `playlists/` folder with a `.toml` extension, case-insensitively. The CLI or a user can create these files. cliamp-x skips files that it cannot parse. The file name without its extension is the playlist name:

```text
~/.config/cliamp-x/playlists/
├── radio-stations.toml   → playlist "radio-stations"
├── music.toml            → playlist "music"
└── gym.toml              → playlist "gym"
```

Use one file for each collection. You can use files that you create and files that the CLI creates. cliamp-x keeps empty playlists on disk so they remain visible in the TUI and CLI.

```toml
# ~/.config/cliamp-x/playlists/radio-stations.toml

[[track]]
path = "http://station-1.com/stream"
title = "Radio Station 1"
realtime = true

[[track]]
path = "http://station-2.com/stream/hd"
title = "Radio Station 2"
artist = "Radio Network"

[[track]]
path = "/home/user/Music/song.mp3"
title = "My Song"
artist = "My Artist"
```

Each `[[track]]` section supports these keys:

| Key | Required | Description |
|-----|----------|-------------|
| `path` | Yes | File path or HTTP URL |
| `title` | Yes | Title to display |
| `artist` | No | Artist name |
| `album` | No | Album name |
| `genre` | No | Genre name |
| `year` | No | Release year |
| `track_number` | No | Track number |
| `duration_secs` | No | Duration in seconds |
| `stream` | No | Treat a provider URI as a network stream. cliamp-x writes it for provider tracks. HTTP and HTTPS paths are always streams. |
| `realtime` | No | Treat an HTTP URL as live radio. Reconnect after pause or disconnect. |
| `restricted` | No | Mark a track that its provider may refuse to play. cliamp-x shows an `[E]` suffix and still tries to play it. |
| `feed` | No | Treat the URL as an RSS or podcast feed. cliamp-x resolves it before playback. |
| `provider_meta.<key>` | No | Provider data, such as the YouTube Music video ID. cliamp-x uses it to recognize provider tracks. The `<key>` holds only letters, digits, `.`, `_` and `-`. cliamp-x does not save a key with other characters. |
| `embedded_lyrics` | No | Lyrics from local file tags |
| `album_art_url` | No | Album art URL: a cached file URL for embedded art, or the cover URL of a provider track. The media controls show it. |
| `bookmark` | No | Legacy bookmark flag. cliamp-x reads it one time and copies the track into favorites. See [Favorites](#favorites). |

cliamp-x treats HTTP/HTTPS paths as streams. Set `realtime = true` for live radio.
cliamp-x keeps this flag when it saves the playlist.

`favorites.toml` and `history.toml` use the same track keys, except
`embedded_lyrics` and `bookmark`.

### Directory Sources (`[[dir]]`)

Instead of listing each file, reference a directory in a playlist. cliamp-x scans
the directory for audio files each time it loads the playlist. New files appear
automatically. Removed files no longer appear:

```toml
# ~/.config/cliamp-x/playlists/music.toml

[[dir]]
path = "~/Music"

[[track]]
path = "https://radio.example.com/stream"
title = "My Radio"
```

Each `[[dir]]` section supports these keys:

| Key | Required | Description |
|-----|----------|-------------|
| `path` | Yes | Directory path. cliamp-x expands `~` and environment variables. |
| `recursive` | No | Also scan subdirectories (default `true`) |

cliamp-x returns directory tracks in document order and sorts each directory by
path. An explicit `[[track]]` with the same path overrides a directory scan. Use
this to save custom metadata for a file. A favorite needs no explicit entry,
because favorites.toml stores it. Unreadable or missing directories add no
tracks. Files and subdirectories whose name starts with `.` are skipped, so
sidecars such as macOS `._` AppleDouble files never become tracks.

Use `--dir` to create or extend these playlists on the CLI:

```sh
cliamp-x playlist create "Music" --dir ~/Music
cliamp-x playlist add "Music" --dir ~/Downloads/live
cliamp-x playlist dirs "Music"               # list referenced directories
```

Do not combine `--dir` with `--ssh`. The CLI and TUI refuse to remove a
directory track because the directory source owns it. Delete the file from the
directory or edit the `[[dir]]` section instead.

### Adding Files and Directories

A playlist file can contain any number of `[[track]]` and `[[dir]]` sections.
Mix them as needed. This `mix.toml` has two directory sources, two absolute file
paths, and a stream URL:

```toml
# ~/.config/cliamp-x/playlists/mix.toml

[[dir]]
path = "~/Music"

[[track]]
path = "/home/user/Downloads/live-set.mp3"
title = "Live Set"

[[dir]]
path = "~/Downloads/podcasts"
recursive = false

[[track]]
path = "https://radio.example.com/stream"
title = "My Radio"
```

You can also use many files. Each `.toml` file in the `playlists/` folder is a
separate playlist:

```toml
# ~/.config/cliamp-x/playlists/gym.toml

[[dir]]
path = "~/Music/gym"

[[track]]
path = "/home/user/Documents/warmup.mp3"
title = "Warmup"
```

```toml
# ~/.config/cliamp-x/playlists/chill.toml

[[track]]
path = "/home/user/Downloads/chill-lofi.mp3"
title = "Lofi"
```

Build the same playlists on the command line without editing files. Positional
paths become `[[track]]` entries. `--dir` stores a directory reference that
cliamp-x scans on every load:

```sh
cliamp-x playlist create "mix" ~/Downloads/live-set.mp3 --dir ~/Music --dir ~/Downloads/podcasts
cliamp-x playlist create "gym" ~/Documents/warmup.mp3 --dir ~/Music/gym
cliamp-x playlist create "chill" ~/Downloads/chill-lofi.mp3
```

Add more files and directories to an existing playlist:

```sh
cliamp-x playlist add "mix" another-track.mp3 --dir ~/Downloads/live
```

`recursive = false` has no CLI flag. Set it in the playlist `[[dir]]` section.

### Podcast / RSS Feed Playlists

Save podcast RSS feed URLs in a playlist. Set `feed = true` to mark a track as a
feed. When you play it, cliamp-x resolves the feed into individual episodes. It
does not stream the feed directly.

```toml
# ~/.config/cliamp-x/playlists/podcasts.toml

[[track]]
path = "https://feeds.simplecast.com/54nAGcIl"
title = "The Daily"
feed = true

[[track]]
path = "https://lexfridman.com/feed/podcast/"
title = "Lex Fridman Podcast"
feed = true
```

Each `[[track]]` with `feed = true` supports these keys:

| Key | Required | Description |
|-----|----------|-------------|
| `path` | Yes | RSS/Atom feed URL |
| `title` | Yes | Display name for the feed |
| `feed` | Yes | Must be `true` to enable feed resolution |

When you select a feed entry, cliamp-x fetches the RSS feed. It extracts episodes
with audio enclosures and loads them into the playlist. It keeps episode titles
and durations from `<itunes:duration>`.

cliamp-x also detects URLs with `.xml`, `.rss`, or `.atom` extensions as feeds. You
do not need `feed = true` for these URLs.

### Browsing and Loading Playlists

Run `cliamp-x` without arguments to open the provider browser of your default provider, YouTube Music. See [youtube-music.md](youtube-music.md).

To browse local playlists, press `Esc` or `b` during playback to open the
provider browser. Use `Up`/`Down` or `j`/`k` to navigate. Press `Enter` to load
a playlist. Its tracks replace the current playlist and start playback. Press
`Esc` or `b` to return to the now-playing playlist without loading it again.

The YouTube Music browser lists your local playlists in the same menus, with
provider labels such as `[Local Playlists] favorites`.

You can start with CLI files and browse playlists later:

```sh
cliamp-x song.mp3                    # starts playing, Esc opens browser
```

### Managing Playlists

The **Local** source opens the playlist in place. Press `p` in any
other view to open it as an overlay:

1. **Browse**: View all playlists and their track counts.
2. **Filter**: Press `/` to filter the list as you type. This works on the playlists and tracks screens. `Esc` clears the filter.
3. **Open**: Press `Enter` or `→` to view playlist tracks.
4. **Create playlist**: Press `a`, enter a name, and press `Enter`. The file browser opens at `~` for the new playlist. Use `Space` to select folders or files. Folders become live `[[dir]]` sources. Press `Enter` to confirm or `Esc` to finish.
5. **Rename playlist**: Press `r` on the list screen.
6. **Delete playlist**: Press `d`, then `y` to confirm.
7. **Mark tracks**: Open a playlist. Press `Space` to mark a track and advance, or unmark it and stay on the row. Press `a` to mark or unmark all visible tracks.
8. **Move tracks**: Press `[` or `]`. cliamp-x saves the playlist immediately.
9. **Sort tracks**: Press `s` to cycle `track`, `title`, `artist`, `album`, `artist+album`, and `path`.
10. **Remove tracks**: Press `d` to remove marked tracks, or the selected track when none are marked.
11. **Undo manager edits**: Press `u` after delete, remove, move, or sort.
12. **Write tracks elsewhere**: Press `w` to copy marked or selected tracks to another playlist. cliamp-x skips duplicate paths.
13. **Add files**: Press `o` inside a playlist to browse and add files to it.
14. **Play this**: Press `Enter` in the track list to start at the selected track. The rest of the playlist follows.
15. **Play all**: Press `p` to start from the top, regardless of cursor position.
16. **New playlist**: Select "+ New Playlist...", enter a name, and press `Enter`. The file browser opens so you can add tracks immediately. If a `/` filter is active, cliamp-x fills the new playlist name with the filter text.

Tracks with an `album` field are grouped by album with separator headers in the
playlist and the main player view. Album grouping is hidden while a
filter is active.

cliamp-x creates `~/.config/cliamp-x/playlists/` on first use. Removing the last
track leaves an empty playlist file. Use `d` in the playlist list or `cliamp-x
playlist delete` to delete the playlist.

### Writing to Playlists

Press `w` on a track in the main playlist to open the local playlist picker.
Select an existing playlist or `+ New Playlist...`. cliamp-x skips and reports
exact duplicate paths.

In the file browser, use `Space` to select files. Use `a` to select all visible
audio files. Press `w` to write the selection to a playlist instead of loading
it into the current queue.

### Command Line Management

Manage local TOML playlists without opening the TUI:

```sh
cliamp-x playlist list
cliamp-x playlist create "Name"                    # create an empty playlist
cliamp-x playlist create "Name" file1 dir/ ...     # create from files/folders
cliamp-x playlist create "Name" --dir ~/Music      # reference a directory dynamically
cliamp-x playlist add "Name" file1 dir/ ...        # append, skipping duplicate paths
cliamp-x playlist add "Name" --dir ~/Music         # add another directory source
cliamp-x playlist dirs "Name"                      # list directory sources
cliamp-x playlist rename "Old" "New"
cliamp-x playlist dedupe "Name"
cliamp-x playlist sort "Name" --by artist+album
cliamp-x playlist doctor                           # report missing local files in all playlists
cliamp-x playlist doctor "Name" --fix              # prune missing local files
cliamp-x playlist export "Name" --format m3u -o mix.m3u
cliamp-x playlist import mix.pls --name "Imported"
cliamp-x playlist show "Name" --json
cliamp-x playlist remove "Name" --index 3
cliamp-x playlist favorite "Name" --index 3       # toggle the favorite of a track
cliamp-x playlist favorites                        # list all favorite tracks
cliamp-x playlist enrich "Name"                    # backfill duration/album metadata
cliamp-x playlist delete "Name"
```

Use `track`, `title`, `artist`, `album`, `artist+album`, or `path` as sort keys.

`playlist doctor --fix` prunes only playlist files. It reports a missing favorite and keeps it. To remove a favorite, run `cliamp-x playlist favorite Favorites --index N`.

New playlist names reject path separators and non-portable file name characters.
cliamp-x can still read and write existing playlist files with older Unix-only
names.

### Creating Playlists Manually

Create the directory, then add a `.toml` file:

```sh
mkdir -p ~/.config/cliamp-x/playlists
```

```toml
# ~/.config/cliamp-x/playlists/favorites.toml

[[track]]
path = "/home/user/Music/song.mp3"
title = "Great Song"
artist = "Good Artist"

[[track]]
path = "https://radio.example.com/stream"
title = "My Radio"
```

### Controls

**Playlist browser (provider view):**

| Key | Action |
|-----|--------|
| `Up` `Down` / `j` `k` | Navigate playlists |
| `Enter` | Load the selected playlist |
| `Tab` | Return to playback controls, starting at Source when visible ([navigation](keybindings.md#navigation)) |
| `Esc` `b` | Open browser (from playlist view) |

**Playlist (`p` key):**

| Key | Action |
|-----|--------|
| `p` / `Esc` | Open or close the playlist. `Esc` on the tracks screen goes back. |
| `Up` `Down` / `j` `k` | Navigate |
| `/` | Filter playlists or tracks; `Esc` clears |
| `Enter` / `→` | List screen: open a playlist. Tracks screen: play the **selected** track. |
| `p` | Play all tracks from the top (tracks screen) |
| `a` | List: create a playlist. After naming it, the file browser opens at `~`. Use `Space` to select folders or files and `Esc` to finish. Tracks: mark or unmark all visible tracks. |
| `r` | Rename the highlighted playlist (list screen; `Recently Played` cannot be renamed) |
| `Space` | Mark/unmark track and advance (tracks screen) |
| `s` | Cycle supported sort keys (tracks screen) |
| `w` | Write marked or selected tracks to another playlist. On the list screen, write the current queue. |
| `o` | Add files to the open playlist (tracks screen) |
| `D` | List: open the file browser to add `[[dir]]` sources to the selected playlist. Tracks: open the directory-sources screen for the open playlist. |
| `[` `]` | Move track up/down and save (tracks screen) |
| `d` | Delete a playlist after confirmation. `Recently Played` cannot be deleted. In tracks, remove marked tracks or the selected track if none are marked. |
| `u` | Undo the last playlist edit |
| `←` / `Backspace` | Go back from tracks screen to list |

The playlist list marks playlists with `[[dir]]` sources with a `· N dir(s)`
indicator next to the track count.

**Directory sources screen (tracks screen, `D`):**

| Key | Action |
|-----|--------|
| `Up` `Down` / `j` `k` | Navigate directory sources |
| `a` | Open the file browser to add a directory as a `[[dir]]` source |
| `d` then `y` | Remove the selected source. Confirm with `y`; any other key cancels. |
| `r` | Toggle `recursive` on the selected source. cliamp-x scans it again immediately. |
| `←` / `Backspace` / `Esc` | Back to the tracks screen |

In the file browser, press `D` to add selected directories as live `[[dir]]`
sources instead of expanding them into explicit tracks. If none are selected,
cliamp-x adds the selected directory or the directory that you are browsing. This
browser mode starts with `a` above, `o` from the tracks screen, or `D` from the
list screen. cliamp-x skips and reports directories that are already referenced.

## Favorites

Press `f` on a track to toggle its favorite state. The key works in the
queue, the playlist, a provider browser track list, and
search results. cliamp-x collects favorite tracks in the virtual **"Favorites"**
playlist. This playlist always appears at the top of the playlist list, even
when empty and regardless of the source playlist.

Favorites apply across playlists. A track that you favorite in "gym" appears in
"Favorites", and the reverse is also true. `~/.config/cliamp-x/favorites.toml`
stores the "Favorites" playlist. Like "Recently Played", it is a virtual
playlist that you cannot rename, delete, or change in the playlist. Use
`f` again to remove a favorite.

Favorite tracks show a small red `♥` marker in the track list. The marker
column is always reserved, so a toggle does not move the titles.


### Bookmarks became favorites

Older versions had a separate per-playlist bookmark on `f` with a `★` marker,
and the favorite was on `n`. Now `f` is the only favorite key, and `n` has no
action.

The first time cliamp-x starts after the update, it copies each bookmarked track
into favorites. It does this one time. It writes the favorites first, then
creates `~/.config/cliamp-x/bookmarks_migrated`. Delete that file to copy the
bookmarks again. cliamp-x does not rewrite your playlist files. Files that
contain `bookmark = true` still load.

`cliamp-x playlist bookmark` and `cliamp-x playlist bookmarks` still work. They
are aliases of `cliamp-x playlist favorite` and `cliamp-x playlist favorites`.
These CLI commands change only the local favorites:

```sh
cliamp-x playlist favorite "gym" --index 3
cliamp-x playlist bookmarks
```

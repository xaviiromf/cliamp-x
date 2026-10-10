# Keybindings

Press `Ctrl+K` in any mode, or `?` in the player, to view keybindings. The
keymap first shows commands for the active screen. It then shows player and
library commands.

To run a command, select it and press `Enter`. The command acts as if you
pressed its key on the screen that opened the keymap. Press `/` to filter the
list first. The keymap cannot run key pairs such as `Left` `Right`, the `N`
then `j` seek, or a player command while another screen is open. For these
entries, a status message tells you which key to press.

## Playback

| Key | Action |
|---|---|
| `Space` | Play / Pause |
| `s` | Stop |
| `>` `.` | Next track |
| `<` `,` | Previous track |
| `Left` `Right` | Seek -/+5s |
| `Shift+Left` `Shift+Right` | Seek -/+30s (configurable) |
| `N` then `j` | Seek to N * 10% of the track (for example, `7j` jumps to 70%, `0j` to the start) |
| `+` `-` | Volume up/down |
| `]` `[` | Change speed by 0.25x |
| `m` | Toggle mono |
| `n` | Volume normalization: off, by song, by album (saved in `normalize`) |
| `Ctrl+J` | Jump to time |

## Navigation

| Key | Action |
|---|---|
| `Tab` | Cycle visible controls: Playlist / Source / Volume / EQ / Shuffle / Repeat / Speed / Playlist |
| `Shift+Tab` | Cycle the same controls in reverse |
| `j` `k` / `Up` `Down` | Move playlist cursor (wraps); see focused settings below for control actions |
| `PageUp` `PageDown` / `Ctrl+U` `Ctrl+D` | Scroll playlist/file browser by page (outside text input) |
| `Home` `End` / `g` `G` | Go to top/end of playlist/file browser |
| `Shift+Up` `Shift+Down` | Move track up/down in playlist/queue. Turn off shuffle to move a playlist track. |
| `h` `l` | Adjust the focused setting (EQ: select band) |
| `Enter` | Play selected track |
| `/` | Filter the playlist (navigate results with `↑` `↓` / `Ctrl+N` `Ctrl+P`; `Ctrl+U` clears the query; `Tab` toggles the queue for the selected result) |
| `Ctrl+X` | Expand/collapse playlist |
| `Ctrl+Z` | Undo the last playlist or queue change |
| `o` | Open file browser |
| `b` `Esc` | Back to provider |

In full and compact playback layouts, the first `Tab` from the playlist focuses
Source (`SRC`); `Shift+Tab` starts at Speed when it is visible. Source is skipped
when only one provider is available. Closing Settings skips EQ and Speed. A
short sidebar can omit Shuffle and Repeat together, removing both Tab stops.
Metadata is read-only and never a separate Tab stop.

In the minimal (`40x10`) and simplified layouts, `Tab` and `Shift+Tab` keep
playback focus on the playlist, even though simplified mode hides the list.
`Esc` still opens the separate provider-list view. In the minimal layout, the
hint bar then shows the provider keys. Below `40x10`, only a resize message is
shown.

### Focused Settings

| Control | Keys |
|---|---|
| Source | `Left` `Right` / `h` `l` choose a provider; `Enter` opens it |
| Volume | `Right` `Up` / `l` `k` raise volume by 1 dB; `Left` `Down` / `h` `j` lower it |
| EQ | `Left` `Right` / `h` `l` select a band; `Up` `Down` / `k` `j` adjust its gain; `e` cycles presets |
| Shuffle | `Enter`, any arrow key, or `h` `j` `k` `l` toggles shuffle |
| Repeat | `Enter`, `Right` `Up` / `l` `k` cycle forward (Off / All / One); `Left` `Down` / `h` `j` cycle backward |
| Speed | `Right` `Up` / `l` `k` / `]` increase by 0.25x; `Left` `Down` / `h` `j` / `[` decrease by 0.25x |

## Text Input

Playlist search, native-provider search, URL, playlist-name, keymap, and jump
fields support these editor keys:

| Key | Action |
|---|---|
| `Left` `Right` / `Home` `End` | Move cursor |
| `Backspace` `Delete` | Delete before/at cursor |
| `Ctrl+W` | Delete previous word |
| `Ctrl+U` | Clear text before cursor |

The Metadata shortcut is inactive while a text input is active.

### Search and filter modes

`/` filters the list on the screen. `Ctrl+F` searches YouTube Music, or your
saved playlists when the active provider is Local.

While a search or filter input is open, its line starts with a badge that
names the mode and the source, such as `[Filter: Playlist]`, `[Filter: Files]`
or `[Search: YouTube Music]`. The line ends with `Esc Exit`. Press `Esc` to
leave the input. On a narrow panel, the hint bar still shows `Esc`.

## EQ and Appearance

| Key | Action |
|---|---|
| `e` | Cycle EQ preset, including the saved Custom curve |
| `E` | Open the equalizer panel (see below) |
| `t` | Choose theme |
| `v` | Cycle visualizer and save the choice in `config.toml` |
| `}` / `{` | Visualizer sensitivity up / down (1-10, saved in `vis_sensitivity`). Also works in the full screen visualizer |
| `Ctrl+V` | Pick visualizer from a list (live preview) |
| `V` | Full screen visualizer. Inside it, `v` cycles modes and saves the choice, `<`/`>` change track, `+`/`-` change volume, and `t` hides the episode name, leaving only the bracketed source. `q` or `Esc` exits back to the player; `Ctrl+C` quits. |
| `Ctrl+H` | Toggle album headers |
| `Ctrl+G` | Toggle the key-binding hint bar (remembered in `hide_help_bar`) |
| `R` | Toggle the auto radio: when the queue ends, keep playing songs similar to the last one (remembered in `[ytmusic] auto_radio`) |
| `Ctrl+B` | Open/close the settings pane (remembered in `hide_settings_pane`) |

Theme and visualizer pickers support `/` filtering. While you browse, arrow
keys preview the selected option. `Enter` keeps it. `Esc` or `q` restores the option
active when the picker opened. While you type a filter, `Enter` completes it
and `Esc` clears it.

## Equalizer panel (`E` key)

The panel draws the ten bands as a curve from +12 to -12 dB, grouped into
bass (70-320 Hz), mids (600 Hz-3 kHz) and treble (6-16 kHz), and lists the
presets in two groups: genres (Rock, Metal, Pop, Electronic, Hip-Hop,
Reggaeton, Cumbia, Jazz, Classical, Acoustic, R&B, Indie, Lo-Fi, Soundtrack,
Blues) and utility curves (Flat, Bass Boost, Vocal, Loudness, Late Night,
Podcast, ...). A pre-amp cut follows the largest boost, and a soft clipper
rounds off the peaks, so boosts do not distort.

| Key | Action |
|-----|--------|
| `Up` `Down` / `k` `j` | Previous / next preset, applied at once as a preview; on the bands, gain +/-1 dB |
| `Left` `Right` / `h` `l` | Select a band |
| `Tab` | Switch between the preset list and the bands |
| `+` `-` | Band gain +/-1 dB |
| `0` | Set the band to 0 dB |
| `r` | Reset to Flat |
| `b` | Bypass the EQ to compare (A/B) |
| `a` | Auto EQ: switch to the preset of each song's genre (local tags, YouTube Music genre mixes); saved as `eq_auto` |
| `Esc` / `Enter` / `E` | Close the panel |

## Features

| Key | Action |
|---|---|
| `f` | Toggle the favorite ♥ on the selected track. Favorites appear in the "Favorites" virtual playlist. |
| `Ctrl+F` | Search YouTube Music by name or link, or your saved playlists in the Local provider. Available in playlist and provider-browser views. The search line names the source. See [Search and filter modes](#search-and-filter-modes). |
| `u` | Load URL (stream/playlist) |
| `d` | Open the audio device picker |
| `y` | Show or close lyrics |
| `r` | Retry lyrics lookup while lyrics are open |
| `[` / `]` | Adjust synced-lyrics timing offset (−/+250 ms) while lyrics show timestamped lines |
| `i` | From the playlist, open full info for the highlighted item, including Path (`Up`/`Down` or `j`/`k` scroll; `i`/`Esc`/`q` closes) |
| `Ctrl+I` | Toggle Metadata below Settings for the highlighted playlist item (remembered in `show_metadata`; requires a terminal that distinguishes Ctrl+I from Tab) |
| `Ctrl+S` | Save track to `[downloads].directory` (default `~/Music/cliamp-x`) |
| `w` | Write the highlighted track to a local playlist |
| `N` | Open the active provider browser |
| `Y` | Open YouTube Music provider |
| `L` | Open Local provider |

Metadata belongs to the main playback view, not provider browsers, and follows
the highlighted playlist row even when another item is playing. Enabling it
without a usable Settings sidebar opens the full info overlay instead; the
preference remains saved for a wider layout. See
[Metadata](configuration.md#metadata) for fields and layout behavior.

## Playlist and Queue

| Key | Action |
|---|---|
| `a` | Toggle the queue (play next) |
| `A` | Queue manager |
| `c` | In the queue manager: clear the queue (press again to confirm when more than one track) |
| `d` | In the queue manager: remove the highlighted track |
| `x` | Remove the highlighted track from the current playlist |
| `p` | Playlist |
| `r` | Cycle repeat mode (Off / All / One) |
| `z` | Toggle shuffle |

While shuffle is on, the playlist lists tracks in play order. The tracks that
played before the current track are above it. The tracks that play next are
below it. Each row keeps its original track number. When you turn off shuffle,
the playlist returns to the original order and the cursor stays on the same
track.

### Inside the save-to-playlist picker

Reached with `w`. The list shows your saved playlists plus a **New playlist**
row at the end.

| Key | Action |
|---|---|
| `Enter` | Add the tracks to the end of the selected playlist, or create a new one |
| `p` | Add the tracks to the start of the selected playlist instead |
| `Esc` `q` | Cancel |

`Enter` skips tracks the playlist already holds. `p` moves them to the front
instead, since putting a track first is an ordering request rather than a
duplicate. Tracks the playlist only holds through a `[[dir]]` source cannot be
reordered, so `p` leaves them alone and reports them as skipped.

### Inside the playlist

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Move cursor |
| `/` | Filter (incremental); `Esc` clears |
| `Enter` / `→` | List screen: open the selected playlist. Tracks screen: play the **selected** track. |
| `p` | Tracks screen: play all from the top |
| `w` | List: save the current queue with the playlist picker. Tracks: copy marked or selected tracks to another playlist. |
| `Space` | Tracks: mark/unmark highlighted track (advances on mark, stays on unmark) |
| `[` `]` | Tracks: move highlighted track and save the playlist |
| `f` | Tracks: toggle the favorite ♥ on the highlighted track. In "Favorites", an unfavorite removes the row. |
| `s` | Tracks: sort and save, cycling `track`, `title`, `artist`, `album`, `artist+album`, `path` |
| `o` | Tracks: open file browser to add files to this playlist |
| `D` | List: open the file browser to add `[[dir]]` sources to the selected playlist. Tracks: open the directory-sources screen. |
| `a` | List: create a playlist. After naming it, the file browser opens at `~`. Use `Enter` to enter a directory, `Space` to select folders or files, `Enter` to confirm, or `Esc` to finish. Tracks: mark or unmark all visible tracks. |
| `r` | List: rename the playlist (`Recently Played` cannot be renamed) |
| `d` | List: delete playlist (confirms; `Recently Played` cannot be deleted). Tracks: remove marked tracks, or highlighted track when none are marked |
| `A` | List: append the selected playlist to the current one, keeping what is loaded. Tracks: append the marked tracks, or the highlighted one. |
| `u` | Undo the last manager edit |
| `←` `Backspace` `h` `q` | Tracks screen: go back to the list |
| `Esc` `q` | Close the playlist or go back |

`Y` and `L` switch providers. Playlist-manager track actions use lowercase
or punctuation keys. `D` is the exception. It opens the directory-sources
screen. While a delete prompt waits for an answer, `y` or `Y` confirms and any
other key cancels. `Y` and `L` do not switch providers then.

#### Directory sources screen (`D` from the tracks screen)

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Navigate directory sources |
| `a` | Open the file browser to add a directory as a `[[dir]]` source |
| `d` then `y` | Remove the selected source. `y` confirms; any other key cancels. |
| `r` | Toggle `recursive` on the highlighted source |
| `←` `Backspace` `h` `Esc` | Back to the tracks screen |

## File browser

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Move cursor |
| `←` `→` / `h` `l` / `Enter` | Go back; open a directory or file |
| `/` | Filter files |
| `Space` | Select or unselect file/directory |
| `a` | Select/unselect all visible audio files |
| `R` | Replace the current queue with selected files (confirm when it is non-empty) |
| `w` | Write selected files to a local playlist |
| `D` | Add selected folders as live `[[dir]]` sources to the target playlist. If none are selected, add the selected folder or the open directory. The browser stays open. |
| `~` `.` | Jump to home / current working directory |
| `Esc` `q` `o` | Close file browser |

When the browser adds to a playlist, selected folders become `[[dir]]` sources.
Selected audio files become explicit tracks. This mode starts when you open the
browser with `D` from the manager list, with `o` from the tracks screen, or
after you create a playlist with `a`. In this mode, `Esc` means "done". cliamp-x
commits pending selections before it closes the browser.

## Provider browser (`N` key)

Press `N` to open the YouTube Music browser. Its menus are Inicio, Favoritos,
Álbumes, Artistas, Géneros, Playlists and Historial. Your local playlists
appear in the same menus. See [YouTube Music](youtube-music.md).

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Move cursor (wraps from top to bottom) |
| `←` `→` / `h` `l` | Go back; open the selected item |
| `/` | Filter the visible list |
| `f` | On a track, toggle its favorite ♥ |
| `a` | Append all visible tracks to the queue |
| `Enter` | Open the selected artist, album or playlist. A selected track plays and queues the rest of the visible list. |
| `R` | Replace the queue with all visible tracks (start from the top, confirm when non-empty) |
| `q` | Queue the highlighted track to play next |
| `s` | Cycle album sort (album list only) |
| `Y` `L` | Switch to that provider without opening the main pane |
| `Esc` `b` | Go back one level; close the browser |

While you type in the `/` filter, `Y` and `L` do not work. These keys add
their letter to the filter.

The header shows a source path such as `YouTube Music / Artist / Album / Tracks`.
This keeps the current provider and open location visible. Track rows show
right-aligned durations when the provider provides them.

## Provider playlist list

The playlists pane appears when the focus is on a provider, such as YouTube
Music or Local Playlists:

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` | Move cursor (wraps) |
| `Ctrl+U` `Ctrl+D` | Scroll by page |
| `Enter` | Load the selected playlist tracks into the queue |
| `/` | Filter the playlist list |
| `Ctrl+F` | Search YouTube Music |
| `Ctrl+R` | Refresh the provider: reload the currently open playlist in place, or return to the playlist list |
| `p` | Open the playlist as an overlay (the Local source shows it directly; create, rename, delete, add dirs/tracks) |
| `Y` `L` | Switch to that provider |
| `N` | Open the browser of the provider on screen |
| `Tab` | Leave the provider pane and focus Source, or the first visible playback control |
| `Shift+Tab` | Leave the provider pane and focus the last visible playback control (Speed, or Repeat with Settings closed) |
| `Esc` `b` | Back to the playlist pane |

Playlist rows show `Name · N tracks · 1h 23m` when the provider returns track
counts and total duration. The header shows `Provider / Playlists`. The loaded
playlist has a `▶` prefix.

## Search results overlays

Use these keys when `Ctrl+F` opens a search and the results list is open:

| Key | Action |
|---|---|
| `↑` `↓` / `j` `k` / `Ctrl+N` `Ctrl+P` | Move cursor (single item) |
| `Ctrl+U` `Ctrl+D` | Scroll results by page |
| `Enter` | Play the selected track now |
| `a` | Append the selected track to the playlist |
| `q` | Queue the selected track to play next |
| `f` | Toggle the favorite ♥ on the selected track |
| `Esc` `Backspace` | Back to the search input |

## Fuzzy search

Local search boxes use fuzzy matching. Query characters must appear in order but
do not need to be next to each other. Results are ranked by relevance, with the
best match first. For example, `skr` and `saku` both find a track named "Sakura".

This applies to:

- `/` playlist search
- `/` file browser filter
- `Ctrl+F` when the active provider is Local (your saved playlists)

YouTube Music search sends the query to YouTube Music, which controls the
matching rules.

## General

| Key | Action |
|---|---|
| `?` / `Ctrl+K` | Show keymap. `Enter` runs the selected command. `Esc` or `q` closes it. |
| `q` / `Ctrl+C` | Quit |

cliamp-x also quits as the `q` key does when it gets `SIGINT`, `SIGTERM` or `SIGHUP`. It then saves the resume position. The terminal sends `SIGHUP` when you close its window.

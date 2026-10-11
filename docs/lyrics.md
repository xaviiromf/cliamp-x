# Lyrics

Press `y` to show lyrics for the current track. For a local file, cliamp-x first uses lyrics embedded in file tags. If the file has no embedded lyrics, cliamp-x fetches lyrics from LRCLIB and NetEase Cloud Music. A YouTube Music song also has the lyrics YouTube Music shows; those carry no timestamps, so synced lyrics from LRCLIB win and the YouTube Music text is used when no source has synced lyrics.

The `lyrics` remote operation uses the same order, and the same artist and title as the overlay. For a radio stream, that is the current song from the stream title.

When LRCLIB or NetEase fails and no step finds lyrics, cliamp-x shows the error, for example a network error, instead of "No lyrics found".

Lyrics that cliamp-x finds online are kept in `~/.local/share/cliamp-x/lyrics/`, one small file per artist and title. A song played again shows its lyrics at once, also without a connection. A song with no lyrics is not remembered, so `r` can retry it later. Delete the folder to clear the cache.

## Modes

- **Synced lyrics**: For any track whose playback position maps to song time, lyrics scroll automatically and highlight the active line during playback. That covers local files and YouTube Music/yt-dlp tracks with a known duration. If the highlight is consistently early or late (some tracks are offset from the audio master), nudge the timing with `[`/`]` while the lyrics overlay shows timestamped lines; the offset is saved to `lyrics_offset_ms` in your config and applies to all synced sources.
- **Scroll mode**: For plain lyrics without timestamps, live radio (ICY), and YouTube Live, use `j`/`k` or the arrow keys to scroll manually. Live streams use this mode even with timestamped lyrics and station metadata, because their playback position is not relative to the song.

cliamp-x keeps timestamps in embedded LRC lyrics. It shows embedded plain-text lyrics in scroll mode.

## Streams

cliamp-x updates lyrics when ICY metadata changes, for example when an internet radio station changes tracks.

## YouTube

cliamp-x parses titles such as "Artist - Song (Official Video)" to build search queries.

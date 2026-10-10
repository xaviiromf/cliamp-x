# Streaming

cliamp-x plays audio from URLs, M3U/PLS playlists, and podcast RSS feeds.

## HTTP Streams

Play audio directly from a URL:

```sh
cliamp-x https://example.com/song.mp3
cliamp-x http://radio-station.com/stream.m3u
cliamp-x local.mp3 https://example.com/remote.mp3   # mix local + remote
```

For a non-seekable HTTP stream, the UI shows `● Streaming` and a static seek bar. Seek keys have no effect.

## PLS Playlists

cliamp-x supports PLS playlist files and M3U files:

```sh
cliamp-x https://example.com/lofi/stream.pls
```

## HLS Streams

cliamp-x supports HLS master and media playlists (`.m3u8`). Large broadcasters, such as Brazilian RBS/Wowza stations, use these playlists:

```sh
cliamp-x "https://example.com/live/playlist.m3u8"
```

cliamp-x passes the URL to ffmpeg. ffmpeg resolves relative chunklist and segment URIs and follows the live segment window. This requires `ffmpeg`, which AAC and Opus also require.

Live HLS uses timed metadata, not inline ICY. cliamp-x does not update the now-playing track title for HLS streams.

## Podcast Feeds

Play a podcast by passing its RSS feed URL:

```sh
cliamp-x https://example.com/podcast/feed.xml
```

cliamp-x loads the first 300 playable episodes in feed order, retaining episode
titles, show names, durations, artwork, and episode numbers when available.

### Xiaoyuzhou (小宇宙)

Play an individual [Xiaoyuzhou](https://www.xiaoyuzhoufm.com) episode by passing its URL:

```sh
cliamp-x https://www.xiaoyuzhoufm.com/episode/xxxx
```

## Track Info

For live radio, cliamp-x shows the current track from inline ICY metadata (`StreamTitle`). This works for most stations and codecs, including MP3, AAC, and Opus.

## Load URL at Runtime

Press `u` during playback to load a stream or playlist URL without restarting. It supports the same URL types as CLI arguments: direct audio URLs, M3U/PLS playlists, RSS podcast feeds, and yt-dlp compatible links.

See [playlists.md](playlists.md) for M3U playlist details.

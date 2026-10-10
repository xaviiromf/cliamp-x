# YouTube and yt-dlp

Install [yt-dlp](https://github.com/yt-dlp/yt-dlp), then play YouTube and YouTube Music URLs:

```sh
cliamp-x https://www.youtube.com/watch?v=dQw4w9WgXcQ
cliamp-x https://music.youtube.com/watch?v=dQw4w9WgXcQ
cliamp-x 'https://music.youtube.com/playlist?list=PLAYLIST'
```

You can play playlists and albums. Press `Ctrl+S` to download the current track to the [download directory](configuration.md#download-directory), `~/Music/cliamp-x` by default.

You can also play live streams, such as 24/7 YouTube lofi radios. They have no audio-only formats. cliamp-x uses the best muxed stream and plays only its audio track.

## Search

In the TUI, press `Ctrl+F` to search YouTube Music. See [YouTube Music](youtube-music.md).

## Disclaimer

**Use at your own risk.** Downloading or streaming copyrighted content can violate the terms of service of these platforms. You are responsible for use of this feature.

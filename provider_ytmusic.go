package main

import (
	"fmt"
	"os"

	"github.com/xaviiromf/cliamp-x/external/ytmusic"
	"github.com/xaviiromf/cliamp-x/internal/appdir"
	"github.com/xaviiromf/cliamp-x/internal/ytdlcookies"
	"github.com/xaviiromf/cliamp-x/internal/ytdlp"
	"github.com/xaviiromf/cliamp-x/player"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/resolve"
)

// YouTube Music comes first: it is the view cliamp-x opens on.
func init() {
	registerProvider(providerFactory{
		providerKey: providerKey{key: "ytmusic", name: "YouTube Music"},
		order:       10,
		shortcut:    "Y",
		build:       youTubeEntries,
	})
}

// youTubeEntries returns the YouTube Music entry. It returns none when the
// [ytmusic] section sets enabled = false. The sign-in comes from the browser
// cookies that `cliamp-x login` remembered; without one the provider still
// browses and searches. A missing yt-dlp only blocks playback.
func youTubeEntries(env providerEnv) []provider.Entry {
	cfg := env.cfg
	if !cfg.YouTubeMusic.IsSet() {
		logProviderSkipped(providerName("ytmusic"), "ytmusic", "disabled in config")
		return nil
	}
	if !player.YTDLPAvailable() {
		fmt.Fprintf(os.Stderr, "\nYouTube Music requires yt-dlp for audio playback.\n")
		fmt.Fprintf(os.Stderr, "Install command: %s\n\n", ytdlp.InstallHint())
		if offerYTDLPInstall(env.interactive, os.Stdin, os.Stderr) {
			fmt.Fprintf(os.Stderr, "Installing yt-dlp...\n")
			if err := installYTDLP(); err != nil {
				fmt.Fprintf(os.Stderr, "Installation failed: %v\n", err)
				fmt.Fprintf(os.Stderr, "Install yt-dlp manually and restart to play songs.\n\n")
			} else {
				fmt.Fprintf(os.Stderr, "yt-dlp installed successfully!\n\n")
			}
		}
	}

	auth := ytmusic.LoadAuth(cfg.YouTubeMusic.CookiesFrom)
	if auth.SignedIn() {
		// yt-dlp plays with the same session, so Premium quality and
		// age-restricted songs work.
		if path, err := ytmusic.CookiePath(); err == nil {
			for _, host := range []string{"youtube.com", "youtu.be", "music.youtube.com"} {
				resolve.SetYTDLCookiesForHost(host, ytdlcookies.FilePrefix+path)
			}
		}
	}
	client := ytmusic.NewClient(auth, cfg.YouTubeMusic.Language, cfg.YouTubeMusic.Region)
	music := ytmusic.New(client, ytmusic.Options{
		Favorites:     env.favorites,
		History:       env.history,
		Local:         env.library,
		Taste:         env.taste,
		ReportHistory: cfg.YouTubeMusic.ReportHistory,
		DataDir:       ytmusicDataDir(),
	})
	return []provider.Entry{{Provider: music}}
}

// ytmusicDataDir is where the YouTube Music provider keeps its caches.
func ytmusicDataDir() string {
	dir, err := appdir.DataDir()
	if err != nil {
		return ""
	}
	return dir
}

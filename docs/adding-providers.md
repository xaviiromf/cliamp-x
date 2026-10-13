# Adding a provider

A provider is a music source in the provider list, such as YouTube Music or
Local. This guide adds one in a fork of cliamp-x. Each provider lives in its
own files, so updates from upstream merge without conflicts. Your providers
keep working after each merge.

## The rule: new files only

Put everything in new files. Do not edit `providers.go`, `main.go` or the UI
to wire your provider in:

| File | Contents |
|------|----------|
| `external/<name>/` | Your provider package: the API client and the `Provider` type |
| `provider_<name>.go` (repo root) | The registration: one `init` function |
| `docs/<name>.md` | Optional: the user guide of your provider |

Upstream never creates a file named `provider_<name>.go` for a provider it
does not ship, so those files cannot collide. A merge from upstream only
touches files you did not change.

## 1. Write the provider

A provider implements `playlist.Provider` from `playlist/provider.go`:

```go
package myradio

import "github.com/xaviiromf/cliamp-x/playlist"

type Provider struct{ baseURL string }

func New(baseURL string) *Provider { return &Provider{baseURL: baseURL} }

// Name is the label in the provider list.
func (p *Provider) Name() string { return "My Radio" }

// Playlists returns the rows of the provider pane. Section groups rows
// under a heading.
func (p *Provider) Playlists() ([]playlist.PlaylistInfo, error) {
	return []playlist.PlaylistInfo{
		{ID: "top", Name: "Top stations", Section: "Browse"},
	}, nil
}

// Tracks returns the songs of one row. Path is what the player plays: a
// file, an http(s) URL, or a URL that yt-dlp understands.
func (p *Provider) Tracks(id string) ([]playlist.Track, error) {
	return []playlist.Track{
		{Path: p.baseURL + "/stream.mp3", Title: "Live", Stream: true},
	}, nil
}
```

Every other feature is optional. Implement the interfaces you need. The UI
finds them with type assertions. `provider/interfaces.go` documents each one,
and `playlist/provider.go` has `Authenticator`, `Refresher` and
`RefreshablePlaylist`. These are the common ones:

| Interface | What it gives |
|-----------|---------------|
| `playlist.Refresher` | `F5` / `Ctrl+R` drop your caches |
| `provider.Searcher` | `Ctrl+F` search |
| `provider.AlbumBrowser`, `provider.ArtistBrowser`, `provider.GenreBrowser` | The album, artist and genre browser (`N`) |
| `provider.BrowseEntryProvider` | Rows in your pane that open the browser |
| `provider.ArtistPager` | An artist list that loads more as the listener scrolls |
| `provider.TrackFavoriter` | `f` copies the ♥ favorite to your service |
| `provider.TrackDisliker` | `H` (not interested) sends a dislike to your service |
| `provider.Relater` | `M` (more like this) and the auto radio |
| `provider.PreviewLoader` | Details in the preview pane |
| `provider.CustomStreamer` | Your own URI scheme, such as `myradio://` |
| `provider.Closer` | Cleanup at exit |

To follow the listener's "interested" (`I`) and "not interested" (`H`)
ratings, take a `*taste.Store` in your options. Call `Hidden` to filter what
you recommend, and `Liked` to get the seeds. `external/ytmusic/discover.go`
shows both.

## 2. Register it

Create `provider_myradio.go` at the repo root:

```go
package main

import (
	"github.com/xaviiromf/cliamp-x/external/myradio"
	"github.com/xaviiromf/cliamp-x/provider"
)

func init() {
	registerProvider(providerFactory{
		providerKey: providerKey{key: "myradio", name: "My Radio", optional: true},
		order:       50,  // place in the list: YouTube Music is 10, Local is 100
		shortcut:    "K", // opens it from the player; "" for none
		build: func(env providerEnv) []provider.Entry {
			cfg := env.cfg.ProviderSection("myradio") // the [providers.myradio] section
			if cfg["url"] == "" {
				return nil // not configured: the log says so, nothing else changes
			}
			return []provider.Entry{{Provider: myradio.New(cfg["url"])}}
		},
	})
}
```

That is all the wiring. `registerProvider` adds the provider to these:

- The provider list, at its `order`.
- `--provider myradio` and `provider = "myradio"` in `config.toml`.
- The log line at start that says the provider registered or was skipped.

`env` also gives the shared stores: `favorites`, `history`, `taste`, the local
music `library`, and `interactive`, which tells whether a prompt can be
shown. Use `env.cfg` for the rest of the config.

## 3. Configure it

Users configure your provider in its own `config.toml` section:

```toml
[providers.myradio]
url = "https://radio.example.com"
```

`env.cfg.ProviderSection("myradio")` returns the keys as strings, without
quotes. When the section is missing, it returns nil. The section name ignores
letter case.

## What keeps a fork working across updates

- A factory that panics is left out and logged. The other providers still
  start.
- A factory that returns no entries, or an entry with a nil provider, is
  skipped.
- If two factories register the same key, the one registered last wins. Use
  your own key; do not reuse `ytmusic` or `local`.
- A `shortcut` that the player already uses for a command is ignored, so it
  cannot take over a player key. `Y` and `L` belong to YouTube Music and
  Local. Check `docs/keybindings.md` for free letters.
- The optional interfaces only add methods over time. A provider written
  against an older version still compiles. Run `make check` after each merge
  from upstream.

## Test it

`provider_registry_test.go` shows how a test registers a provider and checks
the list. Add table-driven tests for your package under `external/<name>/`,
and run:

```sh
mise exec -- make check
```

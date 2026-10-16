package model

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/stats"
	"github.com/xaviiromf/cliamp-x/ipc"
	"github.com/xaviiromf/cliamp-x/lyrics"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/resolve"
	"github.com/xaviiromf/cliamp-x/tracksave"
)

// ipcLibraryRequest carries a provider or saved-playlist operation to
// handleIPCLibrary. Reply receives the result of the job.
type ipcLibraryRequest struct {
	Op       string
	Provider string
	Playlist string
	Query    string
	Artist   string
	Album    string
	Sort     string
	Offset   int
	Limit    int
	Index    int
	NewName  string
	Track    *ipc.TrackInfo
	Tracks   []ipc.TrackInfo
	Context  context.Context
	Reply    chan ipc.Response
}

// ipcURLRequest carries url.load to handleIPCURL. Play starts the first
// added track even when something already plays. Without it the URL is
// appended and plays only when the player was idle. ListOnly (url.list) only
// returns the tracks and leaves the queue and the player alone.
type ipcURLRequest struct {
	URL      string
	Play     bool
	ListOnly bool
	Context  context.Context
	Reply    chan ipc.Response
}

// ipcSaveRequest, ipcLyricsRequest and ipcHistoryRequest carry the save,
// lyrics, history and history.clear operations.
type ipcSaveRequest struct {
	Context context.Context
	Reply   chan ipc.Response
}

type ipcLyricsRequest struct {
	Context context.Context
	Reply   chan ipc.Response
}

type ipcHistoryRequest struct {
	Op    string
	Limit int
	Reply chan ipc.Response
}

type ipcProviderLoadResult struct {
	request  ipcLibraryRequest
	tracks   []playlist.Track
	provider string // Name of the provider that served the tracks
	loaded   string
	source   string // Provider key and ID for the runtime snapshot
	err      error
}

// ipcPlaylistRenamedMsg tells Update that an IPC rename of a playlist of the
// named provider succeeded. Update follows the rename, then replies.
type ipcPlaylistRenamedMsg struct {
	provider         string
	oldName, newName string
	reply            chan ipc.Response
}

// ipcHistoryClearedMsg tells Update that IPC history.clear emptied the
// history. Update refreshes the views of it, then replies.
type ipcHistoryClearedMsg struct {
	reply chan ipc.Response
}

type ipcURLLoadResult struct {
	request ipcURLRequest
	tracks  []playlist.Track
	err     error
}

type ipcFeedLoadResult struct {
	op       string
	feed     playlist.Track
	jobs     *ipc.JobStore
	jobID    string
	revision uint64
	tracks   []playlist.Track
	err      error
}

// handleIPCURL resolves the URL of request in a command. A job.cancel request
// or an IPC server shutdown cancels the request context, which stops the
// resolve.
func (m *Model) handleIPCURL(request ipcURLRequest) tea.Cmd {
	return func() tea.Msg {
		tracks, err := resolve.URLContext(requestContext(request.Context), request.URL)
		return ipcURLLoadResult{request: request, tracks: tracks, err: err}
	}
}

func (m *Model) handleIPCURLResult(result ipcURLLoadResult) tea.Cmd {
	if result.request.Context != nil && result.request.Context.Err() != nil {
		return nil
	}
	if result.err != nil {
		result.request.Reply <- ipc.Response{OK: false, Error: result.err.Error()}
		return nil
	}
	if len(result.tracks) == 0 {
		result.request.Reply <- ipc.Response{OK: false, Error: "no tracks found at URL"}
		return nil
	}
	if result.request.ListOnly {
		result.request.Reply <- ipc.Response{OK: true, Tracks: ipcTrackInfos(result.tracks, func(playlist.Track) bool { return false }), Total: len(result.tracks)}
		return nil
	}
	// A Play request jumps to the first newly added track, so a caller that
	// asked to play a URL hears it even when something is already playing.
	// Without it the tracks are appended and only start when the player is
	// idle, which is the right default for a plain append.
	wasStopped := !m.player.IsPlaying()
	start := m.appendTracks(result.tracks...)
	result.request.Reply <- ipc.Response{OK: true, Tracks: ipcTrackInfos(result.tracks, m.trackFavoriteLookup(true)), Total: len(result.tracks)}
	if result.request.Play {
		m.stopPlayback()
		m.player.ClearPreload()
		m.playlist.SetIndex(start)
		m.plCursor = start
		m.adjustScroll()
		return m.playCurrentTrack()
	}
	if wasStopped {
		return m.playCurrentTrack()
	}
	return nil
}

func (m *Model) handleIPCSave(request ipcSaveRequest) tea.Cmd {
	track, index := m.currentPlaybackTrack()
	if index < 0 {
		request.Reply <- ipc.Response{OK: false, Error: "nothing to save"}
		return nil
	}
	directory := m.downloadsDirectory
	return func() tea.Msg {
		path, err := tracksave.SaveTo(requestContext(request.Context), track, directory)
		if err != nil {
			request.Reply <- ipc.Response{OK: false, Error: err.Error()}
		} else {
			request.Reply <- ipc.Response{OK: true, Output: path}
		}
		return nil
	}
}

func ipcFeedLoadCmd(ctx context.Context, op string, feed playlist.Track, jobs *ipc.JobStore, jobID string, revision uint64) tea.Cmd {
	return func() tea.Msg {
		resolveCtx, cancel := context.WithTimeout(requestContext(ctx), 30*time.Second)
		defer cancel()
		tracks, err := resolve.Feed(resolveCtx, feed.Path)
		if err == nil {
			err = resolveCtx.Err()
		}
		return ipcFeedLoadResult{
			op: op, feed: feed, jobs: jobs, jobID: jobID, revision: revision,
			tracks: tracks, err: err,
		}
	}
}

func (m *Model) handleIPCFeedLoad(result ipcFeedLoadResult) tea.Cmd {
	ctx, ok := result.jobs.Context(result.jobID)
	if !ok || ctx.Err() != nil {
		return nil
	}
	if result.err == nil && len(result.tracks) == 0 {
		result.err = fmt.Errorf("no playable episodes found in feed")
	}
	if result.err != nil {
		err := v2InternalError()
		err.Detail = result.err.Error()
		m.failV2Job(result.jobs, result.jobID, err)
		return nil
	}
	if result.revision != 0 && result.revision != m.playlist.Revision() {
		m.failV2Job(result.jobs, result.jobID, v2ConflictError())
		return nil
	}
	// Expand before touching the playlist: playing a feed placeholder would
	// invoke the legacy feed resolver, which replaces the entire playlist.
	var cmd tea.Cmd
	if result.op == "track.play" {
		cmd = m.playAlbumImmediate(result.feed, result.tracks)
	} else {
		cmd = m.queueAlbumNext(result.feed, result.tracks)
	}
	// Capture completion with this mutation, not in a later waiter update.
	m.completeV2Job(result.jobs, result.jobID, m.v2PlaylistResponse())
	return cmd
}

func (m *Model) handleIPCLibrary(request ipcLibraryRequest) tea.Cmd {
	if request.Context != nil && request.Context.Err() != nil {
		return nil
	}
	if request.Op == "provider.list" {
		items := make([]ipc.ProviderInfo, 0, len(m.providers))
		for _, entry := range m.providers {
			_, searchable := entry.Provider.(provider.Searcher)
			if _, ok := entry.Provider.(stationSearcher); ok {
				searchable = true
			}
			_, browseArtists := entry.Provider.(provider.ArtistBrowser)
			_, browseAlbums := entry.Provider.(provider.AlbumBrowser)
			_, catalog := entry.Provider.(provider.CatalogLoader)
			info := ipc.ProviderInfo{Key: entry.Key, Name: entry.Name, Searchable: searchable, BrowseArtists: browseArtists, BrowseAlbums: browseAlbums, Catalog: catalog}
			if account, ok := entry.Provider.(interface{ SignedIn() bool }); ok {
				signedIn := account.SignedIn()
				info.SignedIn = &signedIn
			}
			items = append(items, info)
		}
		request.Reply <- ipc.Response{OK: true, Providers: items}
		return nil
	}

	entry, ok := m.ipcProvider(request.Provider)
	if !ok {
		request.Reply <- ipc.Response{OK: false, Error: fmt.Sprintf("unknown provider %q", request.Provider)}
		return nil
	}

	switch request.Op {
	case "playlist.create":
		creator, ok := entry.Provider.(provider.PlaylistCreator)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support playlist creation"}
			return nil
		}
		return func() tea.Msg {
			if request.Context != nil && request.Context.Err() != nil {
				return nil
			}
			id, err := creator.CreatePlaylist(requestContext(request.Context), request.Playlist)
			response := ipcResponseError(err)
			if err == nil {
				response.Playlist = id
			}
			request.Reply <- response
			return nil
		}
	case "playlist.rename":
		renamer, ok := entry.Provider.(provider.PlaylistRenamer)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support playlist renaming"}
			return nil
		}
		return func() tea.Msg {
			if request.Context != nil && request.Context.Err() != nil {
				return nil
			}
			if err := renamer.RenamePlaylist(request.Playlist, request.NewName); err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			return ipcPlaylistRenamedMsg{provider: entry.Provider.Name(), oldName: request.Playlist, newName: request.NewName, reply: request.Reply}
		}
	case "playlist.delete":
		deleter, ok := entry.Provider.(provider.PlaylistDeleter)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support playlist deletion"}
			return nil
		}
		return ipcMutationCmd(request.Context, request.Reply, func() error { return deleter.DeletePlaylist(request.Playlist) })
	case "playlist.remove":
		deleter, ok := entry.Provider.(provider.PlaylistDeleter)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support removing tracks"}
			return nil
		}
		return ipcMutationCmd(request.Context, request.Reply, func() error { return deleter.RemoveTrack(request.Playlist, request.Index) })
	case "playlist.add":
		writer, ok := entry.Provider.(provider.PlaylistWriter)
		if !ok || request.Track == nil {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support adding this track"}
			return nil
		}
		track := ipcTrackFromInfo(*request.Track)
		return ipcMutationCmd(request.Context, request.Reply, func() error {
			return writer.AddTrackToPlaylist(requestContext(request.Context), request.Playlist, track)
		})
	case "playlist.add_many":
		if len(request.Tracks) == 0 {
			request.Reply <- ipc.Response{OK: false, Error: "tracks are required"}
			return nil
		}
		tracks := make([]playlist.Track, len(request.Tracks))
		for i, info := range request.Tracks {
			tracks[i] = ipcTrackFromInfo(info)
		}
		return func() tea.Msg {
			if request.Context != nil && request.Context.Err() != nil {
				return nil
			}
			added, skipped, err := provider.AddTracks(requestContext(request.Context), entry.Provider, request.Playlist, tracks)
			if err != nil {
				request.Reply <- ipcResponseError(err)
			} else {
				request.Reply <- ipc.Response{OK: true, Total: added, Items: []string{fmt.Sprintf("skipped:%d", skipped)}}
			}
			return nil
		}
	case "playlist.replace":
		saver, ok := entry.Provider.(provider.PlaylistSaver)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support replacing playlists"}
			return nil
		}
		tracks := make([]playlist.Track, len(request.Tracks))
		for i, info := range request.Tracks {
			tracks[i] = ipcTrackFromInfo(info)
		}
		return ipcMutationCmd(request.Context, request.Reply, func() error { return saver.SavePlaylist(request.Playlist, tracks) })
	case "playlist.bookmark":
		// playlist.bookmark keeps its name for old scripts. It toggles the ♥
		// favorite of the track, as f does. The track can come from the queue
		// or from a provider list, so the saved rule applies only to a queue row.
		if request.Track == nil {
			request.Reply <- ipc.Response{OK: false, Error: "track is required"}
			return nil
		}
		track := ipcTrackFromInfo(*request.Track)
		cmd, err := m.togglePlaylistTrackFavorite(track, m.savedPlaylistRow(track))
		response := ipcResponseError(err)
		if err == nil {
			favorite := m.playlistTrackFavorited(track)
			response.Favorite = &favorite
		}
		request.Reply <- response
		return cmd
	case "provider.playlists":
		return func() tea.Msg {
			items, err := ipcProviderPlaylistInfos(entry)
			if err != nil {
				request.Reply <- ipc.Response{OK: false, Error: err.Error()}
				return nil
			}
			page, total := ipcPage(items, request.Offset, request.Limit, 200)
			request.Reply <- ipc.Response{OK: true, Playlists: page, Total: total}
			return nil
		}
	case "provider.catalog":
		loader, ok := entry.Provider.(provider.CatalogLoader)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support catalog paging"}
			return nil
		}
		return func() tea.Msg {
			limit := request.Limit
			if limit <= 0 || limit > 200 {
				limit = 50
			}
			added, err := loader.LoadCatalogPage(request.Offset, limit)
			if err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			items, err := ipcProviderPlaylistInfos(entry)
			if err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			request.Reply <- ipc.Response{OK: true, Playlists: items, Total: added}
			return nil
		}
	case "provider.tracks":
		favorite := m.trackFavoriteLookup(false)
		return func() tea.Msg {
			tracks, err := entry.Provider.Tracks(request.Playlist)
			if err != nil {
				request.Reply <- ipc.Response{OK: false, Error: err.Error()}
			} else {
				page, total := ipcPage(tracks, request.Offset, request.Limit, 200)
				request.Reply <- ipc.Response{OK: true, Tracks: ipcTrackInfos(page, favorite), Playlist: request.Playlist, Total: total}
			}
			return nil
		}
	case "provider.load":
		name := entry.Provider.Name()
		return func() tea.Msg {
			tracks, err := entry.Provider.Tracks(request.Playlist)
			return ipcProviderLoadResult{request: request, tracks: tracks, provider: name, loaded: request.Playlist, source: entry.Key + ":" + request.Playlist, err: err}
		}
	case "provider.search":
		favorite := m.trackFavoriteLookup(false)
		return func() tea.Msg {
			limit := request.Limit
			if limit <= 0 || limit > 100 {
				limit = 25
			}
			tracks, err := ipcSearchProvider(requestContext(request.Context), entry.Provider, request.Query, min(100, request.Offset+limit))
			if err != nil {
				request.Reply <- ipc.Response{OK: false, Error: err.Error()}
			} else {
				page, total := ipcPage(tracks, request.Offset, limit, 100)
				request.Reply <- ipc.Response{OK: true, Tracks: ipcTrackInfos(page, favorite), Total: total}
			}
			return nil
		}
	case "provider.artists":
		if pager, ok := entry.Provider.(provider.ArtistPager); ok && request.Sort == "foryou" {
			return func() tea.Msg {
				limit := request.Limit
				if limit <= 0 {
					limit = 40
				}
				artists, err := pager.ArtistList(request.Offset, min(limit, 200))
				if err != nil {
					request.Reply <- ipcResponseError(err)
					return nil
				}
				// The list has no end, so total is the page length.
				request.Reply <- ipc.Response{OK: true, Artists: ipcArtistInfos(artists), Total: len(artists)}
				return nil
			}
		}
		browser, ok := entry.Provider.(provider.ArtistBrowser)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support artist browsing"}
			return nil
		}
		return func() tea.Msg {
			artists, err := browser.Artists()
			if err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			page, total := ipcPage(artists, request.Offset, request.Limit, 200)
			request.Reply <- ipc.Response{OK: true, Artists: ipcArtistInfos(page), Total: total}
			return nil
		}
	case "provider.artist_albums":
		browser, ok := entry.Provider.(provider.ArtistBrowser)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support artist browsing"}
			return nil
		}
		return func() tea.Msg {
			albums, err := browser.ArtistAlbums(request.Artist)
			if err != nil {
				request.Reply <- ipcResponseError(err)
			} else {
				page, total := ipcPage(albums, request.Offset, request.Limit, 200)
				request.Reply <- ipc.Response{OK: true, Albums: ipcAlbumInfos(page), Total: total}
			}
			return nil
		}
	case "provider.albums":
		browser, ok := entry.Provider.(provider.AlbumBrowser)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support album browsing"}
			return nil
		}
		return func() tea.Msg {
			sortType := request.Sort
			if sortType == "" {
				sortType = browser.DefaultAlbumSort()
			}
			limit := request.Limit
			if limit <= 0 || limit > 200 {
				limit = 100
			}
			albums, err := browser.AlbumList(sortType, request.Offset, limit)
			if err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			request.Reply <- ipc.Response{OK: true, Albums: ipcAlbumInfos(albums), Sorts: ipcSortInfos(browser.AlbumSortTypes()), Total: len(albums)}
			return nil
		}
	case "provider.album_tracks", "provider.load_album":
		loader, ok := entry.Provider.(provider.AlbumTrackLoader)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support album tracks"}
			return nil
		}
		favorite := m.trackFavoriteLookup(false)
		name := entry.Provider.Name()
		return func() tea.Msg {
			tracks, err := loader.AlbumTracks(request.Album)
			if request.Op == "provider.load_album" {
				return ipcProviderLoadResult{request: request, tracks: tracks, provider: name, loaded: "album:" + request.Album, source: entry.Key + ":album:" + request.Album, err: err}
			}
			if err != nil {
				request.Reply <- ipcResponseError(err)
			} else {
				page, total := ipcPage(tracks, request.Offset, request.Limit, 200)
				request.Reply <- ipc.Response{OK: true, Tracks: ipcTrackInfos(page, favorite), Total: total}
			}
			return nil
		}
	case "provider.favorite":
		favorites, ok := entry.Provider.(provider.FavoriteToggler)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support favorites"}
			return nil
		}
		return func() tea.Msg {
			added, _, err := favorites.ToggleFavorite(request.Playlist)
			response := ipcResponseError(err)
			if err == nil {
				response.Favorite = &added
			}
			request.Reply <- response
			return nil
		}
	case "provider.home":
		loader, ok := entry.Provider.(provider.HomeLoader)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support home"}
			return nil
		}
		favorite := m.trackFavoriteLookup(false)
		return func() tea.Msg {
			shelves, err := loader.HomeShelves()
			if err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			request.Reply <- ipc.Response{OK: true, Shelves: ipcShelfInfos(shelves, favorite), Total: len(shelves)}
			return nil
		}
	case "provider.genres":
		browser, ok := entry.Provider.(provider.GenreBrowser)
		if router, routed := entry.Provider.(provider.GenreBrowseRouter); routed && request.Playlist != "" {
			browser = router.GenreBrowserFor(request.Playlist)
			ok = browser != nil
		}
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support genres"}
			return nil
		}
		return func() tea.Msg {
			genres, err := browser.Genres()
			if err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			items := make([]ipc.GenreInfo, len(genres))
			for i, genre := range genres {
				items[i] = ipc.GenreInfo{ID: genre.ID, Name: genre.Name, Group: genre.Group, Favorite: genre.Favorite}
			}
			request.Reply <- ipc.Response{OK: true, Genres: items, Sorts: ipcSortInfos(browser.GenreSortTypes()), Total: len(items)}
			return nil
		}
	case "provider.genre_tracks":
		browser, ok := entry.Provider.(provider.GenreBrowser)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support genres"}
			return nil
		}
		favorite := m.trackFavoriteLookup(false)
		return func() tea.Msg {
			tracks, err := browser.GenreTracks(request.Playlist, request.Sort)
			if err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			page, total := ipcPage(tracks, request.Offset, request.Limit, 200)
			request.Reply <- ipc.Response{OK: true, Tracks: ipcTrackInfos(page, favorite), Playlist: request.Playlist, Total: total}
			return nil
		}
	case "provider.genre_favorite":
		toggler, ok := entry.Provider.(provider.GenreFavoriteToggler)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support genre favorites"}
			return nil
		}
		if request.Playlist == "" {
			request.Reply <- ipc.Response{OK: false, Error: "playlist is required"}
			return nil
		}
		return func() tea.Msg {
			on, err := toggler.ToggleGenreFavorite(request.Playlist)
			response := ipcResponseError(err)
			if err == nil {
				response.Favorite = &on
			}
			request.Reply <- response
			return nil
		}
	case "provider.refresh":
		refresher, ok := entry.Provider.(playlist.Refresher)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support refresh"}
			return nil
		}
		return ipcMutationCmd(request.Context, request.Reply, func() error {
			refresher.Refresh()
			return nil
		})
	case "provider.preview":
		loader, ok := entry.Provider.(provider.PreviewLoader)
		if !ok {
			request.Reply <- ipc.Response{OK: false, Error: "provider does not support previews"}
			return nil
		}
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(requestContext(request.Context), 15*time.Second)
			defer cancel()
			details, err := loader.PreviewDetails(ctx, request.Playlist)
			if err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			preview := &ipc.PreviewInfo{ImageURL: details.ImageURL, Lines: compactLines(details.Lines...)} // a playlist without author gives a blank line
			if stater, ok := entry.Provider.(provider.FavoriteStater); ok {
				if on, known := stater.FavoriteState(request.Playlist); known {
					preview.Favorite = &on
				}
			}
			request.Reply <- ipc.Response{OK: true, Preview: preview}
			return nil
		}
	default:
		request.Reply <- ipc.Response{OK: false, Error: "unknown provider operation"}
		return nil
	}
}

func ipcProviderPlaylistInfos(entry provider.Entry) ([]ipc.PlaylistInfo, error) {
	lists, err := entry.Provider.Playlists()
	if err != nil {
		return nil, err
	}
	_, writer := entry.Provider.(provider.PlaylistWriter)
	targets, filtered := entry.Provider.(provider.PlaylistTargetFilter)
	items := make([]ipc.PlaylistInfo, len(lists))
	for i, list := range lists {
		items[i] = ipc.PlaylistInfo{
			ID: list.ID, Name: list.Name, Provider: entry.Key, Section: list.Section, TrackCount: list.TrackCount,
			DurationSecs: list.DurationSecs, Favorite: list.Favorite, ImageURL: list.ImageURL,
			Writable: writer && (!filtered || targets.CanAddToPlaylist(list)),
			Unseen:   strings.HasPrefix(list.Name, "● "),
		}
		if sectioned, ok := entry.Provider.(provider.SectionedList); ok {
			items[i].Favoritable = sectioned.IsFavoritableID(list.ID)
		}
	}
	return items, nil
}

func ipcMutationCmd(ctx context.Context, reply chan ipc.Response, mutate func() error) tea.Cmd {
	return func() tea.Msg {
		if ctx != nil && ctx.Err() != nil {
			return nil
		}
		reply <- ipcResponseError(mutate())
		return nil
	}
}

func ipcResponseError(err error) ipc.Response {
	if err != nil {
		return ipc.Response{OK: false, Error: err.Error()}
	}
	return ipc.Response{OK: true}
}

// stationSearcher is a station catalog that can search without the search
// state of its pane, as the radio provider does with SearchStations.
type stationSearcher interface {
	SearchStations(ctx context.Context, query string, limit int) ([]playlist.Track, error)
}

// ipcSearchProvider runs an IPC search on source with SearchTracks or
// SearchStations. Neither one changes the search of the pane. A provider with
// only a catalog search gets an error, because that search replaces the pane
// search.
func ipcSearchProvider(ctx context.Context, source playlist.Provider, query string, limit int) ([]playlist.Track, error) {
	ctx, cancel := context.WithTimeout(requestContext(ctx), 30*time.Second)
	defer cancel()
	if searcher, ok := source.(provider.Searcher); ok {
		return searcher.SearchTracks(ctx, query, limit)
	}
	if stations, ok := source.(stationSearcher); ok {
		return stations.SearchStations(ctx, query, limit)
	}
	return nil, fmt.Errorf("provider does not support search")
}

func (m *Model) handleIPCProviderLoad(result ipcProviderLoadResult) tea.Cmd {
	if result.request.Context != nil && result.request.Context.Err() != nil {
		return nil
	}
	if result.err != nil {
		result.request.Reply <- ipc.Response{OK: false, Error: result.err.Error()}
		return nil
	}
	// This replaces the queue wholesale, so retire any in-flight paged load:
	// its later pages would otherwise still pass the generation guard and
	// append onto the list loaded here.
	m.retireTracksPaging()
	m.replacePlaylist(result.tracks)
	m.setLoadedLocalPlaylist(result.provider, result.loaded)
	if m.loadedPlaylist == "" {
		m.playlistSource = result.source
	}
	m.setHeaderStateFromTracks(result.tracks)
	start := 0
	if i := result.request.Index; i > 0 && i < len(result.tracks) {
		start = i
	}
	m.playlist.SetIndex(start)
	m.plCursor = start
	// The first page only: a long list would not fit in one frame.
	resp := m.v2PlaylistResponse()
	resp.Playlist = result.request.Playlist
	result.request.Reply <- resp
	return m.playCurrentTrack()
}

// handleIPCPlaylistRenamed moves the loaded playlist to the new name when
// the local provider renamed it, as the manager rename key does.
func (m *Model) handleIPCPlaylistRenamed(msg ipcPlaylistRenamedMsg) {
	if m.localProvider != nil && msg.provider == m.localProvider.Name() {
		m.renameLoadedPlaylist(msg.oldName, msg.newName)
	}
	msg.reply <- ipc.Response{OK: true}
}

func requestContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func ipcPage[T any](items []T, offset, limit, max int) ([]T, int) {
	total := len(items)
	if offset < 0 || offset >= total {
		return nil, total
	}
	if limit <= 0 || limit > max {
		limit = max
	}
	end := min(total, offset+limit)
	return items[offset:end], total
}

// handleIPCLyrics looks up the lyrics of the track that plays in the same
// order and with the same artist and title as the lyrics overlay.
func (m *Model) handleIPCLyrics(request ipcLyricsRequest) tea.Cmd {
	track, idx := m.currentPlaybackTrack()
	if idx < 0 {
		request.Reply <- ipc.Response{OK: false, Error: "no current track"}
		return nil
	}
	artist, title := m.lyricsArtistTitle()
	lookups := lyricsLookups(track, m.trackLyricsSources())
	ctx := requestContext(request.Context)
	return func() tea.Msg {
		lines, err := lyrics.Lookup(ctx, track.EmbeddedLyrics, artist, title, lookups...)
		if err != nil {
			request.Reply <- ipc.Response{OK: false, Error: err.Error()}
			return nil
		}
		items := make([]ipc.LyricLine, len(lines))
		for i, line := range lines {
			items[i] = ipc.LyricLine{Start: line.Start.Seconds(), Text: line.Text}
		}
		request.Reply <- ipc.Response{OK: true, Lyrics: items}
		return nil
	}
}

func (m *Model) handleIPCHistory(request ipcHistoryRequest) tea.Cmd {
	favorite := m.trackFavoriteLookup(false)
	return func() tea.Msg {
		if m.historyStore == nil {
			request.Reply <- ipc.Response{OK: true}
			return nil
		}
		if request.Op == "history.clear" {
			if err := m.historyStore.Clear(); err != nil {
				request.Reply <- ipcResponseError(err)
				return nil
			}
			return ipcHistoryClearedMsg{reply: request.Reply}
		}
		entries, err := m.historyStore.Recent(request.Limit)
		if err != nil {
			request.Reply <- ipc.Response{OK: false, Error: err.Error()}
			return nil
		}
		items := make([]ipc.HistoryInfo, len(entries))
		for i, entry := range entries {
			items[i] = ipc.HistoryInfo{Track: ipcTrackInfo(entry.Track, i, 0, favorite(entry.Track)), PlayedAt: entry.PlayedAt.Format(time.RFC3339)}
		}
		request.Reply <- ipc.Response{OK: true, History: items}
		return nil
	}
}

// handleIPCHistoryCleared refreshes the views of the emptied history, as a
// history write does, then replies to history.clear.
func (m *Model) handleIPCHistoryCleared(msg ipcHistoryClearedMsg) tea.Cmd {
	cmd := m.refreshHistoryViews()
	msg.reply <- ipc.Response{OK: true}
	return cmd
}

// ipcSeed returns what radio.start and download act on: track, or else the
// item playlist of provider, named name.
func (m *Model) ipcSeed(request ipc.Request) (discoverSeed, bool) {
	if request.Track != nil {
		track := ipcTrackFromInfo(*request.Track)
		return discoverSeed{track: track, label: track.Title}, track.Path != ""
	}
	entry, ok := m.ipcProvider(request.Provider)
	return discoverSeed{itemID: request.Playlist, prov: entry.Provider, label: cmp.Or(request.Name, request.Playlist)}, ok && request.Playlist != ""
}

// handleIPCRemixes finds the versions of track, grouped by kind in
// provider_meta section, as X does, without opening the search overlay.
func (m *Model) handleIPCRemixes(ctx context.Context, track playlist.Track, reply chan ipc.Response) tea.Cmd {
	searcher, _ := findCapable(m, func(provider.Searcher) bool { return true })
	if searcher == nil {
		reply <- ipc.Response{Error: L("No provider can search")}
		return nil
	}
	base := cleanSongTitle(track.Title)
	if base == "" {
		base = track.Title
	}
	favorite := m.trackFavoriteLookup(false)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(requestContext(ctx), discoverTimeout)
		defer cancel()
		tracks, err := searchRemixes(ctx, searcher, track, base)
		if err != nil {
			reply <- ipcResponseError(err)
		} else {
			reply <- ipc.Response{OK: true, Tracks: ipcTrackInfos(tracks, favorite), Total: len(tracks)}
		}
		return nil
	}
}

// statsPeriods are the period names of the stats operation.
var statsPeriods = map[string]stats.Period{"week": stats.Week, "month": stats.Month, "year": stats.Year, "all": stats.All}

// handleIPCStats sums the play log over period, as the statistics screen
// does.
func (m *Model) handleIPCStats(period stats.Period, reply chan ipc.Response) tea.Cmd {
	log := m.statsLog
	return func() tea.Msg {
		plays, err := log.Load()
		if err != nil {
			reply <- ipcResponseError(err)
			return nil
		}
		info := ipcStatsInfo(stats.Summarize(plays, period, time.Now(), statsTop, statsDays))
		reply <- ipc.Response{OK: true, Stats: &info}
		return nil
	}
}

// handleIPCWrapped sums the play log over the calendar year, or the current
// year when year is 0, in the local zone.
func (m *Model) handleIPCWrapped(year int, reply chan ipc.Response) tea.Cmd {
	log := m.statsLog
	return func() tea.Msg {
		plays, err := log.Load()
		if err != nil {
			reply <- ipcResponseError(err)
			return nil
		}
		if year == 0 {
			year = time.Now().Year()
		}
		w := stats.Wrapped(plays, year, time.Local, statsTop)
		info := ipcStatsInfo(w.Summary)
		info.Year, info.Minutes, info.Days = w.Year, int(w.Secs/60), w.Days
		info.TopAlbums = ipcStatsCounts(w.TopAlbums)
		if p := w.First; p != nil {
			info.First = &ipc.StatsPlay{Name: p.Title, Detail: p.Artist, Path: p.Key, At: p.At.Format(time.RFC3339)}
		}
		if c := w.MostReplayed; c != nil {
			most := ipc.StatsCount(*c)
			info.MostReplayed = &most
		}
		reply <- ipc.Response{OK: true, Stats: &info}
		return nil
	}
}

// ipcStatsInfo converts a summary for the stats operation.
func ipcStatsInfo(s stats.Summary) ipc.StatsInfo {
	return ipc.StatsInfo{
		Plays: s.Plays, Secs: s.Secs, Songs: s.Songs, Artists: s.Artists, TopArtists: ipcStatsCounts(s.TopArtists),
		TopSongs: ipcStatsCounts(s.TopSongs), TopGenres: ipcStatsCounts(s.TopGenres), Daily: s.Daily, Streak: s.Streak,
	}
}

func ipcStatsCounts(in []stats.Count) []ipc.StatsCount {
	out := make([]ipc.StatsCount, len(in))
	for i, c := range in {
		out[i] = ipc.StatsCount(c)
	}
	return out
}

func (m *Model) ipcProvider(key string) (provider.Entry, bool) {
	for _, entry := range m.providers {
		if strings.EqualFold(entry.Key, key) {
			return entry, true
		}
	}
	return provider.Entry{}, false
}

// ipcTrackInfos converts tracks for IPC. favorite reports the ♥ state of a
// track, see trackFavoriteLookup.
func ipcTrackInfos(tracks []playlist.Track, favorite func(playlist.Track) bool) []ipc.TrackInfo {
	items := make([]ipc.TrackInfo, len(tracks))
	for i, track := range tracks {
		items[i] = ipcTrackInfo(track, i, 0, favorite(track))
	}
	return items
}

// ipcAlbumInfos converts albums for IPC. Names keep their marks, such as the
// ♥ of a saved album, for the CLI; the flags carry them for other clients.
func ipcAlbumInfos(albums []provider.AlbumInfo) []ipc.AlbumInfo {
	items := make([]ipc.AlbumInfo, len(albums))
	for i, album := range albums {
		items[i] = ipc.AlbumInfo{
			ID: album.ID, Name: album.Name, Artist: album.Artist, ArtistID: album.ArtistID, Year: album.Year,
			TrackCount: album.TrackCount, Genre: album.Genre, ImageURL: album.ImageURL,
			Favorite: strings.HasPrefix(album.Name, "♥ "), Single: album.Single,
		}
	}
	return items
}

// ipcArtistInfos converts artists for IPC; a followed artist's name starts
// with ♥.
func ipcArtistInfos(artists []provider.ArtistInfo) []ipc.ArtistInfo {
	items := make([]ipc.ArtistInfo, len(artists))
	for i, artist := range artists {
		items[i] = ipc.ArtistInfo{ID: artist.ID, Name: artist.Name, AlbumCount: artist.AlbumCount, ImageURL: artist.ImageURL, Favorite: strings.HasPrefix(artist.Name, "♥ ")}
	}
	return items
}

func ipcSortInfos(sorts []provider.SortType) []ipc.SortInfo {
	items := make([]ipc.SortInfo, len(sorts))
	for i, item := range sorts {
		items[i] = ipc.SortInfo{ID: item.ID, Label: item.Label}
	}
	return items
}

// ipcShelfInfos converts home shelves for IPC. favorite reports the ♥ state
// of a song card's track.
func ipcShelfInfos(shelves []provider.Shelf, favorite func(playlist.Track) bool) []ipc.ShelfInfo {
	items := make([]ipc.ShelfInfo, len(shelves))
	for i, shelf := range shelves {
		cards := make([]ipc.ShelfItemInfo, len(shelf.Items))
		for j, it := range shelf.Items {
			cards[j] = ipc.ShelfItemInfo{Kind: it.Kind, ID: it.ID, Title: it.Title, Subtitle: it.Subtitle, ImageURL: it.ImageURL}
			if it.Track != nil {
				track := ipcTrackInfo(*it.Track, 0, 0, favorite(*it.Track))
				cards[j].Track = &track
			}
		}
		items[i] = ipc.ShelfInfo{Title: shelf.Title, Items: cards}
	}
	return items
}

// ipcTrackInfo converts a track for IPC. The bookmark field keeps its JSON
// name for old scripts and reports the ♥ favorite state.
func ipcTrackInfo(track playlist.Track, index, queuePosition int, favorite bool) ipc.TrackInfo {
	return ipc.TrackInfo{
		Title: track.Title, Artist: track.Artist, Album: track.Album, Genre: track.Genre,
		Path: track.Path, AlbumArtURL: track.AlbumArtURL, Year: track.Year,
		TrackNumber: track.TrackNumber, DurationSecs: track.DurationSecs, Index: index,
		QueuePosition: queuePosition, Stream: track.Stream, Realtime: track.Realtime,
		Restricted: track.Restricted, Feed: track.Feed, Bookmark: favorite, Unplayable: track.Unplayable,
		DirSourced: track.DirSourced, ProviderMeta: maps.Clone(track.ProviderMeta),
	}
}

func ipcTrackFromInfo(info ipc.TrackInfo) playlist.Track {
	return playlist.Track{
		Title: info.Title, Artist: info.Artist, Album: info.Album, Genre: info.Genre,
		Path: info.Path, AlbumArtURL: info.AlbumArtURL, Year: info.Year,
		TrackNumber: info.TrackNumber, DurationSecs: info.DurationSecs,
		Stream: info.Stream || playlist.IsURL(info.Path), Realtime: info.Realtime,
		Restricted: info.Restricted, Feed: info.Feed, Unplayable: info.Unplayable,
		DirSourced: info.DirSourced, ProviderMeta: maps.Clone(info.ProviderMeta),
	}
}

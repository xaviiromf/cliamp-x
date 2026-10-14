package ytmusic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/favorites"
	"github.com/xaviiromf/cliamp-x/history"
	"github.com/xaviiromf/cliamp-x/locallib"
	"github.com/xaviiromf/cliamp-x/lyrics"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
)

// ProviderMeta keys of YouTube Music tracks.
const (
	MetaVideoID  = "ytmusic.id"
	MetaArtistID = "ytmusic.artist"
	MetaAlbumID  = "ytmusic.album"
	MetaPlays    = "ytmusic.plays"
)

// Pane row and browse IDs.
const (
	rowFavorites = "fav"
	rowHistory   = "hist"
	prefixList   = "pl:"    // a playlist, by browse ID (VL...)
	prefixAlbum  = "al:"    // an album, by browse ID (MPREb_...)
	prefixShelf  = "shelf:" // the songs of a home shelf, by index
	prefixArtist = "ar:"    // the top songs of an artist, by channel ID
	prefixGenre  = "genre:" // the mix of a genre, by category params
	prefixRadio  = "radio:" // the song radio of a video

	rowLocal          = "local:all"    // every track of the local library
	prefixLocalAlbum  = "local:al:"    // a local album, by its library key
	prefixLocalArtist = "local:ar:"    // a local-only artist, by name
	prefixLocalGenre  = "local:genre:" // a local genre, by name

	browseAlbums  = "browse:albums"
	browseArtists = "browse:artists"
	browseGenres  = "browse:genres"
	browseMoods   = "browse:moods"

	cacheTTL      = 15 * time.Minute
	requestBudget = 45 * time.Second
	homeRowLimit  = 15
)

// Options configures the provider.
type Options struct {
	Favorites     *favorites.Store // the ♥ favorites, shared with the UI
	History       *history.Store   // the local play history, shared with the UI
	Local         *locallib.Library
	ReportHistory bool
	// DataDir holds the provider's caches, such as the new releases of the
	// subscribed artists. Empty keeps them in memory only.
	DataDir string
	// Taste hides what the listener is not interested in and seeds the
	// recommendations with what they are. Nil hides nothing.
	Taste *taste.Store
}

// Provider is the YouTube Music provider.
type Provider struct {
	client *Client
	opts   Options
	words  labels
	genres *genreFavorites

	releases newReleases

	mu        sync.Mutex
	home      []Shelf
	homeAt    time.Time
	homeErr   error // why the last home read failed, or nil
	library   []Item
	libraryAt time.Time
	liked     []Song
	likedAt   time.Time
	cats      []Category
	catsAt    time.Time
	albums    []provider.AlbumInfo
	albumsAt  time.Time
	artists   []provider.ArtistInfo
	artistsAt time.Time

	previews previewCache

	// The browse snapshot on disk: what home, the library and the genres held
	// last run. Each flag marks a list that came from it and still waits for
	// its first refresh.
	snapLoaded                      bool
	homeDisk, libraryDisk, catsDisk bool
	snapSeq                         uint64     // snapshots taken, under mu
	snapWriteMu                     sync.Mutex // orders the snapshot writes
	snapWritten                     uint64     // newest snapshot written, under snapWriteMu

	savedAlbums map[string]bool // library albums, by browse ID
	subscribed  map[string]bool // subscribed artists, by channel ID

	// discMu orders the discovery rounds and is taken before mu. Refresh
	// bumps discEpoch, and the next round starts the lists over.
	discMu    sync.Mutex
	disc      discovery
	discEpoch atomic.Uint64
}

// heart marks a saved album or a subscribed artist in the browse lists.
const heart = "♥ "

// New returns a provider over a signed-in (or signed-out) client.
func New(client *Client, opts Options) *Provider {
	return &Provider{
		client: client,
		opts:   opts,
		words:  labelsFor(client.hl),
		genres: loadGenreFavorites(),
	}
}

// Client returns the API client of the provider.
func (p *Provider) Client() *Client { return p.client }

func (p *Provider) Name() string { return "YouTube Music" }

// SignedIn reports whether the cookies hold a YouTube sign-in. It does no
// I/O.
func (p *Provider) SignedIn() bool { return p.client.SignedIn() }

func budget() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestBudget)
}

// songTrack turns a song into a playlist track.
func songTrack(s Song) playlist.Track {
	t := playlist.Track{
		Path:         WatchURL(s.VideoID),
		Title:        s.Title,
		Artist:       s.ArtistName(),
		Album:        s.Album,
		DurationSecs: s.Duration,
		Stream:       true,
		ProviderMeta: map[string]string{MetaVideoID: s.VideoID},
	}
	if id := s.ArtistID(); id != "" {
		t.ProviderMeta[MetaArtistID] = id
	}
	if s.AlbumID != "" {
		t.ProviderMeta[MetaAlbumID] = s.AlbumID
	}
	if s.Plays != "" {
		t.ProviderMeta[MetaPlays] = s.Plays
	}
	t.AlbumArtURL = s.Thumbnail
	if t.AlbumArtURL == "" {
		t.AlbumArtURL = VideoThumbnail(s.VideoID)
	}
	return t
}

// VideoThumbnail returns the 16:9 thumbnail of a YouTube video.
func VideoThumbnail(videoID string) string {
	return "https://i.ytimg.com/vi/" + videoID + "/mqdefault.jpg"
}

func songTracks(songs []Song) []playlist.Track {
	out := make([]playlist.Track, 0, len(songs))
	seen := make(map[string]bool, len(songs))
	for _, s := range songs {
		if seen[s.VideoID] {
			continue
		}
		seen[s.VideoID] = true
		out = append(out, songTrack(s))
	}
	return out
}

// trackVideoID returns the video ID of a track from any YouTube source.
func trackVideoID(t playlist.Track) string {
	if id := t.Meta(MetaVideoID); id != "" {
		return id
	}
	return VideoIDFromPath(t.Path)
}

// --- cached reads ---

func (p *Provider) cachedHome() []Shelf {
	p.mu.Lock()
	p.loadSnapshotLocked()
	if p.home != nil && time.Since(p.homeAt) < cacheTTL {
		defer p.mu.Unlock()
		return p.home
	}
	if p.homeDisk {
		p.homeDisk = false
		defer p.mu.Unlock()
		go p.fetchHome()
		return p.home
	}
	p.mu.Unlock()
	return p.fetchHome()
}

// fetchHome reads the home page from YouTube Music and keeps it.
func (p *Provider) fetchHome() []Shelf {
	ctx, cancel := budget()
	defer cancel()
	home, err := p.client.Home(ctx, 10)
	if err != nil {
		applog.Warn("ytmusic: home: %v", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.homeErr = err
	if err == nil {
		p.home, p.homeAt = home, time.Now()
		p.saveSnapshotLocked()
	}
	return p.home
}

// HomeShelves returns the home page as cards, without what the listener is
// not interested in, the liked-songs playlist and the shelves left empty.
func (p *Provider) HomeShelves() ([]provider.Shelf, error) {
	home := p.cachedHome()
	if len(home) == 0 {
		p.mu.Lock()
		err := p.homeErr
		p.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	var out []provider.Shelf
	for _, sh := range home {
		shelf := provider.Shelf{Title: sh.Title}
		for _, it := range sh.Items {
			if p.hidden([]string{it.Title, it.Artist}, it.ID) {
				continue
			}
			item := provider.ShelfItem{ID: it.ID, Title: it.Title, Subtitle: it.Subtitle, ImageURL: it.Thumbnail}
			if item.Subtitle == "" {
				item.Subtitle = it.Artist
			}
			switch it.Kind {
			case KindSong, KindVideo:
				if it.Song == nil {
					continue
				}
				track := songTrack(*it.Song)
				if len(p.visibleTracks([]playlist.Track{track})) == 0 {
					continue
				}
				item.Kind, item.Track = "song", &track
			case KindAlbum:
				item.Kind = "album"
			case KindPlaylist:
				if strings.TrimPrefix(it.ID, "VL") == "LM" {
					continue
				}
				item.Kind, item.ID = "playlist", prefixList+it.ID
			case KindArtist:
				item.Kind = "artist"
			default:
				continue
			}
			shelf.Items = append(shelf.Items, item)
		}
		if len(shelf.Items) > 0 {
			out = append(out, shelf)
		}
	}
	return out, nil
}

func (p *Provider) cachedLibrary() []Item {
	if !p.client.SignedIn() {
		return nil
	}
	p.mu.Lock()
	p.loadSnapshotLocked()
	if p.library != nil && time.Since(p.libraryAt) < cacheTTL {
		defer p.mu.Unlock()
		return p.library
	}
	if p.libraryDisk {
		p.libraryDisk = false
		defer p.mu.Unlock()
		go p.fetchLibrary()
		return p.library
	}
	p.mu.Unlock()
	return p.fetchLibrary()
}

// fetchLibrary reads the account's playlists from YouTube Music and keeps
// them.
func (p *Provider) fetchLibrary() []Item {
	ctx, cancel := budget()
	defer cancel()
	items, err := p.client.LibraryPlaylists(ctx)
	if err != nil {
		applog.Warn("ytmusic: library playlists: %v", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err == nil {
		if items == nil {
			items = []Item{}
		}
		p.library, p.libraryAt = items, time.Now()
		p.saveSnapshotLocked()
	}
	return p.library
}

// cachedLiked returns the account's liked songs and mirrors them into the
// local favorites store, so a song liked on another device shows its ♥.
func (p *Provider) cachedLiked() []Song {
	if !p.client.SignedIn() {
		return nil
	}
	p.mu.Lock()
	if p.liked != nil && time.Since(p.likedAt) < cacheTTL {
		defer p.mu.Unlock()
		return p.liked
	}
	p.mu.Unlock()
	ctx, cancel := budget()
	defer cancel()
	songs, err := p.client.LikedSongs(ctx, 0)
	if err != nil {
		applog.Warn("ytmusic: liked songs: %v", err)
		p.mu.Lock()
		defer p.mu.Unlock()
		return p.liked
	}
	if songs == nil {
		songs = []Song{}
	}
	if p.opts.Favorites != nil && len(songs) > 0 {
		if added, err := p.opts.Favorites.Import(songTracks(songs)); err != nil {
			applog.Warn("ytmusic: import liked songs: %v", err)
		} else if added > 0 {
			applog.Info("ytmusic: %d liked songs added to favorites", added)
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.liked, p.likedAt = songs, time.Now()
	return p.liked
}

func (p *Provider) cachedCategories() []Category {
	p.mu.Lock()
	p.loadSnapshotLocked()
	if p.cats != nil && time.Since(p.catsAt) < cacheTTL*4 {
		defer p.mu.Unlock()
		return p.cats
	}
	if p.catsDisk {
		p.catsDisk = false
		defer p.mu.Unlock()
		go p.fetchCategories()
		return p.cats
	}
	p.mu.Unlock()
	return p.fetchCategories()
}

// fetchCategories reads the genres and moods from YouTube Music and keeps
// them.
func (p *Provider) fetchCategories() []Category {
	ctx, cancel := budget()
	defer cancel()
	cats, err := p.client.Categories(ctx)
	if err != nil {
		applog.Warn("ytmusic: categories: %v", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err == nil {
		p.cats, p.catsAt = cats, time.Now()
		p.saveSnapshotLocked()
	}
	return p.cats
}

// Refresh drops every cached list, so the next read asks YouTube Music, and
// starts the endless lists over in a new order.
func (p *Provider) Refresh() {
	p.resetDiscovery()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.opts.Local.Refresh()
	var zero time.Time
	p.homeAt, p.libraryAt, p.likedAt, p.catsAt, p.albumsAt, p.artistsAt = zero, zero, zero, zero, zero, zero
	p.homeDisk, p.libraryDisk, p.catsDisk = false, false, false
}

// CanRefreshPlaylist reports that every row reloads in place.
func (p *Provider) CanRefreshPlaylist(string) bool { return true }

// --- the provider pane ---

// Playlists returns the rows of the provider pane: your music, your
// favorite genres and the home page. The browse menus come from
// BrowseEntries.
func (p *Provider) Playlists() ([]playlist.PlaylistInfo, error) {
	w := p.words
	var rows []playlist.PlaylistInfo
	favCount := 0
	if p.opts.Favorites != nil {
		if p.client.SignedIn() {
			p.cachedLiked()
		}
		favCount = p.opts.Favorites.Count()
	}
	rows = append(rows,
		playlist.PlaylistInfo{ID: rowFavorites, Name: w.favorites, TrackCount: favCount, Section: w.yourMusic},
		playlist.PlaylistInfo{ID: rowHistory, Name: w.history, Section: w.yourMusic},
	)
	if n := len(p.opts.Local.Tracks()); n > 0 {
		rows = append(rows, playlist.PlaylistInfo{ID: rowLocal, Name: w.localMusic, TrackCount: n, Section: w.yourMusic})
	}
	for _, it := range p.cachedLibrary() {
		id := strings.TrimPrefix(it.ID, "VL")
		if id == "LM" || id == "SE" {
			continue // Liked music is in Favorites; SE is podcast episodes.
		}
		rows = append(rows, playlist.PlaylistInfo{ID: prefixList + it.ID, Name: it.Title, Section: w.yourMusic, ImageURL: it.Thumbnail})
	}

	for _, r := range p.releaseRows() {
		if !p.hidden([]string{r.Name}, r.ID) {
			rows = append(rows, r)
		}
	}

	for _, g := range p.favoriteGenres() {
		rows = append(rows, playlist.PlaylistInfo{ID: prefixGenre + g.Params, Name: "★ " + g.Title, Section: w.genresSection})
	}

	seen := map[string]bool{}
	homeRows := 0
	for i, sh := range p.cachedHome() {
		if homeRows >= homeRowLimit {
			break
		}
		songs := 0
		for _, it := range sh.Items {
			if it.Kind == KindSong || it.Kind == KindVideo {
				songs++
			}
		}
		if songs > 0 && songs*2 >= len(sh.Items) {
			if !p.hidden([]string{sh.Title}, prefixShelf+strconv.Itoa(i)) {
				rows = append(rows, playlist.PlaylistInfo{ID: prefixShelf + strconv.Itoa(i), Name: sh.Title, TrackCount: songs, Section: w.home, ImageURL: sh.Items[0].Thumbnail})
				homeRows++
			}
			continue
		}
		// Only the home mixes and playlists get rows: the albums and artists
		// the home page recommends are in Albums and Artists, which keeps
		// this pane short.
		for _, it := range sh.Items {
			if it.Kind != KindPlaylist {
				continue
			}
			id := prefixList + it.ID
			if seen[id] || strings.TrimPrefix(it.ID, "VL") == "LM" || p.hidden([]string{it.Title, it.Artist}, it.ID) {
				continue
			}
			seen[id] = true
			name := it.Title
			if it.Artist != "" && it.Kind != KindArtist && it.Artist != it.Title {
				name += " · " + it.Artist
			}
			rows = append(rows, playlist.PlaylistInfo{ID: id, Name: name, Section: w.home + " · " + sh.Title, ImageURL: it.Thumbnail})
			homeRows++
		}
	}
	return rows, nil
}

// BrowseEntries puts the Explore menus at the top of the provider pane.
func (p *Provider) BrowseEntries() []provider.BrowseEntry {
	w := p.words
	return []provider.BrowseEntry{
		{ID: browseAlbums, Name: w.albums, Section: w.explore, Mode: provider.BrowseAlbums},
		{ID: browseArtists, Name: w.artists, Section: w.explore, Mode: provider.BrowseArtistAlbums},
		{ID: browseGenres, Name: w.genres, Section: w.explore, Mode: provider.BrowseGenres},
		{ID: browseMoods, Name: w.moods, Section: w.explore, Mode: provider.BrowseGenres},
	}
}

// BrowseLabels names the browse levels.
func (p *Provider) BrowseLabels() (artist, album string) {
	if p.words.albums == spanish.albums {
		return "Artista", "Álbum"
	}
	return "Artist", "Album"
}

// Tracks loads the songs of a pane row or browse leaf.
func (p *Provider) Tracks(id string) ([]playlist.Track, error) {
	ctx, cancel := budget()
	defer cancel()
	switch {
	case id == rowFavorites:
		return p.favoriteTracks()
	case id == rowHistory:
		return p.historyTracks(ctx)
	case id == rowLocal:
		return p.opts.Local.Tracks(), nil
	case strings.HasPrefix(id, "local:"):
		return p.AlbumTracks(id)
	case strings.HasPrefix(id, prefixList):
		return p.listTracks(ctx, strings.TrimPrefix(id, prefixList))
	case strings.HasPrefix(id, prefixAlbum):
		albumID := strings.TrimPrefix(id, prefixAlbum)
		p.markReleaseSeen(albumID)
		return p.AlbumTracks(albumID)
	case strings.HasPrefix(id, prefixShelf):
		return p.shelfTracks(strings.TrimPrefix(id, prefixShelf))
	case strings.HasPrefix(id, prefixArtist):
		return p.artistTopTracks(ctx, strings.TrimPrefix(id, prefixArtist))
	case strings.HasPrefix(id, prefixGenre):
		return p.GenreTracks(strings.TrimPrefix(id, prefixGenre), "mix")
	case strings.HasPrefix(id, prefixRadio):
		songs, err := p.client.Radio(ctx, strings.TrimPrefix(id, prefixRadio), "")
		return songTracks(songs), err
	}
	return nil, fmt.Errorf("youtube music: unknown list %q", id)
}

// listTracks loads a playlist. Radio mixes (RD...) are dynamic and load
// through the watch queue instead of the playlist page.
func (p *Provider) listTracks(ctx context.Context, id string) ([]playlist.Track, error) {
	plain := strings.TrimPrefix(id, "VL")
	if strings.HasPrefix(plain, "RD") && !strings.HasPrefix(plain, "RDCLAK") {
		songs, err := p.client.Radio(ctx, "", plain)
		return songTracks(songs), err
	}
	page, err := p.client.Playlist(ctx, plain, 0)
	if err != nil {
		return nil, err
	}
	if len(page.Songs) == 0 && strings.HasPrefix(plain, "RD") {
		songs, err := p.client.Radio(ctx, "", plain)
		return songTracks(songs), err
	}
	return songTracks(page.Songs), nil
}

// favoriteTracks returns the ♥ favorites: the account's liked songs (mirrored
// into the store) and every local or YouTube track favorited in cliamp-x.
func (p *Provider) favoriteTracks() ([]playlist.Track, error) {
	if p.client.SignedIn() {
		p.cachedLiked()
	}
	if p.opts.Favorites == nil {
		return songTracks(p.cachedLiked()), nil
	}
	return p.opts.Favorites.Tracks()
}

// historyTracks returns what cliamp-x played, newest first, followed by the
// rest of the account's YouTube Music history.
func (p *Provider) historyTracks(ctx context.Context) ([]playlist.Track, error) {
	var out []playlist.Track
	seen := map[string]bool{}
	if p.opts.History != nil {
		local, err := p.opts.History.Tracks(500)
		if err != nil {
			applog.Warn("ytmusic: history: %v", err)
		}
		for _, t := range local {
			if !seen[t.Path] {
				seen[t.Path] = true
				out = append(out, t)
			}
		}
	}
	if p.client.SignedIn() {
		songs, err := p.client.History(ctx)
		if err != nil {
			applog.Warn("ytmusic: account history: %v", err)
		}
		for _, t := range songTracks(songs) {
			if !seen[t.Path] {
				seen[t.Path] = true
				out = append(out, t)
			}
		}
	}
	return out, nil
}

func (p *Provider) shelfTracks(index string) ([]playlist.Track, error) {
	i, err := strconv.Atoi(index)
	home := p.cachedHome()
	if err != nil || i < 0 || i >= len(home) {
		return nil, errors.New("youtube music: the home page changed; press Ctrl+R")
	}
	var songs []Song
	for _, it := range home[i].Items {
		if it.Song != nil {
			songs = append(songs, *it.Song)
		}
	}
	return songTracks(songs), nil
}

func (p *Provider) artistTopTracks(ctx context.Context, channel string) ([]playlist.Track, error) {
	page, err := p.client.Artist(ctx, channel)
	if err != nil {
		return nil, err
	}
	if page.SongsListID != "" {
		if list, err := p.client.Playlist(ctx, page.SongsListID, 200); err == nil && len(list.Songs) > 0 {
			return songTracks(list.Songs), nil
		}
	}
	return songTracks(page.TopSongs), nil
}

// --- albums ---

// AlbumSortTypes lists the album orders.
func (p *Provider) AlbumSortTypes() []provider.SortType {
	w := p.words
	return []provider.SortType{
		{ID: sortForYou, Label: w.sortForYou},
		{ID: "top", Label: w.sortTop},
		{ID: "alpha", Label: w.sortAlpha},
		{ID: "artist", Label: w.sortArtist},
		{ID: "year", Label: w.sortYear},
	}
}

func (p *Provider) DefaultAlbumSort() string { return sortForYou }

// AlbumList returns a page of the album list: the saved albums and the
// albums of the liked songs, most liked first, then the albums the home page
// recommends.
func (p *Provider) AlbumList(sortType string, offset, size int) ([]provider.AlbumInfo, error) {
	if sortType == sortForYou || sortType == "" {
		return p.discoverAlbums(offset, size), nil
	}
	var sorted []provider.AlbumInfo
	for _, a := range p.allAlbums() {
		if !p.albumHidden(a) {
			sorted = append(sorted, a)
		}
	}
	switch sortType {
	case "alpha":
		sort.SliceStable(sorted, func(i, j int) bool { return lessFold(sorted[i].Name, sorted[j].Name) })
	case "artist":
		sort.SliceStable(sorted, func(i, j int) bool {
			if !strings.EqualFold(sorted[i].Artist, sorted[j].Artist) {
				return lessFold(sorted[i].Artist, sorted[j].Artist)
			}
			return lessFold(sorted[i].Name, sorted[j].Name)
		})
	case "year":
		sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Year > sorted[j].Year })
	}
	if offset >= len(sorted) {
		return nil, nil
	}
	end := len(sorted)
	if size > 0 && offset+size < end {
		end = offset + size
	}
	return sorted[offset:end], nil
}

func lessFold(a, b string) bool { return strings.ToLower(a) < strings.ToLower(b) }

// ranked collects entries by ID, adding up their scores, and returns them
// best first; ties keep the order of first appearance.
type ranked[T any] struct {
	byID  map[string]*rankedEntry[T]
	order int
}

type rankedEntry[T any] struct {
	value T
	score int
	order int
}

func newRanked[T any]() *ranked[T] {
	return &ranked[T]{byID: map[string]*rankedEntry[T]{}}
}

// add records value under id, or adds score to the entry already there and
// lets merge fill its gaps.
func (r *ranked[T]) add(id string, value T, score int, merge func(old *T, add T)) {
	if e, ok := r.byID[id]; ok {
		e.score += score
		if merge != nil {
			merge(&e.value, value)
		}
		return
	}
	r.order++
	r.byID[id] = &rankedEntry[T]{value: value, score: score, order: r.order}
}

func (r *ranked[T]) sorted() []T {
	entries := make([]*rankedEntry[T], 0, len(r.byID))
	for _, e := range r.byID {
		entries = append(entries, e)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].score != entries[j].score {
			return entries[i].score > entries[j].score
		}
		return entries[i].order < entries[j].order
	})
	out := make([]T, len(entries))
	for i, e := range entries {
		out[i] = e.value
	}
	return out
}

func (p *Provider) allAlbums() []provider.AlbumInfo {
	p.mu.Lock()
	if p.albums != nil && time.Since(p.albumsAt) < cacheTTL {
		defer p.mu.Unlock()
		return p.albums
	}
	p.mu.Unlock()

	r := newRanked[provider.AlbumInfo]()
	add := func(info provider.AlbumInfo, score int) {
		if info.ID == "" || info.Name == "" {
			return
		}
		r.add(info.ID, info, score, func(old *provider.AlbumInfo, add provider.AlbumInfo) {
			if old.Year == 0 {
				old.Year = add.Year
			}
			if old.Artist == "" {
				old.Artist, old.ArtistID = add.Artist, add.ArtistID
			}
			if old.ImageURL == "" {
				old.ImageURL = add.ImageURL
			}
		})
	}
	if p.client.SignedIn() {
		ctx, cancel := budget()
		saved, err := p.client.LibraryAlbums(ctx)
		cancel()
		if err != nil {
			applog.Warn("ytmusic: library albums: %v", err)
		}
		savedSet := map[string]bool{}
		for _, it := range saved {
			savedSet[it.ID] = true
			add(provider.AlbumInfo{ID: it.ID, Name: heart + it.Title, Artist: it.Artist, ArtistID: it.ArtistID, Year: it.Year, ImageURL: it.Thumbnail}, 1000)
		}
		p.mu.Lock()
		p.savedAlbums = savedSet
		p.mu.Unlock()
		for _, s := range p.cachedLiked() {
			if s.AlbumID != "" {
				add(provider.AlbumInfo{ID: s.AlbumID, Name: s.Album, Artist: s.ArtistName(), ArtistID: s.ArtistID(), ImageURL: s.Thumbnail}, 10)
			}
		}
	}
	for _, sh := range p.cachedHome() {
		for _, it := range sh.Items {
			if it.Kind == KindAlbum {
				add(provider.AlbumInfo{ID: it.ID, Name: it.Title, Artist: it.Artist, ArtistID: it.ArtistID, Year: it.Year, ImageURL: it.Thumbnail}, 1)
			}
		}
	}
	if local := p.opts.Local.Albums(); len(local) > 0 {
		favs := p.favoriteSet()
		for _, a := range local {
			liked := 0
			for _, t := range a.Tracks {
				if favs[t.Path] {
					liked++
				}
			}
			add(provider.AlbumInfo{ID: prefixLocalAlbum + a.Key, Name: a.Name + p.words.localTag, Artist: a.Artist, Year: a.Year}, 5+10*liked)
		}
	}
	out := r.sorted()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.albums, p.albumsAt = out, time.Now()
	return out
}

// AlbumTracks loads an album, a playlist, an artist's top songs or a song
// radio, after the prefix of its ID.
func (p *Provider) AlbumTracks(albumID string) ([]playlist.Track, error) {
	ctx, cancel := budget()
	defer cancel()
	switch {
	case strings.HasPrefix(albumID, prefixLocalAlbum):
		a, ok := p.opts.Local.Album(strings.TrimPrefix(albumID, prefixLocalAlbum))
		if !ok {
			return nil, errors.New("this local album is gone; press Ctrl+R")
		}
		return a.Tracks, nil
	case strings.HasPrefix(albumID, prefixLocalArtist):
		var tracks []playlist.Track
		for _, a := range p.opts.Local.ArtistAlbums(strings.TrimPrefix(albumID, prefixLocalArtist)) {
			tracks = append(tracks, a.Tracks...)
		}
		return tracks, nil
	case strings.HasPrefix(albumID, prefixLocalGenre):
		return p.opts.Local.GenreTracks(strings.TrimPrefix(albumID, prefixLocalGenre)), nil
	case strings.HasPrefix(albumID, "MPREb_"):
		page, err := p.client.Album(ctx, albumID)
		return songTracks(page.Songs), err
	case strings.HasPrefix(albumID, prefixArtist):
		return p.artistTopTracks(ctx, strings.TrimPrefix(albumID, prefixArtist))
	case strings.HasPrefix(albumID, prefixRadio):
		songs, err := p.client.Radio(ctx, strings.TrimPrefix(albumID, prefixRadio), "")
		return songTracks(songs), err
	case strings.HasPrefix(albumID, "UC"):
		return p.artistTopTracks(ctx, albumID)
	default:
		return p.listTracks(ctx, strings.TrimPrefix(albumID, prefixList))
	}
}

// --- artists ---

// Artists returns the artists of the library, the subscriptions and the
// liked songs, most liked first, then the ones the home page recommends.
func (p *Provider) Artists() ([]provider.ArtistInfo, error) {
	p.mu.Lock()
	if p.artists != nil && time.Since(p.artistsAt) < cacheTTL {
		defer p.mu.Unlock()
		return p.artists, nil
	}
	p.mu.Unlock()

	r := newRanked[provider.ArtistInfo]()
	add := func(id, name, image string, score int) {
		if id != "" && name != "" {
			r.add(id, provider.ArtistInfo{ID: id, Name: name, ImageURL: image}, score, func(old *provider.ArtistInfo, add provider.ArtistInfo) {
				if old.ImageURL == "" {
					old.ImageURL = add.ImageURL
				}
			})
		}
	}
	if p.client.SignedIn() {
		ctx, cancel := budget()
		subs, err := p.client.Subscriptions(ctx)
		if err != nil {
			applog.Warn("ytmusic: subscriptions: %v", err)
		}
		subSet := map[string]bool{}
		for _, it := range subs {
			subSet[it.ID] = true
			add(it.ID, heart+it.Title, it.Thumbnail, 1000)
		}
		p.mu.Lock()
		p.subscribed = subSet
		p.mu.Unlock()
		lib, err := p.client.LibraryArtists(ctx)
		cancel()
		if err != nil {
			applog.Warn("ytmusic: library artists: %v", err)
		}
		for _, it := range lib {
			add(it.ID, it.Title, it.Thumbnail, 100)
		}
		for _, s := range p.cachedLiked() {
			for _, a := range s.Artists {
				add(a.ID, a.Name, "", 10)
			}
		}
	}
	for _, sh := range p.cachedHome() {
		for _, it := range sh.Items {
			if it.Kind == KindArtist {
				add(it.ID, it.Title, it.Thumbnail, 1)
			} else if it.ArtistID != "" {
				add(it.ArtistID, it.Artist, "", 1)
			}
		}
	}
	// Local artists join the YouTube Music artist of the same name, so one
	// row opens both; the others get a local row.
	byName := map[string]string{}
	for id, e := range r.byID {
		byName[strings.ToLower(e.value.Name)] = id
	}
	for name, count := range p.opts.Local.Artists() {
		if id, ok := byName[strings.ToLower(name)]; ok {
			r.add(id, provider.ArtistInfo{}, 5+count, nil)
			continue
		}
		r.add(prefixLocalArtist+name, provider.ArtistInfo{ID: prefixLocalArtist + name, Name: name + p.words.localTag}, 5+count, nil)
	}
	out := r.sorted()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.artists, p.artistsAt = out, time.Now()
	return out, nil
}

// ArtistAlbums lists what an artist page offers: the top songs, then every
// album and single.
func (p *Provider) ArtistAlbums(artistID string) ([]provider.AlbumInfo, error) {
	if name, ok := strings.CutPrefix(artistID, prefixLocalArtist); ok {
		return p.localAlbumInfos(name), nil
	}
	ctx, cancel := budget()
	defer cancel()
	page, err := p.client.Artist(ctx, artistID)
	if err != nil {
		return nil, err
	}
	name := page.Name
	out := []provider.AlbumInfo{{ID: prefixArtist + artistID, Name: p.words.topSongs, Artist: name, ArtistID: artistID, ImageURL: page.Thumbnail}}
	albums := page.Albums
	if full, err := p.client.ArtistReleases(ctx, page.AlbumsMore); err == nil && len(full) > 0 {
		albums = full
	}
	singles := page.Singles
	if full, err := p.client.ArtistReleases(ctx, page.SinglesMore); err == nil && len(full) > 0 {
		singles = full
	}
	for _, it := range albums {
		out = append(out, provider.AlbumInfo{ID: it.ID, Name: it.Title, Artist: name, ArtistID: artistID, Year: it.Year, ImageURL: it.Thumbnail})
	}
	for _, it := range singles {
		out = append(out, provider.AlbumInfo{ID: it.ID, Name: it.Title + " · " + p.words.singles, Artist: name, ArtistID: artistID, Year: it.Year, ImageURL: it.Thumbnail, Single: true})
	}
	return append(out, p.localAlbumInfos(name)...), nil
}

// localAlbumInfos lists the local albums of an artist, matched by name.
func (p *Provider) localAlbumInfos(artist string) []provider.AlbumInfo {
	var out []provider.AlbumInfo
	for _, a := range p.opts.Local.ArtistAlbums(artist) {
		out = append(out, provider.AlbumInfo{ID: prefixLocalAlbum + a.Key, Name: a.Name + p.words.localTag, Artist: a.Artist, Year: a.Year})
	}
	return out
}

// favoriteSet returns the paths of the ♥ favorites.
func (p *Provider) favoriteSet() map[string]bool {
	set := map[string]bool{}
	if p.opts.Favorites == nil {
		return set
	}
	tracks, err := p.opts.Favorites.Tracks()
	if err != nil {
		return set
	}
	for _, t := range tracks {
		set[t.Path] = true
	}
	return set
}

// ToggleFavorite saves or removes an album from the library (f on an album)
// or subscribes to or unsubscribes from an artist (f on an artist), in the
// account. Saved albums and subscribed artists carry a ♥ and lead the lists.
func (p *Provider) ToggleFavorite(id string) (added bool, name string, err error) {
	if strings.HasPrefix(id, "local:") {
		return false, "", errors.New("local items have no account favorite: press f on their songs")
	}
	if !p.client.SignedIn() {
		return false, "", ErrSignInRequired
	}
	ctx, cancel := budget()
	defer cancel()
	on, _ := p.FavoriteState(id)
	added = !on
	switch {
	case strings.HasPrefix(id, "MPREb_"):
		page, err := p.client.Album(ctx, id)
		if err != nil {
			return false, "", err
		}
		playlistID, err := p.client.AlbumPlaylistID(ctx, id)
		if err != nil {
			return false, page.Title, err
		}
		if err := p.client.SetPlaylistSaved(ctx, playlistID, added); err != nil {
			return false, page.Title, err
		}
		name = page.Title
	case strings.HasPrefix(id, "UC"):
		if err := p.client.SetSubscribed(ctx, id, added); err != nil {
			return false, "", err
		}
		page, err := p.client.Artist(ctx, id)
		if err == nil {
			name = page.Name
		}
	default:
		return false, "", errors.New("only albums and artists can be favorited here")
	}
	p.mu.Lock()
	p.albumsAt, p.artistsAt = time.Time{}, time.Time{}
	if strings.HasPrefix(id, "MPREb_") {
		if p.savedAlbums == nil {
			p.savedAlbums = map[string]bool{}
		}
		p.savedAlbums[id] = added
	}
	if strings.HasPrefix(id, "UC") {
		if p.subscribed == nil {
			p.subscribed = map[string]bool{}
		}
		p.subscribed[id] = added
	}
	p.mu.Unlock()
	return added, name, nil
}

// FavoriteState reports whether an album (MPREb_) is saved or an artist (UC)
// followed in the account. It reads the library once when it has not yet.
// known is false when signed out and for other IDs.
func (p *Provider) FavoriteState(id string) (on, known bool) {
	album := strings.HasPrefix(id, "MPREb_")
	if !p.client.SignedIn() || !album && !strings.HasPrefix(id, "UC") {
		return false, false
	}
	state := func() (bool, bool) {
		p.mu.Lock()
		defer p.mu.Unlock()
		saved := p.subscribed
		if album {
			saved = p.savedAlbums
		}
		return saved[id], saved != nil
	}
	if on, known = state(); !known {
		if album {
			p.allAlbums()
		} else {
			_, _ = p.Artists() // it never fails: a failed read leaves the set empty
		}
		on, known = state()
	}
	return on, known
}

// ArtistForTrack resolves a YouTube Music track to its artist.
func (p *Provider) ArtistForTrack(track playlist.Track) (provider.ArtistInfo, bool) {
	id := track.Meta(MetaArtistID)
	if id == "" {
		return provider.ArtistInfo{}, false
	}
	name := track.Artist
	if i := strings.Index(name, ", "); i > 0 {
		name = name[:i]
	}
	return provider.ArtistInfo{ID: id, Name: name}, true
}

// AlbumForTrack resolves a YouTube Music song to its album.
func (p *Provider) AlbumForTrack(track playlist.Track) (provider.AlbumInfo, bool) {
	id := track.Meta(MetaAlbumID)
	if id == "" {
		return provider.AlbumInfo{}, false
	}
	return provider.AlbumInfo{ID: id, Name: track.Album, Artist: track.Artist, ArtistID: track.Meta(MetaArtistID)}, true
}

// --- favorites, history and playlists ---

// CanFavoriteTrack reports whether a ♥ on track goes to YouTube Music.
func (p *Provider) CanFavoriteTrack(track playlist.Track) bool {
	return trackVideoID(track) != ""
}

// SetTrackFavorite likes or un-likes the song in the account. Signed out,
// the ♥ stays local.
func (p *Provider) SetTrackFavorite(ctx context.Context, track playlist.Track, favorite bool) error {
	id := trackVideoID(track)
	if id == "" || !p.client.SignedIn() {
		return nil
	}
	if err := p.client.SetLike(ctx, id, favorite); err != nil {
		return err
	}
	p.mu.Lock()
	p.likedAt = time.Time{}
	p.mu.Unlock()
	return nil
}

// CanReportPlayback reports whether plays go to the account history.
func (p *Provider) CanReportPlayback(track playlist.Track) bool {
	return p.opts.ReportHistory && p.client.SignedIn() && trackVideoID(track) != ""
}

// ReportNowPlaying adds the song to the YouTube Music history.
func (p *Provider) ReportNowPlaying(track playlist.Track, _ time.Duration, _ bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return p.client.ReportPlayback(ctx, trackVideoID(track))
}

// ReportScrobble does nothing: the history entry is made at the start.
func (p *Provider) ReportScrobble(playlist.Track, time.Duration, time.Duration, bool) error {
	return nil
}

// CanRelate reports whether track has a song radio.
func (p *Provider) CanRelate(track playlist.Track) bool {
	return trackVideoID(track) != ""
}

// RelatedTracks returns up to n songs of the seed's song radio.
func (p *Provider) RelatedTracks(ctx context.Context, seed playlist.Track, n int) ([]playlist.Track, error) {
	id := trackVideoID(seed)
	if id == "" || n < 1 {
		return nil, nil
	}
	songs, err := p.client.Radio(ctx, id, "")
	if err != nil {
		return nil, err
	}
	var out []playlist.Track
	for _, t := range p.visibleTracks(songTracks(songs)) {
		if trackVideoID(t) == id {
			continue
		}
		out = append(out, t)
		if len(out) >= n {
			break
		}
	}
	return out, nil
}

// AddTrackToPlaylist adds a song to one of the account's playlists.
func (p *Provider) AddTrackToPlaylist(ctx context.Context, playlistID string, track playlist.Track) error {
	id := trackVideoID(track)
	if id == "" {
		return errors.New("only YouTube songs can go into a YouTube Music playlist")
	}
	return p.client.AddToPlaylist(ctx, strings.TrimPrefix(playlistID, prefixList), id)
}

// AddTracksToPlaylist adds songs to one of the account's playlists, 50 per
// request. Tracks that are not YouTube songs are skipped.
func (p *Provider) AddTracksToPlaylist(ctx context.Context, playlistID string, tracks []playlist.Track) (added, skipped int, err error) {
	var ids []string
	for _, t := range tracks {
		if id := trackVideoID(t); id != "" {
			ids = append(ids, id)
		} else {
			skipped++
		}
	}
	plain := strings.TrimPrefix(playlistID, prefixList)
	for start := 0; start < len(ids); start += 50 {
		chunk := ids[start:min(len(ids), start+50)]
		if err := p.client.AddToPlaylist(ctx, plain, chunk...); err != nil {
			return added, skipped, err
		}
		added += len(chunk)
	}
	return added, skipped, nil
}

// CanAddToPlaylist reports whether pl is one of the account's playlists.
func (p *Provider) CanAddToPlaylist(pl playlist.PlaylistInfo) bool {
	if !strings.HasPrefix(pl.ID, prefixList) || pl.Section != p.words.yourMusic {
		return false
	}
	id := strings.TrimPrefix(strings.TrimPrefix(pl.ID, prefixList), "VL")
	return strings.HasPrefix(id, "PL")
}

// CreatePlaylist creates a private playlist in the account.
func (p *Provider) CreatePlaylist(ctx context.Context, name string) (string, error) {
	id, err := p.client.CreatePlaylist(ctx, name)
	if err != nil {
		return "", err
	}
	p.mu.Lock()
	p.libraryAt = time.Time{}
	p.mu.Unlock()
	return prefixList + "VL" + id, nil
}

// --- search ---

// SearchTracks searches YouTube Music. A YouTube or YouTube Music link
// loads what it points to instead. Results come grouped: artists, albums
// and playlists first (they expand when chosen), then songs.
func (p *Provider) SearchTracks(ctx context.Context, query string, limit int) ([]playlist.Track, error) {
	if link, ok := ParseLink(query); ok {
		return p.openLink(ctx, link)
	}
	if limit <= 0 {
		limit = 30
	}
	filters := []SearchFilter{SearchArtists, SearchAlbums, SearchPlaylists, SearchSongs}
	limits := map[SearchFilter]int{SearchArtists: 3, SearchAlbums: 6, SearchPlaylists: 4, SearchSongs: limit}
	type result struct {
		shelves []Shelf
		err     error
	}
	results := make([]result, len(filters))
	var wg sync.WaitGroup
	for i, f := range filters {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sh, err := p.client.Search(ctx, query, f, limits[f])
			results[i] = result{sh, err}
		}()
	}
	wg.Wait()

	w := p.words
	var out []playlist.Track
	for _, t := range p.opts.Local.Search(query, 10) {
		if t.ProviderMeta == nil {
			t.ProviderMeta = map[string]string{}
		}
		t.ProviderMeta[MetaSection] = w.localMusic
		out = append(out, t)
	}
	var firstErr error
	for i, f := range filters {
		if results[i].err != nil {
			if firstErr == nil {
				firstErr = results[i].err
			}
			continue
		}
		count := 0
		for _, sh := range results[i].shelves {
			for _, it := range sh.Items {
				if count >= limits[f] {
					break
				}
				switch f {
				case SearchSongs:
					if it.Song == nil {
						continue
					}
					t := songTrack(*it.Song)
					t.ProviderMeta[MetaSection] = w.searchSongs
					out = append(out, t)
				case SearchArtists:
					t := placeholder(prefixArtist+it.ID, it.Title, "", w.searchArtists)
					t.AlbumArtURL = it.Thumbnail
					out = append(out, t)
				case SearchAlbums:
					t := placeholder(it.ID, it.Title, it.Artist, w.searchAlbums)
					t.Year = it.Year
					t.AlbumArtURL = it.Thumbnail
					out = append(out, t)
				case SearchPlaylists:
					t := placeholder(prefixList+it.ID, it.Title, it.Artist, w.searchPlaylists)
					t.AlbumArtURL = it.Thumbnail
					out = append(out, t)
				}
				count++
			}
		}
	}
	if len(out) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

// MetaSection names the search section of a result row.
const MetaSection = playlist.MetaSection

// placeholder is a search row that expands into tracks through AlbumTracks.
func placeholder(id, title, artist, section string) playlist.Track {
	return playlist.Track{
		Path:   "ytmusic:" + id,
		Title:  title,
		Artist: artist,
		Album:  title,
		ProviderMeta: map[string]string{
			playlist.MetaKind:    playlist.MetaKindAlbum,
			playlist.MetaAlbumID: id,
			MetaSection:          section,
		},
	}
}

// openLink loads the tracks a YouTube or YouTube Music link points to.
func (p *Provider) openLink(ctx context.Context, link Link) ([]playlist.Track, error) {
	if link.Kind == LinkUnknown {
		resolved, err := p.client.resolveLink(ctx, link)
		if err != nil {
			return nil, err
		}
		link = resolved
	}
	tracks, err := p.openLinkTracks(ctx, link)
	for i := range tracks {
		if tracks[i].ProviderMeta == nil {
			tracks[i].ProviderMeta = map[string]string{}
		}
		tracks[i].ProviderMeta[MetaSection] = p.words.searchLink
	}
	return tracks, err
}

// openLinkTracks resolves a parsed link to its tracks.
func (p *Provider) openLinkTracks(ctx context.Context, link Link) ([]playlist.Track, error) {
	switch link.Kind {
	case LinkVideo:
		songs, err := p.client.Radio(ctx, link.VideoID, "")
		if err == nil && len(songs) > 0 && songs[0].VideoID == link.VideoID {
			return []playlist.Track{songTrack(songs[0])}, nil
		}
		return []playlist.Track{{
			Path: WatchURL(link.VideoID), Title: link.VideoID, Stream: true,
			AlbumArtURL:  VideoThumbnail(link.VideoID),
			ProviderMeta: map[string]string{MetaVideoID: link.VideoID},
		}}, nil
	case LinkPlaylist:
		return p.listTracks(ctx, link.PlaylistID)
	case LinkAlbum:
		page, err := p.client.Album(ctx, link.BrowseID)
		return songTracks(page.Songs), err
	case LinkArtist:
		return p.artistTopTracks(ctx, link.BrowseID)
	}
	return nil, errors.New("youtube music: this link points to nothing playable")
}

// errNoSeed reports a row with no YouTube song to start a radio from.
var errNoSeed = errors.New("youtube music: no YouTube song to start from")

// RelatedToItem returns the radio of the first YouTube song of an album,
// artist or playlist row.
func (p *Provider) RelatedToItem(ctx context.Context, id string, n int) (playlist.Track, []playlist.Track, error) {
	var tracks []playlist.Track
	var err error
	switch {
	case strings.HasPrefix(id, prefixArtist):
		tracks, err = p.artistTopTracks(ctx, strings.TrimPrefix(id, prefixArtist))
	case strings.HasPrefix(id, "UC"):
		tracks, err = p.artistTopTracks(ctx, id)
	case strings.HasPrefix(id, "MPREb_"), strings.HasPrefix(id, prefixLocalAlbum), strings.HasPrefix(id, prefixLocalArtist):
		tracks, err = p.AlbumTracks(id)
	default:
		tracks, err = p.Tracks(id)
	}
	if err != nil {
		return playlist.Track{}, nil, err
	}
	for _, t := range tracks {
		if p.CanRelate(t) {
			related, err := p.RelatedTracks(ctx, t, n)
			return t, related, err
		}
	}
	return playlist.Track{}, nil, errNoSeed
}

// TrackLyrics returns the YouTube Music lyrics of a song as plain lines, or
// lyrics.ErrNotFound. YouTube Music has no timestamps for them, so synced
// lyrics from LRCLIB win when they exist.
func (p *Provider) TrackLyrics(ctx context.Context, track playlist.Track) ([]lyrics.Line, error) {
	id := trackVideoID(track)
	if id == "" {
		return nil, lyrics.ErrNotFound
	}
	text, _, err := p.client.Lyrics(ctx, id)
	if err != nil {
		return nil, err
	}
	var lines []lyrics.Line
	for raw := range strings.SplitSeq(strings.TrimSpace(text), "\n") {
		lines = append(lines, lyrics.Line{Text: strings.TrimSpace(raw)})
	}
	if len(lines) == 0 || strings.TrimSpace(text) == "" {
		return nil, lyrics.ErrNotFound
	}
	return lines, nil
}

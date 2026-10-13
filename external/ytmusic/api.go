package ytmusic

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// maxContinuations bounds how many pages one list request follows.
const maxContinuations = 60

// fetchContinuation fetches the next page of a browse or search list.
func (c *Client) fetchContinuation(ctx context.Context, endpoint, token string, newStyle bool) (map[string]any, error) {
	if newStyle {
		return c.call(ctx, endpoint, map[string]any{"continuation": token}, "")
	}
	return c.call(ctx, endpoint, map[string]any{}, "ctoken="+token+"&continuation="+token+"&type=next")
}

// followContinuations appends the pages after a first page of contents
// until the list ends or holds limit entries (0 = no limit).
func (c *Client) followContinuations(ctx context.Context, endpoint string, contents []any, token string, newStyle bool, limit int) []any {
	for page := 0; token != "" && page < maxContinuations; page++ {
		if limit > 0 && len(contents) >= limit {
			break
		}
		resp, err := c.fetchContinuation(ctx, endpoint, token, newStyle)
		if err != nil {
			break
		}
		var more []any
		more, token, newStyle = continuationPage(resp)
		if len(more) == 0 {
			break
		}
		contents = append(contents, more...)
	}
	return contents
}

// trimContinuation drops a trailing continuationItemRenderer from contents.
func trimContinuation(contents []any) []any {
	if n := len(contents); n > 0 && obj(contents[n-1], "continuationItemRenderer") != nil {
		return contents[:n-1]
	}
	return contents
}

// listContents returns the entries of a shelf-like renderer.
func listContents(r map[string]any) []any {
	if contents := arr(r, "contents"); contents != nil {
		return contents
	}
	return arr(r, "items")
}

// firstListRenderer returns the first shelf-like renderer of a section list
// and its contents.
func firstListRenderer(sl map[string]any) (map[string]any, []any) {
	for _, sec := range arr(sl, "contents") {
		candidates := append([]any{sec}, arr(sec, "itemSectionRenderer", "contents")...)
		for _, cand := range candidates {
			for _, name := range shelfRenderers {
				if r := obj(cand, name); r != nil {
					return r, listContents(r)
				}
			}
		}
	}
	return nil, nil
}

// browseList browses a page that holds one list, such as a library list or
// a playlist, and returns all its entries.
func (c *Client) browseList(ctx context.Context, browseID, params string, limit int) (map[string]any, []any, error) {
	resp, err := c.browse(ctx, browseID, params)
	if err != nil {
		return nil, nil, err
	}
	r, contents := firstListRenderer(sectionList(resp))
	if r == nil {
		return resp, nil, nil
	}
	token, newStyle := nextToken(r)
	contents = trimContinuation(contents)
	return resp, c.followContinuations(ctx, "browse", contents, token, newStyle, limit), nil
}

// Home returns the shelves of the YouTube Music home page, following its
// pages until it holds at least minShelves shelves.
func (c *Client) Home(ctx context.Context, minShelves int) ([]Shelf, error) {
	resp, err := c.browse(ctx, "FEmusic_home", "")
	if err != nil {
		return nil, err
	}
	sl := sectionList(resp)
	shelves := parseShelves(arr(sl, "contents"))
	token, newStyle := nextToken(sl)
	for page := 0; token != "" && page < 6 && len(shelves) < minShelves; page++ {
		next, err := c.fetchContinuation(ctx, "browse", token, newStyle)
		if err != nil {
			break
		}
		var contents []any
		contents, token, newStyle = continuationPage(next)
		if len(contents) == 0 {
			break
		}
		shelves = append(shelves, parseShelves(contents)...)
	}
	return shelves, nil
}

// PlaylistPage is a playlist with its songs.
type PlaylistPage struct {
	ID        string
	Title     string
	Author    string
	Length    string // "113 pistas • 4+ horas"
	Thumbnail string
	Songs     []Song
}

// Playlist returns a playlist and its songs, up to limit (0 = all). id may
// carry the VL browse prefix or not.
func (c *Client) Playlist(ctx context.Context, id string, limit int) (PlaylistPage, error) {
	playlistID := strings.TrimPrefix(id, "VL")
	resp, contents, err := c.browseList(ctx, "VL"+playlistID, "", limit)
	if err != nil {
		return PlaylistPage{}, err
	}
	page := PlaylistPage{ID: playlistID, Songs: parseSongs(contents)}
	if h := responsiveHeader(resp); h != nil {
		page.Title = text(h["title"])
		page.Author = text(h["straplineTextOne"])
		page.Length = text(h["secondSubtitle"])
		page.Thumbnail = thumbnailURL(h["thumbnail"])
	}
	if limit > 0 && len(page.Songs) > limit {
		page.Songs = page.Songs[:limit]
	}
	return page, nil
}

// responsiveHeader returns the header renderer of an album or playlist page.
func responsiveHeader(resp map[string]any) map[string]any {
	for _, name := range []string{"musicResponsiveHeaderRenderer", "musicDetailHeaderRenderer", "musicEditablePlaylistDetailHeaderRenderer"} {
		if h, ok := findFirst(resp, name).(map[string]any); ok {
			if inner := obj(h, "header", "musicResponsiveHeaderRenderer"); inner != nil {
				return inner
			}
			return h
		}
	}
	return nil
}

// AlbumPage is an album with its songs.
type AlbumPage struct {
	ID        string
	Title     string
	Artist    string
	ArtistID  string
	Year      int
	Kind      string // "Álbum", "Single", "EP"... as the header words it
	Length    string // "10 canciones • 40 minutos"
	Thumbnail string
	Songs     []Song
}

// Album returns an album (browse ID MPREb_...) and its songs.
func (c *Client) Album(ctx context.Context, browseID string) (AlbumPage, error) {
	resp, err := c.browse(ctx, browseID, "")
	if err != nil {
		return AlbumPage{}, err
	}
	page := AlbumPage{ID: browseID}
	if h := responsiveHeader(resp); h != nil {
		page.Title = text(h["title"])
		var credit Song
		creditRuns(arr(h, "straplineTextOne", "runs"), &credit)
		page.Artist, page.ArtistID = credit.ArtistName(), credit.ArtistID()
		var sub Song
		page.Year = creditRuns(arr(h, "subtitle", "runs"), &sub)
		page.Kind = strings.TrimSpace(strings.Split(text(h["subtitle"]), "•")[0])
		page.Length = text(h["secondSubtitle"])
		page.Thumbnail = thumbnailURL(h["thumbnail"])
	}
	r, contents := firstListRenderer(sectionList(resp))
	if r != nil {
		token, newStyle := nextToken(r)
		contents = c.followContinuations(ctx, "browse", trimContinuation(contents), token, newStyle, 0)
	}
	page.Songs = parseSongs(contents)
	for i := range page.Songs {
		s := &page.Songs[i]
		if len(s.Artists) == 0 && page.Artist != "" {
			s.Artists = []Artist{{Name: page.Artist, ID: page.ArtistID}}
		}
		if s.Album == "" {
			s.Album, s.AlbumID = page.Title, browseID
		}
		if s.Thumbnail == "" {
			s.Thumbnail = page.Thumbnail
		}
	}
	return page, nil
}

// ArtistPage is the page of an artist.
type ArtistPage struct {
	ID          string
	Name        string
	Thumbnail   string
	Listeners   string // "14,6 M usuarios mensuales"
	Subscribers string // "3,03 M"
	TopSongs    []Song
	SongsListID string // browse ID of the full song list, if any
	Albums      []Item
	AlbumsMore  Item // browse target of the full album list, if any
	Singles     []Item
	SinglesMore Item
	Playlists   []Item
	Related     []Item
	Videos      []Item
}

// Artist returns the page of an artist channel (UC...).
func (c *Client) Artist(ctx context.Context, channelID string) (ArtistPage, error) {
	resp, err := c.browse(ctx, channelID, "")
	if err != nil {
		return ArtistPage{}, err
	}
	page := ArtistPage{ID: channelID}
	for _, name := range []string{"musicImmersiveHeaderRenderer", "musicVisualHeaderRenderer", "musicResponsiveHeaderRenderer"} {
		if h := obj(resp, "header", name); h != nil {
			page.Name = text(h["title"])
			page.Thumbnail = thumbnailURL(h["thumbnail"], resp["microformat"])
			page.Listeners = text(h["monthlyListenerCount"])
			page.Subscribers = text(findFirst(h["subscriptionButton"], "subscriberCountText"))
			break
		}
	}
	albumShelves := 0
	for _, sec := range arr(sectionList(resp), "contents") {
		if r := obj(sec, "musicShelfRenderer"); r != nil && page.TopSongs == nil {
			page.TopSongs = parseSongs(arr(r, "contents"))
			page.SongsListID = str(r, "title", "runs", 0, "navigationEndpoint", "browseEndpoint", "browseId")
			if page.SongsListID == "" {
				page.SongsListID = str(r, "bottomEndpoint", "browseEndpoint", "browseId")
			}
			continue
		}
		sh, ok := parseShelf(sec)
		if !ok {
			continue
		}
		switch sh.Items[0].Kind {
		case KindAlbum:
			more := Item{ID: sh.MoreID, Params: sh.MoreParams}
			switch albumShelves {
			case 0:
				page.Albums, page.AlbumsMore = sh.Items, more
			case 1:
				page.Singles, page.SinglesMore = sh.Items, more
			}
			albumShelves++
		case KindPlaylist:
			page.Playlists = append(page.Playlists, sh.Items...)
		case KindArtist:
			page.Related = sh.Items
		case KindVideo, KindSong:
			if page.Videos == nil {
				page.Videos = sh.Items
			}
		}
	}
	for i := range page.TopSongs {
		if len(page.TopSongs[i].Artists) == 0 {
			page.TopSongs[i].Artists = []Artist{{Name: page.Name, ID: channelID}}
		}
	}
	return page, nil
}

// ArtistReleases returns the full album or single list behind an artist
// shelf's "more" button.
func (c *Client) ArtistReleases(ctx context.Context, more Item) ([]Item, error) {
	if more.ID == "" {
		return nil, nil
	}
	_, contents, err := c.browseList(ctx, more.ID, more.Params, 0)
	if err != nil {
		return nil, err
	}
	return parseItems(contents), nil
}

// LibraryPlaylists returns the playlists of the library.
func (c *Client) LibraryPlaylists(ctx context.Context) ([]Item, error) {
	return c.libraryItems(ctx, "FEmusic_liked_playlists")
}

// LibraryAlbums returns the saved albums of the library.
func (c *Client) LibraryAlbums(ctx context.Context) ([]Item, error) {
	return c.libraryItems(ctx, "FEmusic_liked_albums")
}

// LibraryArtists returns the artists of the library's songs.
func (c *Client) LibraryArtists(ctx context.Context) ([]Item, error) {
	return c.libraryItems(ctx, "FEmusic_library_corpus_track_artists")
}

// Subscriptions returns the subscribed artists.
func (c *Client) Subscriptions(ctx context.Context) ([]Item, error) {
	return c.libraryItems(ctx, "FEmusic_library_corpus_artists")
}

func (c *Client) libraryItems(ctx context.Context, browseID string) ([]Item, error) {
	if !c.SignedIn() {
		return nil, ErrSignInRequired
	}
	_, contents, err := c.browseList(ctx, browseID, "", 0)
	if err != nil {
		return nil, err
	}
	return parseItems(contents), nil
}

// LikedSongs returns the "Liked music" playlist.
func (c *Client) LikedSongs(ctx context.Context, limit int) ([]Song, error) {
	if !c.SignedIn() {
		return nil, ErrSignInRequired
	}
	page, err := c.Playlist(ctx, "LM", limit)
	return page.Songs, err
}

// History returns the songs of the YouTube Music history, newest first.
func (c *Client) History(ctx context.Context) ([]Song, error) {
	if !c.SignedIn() {
		return nil, ErrSignInRequired
	}
	resp, err := c.browse(ctx, "FEmusic_history", "")
	if err != nil {
		return nil, err
	}
	var songs []Song
	for _, sec := range arr(sectionList(resp), "contents") {
		songs = append(songs, parseSongs(arr(sec, "musicShelfRenderer", "contents"))...)
	}
	return songs, nil
}

// Category is one entry of the "Moods & genres" page.
type Category struct {
	Title   string
	Section string // "Géneros", "Estados de ánimo y momentos", ...
	Params  string
}

// Categories returns the moods and genres of YouTube Music.
func (c *Client) Categories(ctx context.Context) ([]Category, error) {
	resp, err := c.browse(ctx, "FEmusic_moods_and_genres", "")
	if err != nil {
		return nil, err
	}
	var out []Category
	for _, sec := range arr(sectionList(resp), "contents") {
		grid := obj(sec, "gridRenderer")
		if grid == nil {
			continue
		}
		section := text(obj(grid, "header", "gridHeaderRenderer", "title"))
		for _, it := range arr(grid, "items") {
			b := obj(it, "musicNavigationButtonRenderer")
			params := str(b, "clickCommand", "browseEndpoint", "params")
			if b == nil || params == "" {
				continue
			}
			out = append(out, Category{Title: text(b["buttonText"]), Section: section, Params: params})
		}
	}
	return out, nil
}

// CategoryPage returns the shelves of one mood or genre.
func (c *Client) CategoryPage(ctx context.Context, params string) ([]Shelf, error) {
	resp, err := c.browse(ctx, "FEmusic_moods_and_genres_category", params)
	if err != nil {
		return nil, err
	}
	return parseShelves(arr(sectionList(resp), "contents")), nil
}

// SearchFilter selects one kind of search result.
type SearchFilter string

const (
	SearchAll       SearchFilter = ""
	SearchSongs     SearchFilter = "songs"
	SearchVideos    SearchFilter = "videos"
	SearchAlbums    SearchFilter = "albums"
	SearchArtists   SearchFilter = "artists"
	SearchPlaylists SearchFilter = "playlists"
)

// searchParams are the protobuf params that select one result kind, as
// music.youtube.com sends them for its filter chips.
var searchParams = map[SearchFilter]string{
	SearchSongs:     "EgWKAQIIAWoMEA4QChADEAQQCRAF",
	SearchVideos:    "EgWKAQIQAWoMEA4QChADEAQQCRAF",
	SearchAlbums:    "EgWKAQIYAWoMEA4QChADEAQQCRAF",
	SearchArtists:   "EgWKAQIgAWoMEA4QChADEAQQCRAF",
	SearchPlaylists: "Eg-KAQwIABAAGAAgACgBMABqChAEEAMQCRAFEAo=",
}

// Search runs a search. With a filter it returns one shelf of that kind,
// following its pages up to limit results; without one it returns the
// shelves of the mixed result page.
func (c *Client) Search(ctx context.Context, query string, filter SearchFilter, limit int) ([]Shelf, error) {
	body := map[string]any{"query": query}
	if p := searchParams[filter]; p != "" {
		body["params"] = p
	}
	resp, err := c.call(ctx, "search", body, "")
	if err != nil {
		return nil, err
	}
	sl := sectionList(resp)
	if filter == SearchAll {
		return parseShelves(arr(sl, "contents")), nil
	}
	r, contents := firstListRenderer(sl)
	if r == nil {
		return nil, nil
	}
	token, newStyle := nextToken(r)
	contents = c.followContinuations(ctx, "search", trimContinuation(contents), token, newStyle, limit)
	items := parseItems(contents)
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return []Shelf{{Title: text(r["title"]), Items: items}}, nil
}

// Radio returns the "song radio" queue of a video: songs like it, as the
// YouTube Music player continues after a song. The seed comes first.
func (c *Client) Radio(ctx context.Context, videoID, playlistID string) ([]Song, error) {
	if playlistID == "" {
		playlistID = "RDAMVM" + videoID
	}
	body := map[string]any{
		"enablePersistentPlaylistPanel": true,
		"isAudioOnly":                   true,
		"tunerSettingValue":             "AUTOMIX_SETTING_NORMAL",
		"playlistId":                    playlistID,
		"watchEndpointMusicSupportedConfigs": map[string]any{
			"watchEndpointMusicConfig": map[string]any{
				"hasPersistentPlaylistPanel": true,
				"musicVideoType":             "MUSIC_VIDEO_TYPE_ATV",
			},
		},
	}
	if videoID != "" {
		body["videoId"] = videoID
	}
	resp, err := c.call(ctx, "next", body, "")
	if err != nil {
		return nil, err
	}
	panel, _ := findFirst(resp, "playlistPanelRenderer").(map[string]any)
	if panel == nil {
		return nil, errors.New("youtube music: no radio for this song")
	}
	return parseSongs(arr(panel, "contents")), nil
}

// SetLike likes or un-likes a song in the account.
func (c *Client) SetLike(ctx context.Context, videoID string, like bool) error {
	if !c.SignedIn() {
		return ErrSignInRequired
	}
	endpoint := "like/removelike"
	if like {
		endpoint = "like/like"
	}
	_, err := c.call(ctx, endpoint, map[string]any{"target": map[string]any{"videoId": videoID}}, "")
	return err
}

// SetDislike marks a song "dislike" in the account, which YouTube Music
// uses to recommend less like it.
func (c *Client) SetDislike(ctx context.Context, videoID string) error {
	if !c.SignedIn() {
		return ErrSignInRequired
	}
	_, err := c.call(ctx, "like/dislike", map[string]any{"target": map[string]any{"videoId": videoID}}, "")
	return err
}

// AlbumPlaylistID returns the playlist ID (OLAK5uy_...) behind an album,
// which saving the album to the library acts on.
func (c *Client) AlbumPlaylistID(ctx context.Context, browseID string) (string, error) {
	resp, err := c.browse(ctx, browseID, "")
	if err != nil {
		return "", err
	}
	if id, _ := findFirst(resp, "audioPlaylistId").(string); id != "" {
		return id, nil
	}
	for _, v := range findAll(resp, "playlistId", 0) {
		if id, _ := v.(string); strings.HasPrefix(id, "OLAK5uy_") {
			return id, nil
		}
	}
	return "", errors.New("youtube music: this album has no playlist to save")
}

// SetPlaylistSaved saves a playlist or album to the library, or removes it.
func (c *Client) SetPlaylistSaved(ctx context.Context, playlistID string, saved bool) error {
	if !c.SignedIn() {
		return ErrSignInRequired
	}
	endpoint := "like/removelike"
	if saved {
		endpoint = "like/like"
	}
	_, err := c.call(ctx, endpoint, map[string]any{"target": map[string]any{"playlistId": strings.TrimPrefix(playlistID, "VL")}}, "")
	return err
}

// SetSubscribed subscribes to an artist channel, or unsubscribes.
func (c *Client) SetSubscribed(ctx context.Context, channelID string, on bool) error {
	if !c.SignedIn() {
		return ErrSignInRequired
	}
	endpoint := "subscription/unsubscribe"
	if on {
		endpoint = "subscription/subscribe"
	}
	_, err := c.call(ctx, endpoint, map[string]any{"channelIds": []string{channelID}}, "")
	return err
}

// AddToPlaylist adds songs to one of the account's playlists. Songs already
// in the playlist are skipped.
func (c *Client) AddToPlaylist(ctx context.Context, playlistID string, videoIDs ...string) error {
	if !c.SignedIn() {
		return ErrSignInRequired
	}
	actions := make([]any, 0, len(videoIDs))
	for _, id := range videoIDs {
		actions = append(actions, map[string]any{
			"action":       "ACTION_ADD_VIDEO",
			"addedVideoId": id,
			"dedupeOption": "DEDUPE_OPTION_SKIP",
		})
	}
	resp, err := c.call(ctx, "browse/edit_playlist", map[string]any{
		"playlistId": strings.TrimPrefix(playlistID, "VL"),
		"actions":    actions,
	}, "")
	if err != nil {
		return err
	}
	if status := str(resp, "status"); status != "" && status != "STATUS_SUCCEEDED" {
		return fmt.Errorf("youtube music: adding to the playlist failed (%s)", status)
	}
	return nil
}

// CreatePlaylist creates a private playlist and returns its ID.
func (c *Client) CreatePlaylist(ctx context.Context, title string) (string, error) {
	if !c.SignedIn() {
		return "", ErrSignInRequired
	}
	resp, err := c.call(ctx, "playlist/create", map[string]any{
		"title":         title,
		"description":   "",
		"privacyStatus": "PRIVATE",
	}, "")
	if err != nil {
		return "", err
	}
	id := str(resp, "playlistId")
	if id == "" {
		return "", errors.New("youtube music: the new playlist has no ID")
	}
	return id, nil
}

// Lyrics returns the plain lyrics of a song and their source line, or ""
// when YouTube Music has none.
func (c *Client) Lyrics(ctx context.Context, videoID string) (lyrics, source string, err error) {
	resp, err := c.call(ctx, "next", map[string]any{"videoId": videoID, "isAudioOnly": true}, "")
	if err != nil {
		return "", "", err
	}
	var browseID string
	for _, tab := range arr(findFirst(resp, "watchNextTabbedResultsRenderer"), "tabs") {
		be := obj(tab, "tabRenderer", "endpoint", "browseEndpoint")
		if pageType(be) == "TRACK_LYRICS" {
			browseID = str(be, "browseId")
		}
	}
	if browseID == "" {
		return "", "", nil
	}
	page, err := c.browse(ctx, browseID, "")
	if err != nil {
		return "", "", err
	}
	shelf, _ := findFirst(page, "musicDescriptionShelfRenderer").(map[string]any)
	return text(shelf["description"]), text(shelf["footer"]), nil
}

// ReportPlayback adds a song to the account's watch history, as the YouTube
// Music player does when a song starts.
func (c *Client) ReportPlayback(ctx context.Context, videoID string) error {
	if !c.SignedIn() {
		return ErrSignInRequired
	}
	resp, err := c.call(ctx, "player", map[string]any{"videoId": videoID}, "")
	if err != nil {
		return err
	}
	base := str(resp, "playbackTracking", "videostatsPlaybackUrl", "baseUrl")
	if base == "" {
		return errors.New("youtube music: no playback tracking URL")
	}
	sep := "&"
	if !strings.Contains(base, "?") {
		sep = "?"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+sep+"ver=2&c=WEB_REMIX&cpn="+cpn(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	if cookie := c.auth.CookieHeader(); cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	if authz := c.auth.AuthorizationHeader(time.Now()); authz != "" {
		req.Header.Set("Authorization", authz)
		req.Header.Set("X-Goog-AuthUser", "0")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("youtube music: history report HTTP %d", res.StatusCode)
	}
	return nil
}

// cpn returns a random client playback nonce: 16 characters of the URL-safe
// base64 alphabet.
func cpn() string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

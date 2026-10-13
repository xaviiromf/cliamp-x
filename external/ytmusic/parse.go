package ytmusic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Artist is one credited artist of a song or album.
type Artist struct {
	Name string
	ID   string // channel browse ID (UC...), or "" when the credit has no page
}

// Song is one playable YouTube Music song or video.
type Song struct {
	VideoID    string
	Title      string
	Artists    []Artist
	Album      string
	AlbumID    string // album browse ID (MPREb_...)
	Duration   int    // seconds, 0 when unknown
	Liked      bool
	SetVideoID string // the playlist entry ID, for playlist edits
	Thumbnail  string // cover or video thumbnail URL
	Plays      string // play count as YouTube Music words it, such as "283 M plays"
}

// ArtistName returns the credited artists joined with ", ".
func (s Song) ArtistName() string {
	names := make([]string, 0, len(s.Artists))
	for _, a := range s.Artists {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return strings.Join(names, ", ")
}

// ArtistID returns the first credited artist with a page.
func (s Song) ArtistID() string {
	for _, a := range s.Artists {
		if a.ID != "" {
			return a.ID
		}
	}
	return ""
}

// ItemKind is the kind of a browse card.
type ItemKind uint8

const (
	KindSong ItemKind = iota + 1
	KindVideo
	KindAlbum
	KindPlaylist
	KindArtist
)

// Item is one card of a shelf: a song, an album, a playlist or an artist.
type Item struct {
	Kind      ItemKind
	ID        string // browse ID, or the video ID of a song
	Params    string // browse params, for pages such as an artist's albums
	Title     string
	Subtitle  string
	Artist    string
	ArtistID  string
	Year      int
	Thumbnail string // cover, picture or thumbnail URL
	Song      *Song  // set for KindSong and KindVideo
}

// Shelf is one titled row of cards, such as "Quick picks".
type Shelf struct {
	Title      string
	Items      []Item
	MoreID     string // browse ID of the shelf's "more" page, if any
	MoreParams string
}

var (
	durationRe = regexp.MustCompile(`^(\d{1,2}:)?\d{1,2}:\d{2}$`)
	yearRe     = regexp.MustCompile(`^(19|20)\d{2}$`)
)

// thumbnailURL picks the picture of an item from the thumbnail lists under
// sources: the largest square googleusercontent picture, the album art YouTube
// Music shows, else the largest thumbnail. An artist page puts its square
// picture in a second source, beside a wide banner. A googleusercontent
// picture is asked for thumbnailWidth pixels: clients scale it again by its
// URL.
func thumbnailURL(sources ...any) string {
	best, bestW, bestSquare := "", -1, false
	for _, o := range sources {
		list, _ := findFirst(o, "thumbnails").([]any)
		for _, t := range list {
			url := str(t, "url")
			if url == "" {
				continue
			}
			if strings.HasPrefix(url, "//") {
				url = "https:" + url
			}
			w, _ := get(t, "width").(float64)
			h, _ := get(t, "height").(float64)
			square := w > 0 && w == h && googleImage(url)
			if (square && !bestSquare) || (square == bestSquare && int(w) > bestW) {
				best, bestW, bestSquare = url, int(w), square
			}
		}
	}
	return resizeThumbnail(best, thumbnailWidth)
}

// thumbnailWidth is the picture size the preview pane wants: sharp in a
// terminal that draws images, and still a small download.
const thumbnailWidth = 400

// thumbSize matches the "=w120-h120" size of a googleusercontent image URL.
var thumbSize = regexp.MustCompile(`=w(\d+)-h(\d+)`)

// googleImage reports whether url is a googleusercontent picture, which the
// server scales to the size in the URL.
func googleImage(url string) bool {
	return strings.Contains(url, "googleusercontent.com/") || strings.Contains(url, "ggpht.com/")
}

// resizeThumbnail asks a googleusercontent image URL for a picture whose
// longest side is size pixels, keeping its aspect. The server scales the
// image to the size in the URL, so a small listed thumbnail can be fetched
// sharp. Other URLs are returned unchanged.
func resizeThumbnail(url string, size int) string {
	if !googleImage(url) {
		return url
	}
	m := thumbSize.FindStringSubmatchIndex(url)
	if m == nil {
		return url
	}
	w, _ := strconv.Atoi(url[m[2]:m[3]])
	h, _ := strconv.Atoi(url[m[4]:m[5]])
	if w <= 0 || h <= 0 {
		return url
	}
	long := max(w, h)
	w, h = max(1, w*size/long), max(1, h*size/long)
	return url[:m[0]] + fmt.Sprintf("=w%d-h%d", w, h) + url[m[1]:]
}

// isPlays reports whether a subtitle segment counts plays or views.
func isPlays(seg string) bool {
	lower := strings.ToLower(seg)
	for _, w := range []string{"reproduccion", "visualizacion", "views", "plays", "vistas"} {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// parseDuration turns "3:21" or "1:02:03" into seconds.
func parseDuration(s string) int {
	s = strings.TrimSpace(s)
	if !durationRe.MatchString(s) {
		return 0
	}
	total := 0
	for _, part := range strings.Split(s, ":") {
		n, _ := strconv.Atoi(part)
		total = total*60 + n
	}
	return total
}

// pageType returns the page type of a browse endpoint, without its prefix:
// ALBUM, PLAYLIST, ARTIST, USER_CHANNEL, LIBRARY_ARTIST and so on.
func pageType(browseEndpoint map[string]any) string {
	pt := str(browseEndpoint, "browseEndpointContextSupportedConfigs", "browseEndpointContextMusicConfig", "pageType")
	return strings.TrimPrefix(pt, "MUSIC_PAGE_TYPE_")
}

func isArtistPage(pt string) bool {
	return pt == "ARTIST" || pt == "USER_CHANNEL" || pt == "LIBRARY_ARTIST"
}

// channelID turns a library artist ID (MPLAUC...) into the channel ID.
func channelID(id string) string {
	return strings.TrimPrefix(id, "MPLA")
}

// creditRuns reads artists, album, duration and year from the runs of a
// subtitle or a byline. The runs that link to pages are reliable; plain text
// is used only for an artist without a page.
func creditRuns(runs []any, s *Song) (year int) {
	var plain []string
	for _, r := range runs {
		t := str(r, "text")
		be := obj(r, "navigationEndpoint", "browseEndpoint")
		switch pt := pageType(be); {
		case be != nil && isArtistPage(pt):
			s.Artists = append(s.Artists, Artist{Name: t, ID: channelID(str(be, "browseId"))})
		case be != nil && pt == "ALBUM":
			s.Album, s.AlbumID = t, str(be, "browseId")
		case durationRe.MatchString(strings.TrimSpace(t)):
			if s.Duration == 0 {
				s.Duration = parseDuration(t)
			}
		case yearRe.MatchString(strings.TrimSpace(t)):
			year, _ = strconv.Atoi(strings.TrimSpace(t))
		default:
			plain = append(plain, t)
		}
	}
	if len(s.Artists) == 0 {
		if name := plainArtist(strings.Join(plain, "")); name != "" {
			s.Artists = []Artist{{Name: name}}
		}
	}
	for _, seg := range strings.Split(strings.Join(plain, ""), "•") {
		if seg = strings.TrimSpace(seg); s.Plays == "" && isPlays(seg) {
			s.Plays = seg
		}
	}
	return year
}

// typeLabels are the item-type words YouTube Music puts first in a subtitle,
// in the languages cliamp-x users are likely to run.
var typeLabels = map[string]bool{
	"canción": true, "song": true, "vídeo": true, "video": true, "episodio": true, "episode": true,
	"álbum": true, "album": true, "single": true, "sencillo": true, "ep": true, "artista": true,
	"artist": true, "lista de reproducción": true, "playlist": true, "música": true, "chanson": true,
	"titre": true, "lied": true, "brano": true, "música de": true,
}

// statWords mark a subtitle segment that counts plays, views or listeners.
var statWords = []string{
	"reproduccion", "visualizacion", "vista", "views", "plays", "suscriptor", "subscriber",
	"usuarios mensuales", "monthly", "oyentes", "listeners", "canciones", "songs", "pistas", "tracks",
}

// plainArtist picks the artist out of a plain "Song • Artist • 3:21" text.
func plainArtist(s string) string {
	for _, seg := range strings.Split(s, "•") {
		seg = strings.TrimSpace(seg)
		if seg == "" || typeLabels[strings.ToLower(seg)] || durationRe.MatchString(seg) || yearRe.MatchString(seg) {
			continue
		}
		lower := strings.ToLower(seg)
		stat := false
		for _, w := range statWords {
			if strings.Contains(lower, w) {
				stat = true
				break
			}
		}
		if !stat {
			return seg
		}
	}
	return ""
}

// likeStatus reports whether a renderer's menu marks the song as liked.
func likeStatus(r map[string]any) bool {
	for _, v := range findAll(r["menu"], "likeStatus", 3) {
		if s, _ := v.(string); s == "LIKE" {
			return true
		}
	}
	return false
}

func greyedOut(r map[string]any) bool {
	return str(r, "musicItemRendererDisplayPolicy") == "MUSIC_ITEM_RENDERER_DISPLAY_POLICY_GREY_OUT"
}

// flexColumn returns the text object of flex column i.
func flexColumn(r map[string]any, i int) map[string]any {
	return obj(r, "flexColumns", i, "musicResponsiveListItemFlexColumnRenderer", "text")
}

// parseSongRow parses a musicResponsiveListItemRenderer that is a song or
// video row. It returns false for rows that are not playable songs.
func parseSongRow(r map[string]any) (Song, bool) {
	if r == nil || greyedOut(r) {
		return Song{}, false
	}
	var s Song
	s.VideoID = str(r, "playlistItemData", "videoId")
	s.SetVideoID = str(r, "playlistItemData", "playlistSetVideoId")
	title := flexColumn(r, 0)
	s.Title = text(title)
	if s.VideoID == "" {
		s.VideoID = str(title, "runs", 0, "navigationEndpoint", "watchEndpoint", "videoId")
	}
	if s.VideoID == "" {
		s.VideoID = str(findFirst(r["overlay"], "watchEndpoint"), "videoId")
	}
	if s.VideoID == "" || s.Title == "" {
		return Song{}, false
	}
	var runs []any
	for i := 1; i < len(arr(r, "flexColumns")); i++ {
		runs = append(runs, arr(flexColumn(r, i), "runs")...)
		runs = append(runs, map[string]any{"text": " • "})
	}
	creditRuns(runs, &s)
	if fixed := text(obj(r, "fixedColumns", 0, "musicResponsiveListItemFixedColumnRenderer", "text")); fixed != "" {
		if d := parseDuration(fixed); d > 0 {
			s.Duration = d
		}
	}
	s.Liked = likeStatus(r)
	s.Thumbnail = thumbnailURL(r["thumbnail"])
	return s, true
}

// parsePanelSong parses a playlistPanelVideoRenderer of a watch queue.
func parsePanelSong(r map[string]any) (Song, bool) {
	if r == nil {
		return Song{}, false
	}
	s := Song{VideoID: str(r, "videoId"), Title: text(r["title"]), SetVideoID: str(r, "playlistSetVideoId")}
	if s.VideoID == "" || s.Title == "" {
		return Song{}, false
	}
	creditRuns(arr(r, "longBylineText", "runs"), &s)
	if d := parseDuration(text(r["lengthText"])); d > 0 {
		s.Duration = d
	}
	s.Liked = likeStatus(r)
	s.Thumbnail = thumbnailURL(r["thumbnail"])
	return s, true
}

// parseTwoRow parses a musicTwoRowItemRenderer card.
func parseTwoRow(r map[string]any) (Item, bool) {
	if r == nil {
		return Item{}, false
	}
	it := Item{Title: text(r["title"]), Subtitle: text(r["subtitle"]), Thumbnail: thumbnailURL(r["thumbnailRenderer"])}
	nav := obj(r, "navigationEndpoint")
	var credit Song
	it.Year = creditRuns(arr(r, "subtitle", "runs"), &credit)
	it.Artist, it.ArtistID = credit.ArtistName(), credit.ArtistID()
	if be := obj(nav, "browseEndpoint"); be != nil {
		it.ID = str(be, "browseId")
		it.Params = str(be, "params")
		switch pt := pageType(be); {
		case pt == "ALBUM" || pt == "AUDIOBOOK":
			it.Kind = KindAlbum
		case pt == "PLAYLIST":
			it.Kind = KindPlaylist
		case isArtistPage(pt):
			it.Kind = KindArtist
			it.ID = channelID(it.ID)
			it.Artist, it.ArtistID = it.Title, it.ID
		default:
			return Item{}, false
		}
		return it, it.ID != "" && it.Title != ""
	}
	if we := obj(nav, "watchEndpoint"); we != nil {
		credit.VideoID = str(we, "videoId")
		credit.Title = it.Title
		credit.Thumbnail = it.Thumbnail
		if credit.VideoID == "" {
			return Item{}, false
		}
		it.Kind = KindVideo
		if str(we, "watchEndpointMusicSupportedConfigs", "watchEndpointMusicConfig", "musicVideoType") == "MUSIC_VIDEO_TYPE_ATV" {
			it.Kind = KindSong
		}
		it.ID = credit.VideoID
		it.Song = &credit
		return it, true
	}
	if wpe := obj(nav, "watchPlaylistEndpoint"); wpe != nil {
		it.Kind = KindPlaylist
		it.ID = "VL" + str(wpe, "playlistId")
		return it, it.ID != "VL" && it.Title != ""
	}
	return Item{}, false
}

// parseListItem parses a musicResponsiveListItemRenderer that may be a song
// or a link to an album, a playlist or an artist, as search results and the
// library lists use.
func parseListItem(r map[string]any) (Item, bool) {
	if s, ok := parseSongRow(r); ok {
		kind := KindSong
		if s.Album == "" {
			kind = KindVideo
		}
		return Item{Kind: kind, ID: s.VideoID, Title: s.Title, Artist: s.ArtistName(), ArtistID: s.ArtistID(), Thumbnail: s.Thumbnail, Song: &s}, true
	}
	be := obj(r, "navigationEndpoint", "browseEndpoint")
	if be == nil {
		return Item{}, false
	}
	it := Item{ID: str(be, "browseId"), Params: str(be, "params"), Title: text(flexColumn(r, 0)), Subtitle: text(flexColumn(r, 1)), Thumbnail: thumbnailURL(r["thumbnail"])}
	var credit Song
	it.Year = creditRuns(arr(flexColumn(r, 1), "runs"), &credit)
	it.Artist, it.ArtistID = credit.ArtistName(), credit.ArtistID()
	switch pt := pageType(be); {
	case pt == "ALBUM" || pt == "AUDIOBOOK":
		it.Kind = KindAlbum
	case pt == "PLAYLIST":
		it.Kind = KindPlaylist
	case isArtistPage(pt):
		it.Kind = KindArtist
		it.ID = channelID(it.ID)
		it.Artist, it.ArtistID = it.Title, it.ID
	default:
		return Item{}, false
	}
	return it, it.ID != "" && it.Title != ""
}

// parseItems parses the cards of a shelf, grid or list.
func parseItems(contents []any) []Item {
	items := make([]Item, 0, len(contents))
	for _, c := range contents {
		if r := obj(c, "musicTwoRowItemRenderer"); r != nil {
			if it, ok := parseTwoRow(r); ok {
				items = append(items, it)
			}
			continue
		}
		if r := obj(c, "musicResponsiveListItemRenderer"); r != nil {
			if it, ok := parseListItem(r); ok {
				items = append(items, it)
			}
		}
	}
	return items
}

// parseSongs parses the song rows of a list, skipping everything else.
func parseSongs(contents []any) []Song {
	songs := make([]Song, 0, len(contents))
	for _, c := range contents {
		if r := obj(c, "musicResponsiveListItemRenderer"); r != nil {
			if s, ok := parseSongRow(r); ok {
				songs = append(songs, s)
			}
			continue
		}
		if r := obj(c, "playlistPanelVideoRenderer"); r != nil {
			if s, ok := parsePanelSong(r); ok {
				songs = append(songs, s)
			}
		}
	}
	return songs
}

// shelfRenderers are the renderers that hold one titled list of cards.
var shelfRenderers = []string{
	"musicCarouselShelfRenderer",
	"musicShelfRenderer",
	"gridRenderer",
	"musicPlaylistShelfRenderer",
	"musicImmersiveCarouselShelfRenderer",
}

// parseShelf parses one section of a section list. ok is false for sections
// that hold no cards.
func parseShelf(section any) (Shelf, bool) {
	for _, name := range shelfRenderers {
		r := obj(section, name)
		if r == nil {
			continue
		}
		sh := Shelf{Title: text(r["title"])}
		header := obj(r, "header")
		for _, h := range []string{"musicCarouselShelfBasicHeaderRenderer", "gridHeaderRenderer", "musicImmersiveCarouselShelfBasicHeaderRenderer"} {
			if hr := obj(header, h); hr != nil {
				if sh.Title == "" {
					sh.Title = text(hr["title"])
				}
				more := obj(hr, "moreContentButton", "buttonRenderer", "navigationEndpoint", "browseEndpoint")
				if more == nil {
					more = obj(hr, "title", "runs", 0, "navigationEndpoint", "browseEndpoint")
				}
				sh.MoreID, sh.MoreParams = str(more, "browseId"), str(more, "params")
			}
		}
		if sh.MoreID == "" {
			more := obj(r, "title", "runs", 0, "navigationEndpoint", "browseEndpoint")
			sh.MoreID, sh.MoreParams = str(more, "browseId"), str(more, "params")
		}
		contents := arr(r, "contents")
		if contents == nil {
			contents = arr(r, "items")
		}
		sh.Items = parseItems(contents)
		return sh, len(sh.Items) > 0
	}
	if r := obj(section, "musicCardShelfRenderer"); r != nil {
		// The top result of a search: the card itself, then its rows.
		sh := Shelf{Title: text(obj(r, "header", "musicCardShelfHeaderBasicRenderer", "title"))}
		titleRun := obj(r, "title", "runs", 0)
		if be := obj(titleRun, "navigationEndpoint", "browseEndpoint"); be != nil {
			it := Item{ID: str(be, "browseId"), Title: text(r["title"]), Subtitle: text(r["subtitle"])}
			switch pt := pageType(be); {
			case isArtistPage(pt):
				it.Kind, it.ID, it.Artist = KindArtist, channelID(it.ID), it.Title
				it.ArtistID = it.ID
			case pt == "ALBUM":
				it.Kind = KindAlbum
			case pt == "PLAYLIST":
				it.Kind = KindPlaylist
			}
			if it.Kind != 0 {
				sh.Items = append(sh.Items, it)
			}
		} else if we := obj(titleRun, "navigationEndpoint", "watchEndpoint"); we != nil {
			s := Song{VideoID: str(we, "videoId"), Title: text(r["title"])}
			creditRuns(arr(r, "subtitle", "runs"), &s)
			if s.VideoID != "" {
				sh.Items = append(sh.Items, Item{Kind: KindSong, ID: s.VideoID, Title: s.Title, Artist: s.ArtistName(), ArtistID: s.ArtistID(), Song: &s})
			}
		}
		sh.Items = append(sh.Items, parseItems(arr(r, "contents"))...)
		return sh, len(sh.Items) > 0
	}
	return Shelf{}, false
}

// sectionList finds the section list of a browse or search response, either
// in the single-column layout or in the two-column layout of playlist and
// album pages.
func sectionList(resp map[string]any) map[string]any {
	contents := obj(resp, "contents")
	for _, path := range [][]any{
		{"singleColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer"},
		{"twoColumnBrowseResultsRenderer", "secondaryContents", "sectionListRenderer"},
		{"tabbedSearchResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer"},
		{"sectionListRenderer"},
	} {
		if sl := obj(contents, path...); sl != nil {
			return sl
		}
	}
	return nil
}

// parseShelves parses every shelf of a section list's contents.
func parseShelves(sections []any) []Shelf {
	var out []Shelf
	for _, sec := range sections {
		if sh, ok := parseShelf(sec); ok {
			out = append(out, sh)
		}
		// An item section can wrap a shelf.
		for _, inner := range arr(sec, "itemSectionRenderer", "contents") {
			if sh, ok := parseShelf(inner); ok {
				out = append(out, sh)
			}
		}
	}
	return out
}

// nextToken returns the continuation token of a renderer in either layout:
// the old continuations array, or a trailing continuationItemRenderer in its
// contents. newStyle reports which request form the token needs.
func nextToken(r map[string]any) (token string, newStyle bool) {
	if t := str(r, "continuations", 0, "nextContinuationData", "continuation"); t != "" {
		return t, false
	}
	contents := arr(r, "contents")
	if contents == nil {
		contents = arr(r, "items")
	}
	if last := obj(get(contents, -1), "continuationItemRenderer"); last != nil {
		if t := str(last, "continuationEndpoint", "continuationCommand", "token"); t != "" {
			return t, true
		}
	}
	return "", false
}

// continuationPage returns the items and the next token of a continuation
// response in either layout.
func continuationPage(resp map[string]any) (contents []any, token string, newStyle bool) {
	if cc := obj(resp, "continuationContents"); cc != nil {
		for _, v := range cc {
			m, _ := v.(map[string]any)
			if m == nil {
				continue
			}
			contents = arr(m, "contents")
			if contents == nil {
				contents = arr(m, "items")
			}
			token, newStyle = nextToken(m)
			return contents, token, newStyle
		}
	}
	for _, action := range arr(resp, "onResponseReceivedActions") {
		items := arr(action, "appendContinuationItemsAction", "continuationItems")
		if items == nil {
			continue
		}
		if last := obj(get(items, -1), "continuationItemRenderer"); last != nil {
			token = str(last, "continuationEndpoint", "continuationCommand", "token")
			items = items[:len(items)-1]
		}
		return items, token, true
	}
	return nil, "", false
}

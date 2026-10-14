// Package ipc provides Unix socket IPC for remote playback control of cliamp.
// The protocol is newline-delimited JSON over a Unix domain socket.
package ipc

// Request is the parameter object decoded from a V2 operation's params field.
// It is an in-process adapter while runtime owners migrate to narrower typed
// operation structs; it is never sent as a top-level protocol envelope.
type Request struct {
	Cmd      string      `json:"-"`
	Value    float64     `json:"value,omitempty"`
	Playlist string      `json:"playlist,omitempty"`
	Path     string      `json:"path,omitempty"`
	Name     string      `json:"name,omitempty"`
	Band     int         `json:"band,omitempty"`
	Sub      string      `json:"sub,omitempty"`
	Args     []string    `json:"args,omitempty"`
	Provider string      `json:"provider,omitempty"`
	Query    string      `json:"query,omitempty"`
	Artist   string      `json:"artist,omitempty"`
	Album    string      `json:"album,omitempty"`
	Sort     string      `json:"sort,omitempty"`
	Offset   int         `json:"offset,omitempty"`
	Index    int         `json:"index,omitempty"`
	To       int         `json:"to,omitempty"`
	Limit    int         `json:"limit,omitempty"`
	Revision uint64      `json:"if_revision,omitempty"`
	NewName  string      `json:"new_name,omitempty"`
	Track    *TrackInfo  `json:"track,omitempty"`
	Tracks   []TrackInfo `json:"tracks,omitempty"`
	Topics   []string    `json:"topics,omitempty"`
	Play     bool        `json:"play,omitempty"`
	Kind     string      `json:"kind,omitempty"` // what taste.rate rates: song, album, artist or genre
	Text     string      `json:"text,omitempty"` // the value of a settings.set name that takes text
}

// Response is the operation-specific data embedded in a successful V2 job.
// V2Response and Job carry protocol success and failure state.
type Response struct {
	OK         bool           `json:"ok"`
	Error      string         `json:"error,omitempty"`
	State      string         `json:"state,omitempty"`
	Track      *TrackInfo     `json:"track,omitempty"`
	Position   float64        `json:"position,omitempty"`
	Duration   float64        `json:"duration,omitempty"`
	Volume     float64        `json:"volume,omitempty"`
	Playlist   string         `json:"playlist,omitempty"`
	Index      int            `json:"index,omitempty"`
	Total      int            `json:"total,omitempty"`
	Visualizer string         `json:"visualizer,omitempty"`
	Shuffle    *bool          `json:"shuffle,omitempty"`
	Repeat     string         `json:"repeat,omitempty"`
	Mono       *bool          `json:"mono,omitempty"`
	Speed      float64        `json:"speed,omitempty"`
	EQPreset   string         `json:"eq_preset,omitempty"`
	Device     string         `json:"device,omitempty"`
	Output     string         `json:"output,omitempty"`
	Items      []string       `json:"items,omitempty"`
	Theme      *ThemeInfo     `json:"theme,omitempty"`
	Bands      []float64      `json:"bands,omitempty"`
	EQBands    []float64      `json:"eq_bands,omitempty"`
	Tracks     []TrackInfo    `json:"tracks,omitempty"`
	Playlists  []PlaylistInfo `json:"playlists,omitempty"`
	Providers  []ProviderInfo `json:"providers,omitempty"`
	Artists    []ArtistInfo   `json:"artists,omitempty"`
	Albums     []AlbumInfo    `json:"albums,omitempty"`
	Sorts      []SortInfo     `json:"sorts,omitempty"`
	Lyrics     []LyricLine    `json:"lyrics,omitempty"`
	History    []HistoryInfo  `json:"history,omitempty"`
	Devices    []DeviceInfo   `json:"devices,omitempty"`
	Favorite   *bool          `json:"favorite,omitempty"`
	Themes     []ThemeInfo    `json:"themes,omitempty"`
	Shelves    []ShelfInfo    `json:"shelves,omitempty"`
	Genres     []GenreInfo    `json:"genres,omitempty"`
	Preview    *PreviewInfo   `json:"preview,omitempty"`
	Ratings    []RatingInfo   `json:"ratings,omitempty"`
	Downloads  *DownloadsInfo `json:"downloads,omitempty"`
	Stats      *StatsInfo     `json:"stats,omitempty"`
	URL        string         `json:"url,omitempty"`   // the page that lastfm.auth start opens
	Token      string         `json:"token,omitempty"` // the token that lastfm.auth finish takes
}

// ThemeInfo carries the active theme name and its resolved hex colors.
// Empty hex fields mean the default (ANSI fallback) theme is active.
type ThemeInfo struct {
	Name     string `json:"name"`
	BG       string `json:"bg,omitempty"`
	Accent   string `json:"accent,omitempty"`
	Fg       string `json:"fg,omitempty"`
	BrightFg string `json:"bright_fg,omitempty"`
	Green    string `json:"green,omitempty"`
	Yellow   string `json:"yellow,omitempty"`
	Red      string `json:"red,omitempty"`
}

// TrackInfo is the track metadata in a status response.
type TrackInfo struct {
	Title         string            `json:"title,omitempty"`
	Artist        string            `json:"artist,omitempty"`
	Album         string            `json:"album,omitempty"`
	Genre         string            `json:"genre,omitempty"`
	Path          string            `json:"path"`
	AlbumArtURL   string            `json:"album_art_url,omitempty"`
	Year          int               `json:"year,omitempty"`
	TrackNumber   int               `json:"track_number,omitempty"`
	DurationSecs  int               `json:"duration_secs,omitempty"`
	Index         int               `json:"index,omitempty"`
	QueuePosition int               `json:"queue_position,omitempty"`
	Stream        bool              `json:"stream,omitempty"`
	StreamTitle   string            `json:"stream_title,omitempty"`
	Station       string            `json:"station,omitempty"`
	Realtime      bool              `json:"realtime,omitempty"`
	Restricted    bool              `json:"restricted,omitempty"`
	Feed          bool              `json:"feed,omitempty"`
	Bookmark      bool              `json:"bookmark,omitempty"`
	Unplayable    bool              `json:"unplayable,omitempty"`
	DirSourced    bool              `json:"dir_sourced,omitempty"`
	ProviderMeta  map[string]string `json:"provider_meta,omitempty"`
}

type PlaylistInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Section      string `json:"section,omitempty"`
	TrackCount   int    `json:"track_count,omitempty"`
	DurationSecs int    `json:"duration_secs,omitempty"`
	Favoritable  bool   `json:"favoritable,omitempty"`
	Favorite     bool   `json:"favorite,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
	Writable     bool   `json:"writable,omitempty"` // takes new tracks with playlist.add
	Unseen       bool   `json:"unseen,omitempty"`   // a new release not opened yet
}

type ProviderInfo struct {
	Key           string `json:"key"`
	Name          string `json:"name"`
	Searchable    bool   `json:"searchable"`
	BrowseArtists bool   `json:"browse_artists,omitempty"`
	BrowseAlbums  bool   `json:"browse_albums,omitempty"`
	Catalog       bool   `json:"catalog,omitempty"`
	SignedIn      *bool  `json:"signed_in,omitempty"`
}

type ArtistInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AlbumCount int    `json:"album_count,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	Favorite   bool   `json:"favorite,omitempty"` // followed in the account
}

type AlbumInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Artist     string `json:"artist,omitempty"`
	ArtistID   string `json:"artist_id,omitempty"`
	Year       int    `json:"year,omitempty"`
	TrackCount int    `json:"track_count,omitempty"`
	Genre      string `json:"genre,omitempty"`
	ImageURL   string `json:"image_url,omitempty"`
	Favorite   bool   `json:"favorite,omitempty"` // saved in the account
	Single     bool   `json:"single,omitempty"`
}

// ShelfInfo is one titled row of the provider.home cards.
type ShelfInfo struct {
	Title string          `json:"title"`
	Items []ShelfItemInfo `json:"items"`
}

// ShelfItemInfo is one card of a shelf. Kind is song, album, playlist or
// artist. A song carries its track.
type ShelfItemInfo struct {
	Kind     string     `json:"kind"`
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Subtitle string     `json:"subtitle,omitempty"`
	ImageURL string     `json:"image_url,omitempty"`
	Track    *TrackInfo `json:"track,omitempty"`
}

// GenreInfo is one category of provider.genres.
type GenreInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Group    string `json:"group,omitempty"`
	Favorite bool   `json:"favorite,omitempty"`
}

// PreviewInfo is the picture and the detail lines of an album, artist or
// playlist. Favorite is set when the provider knows whether the item is
// saved or followed.
type PreviewInfo struct {
	ImageURL string   `json:"image_url,omitempty"`
	Lines    []string `json:"lines,omitempty"`
	Favorite *bool    `json:"favorite,omitempty"`
}

// RatingInfo is one rating of taste.list. Interest is 1 for interested and
// -1 for not interested; At is RFC 3339.
type RatingInfo struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Artist   string `json:"artist,omitempty"`
	Interest int    `json:"interest"`
	At       string `json:"at"`
}

// DownloadsInfo is the offline download queue of download.status. Done
// counts the songs downloaded since the queue last ran empty.
type DownloadsInfo struct {
	Running   bool              `json:"running,omitempty"`
	Current   *TrackInfo        `json:"current,omitempty"`
	Percent   float64           `json:"percent,omitempty"`
	Queued    []TrackInfo       `json:"queued,omitempty"`
	Done      int               `json:"done,omitempty"`
	Failed    []DownloadFailure `json:"failed,omitempty"`
	Directory string            `json:"directory,omitempty"`
}

// DownloadFailure is a song whose download failed, with the reason.
type DownloadFailure struct {
	Track TrackInfo `json:"track"`
	Error string    `json:"error"`
}

// StatsInfo is the listening summary of one period. Daily holds the minutes
// of each of the last 30 days, oldest first.
type StatsInfo struct {
	Plays      int          `json:"plays"`
	Secs       float64      `json:"secs"`
	Songs      int          `json:"songs"`
	Artists    int          `json:"artists"`
	TopArtists []StatsCount `json:"top_artists,omitempty"`
	TopSongs   []StatsCount `json:"top_songs,omitempty"`
	TopGenres  []StatsCount `json:"top_genres,omitempty"`
	Daily      []float64    `json:"daily"`
	Streak     int          `json:"streak"`
	// The wrapped name fills these: the year, the minutes listened, the
	// days with plays, the top albums, the first song of the year and the
	// song replayed most in one day.
	Year         int          `json:"year,omitempty"`
	Minutes      int          `json:"minutes,omitempty"`
	Days         int          `json:"days,omitempty"`
	TopAlbums    []StatsCount `json:"top_albums,omitempty"`
	First        *StatsPlay   `json:"first,omitempty"`
	MostReplayed *StatsCount  `json:"most_replayed,omitempty"`
}

// StatsPlay is one listened song: its title, its artist in Detail, its path
// and when it played, in RFC 3339.
type StatsPlay struct {
	Name   string `json:"name"`
	Detail string `json:"detail,omitempty"`
	Path   string `json:"path,omitempty"`
	At     string `json:"at"`
}

// StatsCount is one ranked artist, song or genre. Detail is a song's artist.
type StatsCount struct {
	Name   string  `json:"name"`
	Detail string  `json:"detail,omitempty"`
	Plays  int     `json:"plays"`
	Secs   float64 `json:"secs"`
}

type SortInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type LyricLine struct {
	Start float64 `json:"start"`
	Text  string  `json:"text"`
}

type HistoryInfo struct {
	Track    TrackInfo `json:"track"`
	PlayedAt string    `json:"played_at"`
}

type DeviceInfo struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// Package locallib indexes the music files of some folders by their tags, so
// local albums, artists and genres can sit next to the YouTube Music ones in
// the same menus. The index is cached and refreshed incrementally: only
// files whose size or modification time changed are read again.
package locallib

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/internal/appdir"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// maxFiles bounds one scan, so a huge or wrong folder cannot stall the UI.
const maxFiles = 50000

// rescanAfter is how long a scan result stays fresh within one run.
const rescanAfter = 10 * time.Minute

// entry is one indexed file.
type entry struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	Title   string `json:"title"`
	Artist  string `json:"artist,omitempty"`
	Album   string `json:"album,omitempty"`
	Genre   string `json:"genre,omitempty"`
	Year    int    `json:"year,omitempty"`
	Number  int    `json:"track,omitempty"`
}

// Album is one local album.
type Album struct {
	Key    string // stable ID: artist and album name
	Name   string
	Artist string
	Year   int
	Tracks []playlist.Track
}

// Library is the index of a list of music folders. A file under two of
// them, as in nested folders, is indexed once. A nil Library is empty.
type Library struct {
	dirs      []string
	indexPath string

	mu        sync.Mutex
	entries   []entry
	scannedAt time.Time
}

// DefaultDir returns the user's music folder: $XDG_MUSIC_DIR, the MUSIC entry
// of user-dirs.dirs, or ~/Music. It returns "" when none exists.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidates := []string{os.Getenv("XDG_MUSIC_DIR")}
	if dir := userDirsMusic(home); dir != "" {
		candidates = append(candidates, dir)
	}
	candidates = append(candidates, filepath.Join(home, "Music"), filepath.Join(home, "Música"))
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

// userDirsMusic reads XDG_MUSIC_DIR from ~/.config/user-dirs.dirs.
func userDirsMusic(home string) string {
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	f, err := os.Open(filepath.Join(cfg, "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		value, ok := strings.CutPrefix(line, "XDG_MUSIC_DIR=")
		if !ok {
			continue
		}
		value = strings.Trim(value, `"`)
		value = strings.ReplaceAll(value, "$HOME", home)
		if value == home || value == home+"/" {
			return ""
		}
		return value
	}
	return ""
}

// New returns the library of dirs. "off" as the only folder gives nil.
func New(dirs ...string) *Library {
	if len(dirs) == 1 && strings.EqualFold(strings.TrimSpace(dirs[0]), "off") {
		return nil
	}
	l := &Library{dirs: CleanDirs(dirs)}
	if data, err := appdir.DataDir(); err == nil {
		l.indexPath = filepath.Join(data, "local_library.json")
	}
	return l
}

// CleanDirs expands ~ and cleans each folder, and drops the empty ones and
// the repeats.
func CleanDirs(dirs []string) []string {
	out := []string{}
	for _, d := range dirs {
		if d = strings.TrimSpace(d); d == "" {
			continue
		}
		d = filepath.Clean(expandHome(d))
		if !slices.Contains(out, d) {
			out = append(out, d)
		}
	}
	return out
}

func expandHome(dir string) string {
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, dir[2:])
		}
	}
	return dir
}

// Dirs returns the indexed folders.
func (l *Library) Dirs() []string {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.dirs)
}

// SetDirs indexes dirs from now on. The next read scans them, and reuses
// the index of the files it already read. A nil Library stays empty.
func (l *Library) SetDirs(dirs []string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.dirs = CleanDirs(dirs)
	l.scannedAt = time.Time{}
	l.mu.Unlock()
}

// snapshot returns the index, scanning the folder when the index is stale.
func (l *Library) snapshot() []entry {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.scannedAt.IsZero() || time.Since(l.scannedAt) > rescanAfter {
		l.scanLocked()
	}
	return l.entries
}

// Refresh forgets the scan time, so the next read checks the folder again.
func (l *Library) Refresh() {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.scannedAt = time.Time{}
	l.mu.Unlock()
}

func (l *Library) loadIndexLocked() map[string]entry {
	cached := map[string]entry{}
	if l.indexPath == "" {
		return cached
	}
	data, err := os.ReadFile(l.indexPath)
	if err != nil {
		return cached
	}
	var list []entry
	if json.Unmarshal(data, &list) != nil {
		return cached
	}
	for _, e := range list {
		cached[e.Path] = e
	}
	return cached
}

func (l *Library) scanLocked() {
	start := time.Now()
	cached := map[string]entry{}
	if l.entries == nil {
		cached = l.loadIndexLocked()
	} else {
		for _, e := range l.entries {
			cached[e.Path] = e
		}
	}
	var out []entry
	seen := map[string]bool{}
	read := 0
	for _, root := range l.dirs {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if len(out) >= maxFiles || seen[path] || !playlist.IsAudioFile(path) {
				return nil
			}
			seen[path] = true
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if e, ok := cached[path]; ok && e.Size == info.Size() && e.ModTime == info.ModTime().Unix() {
				out = append(out, e)
				return nil
			}
			t := playlist.TrackFromPath(path)
			read++
			out = append(out, entry{
				Path: path, Size: info.Size(), ModTime: info.ModTime().Unix(),
				Title: t.Title, Artist: t.Artist, Album: t.Album, Genre: t.Genre, Year: t.Year, Number: t.TrackNumber,
			})
			return nil
		})
	}
	l.entries = out
	l.scannedAt = time.Now()
	if read > 0 || len(out) != len(cached) {
		l.saveIndexLocked()
	}
	applog.Info("local library: %d files (%d read) in %s", len(out), read, time.Since(start).Round(time.Millisecond))
}

func (l *Library) saveIndexLocked() {
	if l.indexPath == "" {
		return
	}
	data, err := json.Marshal(l.entries)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(l.indexPath), 0o755); err != nil {
		return
	}
	tmp := l.indexPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, l.indexPath)
}

// albumName is the album of an entry: its tag, or else its folder.
func (e entry) albumName() string {
	if e.Album != "" {
		return e.Album
	}
	return filepath.Base(filepath.Dir(e.Path))
}

func (e entry) track() playlist.Track {
	return playlist.Track{
		Path: e.Path, Title: e.Title, Artist: e.Artist, Album: e.albumName(),
		Genre: e.Genre, Year: e.Year, TrackNumber: e.Number,
	}
}

// Tracks returns every indexed track.
func (l *Library) Tracks() []playlist.Track {
	entries := l.snapshot()
	out := make([]playlist.Track, len(entries))
	for i, e := range entries {
		out[i] = e.track()
	}
	return out
}

func fold(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// Albums returns the local albums, sorted by artist and name.
func (l *Library) Albums() []Album {
	byKey := map[string]*Album{}
	var keys []string
	for _, e := range l.snapshot() {
		name := e.albumName()
		key := fold(e.Artist) + "|" + fold(name)
		a, ok := byKey[key]
		if !ok {
			a = &Album{Key: key, Name: name, Artist: e.Artist, Year: e.Year}
			byKey[key] = a
			keys = append(keys, key)
		}
		if a.Year == 0 {
			a.Year = e.Year
		}
		a.Tracks = append(a.Tracks, e.track())
	}
	out := make([]Album, 0, len(keys))
	for _, k := range keys {
		a := byKey[k]
		sortAlbumTracks(a.Tracks)
		out = append(out, *a)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if fold(out[i].Artist) != fold(out[j].Artist) {
			return fold(out[i].Artist) < fold(out[j].Artist)
		}
		return fold(out[i].Name) < fold(out[j].Name)
	})
	return out
}

func sortAlbumTracks(tracks []playlist.Track) {
	sort.SliceStable(tracks, func(i, j int) bool {
		if tracks[i].TrackNumber != tracks[j].TrackNumber {
			return tracks[i].TrackNumber < tracks[j].TrackNumber
		}
		return tracks[i].Path < tracks[j].Path
	})
}

// Album returns the album with the given key.
func (l *Library) Album(key string) (Album, bool) {
	for _, a := range l.Albums() {
		if a.Key == key {
			return a, true
		}
	}
	return Album{}, false
}

// Artists returns the local artist names with their track counts.
func (l *Library) Artists() map[string]int {
	out := map[string]int{}
	names := map[string]string{}
	for _, e := range l.snapshot() {
		if e.Artist == "" {
			continue
		}
		k := fold(e.Artist)
		if _, ok := names[k]; !ok {
			names[k] = e.Artist
		}
		out[names[k]]++
	}
	return out
}

// ArtistAlbums returns the local albums of an artist, matched by name.
func (l *Library) ArtistAlbums(artist string) []Album {
	var out []Album
	for _, a := range l.Albums() {
		if fold(a.Artist) == fold(artist) {
			out = append(out, a)
		}
	}
	return out
}

// Genres returns the local genre names with their track counts.
func (l *Library) Genres() map[string]int {
	out := map[string]int{}
	for _, e := range l.snapshot() {
		if e.Genre != "" {
			out[e.Genre]++
		}
	}
	return out
}

// GenreTracks returns the local tracks of a genre.
func (l *Library) GenreTracks(genre string) []playlist.Track {
	var out []playlist.Track
	for _, e := range l.snapshot() {
		if fold(e.Genre) == fold(genre) {
			out = append(out, e.track())
		}
	}
	return out
}

// Search returns up to limit local tracks whose title, artist or album
// contains every word of query.
func (l *Library) Search(query string, limit int) []playlist.Track {
	words := strings.Fields(fold(query))
	if len(words) == 0 {
		return nil
	}
	var out []playlist.Track
	for _, e := range l.snapshot() {
		hay := fold(e.Title + " " + e.Artist + " " + e.albumName())
		match := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				match = false
				break
			}
		}
		if match {
			out = append(out, e.track())
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}

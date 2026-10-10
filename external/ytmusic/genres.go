package ytmusic

import (
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/xaviiromf/cliamp-x/internal/appdir"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

const genreFileName = "ytmusic_genres.json"

// genreFavorites are the genres the user starred, kept in the config
// directory as {"params": "title"}.
type genreFavorites struct {
	mu   sync.Mutex
	path string
	set  map[string]string
}

func loadGenreFavorites() *genreFavorites {
	g := &genreFavorites{set: map[string]string{}}
	dir, err := appdir.Dir()
	if err != nil {
		return g
	}
	g.path = filepath.Join(dir, genreFileName)
	if data, err := os.ReadFile(g.path); err == nil {
		_ = json.Unmarshal(data, &g.set)
	}
	return g
}

func (g *genreFavorites) has(params string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.set[params]
	return ok
}

func (g *genreFavorites) toggle(params, title string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, on := g.set[params]
	if on {
		delete(g.set, params)
	} else {
		g.set[params] = title
	}
	if g.path == "" {
		return !on, nil
	}
	data, err := json.MarshalIndent(g.set, "", "  ")
	if err != nil {
		return !on, err
	}
	if err := os.MkdirAll(filepath.Dir(g.path), 0o700); err != nil {
		return !on, err
	}
	return !on, os.WriteFile(g.path, append(data, '\n'), 0o600)
}

func (g *genreFavorites) snapshot() map[string]string {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make(map[string]string, len(g.set))
	for k, v := range g.set {
		out[k] = v
	}
	return out
}

// moodWords mark the "Moods & moments" section in the languages YouTube
// Music shows. Every other section holds genres.
var moodWords = []string{"ánimo", "animo", "mood", "momento", "moment", "humeur", "stimmung", "humor"}

func isMoodSection(title string) bool {
	lower := strings.ToLower(title)
	for _, w := range moodWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// favoriteGenres returns the starred genres and moods, in catalog order.
func (p *Provider) favoriteGenres() []Category {
	starred := p.genres.snapshot()
	if len(starred) == 0 {
		return nil
	}
	var out []Category
	seen := map[string]bool{}
	for _, c := range p.cachedCategories() {
		if _, ok := starred[c.Params]; ok && !seen[c.Params] {
			seen[c.Params] = true
			out = append(out, c)
		}
	}
	// A starred category the catalog no longer lists keeps its saved title.
	var rest []Category
	for params, title := range starred {
		if !seen[params] {
			rest = append(rest, Category{Title: title, Params: params})
		}
	}
	sort.Slice(rest, func(i, j int) bool { return rest[i].Title < rest[j].Title })
	return append(out, rest...)
}

// genreBrowser is one category catalog: the genres or the moods.
type genreBrowser struct {
	p     *Provider
	moods bool
}

var (
	_ provider.GenreBrowser         = genreBrowser{}
	_ provider.GenreFavoriteToggler = genreBrowser{}
	_ provider.GenreLabeler         = genreBrowser{}
)

// Genres lists the categories of the catalog, starred ones first.
func (b genreBrowser) Genres() ([]provider.GenreInfo, error) {
	var fav, rest []provider.GenreInfo
	seen := map[string]bool{}
	for _, c := range b.p.cachedCategories() {
		if isMoodSection(c.Section) != b.moods || seen[c.Params] {
			continue
		}
		seen[c.Params] = true
		info := provider.GenreInfo{ID: c.Params, Name: c.Title, Group: c.Section, Favorite: b.p.genres.has(c.Params)}
		if info.Favorite {
			fav = append(fav, info)
		} else {
			rest = append(rest, info)
		}
	}
	out := append(fav, rest...)
	if !b.moods {
		names := make([]string, 0)
		for name := range b.p.opts.Local.Genres() {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			out = append(out, provider.GenreInfo{ID: prefixLocalGenre + name, Name: name + b.p.words.localTag, Group: b.p.words.localMusic})
		}
	}
	return out, nil
}

// GenreSortTypes lists what a category can play.
func (b genreBrowser) GenreSortTypes() []provider.SortType {
	w := b.p.words
	return []provider.SortType{
		{ID: "mix", Label: w.genreMix},
		{ID: "songs", Label: w.genreSongs},
		{ID: "featured", Label: w.genreFeatured},
		{ID: "community", Label: w.genreCommunity},
	}
}

func (b genreBrowser) GenreTracks(genreID, sortType string) ([]playlist.Track, error) {
	return b.p.GenreTracks(genreID, sortType)
}

func (b genreBrowser) GenreLabel() string {
	if b.moods {
		return b.p.words.moodLabel
	}
	return b.p.words.genreLabel
}

// ToggleGenreFavorite stars or un-stars a category. Starred categories come
// first in the catalog and get a row in the provider pane.
func (b genreBrowser) ToggleGenreFavorite(genreID string) (bool, error) {
	title := genreID
	for _, c := range b.p.cachedCategories() {
		if c.Params == genreID {
			title = c.Title
			break
		}
	}
	return b.p.genres.toggle(genreID, title)
}

// The provider is the genre catalog itself; GenreBrowserFor picks the moods.

func (p *Provider) Genres() ([]provider.GenreInfo, error) { return genreBrowser{p: p}.Genres() }
func (p *Provider) GenreSortTypes() []provider.SortType   { return genreBrowser{p: p}.GenreSortTypes() }
func (p *Provider) GenreLabel() string                    { return genreBrowser{p: p}.GenreLabel() }

func (p *Provider) ToggleGenreFavorite(id string) (bool, error) {
	return genreBrowser{p: p}.ToggleGenreFavorite(id)
}

// GenreBrowserFor returns the catalog behind a pane entry.
func (p *Provider) GenreBrowserFor(entryID string) provider.GenreBrowser {
	return genreBrowser{p: p, moods: entryID == browseMoods}
}

// GenreTracks plays a category: its popular songs, its featured or
// community playlist, or a shuffled mix of the songs and the featured list.
func (p *Provider) GenreTracks(genreID, sortType string) ([]playlist.Track, error) {
	if name, ok := strings.CutPrefix(genreID, prefixLocalGenre); ok {
		tracks := p.opts.Local.GenreTracks(name)
		if sortType == "mix" {
			rand.Shuffle(len(tracks), func(i, j int) { tracks[i], tracks[j] = tracks[j], tracks[i] })
		}
		return tracks, nil
	}
	ctx, cancel := budget()
	defer cancel()
	shelves, err := p.client.CategoryPage(ctx, genreID)
	if err != nil {
		return nil, err
	}
	var songs []Song
	var lists []Item
	for _, sh := range shelves {
		if len(sh.Items) == 0 {
			continue
		}
		switch sh.Items[0].Kind {
		case KindSong, KindVideo:
			if songs == nil {
				for _, it := range sh.Items {
					if it.Song != nil {
						songs = append(songs, *it.Song)
					}
				}
			}
		case KindPlaylist:
			lists = append(lists, sh.Items[0])
		}
	}
	playlistSongs := func(i int) []Song {
		if i >= len(lists) {
			return nil
		}
		page, err := p.client.Playlist(ctx, lists[i].ID, 100)
		if err != nil {
			return nil
		}
		return page.Songs
	}
	switch sortType {
	case "songs":
		return songTracks(songs), nil
	case "featured":
		return songTracks(playlistSongs(0)), nil
	case "community":
		if len(lists) > 1 {
			return songTracks(playlistSongs(1)), nil
		}
		return songTracks(playlistSongs(0)), nil
	default:
		mix := append(append([]Song(nil), songs...), playlistSongs(0)...)
		rand.Shuffle(len(mix), func(i, j int) { mix[i], mix[j] = mix[j], mix[i] })
		return songTracks(mix), nil
	}
}

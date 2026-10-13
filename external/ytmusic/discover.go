package ytmusic

import (
	"context"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
)

// Discovery: the "For you" album list and the artist list grow without end
// as the listener scrolls. Each round opens the pages of a few seed artists
// (rated "interested", subscribed, liked or recently played), takes their
// related artists and the albums of both, and drops what the taste store
// hides. The related artists become later seeds, so the lists keep going. A
// refresh starts over with a new random order, so the lists change.

const (
	sortForYou = "foryou"
	// discoverSeeds is how many seed artists one round opens.
	discoverSeeds = 3
	// discoverRelated is how many related artists one round opens.
	discoverRelated = 6
	// discoverRounds bounds the rounds one page may run.
	discoverRounds = 4
	// discoverParallel bounds the artist pages fetched at once.
	discoverParallel = 6
)

// discovery is the state of the endless lists since the last refresh.
type discovery struct {
	rng   *rand.Rand
	init  bool
	epoch uint64 // the refresh the lists belong to

	seeds     []string        // artist channel IDs still to open
	seedSeen  map[string]bool // every artist queued as a seed
	albumSeen map[string]bool
	artSeen   map[string]bool

	baseAlbums  []provider.AlbumInfo // the library list, shuffled for this round
	baseArtists []provider.ArtistInfo
	albums      []provider.AlbumInfo // the "For you" list built so far
	artists     []provider.ArtistInfo
	nextAlbum   int // next base album to mix in
	nextArtist  int
	exhausted   bool
}

// resetDiscovery makes the next read build new lists. It does not wait for
// a round in flight, because Refresh runs on the UI goroutine.
func (p *Provider) resetDiscovery() { p.discEpoch.Add(1) }

// discoveryLocked returns the lists of the current refresh, started when
// needed. The caller holds discMu.
func (p *Provider) discoveryLocked() *discovery {
	d := &p.disc
	if e := p.discEpoch.Load(); !d.init || d.epoch != e {
		*d = discovery{epoch: e}
		p.startDiscoveryLocked(d)
	}
	return d
}

// hidden reports whether the taste store hides an item. ids are tried as
// given and with each pane prefix, so a rating made in the pane or in the
// browser applies to both.
func (p *Provider) hidden(texts []string, ids ...string) bool {
	if p.opts.Taste == nil {
		return false
	}
	var all []string
	for _, id := range ids {
		if id == "" {
			continue
		}
		all = append(all, id, prefixAlbum+id, prefixArtist+id, prefixList+id, prefixGenre+id)
	}
	return p.opts.Taste.Hidden(all, texts...)
}

func (p *Provider) albumHidden(a provider.AlbumInfo) bool {
	return p.hidden([]string{a.Name, a.Artist, a.Genre}, a.ID, a.ArtistID)
}

func (p *Provider) artistHidden(a provider.ArtistInfo) bool {
	return p.hidden([]string{a.Name}, a.ID)
}

// visibleTracks drops the songs the taste store hides.
func (p *Provider) visibleTracks(tracks []playlist.Track) []playlist.Track {
	if p.opts.Taste == nil {
		return tracks
	}
	out := tracks[:0:0]
	for _, t := range tracks {
		if !p.hidden([]string{t.Title, t.Artist, t.Album, t.Genre}, t.Path, t.Meta(MetaArtistID), t.Meta(MetaAlbumID)) {
			out = append(out, t)
		}
	}
	return out
}

// startDiscoveryLocked seeds a new round of the lists.
func (p *Provider) startDiscoveryLocked(d *discovery) {
	d.init = true
	d.rng = rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), 0x5eed))
	d.seedSeen, d.albumSeen, d.artSeen = map[string]bool{}, map[string]bool{}, map[string]bool{}

	// The library lists, best first, with the order inside each tenth
	// shuffled: a refresh shows other albums near the top.
	for _, a := range p.allAlbums() {
		if !p.albumHidden(a) {
			d.baseAlbums = append(d.baseAlbums, a)
		}
	}
	shuffleBands(d.rng, d.baseAlbums)
	arts, _ := p.Artists()
	for _, a := range arts {
		if !p.artistHidden(a) {
			d.baseArtists = append(d.baseArtists, a)
		}
	}
	shuffleBands(d.rng, d.baseArtists)

	// Seeds: the artists rated "interested" first, then the library's, in a
	// random order.
	var liked []string
	if p.opts.Taste != nil {
		for _, id := range p.opts.Taste.Liked(taste.Artist) {
			liked = append(liked, strings.TrimPrefix(id, prefixArtist))
		}
		for _, id := range p.opts.Taste.Liked(taste.Song) {
			if t, ok := p.favoriteByPath(id); ok {
				liked = append(liked, t.Meta(MetaArtistID))
			}
		}
	}
	var rest []string
	for _, a := range d.baseArtists {
		if strings.HasPrefix(a.ID, "UC") {
			rest = append(rest, a.ID)
		}
	}
	if p.opts.History != nil {
		if hist, err := p.opts.History.Tracks(200); err == nil {
			for _, t := range hist {
				rest = append(rest, t.Meta(MetaArtistID))
			}
		}
	}
	d.rng.Shuffle(len(rest), func(i, j int) { rest[i], rest[j] = rest[j], rest[i] })
	for _, id := range append(liked, rest...) {
		d.addSeed(id)
	}
}

// favoriteByPath finds a ♥ favorite by its path.
func (p *Provider) favoriteByPath(path string) (playlist.Track, bool) {
	if p.opts.Favorites == nil {
		return playlist.Track{}, false
	}
	tracks, err := p.opts.Favorites.Tracks()
	if err != nil {
		return playlist.Track{}, false
	}
	for _, t := range tracks {
		if t.Path == path {
			return t, true
		}
	}
	return playlist.Track{}, false
}

func (d *discovery) addSeed(id string) {
	if strings.HasPrefix(id, "UC") && !d.seedSeen[id] {
		d.seedSeen[id] = true
		d.seeds = append(d.seeds, id)
	}
}

// shuffleBands shuffles each tenth of a ranked list on its own, so the best
// stay near the top and the order still changes.
func shuffleBands[T any](rng *rand.Rand, s []T) {
	band := max(5, len(s)/10)
	for start := 0; start < len(s); start += band {
		part := s[start:min(len(s), start+band)]
		rng.Shuffle(len(part), func(i, j int) { part[i], part[j] = part[j], part[i] })
	}
}

// discoverRound opens a few seeds and their related artists and adds what
// they offer to the lists. It reports whether anything new came.
func (p *Provider) discoverRound(d *discovery) bool {
	if len(d.seeds) == 0 {
		return false
	}
	n := min(discoverSeeds, len(d.seeds))
	seeds := d.seeds[:n]
	d.seeds = d.seeds[n:]

	pages := p.artistPages(seeds)
	var related []string
	for _, page := range pages {
		for _, it := range page.Related {
			if strings.HasPrefix(it.ID, "UC") && !d.seedSeen[it.ID] && !p.hidden([]string{it.Title}, it.ID) {
				related = append(related, it.ID)
				p.addDiscoveredArtist(d, provider.ArtistInfo{ID: it.ID, Name: it.Title, ImageURL: it.Thumbnail})
			}
		}
	}
	d.rng.Shuffle(len(related), func(i, j int) { related[i], related[j] = related[j], related[i] })
	related = related[:min(discoverRelated, len(related))]
	for _, id := range related {
		d.addSeed(id)
	}
	pages = append(pages, p.artistPages(related)...)

	before := len(d.albums)
	var found []provider.AlbumInfo
	for _, page := range pages {
		for _, it := range append(append([]Item(nil), page.Albums...), page.Singles...) {
			found = append(found, provider.AlbumInfo{ID: it.ID, Name: it.Title, Artist: page.Name, ArtistID: page.ID, Year: it.Year, ImageURL: it.Thumbnail})
		}
	}
	d.rng.Shuffle(len(found), func(i, j int) { found[i], found[j] = found[j], found[i] })
	for _, a := range found {
		// Mix one library album in after every two discovered ones.
		if (len(d.albums)-before)%3 == 2 {
			p.mixBaseAlbum(d)
		}
		p.addDiscoveredAlbum(d, a)
	}
	return len(d.albums) > before || len(d.seeds) > 0
}

func (p *Provider) addDiscoveredAlbum(d *discovery, a provider.AlbumInfo) {
	if a.ID == "" || a.Name == "" || d.albumSeen[a.ID] || p.albumHidden(a) {
		return
	}
	d.albumSeen[a.ID] = true
	d.albums = append(d.albums, a)
}

func (p *Provider) addDiscoveredArtist(d *discovery, a provider.ArtistInfo) {
	if a.ID == "" || a.Name == "" || d.artSeen[a.ID] {
		return
	}
	d.artSeen[a.ID] = true
	d.artists = append(d.artists, a)
}

// mixBaseAlbum adds the next library album to the list.
func (p *Provider) mixBaseAlbum(d *discovery) {
	for d.nextAlbum < len(d.baseAlbums) {
		a := d.baseAlbums[d.nextAlbum]
		d.nextAlbum++
		if !d.albumSeen[a.ID] {
			d.albumSeen[a.ID] = true
			d.albums = append(d.albums, a)
			return
		}
	}
}

// artistPages opens artist pages in parallel; failed ones are left out.
func (p *Provider) artistPages(ids []string) []ArtistPage {
	out := make([]ArtistPage, len(ids))
	sem := make(chan struct{}, discoverParallel)
	var wg sync.WaitGroup
	for i, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			page, err := p.client.Artist(ctx, id)
			if err != nil {
				applog.Debug("ytmusic: discover artist %s: %v", id, err)
				return
			}
			out[i] = page
		}()
	}
	wg.Wait()
	pages := out[:0]
	for _, page := range out {
		if page.ID != "" {
			pages = append(pages, page)
		}
	}
	return pages
}

// discoverAlbums returns the "For you" page at offset, running rounds until
// the page is full or the seeds run out.
func (p *Provider) discoverAlbums(offset, size int) []provider.AlbumInfo {
	p.discMu.Lock()
	defer p.discMu.Unlock()
	d := p.discoveryLocked()
	want := offset + size
	for rounds := 0; len(d.albums) < want && !d.exhausted && rounds < discoverRounds; rounds++ {
		if !p.discoverRound(d) {
			d.exhausted = true
		}
	}
	// With the seeds spent, the rest of the library still follows.
	for len(d.albums) < want && d.nextAlbum < len(d.baseAlbums) {
		p.mixBaseAlbum(d)
	}
	// Paging the whole list keeps the offsets stable when a row is hidden
	// between pages; the hidden rows leave each page.
	return slices.DeleteFunc(slices.Clone(pageOf(d.albums, offset, size)), p.albumHidden)
}

// ArtistList returns a page of the artist list: the library's artists mixed
// with the related artists that discovery finds, without end.
func (p *Provider) ArtistList(offset, size int) ([]provider.ArtistInfo, error) {
	p.discMu.Lock()
	defer p.discMu.Unlock()
	d := p.discoveryLocked()
	want := offset + size
	for rounds := 0; len(d.artists) < want && !d.exhausted && rounds < discoverRounds; rounds++ {
		// One library artist leads, then the related ones that round finds.
		for range 3 {
			if d.nextArtist < len(d.baseArtists) {
				p.addDiscoveredArtist(d, d.baseArtists[d.nextArtist])
				d.nextArtist++
			}
		}
		if !p.discoverRound(d) {
			d.exhausted = true
		}
	}
	for len(d.artists) < want && d.nextArtist < len(d.baseArtists) {
		p.addDiscoveredArtist(d, d.baseArtists[d.nextArtist])
		d.nextArtist++
	}
	return slices.DeleteFunc(slices.Clone(pageOf(d.artists, offset, size)), p.artistHidden), nil
}

func pageOf[T any](s []T, offset, size int) []T {
	if offset >= len(s) {
		return nil
	}
	end := len(s)
	if size > 0 && offset+size < end {
		end = offset + size
	}
	return s[offset:end]
}

// extraGenres are genres YouTube Music has no category for. Each plays a
// mix of the playlists and songs a search for it finds.
var extraGenres = []string{
	"Shoegaze", "Dream pop", "Post-punk", "Darkwave", "Coldwave", "Synthwave", "Vaporwave",
	"Chillwave", "City pop", "J-rock", "J-pop", "Vocaloid", "Anime", "Video game music",
	"Math rock", "Post-rock", "Midwest emo", "Indie folk", "Lo-fi", "Trip hop", "Drum and bass",
	"Breakcore", "Hardstyle", "IDM", "Ambient", "Krautrock", "Psychedelic rock", "Stoner rock",
	"Nu metal", "Metalcore", "Progressive metal", "Jazz fusion", "Nu jazz", "Bossa nova",
	"Neo soul", "Funk", "Disco", "Italo disco", "Chiptune", "Rock en español",
	"Rock chileno", "Música andina", "Electro swing", "Soundtrack",
}

const prefixSearchGenre = "search:"

// searchGenreTracks plays an extra genre: two random playlists of the first
// results and the songs the search finds, shuffled.
func (p *Provider) searchGenreTracks(query string) ([]playlist.Track, error) {
	ctx, cancel := budget()
	defer cancel()
	var songs []Song
	if shelves, err := p.client.Search(ctx, query, SearchSongs, 40); err == nil {
		for _, sh := range shelves {
			for _, it := range sh.Items {
				if it.Song != nil {
					songs = append(songs, *it.Song)
				}
			}
		}
	}
	shelves, err := p.client.Search(ctx, query, SearchPlaylists, 10)
	if err != nil && len(songs) == 0 {
		return nil, err
	}
	var lists []Item
	for _, sh := range shelves {
		lists = append(lists, sh.Items...)
	}
	rand.Shuffle(len(lists), func(i, j int) { lists[i], lists[j] = lists[j], lists[i] })
	for _, it := range lists[:min(2, len(lists))] {
		if page, err := p.client.Playlist(ctx, it.ID, 80); err == nil {
			songs = append(songs, page.Songs...)
		}
	}
	rand.Shuffle(len(songs), func(i, j int) { songs[i], songs[j] = songs[j], songs[i] })
	tracks := p.visibleTracks(dedupeTracks(songTracks(songs)))
	for i := range tracks {
		tracks[i].Genre = query
	}
	return tracks, nil
}

// dedupeTracks drops repeated songs, keeping the first.
func dedupeTracks(tracks []playlist.Track) []playlist.Track {
	seen := map[string]bool{}
	out := tracks[:0]
	for _, t := range tracks {
		if !seen[t.Path] {
			seen[t.Path] = true
			out = append(out, t)
		}
	}
	return out
}

// DislikeTrack tells YouTube Music the listener does not like the song, which
// "not interested" (H) on a song sends.
func (p *Provider) DislikeTrack(ctx context.Context, track playlist.Track) error {
	id := trackVideoID(track)
	if id == "" || !p.client.SignedIn() {
		return nil
	}
	return p.client.SetDislike(ctx, id)
}

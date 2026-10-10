package ytmusic

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/applog"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// "New from your artists" lists the latest albums and singles of the artists
// the account subscribes to. YouTube Music gives a release year, not a date,
// so a release counts as new in its year (and in the first two months of the
// next). An artist's page lists releases newest first, which orders releases
// of the same year.
//
// Looking it up reads one page per artist, so the list is cached on disk and
// refreshed in the background after newReleasesTTL: the provider pane never
// waits for it, except on the very first run, which waits up to
// newReleasesFirstWait.

const (
	newReleasesTTL       = 6 * time.Hour
	newReleasesFirstWait = 8 * time.Second
	newReleasesArtists   = 80 // subscribed artists read per refresh
	newReleasesParallel  = 6
	newReleasesKeep      = 40 // releases kept
	newReleasesShown     = 15 // releases shown in the pane
	newReleasesFile      = "ytmusic_new_releases.json"
)

// release is one album or single in "New from your artists".
type release struct {
	ID        string `json:"id"` // album browse ID (MPREb_...)
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Year      int    `json:"year"`
	Single    bool   `json:"single,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Rank      int    `json:"rank"` // position on the artist's shelf, 0 = newest
}

type releasesFile struct {
	Fetched  time.Time       `json:"fetched"`
	Releases []release       `json:"releases"`
	Seen     map[string]bool `json:"seen,omitempty"`
}

// newReleases holds the cached list. All fields are guarded by mu.
type newReleases struct {
	mu         sync.Mutex
	loaded     bool
	data       releasesFile
	refreshing bool
}

func (p *Provider) releasesPath() string {
	if p.opts.DataDir == "" {
		return ""
	}
	return filepath.Join(p.opts.DataDir, newReleasesFile)
}

// loadReleases reads the cache file once.
func (p *Provider) loadReleases() {
	nr := &p.releases
	if nr.loaded {
		return
	}
	nr.loaded = true
	if path := p.releasesPath(); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &nr.data)
		}
	}
	if nr.data.Seen == nil {
		nr.data.Seen = map[string]bool{}
	}
}

func (p *Provider) saveReleasesLocked() {
	path := p.releasesPath()
	if path == "" {
		return
	}
	data, err := json.Marshal(p.releases.data)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, path)
	}
}

// cachedReleases returns the releases to show and starts a refresh when the
// cache is stale. Signed out, there are none.
func (p *Provider) cachedReleases() ([]release, map[string]bool) {
	if !p.client.SignedIn() {
		return nil, nil
	}
	nr := &p.releases
	nr.mu.Lock()
	p.loadReleases()
	stale := time.Since(nr.data.Fetched) > newReleasesTTL
	first := nr.data.Fetched.IsZero()
	start := stale && !nr.refreshing
	if start {
		nr.refreshing = true
	}
	nr.mu.Unlock()

	if start {
		done := make(chan struct{})
		go func() {
			defer close(done)
			p.refreshReleases()
		}()
		if first {
			select {
			case <-done:
			case <-time.After(newReleasesFirstWait):
			}
		}
	}

	nr.mu.Lock()
	defer nr.mu.Unlock()
	rel := append([]release(nil), nr.data.Releases...)
	seen := make(map[string]bool, len(nr.data.Seen))
	for k, v := range nr.data.Seen {
		seen[k] = v
	}
	return rel, seen
}

// refreshReleases reads the subscribed artists' pages and stores the newest
// releases.
func (p *Provider) refreshReleases() {
	nr := &p.releases
	defer func() {
		nr.mu.Lock()
		nr.refreshing = false
		nr.mu.Unlock()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	subs, err := p.client.Subscriptions(ctx)
	if err != nil {
		applog.Info("ytmusic: new releases: %v", err)
		return
	}
	var artists []Item
	for _, it := range subs {
		if it.Kind == KindArtist && it.ID != "" {
			artists = append(artists, it)
		}
		if len(artists) == newReleasesArtists {
			break
		}
	}
	found := collectReleases(ctx, artists, time.Now(), func(ctx context.Context, id string) (ArtistPage, error) {
		return p.client.Artist(ctx, id)
	})
	nr.mu.Lock()
	defer nr.mu.Unlock()
	if len(found) == 0 && len(artists) > 0 && ctx.Err() != nil {
		return // timed out: keep the old list
	}
	nr.data.Releases = found
	nr.data.Fetched = time.Now()
	// Forget the seen marks of releases that left the list.
	keep := map[string]bool{}
	for _, r := range found {
		if nr.data.Seen[r.ID] {
			keep[r.ID] = true
		}
	}
	nr.data.Seen = keep
	p.saveReleasesLocked()
}

// isNewYear reports whether a release year counts as new at now.
func isNewYear(year int, now time.Time) bool {
	if year >= now.Year() {
		return true
	}
	return year == now.Year()-1 && now.Month() <= time.February
}

// collectReleases reads the artists' pages in parallel and returns their new
// releases, newest first.
func collectReleases(ctx context.Context, artists []Item, now time.Time, artist func(context.Context, string) (ArtistPage, error)) []release {
	var (
		mu    sync.Mutex
		found []release
		seen  = map[string]bool{}
		wg    sync.WaitGroup
		sem   = make(chan struct{}, newReleasesParallel)
	)
	for _, a := range artists {
		wg.Add(1)
		go func(a Item) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			page, err := artist(ctx, a.ID)
			if err != nil {
				return
			}
			name := page.Name
			if name == "" {
				name = a.Title
			}
			add := func(items []Item, single bool) {
				for rank, it := range items {
					if it.Kind != KindAlbum || it.ID == "" || !isNewYear(it.Year, now) {
						continue
					}
					mu.Lock()
					if !seen[it.ID] {
						seen[it.ID] = true
						found = append(found, release{ID: it.ID, Title: it.Title, Artist: name, Year: it.Year, Single: single, Thumbnail: it.Thumbnail, Rank: rank})
					}
					mu.Unlock()
				}
			}
			add(page.Singles, true)
			add(page.Albums, false)
		}(a)
	}
	wg.Wait()
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].Year != found[j].Year {
			return found[i].Year > found[j].Year
		}
		if found[i].Rank != found[j].Rank {
			return found[i].Rank < found[j].Rank
		}
		return found[i].Artist < found[j].Artist
	})
	if len(found) > newReleasesKeep {
		found = found[:newReleasesKeep]
	}
	return found
}

// releaseRows turns the releases into provider pane rows. Unopened ones are
// marked with a dot.
func (p *Provider) releaseRows() []playlist.PlaylistInfo {
	rel, seen := p.cachedReleases()
	var rows []playlist.PlaylistInfo
	for _, r := range rel {
		if len(rows) == newReleasesShown {
			break
		}
		name := r.Title + " · " + r.Artist
		if r.Single {
			name += " · Single"
		}
		if !seen[r.ID] {
			name = "● " + name
		}
		rows = append(rows, playlist.PlaylistInfo{ID: prefixAlbum + r.ID, Name: name, Section: p.words.newReleases, ImageURL: r.Thumbnail})
	}
	return rows
}

// markReleaseSeen drops the dot of a release once it is opened.
func (p *Provider) markReleaseSeen(albumID string) {
	nr := &p.releases
	nr.mu.Lock()
	defer nr.mu.Unlock()
	if !nr.loaded || nr.data.Seen[albumID] {
		return
	}
	for _, r := range nr.data.Releases {
		if r.ID == albumID {
			nr.data.Seen[albumID] = true
			p.saveReleasesLocked()
			return
		}
	}
}

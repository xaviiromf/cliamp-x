package ytmusic

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/xaviiromf/cliamp-x/provider"
)

// previewCache keeps the preview details the provider looked up, by ID.
type previewCache struct {
	mu   sync.Mutex
	byID map[string]provider.PreviewDetails
}

func (c *previewCache) get(id string) (provider.PreviewDetails, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.byID[id]
	return d, ok
}

func (c *previewCache) put(id string, d provider.PreviewDetails) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byID == nil {
		c.byID = map[string]provider.PreviewDetails{}
	}
	c.byID[id] = d
}

// PreviewDetails looks up the picture and the extra lines the preview pane
// shows for an album, artist or playlist row, by any of the IDs the provider
// hands out. Answers are cached for the run.
func (p *Provider) PreviewDetails(ctx context.Context, id string) (provider.PreviewDetails, error) {
	if d, ok := p.previews.get(id); ok {
		return d, nil
	}
	d, err := p.lookupPreview(ctx, id)
	if err != nil {
		return d, err
	}
	p.previews.put(id, d)
	return d, nil
}

func joinNonEmpty(sep string, parts ...string) string {
	var kept []string
	for _, s := range parts {
		if s = strings.TrimSpace(s); s != "" {
			kept = append(kept, s)
		}
	}
	return strings.Join(kept, sep)
}

func (p *Provider) lookupPreview(ctx context.Context, id string) (provider.PreviewDetails, error) {
	raw := id
	for _, prefix := range []string{prefixAlbum, prefixList, prefixArtist} {
		raw = strings.TrimPrefix(raw, prefix)
	}
	switch {
	case strings.HasPrefix(id, prefixLocalAlbum):
		return p.localAlbumPreview(strings.TrimPrefix(id, prefixLocalAlbum)), nil
	case strings.HasPrefix(id, prefixLocalArtist):
		return p.localArtistPreview(strings.TrimPrefix(id, prefixLocalArtist)), nil
	case strings.HasPrefix(raw, "MPREb_"):
		page, err := p.client.Album(ctx, raw)
		if err != nil {
			return provider.PreviewDetails{}, err
		}
		year := ""
		if page.Year > 0 {
			year = fmt.Sprint(page.Year)
		}
		return provider.PreviewDetails{
			ImageURL: page.Thumbnail,
			Lines: []string{
				joinNonEmpty(" · ", page.Kind, year),
				page.Artist,
				page.Length,
			},
		}, nil
	case strings.HasPrefix(raw, "UC"):
		page, err := p.client.Artist(ctx, raw)
		if err != nil {
			return provider.PreviewDetails{}, err
		}
		lines := []string{page.Listeners}
		if page.Subscribers != "" {
			lines = append(lines, page.Subscribers+" "+p.words.subscribers)
		}
		var top []string
		for i, s := range page.TopSongs {
			if i == 3 {
				break
			}
			top = append(top, s.Title)
		}
		if len(top) > 0 {
			lines = append(lines, "♫ "+strings.Join(top, " · "))
		}
		return provider.PreviewDetails{ImageURL: page.Thumbnail, Lines: lines}, nil
	case strings.HasPrefix(raw, "VL") || strings.HasPrefix(raw, "PL") || strings.HasPrefix(raw, "RDCLAK") || strings.HasPrefix(raw, "OLAK"):
		page, err := p.client.Playlist(ctx, raw, 1)
		if err != nil {
			return provider.PreviewDetails{}, err
		}
		return provider.PreviewDetails{ImageURL: page.Thumbnail, Lines: []string{page.Author, page.Length}}, nil
	}
	return provider.PreviewDetails{}, nil
}

func (p *Provider) localAlbumPreview(key string) provider.PreviewDetails {
	a, ok := p.opts.Local.Album(key)
	if !ok || len(a.Tracks) == 0 {
		return provider.PreviewDetails{}
	}
	year := ""
	if a.Year > 0 {
		year = fmt.Sprint(a.Year)
	}
	return provider.PreviewDetails{
		ImageURL: a.Tracks[0].Path,
		Lines: []string{
			joinNonEmpty(" · ", p.words.localMusic, year),
			a.Artist,
			fmt.Sprintf("%d %s", len(a.Tracks), p.words.songsWord),
		},
	}
}

func (p *Provider) localArtistPreview(name string) provider.PreviewDetails {
	albums := p.opts.Local.ArtistAlbums(name)
	if len(albums) == 0 {
		return provider.PreviewDetails{}
	}
	songs := 0
	for _, a := range albums {
		songs += len(a.Tracks)
	}
	return provider.PreviewDetails{
		ImageURL: albums[0].Tracks[0].Path,
		Lines: []string{
			p.words.localMusic,
			fmt.Sprintf("%d %s · %d %s", len(albums), strings.ToLower(p.words.albums), songs, p.words.songsWord),
		},
	}
}

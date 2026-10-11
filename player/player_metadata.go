package player

import (
	"context"
	"time"
)

// Now-playing titles of internet radio: ICY titles and the metadata
// resolvers that providers register for their streams.

// StreamMetadataResolver matches a stream URL to a now-playing fetcher for
// broadcasters that carry no inline ICY metadata (e.g. NTS, FIP) and instead
// publish the current track via a separate JSON API. It returns a fetch
// function, the poll interval, and ok=false when the URL is not recognized.
type StreamMetadataResolver func(streamURL string) (fetch func(ctx context.Context) (string, error), interval time.Duration, ok bool)

// StreamTitle returns the current ICY stream title (e.g., "Artist - Song").
// Returns "" when no ICY metadata has been received.
func (p *Player) StreamTitle() string {
	v, _ := p.streamTitle.Load().(string)
	return v
}

// setStreamTitle is the ICY onMeta callback, called from the reader goroutine.
func (p *Player) setStreamTitle(title string) {
	p.streamTitle.Store(title)
}

// RegisterStreamMetadataResolver installs a resolver used to pull now-playing
// metadata for streams that lack inline ICY metadata. Pass nil to disable.
func (p *Player) RegisterStreamMetadataResolver(r StreamMetadataResolver) {
	p.mu.Lock()
	p.streamMetaResolver = r
	p.mu.Unlock()
}

// startStreamMetadata (re)starts background now-playing polling for streamURL,
// cancelling any previous poller. It is a no-op unless a registered resolver
// recognizes the URL, so streams that carry ICY metadata (or local files) are
// unaffected. The poller feeds titles through the same path as ICY metadata.
func (p *Player) startStreamMetadata(streamURL string) {
	p.stopStreamMetadata()

	p.mu.Lock()
	resolver := p.streamMetaResolver
	p.mu.Unlock()
	if resolver == nil || streamURL == "" || !isURL(streamURL) {
		return
	}
	fetch, interval, ok := resolver(streamURL)
	if !ok || fetch == nil {
		return
	}
	if interval <= 0 {
		interval = 15 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.mu.Lock()
	p.metaCancel = cancel
	p.mu.Unlock()

	go p.pollStreamMetadata(ctx, fetch, interval)
}

// stopStreamMetadata cancels the active metadata poller, if any.
func (p *Player) stopStreamMetadata() {
	p.mu.Lock()
	cancel := p.metaCancel
	p.metaCancel = nil
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// pollStreamMetadata fetches the current title immediately and then on each
// interval tick until ctx is cancelled, publishing non-empty titles via
// setStreamTitle. A title fetched after cancellation is discarded so a stale
// poller cannot clobber the next stream's metadata.
func (p *Player) pollStreamMetadata(ctx context.Context, fetch func(context.Context) (string, error), interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		title, err := fetch(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil && title != "" {
			p.setStreamTitle(title)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

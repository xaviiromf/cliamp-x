package player

import (
	"time"

	"github.com/gopxl/beep/v2"
)

// What providers register with the player: stream factories for custom
// URIs, source resolvers, URL matchers and the downloaded copies of tracks.

// StreamerFactory creates a beep.StreamSeekCloser for a custom URI scheme
// (e.g., spotify:track:xxx). Returns the streamer, its format, the track
// duration, and any error.
type StreamerFactory func(uri string) (beep.StreamSeekCloser, beep.Format, time.Duration, error)

// RegisterStreamerFactory registers a factory for a custom URI scheme prefix
// (e.g., "spotify:"). When buildPipeline encounters a path starting with this
// prefix, it calls the factory to create the decoder instead of the normal
// file/HTTP pipeline.
func (p *Player) RegisterStreamerFactory(scheme string, f StreamerFactory) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.customFactories == nil {
		p.customFactories = make(map[string]StreamerFactory)
	}
	p.customFactories[scheme] = f
}

// RegisterBufferedURLMatcher registers a function that identifies HTTP URLs
// requiring the buffered download + ffmpeg pipeline (e.g. Subsonic stream
// endpoints). This replaces hardcoded URL pattern checks.
func (p *Player) RegisterBufferedURLMatcher(match func(string) bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bufferedURLMatch = match
}

// RegisterYTDLMatcher registers a function that identifies page URLs, such
// as YouTube or SoundCloud pages, that play through the yt-dlp | ffmpeg
// pipe chain.
func (p *Player) RegisterYTDLMatcher(match func(string) bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ytdlURLMatch = match
}

// ResolvedSource is a playable source produced by a SourceResolver at play
// time: either a direct HTTP URL, or an ordered list of media segment URLs
// whose concatenated bytes form one progressive stream (e.g. unencrypted
// DASH fMP4 segments). Buffered sends URL to the buffered download and
// ffmpeg pipeline, for a finite file at a signed URL that no buffered-URL
// matcher can recognize.
type ResolvedSource struct {
	URL      string
	Segments []string
	Buffered bool
}

// SourceResolver turns a custom URI (e.g. "tidal://track/123") into a
// ResolvedSource when playback starts. Resolving at play time keeps
// short-lived signed URLs fresh no matter how long a track sat in the queue.
type SourceResolver func(uri string) (ResolvedSource, error)

// RegisterSourceResolver registers a resolver for a custom URI scheme prefix.
// Unlike RegisterStreamerFactory, the provider only supplies bytes to fetch;
// decoding stays in the player's buffered ffmpeg pipeline.
func (p *Player) RegisterSourceResolver(scheme string, r SourceResolver) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sourceResolvers == nil {
		p.sourceResolvers = make(map[string]SourceResolver)
	}
	p.sourceResolvers[scheme] = r
}

// HasSourceResolver reports whether a registered SourceResolver claims path.
// Such a path opens over the network when playback starts.
func (p *Player) HasSourceResolver(path string) bool {
	return p.matchSourceResolver(path) != nil
}

// SetLocalCopies sets how the player finds a downloaded copy of a track:
// lookup returns the file to play instead of streaming path.
func (p *Player) SetLocalCopies(lookup func(path string) (string, bool)) {
	p.mu.Lock()
	p.localCopies = lookup
	p.mu.Unlock()
}

func (p *Player) localCopy(path string) (string, bool) {
	p.mu.Lock()
	lookup := p.localCopies
	p.mu.Unlock()
	if lookup == nil {
		return "", false
	}
	return lookup(path)
}

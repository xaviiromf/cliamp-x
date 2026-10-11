package model

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/sponsorblock"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// SponsorBlock skips the marked parts of YouTube videos. Segments are looked
// up for the song that plays and the one after it, so they are usually known
// before a song starts: an intro is then skipped by starting the song after
// it, which costs nothing, instead of seeking (a yt-dlp seek restarts the
// download). A segment that runs to the end of the song moves to the next
// one. Each segment is skipped once per play, so seeking back into it plays
// it.

const (
	// sponsorRetryAfter is how long a failed lookup waits before it is tried
	// again.
	sponsorRetryAfter = 2 * time.Minute
	// sponsorEndSlack treats a segment that ends this close to the end of
	// the song as running to the end.
	sponsorEndSlack = 1.5
	// sponsorMinSkip ignores a skip that would move less than this.
	sponsorMinSkip = 1.0
)

// sponsorState is the Model's SponsorBlock state.
type sponsorState struct {
	client  *sponsorblock.Client
	pending map[string]bool
	failed  map[string]time.Time
	skipped map[string]bool // "path|start" of segments skipped in this play
}

// sponsorSegmentsMsg reports that the segments of a video were looked up.
type sponsorSegmentsMsg struct {
	id  string
	err error
}

// SetSponsorBlock turns SponsorBlock on with the given client, or off with nil.
func (m *Model) SetSponsorBlock(c *sponsorblock.Client) {
	m.sponsor = sponsorState{
		client:  c,
		pending: map[string]bool{},
		failed:  map[string]time.Time{},
		skipped: map[string]bool{},
	}
}

// sponsorSegments returns the known segments of track.
func (m *Model) sponsorSegments(track playlist.Track) []sponsorblock.Segment {
	if m.sponsor.client == nil {
		return nil
	}
	id := sponsorblock.VideoID(track.Path)
	if id == "" {
		return nil
	}
	segs, _ := m.sponsor.client.Cached(id)
	return segs
}

// sponsorStartOffset returns where track should start to skip a segment at
// its very beginning, or 0.
func (m *Model) sponsorStartOffset(track playlist.Track) time.Duration {
	segs := m.sponsorSegments(track)
	if len(segs) == 0 || segs[0].Start > sponsorMinSkip {
		return 0
	}
	s := segs[0] // segments are sorted, so only the first can start the song
	if track.DurationSecs > 0 && s.End >= float64(track.DurationSecs)-sponsorEndSlack {
		// The whole song is marked; play it rather than skip it all.
		return 0
	}
	m.sponsor.skipped[sponsorKey(track, s)] = true
	return time.Duration(s.End * float64(time.Second))
}

func sponsorKey(track playlist.Track, s sponsorblock.Segment) string {
	return fmt.Sprintf("%s|%.1f", track.Path, s.Start)
}

// sponsorCmds looks up the segments of the song that plays and of the next
// one.
func (m *Model) sponsorCmds() tea.Cmd {
	if m.sponsor.client == nil || m.playlist == nil {
		return nil
	}
	var cmds []tea.Cmd
	want := func(track playlist.Track) {
		id := sponsorblock.VideoID(track.Path)
		if id == "" || m.sponsor.pending[id] {
			return
		}
		if _, ok := m.sponsor.client.Cached(id); ok {
			return
		}
		if at, ok := m.sponsor.failed[id]; ok && time.Since(at) < sponsorRetryAfter {
			return
		}
		m.sponsor.pending[id] = true
		client := m.sponsor.client
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := client.Segments(ctx, id)
			return sponsorSegmentsMsg{id: id, err: err}
		})
	}
	if track, idx := m.currentPlaybackTrack(); idx >= 0 {
		want(track)
	}
	if next, ok := m.preloadTarget(); ok {
		want(next)
	}
	return tea.Batch(cmds...)
}

func (m *Model) handleSponsorSegments(msg sponsorSegmentsMsg) {
	delete(m.sponsor.pending, msg.id)
	if msg.err != nil {
		m.sponsor.failed[msg.id] = time.Now()
	}
}

// tickSponsor skips a marked segment the playback has entered.
func (m *Model) tickSponsor() tea.Cmd {
	if m.sponsor.client == nil || m.buffering || m.seek.active || !m.player.IsPlaying() || m.player.IsPaused() {
		return nil
	}
	track, idx := m.currentPlaybackTrack()
	if idx < 0 {
		return nil
	}
	segs := m.sponsorSegments(track)
	if len(segs) == 0 {
		return nil
	}
	pos := m.cachedPos.Seconds()
	dur := m.cachedDur.Seconds()
	if dur <= 0 {
		dur = float64(track.DurationSecs)
	}
	for _, s := range segs {
		key := sponsorKey(track, s)
		if m.sponsor.skipped[key] || pos < s.Start || pos >= s.End-sponsorMinSkip {
			continue
		}
		if len(m.sponsor.skipped) > 512 {
			clear(m.sponsor.skipped)
		}
		m.sponsor.skipped[key] = true
		label := sponsorCategoryLabel(s.Category)
		if dur > 0 && s.End >= dur-sponsorEndSlack {
			if s.Start <= sponsorMinSkip {
				return nil // the whole song is marked; play it
			}
			m.status.Showf(statusTTLMedium, "SponsorBlock: %s %s", L("skipped"), label)
			return m.nextTrack()
		}
		m.status.Showf(statusTTLMedium, "SponsorBlock: %s %s (%s)", L("skipped"), label, formatTrackTime(int(s.End-pos+0.5)))
		return m.seekAbsolute(time.Duration(s.End * float64(time.Second)))
	}
	return nil
}

// sponsorCategoryLabel names a category for the status line.
func sponsorCategoryLabel(category string) string {
	switch category {
	case "intro":
		return L("intro")
	case "outro":
		return L("outro")
	case "sponsor":
		return L("sponsor")
	case "selfpromo":
		return L("self-promotion")
	case "music_offtopic":
		return L("non-music part")
	case "interaction":
		return L("interaction reminder")
	case "preview":
		return L("preview")
	case "filler":
		return L("filler")
	}
	return category
}

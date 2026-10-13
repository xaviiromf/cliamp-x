package model

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

const (
	// autoRadioTrackCount is how many related songs one auto radio step adds.
	autoRadioTrackCount = 25
	// autoRadioTimeout bounds the related-songs lookup.
	autoRadioTimeout = 30 * time.Second
)

// autoRadioLoadedMsg carries the songs related to the last track of a queue
// that ran out.
type autoRadioLoadedMsg struct {
	tracks []playlist.Track
	err    error
	gen    uint64
}

// sectionBoolSaver is implemented by config savers that can write a key of
// a config section, such as auto_radio in [ytmusic].
type sectionBoolSaver interface {
	SaveSectionBool(section, key string, value bool) error
}

// SetAutoRadio turns on or off the auto radio: when the queue runs out, the
// player keeps going with songs related to the last one.
func (m *Model) SetAutoRadio(on bool) { m.autoRadio = on }

// AutoRadio reports whether the auto radio is on.
func (m Model) AutoRadio() bool { return m.autoRadio }

// toggleAutoRadio flips the auto radio and saves the choice.
func (m *Model) toggleAutoRadio() {
	m.autoRadio = !m.autoRadio
	if m.autoRadio {
		m.status.Show("Auto radio on: similar songs follow the end of the queue", statusTTLMedium)
	} else {
		m.status.Show("Auto radio off", statusTTLMedium)
	}
	if s, ok := m.configSaver.(sectionBoolSaver); ok {
		if err := s.SaveSectionBool("ytmusic", "auto_radio", m.autoRadio); err != nil {
			m.status.Errorf(statusTTLDefault, "Saving auto radio failed: %s", err)
		}
	}
}

// autoRadioCmd looks up songs related to seed, the last track of a queue
// that just ran out. It returns nil when the auto radio is off or no
// provider knows songs related to seed.
func (m *Model) autoRadioCmd(seed playlist.Track) tea.Cmd {
	if !m.autoRadio || seed.Path == "" {
		return nil
	}
	relater, _ := findCapable(m, func(r provider.Relater) bool { return r.CanRelate(seed) })
	if relater == nil {
		return nil
	}
	gen := nextRequest(&m.requests.autoRadio)
	m.status.Activityf(statusTTLMedium, "Auto radio: finding songs like %s…", seed.DisplayName())
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), autoRadioTimeout)
		defer cancel()
		tracks, err := relater.RelatedTracks(ctx, seed, autoRadioTrackCount)
		return autoRadioLoadedMsg{tracks: tracks, err: err, gen: gen}
	}
}

// handleAutoRadioLoaded appends the related songs that the queue does not
// hold yet and plays the first of them.
func (m *Model) handleAutoRadioLoaded(msg autoRadioLoadedMsg) tea.Cmd {
	if msg.gen != m.requests.autoRadio || !m.autoRadio {
		return nil
	}
	if msg.err != nil {
		m.status.Warningf(statusTTLMedium, "Auto radio: %s", msg.err)
		return nil
	}
	have := make(map[string]bool, m.playlist.Len())
	for _, t := range m.playlist.Tracks() {
		have[t.Path] = true
	}
	var fresh []playlist.Track
	for _, t := range m.visibleTracks(msg.tracks) {
		if !have[t.Path] {
			have[t.Path] = true
			fresh = append(fresh, t)
		}
	}
	if len(fresh) == 0 {
		m.status.Show("Auto radio: no new songs", statusTTLMedium)
		return nil
	}
	// The queue ended on its last track, so Next moves onto the first new one.
	m.playlist.Add(fresh...)
	m.status.Showf(statusTTLMedium, "Auto radio: added %d songs", len(fresh))
	return m.nextTrack()
}

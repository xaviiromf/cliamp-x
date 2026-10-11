package model

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
)

// Desktop notifications: each song that starts shows a "now playing"
// notification with its cover, which replaces the previous one. The cover
// often loads a moment after the song starts, so the notification goes out
// at once and is replaced with the picture when it arrives soon enough.

// notifyCoverWait is how long a notification waits to be redone with its
// cover.
const notifyCoverWait = 8 * time.Second

// DesktopNotifier shows notifications.
type DesktopNotifier interface {
	Show(summary, body, image string) error
}

// NotifyActionMsg carries a notification button press: "pause", "next" or
// "like".
type NotifyActionMsg struct{ Action string }

type notifyState struct {
	n       DesktopNotifier
	pending playlist.Track // the song whose notification waits for its cover
	until   time.Time
}

// SetDesktopNotifier turns notifications on, or off with nil.
func (m *Model) SetDesktopNotifier(n DesktopNotifier) { m.notify.n = n }

// notifyTrack shows the notification of a song that started.
func (m *Model) notifyTrack(track playlist.Track) {
	// The full-screen visualizer on a focused terminal already shows the
	// song, so a notification would only cover the screen.
	if m.notify.n == nil || m.fullVis && !m.blurred {
		return
	}
	image := ""
	if m.art != nil {
		if path, ok := m.art.ImageFile(trackArtURL(track)); ok {
			image = path
		}
	}
	m.sendNotification(track, image)
	m.notify.pending = playlist.Track{}
	if image == "" && trackArtURL(track) != "" {
		m.notify.pending, m.notify.until = track, time.Now().Add(notifyCoverWait)
	}
}

func (m *Model) sendNotification(track playlist.Track, image string) {
	title := track.Title
	if title == "" {
		title = track.DisplayName()
	}
	var body []string
	if track.Artist != "" {
		body = append(body, track.Artist)
	}
	if track.Album != "" && track.Album != track.Title {
		body = append(body, track.Album)
	}
	n := m.notify.n
	go func() { _ = n.Show(title, strings.Join(body, " — "), image) }()
}

// notifyCmds loads the cover a pending notification waits for, and redoes
// the notification once it is there.
func (m *Model) notifyCmds() tea.Cmd {
	p := m.notify.pending
	if m.notify.n == nil || p.Path == "" || m.art == nil {
		return nil
	}
	if time.Now().After(m.notify.until) {
		m.notify.pending = playlist.Track{}
		return nil
	}
	url := trackArtURL(p)
	if path, ok := m.art.ImageFile(url); ok {
		m.notify.pending = playlist.Track{}
		m.sendNotification(p, path)
		return nil
	}
	if m.art.Claim(url) {
		return loadArtworkCmd(m.art, url)
	}
	return nil
}

// handleNotifyAction runs a notification button.
func (m *Model) handleNotifyAction(msg NotifyActionMsg) tea.Cmd {
	switch msg.Action {
	case "pause":
		return m.togglePlayPause()
	case "next":
		return m.skipNext()
	case "like":
		track, idx := m.currentPlaybackTrack()
		if idx < 0 {
			return nil
		}
		cmd, err := m.toggleTrackFavorite(track)
		if err != nil {
			m.status.Errorf(statusTTLDefault, "Favorite failed: %s", err)
			return nil
		}
		return cmd
	}
	return nil
}

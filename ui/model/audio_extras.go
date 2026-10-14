package model

import (
	"time"

	"github.com/xaviiromf/cliamp-x/player"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// normalizer and crossfader are the engine features of the real player that
// the test engines may lack.
type normalizer interface {
	SetNormalize(mode player.NormalizeMode, targetLUFS float64)
	Normalize() (player.NormalizeMode, float64)
}

type crossfader interface {
	SetCrossfade(d time.Duration)
}

// audioFormatter reports the codec, bitrate and rate of the playing source.
type audioFormatter interface {
	AudioFormat() player.AudioFormat
}

// playingAudioFormat is the format of the playing source, or zero when the
// engine does not know it.
func (m *Model) playingAudioFormat() player.AudioFormat {
	if f, ok := m.player.(audioFormatter); ok {
		return f.AudioFormat()
	}
	return player.AudioFormat{}
}

type offsetPreloader interface {
	PreloadAtForGeneration(path string, knownDuration, offset time.Duration, generation uint64) error
}

// parseNormalizeMode reads a config value; anything unknown is "track".
func parseNormalizeMode(s string) player.NormalizeMode {
	switch s {
	case "off":
		return player.NormalizeOff
	case "album":
		return player.NormalizeAlbum
	}
	return player.NormalizeTrack
}

func normalizeModeName(mode player.NormalizeMode) string {
	switch mode {
	case player.NormalizeOff:
		return "off"
	case player.NormalizeAlbum:
		return "album"
	}
	return "track"
}

func normalizeModeLabel(mode player.NormalizeMode) string {
	switch mode {
	case player.NormalizeOff:
		return L("off")
	case player.NormalizeAlbum:
		return L("by album")
	}
	return L("by song")
}

// SetNormalize applies the normalization config: mode is "track", "album" or
// "off", target is in LUFS (0 for the default).
func (m *Model) SetNormalize(mode string, target float64) {
	if n, ok := m.player.(normalizer); ok {
		n.SetNormalize(parseNormalizeMode(mode), target)
	}
}

// cycleNormalize steps off -> by song -> by album, shows it and saves it.
func (m *Model) cycleNormalize() {
	n, ok := m.player.(normalizer)
	if !ok {
		return
	}
	mode, _ := n.Normalize()
	switch mode {
	case player.NormalizeOff:
		mode = player.NormalizeTrack
	case player.NormalizeTrack:
		mode = player.NormalizeAlbum
	default:
		mode = player.NormalizeOff
	}
	_ = m.setNormalizeMode(mode)
}

// setNormalizeMode sets the volume normalization, shows it and saves it.
func (m *Model) setNormalizeMode(mode player.NormalizeMode) error {
	n, ok := m.player.(normalizer)
	if !ok {
		return nil
	}
	_, target := n.Normalize()
	n.SetNormalize(mode, target)
	m.status.Showf(statusTTLMedium, "%s: %s", L("Volume normalization"), normalizeModeLabel(mode))
	return m.saveConfigString("normalize", normalizeModeName(mode))
}

// SetCrossfade sets the crossfade length used between songs of different
// albums.
func (m *Model) SetCrossfade(seconds int) {
	m.crossfade = time.Duration(max(0, min(12, seconds))) * time.Second
}

// sameAlbum reports whether two songs are consecutive tracks of one album,
// which play back to back instead of crossfading.
func sameAlbum(a, b playlist.Track) bool {
	if a.Album == "" || a.Album != b.Album {
		return false
	}
	return a.Artist == b.Artist || a.Meta(playlist.MetaAlbumID) != "" && a.Meta(playlist.MetaAlbumID) == b.Meta(playlist.MetaAlbumID)
}

// armCrossfade sets the overlap of the transition to next, before it is
// preloaded.
func (m *Model) armCrossfade(next playlist.Track) {
	cf, ok := m.player.(crossfader)
	if !ok {
		return
	}
	d := m.crossfade
	if cur, idx := m.currentPlaybackTrack(); idx >= 0 && sameAlbum(cur, next) {
		d = 0
	}
	cf.SetCrossfade(d)
}

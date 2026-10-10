// Package loudstore remembers the integrated loudness of tracks for the
// player's normalization. Local files are read from their ReplayGain tags
// when they have them; any other track's loudness is the measurement of an
// earlier play, kept in a small JSON file in the data directory.
package loudstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dhowden/tag"

	"github.com/xaviiromf/cliamp-x/internal/sponsorblock"
)

// replayGainReference is the loudness ReplayGain 2.0 levels to: a track
// tagged with gain g measures -18 - g LUFS.
const replayGainReference = -18.0

// maxEntries caps the remembered measurements; the oldest go first.
const maxEntries = 20000

// saveDelay groups the writes of a run of finished tracks.
const saveDelay = 5 * time.Second

type fileFormat struct {
	Version int                `json:"version"`
	Tracks  map[string]float64 `json:"tracks"`
	Order   []string           `json:"order,omitempty"`
}

// Store implements player.LoudnessStore. It is safe for concurrent use.
type Store struct {
	path string

	mu       sync.Mutex
	tracks   map[string]float64
	order    []string // keys oldest first
	tags     map[string]tagGain
	saveTime *time.Timer
}

type tagGain struct {
	track, album float64
	hasTrack     bool
	hasAlbum     bool
}

// Open loads the store at path. A missing or unreadable file starts empty.
func Open(path string) *Store {
	s := &Store{path: path, tracks: map[string]float64{}, tags: map[string]tagGain{}}
	if data, err := os.ReadFile(path); err == nil {
		var f fileFormat
		if json.Unmarshal(data, &f) == nil && f.Tracks != nil {
			s.tracks = f.Tracks
			for _, k := range f.Order {
				if _, ok := s.tracks[k]; ok {
					s.order = append(s.order, k)
				}
			}
			if len(s.order) != len(s.tracks) {
				s.order = s.order[:0]
				for k := range s.tracks {
					s.order = append(s.order, k)
				}
			}
		}
	}
	return s
}

// key identifies a track: a YouTube video by its ID, so every URL form of
// it shares one entry, and anything else by its path.
func key(path string) string {
	if id := sponsorblock.VideoID(path); id != "" {
		return "yt:" + id
	}
	return path
}

func isLocal(path string) bool {
	return path != "" && !strings.Contains(path, "://")
}

// Loudness returns the loudness of path: from ReplayGain tags for a local
// file that has them (album gain when album is set and present), otherwise
// from an earlier measurement.
func (s *Store) Loudness(path string, album bool) (float64, bool) {
	if isLocal(path) {
		if g := s.tagGain(path); g.hasTrack || g.hasAlbum {
			if album && g.hasAlbum {
				return replayGainReference - g.album, true
			}
			if g.hasTrack {
				return replayGainReference - g.track, true
			}
			return replayGainReference - g.album, true
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.tracks[key(path)]
	return v, ok
}

// StoreLoudness remembers a measurement and saves the file shortly after.
func (s *Store) StoreLoudness(path string, lufs float64) {
	k := key(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tracks[k]; !ok {
		s.order = append(s.order, k)
	}
	s.tracks[k] = lufs
	for len(s.order) > maxEntries {
		delete(s.tracks, s.order[0])
		s.order = s.order[1:]
	}
	if s.path == "" {
		return
	}
	if s.saveTime == nil {
		s.saveTime = time.AfterFunc(saveDelay, func() { _ = s.Save() })
	} else {
		s.saveTime.Reset(saveDelay)
	}
}

// Save writes the store now.
func (s *Store) Save() error {
	s.mu.Lock()
	f := fileFormat{Version: 1, Tracks: make(map[string]float64, len(s.tracks)), Order: append([]string(nil), s.order...)}
	for k, v := range s.tracks {
		f.Tracks[k] = v
	}
	s.mu.Unlock()
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("loudstore: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("loudstore: %w", err)
	}
	return os.Rename(tmp, s.path)
}

// tagGain reads and caches the ReplayGain tags of a local file.
func (s *Store) tagGain(path string) tagGain {
	s.mu.Lock()
	g, ok := s.tags[path]
	s.mu.Unlock()
	if ok {
		return g
	}
	g = readReplayGain(path)
	s.mu.Lock()
	s.tags[path] = g
	s.mu.Unlock()
	return g
}

func readReplayGain(path string) tagGain {
	var g tagGain
	f, err := os.Open(path)
	if err != nil {
		return g
	}
	defer f.Close()
	m, err := tag.ReadFrom(f)
	if err != nil || m == nil {
		return g
	}
	for k, v := range m.Raw() {
		name, text := strings.ToLower(k), ""
		switch val := v.(type) {
		case string:
			text = val
		case *tag.Comm:
			name, text = strings.ToLower(val.Description), val.Text
		case []string:
			if len(val) > 0 {
				text = val[0]
			}
		default:
			continue
		}
		db, ok := parseGain(text)
		if !ok {
			continue
		}
		switch {
		case strings.Contains(name, "replaygain_track_gain"):
			g.track, g.hasTrack = db, true
		case strings.Contains(name, "replaygain_album_gain"):
			g.album, g.hasAlbum = db, true
		}
	}
	return g
}

// parseGain reads a ReplayGain value such as "-7.43 dB".
func parseGain(s string) (float64, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(strings.ToLower(s)), "db"))
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v < -60 || v > 60 {
		return 0, false
	}
	return v, true
}

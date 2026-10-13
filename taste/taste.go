// Package taste keeps what the listener wants more and less of: the songs,
// albums, artists and genres marked "I'm interested" (I) or "not
// interested" (H), and the words and artists that hide music outright. Any
// provider can read it to rank and filter what it recommends.
//
// The store lives in taste.json in the config directory. It is safe for
// concurrent callers and writes atomically.
package taste

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/appdir"
	"github.com/xaviiromf/cliamp-x/internal/fileutil"
)

// Kind is what a rating is about.
type Kind string

const (
	Song   Kind = "song"
	Album  Kind = "album"
	Artist Kind = "artist"
	Genre  Kind = "genre"
	List   Kind = "list" // a playlist or another browse row
)

// Interest values.
const (
	NotInterested = -1
	Neutral       = 0
	Interested    = 1
)

// Item is one rated thing.
type Item struct {
	Kind   Kind      `json:"kind"`
	Name   string    `json:"name"`
	Artist string    `json:"artist,omitempty"`
	At     time.Time `json:"at"`
}

// DefaultBlockedWords hide genres the listener does not want, matched as
// whole words in titles, genre names and artist names. They are written to
// taste.json the first time, where the listener can edit them.
var DefaultBlockedWords = []string{
	"reggaeton", "reggaetón", "reguetón", "regueton", "urbano", "urbana", "latin urban",
	"trap", "trap latino", "latin trap", "bachata", "salsa", "dembow", "perreo",
}

// DefaultBlockedArtists are well-known artists of the blocked genres, so the
// recommendations skip them before any of them is rated. taste.json keeps
// the list editable.
var DefaultBlockedArtists = []string{
	// Reggaeton and Latin urban.
	"Bad Bunny", "J Balvin", "Daddy Yankee", "Ozuna", "Anuel AA", "Karol G", "Maluma", "Nicky Jam",
	"Farruko", "Feid", "Rauw Alejandro", "Myke Towers", "Arcángel", "De La Ghetto", "Wisin", "Yandel",
	"Wisin & Yandel", "Don Omar", "Sech", "Jhayco", "Jhay Cortez", "Lunay", "Bryant Myers",
	"Eladio Carrión", "Young Miko", "Dei V", "Ñengo Flow", "Cosculluela", "Tito El Bambino",
	"Zion & Lennox", "Plan B", "Lenny Tavárez", "Natti Natasha", "Jowell & Randy", "Justin Quiles",
	"Dalex", "Lyanno", "Rafa Pabön", "Jay Wheeler", "Brray", "Kevvo", "Chencho Corleone", "El Alfa",
	"Tokischa", "Manuel Turizo", "Blessd", "Ryan Castro", "Ovy On The Drums",
	// Latin trap.
	"Duki", "Khea", "Paulo Londra", "Cazzu", "Tiago PZK", "Nicki Nicole", "Rels B", "Quevedo",
	"Pablo Chill-E", "Cris MJ", "Jere Klein", "Marcianeke", "Polimá Westcoast", "Julianno Sosa",
	"Young Cister", "Harry Nach", "Paloma Mami", "Standly", "Gino Mella", "Drefquila", "Jairo Vera",
	"Floyymenor", "Ak4:20", "Kidd Voodoo",
	// Bachata.
	"Romeo Santos", "Aventura", "Prince Royce", "Anthony Santos", "Frank Reyes", "Zacarías Ferreira",
	// Salsa.
	"Marc Anthony", "Grupo Niche", "Celia Cruz", "Héctor Lavoe", "Willie Colón", "Gilberto Santa Rosa",
	"Víctor Manuelle", "Frankie Ruiz", "Oscar D'León", "Fruko y sus Tesos", "Joe Arroyo",
	"Guayacán Orquesta", "Jerry Rivera", "Eddie Santiago", "Tito Rojas",
}

type data struct {
	Interested     map[string]Item `json:"interested"`
	NotInterested  map[string]Item `json:"not_interested"`
	BlockedWords   []string        `json:"blocked_words"`
	BlockedArtists []string        `json:"blocked_artists"`
}

// Store reads and writes taste.json.
type Store struct {
	path string

	mu      sync.Mutex
	d       data
	words   *regexp.Regexp // the blocked words and the hidden artists, as whole words
	artists []string       // lowercase names of hidden and blocked artists
}

// New returns the store in the config directory. Without a config directory
// it keeps the ratings in memory.
func New() *Store {
	path := ""
	if dir, err := appdir.Dir(); err == nil {
		path = filepath.Join(dir, "taste.json")
	}
	return NewAt(path)
}

// NewAt returns a store backed by path, or kept in memory when path is "".
func NewAt(path string) *Store {
	s := &Store{path: path}
	s.load()
	return s
}

func (s *Store) load() {
	s.d = data{}
	fresh := true
	if s.path != "" {
		if raw, err := os.ReadFile(s.path); err == nil {
			fresh = json.Unmarshal(raw, &s.d) != nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			fresh = false // unreadable: do not write the defaults over it
		}
	}
	if s.d.Interested == nil {
		s.d.Interested = map[string]Item{}
	}
	if s.d.NotInterested == nil {
		s.d.NotInterested = map[string]Item{}
	}
	if fresh {
		s.d.BlockedWords = slices.Clone(DefaultBlockedWords)
		s.d.BlockedArtists = slices.Clone(DefaultBlockedArtists)
		_ = s.saveLocked()
	}
	s.rebuildLocked()
}

// rebuildLocked compiles the matchers of the hidden words and artists.
func (s *Store) rebuildLocked() {
	var alts []string
	s.artists = s.artists[:0]
	add := func(w string) {
		if w = strings.ToLower(strings.TrimSpace(w)); w != "" {
			alts = append(alts, regexp.QuoteMeta(w))
		}
	}
	for _, w := range s.d.BlockedWords {
		add(w)
	}
	for _, a := range s.d.BlockedArtists {
		add(a)
		s.artists = append(s.artists, strings.ToLower(a))
	}
	for _, it := range s.d.NotInterested {
		if it.Kind == Artist {
			add(it.Name)
			s.artists = append(s.artists, strings.ToLower(it.Name))
		}
	}
	s.words = nil
	if len(alts) > 0 {
		// Letters on either side end a match, so "trap" does not hide "trapped".
		s.words = regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}])(` + strings.Join(alts, "|") + `)($|[^\p{L}\p{N}])`)
	}
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	return fileutil.WriteFileAtomic(s.path, append(raw, '\n'), 0o600)
}

// Rate records interest in id: Interested, NotInterested or Neutral to
// forget it. It returns the interest now stored.
func (s *Store) Rate(id string, it Item, interest int) (int, error) {
	if s == nil || id == "" {
		return Neutral, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.d.Interested, id)
	delete(s.d.NotInterested, id)
	it.At = time.Now()
	switch {
	case interest > 0:
		s.d.Interested[id] = it
	case interest < 0:
		s.d.NotInterested[id] = it
	}
	s.rebuildLocked()
	return max(-1, min(1, interest)), s.saveLocked()
}

// Interest returns the stored interest in id.
func (s *Store) Interest(id string) int {
	if s == nil {
		return Neutral
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.d.Interested[id]; ok {
		return Interested
	}
	if _, ok := s.d.NotInterested[id]; ok {
		return NotInterested
	}
	return Neutral
}

// Hidden reports whether something must not be recommended: one of ids is
// rated "not interested", or one of texts (a title, an artist, a genre)
// holds a blocked word or a hidden artist.
func (s *Store) Hidden(ids []string, texts ...string) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if _, ok := s.d.NotInterested[id]; ok && id != "" {
			return true
		}
	}
	if s.words == nil {
		return false
	}
	for _, t := range texts {
		if t != "" && s.words.MatchString(t) {
			return true
		}
	}
	return false
}

// HiddenArtist reports whether an artist name is blocked or rated "not
// interested".
func (s *Store) HiddenArtist(name string) bool {
	if s == nil || name == "" {
		return false
	}
	name = strings.ToLower(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Contains(s.artists, name)
}

// Liked returns the IDs rated "interested" of one kind, newest first.
func (s *Store) Liked(kind Kind) []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	type rated struct {
		id string
		at time.Time
	}
	var out []rated
	for id, it := range s.d.Interested {
		if it.Kind == kind {
			out = append(out, rated{id, it.At})
		}
	}
	slices.SortFunc(out, func(a, b rated) int { return b.at.Compare(a.at) })
	ids := make([]string, len(out))
	for i, r := range out {
		ids[i] = r.id
	}
	return ids
}

// Path returns the file of the store, or "" in memory.
func (s *Store) Path() string { return s.path }

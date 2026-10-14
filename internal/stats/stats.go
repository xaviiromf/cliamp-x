// Package stats keeps a log of what was listened to and sums it up for the
// statistics screen: plays, listening time, top artists, songs and genres,
// daily activity and the listening streak. Everything stays on this
// computer.
package stats

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Play is one listened song.
type Play struct {
	At     time.Time `json:"t"`
	Key    string    `json:"k"` // the track's path
	Title  string    `json:"ti,omitempty"`
	Artist string    `json:"ar,omitempty"`
	Album  string    `json:"al,omitempty"`
	Genre  string    `json:"g,omitempty"`
	Secs   float64   `json:"s"` // seconds listened
}

// Log appends plays to a JSON-lines file. It is safe for concurrent use.
type Log struct {
	path string
	mu   sync.Mutex
}

// Open returns the log at path.
func Open(path string) *Log { return &Log{path: path} }

// Record appends a play.
func (l *Log) Record(p Play) error {
	if l == nil || l.path == "" {
		return nil
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Load reads every play. Broken lines are skipped.
func (l *Log) Load() ([]Play, error) {
	if l == nil || l.path == "" {
		return nil, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	f, err := os.Open(l.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var plays []Play
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		var p Play
		if json.Unmarshal(sc.Bytes(), &p) == nil && !p.At.IsZero() {
			plays = append(plays, p)
		}
	}
	return plays, sc.Err()
}

// Count is one ranked entry: a name and its plays and seconds.
type Count struct {
	Name   string
	Detail string // the artist of a song
	Plays  int
	Secs   float64
}

// Summary sums the plays of a period.
type Summary struct {
	Plays      int
	Secs       float64
	Songs      int // distinct songs
	Artists    int // distinct artists
	TopArtists []Count
	TopSongs   []Count
	TopGenres  []Count
	// Daily holds the minutes listened on each of the last len(Daily) days,
	// oldest first, today last.
	Daily  []float64
	Streak int // days in a row up to today (or yesterday) with plays
}

// Period is a span the statistics screen shows.
type Period int

const (
	Week Period = iota
	Month
	Year
	All
)

// Start returns the first moment of the period at now; All starts at zero.
func (p Period) Start(now time.Time) time.Time {
	switch p {
	case Week:
		return now.AddDate(0, 0, -7)
	case Month:
		return now.AddDate(0, -1, 0)
	case Year:
		return now.AddDate(-1, 0, 0)
	}
	return time.Time{}
}

// Summarize sums plays from the period's start until now. top caps each
// ranking, days sets the length of Daily.
func Summarize(plays []Play, period Period, now time.Time, top, days int) Summary {
	from := period.Start(now)
	var s Summary
	artists := map[string]*Count{}
	songs := map[string]*Count{}
	genres := map[string]*Count{}
	for _, p := range plays {
		if p.At.Before(from) || p.At.After(now) {
			continue
		}
		s.Plays++
		s.Secs += p.Secs
		if p.Artist != "" {
			bump(artists, strings.ToLower(p.Artist), p.Artist, "", p.Secs)
		}
		bump(songs, p.Key, p.title(), p.Artist, p.Secs)
		if p.Genre != "" {
			bump(genres, strings.ToLower(p.Genre), p.Genre, "", p.Secs)
		}
	}
	s.Songs, s.Artists = len(songs), len(artists)
	s.TopArtists = ranked(artists, top)
	s.TopSongs = ranked(songs, top)
	s.TopGenres = ranked(genres, top)
	s.Daily, s.Streak = daily(plays, now, days)
	return s
}

// bump counts one play of secs under key in m, named name.
func bump(m map[string]*Count, key, name, detail string, secs float64) {
	c, ok := m[key]
	if !ok {
		c = &Count{Name: name, Detail: detail}
		m[key] = c
	}
	c.Plays++
	c.Secs += secs
}

// title is the song's title, or the base name of its path.
func (p Play) title() string {
	if p.Title != "" {
		return p.Title
	}
	return filepath.Base(p.Key)
}

// Wrap is the listening of one calendar year, for the yearly story.
type Wrap struct {
	Summary              // the year's totals and rankings; Streak is the longest
	Year         int     // the calendar year
	Days         int     // days with plays
	TopAlbums    []Count // Detail is the album's artist
	First        *Play   // the first song of the year
	MostReplayed *Count  // the song played most times in one day, with that day's plays
}

// Wrapped sums the plays of year in loc: Summarize's totals and rankings,
// the top albums, the days with plays, the longest streak, the first song,
// and the song replayed most in one day. top caps each ranking.
func Wrapped(plays []Play, year int, loc *time.Location, top int) Wrap {
	from := time.Date(year, 1, 1, 0, 0, 0, 0, loc)
	to := from.AddDate(1, 0, 0)
	var in []Play
	for _, p := range plays {
		if !p.At.Before(from) && p.At.Before(to) {
			in = append(in, p)
		}
	}
	w := Wrap{Summary: Summarize(in, All, to, top, 0), Year: year}
	albums := map[string]*Count{}
	perDay := map[string]*Count{} // a song on one day
	days := map[time.Time]bool{}
	for i, p := range in {
		if w.First == nil || p.At.Before(w.First.At) {
			w.First = &in[i]
		}
		if p.Album != "" {
			bump(albums, strings.ToLower(p.Album+"\x00"+p.Artist), p.Album, p.Artist, p.Secs)
		}
		t := p.At.In(loc)
		day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		days[day] = true
		bump(perDay, day.Format(time.DateOnly)+"\x00"+p.Key, p.title(), p.Artist, p.Secs)
	}
	w.TopAlbums = ranked(albums, top)
	if best := ranked(perDay, 1); len(best) == 1 {
		w.MostReplayed = &best[0]
	}
	w.Days = len(days)
	w.Streak = 0
	for day := range days {
		if days[day.AddDate(0, 0, -1)] {
			continue // not the first day of a run
		}
		run := 0
		for d := day; days[d]; d = d.AddDate(0, 0, 1) {
			run++
		}
		w.Streak = max(w.Streak, run)
	}
	return w
}

func ranked(m map[string]*Count, top int) []Count {
	out := make([]Count, 0, len(m))
	for _, c := range m {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Plays != out[j].Plays {
			return out[i].Plays > out[j].Plays
		}
		if out[i].Secs != out[j].Secs {
			return out[i].Secs > out[j].Secs
		}
		return out[i].Name < out[j].Name
	})
	if len(out) > top {
		out = out[:top]
	}
	return out
}

// daily returns the minutes of each of the last days days and the streak.
func daily(plays []Play, now time.Time, days int) ([]float64, int) {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	minutes := map[time.Time]float64{}
	for _, p := range plays {
		t := p.At.In(now.Location())
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
		minutes[d] += p.Secs / 60
	}
	out := make([]float64, days)
	for i := range days {
		out[i] = minutes[today.AddDate(0, 0, i-days+1)]
	}
	// A streak still counts when today has no plays yet.
	day := today
	if minutes[day] == 0 {
		day = day.AddDate(0, 0, -1)
	}
	streak := 0
	for minutes[day] > 0 {
		streak++
		day = day.AddDate(0, 0, -1)
	}
	return out, streak
}

package stats

import (
	"path/filepath"
	"testing"
	"time"
)

func TestLogRoundTrip(t *testing.T) {
	l := Open(filepath.Join(t.TempDir(), "plays.jsonl"))
	at := time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)
	if err := l.Record(Play{At: at, Key: "k", Title: "Song", Artist: "A", Secs: 200}); err != nil {
		t.Fatal(err)
	}
	if err := l.Record(Play{At: at.Add(time.Hour), Key: "k", Secs: 100}); err != nil {
		t.Fatal(err)
	}
	plays, err := l.Load()
	if err != nil || len(plays) != 2 || plays[0].Title != "Song" || plays[1].Secs != 100 {
		t.Fatalf("Load = %+v, %v", plays, err)
	}
}

func TestSummarize(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return now.AddDate(0, 0, -d) }
	plays := []Play{
		{At: day(0), Key: "s1", Title: "One", Artist: "Alpha", Genre: "Rock", Secs: 180},
		{At: day(1), Key: "s1", Title: "One", Artist: "Alpha", Genre: "Rock", Secs: 180},
		{At: day(2), Key: "s2", Title: "Two", Artist: "beta", Secs: 120},
		{At: day(2), Key: "s3", Title: "Three", Artist: "Beta", Secs: 60},
		{At: day(40), Key: "s4", Title: "Old", Artist: "Gamma", Secs: 300},
	}
	week := Summarize(plays, Week, now, 10, 7)
	if week.Plays != 4 || week.Secs != 540 || week.Songs != 3 || week.Artists != 2 {
		t.Fatalf("week = %+v", week)
	}
	// Artist names group regardless of case, ties break by time.
	if week.TopArtists[0].Name != "Alpha" || week.TopArtists[1].Plays != 2 {
		t.Fatalf("top artists = %+v", week.TopArtists)
	}
	if week.TopSongs[0].Name != "One" || week.TopSongs[0].Detail != "Alpha" || week.TopSongs[0].Plays != 2 {
		t.Fatalf("top songs = %+v", week.TopSongs)
	}
	if len(week.TopGenres) != 1 || week.TopGenres[0].Name != "Rock" {
		t.Fatalf("genres = %+v", week.TopGenres)
	}
	if week.Streak != 3 || len(week.Daily) != 7 || week.Daily[6] != 3 || week.Daily[4] != 3 {
		t.Fatalf("daily = %v streak %d", week.Daily, week.Streak)
	}
	if all := Summarize(plays, All, now, 10, 7); all.Plays != 5 {
		t.Fatalf("all plays = %d", all.Plays)
	}
}

func TestStreakSurvivesQuietToday(t *testing.T) {
	now := time.Date(2026, 10, 10, 9, 0, 0, 0, time.UTC)
	plays := []Play{{At: now.AddDate(0, 0, -1), Key: "a", Secs: 60}, {At: now.AddDate(0, 0, -2), Key: "a", Secs: 60}}
	if s := Summarize(plays, All, now, 5, 3); s.Streak != 2 {
		t.Fatalf("streak = %d, want 2 when only today is empty", s.Streak)
	}
}

func TestWrapped(t *testing.T) {
	loc := time.FixedZone("CLT", -3*3600)
	at := func(month, day, hour int) time.Time {
		return time.Date(2025, time.Month(month), day, hour, 0, 0, 0, loc)
	}
	plays := []Play{
		{At: time.Date(2024, 12, 31, 23, 0, 0, 0, loc), Key: "old", Title: "Last Year", Artist: "Z", Secs: 100},
		{At: at(1, 2, 10), Key: "s2", Title: "Two", Artist: "Beta", Album: "B-Sides", Genre: "Pop", Secs: 120},
		{At: at(1, 1, 9), Key: "s1", Title: "One", Artist: "Alpha", Album: "First", Genre: "Rock", Secs: 180},
		{At: at(1, 3, 9), Key: "s1", Title: "One", Artist: "Alpha", Album: "First", Secs: 180},
		{At: at(3, 5, 9), Key: "s3", Title: "Three", Artist: "Alpha", Album: "first", Secs: 60},
		{At: at(3, 5, 10), Key: "s3", Title: "Three", Artist: "Alpha", Album: "first", Secs: 60},
		{At: at(3, 5, 11), Key: "s3", Title: "Three", Artist: "Alpha", Album: "first", Secs: 60},
		{At: at(3, 6, 9), Key: "s1", Title: "One", Artist: "Alpha", Album: "First", Secs: 180},
		// 2026-01-01 00:30 in Chile is still 2025 in UTC: the zone decides.
		{At: time.Date(2026, 1, 1, 3, 30, 0, 0, time.UTC), Key: "new", Title: "Next Year", Artist: "Y", Secs: 100},
	}
	w := Wrapped(plays, 2025, loc, 10)
	if w.Year != 2025 || w.Plays != 7 || w.Secs != 840 || w.Songs != 3 || w.Artists != 2 || w.Days != 5 {
		t.Fatalf("totals = %+v", w)
	}
	if w.Streak != 3 {
		t.Errorf("longest streak = %d, want 3 (Jan 1 to 3)", w.Streak)
	}
	if w.First == nil || w.First.Key != "s1" || !w.First.At.Equal(at(1, 1, 9)) {
		t.Errorf("first = %+v, want One on Jan 1", w.First)
	}
	if w.MostReplayed == nil || w.MostReplayed.Name != "Three" || w.MostReplayed.Plays != 3 {
		t.Errorf("most replayed = %+v, want Three 3 times on Mar 5", w.MostReplayed)
	}
	// Album names group regardless of case, with the album's artist.
	if len(w.TopAlbums) != 2 || w.TopAlbums[0].Name != "First" || w.TopAlbums[0].Detail != "Alpha" || w.TopAlbums[0].Plays != 6 {
		t.Errorf("top albums = %+v", w.TopAlbums)
	}
	if w.TopSongs[0].Name != "One" || w.TopArtists[0].Name != "Alpha" || len(w.TopGenres) != 2 {
		t.Errorf("rankings = %+v %+v %+v", w.TopSongs, w.TopArtists, w.TopGenres)
	}
	if empty := Wrapped(plays, 2020, loc, 10); empty.Plays != 0 || empty.First != nil || empty.MostReplayed != nil || empty.Streak != 0 {
		t.Errorf("empty year = %+v", empty)
	}
}

// A streak runs across the day DST starts at local midnight
// (America/Santiago skips 2026-09-06 00:00).
func TestStreakAcrossDSTStart(t *testing.T) {
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	var plays []Play
	for d := 1; d <= 30; d++ {
		plays = append(plays, Play{At: time.Date(2026, 9, d, 12, 0, 0, 0, loc), Key: "a", Secs: 60})
	}
	if w := Wrapped(plays, 2026, loc, 5); w.Days != 30 || w.Streak != 30 {
		t.Errorf("wrapped: days %d streak %d, want 30 30", w.Days, w.Streak)
	}
	now := time.Date(2026, 9, 30, 20, 0, 0, 0, loc)
	if s := Summarize(plays, All, now, 5, 30); s.Streak != 30 || s.Daily[0] != 1 || s.Daily[29] != 1 {
		t.Errorf("summary: streak %d daily %v, want 30 and one minute a day", s.Streak, s.Daily)
	}
}

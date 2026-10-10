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

package ytmusic

import (
	"sync/atomic"
	"testing"
	"time"
)

// A start shows the lists of the last run at once and refreshes them behind,
// and a snapshot of the other sign-in state is ignored.
func TestBrowseSnapshotFillsTheFirstRead(t *testing.T) {
	dir := t.TempDir()
	var calls atomic.Int32
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any, string) string{
		"browse": func(map[string]any, string) string { calls.Add(1); return "{}" },
	}}

	first := New(f.client(&Auth{}), Options{DataDir: dir})
	first.mu.Lock()
	first.snapLoaded = true
	first.home, first.homeAt = []Shelf{{Title: "Saved shelf"}}, time.Now().Add(-48*time.Hour)
	first.cats, first.catsAt = []Category{{Title: "Rock", Params: "p"}}, time.Now().Add(-48*time.Hour)
	first.mu.Unlock()
	first.writeSnapshot(first.snapshotPath(), 1, browseSnapshot{Home: first.home, HomeAt: first.homeAt, Cats: first.cats, CatsAt: first.catsAt})

	p := New(f.client(&Auth{}), Options{DataDir: dir})
	start := time.Now()
	home := p.cachedHome()
	if len(home) != 1 || home[0].Title != "Saved shelf" {
		t.Fatalf("first read = %+v, want the saved shelf", home)
	}
	if cats := p.cachedCategories(); len(cats) != 1 || cats[0].Title != "Rock" {
		t.Fatalf("categories = %+v, want the saved ones", cats)
	}
	if time.Since(start) > time.Second {
		t.Fatal("the saved lists must not wait for the network")
	}
	deadline := time.Now().Add(5 * time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if calls.Load() < 2 {
		t.Fatalf("%d background refreshes, want 2", calls.Load())
	}

	signedIn := New(f.client(testAuth()), Options{DataDir: t.TempDir()})
	signedIn.opts.DataDir = dir
	signedIn.mu.Lock()
	signedIn.loadSnapshotLocked()
	got := signedIn.home
	signedIn.mu.Unlock()
	if got != nil {
		t.Fatal("a signed-out snapshot must not fill a signed-in provider")
	}
}

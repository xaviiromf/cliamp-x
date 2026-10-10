package lyrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// withLyricsServers points LRCLIB and NetEase at a server that answers
// LRCLIB with body and NetEase with nothing.
func withLyricsServers(t *testing.T, lrclibBody string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/search" && lrclibBody != "" {
			_, _ = io.WriteString(w, lrclibBody)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(srv.Close)
	oldL, oldN := lrclibBaseURL, neteaseBaseURL
	lrclibBaseURL, neteaseBaseURL = srv.URL, srv.URL
	t.Cleanup(func() { lrclibBaseURL, neteaseBaseURL = oldL, oldN })
}

func plainSource(ctx context.Context) ([]Line, error) {
	return []Line{{Text: "plain one"}, {Text: "plain two"}}, nil
}

func TestLookupPrefersSyncedOverPlainSource(t *testing.T) {
	withLyricsServers(t, `[{"syncedLyrics":"[00:01.00]synced one\n[00:02.00]synced two"}]`)
	lines, err := Lookup(context.Background(), "", "Artist", "Song", plainSource)
	if err != nil || !Synced(lines) || lines[0].Text != "synced one" {
		t.Fatalf("Lookup = %+v, %v; want LRCLIB's synced lyrics", lines, err)
	}
}

func TestLookupFallsBackToPlainSource(t *testing.T) {
	withLyricsServers(t, "")
	lines, err := Lookup(context.Background(), "", "Artist", "Song", plainSource)
	if err != nil || len(lines) != 2 || lines[0].Text != "plain one" {
		t.Fatalf("Lookup = %+v, %v; want the plain source", lines, err)
	}
}

func TestLookupSyncedSourceWins(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
	defer srv.Close()
	oldL, oldN := lrclibBaseURL, neteaseBaseURL
	lrclibBaseURL, neteaseBaseURL = srv.URL, srv.URL
	defer func() { lrclibBaseURL, neteaseBaseURL = oldL, oldN }()
	synced := func(context.Context) ([]Line, error) { return []Line{{Start: time.Second, Text: "x"}}, nil }
	if lines, err := Lookup(context.Background(), "", "A", "S", synced); err != nil || len(lines) != 1 || called {
		t.Fatalf("Lookup = %+v, %v, called %v; want the synced source without a network call", lines, err, called)
	}
}

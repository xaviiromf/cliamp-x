package sponsorblock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVideoID(t *testing.T) {
	for in, want := range map[string]string{
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ&list=x": "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ":          "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ":                         "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":           "dQw4w9WgXcQ",
		"https://example.com/watch?v=dQw4w9WgXcQ":              "",
		"/music/a.flac":                         "",
		"https://www.youtube.com/watch?v=short": "",
	} {
		if got := VideoID(in); got != want {
			t.Errorf("VideoID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSegmentsPrivacyAndFiltering(t *testing.T) {
	const id = "dQw4w9WgXcQ"
	sum := sha256.Sum256([]byte(id))
	prefix := hex.EncodeToString(sum[:])[:4]
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/skipSegments/"+prefix {
			t.Errorf("path = %q, want only the hash prefix", r.URL.Path)
		}
		if strings.Contains(r.URL.RawQuery, id) {
			t.Error("the video ID must not be sent")
		}
		_, _ = io.WriteString(w, `[
			{"videoID":"otherVideo1","segments":[{"category":"sponsor","actionType":"skip","segment":[1,2]}]},
			{"videoID":"`+id+`","segments":[
				{"category":"outro","actionType":"skip","segment":[200,212.5],"UUID":"c"},
				{"category":"intro","actionType":"skip","segment":[0,8],"UUID":"a"},
				{"category":"sponsor","actionType":"skip","segment":[7.8,15],"UUID":"b"},
				{"category":"filler","actionType":"skip","segment":[50,60]},
				{"category":"sponsor","actionType":"mute","segment":[90,95]}
			]}]`)
	}))
	defer srv.Close()
	c := New(nil)
	c.API = srv.URL
	segs, err := c.Segments(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	// filler is not a default category, mute is not a skip, and the
	// overlapping intro and sponsor merge into one skip.
	if len(segs) != 2 || segs[0].Start != 0 || segs[0].End != 15 || segs[1].Category != "outro" {
		t.Fatalf("segments = %+v", segs)
	}
	if _, err := c.Segments(context.Background(), id); err != nil || calls != 1 {
		t.Fatalf("second lookup: err %v, calls %d; want the cache", err, calls)
	}
}

func TestSegmentsNotFound(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	c := New(nil)
	c.API = srv.URL
	segs, err := c.Segments(context.Background(), "dQw4w9WgXcQ")
	if err != nil || len(segs) != 0 {
		t.Fatalf("Segments on 404 = %v, %v; want none and no error", segs, err)
	}
	if _, ok := c.Cached("dQw4w9WgXcQ"); !ok {
		t.Fatal("an empty answer must be cached")
	}
}

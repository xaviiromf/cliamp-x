package artwork

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSize(t *testing.T) {
	tests := []struct {
		name             string
		w, h             int
		maxCols, maxRows int
		cols, rows       int
	}{
		{"square fills the height", 100, 100, 40, 10, 20, 10},
		{"16:9 fills the height", 160, 90, 40, 10, 36, 10},
		{"16:9 capped by the width", 160, 90, 20, 10, 20, 6},
		{"4:3", 120, 90, 40, 9, 24, 9},
		{"portrait", 90, 160, 40, 10, 11, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cols, rows := Size(image.NewRGBA(image.Rect(0, 0, tt.w, tt.h)), tt.maxCols, tt.maxRows)
			if cols != tt.cols || rows != tt.rows {
				t.Fatalf("Size = %dx%d, want %dx%d", cols, rows, tt.cols, tt.rows)
			}
		})
	}
}

func TestRender(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	lines := Render(img, 6, 3)
	if len(lines) != 3 {
		t.Fatalf("Render lines = %d, want 3", len(lines))
	}
	for _, l := range lines {
		if n := strings.Count(l, "\u2580"); n != 6 {
			t.Fatalf("Render line has %d cells, want 6", n)
		}
		if !strings.HasSuffix(l, "\x1b[0m") {
			t.Fatal("Render line must reset its colours")
		}
	}
}

func TestLoadCachesOnDiskAndInMemory(t *testing.T) {
	data := testPNG(t, 400, 225)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write(data)
	}))
	defer srv.Close()
	url := srv.URL + "/thumb.jpg"
	dir := t.TempDir()

	c := New(dir)
	if !c.Claim(url) {
		t.Fatal("Claim of a new url = false")
	}
	if c.Claim(url) {
		t.Fatal("Claim of a loading url = true, want the load deduplicated")
	}
	if err := c.Load(context.Background(), url); err != nil {
		t.Fatal(err)
	}
	img, ok := c.Get(url)
	if !ok {
		t.Fatal("Get after Load = false")
	}
	if b := img.Bounds(); max(b.Dx(), b.Dy()) > maxPixels {
		t.Fatalf("stored image %dx%d, want at most %d pixels a side", b.Dx(), b.Dy(), maxPixels)
	}
	if c.Claim(url) {
		t.Fatal("Claim of a loaded url = true")
	}
	first, cols, rows, ok := c.Draw(url, 40, 10)
	if !ok || cols != 36 || rows != 10 {
		t.Fatalf("Draw = %dx%d %v, want 36x10 for a 16:9 image", cols, rows, ok)
	}
	if again, _, _, _ := c.Draw(url, 40, 10); &again[0] != &first[0] {
		t.Fatal("Draw at the same size must reuse its drawing")
	}

	// A new cache, as after a restart, reads the bytes from disk.
	fresh := New(dir)
	fresh.Claim(url)
	if err := fresh.Load(context.Background(), url); err != nil {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("server hits = %d, want 1: the second load must come from disk", n)
	}
}

func TestLoadFailureIsNotRetriedAtOnce(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	url := srv.URL + "/missing.jpg"
	c := New("")
	c.Claim(url)
	if err := c.Load(context.Background(), url); err == nil {
		t.Fatal("Load of a 404 = nil error")
	}
	if c.Claim(url) {
		t.Fatal("Claim right after a failure = true, want a pause before retrying")
	}
	if _, ok := c.Get(url); ok {
		t.Fatal("Get of a failed url = true")
	}
}

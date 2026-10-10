package ytmusic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/browsercookies"
)

func TestParseLink(t *testing.T) {
	tests := []struct {
		in       string
		kind     LinkKind
		video    string
		list     string
		browseID string
	}{
		{"https://music.youtube.com/watch?v=abcdefghijk&list=RDAMVMabcdefghijk", LinkVideo, "abcdefghijk", "RDAMVMabcdefghijk", ""},
		{"https://www.youtube.com/watch?v=abcdefghijk", LinkVideo, "abcdefghijk", "", ""},
		{"youtube.com/watch?v=abcdefghijk", LinkVideo, "abcdefghijk", "", ""},
		{"https://youtu.be/abcdefghijk?si=x", LinkVideo, "abcdefghijk", "", ""},
		{"https://www.youtube.com/shorts/abcdefghijk", LinkVideo, "abcdefghijk", "", ""},
		{"https://m.youtube.com/watch?v=abcdefghijk", LinkVideo, "abcdefghijk", "", ""},
		{"https://music.youtube.com/playlist?list=PLabc", LinkPlaylist, "", "PLabc", ""},
		{"https://www.youtube.com/playlist?list=VLPLabc", LinkPlaylist, "", "PLabc", ""},
		{"https://music.youtube.com/browse/MPREb_abc", LinkAlbum, "", "", "MPREb_abc"},
		{"https://music.youtube.com/browse/VLPLabc", LinkPlaylist, "", "PLabc", ""},
		{"https://music.youtube.com/channel/UCabc", LinkArtist, "", "", "UCabc"},
		{"https://www.youtube.com/@someband", LinkUnknown, "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			link, ok := ParseLink(tt.in)
			if !ok || link.Kind != tt.kind || link.VideoID != tt.video || link.PlaylistID != tt.list || link.BrowseID != tt.browseID {
				t.Fatalf("ParseLink = %+v ok=%v", link, ok)
			}
		})
	}
	for _, in := range []string{"soda stereo", "https://example.com/watch?v=abcdefghijk", "https://youtu.be/short", ""} {
		if link, ok := ParseLink(in); ok {
			t.Errorf("ParseLink(%q) = %+v, want no link", in, link)
		}
	}
	if got := VideoIDFromPath(WatchURL("abcdefghijk")); got != "abcdefghijk" {
		t.Errorf("VideoIDFromPath(WatchURL) = %q", got)
	}
}

func testAuth() *Auth {
	return NewAuth([]browsercookies.Cookie{
		{Host: ".youtube.com", Name: "SAPISID", Value: "sapi"},
		{Host: ".youtube.com", Name: "__Secure-3PAPISID", Value: "sapi3"},
		{Host: ".youtube.com", Name: "SID", Value: "sid"},
		{Host: ".youtube.com", Name: "OLD", Value: "x", Expires: 1},
		{Host: ".google.com", Name: "SID", Value: "google"},
	})
}

func TestAuthHeaders(t *testing.T) {
	a := testAuth()
	if !a.SignedIn() {
		t.Fatal("SignedIn = false")
	}
	cookie := a.CookieHeader()
	if !strings.Contains(cookie, "SAPISID=sapi") || !strings.Contains(cookie, "SID=sid") ||
		strings.Contains(cookie, "OLD=") || strings.Contains(cookie, "google") {
		t.Fatalf("CookieHeader = %q", cookie)
	}
	got := a.AuthorizationHeader(time.Unix(1700000000, 0))
	want := "SAPISIDHASH 1700000000_" + sapisidHash("1700000000", "sapi") + " SAPISID3PHASH 1700000000_" + sapisidHash("1700000000", "sapi3")
	if got != want {
		t.Fatalf("AuthorizationHeader = %q, want %q", got, want)
	}
	if h := sapisidHash("1700000000", "sapi"); len(h) != 40 {
		t.Fatalf("hash %q is not hex SHA-1", h)
	}
	var signedOut *Auth
	if signedOut.SignedIn() || signedOut.CookieHeader() != "" || signedOut.AuthorizationHeader(time.Now()) != "" {
		t.Fatal("a nil Auth must be signed out")
	}
}

func TestLocaleFromEnv(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "es_CL.UTF-8")
	if hl, gl := localeFromEnv(); hl != "es" || gl != "CL" {
		t.Fatalf("localeFromEnv = %s %s", hl, gl)
	}
	t.Setenv("LANG", "C.UTF-8")
	if hl, gl := localeFromEnv(); hl != "en" || gl != "US" {
		t.Fatalf("localeFromEnv(C) = %s %s", hl, gl)
	}
}

// fakeAPI serves InnerTube endpoints from a handler map and records the
// request headers.
type fakeAPI struct {
	t        *testing.T
	handlers map[string]func(body map[string]any, query string) string
	headers  []http.Header
}

func (f *fakeAPI) client(auth *Auth) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.headers = append(f.headers, r.Header.Clone())
		endpoint := strings.TrimPrefix(r.URL.Path, "/")
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(data, &body)
		h, ok := f.handlers[endpoint]
		if !ok {
			http.Error(w, `{"error":{"message":"no handler"}}`, http.StatusNotFound)
			return
		}
		if out := h(body, r.URL.RawQuery); out == "401" {
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			io.WriteString(w, out)
		}
	}))
	f.t.Cleanup(srv.Close)
	c := NewClient(auth, "es", "CL")
	c.base = srv.URL + "/"
	return c
}

func TestClientPlaylistFollowsContinuations(t *testing.T) {
	row := func(id, title string) string {
		return `{"musicResponsiveListItemRenderer":{"playlistItemData":{"videoId":"` + id + `"},"flexColumns":[{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":[{"text":"` + title + `"}]}}}]}}`
	}
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any, string) string{
		"browse": func(body map[string]any, query string) string {
			switch {
			case body["browseId"] == "VLPLtest":
				return `{"header":{"musicResponsiveHeaderRenderer":{"title":{"runs":[{"text":"Test list"}]}}},
				 "contents":{"twoColumnBrowseResultsRenderer":{"secondaryContents":{"sectionListRenderer":{"contents":[{"musicPlaylistShelfRenderer":{"contents":[` +
					row("aaaaaaaaaaa", "One") + `,{"continuationItemRenderer":{"continuationEndpoint":{"continuationCommand":{"token":"NEW"}}}}]}}]}}}}}`
			case body["continuation"] == "NEW":
				return `{"onResponseReceivedActions":[{"appendContinuationItemsAction":{"continuationItems":[` + row("bbbbbbbbbbb", "Two") + `]}}]}`
			}
			t.Errorf("unexpected browse %v %s", body, query)
			return `{}`
		},
	}}
	c := f.client(testAuth())
	page, err := c.Playlist(context.Background(), "PLtest", 0)
	if err != nil {
		t.Fatal(err)
	}
	if page.Title != "Test list" || len(page.Songs) != 2 || page.Songs[1].Title != "Two" {
		t.Fatalf("page = %+v", page)
	}
	h := f.headers[0]
	if !strings.HasPrefix(h.Get("Authorization"), "SAPISIDHASH ") || h.Get("X-Origin") != origin || !strings.Contains(h.Get("Cookie"), "SAPISID=sapi") {
		t.Fatalf("request headers = %v", h)
	}
}

func TestClientOldContinuationAndSignInErrors(t *testing.T) {
	artistRow := func(id, name string) string {
		return `{"musicResponsiveListItemRenderer":{"navigationEndpoint":{"browseEndpoint":{"browseId":"MPLA` + id + `","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_LIBRARY_ARTIST"}}}},
		 "flexColumns":[{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":[{"text":"` + name + `"}]}}}]}}`
	}
	f := &fakeAPI{t: t, handlers: map[string]func(map[string]any, string) string{
		"browse": func(body map[string]any, query string) string {
			if strings.Contains(query, "ctoken=OLD") {
				return `{"continuationContents":{"musicShelfContinuation":{"contents":[` + artistRow("UCb", "B") + `]}}}`
			}
			return `{"contents":{"singleColumnBrowseResultsRenderer":{"tabs":[{"tabRenderer":{"content":{"sectionListRenderer":{"contents":[{"musicShelfRenderer":{
			 "contents":[` + artistRow("UCa", "A") + `],"continuations":[{"nextContinuationData":{"continuation":"OLD"}}]}}]}}}}]}}}`
		},
		"like/like": func(map[string]any, string) string { return "401" },
	}}
	c := f.client(testAuth())
	artists, err := c.LibraryArtists(context.Background())
	if err != nil || len(artists) != 2 || artists[0].ID != "UCa" || artists[1].Title != "B" {
		t.Fatalf("artists = %+v, %v", artists, err)
	}
	if err := c.SetLike(context.Background(), "aaaaaaaaaaa", true); err == nil || !strings.Contains(err.Error(), "login") {
		t.Fatalf("SetLike with a refused sign-in = %v", err)
	}

	signedOut := f.client(&Auth{})
	if _, err := signedOut.LibraryArtists(context.Background()); !errors.Is(err, ErrSignInRequired) {
		t.Fatalf("signed-out library = %v, want ErrSignInRequired", err)
	}
}

package ytmusic

import (
	"encoding/json"
	"testing"
)

// decode parses a JSON fixture into the generic shape the parsers walk.
func decode(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return m
}

func browseRun(text, id, pageType string) string {
	return `{"text":"` + text + `","navigationEndpoint":{"browseEndpoint":{"browseId":"` + id +
		`","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_` + pageType + `"}}}}}`
}

const sep = `{"text":" • "}`

// songRowJSON is a search-style song row: title, then "Artist • Album •
// 3:21" with page links, then a play count.
var songRowJSON = `{
 "playlistItemData": {"videoId": "abcdefghijk"},
 "flexColumns": [
  {"musicResponsiveListItemFlexColumnRenderer": {"text": {"runs": [{"text": "Song One"}]}}},
  {"musicResponsiveListItemFlexColumnRenderer": {"text": {"runs": [` +
	browseRun("Band", "UCband", "ARTIST") + `,` + sep + `,` + browseRun("Record", "MPREb_rec", "ALBUM") + `,` + sep + `,{"text":"3:21"}]}}},
  {"musicResponsiveListItemFlexColumnRenderer": {"text": {"runs": [{"text": "1 M plays"}]}}}
 ],
 "menu": {"menuRenderer": {"topLevelButtons": [{"likeButtonRenderer": {"likeStatus": "LIKE"}}]}}
}`

func TestParseSongRow(t *testing.T) {
	s, ok := parseSongRow(decode(t, songRowJSON))
	if !ok {
		t.Fatal("row not parsed")
	}
	if s.VideoID != "abcdefghijk" || s.Title != "Song One" || s.ArtistName() != "Band" || s.ArtistID() != "UCband" ||
		s.Album != "Record" || s.AlbumID != "MPREb_rec" || s.Duration != 201 || !s.Liked {
		t.Fatalf("song = %+v", s)
	}
}

func TestParseSongRowFixedDurationAndPlainArtist(t *testing.T) {
	row := decode(t, `{
	 "flexColumns": [
	  {"musicResponsiveListItemFlexColumnRenderer": {"text": {"runs": [{"text": "Track", "navigationEndpoint": {"watchEndpoint": {"videoId": "zyxwvutsrqp"}}}]}}},
	  {"musicResponsiveListItemFlexColumnRenderer": {"text": {"runs": [{"text": "Canción • Some Artist • 12 M reproducciones"}]}}}
	 ],
	 "fixedColumns": [{"musicResponsiveListItemFixedColumnRenderer": {"text": {"runs": [{"text": "1:02:03"}]}}}]
	}`)
	s, ok := parseSongRow(row)
	if !ok || s.VideoID != "zyxwvutsrqp" || s.ArtistName() != "Some Artist" || s.Duration != 3723 {
		t.Fatalf("song = %+v ok=%v", s, ok)
	}
}

func TestParseSongRowSkipsGreyedOutAndNonSongs(t *testing.T) {
	grey := decode(t, songRowJSON)
	grey["musicItemRendererDisplayPolicy"] = "MUSIC_ITEM_RENDERER_DISPLAY_POLICY_GREY_OUT"
	if _, ok := parseSongRow(grey); ok {
		t.Error("greyed-out row parsed as a song")
	}
	shuffle := decode(t, `{"flexColumns":[{"musicResponsiveListItemFlexColumnRenderer":{"text":{"runs":[{"text":"Shuffle all"}]}}}]}`)
	if _, ok := parseSongRow(shuffle); ok {
		t.Error("row without a video parsed as a song")
	}
}

func TestParsePanelSong(t *testing.T) {
	r := decode(t, `{"videoId":"abcdefghijk","title":{"runs":[{"text":"Radio Song"}]},
	 "lengthText":{"runs":[{"text":"4:05"}]},
	 "longBylineText":{"runs":[`+browseRun("Singer", "UCsinger", "ARTIST")+`,`+sep+`,`+browseRun("LP", "MPREb_lp", "ALBUM")+`,`+sep+`,{"text":"1999"}]}}`)
	s, ok := parsePanelSong(r)
	if !ok || s.Title != "Radio Song" || s.ArtistID() != "UCsinger" || s.AlbumID != "MPREb_lp" || s.Duration != 245 {
		t.Fatalf("song = %+v ok=%v", s, ok)
	}
}

func TestParseTwoRow(t *testing.T) {
	tests := []struct {
		name string
		json string
		kind ItemKind
		id   string
		year int
	}{
		{"album", `{"title":{"runs":[{"text":"Album X"}]},"subtitle":{"runs":[{"text":"Álbum"},` + sep + `,` + browseRun("Band", "UCband", "ARTIST") + `,` + sep + `,{"text":"2019"}]},
		  "navigationEndpoint":{"browseEndpoint":{"browseId":"MPREb_x","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_ALBUM"}}}}}`, KindAlbum, "MPREb_x", 2019},
		{"playlist", `{"title":{"runs":[{"text":"Mix"}]},"navigationEndpoint":{"browseEndpoint":{"browseId":"VLRDmix","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_PLAYLIST"}}}}}`, KindPlaylist, "VLRDmix", 0},
		{"library artist", `{"title":{"runs":[{"text":"Band"}]},"navigationEndpoint":{"browseEndpoint":{"browseId":"MPLAUCband","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_LIBRARY_ARTIST"}}}}}`, KindArtist, "UCband", 0},
		{"song", `{"title":{"runs":[{"text":"Tune"}]},"navigationEndpoint":{"watchEndpoint":{"videoId":"abcdefghijk","watchEndpointMusicSupportedConfigs":{"watchEndpointMusicConfig":{"musicVideoType":"MUSIC_VIDEO_TYPE_ATV"}}}}}`, KindSong, "abcdefghijk", 0},
		{"video", `{"title":{"runs":[{"text":"Clip"}]},"navigationEndpoint":{"watchEndpoint":{"videoId":"abcdefghijk"}}}`, KindVideo, "abcdefghijk", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it, ok := parseTwoRow(decode(t, tt.json))
			if !ok || it.Kind != tt.kind || it.ID != tt.id || it.Year != tt.year {
				t.Fatalf("item = %+v ok=%v, want kind %d id %s year %d", it, ok, tt.kind, tt.id, tt.year)
			}
			if tt.kind == KindAlbum && (it.Artist != "Band" || it.ArtistID != "UCband") {
				t.Fatalf("album credit = %q %q", it.Artist, it.ArtistID)
			}
			if (tt.kind == KindSong || tt.kind == KindVideo) && (it.Song == nil || it.Song.VideoID != tt.id) {
				t.Fatalf("song = %+v", it.Song)
			}
		})
	}
}

func TestParseShelfWithMoreButton(t *testing.T) {
	section := decode(t, `{"musicCarouselShelfRenderer":{
	 "header":{"musicCarouselShelfBasicHeaderRenderer":{"title":{"runs":[{"text":"Albums"}]},
	  "moreContentButton":{"buttonRenderer":{"navigationEndpoint":{"browseEndpoint":{"browseId":"MPADUCband","params":"p1"}}}}}},
	 "contents":[{"musicTwoRowItemRenderer":{"title":{"runs":[{"text":"A"}]},"navigationEndpoint":{"browseEndpoint":{"browseId":"MPREb_a","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_ALBUM"}}}}}}]}}`)
	sh, ok := parseShelf(section)
	if !ok || sh.Title != "Albums" || sh.MoreID != "MPADUCband" || sh.MoreParams != "p1" || len(sh.Items) != 1 {
		t.Fatalf("shelf = %+v ok=%v", sh, ok)
	}
}

func TestContinuationLayouts(t *testing.T) {
	old := decode(t, `{"continuationContents":{"musicShelfContinuation":{
	 "contents":[{"musicResponsiveListItemRenderer":`+songRowJSON+`}],
	 "continuations":[{"nextContinuationData":{"continuation":"TOK2"}}]}}}`)
	contents, token, newStyle := continuationPage(old)
	if len(contents) != 1 || token != "TOK2" || newStyle {
		t.Fatalf("old layout = %d %q %v", len(contents), token, newStyle)
	}

	fresh := decode(t, `{"onResponseReceivedActions":[{"appendContinuationItemsAction":{"continuationItems":[
	 {"musicResponsiveListItemRenderer":`+songRowJSON+`},
	 {"continuationItemRenderer":{"continuationEndpoint":{"continuationCommand":{"token":"TOK3"}}}}]}}]}`)
	contents, token, newStyle = continuationPage(fresh)
	if len(contents) != 1 || token != "TOK3" || !newStyle {
		t.Fatalf("new layout = %d %q %v", len(contents), token, newStyle)
	}

	shelf := decode(t, `{"contents":[{"musicResponsiveListItemRenderer":`+songRowJSON+`},{"continuationItemRenderer":{"continuationEndpoint":{"continuationCommand":{"token":"T"}}}}]}`)
	if token, newStyle := nextToken(shelf); token != "T" || !newStyle {
		t.Fatalf("nextToken = %q %v", token, newStyle)
	}
	if got := trimContinuation(arr(shelf, "contents")); len(got) != 1 {
		t.Fatalf("trimContinuation left %d entries", len(got))
	}
}

func TestParseDurationAndPlainArtist(t *testing.T) {
	for in, want := range map[string]int{"3:21": 201, "0:05": 5, "1:00:00": 3600, "abc": 0, "": 0} {
		if got := parseDuration(in); got != want {
			t.Errorf("parseDuration(%q) = %d, want %d", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"Canción • Band • 3:21":     "Band",
		"Song • Band • 2,1 M plays": "Band",
		"12 M visualizaciones":      "",
		"Álbum • Two Words • 2001":  "Two Words",
		"Single • A, B & C":         "A, B & C",
	} {
		if got := plainArtist(in); got != want {
			t.Errorf("plainArtist(%q) = %q, want %q", in, got, want)
		}
	}
}

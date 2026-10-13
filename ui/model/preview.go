package model

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/xaviiromf/cliamp-x/internal/appdir"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/ui/artwork"
)

// The preview pane sits under the list of a browsing screen (and under the
// playlist when the terminal is tall). It shows the cover or thumbnail of the
// highlighted album, artist, playlist or song, falling back to the song that
// plays, with a few lines about it. Covers load in the background and the
// rows around the cursor are fetched ahead, so moving through a list shows
// art without waiting.
const (
	previewMinRows = 6
	previewMaxRows = 12
	// previewAhead and previewBehind are the rows around the cursor whose art
	// loads ahead of time.
	previewAhead  = 10
	previewBehind = 3
	// previewDetailsAhead is how many rows past the cursor get their details
	// looked up ahead: each costs an API call.
	previewDetailsAhead = 2
	previewLoadTimeout  = 20 * time.Second
)

// previewItem is what the pane shows for one row.
type previewItem struct {
	detailID string // ID for a PreviewLoader lookup, or ""
	loader   provider.PreviewLoader
	imageURL string
	title    string
	lines    []string
	playing  bool // the item is the now-playing fallback
}

// artworkLoadedMsg tells the Model that an image finished loading.
type artworkLoadedMsg struct{}

// kittyImageMsg carries the escape sequences that put an image in a
// kitty-protocol terminal ("" when it could not be prepared), and
// kittyShownMsg follows once they are written.
type kittyImageMsg struct{ url, seq string }
type kittyShownMsg struct{ url string }

// handleKittyImage writes the image to the terminal, then marks it shown so
// the next frame prints its placeholders. Messages are handled in order, so
// the shown mark lands after the terminal has the image.
func (m *Model) handleKittyImage(msg kittyImageMsg) tea.Cmd {
	if msg.seq == "" {
		return nil
	}
	url := msg.url
	return tea.Sequence(tea.Raw(msg.seq), func() tea.Msg { return kittyShownMsg{url: url} })
}

// previewDetailsMsg carries the details a PreviewLoader found.
type previewDetailsMsg struct {
	id      string
	details provider.PreviewDetails
}

// SetPreviewImages picks how the pane draws pictures: "kitty" for the kitty
// graphics protocol, "blocks" for half-block characters, or "auto" (or "")
// for kitty where the terminal supports it.
func (m *Model) SetPreviewImages(mode string) {
	if m.art == nil {
		return
	}
	if mode == "kitty" || ((mode == "" || mode == "auto") && artwork.KittySupported()) {
		m.art.EnableKitty()
	}
}

// SetPreview turns the preview pane on or off. The cover cache serves the
// cover features either way, so it is created the first time this runs; a
// Model in a test that never calls it loads no images.
func (m *Model) SetPreview(on bool) {
	if m.art == nil {
		m.art = newArtworkCache()
	}
	m.previewOff = !on
}

func newArtworkCache() *artwork.Cache {
	dir := ""
	if data, err := appdir.DataDir(); err == nil {
		dir = filepath.Join(data, "artwork")
	}
	return artwork.New(dir)
}

// previewEnabled reports whether the pane may take rows at all.
func (m Model) previewEnabled() bool {
	return !m.headless && !m.previewOff && m.art != nil
}

// trackPreview describes a track.
func trackPreview(t playlist.Track) previewItem {
	it := previewItem{title: t.Title, imageURL: trackArtURL(t), detailID: t.AlbumID()}
	dur, year := "", ""
	if t.DurationSecs > 0 {
		dur = formatTrackTime(t.DurationSecs)
	}
	if t.Year > 0 {
		year = fmt.Sprint(t.Year)
	}
	plays := t.Meta("ytmusic.plays")
	it.lines = compactLines(t.Artist, t.Album, joinDot(year, dur), plays, t.Genre)
	return it
}

// trackArtURL returns the picture of a track: its art URL, a YouTube
// thumbnail, or the embedded picture of a local file.
func trackArtURL(t playlist.Track) string {
	if t.AlbumArtURL != "" {
		return t.AlbumArtURL
	}
	if id := youTubeVideoID(t.Path); id != "" {
		return "https://i.ytimg.com/vi/" + id + "/mqdefault.jpg"
	}
	if t.Path != "" && !t.Stream && !playlist.IsURL(t.Path) && !strings.HasPrefix(t.Path, "ssh://") {
		return t.Path
	}
	return ""
}

// youTubeVideoID returns the v= ID of a YouTube watch URL.
func youTubeVideoID(path string) string {
	i := strings.Index(path, "watch?v=")
	if i < 0 || !strings.Contains(path, "youtube.com/") {
		return ""
	}
	id := path[i+len("watch?v="):]
	if j := strings.IndexAny(id, "&#"); j >= 0 {
		id = id[:j]
	}
	if len(id) != 11 {
		return ""
	}
	return id
}

func joinDot(parts ...string) string {
	return strings.Join(compactLines(parts...), " · ")
}

func compactLines(lines ...string) []string {
	var out []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// previewList is the list the user moves through: count rows, the cursor,
// and the preview of row i (false when the row has none).
type previewList struct {
	count  int
	cursor int
	item   func(i int) (previewItem, bool)
}

// currentPreviewList returns the list on screen, or count 0 when the screen
// shows no list with previews.
func (m Model) currentPreviewList() previewList {
	switch m.activeScreen() {
	case screenNavBrowser:
		loader, _ := m.navBrowser.prov.(provider.PreviewLoader)
		switch m.navView() {
		case navViewAlbums:
			albums := m.navBrowser.albums
			return previewList{count: len(albums), cursor: m.selectedNavRawIndex(len(albums)), item: func(i int) (previewItem, bool) {
				a := albums[i]
				year := ""
				if a.Year > 0 {
					year = fmt.Sprint(a.Year)
				}
				return previewItem{detailID: a.ID, loader: loader, imageURL: a.ImageURL, title: a.Name, lines: compactLines(a.Artist, year)}, true
			}}
		case navViewArtists:
			artists := m.navBrowser.artists
			return previewList{count: len(artists), cursor: m.selectedNavRawIndex(len(artists)), item: func(i int) (previewItem, bool) {
				a := artists[i]
				return previewItem{detailID: a.ID, loader: loader, imageURL: a.ImageURL, title: a.Name}, true
			}}
		case navViewTracks:
			tracks := m.navBrowser.tracks
			return previewList{count: len(tracks), cursor: m.selectedNavRawIndex(len(tracks)), item: func(i int) (previewItem, bool) {
				it := m.trackItem(tracks[i])
				it.loader = loader
				return it, true
			}}
		}
	case screenSearchOverlay:
		if m.searchOverlay.screen != searchOverlayResults {
			return previewList{}
		}
		loader, _ := m.searchOverlay.prov.(provider.PreviewLoader)
		results := m.searchOverlay.results
		return previewList{count: len(results), cursor: m.searchOverlay.cursor, item: func(i int) (previewItem, bool) {
			it := m.trackItem(results[i])
			it.loader = loader
			return it, true
		}}
	case screenMain:
		switch m.focus {
		case focusProvider:
			loader, _ := m.provider.(provider.PreviewLoader)
			lists, prov := m.provPane.lists, m.provider
			return previewList{count: len(lists), cursor: m.provPane.cursor, item: func(i int) (previewItem, bool) {
				pl := lists[i]
				if _, entry := providerBrowseEntryForID(prov, pl.ID); entry {
					return previewItem{}, false
				}
				return previewItem{detailID: pl.ID, loader: loader, imageURL: pl.ImageURL, title: pl.Name}, true
			}}
		case focusPlaylist:
			pl := m.playlist
			if pl == nil {
				return previewList{}
			}
			return previewList{count: pl.Len(), cursor: m.plCursor, item: func(i int) (previewItem, bool) {
				t, ok := pl.Track(i)
				if !ok {
					return previewItem{}, false
				}
				return m.trackItem(t), true
			}}
		}
	}
	return previewList{}
}

// previewTarget returns what the pane shows: the highlighted row, or the
// song that plays. The browse screens show only their own rows, so the
// playing album does not mix into them.
func (m Model) previewTarget() (previewItem, bool) {
	list := m.currentPreviewList()
	if list.cursor >= 0 && list.cursor < list.count {
		if it, ok := list.item(list.cursor); ok {
			return m.withDetails(it), true
		}
	}
	if m.focus == focusProvider || m.activeScreen() == screenNavBrowser {
		return previewItem{}, false
	}
	t, idx := m.currentPlaybackTrack()
	if idx < 0 || t.Path == "" {
		return previewItem{}, false
	}
	it := m.withDetails(m.trackItem(t))
	it.playing = true
	return it, true
}

// trackItem describes a track for the pane, noting a downloaded copy.
func (m Model) trackItem(t playlist.Track) previewItem {
	it := trackPreview(t)
	if m.downloads.manifest.Recorded(t.Path) { // no file system call while drawing
		it.lines = append(it.lines, downloadLine())
	}
	return it
}

// withDetails adds the looked-up details of an item.
func (m Model) withDetails(it previewItem) previewItem {
	if it.detailID == "" {
		return it
	}
	d, ok := m.previewDetails[it.detailID]
	if !ok {
		return it
	}
	if it.imageURL == "" {
		it.imageURL = d.ImageURL
	}
	seen := make(map[string]bool, len(it.lines))
	lines := append([]string(nil), it.lines...)
	for _, l := range lines {
		seen[l] = true
	}
	for _, l := range d.Lines {
		if l = strings.TrimSpace(l); l != "" && !seen[l] {
			lines = append(lines, l)
			seen[l] = true
		}
	}
	it.lines = lines
	return it
}

// previewCmds starts the image and detail loads the pane needs now and soon:
// the highlighted row, the rows around it, and the song that plays.
func (m *Model) previewCmds() tea.Cmd {
	if m.layout.previewRows <= 0 || m.art == nil {
		return nil
	}
	if m.previewDetails == nil {
		m.previewDetails = map[string]provider.PreviewDetails{}
		m.previewPending = map[string]bool{}
	}
	var cmds []tea.Cmd
	want := func(it previewItem, details bool) {
		it = m.withDetails(it)
		if it.imageURL != "" && m.art.Claim(it.imageURL) {
			cmds = append(cmds, loadArtworkCmd(m.art, it.imageURL))
		}
		if !details || it.detailID == "" || it.loader == nil {
			return
		}
		if _, done := m.previewDetails[it.detailID]; done || m.previewPending[it.detailID] {
			return
		}
		m.previewPending[it.detailID] = true
		cmds = append(cmds, loadPreviewDetailsCmd(it.loader, it.detailID))
	}
	list := m.currentPreviewList()
	if list.count > 0 {
		cursor := max(0, min(list.cursor, list.count-1))
		for i := max(0, cursor-previewBehind); i < min(list.count, cursor+previewAhead+1); i++ {
			if it, ok := list.item(i); ok {
				want(it, i >= cursor && i <= cursor+previewDetailsAhead)
			}
		}
	}
	if t, idx := m.currentPlaybackTrack(); idx >= 0 {
		want(trackPreview(t), false)
	}
	// In a kitty-protocol terminal the shown image is sent to the terminal
	// once it is loaded; half blocks stand in until then.
	if it, ok := m.previewTarget(); ok {
		maxCols, body := m.previewBox()
		if send := m.art.KittyUpdate(it.imageURL, maxCols, body); send != nil {
			url := it.imageURL
			cmds = append(cmds, func() tea.Msg {
				return kittyImageMsg{url: url, seq: send()}
			})
		}
	}
	return tea.Batch(cmds...)
}

func loadArtworkCmd(cache *artwork.Cache, url string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), previewLoadTimeout)
		defer cancel()
		_ = cache.Load(ctx, url)
		return artworkLoadedMsg{}
	}
}

func loadPreviewDetailsCmd(loader provider.PreviewLoader, id string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), previewLoadTimeout)
		defer cancel()
		// A failed lookup stores empty details, so the row is not asked
		// again this run.
		d, _ := loader.PreviewDetails(ctx, id)
		return previewDetailsMsg{id: id, details: d}
	}
}

// handlePreviewDetails stores looked-up details.
func (m *Model) handlePreviewDetails(msg previewDetailsMsg) {
	if m.previewDetails == nil {
		return
	}
	delete(m.previewPending, msg.id)
	m.previewDetails[msg.id] = msg.details
}

// previewBox returns the most cells the picture may take: as tall as the
// pane below its header, and as wide as a 16:9 thumbnail of that height,
// but never more than half the panel.
func (m Model) previewBox() (maxCols, rows int) {
	rows = max(0, m.layout.previewRows-1)
	return max(2, min(m.layout.panelWidth/2, (2*rows*16+8)/9)), rows
}

// renderPreview draws the pane: a header rule, then the picture with the
// title and the detail lines beside it.
func (m Model) renderPreview() string {
	rows := m.layout.previewRows
	if rows <= 1 {
		return ""
	}
	width := m.layout.panelWidth
	body := rows - 1
	it, ok := m.previewTarget()
	label := L("Selected")
	if !ok {
		label = L("Preview")
	} else if it.playing {
		label = L("Now playing")
	}
	lines := make([]string, 0, rows)
	lines = append(lines, sepHeader(label, width))
	if !ok {
		for range body {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}

	// The picture is as tall as the pane and may be as wide as a 16:9
	// thumbnail, but never more than half the panel.
	maxCols, _ := m.previewBox()
	picW := min(maxCols, 2*body) // a square placeholder until the image loads
	var picture []string
	if drawn, cols, imgRows, shown := m.art.KittyDraw(it.imageURL, maxCols, body); shown {
		picW = cols
		picture = append(make([]string, (body-imgRows)/2), drawn...)
	} else if drawn, cols, imgRows, loaded := m.art.Draw(it.imageURL, maxCols, body); loaded {
		picW = cols
		picture = append(make([]string, (body-imgRows)/2), drawn...)
	} else {
		for r := range body {
			fill := strings.Repeat("░", picW)
			if r == body/2 && picW >= 3 {
				half := (picW - 1) / 2
				fill = strings.Repeat("░", half) + "♪" + strings.Repeat("░", picW-1-half)
			}
			picture = append(picture, dimStyle.Render(fill))
		}
	}

	textW := max(0, width-picW-2)
	title, details := tidyPreviewText(it.title, it.lines)
	text := []string{titleStyle.Render(truncate(title, textW))}
	for _, l := range details {
		text = append(text, dimStyle.Render(truncate(l, textW)))
	}
	for r := range body {
		pic := ""
		if r < len(picture) {
			pic = picture[r]
		}
		if gap := picW - lipgloss.Width(pic); gap > 0 {
			pic += strings.Repeat(" ", gap)
		}
		row := pic
		if r < len(text) {
			row += "  " + text[r]
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// tidyPreviewText drops what the pane would say twice: a detail line that
// repeats the title (a single named after its song) or that is one part of
// another line ("2023" beside "Single · 2023"), and an artist that a row
// name carries after a dot ("Album · Artist" over an "Artist" line).
func tidyPreviewText(title string, lines []string) (string, []string) {
	inOther := func(i int) bool {
		for j, other := range lines {
			if j == i || other == lines[i] {
				continue
			}
			for _, part := range strings.Split(other, " · ") {
				if part == lines[i] {
					return true
				}
			}
		}
		return false
	}
	kept := make([]string, 0, len(lines))
	for i, l := range lines {
		if l == title || inOther(i) {
			continue
		}
		kept = append(kept, l)
	}
	for _, l := range kept {
		if trimmed, ok := strings.CutSuffix(title, " · "+l); ok && trimmed != "" {
			title = trimmed
			break
		}
	}
	return title, kept
}

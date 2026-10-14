package model

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"golang.org/x/text/unicode/norm"

	"github.com/xaviiromf/cliamp-x/ipc"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

// Discovery: "more like this" (M) starts a radio from the highlighted song,
// album, artist or playlist, and the remix finder (X) looks for remixes and
// other versions of a song.

const (
	moreLikeThisCount = 50
	discoverTimeout   = 30 * time.Second
	remixPerVariant   = 12
)

// discoverSeed is what M or X acts on: a song, or a browse row by ID.
type discoverSeed struct {
	track  playlist.Track
	itemID string
	prov   playlist.Provider
	label  string
}

// discoverTarget returns the highlighted row of the screen, or the song that
// plays.
func (m *Model) discoverTarget() (discoverSeed, bool) {
	switch m.activeScreen() {
	case screenNavBrowser:
		prov := m.navBrowser.prov
		switch m.navView() {
		case navViewAlbums:
			if i := m.selectedNavRawIndex(len(m.navBrowser.albums)); i >= 0 {
				a := m.navBrowser.albums[i]
				return discoverSeed{itemID: a.ID, prov: prov, label: a.Name}, true
			}
		case navViewArtists:
			if i := m.selectedNavRawIndex(len(m.navBrowser.artists)); i >= 0 {
				a := m.navBrowser.artists[i]
				return discoverSeed{itemID: a.ID, prov: prov, label: a.Name}, true
			}
		case navViewTracks:
			if i := m.selectedNavRawIndex(len(m.navBrowser.tracks)); i >= 0 {
				t := m.navBrowser.tracks[i]
				return discoverSeed{track: t, prov: prov, label: t.Title}, true
			}
		}
	case screenSearchOverlay:
		if m.searchOverlay.screen == searchOverlayResults {
			if c := m.searchOverlay.cursor; c >= 0 && c < len(m.searchOverlay.results) {
				t := m.searchOverlay.results[c]
				if t.IsAlbum() {
					return discoverSeed{itemID: t.AlbumID(), prov: m.searchOverlay.prov, label: t.Title}, true
				}
				return discoverSeed{track: t, prov: m.searchOverlay.prov, label: t.Title}, true
			}
		}
	case screenMain:
		switch m.focus {
		case focusProvider:
			if c := m.provPane.cursor; c >= 0 && c < len(m.provPane.lists) {
				row := m.provPane.lists[c]
				if _, entry := providerBrowseEntryForID(m.provider, row.ID); !entry {
					return discoverSeed{itemID: row.ID, prov: m.provider, label: row.Name}, true
				}
			}
		case focusPlaylist:
			if t, ok := m.playlist.Track(m.plCursor); ok {
				return discoverSeed{track: t, label: t.Title}, true
			}
		}
	}
	if t, idx := m.currentPlaybackTrack(); idx >= 0 && t.Path != "" {
		return discoverSeed{track: t, label: t.Title}, true
	}
	return discoverSeed{}, false
}

// moreLikeThisMsg carries a radio: the seed song first, then related ones.
// reply, when set, gets the result of the IPC radio.start that asked for it.
type moreLikeThisMsg struct {
	label  string
	tracks []playlist.Track
	err    error
	gen    uint64
	reply  chan<- ipc.Response
}

// moreLikeThis starts a radio from the highlighted row. Like "Start radio"
// in YouTube Music, it replaces the queue.
func (m *Model) moreLikeThis() tea.Cmd {
	seed, ok := m.discoverTarget()
	if !ok {
		m.status.Show(L("Highlight a song, album or artist"), statusTTLMedium)
		return nil
	}
	return m.moreLikeThisFrom(seed, nil)
}

// moreLikeThisFrom looks up the radio of seed. reply, when set, gets the
// result of radio.start, also when the radio cannot start.
func (m *Model) moreLikeThisFrom(seed discoverSeed, reply chan<- ipc.Response) tea.Cmd {
	gen := nextRequest(&m.requests.discover)
	m.status.Activityf(statusTTLLong, "%s %s…", L("Finding songs like"), seed.label)
	if seed.itemID != "" {
		relater, ok := seed.prov.(provider.ItemRelater)
		if !ok {
			relater, _ = findCapable(m, func(provider.ItemRelater) bool { return true })
		}
		if relater == nil {
			return m.radioFailed(reply, L("This row has no related songs"))
		}
		id, label := seed.itemID, seed.label
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), discoverTimeout)
			defer cancel()
			first, related, err := relater.RelatedToItem(ctx, id, moreLikeThisCount)
			if err != nil {
				return moreLikeThisMsg{label: label, err: err, gen: gen, reply: reply}
			}
			return moreLikeThisMsg{label: label, tracks: append([]playlist.Track{first}, related...), gen: gen, reply: reply}
		}
	}
	relater, _ := findCapable(m, func(r provider.Relater) bool { return r.CanRelate(seed.track) })
	if relater == nil {
		return m.radioFailed(reply, L("Works only with YouTube Music songs"))
	}
	track, label := seed.track, seed.label
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), discoverTimeout)
		defer cancel()
		related, err := relater.RelatedTracks(ctx, track, moreLikeThisCount)
		if err != nil {
			return moreLikeThisMsg{label: label, err: err, gen: gen, reply: reply}
		}
		return moreLikeThisMsg{label: label, tracks: append([]playlist.Track{track}, related...), gen: gen, reply: reply}
	}
}

// radioFailed shows why a radio did not start, and fails the radio.start
// that waits on reply.
func (m *Model) radioFailed(reply chan<- ipc.Response, text string) tea.Cmd {
	m.status.Show(text, statusTTLMedium)
	if reply != nil {
		reply <- ipc.Response{Error: text}
	}
	return nil
}

// handleMoreLikeThis replaces the queue with the radio and plays it.
func (m *Model) handleMoreLikeThis(msg moreLikeThisMsg) tea.Cmd {
	if msg.gen != m.requests.discover {
		if msg.reply != nil {
			msg.reply <- ipc.Response{Error: "superseded"}
		}
		return nil
	}
	if msg.err != nil {
		m.status.Warningf(statusTTLMedium, "%s: %s", L("More like this"), msg.err)
		if msg.reply != nil {
			msg.reply <- ipc.Response{Error: msg.err.Error()}
		}
		return nil
	}
	if len(msg.tracks) < 2 {
		return m.radioFailed(msg.reply, L("No related songs found"))
	}
	m.stopPlayback()
	m.player.ClearPreload()
	m.retireTracksPaging()
	m.replacePlaylist(msg.tracks)
	m.clearLoadedPlaylist()
	m.setHeaderStateFromTracks(msg.tracks)
	m.plCursor, m.plScroll = 0, 0
	m.playlist.SetIndex(0)
	m.focus = focusPlaylist
	m.navBrowser.visible = false
	if m.searchOverlay.visible {
		m.closeSearchOverlay()
	}
	m.status.Successf(statusTTLDefault, "%s %s: %d %s", L("Radio of"), msg.label, len(msg.tracks), L("songs"))
	if msg.reply != nil {
		msg.reply <- m.v2PlaylistResponse()
	}
	return m.playCurrentTrack()
}

// remixVariant is one kind of version the remix finder looks for. words are
// the title words that place a result in this group.
type remixVariant struct {
	section string
	query   string
	words   []string
}

func remixVariants() []remixVariant {
	return []remixVariant{
		{L("Remixes"), "remix", []string{"remix", "rmx", "edit", "flip", "rework", "vip"}},
		{"Slowed + reverb", "slowed reverb", []string{"slowed", "reverb"}},
		{"Sped up", "sped up", []string{"sped up", "speed up", "nightcore"}},
		{"Nightcore", "nightcore", []string{"nightcore"}},
		{L("Bootlegs and mashups"), "bootleg mashup", []string{"bootleg", "mashup", "mash up", "phonk", "funk", "montagem"}},
		{L("Versions and covers"), "cover version", []string{"cover", "version", "versión", "acoustic", "acústic", "live", "en vivo", "instrumental"}},
	}
}

// findRemixes opens the search overlay with the versions of the highlighted
// song, grouped by kind.
func (m *Model) findRemixes() tea.Cmd {
	seed, ok := m.discoverTarget()
	if !ok || seed.itemID != "" || seed.track.Title == "" {
		m.status.Show(L("Highlight a song to find its remixes"), statusTTLMedium)
		return nil
	}
	prov := seed.prov
	if _, ok := prov.(provider.Searcher); !ok {
		prov = nil
		if s, _ := findCapable(m, func(provider.Searcher) bool { return true }); s != nil {
			prov, _ = s.(playlist.Provider)
		}
	}
	searcher, ok := prov.(provider.Searcher)
	if !ok {
		m.status.Show(L("No provider can search"), statusTTLMedium)
		return nil
	}
	base := cleanSongTitle(seed.track.Title)
	if base == "" {
		base = seed.track.Title
	}
	m.openProviderSearchWith(prov)
	m.searchOverlay.query = base + " remix"
	m.searchOverlay.loading = true
	gen := nextRequest(&m.requests.searchOverlay)
	query, orig := m.searchOverlay.query, seed.track
	ctx := m.newSearchOverlayRequestContext(discoverTimeout)
	name := prov.Name()
	return func() tea.Msg {
		tracks, err := searchRemixes(ctx, searcher, orig, base)
		return searchOverlayResultsMsg{tracks: tracks, err: err, providerName: name, query: query, gen: gen}
	}
}

// searchRemixes runs one search per variant, keeps the results that are
// versions of orig and groups them into sections.
func searchRemixes(ctx context.Context, s provider.Searcher, orig playlist.Track, base string) ([]playlist.Track, error) {
	variants := remixVariants()
	baseWords := significantWords(base)
	// A one-word title such as "Arsenal" matches unrelated songs, so the
	// search and the filter also need the original artist.
	artist := mainArtist(orig.Artist)
	query := base
	var artistWords []string
	if len(baseWords) <= 1 && artist != "" {
		query = artist + " " + base
		artistWords = significantWords(artist)
	}
	results := make([][]playlist.Track, len(variants))
	errs := make([]error, len(variants))
	var wg sync.WaitGroup
	for i, v := range variants {
		wg.Add(1)
		go func(i int, v remixVariant) {
			defer wg.Done()
			results[i], errs[i] = s.SearchTracks(ctx, query+" "+v.query, 25)
		}(i, v)
	}
	wg.Wait()

	seen := map[string]bool{orig.Path: true}
	groups := make([][]playlist.Track, len(variants))
	var anyErr error
	for i := range variants {
		if errs[i] != nil {
			anyErr = errs[i]
			continue
		}
		for _, t := range results[i] {
			if t.IsAlbum() || t.Path == "" || seen[t.Path] || !isVersionOf(t, orig, baseWords) {
				continue
			}
			if len(artistWords) > 0 && !containsAll(foldText(t.Title+" "+t.Artist), artistWords) {
				continue
			}
			// The same upload often exists more than once.
			name := foldText(t.Artist + "\x00" + t.Title)
			if seen[name] {
				continue
			}
			seen[name] = true
			group := classifyVersion(t.Title, variants, i)
			if len(groups[group]) >= remixPerVariant {
				continue
			}
			seen[t.Path] = true
			groups[group] = append(groups[group], withSection(t, variants[group].section))
		}
	}
	var out []playlist.Track
	for _, g := range groups {
		out = append(out, g...)
	}
	if len(out) == 0 && anyErr != nil {
		return nil, anyErr
	}
	return out, nil
}

// classifyVersion puts a result in the first variant whose words its title
// names, or in the variant whose search found it.
func classifyVersion(title string, variants []remixVariant, foundBy int) int {
	t := foldText(title)
	for i, v := range variants {
		for _, w := range v.words {
			if strings.Contains(t, foldText(w)) {
				return i
			}
		}
	}
	return foundBy
}

// isVersionOf reports whether t looks like a version of orig: its title
// names most of the original title's words, and its length is plausible.
func isVersionOf(t, orig playlist.Track, baseWords []string) bool {
	if len(baseWords) == 0 {
		return false
	}
	title := foldText(t.Title + " " + t.Artist)
	hits := 0
	for _, w := range baseWords {
		if strings.Contains(title, w) {
			hits++
		}
	}
	if hits*10 < len(baseWords)*7 {
		return false
	}
	if orig.DurationSecs > 0 && t.DurationSecs > 0 {
		r := float64(t.DurationSecs) / float64(orig.DurationSecs)
		if r < 0.35 || r > 3.5 {
			return false
		}
	}
	return true
}

// mainArtist returns the first artist of a credit such as "A, B & C".
func mainArtist(credit string) string {
	for _, sep := range []string{",", " & ", " feat", " ft.", " x ", " X ", " y "} {
		if i := strings.Index(credit, sep); i > 0 {
			credit = credit[:i]
		}
	}
	return strings.TrimSpace(credit)
}

func containsAll(s string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(s, w) {
			return false
		}
	}
	return true
}

func withSection(t playlist.Track, section string) playlist.Track {
	meta := make(map[string]string, len(t.ProviderMeta)+1)
	for k, v := range t.ProviderMeta {
		meta[k] = v
	}
	meta[playlist.MetaSection] = section
	t.ProviderMeta = meta
	return t
}

// cleanSongTitle drops the parts of a title that differ between versions:
// anything in brackets, "feat." credits and the text after " - ".
func cleanSongTitle(title string) string {
	var b strings.Builder
	depth := 0
	for _, r := range title {
		switch r {
		case '(', '[', '{':
			depth++
			continue
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 {
			b.WriteRune(r)
		}
	}
	s := b.String()
	lower := strings.ToLower(s)
	for _, cut := range []string{" feat.", " feat ", " ft.", " ft ", " - ", " | "} {
		if i := strings.Index(lower, cut); i > 0 {
			s, lower = s[:i], lower[:i]
		}
	}
	return strings.Join(strings.Fields(s), " ")
}

// significantWords returns the folded words of s that carry meaning.
func significantWords(s string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(foldText(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if len([]rune(w)) >= 2 || unicode.IsDigit([]rune(w)[0]) || len(out) == 0 {
			out = append(out, w)
		}
	}
	return out
}

// foldText lowercases s and drops accents, so "Canción" matches "cancion".
func foldText(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

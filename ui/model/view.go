package model

import (
	"fmt"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/xaviiromf/cliamp-x/favorites"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

// favHeart is the small, text-presentation favorite heart used everywhere the
// UI shows favorite state (track rows, header badge, status messages).
const favHeart = "♥\uFE0E"

// playedMarker and partialMarker flag an episode listened to the end and one
// stopped part-way. Both carry the text variation selector so terminals render
// them one cell wide, as favHeart does.
const (
	playedMarker  = "\u2713\uFE0E"
	partialMarker = "\u25D1\uFE0E"
)

// Seek-bar glyphs. Played cells are heavy and the unplayed remainder light, so
// the two differ in weight and not only in color, and a round head marks the
// position outright instead of leaving it to the boundary between them.
const (
	seekFillGlyph  = "━"
	seekHeadGlyph  = "●"
	seekEmptyGlyph = "─"
)

// Toggle feedback marks for the status bar. Rendered per call, not stored, so
// they pick up favMarkerStyle/favRemovedStyle as rebuildModelStyles leaves
// them after a theme change. Both call sites are key handlers, not the render
// loop.
func favAddedMark() string { return favMarkerStyle.Render(favHeart) }

func favRemovedMark() string { return favRemovedStyle.Render(favHeart) }

// providerEmptyStateHint, keyed by lowercase provider Name(), returns the
// remediation hint shown under the generic "No playlists in X" message.
var providerEmptyStateHint = map[string]string{
	"local playlists": "Add .toml playlists to ~/.config/cliamp-x/playlists/.",
	"local":           "Add .toml playlists to ~/.config/cliamp-x/playlists/.",
	"youtube music":   "Run `cliamp-x login` to sign in, then press Ctrl+R to reload.",
	"ytmusic":         "Run `cliamp-x login` to sign in, then press Ctrl+R to reload.",
}

// renderProviderEmptyState explains why the playlists pane is empty for the
// current provider and offers a remediation hint. Always pads to budget so the
// pane height stays stable.
func (m Model) renderProviderEmptyState(budget int) string {
	name := "this provider"
	if m.provider != nil {
		name = m.provider.Name()
	}
	if m.providerCatalogSearching() {
		found := "  No results."
		if m.provSearch.query != "" {
			found = fmt.Sprintf("  No results for %q.", m.provSearch.query)
		}
		lines := []string{dimStyle.Render(found), "", m.pressKeyHint(commandModeProvider, "esc", "to clear the search.")}
		return strings.Join(fitLines(lines, budget), "\n")
	}
	lines := []string{
		dimStyle.Render(fmt.Sprintf("  No playlists in %s.", name)),
		"",
	}
	if _, searchable := m.provider.(provider.CatalogSearcher); searchable {
		lines = append(lines,
			dimStyle.Render("  Press ")+helpKeyStyle.Render(" / ")+dimStyle.Render(" to search."))
	} else if _, searchable := m.provider.(provider.Searcher); searchable {
		lines = append(lines,
			dimStyle.Render("  Press ")+helpKeyStyle.Render(" Ctrl+F ")+dimStyle.Render(" to search."))
	}
	if m.provider != nil {
		if hint, ok := providerEmptyStateHint[strings.ToLower(m.provider.Name())]; ok {
			lines = append(lines, dimStyle.Render("  "+hint))
		} else {
			lines = append(lines, dimStyle.Render("  Press Ctrl+R to reload."))
		}
	}
	return strings.Join(fitLines(lines, budget), "\n")
}

// providerRowStyle picks the prefix and style for a provider-list row.
// Cursor takes precedence; "currently loaded" gets the active-track style and
// the ▶ prefix so users can see at a glance which playlist is in the queue.
func (m Model) providerRowStyle(p playlist.PlaylistInfo, isCursor bool) (string, lipgloss.Style) {
	if isCursor {
		return "> ", playlistSelectedStyle
	}
	if m.isProviderRowActive(p) {
		return "▶ ", playlistActiveStyle
	}
	return "  ", playlistItemStyle
}

// isProviderRowActive reports whether the given playlist is the one whose
// tracks are currently loaded into the player.
func (m Model) isProviderRowActive(p playlist.PlaylistInfo) bool {
	if m.activeProviderPlaylistID != "" && m.activeProviderPlaylistID == p.ID {
		return true
	}
	if m.loadedPlaylist != "" && m.loadedPlaylist == p.Name {
		return true
	}
	return false
}

// providerRowLabel formats a provider-list row.
func (m Model) providerRowLabel(prefix string, p playlist.PlaylistInfo) string {
	return playlistLabel(prefix, p)
}

// playlistLabel formats a playlist entry, omitting fields the provider didn't
// supply. Track count and total duration are appended when available. The
// Favorites virtual playlist always shows its count — it stays listed even
// when empty, and a bare name would look broken. Directory-backed playlists
// hide the duration: their track counts are cheap estimates, so a runtime
// total would be misleading.
func playlistLabel(prefix string, p playlist.PlaylistInfo) string {
	out := prefix + p.Name
	var parts []string
	if p.TrackCount > 0 || p.Name == favorites.PlaylistName {
		parts = append(parts, fmt.Sprintf("%d tracks", p.TrackCount))
	}
	if p.DirSourceCount == 0 {
		if d := formatPlaylistDuration(p.DurationSecs); d != "" {
			parts = append(parts, d)
		}
	}
	if len(parts) > 0 {
		out += " · " + strings.Join(parts, " · ")
	}
	return out
}

func (m Model) renderPlaylistHeader() string {
	if ov, ok := m.activeOverlay(); ok {
		return ov.header(&m)
	}
	if m.focus == focusProvider {
		label := m.provider.Name() + " / Playlists"
		_, catalog := m.provider.(provider.CatalogSearcher)
		switch {
		case m.provSearch.active && catalog:
			label += " / Search"
		case m.provSearch.active:
			label += " / Filter"
		case m.providerCatalogSearching():
			label += " / Search results"
		}
		return dimStyle.Render(labeledSeparator("", label, m.layout.panelWidth))
	}
	return m.renderPlaybackHeader()
}

// renderPlaybackHeader is also used to probe playback focus beneath overlays.
func (m Model) renderPlaybackHeader() string {
	// Badges in display order. A narrow pane drops whole badges off the tail
	// rather than letting the separator slice one mid-token.
	badges := make([]string, 0, 6)

	// The two-column body carries shuffle and repeat in its settings pane, so
	// the header only shows them when there is no pane to hold them.
	if !m.layout.twoColumn {
		if m.focus == focusShuffle {
			value := "Off"
			if m.playlist.Shuffled() {
				value = "On"
			}
			badges = append(badges, activeToggle.Render("[Shuffle ▸ "+value+"]"))
		} else if m.playlist.Shuffled() {
			badges = append(badges, activeToggle.Render("[Shuffle]"))
		} else {
			badges = append(badges, dimStyle.Render("[")+trackStyle.Render("Shuffle")+dimStyle.Render("]"))
		}

		repeatVal := m.playlist.Repeat().String()
		if m.focus == focusRepeat {
			badges = append(badges, activeToggle.Render(fmt.Sprintf("[Repeat ▸ %s]", repeatVal)))
		} else if m.playlist.Repeat() != 0 {
			badges = append(badges, activeToggle.Render(fmt.Sprintf("[Repeat: %s]", repeatVal)))
		} else {
			badges = append(badges, dimStyle.Render("[")+trackStyle.Render("Repeat")+dimStyle.Render(": ")+dimStyle.Render(repeatVal)+dimStyle.Render("]"))
		}
	}

	if qLen := m.playlist.QueueLen(); qLen > 0 {
		badges = append(badges, activeToggle.Render(fmt.Sprintf("[Queue: %d]", qLen)))
	}
	// Render from the cached favSet: the render path must not hit disk.
	if count := len(m.favSet); count > 0 {
		badges = append(badges, activeToggle.Render(fmt.Sprintf("[%s %d]", favHeart, count)))
	}
	if total := m.playlist.Len(); total > 0 {
		badges = append(badges, dimStyle.Render(fmt.Sprintf("[%d/%d]", m.playlist.Index()+1, total)))
	}

	headerStyle := dimStyle
	headerLabel := "── Playlist ── "
	if m.focus == focusPlaylist {
		headerStyle = activeToggle
		headerLabel = "▸─ Playlist ── "
	}

	const trailer = " ──"
	out := headerStyle.Render(headerLabel)
	width := lipgloss.Width(headerLabel) + lipgloss.Width(trailer)
	for i, badge := range badges {
		w := lipgloss.Width(badge)
		if i > 0 {
			w++ // separating space
		}
		if width+w > m.layout.panelWidth {
			break
		}
		if i > 0 {
			out += " "
		}
		out += badge
		width += w
	}
	return out + dimStyle.Render(trailer)
}

func (m Model) renderProviderList() string {
	visibleBudget := m.effectivePlaylistVisible()
	if visibleBudget <= 0 {
		return ""
	}
	if m.provPane.signIn {
		return dimStyle.Render(fmt.Sprintf("  Sign in to %s. Press Enter to continue.", m.provider.Name()))
	}
	if m.provPane.loading && (len(m.provPane.lists) == 0 || m.provSearch.loading) && !m.provSearch.active {
		label := fmt.Sprintf("Loading %s…", m.provider.Name())
		if m.provSearch.loading {
			label = fmt.Sprintf("Searching %s…", m.provider.Name())
		}
		lines := []string{loadingLine(label)}
		if m.provPane.authURL != "" {
			lines = append(lines,
				"",
				dimStyle.Render("  If your browser didn't open, visit this URL to sign in:"),
				"  "+m.provPane.authURL,
			)
		}
		for len(lines) < visibleBudget {
			lines = append(lines, "")
		}
		return strings.Join(lines, "\n")
	}
	if m.provPane.askLoc {
		return m.renderLocationPrompt(visibleBudget)
	}
	if len(m.provPane.lists) == 0 && !m.provSearch.active && !m.catalogBatch.loading {
		return m.renderProviderEmptyState(visibleBudget)
	}

	sl, sectioned := m.provider.(provider.SectionedList)
	var lines []string

	if m.provSearch.active {
		_, searchable := m.provider.(provider.CatalogSearcher)
		mode := "Filter: "
		if searchable {
			mode = "Search: "
		}
		lines = append(lines, m.filterHeader(mode+m.provider.Name(), "provider-search", m.provSearch.query, ""))

		if searchable {
			if m.provSearch.query == "" {
				lines = append(lines, dimStyle.Render("  Type a query, Enter to search..."))
			} else {
				lines = append(lines, dimStyle.Render("  Press Enter to search"))
			}
		} else {
			if m.provSearch.query == "" {
				lines = append(lines, dimStyle.Render("  Type to filter…"))
			} else if len(m.provSearch.results) == 0 {
				lines = append(lines, dimStyle.Render("  No matches"))
			} else {
				visible := max(0, min(visibleBudget-1, len(m.provSearch.results)))
				scroll := m.provSearch.scroll
				for j := scroll; j < scroll+visible && j < len(m.provSearch.results); j++ {
					idx := m.provSearch.results[j]
					p := m.provPane.lists[idx]
					prefix, style := m.providerRowStyle(p, j == m.provSearch.cursor)
					lines = append(lines, style.Render(m.providerRowLabel(prefix, p)))
				}
				lines = append(lines, dimStyle.Render(fmt.Sprintf("  %d/%d playlists", len(m.provSearch.results), len(m.provPane.lists))))
			}
		}
	} else {
		scroll := max(0, m.provPane.scroll)
		if scroll >= len(m.provPane.lists) {
			scroll = max(0, len(m.provPane.lists)-1)
		}
		if m.provPane.cursor < scroll {
			scroll = m.provPane.cursor
		}

		hasSections := !sectioned && slices.ContainsFunc(m.provPane.lists, func(p playlist.PlaylistInfo) bool {
			return p.Section != ""
		})

		if sectioned {
			for scroll < len(m.provPane.lists)-1 && m.providerRowsFromScroll(scroll, m.provPane.cursor) > visibleBudget {
				scroll++
			}
		} else if m.provPane.cursor >= scroll+visibleBudget {
			scroll = m.provPane.cursor - visibleBudget + 1
		}

		// Headers are deduplicated on the resolved title, not the ID prefix:
		// distinct prefixes can share one heading (the country browse shortcut
		// sits under the same "Countries" heading as the pinned places).
		prevTitle := ""
		if sectioned && scroll > 0 {
			prevTitle = m.providerSectionTitle(sl.IDPrefix(m.provPane.lists[scroll-1].ID))
		}
		prevSection := ""
		if hasSections && scroll > 0 {
			prevSection = m.provPane.lists[scroll-1].Section
		}

		for j := scroll; j < len(m.provPane.lists) && len(lines) < visibleBudget; j++ {
			p := m.provPane.lists[j]

			if sectioned {
				title := m.providerSectionTitle(sl.IDPrefix(p.ID))
				if title != prevTitle {
					if title != "" && len(lines) < visibleBudget {
						lines = append(lines, dimStyle.Render(labeledSeparator("  ", title, m.layout.panelWidth)))
					}
					prevTitle = title
				}
			} else if hasSections && p.Section != prevSection {
				header := labeledSeparator("  ", p.Section, m.layout.panelWidth)
				if len(lines) < visibleBudget {
					lines = append(lines, dimStyle.Render(header))
				}
				prevSection = p.Section
			}

			if len(lines) >= visibleBudget {
				break
			}

			prefix, style := m.providerRowStyle(p, j == m.provPane.cursor)
			lines = append(lines, style.Render(m.providerRowLabel(prefix, p)))
		}
	}

	// Loading indicator for catalog batch (never displace selected row if full).
	if m.catalogBatch.loading && len(lines) < visibleBudget {
		lines = append(lines, loadingLine("Loading more entries..."))
	}

	return strings.Join(fitLines(lines, visibleBudget), "\n")
}

// renderLocationPrompt puts the provider's location question to the listener.
// The wording comes from the provider, which is the only thing that knows what
// it would do with the answer and where the location would come from.
func (m Model) renderLocationPrompt(budget int) string {
	question := "Use your location?"
	if consenter, ok := m.provider.(provider.LocationConsenter); ok {
		if prompt := consenter.LocationPrompt(); prompt != "" {
			question = prompt
		}
	}

	lines := []string{""}
	for _, line := range wrapText(question, m.layout.panelWidth-6) {
		lines = append(lines, "  "+line)
	}
	lines = append(lines, "", playlistSelectedStyle.Render("  y  Yes"), dimStyle.Render("  n  No, don't"))
	return strings.Join(fitLines(lines, budget), "\n")
}

// providerSectionTitle names a SectionedList pane section. The provider owns
// the wording: only it knows what its ID prefixes mean, and only it knows the
// live state a heading may need to carry, such as the radio catalog's active
// country filter. A provider that supplies no title gets no heading.
func (m Model) providerSectionTitle(prefix string) string {
	titler, ok := m.provider.(provider.SectionTitler)
	if !ok {
		return ""
	}
	return titler.SectionTitle(prefix)
}

func (m Model) renderPlaylist() string {
	budget := m.effectivePlaylistVisible()
	if budget <= 0 {
		return ""
	}

	if m.focus == focusProvider {
		return m.renderProviderList()
	}

	trackCount := m.playlist.Len()
	if trackCount == 0 {
		var lines []string
		if m.feedLoading || (m.provPane.loading && m.activeProviderPlaylistID != "") {
			lines = append(lines, loadingLine("Loading feed…"))
		} else {
			lines = append(lines, paint(dimStyle, "  "+L("No tracks loaded.")), paint(dimStyle, "  "+L("To add music, press one of these keys:")), "")
			for _, key := range []string{"esc", "o", "u", "ctrl+f"} {
				if hint := m.commandHint(commandModeMain, key); hint != "" {
					lines = append(lines, hint)
				}
			}
		}
		return strings.Join(fitLines(lines, budget), "\n")
	}

	currentIdx := m.playlist.Index()
	scroll := m.playlistScroll(budget)
	windowStart := max(0, scroll-1)
	indices, tracks := m.playlist.OrderWindow(windowStart, budget+1)
	localScroll := scroll - windowStart

	lines := make([]string, 0, budget)
	numWidth := len(fmt.Sprintf("%d", trackCount))
	// A list longer than the window gives its last column to a scroll bar.
	width := m.layout.panelWidth
	if trackCount > budget {
		width -= scrollbarCols
	}
	cols := m.markerColumns()
	var stateReporters []provider.PlaybackStateReporter
	if cols.played {
		stateReporters = m.playbackStateReporters()
	}

	for row := range m.playlistRows(tracks, localScroll, m.showAlbumHeaders) {
		if row.Index < 0 {
			if len(lines)+1 >= budget {
				break
			}
			lines = append(lines, m.albumSeparator(row.Album, row.Year))
			continue
		}

		if len(lines) >= budget {
			break
		}

		i, t := indices[row.Index], row.Track
		style := playlistItemStyle
		selected := m.focus == focusPlaylist && i == m.plCursor
		playing := !m.playbackDetached && i == currentIdx && m.player.IsPlaying()
		if playing {
			style = playlistActiveStyle
		}
		if selected {
			style = playlistSelectedStyle
		}

		if t.Unplayable {
			if selected {
				style = dimStyle
			} else {
				style = playlistUnavailableStyle
			}
		}
		cursorMarker := " "
		if selected {
			cursorMarker = ">"
		} else if m.plMarks[t.Path] {
			cursorMarker = "✓"
		}
		// An unplayable track is never the playing one, so both share a cell.
		stateMarker, stateStyle := " ", playlistActiveStyle
		switch {
		case t.Unplayable:
			stateMarker, stateStyle = "!", playlistUnavailableStyle
		case playing:
			stateMarker = "▶"
		}
		markers := cursorMarker + stateMarker
		styledMarkers := paint(dimStyle, cursorMarker) + paint(stateStyle, stateMarker)

		queuePosition := m.playlist.QueuePosition(i)
		if cols.queue {
			mark := " "
			if queuePosition > 0 {
				mark = "Q"
			}
			markers += mark
			styledMarkers += paint(activeToggle, mark)
		}
		if cols.favorite {
			mark := " "
			if m.playlistTrackFavorited(t) {
				mark = favHeart
			}
			markers += mark
			styledMarkers += paint(favMarkerStyle, mark)
		}
		if cols.offline {
			mark := " "
			if m.downloads.manifest.Recorded(t.Path) {
				mark = "↓"
			}
			markers += mark
			styledMarkers += paint(trackStyle, mark)
		}
		if cols.played {
			mark, markStyle := " ", dimStyle
			if state, ok := playbackStateFrom(stateReporters, t); ok {
				switch {
				case state.Played:
					mark, markStyle = playedMarker, activeToggle
				case state.Position > 0:
					mark, markStyle = partialMarker, dimStyle
				}
			}
			markers += mark
			styledMarkers += paint(markStyle, mark)
		}
		markers += " "
		styledMarkers += " "

		name := trackViewName(t)
		queueSuffix := ""
		if queuePosition > 0 && m.layout.panelWidth >= 64 {
			queueSuffix = fmt.Sprintf(" [Q%d]", queuePosition)
		}
		queueLen := textWidth(queueSuffix)
		duration := formatTrackTime(t.DurationSecs)
		durationLen := textWidth(duration)
		durationGap := 0
		if duration != "" {
			durationGap = durationLen + 1
		}

		linePrefixWidth := textWidth(markers) + numWidth + 2 // 2 for ". "

		// State markers always occupy the same cells; low-priority queue position,
		// album, and unavailable labels appear only when the terminal has room.
		name = truncate(name, width-linePrefixWidth-queueLen-durationGap)
		// Truncate the album to fit whatever space remains after the track name.
		albumSuffix := ""
		nameLen := textWidth(name)
		if t.Unplayable && m.layout.panelWidth >= 68 {
			remaining := width - linePrefixWidth - nameLen - queueLen - durationGap
			if remaining >= len(" (unavailable)") {
				albumSuffix = truncate(" (unavailable)", remaining)
			}
		} else if album := t.Album; album != "" && !m.showAlbumHeaders && m.layout.panelWidth >= 56 {
			remaining := width - linePrefixWidth - nameLen - queueLen - durationGap - 3 // 3 = " · "
			if remaining >= 4 {
				albumSuffix = " · " + truncate(album, remaining)
			}
		}

		numStr := fmt.Sprintf("%*d. ", numWidth, i+1)
		line := styledMarkers + paint(style, numStr)
		line += paint(style, name)
		if albumSuffix != "" {
			line += paint(dimStyle, albumSuffix)
		}
		if queueSuffix != "" {
			line += paint(activeToggle, queueSuffix)
		}
		if duration != "" {
			padding := max(1, width-textWidth(line)-durationLen)
			line += strings.Repeat(" ", padding) + paint(dimStyle, duration)
		}
		lines = append(lines, line)
	}

	lines = padLines(lines, budget, len(lines))
	if width < m.layout.panelWidth {
		lines = withScrollbar(lines, width, trackCount, scroll)
	}
	return strings.Join(lines, "\n")
}

// renderHelp renders the key hints of what owns the keys.
func (m Model) renderHelp() string {
	mode, _ := m.commandContext()
	return m.commandHelp(mode)
}

// renderBottomStatus renders the bottom status line: speed (left) and
// network stats (right) on the same row.
func (m Model) renderBottomStatus() string {
	// Left: speed indicator.
	speed := m.player.Speed()
	if speed == 0 {
		speed = 1.0
	}
	speedVal := fmt.Sprintf("%.2gx", speed)

	var left string
	speedLabel := labelStyle.Render("SPD ")
	if m.focus == focusSpeed {
		speedLabel = activeToggle.Render("SPD ▸ ")
		left = speedLabel + activeToggle.Render("["+speedVal+"]")
	} else if speed != 1.0 {
		left = speedLabel + activeToggle.Render("["+speedVal+"]")
	} else {
		left = speedLabel + dimStyle.Render("[") + trackStyle.Render(speedVal) + dimStyle.Render("]")
	}

	// Right: network stream stats (empty for local files).
	var right string
	if stats := m.networkStatsText(); stats != "" {
		right = dimStyle.Render(stats)
	}

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(right)
	gap := max(1, m.layout.panelWidth-leftW-rightW)

	if right == "" {
		return left
	}
	return left + strings.Repeat(" ", gap) + right
}

// networkStatsText formats the live stream download counters, or "" for local
// playback where there is nothing to report. Shared by the bottom status line
// and the two-column settings pane.
func (m Model) networkStatsText() string {
	downloaded, total := m.player.StreamBytes()
	if downloaded <= 0 && total <= 0 {
		return ""
	}

	mb := float64(downloaded) / (1024 * 1024)
	text := fmt.Sprintf("↓ %.1f MB", mb)
	if total > 0 {
		totalMB := float64(total) / (1024 * 1024)
		pct := float64(downloaded) / float64(total) * 100
		text = fmt.Sprintf("↓ %.1f / %.1f MB (%.0f%%)", mb, totalMB, pct)
	}
	if m.network.speed > 0 {
		kbs := m.network.speed / 1024
		if kbs >= 1024 {
			text += fmt.Sprintf("  %.1f MB/s", kbs/1024)
		} else {
			text += fmt.Sprintf("  %.0f KB/s", kbs)
		}
	}
	return text
}

package model

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"cliamp/playlist"
	"cliamp/ui"
)

// formatTrackTime formats a duration in seconds as M:SS or H:MM:SS for tracks.
// Returns "" when secs is non-positive so callers can skip rendering entirely.
func formatTrackTime(secs int) string {
	if secs <= 0 {
		return ""
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
}

// formatPlaylistDuration formats a total runtime for a playlist as "1h 23m"
// or "12m" or "45s". Returns "" when secs is non-positive.
func formatPlaylistDuration(secs int) string {
	if secs <= 0 {
		return ""
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	if h > 0 {
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", secs)
}

// formatTrackRow renders a track list row of the form
//
//	"01. Title · Album         3:42"
//
// with the duration right-aligned at ui.PanelWidth - 4 (to leave space for
// the cursor prefix the caller adds). The title column is truncated as
// needed; the duration is hidden when secs is 0.
func formatTrackRow(num int, name string, secs int) string {
	const prefixOverhead = 4 // "  " or "> " + room
	dur := formatTrackTime(secs)
	numStr := fmt.Sprintf("%d. ", num)
	titleBudget := ui.PanelWidth - prefixOverhead - utf8.RuneCountInString(numStr)
	if dur != "" {
		titleBudget -= utf8.RuneCountInString(dur) + 1 // +1 for spacing gap
	}
	if titleBudget < 4 {
		titleBudget = 4
	}
	title := truncate(name, titleBudget)
	if dur == "" {
		return numStr + title
	}
	used := utf8.RuneCountInString(numStr) + utf8.RuneCountInString(title)
	target := ui.PanelWidth - prefixOverhead - utf8.RuneCountInString(dur)
	pad := target - used
	if pad < 1 {
		pad = 1
	}
	return numStr + title + strings.Repeat(" ", pad) + dur
}

// totalTrackSecs sums DurationSecs across a slice, skipping unknown entries.
func totalTrackSecs(tracks []playlist.Track) int {
	total := 0
	for _, t := range tracks {
		if t.DurationSecs > 0 {
			total += t.DurationSecs
		}
	}
	return total
}

// truncate shortens s to maxW runes, appending "…" if truncated.
// Uses RuneCountInString first to avoid rune slice allocation in the common
// case where the string is already short enough.
func truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}
	r := []rune(s)
	return string(r[:maxW-1]) + "…"
}

// cursorLine renders a list item with "> " prefix when active, "  " otherwise.
func cursorLine(label string, active bool) string {
	if active {
		return playlistSelectedStyle.Render("> " + label)
	}
	return dimStyle.Render("  " + label)
}

// scrollStart returns the scroll offset so that cursor remains visible
// within a window of maxVisible items.
func scrollStart(cursor, maxVisible int) int {
	if cursor >= maxVisible {
		return cursor - maxVisible + 1
	}
	return 0
}

// spinnerFrames is the braille-dot animation used to indicate loading. The
// view re-renders on the model tick so the spinner advances on its own.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinnerFrame returns the current animation frame, time-driven so the caller
// doesn't need to track an animation index.
func spinnerFrame() string {
	idx := (time.Now().UnixMilli() / 100) % int64(len(spinnerFrames))
	return spinnerFrames[idx]
}

// loadingLine renders a single styled "<spinner> <label>" line for use as a
// loading indicator inside a list pane.
func loadingLine(label string) string {
	return activeToggle.Render("  "+spinnerFrame()) + dimStyle.Render(" "+label)
}

// padLines appends empty strings so that rendered items fill maxVisible rows.
func padLines(lines []string, maxVisible, rendered int) []string {
	for range maxVisible - rendered {
		lines = append(lines, "")
	}
	return lines
}

// helpKey renders a key as a pill (background-highlighted) followed by a dim label.
func helpKey(key, label string) string {
	return helpKeyStyle.Render(" "+key+" ") + helpStyle.Render(" "+label)
}

// isStreamingPlaylistTrack reports whether path is a streaming-provider URI
// whose Album metadata may not represent a real album grouping (so separators
// would be misleading).
func isStreamingPlaylistTrack(path string) bool {
	return strings.HasPrefix(path, "spotify:track:")
}

// albumSeparatorRows counts rendered rows between scroll and cursor (inclusive)
// in a playlist view that emits an album-separator row whenever the album
// changes. Streaming tracks are treated as not contributing a separator,
// matching the renderer.
func albumSeparatorRows(tracks []playlist.Track, scroll, cursor int) int {
	if len(tracks) == 0 || scroll < 0 || cursor < scroll || cursor >= len(tracks) {
		return 0
	}
	rows := 0
	prevAlbum := ""
	if scroll > 0 {
		prevAlbum = tracks[scroll-1].Album
	}
	for i := scroll; i <= cursor; i++ {
		if album := tracks[i].Album; album != "" && album != prevAlbum && !isStreamingPlaylistTrack(tracks[i].Path) {
			rows++
		}
		prevAlbum = tracks[i].Album
		rows++
	}
	return rows
}

// albumSeparator builds a full-width album divider line.
func (m Model) albumSeparator(album string, year int) string {
	prefix := "── "
	suffix := " "
	label := prefix + album
	if year != 0 {
		label += fmt.Sprintf(" (%d)", year)
	}
	label += suffix
	if labelLen := utf8.RuneCountInString(label); labelLen < ui.PanelWidth {
		label += strings.Repeat("─", ui.PanelWidth-labelLen)
	}
	return dimStyle.Render(label)
}

// navScrollItems renders a filtered or unfiltered scrolled list for nav browsers.
func (m Model) navScrollItems(total int, labelFn func(int) string) []string {
	maxVisible := max(m.plVisible, 5)

	useFilter := len(m.navBrowser.searchIdx) > 0 || m.navBrowser.search != ""
	scroll := m.navBrowser.scroll

	var lines []string
	rendered := 0

	if useFilter {
		for j := scroll; j < len(m.navBrowser.searchIdx) && rendered < maxVisible; j++ {
			label := labelFn(m.navBrowser.searchIdx[j])
			lines = append(lines, cursorLine(label, j == m.navBrowser.cursor))
			rendered++
		}
	} else {
		for i := scroll; i < total && rendered < maxVisible; i++ {
			label := labelFn(i)
			lines = append(lines, cursorLine(label, i == m.navBrowser.cursor))
			rendered++
		}
	}

	return padLines(lines, maxVisible, rendered)
}

// navCountLine renders an "X/Y noun (filtered)" footer.
func (m Model) navCountLine(noun string, total int) string {
	if len(m.navBrowser.searchIdx) > 0 || m.navBrowser.search != "" {
		return dimStyle.Render(fmt.Sprintf("  %d/%d %s (filtered)", len(m.navBrowser.searchIdx), total, noun))
	}
	return dimStyle.Render(fmt.Sprintf("  %d/%d %s", m.navBrowser.cursor+1, total, noun))
}

// navSearchBar renders a footer help line plus, when a filter is active or
// being typed, an inline reminder. The active filter input is rendered at the
// top of the screen by navSearchHeader; this helper now only owns the footer.
func (m Model) navSearchBar(defaultHelp string) []string {
	return []string{"", defaultHelp}
}

// navSearchHeader renders the filter input below the title (when typing) or a
// dim recap when a filter is set but the input bar is closed. Returns nil when
// no filter is active so callers can append unconditionally.
func (m Model) navSearchHeader() []string {
	if m.navBrowser.searching {
		return []string{
			playlistSelectedStyle.Render("  / " + m.navBrowser.search + "_"),
			"",
		}
	}
	if m.navBrowser.search != "" {
		return []string{
			dimStyle.Render("  / "+m.navBrowser.search) + " " + helpKey("/", "Clear"),
			"",
		}
	}
	return nil
}

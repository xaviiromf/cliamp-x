package model

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/stats"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// The statistics screen (S) sums up what was listened to in cliamp-x: plays,
// listening time, the streak of days, daily activity and the top artists,
// songs and genres of the last week, month, year or all time. A song counts
// once it played for statsMinListen, or half of it when it is shorter.

const (
	statsMinListen = 30 * time.Second
	statsTop       = 10
	statsDays      = 30
)

type statsState struct {
	visible bool
	loading bool
	period  stats.Period
	// summaries holds the sums of each period, by stats.Period. They are
	// made once per opening: summing every play on each frame cost
	// milliseconds per frame with a long history.
	summaries []stats.Summary
	empty     bool // nothing listened yet
	err       string
	scroll    int
}

type statsLoadedMsg struct {
	plays     []stats.Play
	summaries []stats.Summary // nil to make them from plays
	err       error
}

// summarizeStats sums plays for each period, at now.
func summarizeStats(plays []stats.Play, now time.Time) []stats.Summary {
	out := make([]stats.Summary, stats.All+1)
	for p := stats.Week; p <= stats.All; p++ {
		out[p] = stats.Summarize(plays, p, now, statsTop, statsDays)
	}
	return out
}

// SetStatsLog sets where listened songs are recorded.
func (m *Model) SetStatsLog(l *stats.Log) { m.statsLog = l }

// recordPlay logs a song the listener left after elapsed, when it played
// long enough to count.
func (m *Model) recordPlay(track playlist.Track, elapsed, dur time.Duration) {
	if m.statsLog == nil || track.Path == "" {
		return
	}
	if elapsed < statsMinListen && (dur <= 0 || elapsed < dur/2) {
		return
	}
	p := stats.Play{At: time.Now(), Key: track.Path, Title: track.Title, Artist: track.Artist, Album: track.Album, Genre: track.Genre, Secs: elapsed.Seconds()}
	log := m.statsLog
	go func() { _ = log.Record(p) }()
}

func (m *Model) openStats() tea.Cmd {
	if m.statsLog == nil {
		return nil
	}
	m.stats = statsState{visible: true, loading: true, period: m.stats.period}
	log := m.statsLog
	return func() tea.Msg {
		plays, err := log.Load()
		return statsLoadedMsg{plays: plays, summaries: summarizeStats(plays, time.Now()), err: err}
	}
}

func (m *Model) handleStatsLoaded(msg statsLoadedMsg) {
	m.stats.loading = false
	m.stats.empty = len(msg.plays) == 0
	m.stats.summaries = msg.summaries
	if m.stats.summaries == nil {
		m.stats.summaries = summarizeStats(msg.plays, time.Now())
	}
	if msg.err != nil {
		m.stats.err = msg.err.Error()
	}
}

func (m *Model) handleStatsKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "q", "S", "backspace":
		m.stats.visible = false
	case "right", "l", "tab":
		m.stats.period = (m.stats.period + 1) % 4
		m.stats.scroll = 0
	case "left", "h", "shift+tab":
		m.stats.period = (m.stats.period + 3) % 4
		m.stats.scroll = 0
	case "down", "j":
		m.stats.scroll++
	case "up", "k":
		m.stats.scroll = max(0, m.stats.scroll-1)
	case "home", "g":
		m.stats.scroll = 0
	}
	return nil
}

func statsPeriodName(p stats.Period) string {
	switch p {
	case stats.Week:
		return L("Week")
	case stats.Month:
		return L("Month")
	case stats.Year:
		return L("Year")
	}
	return L("All time")
}

func (m Model) statsHeaderLine() string {
	var parts []string
	for p := stats.Week; p <= stats.All; p++ {
		name := statsPeriodName(p)
		if p == m.stats.period {
			name = "[" + name + "]"
		}
		parts = append(parts, name)
	}
	return sepHeader(L("Statistics")+" · "+strings.Join(parts, " "), m.layout.panelWidth)
}

// formatListenTime turns seconds into "12 h 30 min" or "45 min".
func formatListenTime(secs float64) string {
	mins := int(secs / 60)
	if mins < 60 {
		return fmt.Sprintf("%d min", mins)
	}
	return fmt.Sprintf("%d h %d min", mins/60, mins%60)
}

// eighthBlocks are the bar tops from empty to full, in eighths of a cell.
var eighthBlocks = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// statsChartRows is the height of the activity chart.
const statsChartRows = 4

// activityChart draws values as bars rows high, each bar as wide as width
// allows (up to 3 cells with a gap), with eighth blocks for the top of each
// bar, like the bar visualizers.
func activityChart(values []float64, rows, width int) []string {
	if len(values) == 0 || rows <= 0 {
		return nil
	}
	colW := max(1, min(3, width/len(values)))
	peak := 0.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	out := make([]string, rows)
	for r := range rows {
		var b strings.Builder
		floor := float64(rows-1-r) * 8 // eighths below this row
		for _, v := range values {
			level := 0.0
			if peak > 0 && v > 0 {
				level = math.Max(1, math.Round(v/peak*float64(rows*8)))
			}
			cell := " "
			switch fill := level - floor; {
			case fill >= 8:
				cell = "█"
			case fill > 0:
				cell = eighthBlocks[int(fill)]
			}
			b.WriteString(strings.Repeat(cell, max(1, colW-1)))
			if colW > 1 {
				b.WriteByte(' ')
			}
		}
		out[r] = b.String()
	}
	return out
}

func (m Model) renderStatsBody() string {
	budget := m.effectivePlaylistVisible()
	width := m.layout.panelWidth
	var lines []string
	switch {
	case m.stats.loading:
		lines = []string{dimStyle.Render("  " + L("Loading…"))}
	case m.stats.err != "":
		lines = []string{errorStyle.Render("  " + m.stats.err)}
	case m.stats.empty:
		lines = []string{dimStyle.Render("  " + L("Nothing listened yet. A song counts after 30 seconds."))}
	default:
		lines = m.statsLines(width)
	}
	lines = scrolledLines(lines, m.stats.scroll)
	return bodyLines(lines, budget)
}

func (m Model) statsLines(width int) []string {
	s := m.stats.summaries[m.stats.period]
	var lines []string
	summary := fmt.Sprintf("  %s · %d %s · %d %s · %d %s",
		formatListenTime(s.Secs), s.Plays, L("plays"), s.Songs, L("songs"), s.Artists, L("artists"))
	if s.Streak > 0 {
		summary += fmt.Sprintf(" · %s %d %s", L("streak of"), s.Streak, L("days"))
	}
	lines = append(lines, trackStyle.Render(truncate(summary, width)), "")
	lines = append(lines, dimStyle.Render("  "+L("Activity of the last 30 days")))
	for _, row := range activityChart(s.Daily, statsChartRows, width-2) {
		lines = append(lines, "  "+trackStyle.Render(row))
	}
	lines = append(lines, "")
	section := func(title string, counts []stats.Count, withDetail bool) {
		if len(counts) == 0 {
			return
		}
		lines = append(lines, dimStyle.Render("  "+title))
		top := counts[0].Plays
		nameW := min(36, max(12, width/3))
		barW := max(4, min(30, width-nameW-16))
		for i, c := range counts {
			name := c.Name
			if withDetail && c.Detail != "" {
				name += " — " + c.Detail
			}
			bar := strings.Repeat("█", max(1, c.Plays*barW/max(1, top)))
			lines = append(lines, fmt.Sprintf("  %2d. %-*s %s %s", i+1, nameW, truncate(name, nameW), trackStyle.Render(bar), dimStyle.Render(fmt.Sprint(c.Plays))))
		}
		lines = append(lines, "")
	}
	section(L("Top artists"), s.TopArtists, false)
	section(L("Top songs"), s.TopSongs, true)
	section(L("Top genres"), s.TopGenres, false)
	return lines
}

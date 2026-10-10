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
	plays   []stats.Play
	err     string
	scroll  int
}

type statsLoadedMsg struct {
	plays []stats.Play
	err   error
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
		return statsLoadedMsg{plays: plays, err: err}
	}
}

func (m *Model) handleStatsLoaded(msg statsLoadedMsg) {
	m.stats.loading = false
	m.stats.plays = msg.plays
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
		return tr("Semana", "Week")
	case stats.Month:
		return tr("Mes", "Month")
	case stats.Year:
		return tr("Año", "Year")
	}
	return tr("Todo", "All time")
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
	return sepHeader(tr("Estadísticas", "Statistics")+" · "+strings.Join(parts, " "), m.layout.panelWidth)
}

// formatListenTime turns seconds into "12 h 30 min" or "45 min".
func formatListenTime(secs float64) string {
	mins := int(secs / 60)
	if mins < 60 {
		return fmt.Sprintf("%d min", mins)
	}
	return fmt.Sprintf("%d h %d min", mins/60, mins%60)
}

// sparkline draws values as block heights.
func sparkline(values []float64) string {
	const blocks = "▁▂▃▄▅▆▇█"
	peak := 0.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	var b strings.Builder
	for _, v := range values {
		if v <= 0 || peak == 0 {
			b.WriteRune(' ')
			continue
		}
		i := int(v / peak * 7)
		b.WriteRune([]rune(blocks)[min(7, i)])
	}
	return b.String()
}

func (m Model) renderStatsBody() string {
	budget := m.effectivePlaylistVisible()
	width := m.layout.panelWidth
	var lines []string
	switch {
	case m.stats.loading:
		lines = []string{dimStyle.Render("  " + tr("Cargando…", "Loading…"))}
	case m.stats.err != "":
		lines = []string{errorStyle.Render("  " + m.stats.err)}
	case len(m.stats.plays) == 0:
		lines = []string{dimStyle.Render("  " + tr("Todavía no hay canciones escuchadas. Una canción cuenta después de 30 segundos.", "Nothing listened yet. A song counts after 30 seconds."))}
	default:
		lines = m.statsLines(width)
	}
	if m.stats.scroll > 0 && m.stats.scroll < len(lines) {
		lines = lines[m.stats.scroll:]
	}
	return bodyLines(lines, budget)
}

func (m Model) statsLines(width int) []string {
	s := stats.Summarize(m.stats.plays, m.stats.period, time.Now(), statsTop, statsDays)
	var lines []string
	summary := fmt.Sprintf("  %s · %d %s · %d %s · %d %s",
		formatListenTime(s.Secs), s.Plays, tr("reproducciones", "plays"), s.Songs, tr("canciones", "songs"), s.Artists, tr("artistas", "artists"))
	if s.Streak > 0 {
		summary += fmt.Sprintf(" · %s %d %s", tr("racha de", "streak of"), s.Streak, tr("días", "days"))
	}
	lines = append(lines, trackStyle.Render(truncate(summary, width)), "")
	lines = append(lines, dimStyle.Render("  "+tr("Actividad de los últimos 30 días", "Activity of the last 30 days")))
	lines = append(lines, "  "+trackStyle.Render(sparkline(s.Daily)), "")
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
	section(tr("Artistas más escuchados", "Top artists"), s.TopArtists, false)
	section(tr("Canciones más escuchadas", "Top songs"), s.TopSongs, true)
	section(tr("Géneros más escuchados", "Top genres"), s.TopGenres, false)
	return lines
}

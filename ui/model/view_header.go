package model

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

// The top of the player screen: the title row with its mode tags, the song
// that plays, the clock, the seek bar and the playback controls.

// Marquee for the now-playing title. A name that fits its row is drawn as it
// is; a longer one loops through marqueeGap so its end and its restart do not
// run together, and holds at the start of every pass for marqueeHoldTicks so
// the beginning stays readable. titleScrollInterval is how often it advances.
const (
	marqueeGap          = "   ·   "
	marqueeHoldTicks    = 8
	titleScrollInterval = 200 * time.Millisecond
)

func (m Model) renderSimplifiedTrackInfo() string {
	track, _ := m.currentPlaybackTrack()
	name := trackInfoName(track, m.streamTitle)

	duration := formatTrackTime(int(m.cachedDur.Seconds()))
	if duration == "" {
		duration = formatTrackTime(track.DurationSecs)
	}
	if duration == "" {
		duration = "LIVE"
		if !track.Stream {
			duration = "--:--"
		}
	}

	name = scrollTrackName(name, max(1, m.layout.panelWidth-lipgloss.Width(duration)-1), m.titleOff)
	gap := max(1, m.layout.panelWidth-lipgloss.Width(name)-lipgloss.Width(duration))
	return trackStyle.Render(name) + strings.Repeat(" ", gap) + dimStyle.Render(duration)
}

func trackInfoName(track playlist.Track, streamTitle string) string {
	if streamTitle != "" && track.Stream {
		return streamTitle
	}

	name := trackViewName(track)
	if track.Meta(provider.MetaPodcastFeed) != "" {
		name = track.DisplayName()
		if track.Restricted {
			name = strings.TrimSpace(name) + restrictedViewSuffix
		}
	}
	if name == "" {
		name = L("No track loaded")
	}
	if track.Album != "" {
		name += " · " + track.Album
	}
	return name
}

// scrollTrackName returns the width-cell window of name at the given tick.
// Widths are measured in display cells rather than runes, so a title made of
// wide glyphs fills its row exactly instead of overflowing it and wrapping the
// frame onto an extra line.
func scrollTrackName(name string, width, tick int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(name) <= width {
		return name
	}

	source := name + marqueeGap
	cycle := lipgloss.Width(source)
	phase := max(0, tick) % (cycle + marqueeHoldTicks)
	offset := max(0, phase-marqueeHoldTicks)

	// Doubling the source lets a window near the end wrap into the restart
	// without a second case to get wrong.
	return ansi.Truncate(ansi.TruncateLeft(source+source, offset, ""), width, "")
}

func (m *Model) resetTitleScroll() {
	m.titleOff = 0
	m.titleLastScroll = time.Time{}
}

func (m *Model) advanceTitleScroll(now time.Time) {
	if m.player == nil || !m.player.IsPlaying() || m.player.IsPaused() {
		return
	}
	if now.Sub(m.titleLastScroll) < titleScrollInterval {
		return
	}
	m.titleLastScroll = now
	m.titleOff++
}

func (m Model) renderCompactControls() string {
	mono := ""
	if m.player.Mono() {
		mono = " [M]"
	}
	eqLabel := labelStyle.Render("EQ ")
	eqValue := activeToggle.Render("[" + m.EQPresetName() + "]")
	if m.focus == focusEQ {
		eqLabel = activeToggle.Render("EQ ▸ ")
		bands := m.player.EQBands()
		eqValue += " " + eqActiveStyle.Render(fmt.Sprintf("%s %+.0fdB", eqBandLabels[m.eqCursor], bands[m.eqCursor]))
	}
	volLabel := labelStyle.Render("VOL ")
	if m.focus == focusVolume {
		volLabel = activeToggle.Render("VOL ▸ ")
	}
	return eqLabel + eqValue + " " + volLabel + fmt.Sprintf("%+.0fdB", m.player.Volume()) + mono
}

func (m Model) renderCompactSource() string {
	return m.settingsSource(m.layout.panelWidth)
}

func (m Model) renderTitle() string {
	title := titleStyle.Render("C L I A M P - X")
	label := m.focus.label()
	if m.focus == focusPlaylist {
		label = "Queue" // what plays now, apart from Explore
	}
	if screen := m.activeScreen(); screen != screenMain {
		label = screen.label()
	}
	indicator := ""
	if label != "" {
		indicator = dimStyle.Render("[" + L(label) + "]")
	}
	// The mode tags sit before the screen name while they fit; the ones that
	// do not are dropped from the end.
	room := m.layout.panelWidth - lipgloss.Width(title) - lipgloss.Width(indicator) - 4
	var tags []string
	for _, tag := range m.modeTags(time.Now()) {
		if textWidth(strings.Join(append(tags, tag), " · ")) > room {
			break
		}
		tags = append(tags, tag)
	}
	if len(tags) > 0 {
		styled := make([]string, len(tags))
		for i, tag := range tags {
			styled[i] = paint(trackStyle, tag)
		}
		indicator = strings.Join(styled, paint(dimStyle, " · ")) + "  " + indicator
	}
	if indicator == "" {
		return title
	}
	gap := max(m.layout.panelWidth-lipgloss.Width(title)-lipgloss.Width(indicator), 1)
	return title + strings.Repeat(" ", gap) + indicator
}

// modeTags names the modes that are on and change what plays next or when
// it stops, so they show even after their status message is gone: the auto
// radio, the crossfade, the auto EQ, the downloads and the sleep timer.
func (m Model) modeTags(now time.Time) []string {
	var tags []string
	if m.autoRadio {
		tags = append(tags, L("RADIO"))
	}
	if m.crossfade > 0 {
		tags = append(tags, L("XFADE"))
	}
	if m.eqAuto {
		tags = append(tags, L("AUTO EQ"))
	}
	if d := m.downloads; d.running {
		tags = append(tags, fmt.Sprintf("↓%d", len(d.queue)+1))
	}
	if left := m.sleepLabel(now); left != "" {
		tags = append(tags, L("SLEEP")+" "+left)
	}
	return tags
}

func (m Model) renderTrackInfo() string {
	track, _ := m.currentPlaybackTrack()
	name := trackInfoName(track, m.streamTitle)
	// The "♫ " prefix takes two cells; the rest of the row is the marquee's,
	// so a name only scrolls once it genuinely cannot fit.
	return paint(trackStyle, "♫ "+scrollTrackName(name, m.layout.panelWidth-2, m.titleOff))
}

func (m Model) renderTimeStatus() string {
	// Use per-tick cached values to avoid repeated speaker.Lock() calls.
	pos := m.cachedPos
	dur := m.cachedDur

	track, _ := m.currentPlaybackTrack()
	// A YouTube song has a known length before the engine reports one.
	if dur <= 0 && track.DurationSecs > 0 {
		dur = time.Duration(track.DurationSecs) * time.Second
	}

	posMin := int(pos.Minutes())
	posSec := int(pos.Seconds()) % 60
	durMin := int(dur.Minutes())
	durSec := int(dur.Seconds()) % 60

	timeStr := fmt.Sprintf("%02d:%02d / %02d:%02d", posMin, posSec, durMin, durSec)

	if track.Stream && !m.player.Seekable() && (track.Realtime || dur <= 0) {
		timeStr = fmt.Sprintf("%02d:%02d / LIVE", posMin, posSec)
	}

	var status string
	switch {
	case m.seek.active:
		status = paint(statusStyle, "⟳ Seeking...")
	case m.buffering:
		if elapsed := int(time.Since(m.bufferingAt).Seconds()); elapsed > 0 {
			status = paint(statusStyle, fmt.Sprintf("◌ %s (%ds)", L("Buffering..."), elapsed))
		} else {
			status = paint(statusStyle, "◌ "+L("Buffering..."))
		}
	case m.player.IsPlaying() && m.player.IsPaused():
		status = paint(statusStyle, "⏸ "+L("Paused"))
	case m.player.IsPlaying() && track.Stream:
		status = paint(statusStyle, "● Streaming")
	case m.player.IsPlaying():
		status = paint(statusStyle, "▶ "+L("Playing"))
	default:
		status = paint(dimStyle, "■ "+L("Stopped"))
	}

	left := paint(timeStyle, timeStr)
	gap := max(m.layout.panelWidth-textWidth(left)-textWidth(status), 1)

	return left + strings.Repeat(" ", gap) + status
}

func (m Model) renderSpectrum() string {
	if m.visualizerDisabled() {
		return ""
	}
	return m.vis.Render()
}

// renderFullVisualizer renders a full-screen view showing only the visualizer
// with minimal track info and a seek bar.
func (m Model) renderFullVisualizer() string {
	sections := []string{
		m.fullVisTopLine(),
		m.renderTimeStatus(),
		"",
		m.renderFullVisCover(m.renderSpectrum()),
		m.renderSeekBar(),
		"",
		m.fullVisHelp(),
	}

	return strings.Join(sections, "\n")
}

// fullVisHelp is the hint line under the full-screen visualizer. A narrow
// panel drops the middle hints from the right, so Exit and Actions stay.
func (m Model) fullVisHelp() string {
	optional := []string{helpKey("v", "Mode:"+m.vis.ModeName()+" "), helpKey("Spc", "▶❚❚ "), helpKey("<>", "Trk "), helpKey("+-", "Vol "), helpKey("t", "Title ")}
	for n := len(optional); ; n-- {
		line := helpKey("V", "Exit ") + strings.Join(optional[:n], "") + helpKey("?", L("Actions"))
		if n == 0 || textWidth(line) <= m.layout.panelWidth {
			return line
		}
	}
}

// fullVisTopLine names what is playing, or just the source when the track has
// been hidden. The full-screen visualizer is the view most likely to be on a
// shared screen, so naming the episode has to be optional.
func (m Model) fullVisTopLine() string {
	if !m.hideTrackInfo {
		return m.renderTrackInfo()
	}
	// The playing track may belong to a provider the listener has since
	// switched away from, so prefer the one recorded when it started.
	name := m.playingProvider
	if name == "" && m.provider != nil {
		name = m.provider.Name()
	}
	if name == "" {
		name = "Playing"
	}
	return dimStyle.Render("[" + name + "]")
}

func (m Model) renderSeekBar() string {
	if m.layout.panelWidth <= 0 {
		return ""
	}
	// During buffering, show a dim bar — avoids speaker.Lock() contention.
	if m.buffering {
		return paint(seekDimStyle, strings.Repeat(seekEmptyGlyph, m.layout.panelWidth))
	}
	// Show a static streaming bar for non-seekable streams with no known duration.
	if !m.player.Seekable() && m.player.IsPlaying() && m.cachedDur == 0 {
		label := " STREAMING "
		pad := m.layout.panelWidth - textWidth(label)
		if pad < 0 {
			return paint(seekFillStyle, label[:m.layout.panelWidth])
		}
		left := pad / 2
		right := pad - left
		return paint(seekFillStyle, strings.Repeat("━", left)+label+strings.Repeat("━", right))
	}

	pos := m.cachedPos
	dur := m.cachedDur

	var progress float64
	if dur > 0 {
		progress = float64(pos) / float64(dur)
	}
	progress = max(0, min(1, progress))

	w := m.layout.panelWidth
	filled := min(int(progress*float64(w)), w)
	if filled >= w {
		// Finished: there is no cell left to put the head in.
		return paint(seekFillStyle, strings.Repeat(seekFillGlyph, w))
	}

	var b strings.Builder
	b.WriteString(paint(seekFillStyle, strings.Repeat(seekFillGlyph, filled)))
	b.WriteString(paint(seekFillStyle, seekHeadGlyph))
	b.WriteString(m.renderSeekRest(filled+1, w, dur))
	return b.String()
}

func (m Model) renderControls() string {
	// ── EQ [Preset] (left)  ·····  VOL bar dB [Mono] (right) ──

	bands := m.player.EQBands()
	presetName := m.EQPresetName()

	eqParts := make([]string, len(eqBandLabels))
	for i, label := range eqBandLabels {
		style := eqInactiveStyle
		if bands[i] != 0 {
			label = fmt.Sprintf("%+.0f", bands[i])
		}
		if m.focus == focusEQ && i == m.eqCursor {
			style = eqActiveStyle
		}
		eqParts[i] = style.Render(label)
	}

	eqLabel := labelStyle.Render("EQ ")
	if m.focus == focusEQ {
		eqLabel = activeToggle.Render("EQ ▸ ")
	}
	left := eqLabel + dimStyle.Render("[") + activeToggle.Render(presetName) + dimStyle.Render("] ") + strings.Join(eqParts, " ")

	vol := m.player.Volume()
	volMin := m.player.VolumeMin()
	frac := max(0, min(1, (vol-volMin)/(6-volMin)))
	dbStr := fmt.Sprintf(" %+.0fdB", vol)
	monoStr := ""
	if m.player.Mono() {
		monoStr = " " + activeToggle.Render("[M]")
	}

	leftW := lipgloss.Width(left)
	volLabel := labelStyle.Render("VOL ")
	if m.focus == focusVolume {
		volLabel = activeToggle.Render("VOL ▸ ")
	}
	volSuffix := dimStyle.Render(dbStr) + monoStr
	volLabelW := lipgloss.Width(volLabel)
	volSuffixW := lipgloss.Width(volSuffix)
	barW := max(6, (m.layout.panelWidth-leftW-2-volLabelW-volSuffixW)*3/4)
	filled := int(frac * float64(barW))

	bar := volBarStyle.Render(strings.Repeat("█", filled)) +
		dimStyle.Render(strings.Repeat("░", barW-filled))

	right := volLabel + bar + volSuffix
	rightW := lipgloss.Width(right)
	gap := max(1, m.layout.panelWidth-leftW-rightW)

	return left + strings.Repeat(" ", gap) + right
}

// renderSourceVolume is the single chrome row of the closed-pane layout: the
// active source on the left and the volume meter on the right. It is the same
// pair the pane's SRC and VOL rows show, folded onto one row because a closed
// pane is a request for playlist space.
func (m Model) renderSourceVolume() string {
	left := m.renderProviderPill()
	if left == "" {
		left = labelStyle.Render("SRC ") + dimStyle.Render("[") +
			trackStyle.Render(m.providerName()) + dimStyle.Render("]")
	}

	vol := m.player.Volume()
	volMin := m.player.VolumeMin()
	frac := max(0, min(1, (vol-volMin)/(6-volMin)))

	dbStr := fmt.Sprintf(" %+.0fdB", vol)
	mono := ""
	if m.player.Mono() {
		mono = " " + activeToggle.Render("[M]")
	}
	label := labelStyle.Render("VOL ")
	if m.focus == focusVolume {
		label = activeToggle.Render("VOL ▸ ")
	}

	leftW := lipgloss.Width(left)
	fixedW := lipgloss.Width(label) + lipgloss.Width(dbStr) + lipgloss.Width(mono)
	barW := max(6, (m.layout.panelWidth-leftW-2-fixedW)*3/4)
	filled := int(frac * float64(barW))

	right := label + volBarStyle.Render(strings.Repeat("█", filled)) +
		dimStyle.Render(strings.Repeat("░", barW-filled)) + dimStyle.Render(dbStr) + mono

	gap := max(1, m.layout.panelWidth-leftW-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

// providerName names the active provider for chrome that always shows a source
// row, even when there is only one provider to name.
func (m Model) providerName() string {
	if m.provider != nil {
		return m.provider.Name()
	}
	return "Unknown"
}

func (m Model) renderProviderPill() string {
	if len(m.providers) <= 1 {
		return ""
	}

	srcLabel := labelStyle.Render("SRC ")
	if m.focus == focusProvPill {
		srcLabel = activeToggle.Render("SRC ▸ ")
	}
	current := m.providers[m.provPillIdx].Name
	indicator := dimStyle.Render("[") + trackStyle.Render(current) + dimStyle.Render("]") +
		dimStyle.Render(fmt.Sprintf(" %d/%d", m.provPillIdx+1, len(m.providers)))
	if m.focus == focusProvPill {
		indicator = activeToggle.Render("["+current+"]") + dimStyle.Render(fmt.Sprintf(" %d/%d", m.provPillIdx+1, len(m.providers)))
	}
	if m.layout.panelWidth < 110 {
		return srcLabel + indicator
	}

	var neighbors []string
	if m.provPillIdx > 0 {
		neighbors = append(neighbors, dimStyle.Render("["+m.providers[m.provPillIdx-1].Name+"]"))
	}
	if m.provPillIdx+1 < len(m.providers) {
		neighbors = append(neighbors, dimStyle.Render("["+m.providers[m.provPillIdx+1].Name+"]"))
	}
	return srcLabel + strings.Join(append([]string{indicator}, neighbors...), " ")
}

// renderSeekRest draws the unplayed cells from..w of the seek bar. Parts that
// SponsorBlock will skip show in the warning colour.
func (m Model) renderSeekRest(from, w int, dur time.Duration) string {
	if from >= w {
		return ""
	}
	track, _ := m.currentPlaybackTrack()
	segs := m.sponsorSegments(track)
	if len(segs) == 0 || dur <= 0 {
		return paint(seekDimStyle, strings.Repeat(seekEmptyGlyph, w-from))
	}
	cellSecs := dur.Seconds() / float64(w)
	marked := func(cell int) bool {
		lo, hi := float64(cell)*cellSecs, float64(cell+1)*cellSecs
		for _, s := range segs {
			if s.Start < hi && s.End > lo {
				return true
			}
		}
		return false
	}
	var b strings.Builder
	for cell := from; cell < w; {
		run, skip := cell, marked(cell)
		for run < w && marked(run) == skip {
			run++
		}
		style := seekDimStyle
		if skip {
			style = seekSkipStyle
		}
		b.WriteString(paint(style, strings.Repeat(seekEmptyGlyph, run-cell)))
		cell = run
	}
	return b.String()
}

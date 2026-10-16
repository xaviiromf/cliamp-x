package player

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gopxl/beep/v2"

	"github.com/xaviiromf/cliamp-x/applog"
)

const (
	healthTick        = time.Second      // how often the monitor samples the counters
	healthReportEvery = 15 * time.Second // one summary line per this while a track plays
	healthStarveLog   = 100 * time.Millisecond
	healthStarveGap   = 5 * time.Second // at most one starvation line per this
)

// playbackHealth counts how often, and how long, the speaker waited for the
// playing track's audio. The audio goroutine writes it with atomics only; the
// health monitor reads it and does all the formatting and logging.
type playbackHealth struct {
	sr       beep.SampleRate
	starved  atomic.Int64 // waits: prefetch underruns or source reads slower than real time
	waitedNs atomic.Int64
}

// sourceRead records a read of frames from the track source that took
// elapsed on the speaker goroutine. A read slower than the audio it returns
// held the speaker back by the difference: a wait.
func (h *playbackHealth) sourceRead(elapsed time.Duration, frames int) {
	if h == nil || h.sr <= 0 {
		return
	}
	if over := elapsed - h.sr.D(frames); over > 0 {
		h.starved.Add(1)
		h.waitedNs.Add(int64(over))
	}
}

// silence records frames of silence that a prefetch played while it waited
// for audio. start marks the first silence of a wait.
func (h *playbackHealth) silence(frames int, start bool) {
	if h == nil || h.sr <= 0 {
		return
	}
	if start {
		h.starved.Add(1)
	}
	h.waitedNs.Add(int64(h.sr.D(frames)))
}

// healthLine is one "playback:" log line. downloaded < 0 means the source is
// no download, ahead < 0 that the buffered audio is unknown.
type healthLine struct {
	event      string // health, starved or end
	track      string
	downloaded int64
	rate       int64 // bytes/s
	done       bool  // the download finished
	ahead      time.Duration
	aheadBytes int64 // compressed bytes buffered whose duration is unknown
	starved    int64
	waited     time.Duration
	played     time.Duration // audio the speaker took while not paused
	wall       time.Duration // wall time while not paused
}

func (l healthLine) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "playback: %s track=%q", l.event, l.track)
	if l.downloaded >= 0 {
		fmt.Fprintf(&b, " downloaded=%s rate=%s/s", formatHealthBytes(l.downloaded), formatHealthBytes(l.rate))
		if l.done {
			b.WriteString(" download=done")
		}
	}
	if l.ahead >= 0 {
		fmt.Fprintf(&b, " ahead=%.1fs", l.ahead.Seconds())
		if l.aheadBytes > 0 {
			fmt.Fprintf(&b, "+%s", formatHealthBytes(l.aheadBytes))
		}
	}
	fmt.Fprintf(&b, " starved=%d waited=%.1fs", l.starved, l.waited.Seconds())
	if l.wall > 0 {
		fmt.Fprintf(&b, " output=%.1fs/%.1fs", l.played.Seconds(), l.wall.Seconds())
	}
	return b.String()
}

func formatHealthBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0fKB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// pipelineDownload reports the bytes tp has downloaded, whether the download
// finished, and the bytes downloaded but not yet decoded, or -1 when unknown.
// ok is false when tp plays no download.
func pipelineDownload(tp *trackPipeline) (n int64, done bool, pending int64, ok bool) {
	if y, isYTDL := tp.decoder.(*ytdlPipeStreamer); isYTDL && y.relay != nil {
		n, pending, done = y.relay.progress()
		return n, done, pending, true
	}
	if tp.bytesRead != nil {
		n = tp.bytesRead.Load()
		return n, tp.contentLength > 0 && n >= tp.contentLength, -1, true
	}
	return -1, false, -1, false
}

// pipelineAhead is the audio buffered ahead of the speaker: decoded audio in
// the prefetch plus the download not yet decoded, as time when the bitrate is
// known and as bytes otherwise. ahead is -1 when neither is known.
func pipelineAhead(tp *trackPipeline, pending int64) (ahead time.Duration, aheadBytes int64) {
	ahead = -1
	if tp.livePrefetch != nil {
		ahead = tp.livePrefetch.buffered()
	}
	if pending < 0 {
		return ahead, 0
	}
	ahead = max(ahead, 0)
	if kbps := int64(tp.audio.Kbps); kbps > 0 {
		return ahead + time.Duration(pending*8*int64(time.Second)/(kbps*1000)), 0
	}
	return ahead, pending
}

type healthSnap struct {
	at         time.Time
	starved    int64
	waited     time.Duration
	downloaded int64
	played     time.Duration
	wall       time.Duration
}

// healthMonitor turns the counters into log lines. Only the monitor
// goroutine touches it.
type healthMonitor struct {
	p            *Player
	tp           *trackPipeline
	start        healthSnap // when tp became current
	window       healthSnap // last summary line
	prev         healthSnap // last tick
	lastStarve   time.Time
	played, wall time.Duration
	unpaused     bool
	written      int64
}

func (m *healthMonitor) snap(now time.Time) healthSnap {
	h := &m.p.health
	s := healthSnap{at: now, starved: h.starved.Load(), waited: time.Duration(h.waitedNs.Load()), played: m.played, wall: m.wall}
	if m.tp != nil {
		s.downloaded, _, _, _ = pipelineDownload(m.tp)
	}
	return s
}

// line describes m.tp from from to now.
func (m *healthMonitor) line(event string, from, now healthSnap) healthLine {
	l := healthLine{
		event:   event,
		track:   m.tp.path,
		starved: now.starved - from.starved,
		waited:  now.waited - from.waited,
		played:  now.played - from.played,
		wall:    now.wall - from.wall,
	}
	n, done, pending, ok := pipelineDownload(m.tp)
	l.downloaded = -1
	if ok {
		l.downloaded, l.done = n, done
		if secs := now.at.Sub(from.at).Seconds(); secs > 0 {
			l.rate = int64(float64(n-from.downloaded) / secs)
		}
	}
	l.ahead, l.aheadBytes = pipelineAhead(m.tp, pending)
	return l
}

// tick samples the counters at now, logs what is due, and reports whether a
// track still plays.
func (m *healthMonitor) tick(now time.Time) bool {
	p := m.p
	tp, tap := p.healthTrack()

	var written int64
	if tap != nil {
		written = tap.written.Load()
	}
	unpaused := tp != nil && !p.paused.Load()
	if unpaused && m.unpaused && p.sr > 0 {
		m.wall += now.Sub(m.prev.at)
		m.played += p.sr.D(int(written - m.written))
	}
	m.unpaused, m.written = unpaused, written

	if tp != m.tp {
		if m.tp != nil {
			applog.Info("%s", m.line("end", m.start, m.snap(now)))
		}
		m.tp = tp
		m.start = m.snap(now)
		m.start.downloaded = 0 // a preloaded track downloaded before it played
		m.window, m.prev = m.snap(now), m.snap(now)
		return tp != nil
	}
	if tp == nil {
		return false
	}

	cur := m.snap(now)
	if cur.waited-m.prev.waited > healthStarveLog && now.Sub(m.lastStarve) >= healthStarveGap {
		applog.Info("%s", m.line("starved", m.prev, cur))
		m.lastStarve = now
	}
	m.prev = cur
	if now.Sub(m.window.at) >= healthReportEvery {
		// A paused track with a finished download has nothing to report.
		if cur.wall > m.window.wall || cur.downloaded > m.window.downloaded {
			applog.Info("%s", m.line("health", m.window, cur))
		}
		m.window = cur
	}
	return true
}

// healthTrack returns the pipeline that plays, or nil once it ran out with
// no next track, and the speaker tap.
func (p *Player) healthTrack() (*trackPipeline, *tap) {
	p.mu.Lock()
	tp, tap := p.current, p.tap
	p.mu.Unlock()
	if p.gapless != nil && p.gapless.drained.Load() {
		tp = nil
	}
	return tp, tap
}

// startHealthMonitor starts the monitor goroutine unless it runs. It ends
// when nothing plays, after the end line of the last track.
func (p *Player) startHealthMonitor() {
	if !p.healthRunning.CompareAndSwap(false, true) {
		return
	}
	go func() {
		m := &healthMonitor{p: p}
		t := time.NewTicker(healthTick)
		defer t.Stop()
		m.tick(time.Now())
		for now := range t.C {
			if m.tick(now) {
				continue
			}
			p.healthRunning.Store(false)
			// A track that started after the tick but before the Store
			// found the monitor still running: keep watching it.
			if tp, _ := p.healthTrack(); tp == nil || !p.healthRunning.CompareAndSwap(false, true) {
				return
			}
		}
	}()
}

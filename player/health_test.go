package player

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopxl/beep/v2"

	"github.com/xaviiromf/cliamp-x/applog"
)

func TestHealthLineString(t *testing.T) {
	tests := []struct {
		name string
		line healthLine
		want string
	}{
		{
			name: "download with known ahead",
			line: healthLine{event: "health", track: "yt", downloaded: 1258291, rate: 184320, ahead: 34500 * time.Millisecond, starved: 3, waited: 1400 * time.Millisecond, played: 14900 * time.Millisecond, wall: 15 * time.Second},
			want: `playback: health track="yt" downloaded=1.2MB rate=180KB/s ahead=34.5s starved=3 waited=1.4s output=14.9s/15.0s`,
		},
		{
			name: "finished download, ahead in bytes",
			line: healthLine{event: "end", track: "yt", downloaded: 512, rate: 0, done: true, ahead: 3900 * time.Millisecond, aheadBytes: 2048},
			want: `playback: end track="yt" downloaded=512B rate=0B/s download=done ahead=3.9s+2KB starved=0 waited=0.0s`,
		},
		{
			name: "local file",
			line: healthLine{event: "starved", track: "/m/a.flac", downloaded: -1, ahead: -1, starved: 1, waited: 250 * time.Millisecond},
			want: `playback: starved track="/m/a.flac" starved=1 waited=0.2s`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.line.String(); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestPlaybackHealthCounters(t *testing.T) {
	const sr = beep.SampleRate(1000)
	tests := []struct {
		name        string
		record      func(h *playbackHealth)
		wantStarved int64
		wantWaited  time.Duration
	}{
		{"read in real time", func(h *playbackHealth) { h.sourceRead(10*time.Millisecond, 10) }, 0, 0},
		{"read slower than its audio", func(h *playbackHealth) { h.sourceRead(30*time.Millisecond, 10) }, 1, 20 * time.Millisecond},
		{"one underrun over two buffers", func(h *playbackHealth) { h.silence(100, true); h.silence(50, false) }, 1, 150 * time.Millisecond},
		{"two underruns", func(h *playbackHealth) { h.silence(10, true); h.silence(10, true) }, 2, 20 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &playbackHealth{sr: sr}
			tt.record(h)
			if got := h.starved.Load(); got != tt.wantStarved {
				t.Errorf("starved = %d, want %d", got, tt.wantStarved)
			}
			if got := time.Duration(h.waitedNs.Load()); got != tt.wantWaited {
				t.Errorf("waited = %v, want %v", got, tt.wantWaited)
			}
		})
	}
	var nilHealth *playbackHealth
	nilHealth.sourceRead(time.Second, 1) // no panic
	nilHealth.silence(1, true)
	(&playbackHealth{}).sourceRead(time.Second, 1) // no sample rate: no division by zero
}

type slowStreamer struct{ delay time.Duration }

func (s slowStreamer) Stream(samples [][2]float64) (int, bool) {
	time.Sleep(s.delay)
	clear(samples)
	return len(samples), true
}

func (slowStreamer) Err() error { return nil }

func TestGaplessCountsSlowSourceRead(t *testing.T) {
	h := &playbackHealth{sr: 44100}
	g := &gaplessStreamer{health: h}
	g.Replace(slowStreamer{delay: 30 * time.Millisecond})
	g.Stream(make([][2]float64, 441)) // 10 ms of audio that took 30 ms
	if got := h.starved.Load(); got != 1 {
		t.Fatalf("starved = %d, want 1", got)
	}
	if got := time.Duration(h.waitedNs.Load()); got < 20*time.Millisecond {
		t.Fatalf("waited = %v, want at least 20ms", got)
	}
}

// stallAfterStreamer gives n samples, then blocks until release closes.
type stallAfterStreamer struct {
	n       int
	release chan struct{}
}

func (s *stallAfterStreamer) Stream(samples [][2]float64) (int, bool) {
	if s.n > 0 {
		k := min(s.n, len(samples))
		s.n -= k
		return k, true
	}
	<-s.release
	return 0, false
}

func (*stallAfterStreamer) Err() error { return nil }

func TestLivePrefetchCountsUnderrun(t *testing.T) {
	const sr = beep.SampleRate(1000)
	src := &stallAfterStreamer{n: 600, release: make(chan struct{})}
	p := newLivePrefetchStreamer(src, sr)
	h := &playbackHealth{sr: sr}
	p.health = h
	t.Cleanup(func() { p.Close(); close(src.release); p.Wait() })

	waitForLiveSamples(t, p, 600)
	p.Stream(make([][2]float64, 600)) // the start: audio, no wait
	p.Stream(make([][2]float64, 100)) // the download stalled: an underrun begins
	p.Stream(make([][2]float64, 100)) // still waiting
	if got := h.starved.Load(); got != 1 {
		t.Errorf("starved = %d, want 1", got)
	}
	if got := time.Duration(h.waitedNs.Load()); got != 200*time.Millisecond {
		t.Errorf("waited = %v, want 200ms", got)
	}
}

func TestHealthMonitorLogs(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "cliamp.log")
	closeLog, err := applog.Init(logPath, applog.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeLog() })

	var downloaded atomic.Int64
	tp := &trackPipeline{path: "song", bytesRead: &downloaded}
	p := &Player{sr: 1000, gapless: &gaplessStreamer{}, current: tp}
	p.health.sr = 1000
	m := &healthMonitor{p: p}
	t0 := time.Now()

	m.tick(t0)
	downloaded.Store(150 << 10)
	p.health.silence(300, true)
	m.tick(t0.Add(time.Second))
	m.tick(t0.Add(15 * time.Second))
	p.mu.Lock()
	p.current = nil
	p.mu.Unlock()
	if m.tick(t0.Add(16 * time.Second)) {
		t.Error("tick reports a track after stop")
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	log := string(data)
	for _, want := range []string{
		`playback: starved track=\"song\" downloaded=150KB rate=150KB/s starved=1 waited=0.3s`,
		`playback: health track=\"song\" downloaded=150KB rate=10KB/s starved=1 waited=0.3s output=0.0s/15.0s`,
		`playback: end track=\"song\" downloaded=150KB rate=9KB/s starved=1 waited=0.3s output=0.0s/15.0s`,
	} {
		if !strings.Contains(log, want) {
			t.Errorf("log lacks %s\nlog:\n%s", want, log)
		}
	}
}

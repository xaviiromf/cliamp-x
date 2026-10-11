package model

import (
	"fmt"
	"slices"
	"time"
)

// The sleep timer (T) pauses playback after a while, or when the playing
// song ends. The music fades out over the last sleepFade first, so it does
// not stop with a jump.

// sleepFade is how long the fade-out before the pause lasts.
const sleepFade = 10 * time.Second

// sleepCycle is what T steps through, in minutes: off (0), the times, and
// the end of the song (-1).
var sleepCycle = []int{0, 15, 30, 45, 60, 90, -1}

// sleepTimer is the state of the sleep timer.
type sleepTimer struct {
	minutes int       // 0 off, -1 the end of the song, else the time set
	at      time.Time // when the timer pauses, for a time
	track   string    // the song playing when "end of the song" was set
}

func (s sleepTimer) endOfSong() bool { return s.minutes < 0 }

// fader is the part of the real player that fades the output; the test
// engines may lack it.
type fader interface{ SetFade(level float64) }

func (m *Model) setFade(level float64) {
	if f, ok := m.player.(fader); ok {
		f.SetFade(level)
	}
}

// cycleSleepTimer moves the sleep timer to its next step and says so.
func (m *Model) cycleSleepTimer() {
	i := slices.Index(sleepCycle, m.sleep.minutes)
	m.setSleepMinutes(sleepCycle[(i+1)%len(sleepCycle)])
}

// setSleepMinutes sets the timer: minutes > 0 pauses after that long, 0
// turns it off and a negative value pauses at the end of the song.
func (m *Model) setSleepMinutes(minutes int) {
	m.setFade(1)
	m.sleep = sleepTimer{minutes: max(-1, minutes)}
	switch {
	case minutes == 0:
		m.status.Show(L("Sleep timer off"), statusTTLMedium)
	case minutes < 0:
		track, _ := m.currentPlaybackTrack()
		m.sleep.track = track.Path
		m.status.Show(L("Sleep timer: at the end of this song"), statusTTLMedium)
	default:
		m.sleep.at = time.Now().Add(time.Duration(minutes) * time.Minute)
		m.status.Showf(statusTTLMedium, "%s: %d min", L("Sleep timer"), minutes)
	}
}

// sleepLabel is the short text of the header indicator: the minutes left, or
// "end" when the timer waits for the end of the song. It is "" when off.
func (m *Model) sleepLabel(now time.Time) string {
	switch {
	case m.sleep.minutes == 0:
		return ""
	case m.sleep.endOfSong():
		return L("end")
	}
	left := max(0, m.sleep.at.Sub(now))
	return fmt.Sprintf("%dm", int((left+time.Minute-1)/time.Minute))
}

// tickSleep fades the music out as the timer runs out and pauses it at the
// end.
func (m *Model) tickSleep(now time.Time) {
	if m.sleep.minutes == 0 {
		return
	}
	if m.sleep.endOfSong() {
		track, _ := m.currentPlaybackTrack()
		if track.Path != m.sleep.track {
			// The next song started, already silent: pause it at its start.
			m.sleepPause()
			return
		}
		if pos, dur := m.playbackClock(); dur > 0 {
			m.setFade(float64(dur-pos) / float64(sleepFade))
		}
		return
	}
	left := m.sleep.at.Sub(now)
	if left <= 0 {
		m.sleepPause()
		return
	}
	m.setFade(float64(left) / float64(sleepFade))
}

// sleepPause pauses playback for the timer and turns the timer off.
func (m *Model) sleepPause() {
	m.sleep = sleepTimer{}
	m.setFade(1)
	if !m.player.IsPlaying() || m.player.IsPaused() {
		return
	}
	m.togglePlayerPause()
	m.status.Show(L("Sleep timer: playback paused"), statusTTLLong)
	m.notifyPlaybackChange()
}

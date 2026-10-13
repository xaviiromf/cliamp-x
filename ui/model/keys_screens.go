package model

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Keys of the small screens: track info, lyrics, the full-screen visualizer,
// jump to time and the URL input.

// handleInfoKey processes key presses while the track info overlay is open.
func (m *Model) handleInfoKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "i", "q":
		m.info.visible = false
	case "ctrl+i":
		m.info.visible = false
		m.toggleMetadata()
	case "up", "k":
		if m.info.scroll > 0 {
			m.info.scroll--
		}
	case "down", "j":
		m.info.scroll++
		m.infoMaybeAdjustScroll()
	}
	return nil
}

// handleLyricsKey processes key presses while the lyrics overlay is open.
func (m *Model) handleLyricsKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "y", "q":
		nextRequest(&m.requests.lyrics)
		m.lyrics.loading = false
		m.lyrics.query = ""
		m.lyrics.visible = false
	case "r":
		return m.retryLyrics()
	case "[":
		if m.lyricsSyncable() && m.lyricsHaveTimestamps() {
			return m.nudgeLyricsOffset(-250 * time.Millisecond)
		}
	case "]":
		if m.lyricsSyncable() && m.lyricsHaveTimestamps() {
			return m.nudgeLyricsOffset(250 * time.Millisecond)
		}
	case "up", "k":
		if !(m.lyricsSyncable() && m.lyricsHaveTimestamps()) && m.lyrics.scroll > 0 {
			m.lyrics.scroll--
		}
	case "down", "j":
		if !(m.lyricsSyncable() && m.lyricsHaveTimestamps()) {
			maxScroll := max(len(m.lyrics.lines)-1, 0)
			if m.lyrics.scroll < maxScroll {
				m.lyrics.scroll++
			}
		}
	case "ctrl+x":
		m.toggleExpandedView()
	}
	return nil
}

func (m *Model) exitFullVisualizer() {
	m.fullVis = false
	m.recomputeLayout()
}

func (m *Model) handleFullVisualizerKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "backspace", "b", "V", "q":
		m.exitFullVisualizer()
	case "space":
		return m.togglePlayPause()
	case ">", ".":
		return m.skipNext()
	case "<", ",":
		return m.skipPrev()
	case "left":
		return m.doSeek(-5 * time.Second)
	case "shift+left":
		return m.doSeek(-m.seekStepLarge)
	case "right":
		return m.doSeek(5 * time.Second)
	case "shift+right":
		return m.doSeek(m.seekStepLarge)
	case "+", "=":
		m.adjustVolume(1)
	case "-":
		m.adjustVolume(-1)
	case "v":
		_ = m.cycleVisualizer()
	case "}":
		m.adjustVisSensitivity(1)
	case "{":
		m.adjustVisSensitivity(-1)
	case "C":
		m.cycleVisPalette()
	case "t":
		// Hide the episode name so the full-screen visualizer can be put on a
		// shared screen without naming what is playing.
		m.hideTrackInfo = !m.hideTrackInfo
	case "?":
		m.exitFullVisualizer()
		m.openKeymap()

	default:
		// Plugin bindings are global: a pomodoro or sleep-timer key is about
		// the session, not about which screen happens to be open. Without
		// this, every plugin key is dead in the full-screen visualizer —
		// which is exactly where a plugin visualizer is being watched.
		if m.luaMgr != nil {
			m.luaMgr.EmitKey(msg.String())
		}
	}
	return nil
}

func (m *Model) openJumpMode() {
	m.jump = jumpState{active: true}
}

func (m *Model) closeJumpMode() {
	m.jump = jumpState{}
}

// handleJumpKey processes key presses while in jump-time mode.
func (m *Model) handleJumpKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Code {
	case tea.KeyEscape:
		m.closeJumpMode()
		return nil
	case tea.KeyEnter:
		target, err := jumpTarget(m.jump.input, func() time.Duration { return m.player.Duration() })
		if err != nil {
			m.jump.err = "Invalid jump: " + err.Error()
			m.status.Warning(m.jump.err, statusTTLDefault)
			return nil
		}
		if !m.player.Seekable() {
			m.jump.err = "This track cannot be seeked."
			m.status.Warning(m.jump.err, statusTTLDefault)
			return nil
		}
		if dur := m.player.Duration(); dur > 0 && target > dur {
			m.jump.err = fmt.Sprintf("Jump exceeds track duration (%s).", formatJumpClock(dur))
			m.status.Warning(m.jump.err, statusTTLDefault)
			return nil
		}
		cmd, err := m.trySeekAbsolute(target)
		if err != nil {
			m.jump.err = "Seek failed: " + err.Error()
			m.status.Warning(m.jump.err, statusTTLDefault)
			return nil
		}
		m.closeJumpMode()
		return cmd
	}

	if m.editText("jump", &m.jump.input, msg) {
		m.jump.err = ""
	}
	return nil
}

// handleURLInputKey processes key presses while in URL input mode.
func (m *Model) handleURLInputKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Code {
	case tea.KeyEscape:
		m.urlInput.active = false
	case tea.KeyEnter:
		m.urlInput.active = false
		input := strings.TrimSpace(m.urlInput.input)
		if input != "" {
			m.feedLoading = true
			m.status.Activity("Loading URL...", statusTTLLong)
			return resolveURLCmd(input, true)
		}
		m.urlInput.active = true
		m.urlInput.err = "Enter a stream, track, or playlist URL."
	default:
		if m.editText("url", &m.urlInput.input, msg) {
			m.urlInput.err = ""
		}
	}
	return nil
}

package model

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
	"github.com/xaviiromf/cliamp-x/tracksave"
)

// quit shuts down the player and signals the TUI to exit.
func (m *Model) quit() tea.Cmd {
	m.cancelDownloads()
	// Only save resume for seekable tracks:
	// - local files (not stream)
	// - HTTP streams with known duration (podcast MP3s)
	// - finite Mixcloud shows (yt-dlp tracks with a counted PCM position)
	// Other yt-dlp sites and live streams remain excluded. The live check
	// is currentPlaybackIsLive, so a stream the player finds live counts.
	// - YouTube songs with a known length, which seek by restarting yt-dlp
	if track, _ := m.currentPlaybackTrack(); track.Path != "" &&
		(!playlist.IsYTDL(track.Path) || playlist.IsMixcloudURL(track.Path) || track.DurationSecs > 0) &&
		!m.currentPlaybackIsLive(track) &&
		m.player.IsPlaying() && !m.buffering && !m.player.GaplessAdvanced() {
		if secs := int(m.player.Position().Seconds()); secs > 0 {
			context, contextIndex := m.playbackContextFor(track)
			m.exitResume.path = track.Path
			m.exitResume.secs = secs
			m.exitResume.playlist = m.loadedPlaylist
			m.exitResume.context = cloneTracks(context)
			m.exitResume.contextIndex = contextIndex
		}
	}
	m.captureSession()

	m.flushPendingSpeedSave()
	m.flushPendingEQSave()
	// Quit leaves the track that plays, as s does, so it can scrobble.
	m.leaveTrack(m.player.PositionAndDuration())
	m.player.Close()
	m.clearPlaybackTrack()
	m.quitting = true
	return tea.Quit
}

func (m *Model) handleSpeedKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		return m.quit()
	case "]", "right", "l", "up", "k":
		m.changeSpeed(0.25)
	case "[", "left", "h", "down", "j":
		m.changeSpeed(-0.25)
	case "tab":
		m.focus = m.nextMainFocus(focusSpeed)
	case "shift+tab", "esc", "backspace":
		m.focus = m.previousMainFocus(focusSpeed)
	case "space":
		return m.togglePlayPause()
	}
	return nil
}

func normalizeShiftedLetter(msg tea.KeyPressMsg) tea.KeyPressMsg {
	if msg.Text != "" || msg.Mod != tea.ModShift ||
		msg.Code < 'a' || msg.Code > 'z' ||
		msg.ShiftedCode < 'A' || msg.ShiftedCode > 'Z' {
		return msg
	}
	msg.Text = string(msg.ShiftedCode)
	return msg
}

// handleKey processes a single key press and returns an optional command.
// The global keys run first. Then the top overlay, the provider filter or
// the focused area owns the key. The provider filter takes keys only while
// the provider pane has the focus, where commandContext shows it.
func (m *Model) handleKey(msg tea.KeyPressMsg) tea.Cmd {
	msg = normalizeShiftedLetter(msg)

	if cmd, ok := m.handleGlobalKey(msg); ok {
		return cmd
	}
	// The top overlay owns the keys.
	if spec, ok := m.topOverlay(); ok {
		return spec.key(m, msg)
	}

	if m.provSearch.active && m.focus == focusProvider {
		return m.handleProvSearchKey(msg)
	}
	if m.focus != focusProvider {
		switch msg.String() {
		case "ctrl+i":
			m.toggleMetadata()
			return nil
		case "i":
			m.info = infoOverlay{visible: true}
			return nil
		}
	}

	switch m.focus {
	case focusProvider:
		return m.handleProviderPaneKey(msg)
	case focusSpeed:
		return m.handleSpeedKey(msg)
	case focusProvPill:
		return m.handleProvPillKey(msg)
	}
	return m.handleMainKey(msg)
}

// handleGlobalKey processes the keys that work over every overlay and focus:
// quit, undo and the keymap. While the terminal is too small, it takes every
// key and quits on q or ctrl+c. The screen shows only the too-small notice
// then, so no key may change hidden state. ok is false when the key goes on
// to the overlays and the focused area.
func (m *Model) handleGlobalKey(msg tea.KeyPressMsg) (cmd tea.Cmd, ok bool) {
	if msg.String() == "ctrl+c" {
		return m.quit(), true
	}
	if m.width > 0 && m.layout.tooSmall() {
		if msg.String() == "q" {
			return m.quit(), true
		}
		return nil, true
	}
	switch msg.String() {
	case "ctrl+z":
		m.keepPlCursorRow(func() { cmd = m.undoPlaylistMutation() })
		return cmd, true
	case "ctrl+k", "f1":
		if !m.keymap.visible {
			if m.fullVis {
				m.exitFullVisualizer()
			}
			m.openKeymap()
			return nil, true
		}
	}
	return nil, false
}

// handleMainKey processes a key press for the playlist and the focused
// playback controls. It forwards the keys that it does not handle to plugins.
func (m *Model) handleMainKey(msg tea.KeyPressMsg) tea.Cmd {
	// Vim-style count prefix: a digit primes a pending percentage; the next `j`
	// jumps there (e.g. `7j` → 70%). Any other key cancels and runs normally.
	if s := msg.String(); m.focus == focusPlaylist && len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		m.pendingSeekActive = true
		m.pendingSeekPct = int(s[0] - '0')
		m.pendingSeekExpiresAt = time.Now().Add(time.Duration(statusTTLMedium))
		m.status.Activityf(statusTTLMedium, "%dj -> seek to %d%%", m.pendingSeekPct, m.pendingSeekPct*10)
		return nil
	}
	if m.pendingSeekActive {
		pct := m.pendingSeekPct
		m.pendingSeekActive = false
		m.pendingSeekExpiresAt = time.Time{}
		m.status.Clear()
		if msg.String() == "j" && m.focus == focusPlaylist {
			if dur := m.player.Duration(); dur > 0 {
				return m.seekAbsolute(dur * time.Duration(pct) / 10)
			}
			return nil
		}
	}

	// Focused settings reuse the global actions below, including notifications,
	// config persistence, and gapless rearming.
	key := msg.String()
	repeatStep := playlist.RepeatMode(1)
	switch m.focus {
	case focusVolume:
		switch key {
		case "left", "h", "down", "j":
			key = "-"
		case "right", "l", "up", "k":
			key = "+"
		}
	case focusShuffle:
		switch key {
		case "left", "h", "down", "j", "right", "l", "up", "k", "enter":
			key = "z"
		}
	case focusRepeat:
		switch key {
		case "left", "h", "down", "j":
			repeatStep = -1
			key = "r"
		case "right", "l", "up", "k", "enter":
			key = "r"
		}
	}

	if cmd, ok := m.providerShortcut(key); ok {
		return cmd
	}
	switch key {
	case "q":
		return m.quit()
	case "ctrl+r", "f5":
		// The playlist view reloads only an open provider playlist that
		// keeps its ID across Refresh.
		return m.refreshActiveProvider(true)
	case "I":
		return m.rateInterest(taste.Interested)
	case "H":
		return m.rateInterest(taste.NotInterested)
	case "P":
		m.openRatingsMenu()
	case "esc", "backspace", "b":
		if m.focus == focusPlaylist && len(m.plMarks) > 0 {
			// Esc first drops the marks.
			clear(m.plMarks)
			return nil
		}
		if m.focus == focusPlaylist {
			// Keep current expanded/collapsed height mode when switching focus.
			m.focus = focusProvider
			// The Local source has no provider pane list; show its manager.
			m.ensureLocalManager()
		} else {
			m.focus = m.previousMainFocus(m.focus)
		}

	case "space":
		return m.togglePlayPause()

	case "s":
		m.stopByUser()
		return nil

	case ">", ".":
		return m.skipNext()

	case "<", ",":
		return m.skipPrev()

	case "left":
		if m.focus == focusEQ {
			if m.eqCursor > 0 {
				m.eqCursor--
			}
		} else {
			return m.doSeek(-5 * time.Second)
		}

	case "shift+left":
		return m.doSeek(-m.seekStepLarge)

	case "right":
		if m.focus == focusEQ {
			if m.eqCursor < eqBandCount-1 {
				m.eqCursor++
			}
		} else {
			return m.doSeek(5 * time.Second)
		}

	case "shift+right":
		return m.doSeek(m.seekStepLarge)

	case "f":
		return m.togglePlaylistStar()

	case "shift+up", "shift+down":
		if m.focus == focusPlaylist {
			to := m.plCursor + 1
			if key == "shift+up" {
				to = m.plCursor - 1
			}
			cmd, _ := m.moveTrack(m.plCursor, to)
			return cmd
		}

	case "up", "k":
		if m.focus == focusEQ {
			bands := m.player.EQBands()
			m.setCustomEQBand(m.eqCursor, bands[m.eqCursor]+1)
		} else {
			if row := m.plCursorRow(); row > 0 {
				m.setPlCursorRow(row - 1)
			} else if m.playlist.Len() > 0 {
				m.setPlCursorRow(m.playlist.Len() - 1)
			}
		}

	case "down", "j":
		if m.focus == focusEQ {
			bands := m.player.EQBands()
			m.setCustomEQBand(m.eqCursor, bands[m.eqCursor]-1)
		} else {
			if row := m.plCursorRow(); row < m.playlist.Len()-1 {
				m.setPlCursorRow(row + 1)
			} else if m.playlist.Len() > 0 {
				m.setPlCursorRow(0)
			}
		}

	case "pgup", "ctrl+u":
		if row := m.plCursorRow(); m.focus == focusPlaylist && row > 0 {
			visible := max(1, m.effectivePlaylistVisible())
			m.setPlCursorRow(row - min(row, visible))
		}

	case "pgdown", "ctrl+d":
		if row := m.plCursorRow(); m.focus == focusPlaylist && row < m.playlist.Len()-1 {
			visible := max(1, m.effectivePlaylistVisible())
			m.setPlCursorRow(min(m.playlist.Len()-1, row+visible))
		}

	case "g", "home":
		if m.focus == focusPlaylist && m.plCursorRow() != 0 {
			m.setPlCursorRow(0)
		}

	case "G", "end":
		if m.focus == focusPlaylist && m.playlist.Len() > 0 && m.plCursorRow() != m.playlist.Len()-1 {
			m.setPlCursorRow(m.playlist.Len() - 1)
		}

	case "enter":
		if m.focus == focusPlaylist {
			// No-op only if this exact track is still buffering.
			if m.buffering && m.plCursor == m.playlist.Index() {
				break
			}
			return m.playIndex(m.plCursor)
		}

	case "+", "=":
		m.adjustVolume(1)

	case "-":
		m.adjustVolume(-1)

	case "r":
		const repeatModes = playlist.RepeatOne + 1
		return m.setRepeat((m.playlist.Repeat() + repeatStep + repeatModes) % repeatModes)

	case "z":
		return m.setShuffle(!m.playlist.Shuffled())

	case "tab":
		m.focus = m.nextMainFocus(m.focus)
	case "shift+tab":
		m.focus = m.previousMainFocus(m.focus)

	case "h":
		if m.focus == focusEQ && m.eqCursor > 0 {
			m.eqCursor--
		}

	case "l":
		if m.focus == focusEQ && m.eqCursor < eqBandCount-1 {
			m.eqCursor++
		}

	case "e":
		if m.simplified || m.layout.tier == layoutMinimal {
			break
		}
		m.cycleEQPreset()
		m.scheduleEQSave()

	case "a":
		if m.focus == focusPlaylist {
			if !m.playlist.Dequeue(m.plCursor) {
				m.playlist.Queue(m.plCursor)
			}
			m.normalizeQueueOverlay()
			return m.rearmStalePreload()
		}

	case "w":
		if m.focus == focusPlaylist && m.plCursor >= 0 && m.plCursor < m.playlist.Len() {
			if track, ok := m.playlist.Track(m.plCursor); ok {
				m.openPlaylistPicker([]playlist.Track{track}, "Track: "+track.DisplayName())
			}
		}

	case "A":
		if m.focus == focusPlaylist {
			m.queue.visible = true
			m.queue.cursor = 0
			m.queue.scroll = 0
			m.queue.confirmClear = false
		}

	case "F":
		m.openSubsOverlay()

	case "ctrl+s":
		return m.saveTrack()

	case "m":
		m.player.ToggleMono()

	case "/":
		m.search.active = true
		m.search.query = ""
		m.search.results = nil
		m.search.cursor = 0
		m.search.scroll = 0
		m.prevFocus = m.focus
		m.focus = focusSearch
		// Search now renders in the playlist region; recompute chrome so the
		// search header/help are reflected in the visible-row budget.
		m.refreshChrome()
		m.applyHeightMode()

	case "ctrl+f":
		m.openProviderSearch()

	case "ctrl+j":
		m.openJumpMode()
	case "p":
		if m.localProvider != nil {
			m.openPlaylistManager()
		}

	case "t":
		m.openThemePicker()

	case "y":
		m.lyrics.visible = !m.lyrics.visible
		if m.lyrics.visible {
			return m.retryLyrics()
		}

	case "o":
		m.openFileBrowser()

	case "u":
		m.urlInput = urlInputState{active: true}

	case "N":
		if cmd, ok := m.openSelectedTrackArtistBrowser(); ok {
			return cmd
		}
		if providerSupportsBrowse(m.provider) {
			m.openNavBrowserWith(m.provider)
		}

	case "ctrl+h":
		m.toggleAlbumHeadersManual()
		m.adjustScroll()

	case "ctrl+g":
		m.toggleHelpBar()
		m.adjustScroll()

	case "R":
		m.toggleAutoRadio()

	case "E":
		m.openEQPanel()

	case "ctrl+b":
		m.toggleSettingsPane()
		m.adjustScroll()

	case "v":
		if m.simplified {
			break
		}
		_ = m.cycleVisualizer()

	case "ctrl+v":
		if m.simplified {
			break
		}
		m.openVisPicker()

	case "V":
		if m.simplified {
			break
		}
		m.fullVis = !m.fullVis
		m.recomputeLayout()

	case "ctrl+x":
		if !m.simplified && m.focus == focusPlaylist {
			m.toggleExpandedView()
		}

	case "x":
		if m.focus == focusPlaylist {
			if len(m.plMarks) > 0 {
				return m.removeMarkedTracks()
			}
			var cmd tea.Cmd
			m.keepPlCursorRow(func() { cmd, _ = m.removeTrack(m.plCursor, true) })
			return cmd
		}

	case "*":
		if m.focus == focusPlaylist {
			m.toggleMark()
		}

	case "d":
		m.devicePicker.visible = true
		m.devicePicker.cursor = 0
		m.devicePicker.scroll = 0
		if len(m.devicePicker.devices) == 0 {
			m.devicePicker.loading = true
			return listDevicesCmd()
		}

	case "]":
		m.changeSpeed(0.25)

	case "[":
		m.changeSpeed(-0.25)

	case "}":
		m.adjustVisSensitivity(1)

	case "{":
		m.adjustVisSensitivity(-1)

	case "C":
		m.cycleVisPalette()

	case "n":
		m.cycleNormalize()

	case "M":
		return m.moreLikeThis()

	case "X":
		return m.findRemixes()

	case "D":
		return m.downloadHighlighted()

	case "S":
		return m.openStats()

	case "T":
		m.cycleSleepTimer()

	case "ctrl+o":
		m.openDownloads()

	case "c":
		m.revealPlayingTrack()

	case "O":
		m.openRowMenu()

	case "W":
		m.openSaveQueue()

	case "?":
		m.openKeymap()

	default:
		if m.luaMgr != nil {
			m.luaMgr.EmitKey(msg.String())
		}
	}

	return nil
}

// saveTrack saves the current track in the configured downloads directory.
// tracksave.SaveTo runs in a tea.Cmd, so neither a yt-dlp download nor a file
// copy blocks the Update goroutine. IPC save uses the same routine.
func (m *Model) saveTrack() tea.Cmd {
	track, idx := m.currentPlaybackTrack()
	if idx < 0 {
		m.status.Warning("Nothing to save", statusTTLShort)
		return nil
	}
	download := tracksave.NeedsDownload(track)
	if download {
		m.status.Clear()
		m.save.startDownload()
	}
	directory := m.downloadsDirectory
	return func() tea.Msg {
		path, err := tracksave.SaveTo(context.Background(), track, directory)
		return trackSavedMsg{path: path, err: err, download: download}
	}
}

// toggleExpandedView toggles the UI between default and expanded height.
func (m *Model) toggleExpandedView() {
	m.heightExpanded = !m.heightExpanded
	m.applyHeightMode()
	m.adjustScroll()
}

// handlePaste sends pasted text to the text field of the top overlay, as
// handleKey sends keys. With no overlay open, the provider filter takes it
// while the provider pane has the focus.
func (m *Model) handlePaste(content string) tea.Cmd {
	if content == "" {
		return nil
	}
	if spec, ok := m.topOverlay(); ok {
		if spec.paste != nil {
			spec.paste(m, content)
		}
		return nil
	}

	if m.provSearch.active && m.focus == focusProvider {
		m.insertText("provider-search", &m.provSearch.query, content)
		if _, ok := m.provider.(provider.CatalogSearcher); !ok {
			m.updateProvSearch()
		}
		return nil
	}

	return nil
}

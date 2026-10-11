package model

import (
	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/playback"
	"github.com/xaviiromf/cliamp-x/ui"
)

// Update handles messages: key presses, ticks, and window resizes. After each
// message it lays out the frame for View, drops a gapless preload that no
// longer matches the next track, and it tells the media controls and plugins
// when the playback state changed.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(spinnerTickMsg); ok {
		m.spinnerTicking = m.spinnerVisible()
		if !m.spinnerTicking {
			return m, nil
		}
		return m, spinnerTickCmd()
	}
	spinning := m.spinnerVisible()
	next, cmd := m.update(msg)
	if nm, ok := next.(Model); ok {
		nm.recomputeLayout()
		nm.dropStalePreload()
		nm.notifyPlaybackChange()
		// Frame ticks change nothing the preview pane loads, and they come
		// many times a second.
		if _, tick := msg.(tickMsg); !tick {
			if pc := nm.previewCmds(); pc != nil {
				cmd = tea.Batch(cmd, pc)
			}
			if sc := nm.sponsorCmds(); sc != nil {
				cmd = tea.Batch(cmd, sc)
			}
			if cc := nm.coverCmds(); cc != nil {
				cmd = tea.Batch(cmd, cc)
			}
			if nc := nm.notifyCmds(); nc != nil {
				cmd = tea.Batch(cmd, nc)
			}
		}
		// A load that starts now gets its own redraws at once. The main tick
		// can still wait up to ui.TickIdle before it runs at the spinner rate.
		if !spinning && !nm.spinnerTicking && nm.spinnerVisible() {
			nm.spinnerTicking = true
			cmd = tea.Batch(cmd, spinnerTickCmd())
		}
		next = nm
	}
	return next, cmd
}

// update handles one message. Update then lays out the frame, drops a stale
// preload, tells the media controls and plugins about a playback change and
// starts the spinner tick.
func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	wasScreen := m.activeScreen()
	wasVisualizerVisible := m.visualizerVisible()
	wasMode := ui.VisNone
	if m.vis != nil {
		wasMode = m.vis.Mode
	}
	wasPlaying := false
	wasPaused := false
	if m.player != nil {
		wasPlaying = m.player.IsPlaying()
		wasPaused = m.player.IsPaused()
	}
	defer func() {
		m.maybeRequestVisualizerRefresh(msg, wasScreen, wasVisualizerVisible, wasMode, wasPlaying, wasPaused)
		m.emitPluginEvents()
		m.publishIPCRuntimeState()
		m.publishPluginState()
	}()

	switch msg := msg.(type) {
	case tea.PasteMsg:
		cmd := m.handlePaste(msg.Content)
		return m, cmd

	case tea.MouseWheelMsg:
		cmd := m.handleMouseWheel(msg)
		m.adjustScroll()
		return m, cmd

	case tea.MouseClickMsg:
		cmd := m.handleMouseClick(msg)
		m.adjustScroll()
		return m, cmd

	case tea.KeyPressMsg:
		cmd := m.handleKey(msg)
		if m.quitting {
			return m, tea.Quit
		}
		m.applyHeightMode()
		m.adjustScroll()
		return m, cmd

	case autoPlayMsg:
		if m.playlist.Len() > 0 && !m.player.IsPlaying() {
			cmd := m.playCurrentTrack()
			return m, cmd
		}
		return m, nil

	case tea.FocusMsg:
		m.blurred = false
		return m, nil

	case tea.BlurMsg:
		m.blurred = true
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recomputeLayout()
		m.normalizeMainFocus()
		m.clampActiveScrollState()
		return m, nil

	case artworkLoadedMsg:
		return m, nil

	case NotifyActionMsg:
		return m, m.handleNotifyAction(msg)

	case queueSavedMsg:
		m.handleQueueSaved(msg)
		return m, nil

	case statsLoadedMsg:
		m.handleStatsLoaded(msg)
		return m, nil

	case downloadResolvedMsg:
		return m, m.handleDownloadResolved(msg)

	case downloadEventMsg:
		return m, m.handleDownloadEvent(msg)

	case moreLikeThisMsg:
		return m, m.handleMoreLikeThis(msg)

	case sponsorSegmentsMsg:
		m.handleSponsorSegments(msg)
		return m, nil

	case kittyImageMsg:
		return m, m.handleKittyImage(msg)

	case kittyShownMsg:
		m.art.KittyShown(msg.url)
		return m, nil

	case previewDetailsMsg:
		m.handlePreviewDetails(msg)
		return m, nil

	case seekTickMsg:
		cmd := m.handleSeekTick(msg)
		return m, cmd

	case ytdlUnpauseReconnectMsg:
		m.handleYTDLUnpauseReconnect(msg)
		return m, nil

	case tickMsg:
		cmd := m.handleTick(msg)
		return m, cmd

	case openDefaultProviderBrowserMsg:
		if !m.openDefaultProviderOnce {
			return m, nil
		}
		m.openDefaultProviderOnce = false
		cmd := m.openDefaultProviderBrowser()
		return m, cmd

	case autoRadioLoadedMsg:
		cmd := m.handleAutoRadioLoaded(msg)
		return m, cmd

	case playlistsLoadedMsg:
		cmd := m.handlePlaylistsLoaded(msg)
		return m, cmd

	case tracksLoadedMsg:
		cmd := m.handleTracksLoaded(msg)
		return m, cmd

	case navArtistsLoadedMsg:
		m.handleNavArtistsLoaded(msg)
		return m, nil

	case navAlbumsLoadedMsg:
		m.handleNavAlbumsLoaded(msg)
		return m, nil

	case navGenresLoadedMsg:
		m.handleNavGenresLoaded(msg)
		return m, nil

	case navTracksLoadedMsg:
		m.handleNavTracksLoaded(msg)
		return m, nil

	case catalogBatchMsg:
		m.handleCatalogBatch(msg)
		return m, nil

	case catalogSearchMsg:
		m.handleCatalogSearch(msg)
		return m, nil

	case ytdlBatchMsg:
		cmd := m.handleYTDLBatch(msg)
		return m, cmd

	case feedTrackResolvedMsg:
		cmd := m.handleFeedTrackResolved(msg)
		return m, cmd

	case subsEpisodesMsg:
		cmd := m.handleSubsEpisodes(msg)
		return m, cmd

	case subsLatestAllMsg:
		cmd := m.handleSubsLatestAll(msg)
		return m, cmd

	case feedsLoadedMsg:
		cmd := m.handleFeedsLoaded(msg)
		return m, cmd

	case netSearchResultsMsg:
		m.handleNetSearchResults(msg)
		return m, nil

	case lyricsLoadedMsg:
		if msg.gen != m.requests.lyrics || !m.lyrics.visible || msg.query != m.lyrics.query {
			return m, nil
		}
		m.lyrics.loading = false
		m.lyrics.err = msg.err
		m.lyrics.scroll = 0
		if msg.err == nil {
			m.lyrics.lines = msg.lines
		}
		return m, nil

	case fbTracksResolvedMsg:
		cmd := m.handleFBTracksResolved(msg)
		return m, cmd

	case streamPlayedMsg:
		cmd := m.handleStreamPlayed(msg)
		return m, cmd

	case streamPreloadedMsg:
		if msg.gen != m.requests.preload {
			return m, nil
		}
		m.preloading = false
		if msg.err != nil {
			// Playback falls back to a non-gapless start for this track.
			// Retrying on the next tick would rebuild the failing pipeline.
			m.preloadFailed = msg.path
		}
		return m, nil

	case trackSavedMsg:
		m.handleTrackSaved(msg)
		return m, nil

	case searchOverlayResultsMsg:
		m.handleSearchOverlayResults(msg)
		return m, nil

	case searchOverlayAlbumTracksMsg:
		cmd := m.handleSearchOverlayAlbumTracks(msg)
		return m, cmd

	case searchOverlayPlaylistsMsg:
		m.handleSearchOverlayPlaylists(msg)
		return m, nil

	case searchOverlayAddedMsg:
		m.handleSearchOverlayAdded(msg)
		return m, nil

	case searchOverlayCreatedMsg:
		m.handleSearchOverlayCreated(msg)
		return m, nil

	case provAuthDoneMsg:
		cmd := m.handleProvAuthDone(msg)
		return m, cmd

	case ProvAuthURLMsg:
		if !m.provPane.loading || !m.isActiveProvider(msg.ProviderName) {
			return m, nil
		}
		m.provPane.authURL = msg.URL
		return m, nil

	case devicesListedMsg:
		m.devicePicker.loading = false
		if msg.err != nil {
			m.status.Errorf(statusTTLDefault, "Device list failed: %s", msg.err)
			m.devicePicker.visible = false
		} else {
			m.devicePicker.devices = msg.devices
		}
		return m, nil

	case deviceSwitchedMsg:
		if msg.err != nil {
			m.status.Errorf(statusTTLDefault, "Switch failed: %s", msg.err)
		} else {
			m.status.Showf(statusTTLDefault, "Audio output: %s", msg.name)
			m.audioDevice = msg.name
			_ = m.saveConfigString("audio_device", msg.name)
		}
		// Invalidate cached list so the next open refreshes Active markers.
		m.devicePicker.devices = nil
		return m, nil

	case attachNotifierMsg:
		m.attachNotifier(msg.notifier)
		return m, nil

	case playback.PlayPauseMsg:
		cmd := m.togglePlayPause()
		return m, cmd

	case playback.PlayMsg:
		if !m.player.IsPlaying() || m.player.IsPaused() {
			cmd := m.togglePlayPause()
			return m, cmd
		}
		return m, nil

	case playback.PauseMsg:
		if m.player.IsPlaying() && !m.player.IsPaused() {
			m.togglePlayerPause()
		}
		return m, nil

	case playback.NextMsg:
		cmd := m.skipNext()
		return m, cmd

	case playback.PrevMsg:
		cmd := m.skipPrev()
		return m, cmd

	case playback.SeekMsg:
		cmd := m.seekRelative(msg.Offset, 0)
		return m, cmd

	case playback.SetPositionMsg:
		cmd := m.seekAbsolute(msg.Position)
		return m, cmd

	case playback.SetVolumeMsg:
		m.setVolume(msg.VolumeDB)
		return m, nil

	case playback.SetSpeedMsg:
		m.setSpeed(msg.Ratio)
		return m, nil

	case playback.ToggleMonoMsg:
		m.player.ToggleMono()
		return m, nil

	case playback.StopMsg:
		m.stopByUser()
		return m, nil

	case playback.QuitMsg:
		// Media controls and the signals of headless mode quit like the q
		// key, so the resume position is kept too.
		cmd := m.quit()
		return m, cmd

	case SetEQPresetMsg:
		m.SetEQPreset(msg.Name, msg.Bands)
		m.scheduleEQSave()
		return m, nil

	case SetEQBandMsg:
		m.setCustomEQBand(msg.Band, msg.Gain)
		return m, nil

	case PluginQueueMsg:
		cmd := m.handlePluginQueue(msg)
		return m, cmd

	case pluginQueueAddedMsg:
		cmd := m.appendPluginTracks(msg.tracks...)
		return m, cmd

	case trackFavoriteSyncedMsg:
		m.handleTrackFavoriteSynced(msg)
		return m, nil

	case ShowStatusMsg:
		ttl := statusTTLDefault
		if msg.Duration > 0 {
			ttl = statusTTL(msg.Duration)
		}
		m.status.Show(msg.Text, ttl)
		return m, nil

	case ipcProviderLoadResult:
		cmd := m.handleIPCProviderLoad(msg)
		return m, cmd

	case ipcPlaylistRenamedMsg:
		m.handleIPCPlaylistRenamed(msg)
		return m, nil

	case ipcHistoryClearedMsg:
		cmd := m.handleIPCHistoryCleared(msg)
		return m, cmd

	case ipcFeedLoadResult:
		cmd := m.handleIPCFeedLoad(msg)
		return m, cmd

	case ipcURLLoadResult:
		cmd := m.handleIPCURLResult(msg)
		return m, cmd

	case V2RequestMsg:
		cmd := m.handleV2Request(msg)
		return m, cmd

	case ipcV2ResponseMsg:
		m.handleV2Response(msg)
		return m, nil

	}

	return m, nil
}

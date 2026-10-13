package model

import (
	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
)

// Keys of the provider pane and of the source pill, and the scrolling of the
// provider list.

func (m *Model) providerScrollStep() int {
	return max(1, m.effectivePlaylistVisible())
}

func (m *Model) providerMaybeAdjustScroll() {
	visible := m.providerScrollStep()
	total := len(m.provPane.lists)
	if total == 0 {
		m.provPane.scroll = 0
		return
	}

	if m.provPane.cursor < m.provPane.scroll {
		m.provPane.scroll = m.provPane.cursor
	}

	if m.provPane.scroll >= total {
		m.provPane.scroll = max(0, total-1)
	}

	// Provider lists can add radio-prefix or PlaylistInfo.Section headers. Keep
	// the logical cursor visible in their rendered-row viewport.
	for m.provPane.scroll < total && m.providerRowsFromScroll(m.provPane.scroll, m.provPane.cursor) > visible {
		m.provPane.scroll++
	}
}

func (m *Model) providerRowsFromScroll(scroll, cursor int) int {
	total := len(m.provPane.lists)
	if total == 0 || cursor < scroll || scroll < 0 || cursor >= total {
		return 0
	}

	rows := 0
	sl, isRadio := m.provider.(provider.SectionedList)
	// Must resolve the same heading the renderer does, or the two disagree on
	// how many rows a window holds and the cursor scrolls out of view.
	headerAt := func(i int) string {
		if isRadio {
			return m.providerSectionTitle(sl.IDPrefix(m.provPane.lists[i].ID))
		}
		return m.provPane.lists[i].Section
	}

	prevHeader := ""
	if scroll > 0 {
		prevHeader = headerAt(scroll - 1)
	}

	for i := scroll; i <= cursor && i < total; i++ {
		header := headerAt(i)
		if header != "" && header != prevHeader {
			rows++ // section header row
		}
		rows++ // item row
		prevHeader = header
	}
	return rows
}

func (m *Model) providerMoveUp() {
	if m.provPane.cursor > 0 {
		m.provPane.cursor--
	} else if len(m.provPane.lists) > 0 {
		m.provPane.cursor = len(m.provPane.lists) - 1
	}
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerMoveDown() {
	if m.provPane.cursor < len(m.provPane.lists)-1 {
		m.provPane.cursor++
	} else if len(m.provPane.lists) > 0 {
		m.provPane.cursor = 0
	}
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerPageUp() {
	step := m.providerScrollStep()
	if m.provPane.cursor > 0 {
		m.provPane.cursor -= min(m.provPane.cursor, step)
	}
	// Top-anchor behavior: place cursor at top of viewport when paging up.
	m.provPane.scroll = m.provPane.cursor
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerPageDown() {
	step := m.providerScrollStep()
	if m.provPane.cursor < len(m.provPane.lists)-1 {
		m.provPane.cursor = min(len(m.provPane.lists)-1, m.provPane.cursor+step)
	}
	// Bottom-anchor behavior: bias viewport so cursor lands near bottom when paging down.
	m.provPane.scroll = max(0, m.provPane.cursor-step+1)
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerToTop() {
	m.provPane.cursor = 0
	m.providerMaybeAdjustScroll()
}

func (m *Model) providerToBottom() {
	if len(m.provPane.lists) > 0 {
		m.provPane.cursor = len(m.provPane.lists) - 1
	}
	m.providerMaybeAdjustScroll()
}

// handleProviderPaneKey processes a key press while the provider pane has
// the focus.
func (m *Model) handleProviderPaneKey(msg tea.KeyPressMsg) tea.Cmd {
	// The location question owns the keyboard until it is answered: it is
	// a yes/no about the listener's own data, so it must not be dismissed
	// by a stray key that happens to mean something else in this pane.
	if m.provPane.askLoc {
		switch msg.String() {
		case "y", "Y", "enter":
			return m.answerLocationPrompt(true)
		case "n", "N", "esc":
			return m.answerLocationPrompt(false)
		default:
			m.status.Show("Answer y for yes, n for no.", statusTTLShort)
		}
		return nil
	}

	if cmd, ok := m.providerShortcut(msg.String()); ok {
		return cmd
	}
	switch msg.String() {
	case "q":
		return m.quit()
	case "E":
		m.openEQPanel()
	case "R":
		m.toggleAutoRadio()
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
	case "F":
		m.openSubsOverlay()
	case "l":
		return m.loadLatestFromProviderList()
	case "a":
		return m.appendShowFromProviderList()
	case "p":
		if m.localProvider != nil {
			m.openPlaylistManager()
		}
	case "up", "k":
		m.providerMoveUp()
	case "space":
		return m.togglePlayPause()
	case "down", "j":
		m.providerMoveDown()
		// Auto-load next catalog page when scrolling near the bottom.
		return m.maybeLoadCatalogBatch()
	case "enter":
		if m.provPane.signIn {
			if auth, ok := m.provider.(playlist.Authenticator); ok {
				m.provPane.signIn = false
				m.provPane.loading = true
				m.err = nil
				return authenticateProviderCmd(auth, m.provider.Name(), nextRequest(&m.requests.auth))
			}
		}
		if len(m.provPane.lists) > 0 && !m.provPane.loading {
			return m.openProviderList(m.provPane.cursor)
		}
	case "tab", "shift+tab":
		// Leave the content-first provider layout before choosing a control;
		// the playback pane may be closed or too short to show every setting.
		m.focus = focusPlaylist
		m.recomputeLayout()
		if msg.String() == "shift+tab" {
			m.focus = m.previousMainFocus(focusPlaylist)
		} else {
			m.focus = m.nextMainFocus(focusPlaylist)
		}
	case "esc", "backspace", "b":
		// Clear completed results or cancel a search still in flight.
		if m.providerCatalogSearching() {
			return m.restoreCatalog(m.provider.(provider.CatalogSearcher))
		}
		// Leave even with an empty playlist. Starting with nothing to play
		// is what opens this view (StartInProvider), so gating the way out
		// on a loaded playlist made the launch screen inescapable.
		m.focus = focusPlaylist
	case "/":
		m.provSearch.active = true
		m.provSearch.query = ""
		m.provSearch.results = nil
		m.provSearch.cursor = 0
		m.provSearch.scroll = 0
	case "ctrl+r", "f5":
		return m.refreshActiveProvider(false)
	case "I":
		return m.rateInterest(taste.Interested)
	case "H":
		return m.rateInterest(taste.NotInterested)
	case "f":
		return m.toggleProviderFavorite()
	case "o":
		m.openFileBrowser()
	case "N":
		// Provider-pane browsing must stay scoped to the provider being
		// viewed. Falling back to another registered browser can otherwise
		// send (for example) Spotify's pane into Mixcloud.
		if providerSupportsBrowse(m.provider) {
			m.openNavBrowserWith(m.provider)
		}
	case "pgup", "ctrl+u":
		m.providerPageUp()
	case "pgdown", "ctrl+d":
		m.providerPageDown()
		return m.maybeLoadCatalogBatch()
	case "g", "home":
		m.providerToTop()
	case "G", "end":
		m.providerToBottom()
		return m.maybeLoadCatalogBatch()
	case "ctrl+j":
		m.openJumpMode()
	case "ctrl+x":
		m.toggleExpandedView()
	case "ctrl+f":
		m.openProviderSearch()
	}
	return nil
}

// handleProvPillKey processes a key press while the source pills have the
// focus.
func (m *Model) handleProvPillKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		return m.quit()
	case "left", "h":
		if m.provPillIdx > 0 {
			m.provPillIdx--
		}
	case "right", "l":
		if m.provPillIdx < len(m.providers)-1 {
			m.provPillIdx++
		}
	case "enter":
		return m.switchProvider(m.provPillIdx)
	case "tab":
		m.focus = m.nextMainFocus(focusProvPill)
	case "shift+tab", "esc", "backspace":
		m.focus = m.previousMainFocus(focusProvPill)
	case "space":
		return m.togglePlayPause()
	}
	return nil
}

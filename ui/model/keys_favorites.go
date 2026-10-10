package model

import (
	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
)

// maybeLoadCatalogBatch triggers a catalog batch fetch when the cursor is near the
// bottom of the provider list and more entries are available.
func (m *Model) maybeLoadCatalogBatch() tea.Cmd {
	loader, ok := m.provider.(provider.CatalogLoader)
	if !ok {
		return nil
	}
	if m.catalogBatch.loading || m.catalogBatch.done || m.provSearch.active || m.provSearch.loading {
		return nil
	}
	if cs, ok := m.provider.(provider.CatalogSearcher); ok && cs.IsSearching() {
		return nil
	}
	if m.provPane.cursor >= len(m.provPane.lists)-10 {
		m.catalogBatch.loading = true
		return m.fetchCatalogBatch(loader)
	}
	return nil
}

// answerLocationPrompt records the listener's answer to the location question
// and refreshes the pane, where the offer row is replaced by their country.
func (m *Model) answerLocationPrompt(allowed bool) tea.Cmd {
	m.provPane.askLoc = false
	consenter, ok := m.provider.(provider.LocationConsenter)
	if !ok {
		return nil
	}

	place, err := consenter.SetLocationConsent(allowed)
	if err != nil {
		// The answer holds for this run even when it could not be written.
		m.status.Errorf(statusTTLDefault, "Could not save the choice: %s", err)
	}
	switch {
	case !allowed:
		m.status.Show("Location off. Pin countries with f in Countries.", statusTTLLong)
	case place == "":
		m.status.Warning("Could not tell which country you are in. Pin countries with f in Countries.", statusTTLLong)
	default:
		m.status.Showf(statusTTLMedium, "Nearby radio: %s", place)
	}

	// The offer row is gone and, on a yes, a country row has taken its place.
	m.provPane.loading = true
	return m.fetchProviderPlaylists()
}

// toggleProviderFavorite toggles favorite status for the current entry in the
// provider list when the provider supports it.
func (m *Model) toggleProviderFavorite() tea.Cmd {
	if m.provPane.loading || m.provPane.cursor < 0 || m.provPane.cursor >= len(m.provPane.lists) || m.selectedProviderListIsBrowseEntry() {
		return nil
	}
	id := m.provPane.lists[m.provPane.cursor].ID
	if sl, ok := m.provider.(provider.SectionedList); ok {
		if !sl.IsFavoritableID(id) {
			return nil
		}
	}
	if !m.toggleFavorite(m.provider, id) {
		return nil
	}

	m.refreshProviderListsAfterMutation()
	return nil
}

// toggleFavorite shares persistence feedback without decorating item metadata.
func (m *Model) toggleFavorite(prov playlist.Provider, id string) bool {
	ok, _ := m.toggleFavoriteState(prov, id)
	return ok
}

// toggleFavoriteState toggles a provider favorite and reports whether it
// worked and whether the item is now a favorite.
func (m *Model) toggleFavoriteState(prov playlist.Provider, id string) (ok, added bool) {
	ft, isToggler := prov.(provider.FavoriteToggler)
	if !isToggler || id == "" {
		return false, false
	}
	added, name, err := ft.ToggleFavorite(id)
	if err != nil {
		m.status.Errorf(statusTTLDefault, "Favorite save failed: %s", err)
		return false, false
	}
	if added {
		m.status.Showf(statusTTLMedium, "Favorited: %s", name)
	} else {
		m.status.Showf(statusTTLMedium, "Removed: %s", name)
	}
	return true, added
}

// playlistStarAction keeps the key handler and its help scoped to the same
// selection.
type playlistStarAction uint8

const (
	starUnavailable playlistStarAction = iota
	starFavorite
)

func (m Model) selectedPlaylistStarAction() playlistStarAction {
	if m.playlist == nil || m.focus != focusPlaylist || m.plCursor < 0 || m.plCursor >= m.playlist.Len() {
		return starUnavailable
	}
	if m.favStore != nil {
		return starFavorite
	}
	return starUnavailable
}

// savedPlaylistRow reports whether track is a row of a loaded saved playlist.
// It matches rows by path. IPC clients send a track that can come from the
// queue or from a provider list, and only a queue row uses the saved rule.
func (m Model) savedPlaylistRow(track playlist.Track) bool {
	if m.loadedPlaylist == "" || m.playlist == nil {
		return false
	}
	for _, row := range m.playlist.Tracks() {
		if row.Path == track.Path {
			return true
		}
	}
	return false
}

func (m *Model) togglePlaylistStar() tea.Cmd {
	if m.selectedPlaylistStarAction() == starUnavailable {
		return nil
	}
	track, _ := m.playlist.Track(m.plCursor)
	cmd, err := m.togglePlaylistTrackFavorite(track, m.loadedPlaylist != "")
	if err != nil {
		m.status.Errorf(statusTTLDefault, "Favorite failed: %s", err)
		return nil
	}
	return cmd
}

// togglePlaylistTrackFavorite toggles the ♥ of track in the favorites store.
// The f key and IPC playlist.bookmark use it.
func (m *Model) togglePlaylistTrackFavorite(track playlist.Track, _ bool) (tea.Cmd, error) {
	return m.toggleTrackFavorite(track)
}

// playlistTrackFavorited reports whether a playback row shows the ♥ marker.
// The render path calls it, so it must not do I/O.
func (m Model) playlistTrackFavorited(track playlist.Track) bool {
	return trackFavorited(track, m.favSet)
}

// trackFavoriteLookup returns the ♥ rule as a function that is safe to call
// from a tea.Cmd. It captures the current favSet, which refreshFavSet
// replaces and never changes in place.
func (m Model) trackFavoriteLookup(bool) func(playlist.Track) bool {
	favSet := m.favSet
	return func(track playlist.Track) bool {
		return trackFavorited(track, favSet)
	}
}

// trackFavorited reports the ♥ state of track in the favorites store.
func trackFavorited(track playlist.Track, favSet map[string]struct{}) bool {
	_, ok := favSet[track.Path]
	return ok
}

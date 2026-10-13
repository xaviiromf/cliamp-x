package model

import (
	"context"
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/taste"
)

// "I'm interested" (I) and "not interested" (H) on the highlighted song,
// album, artist, genre or list. The rating goes to the taste store, which
// the providers read to rank and filter what they recommend. On a song, I is
// also the ♥ favorite (a like on the service) and H a dislike. H takes the
// row out of sight: it leaves the list, and a playing song is skipped.

// SetTaste shares the taste store with the Model. Nil turns the ratings off.
func (m *Model) SetTaste(s *taste.Store) { m.taste = s }

// interestTarget is what the rating keys act on.
type interestTarget struct {
	kind  taste.Kind
	id    string
	name  string
	track playlist.Track // for a song
	// hide takes the row out of the list on screen after "not interested".
	hide func(*Model) tea.Cmd
}

// currentInterestTarget returns the highlighted row of the screen with the
// keys.
func (m *Model) currentInterestTarget() (interestTarget, bool) {
	if m.navBrowser.visible && !m.navBrowser.searching {
		return m.navInterestTarget()
	}
	switch m.focus {
	case focusProvider:
		i := m.provPane.cursor
		if i < 0 || i >= len(m.provPane.lists) {
			return interestTarget{}, false
		}
		row := m.provPane.lists[i]
		// Only a row that is one album, artist or genre can be rated, never
		// a menu such as "Genres" or a playlist.
		rk, _ := m.provider.(provider.RatingKinder)
		if rk == nil || rk.RatingKind(row.ID) == "" {
			return interestTarget{}, false
		}
		return interestTarget{kind: taste.Kind(rk.RatingKind(row.ID)), id: row.ID, name: trimHeart(row.Name), hide: func(m *Model) tea.Cmd {
			m.provPane.lists = slices.Delete(m.provPane.lists, i, i+1)
			m.provPane.cursor = min(m.provPane.cursor, max(0, len(m.provPane.lists)-1))
			m.providerMaybeAdjustScroll()
			return nil
		}}, true
	case focusPlaylist:
		idx := m.plCursor
		t, ok := m.playlist.Track(idx)
		if !ok {
			return interestTarget{}, false
		}
		return songTarget(t, func(m *Model) tea.Cmd {
			if idx == m.playlist.Index() && m.player.IsPlaying() {
				return m.skipNext()
			}
			cmd, _ := m.removeTrack(idx, true)
			return cmd
		}), true
	}
	return interestTarget{}, false
}

func songTarget(t playlist.Track, hide func(*Model) tea.Cmd) interestTarget {
	return interestTarget{kind: taste.Song, id: t.Path, name: t.DisplayName(), track: t, hide: hide}
}

// navInterestTarget returns the highlighted row of the browser.
func (m *Model) navInterestTarget() (interestTarget, bool) {
	nb := &m.navBrowser
	// drop removes raw index i of a list and keeps the cursor in range.
	drop := func(i int, remove func(int)) func(*Model) tea.Cmd {
		return func(m *Model) tea.Cmd {
			remove(i)
			m.navClearSearchKeepingCursor(min(i, max(0, m.navListLen()-1)))
			m.navMaybeAdjustScroll()
			return nil
		}
	}
	switch {
	case nb.screen == navBrowseScreenTracks:
		i := m.selectedNavRawIndex(len(nb.tracks))
		if i < 0 {
			return interestTarget{}, false
		}
		return songTarget(nb.tracks[i], drop(i, func(i int) { nb.tracks = slices.Delete(nb.tracks, i, i+1) })), true
	case nb.mode == navBrowseModeByGenre && nb.screen == navBrowseScreenList:
		i := m.selectedNavRawIndex(len(nb.genres))
		if i < 0 {
			return interestTarget{}, false
		}
		g := nb.genres[i]
		return interestTarget{kind: taste.Genre, id: g.ID, name: g.Name, hide: drop(i, func(i int) { nb.genres = slices.Delete(nb.genres, i, i+1) })}, true
	case (nb.mode == navBrowseModeByArtist || nb.mode == navBrowseModeByArtistAlbum) && nb.screen == navBrowseScreenList:
		i := m.selectedNavRawIndex(len(nb.artists))
		if i < 0 {
			return interestTarget{}, false
		}
		a := nb.artists[i]
		return interestTarget{kind: taste.Artist, id: a.ID, name: trimHeart(a.Name), hide: drop(i, func(i int) { nb.artists = slices.Delete(nb.artists, i, i+1) })}, true
	case nb.mode == navBrowseModeByAlbum && nb.screen == navBrowseScreenList,
		nb.mode == navBrowseModeByArtistAlbum && nb.screen == navBrowseScreenAlbums:
		i := m.selectedNavRawIndex(len(nb.albums))
		if i < 0 {
			return interestTarget{}, false
		}
		a := nb.albums[i]
		return interestTarget{kind: taste.Album, id: a.ID, name: trimHeart(a.Name), hide: drop(i, func(i int) { nb.albums = slices.Delete(nb.albums, i, i+1) })}, true
	}
	return interestTarget{}, false
}

// navListLen is the length of the raw list on the browser screen.
func (m *Model) navListLen() int {
	nb := &m.navBrowser
	switch {
	case nb.screen == navBrowseScreenTracks:
		return len(nb.tracks)
	case nb.mode == navBrowseModeByGenre && nb.screen == navBrowseScreenList:
		return len(nb.genres)
	case nb.screen == navBrowseScreenList && nb.mode != navBrowseModeByAlbum:
		return len(nb.artists)
	}
	return len(nb.albums)
}

func trimHeart(name string) string { return setHeart(name, false) }

// rateInterest marks the highlighted row: taste.Interested or
// taste.NotInterested.
func (m *Model) rateInterest(interest int) tea.Cmd {
	target, ok := m.currentInterestTarget()
	if !ok {
		m.status.Show(L("Only a song, album, artist or genre can be rated"), statusTTLShort)
		return nil
	}
	return m.applyInterest(target, interest)
}

// applyInterest stores a rating and does what it means on screen.
func (m *Model) applyInterest(target interestTarget, interest int) tea.Cmd {
	if m.taste == nil {
		m.status.Warning(L("Ratings are not available"), statusTTLDefault)
		return nil
	}
	item := taste.Item{Kind: target.kind, Name: target.name, Artist: target.track.Artist}
	if _, err := m.taste.Rate(target.id, item, interest); err != nil {
		m.status.Errorf(statusTTLDefault, "%s: %s", L("Rating not saved"), err)
		return nil
	}
	var cmds []tea.Cmd
	if interest > 0 {
		// On a song, interest is a like: the ♥ favorite copies it to the
		// service.
		if target.kind == taste.Song && !m.playlistTrackFavorited(target.track) {
			if cmd, err := m.toggleTrackFavorite(target.track); err == nil {
				cmds = append(cmds, cmd)
			}
		}
		m.status.Successf(statusTTLMedium, "%s: %s", L("Interested — you will get more like it"), target.name)
		return tea.Batch(cmds...)
	}
	defer m.status.Showf(statusTTLMedium, "%s: %s", L("Not interested — hidden from now on"), target.name)
	if target.kind == taste.Song {
		if m.playlistTrackFavorited(target.track) {
			if cmd, err := m.toggleTrackFavorite(target.track); err == nil {
				cmds = append(cmds, cmd)
			}
		}
		cmds = append(cmds, m.dislikeTrackCmd(target.track))
	}
	if target.hide != nil {
		cmds = append(cmds, target.hide(m))
	}
	return tea.Batch(cmds...)
}

// dislikeTrackCmd sends a dislike to the provider that owns the song, in the
// favorite sync order, so it lands after the un-like.
func (m *Model) dislikeTrackCmd(track playlist.Track) tea.Cmd {
	fav, name := m.findTrackFavoriter(track)
	disliker, ok := fav.(provider.TrackDisliker)
	if !ok {
		return nil
	}
	return m.queueTrackSync(track, name, func(ctx context.Context) error { return disliker.DislikeTrack(ctx, track) })
}

// visibleTracks drops the songs the taste store hides, for the lists that
// recommendations fill (auto radio, more like this).
func (m *Model) visibleTracks(tracks []playlist.Track) []playlist.Track {
	if m.taste == nil {
		return tracks
	}
	return slices.DeleteFunc(tracks, func(t playlist.Track) bool {
		return m.taste.Hidden([]string{t.Path}, t.Title, t.Artist, t.Album, t.Genre)
	})
}

// refreshBrowser reloads the browser screen, after the provider dropped its
// caches, so an endless list starts over with new results (F5, Ctrl+R).
func (m *Model) refreshBrowser() tea.Cmd {
	nb := &m.navBrowser
	if nb.prov == nil {
		return nil
	}
	if r, ok := nb.prov.(playlist.Refresher); ok {
		r.Refresh()
	}
	m.navClearSearch()
	m.status.Activityf(statusTTLShort, "%s", L("Refreshing…"))
	gen := m.nextNavRequest()
	switch {
	case nb.screen == navBrowseScreenTracks && nb.mode == navBrowseModeByGenre:
		if b := m.navGenreBrowser(); b != nil {
			nb.loading = true
			return fetchNavGenreTracksCmd(b, nb.selGenre.ID, nb.selGenreSort.ID, gen)
		}
	case nb.screen == navBrowseScreenTracks:
		if l, ok := nb.prov.(provider.AlbumTrackLoader); ok && nb.selAlbum.ID != "" {
			nb.loading = true
			return fetchNavAlbumTracksCmd(l, nb.selAlbum.ID, gen)
		}
	case nb.mode == navBrowseModeByGenre && nb.screen == navBrowseScreenList:
		if b := m.navGenreBrowser(); b != nil {
			nb.loading = true
			return fetchNavGenresCmd(b, gen)
		}
	case nb.mode == navBrowseModeByArtistAlbum && nb.screen == navBrowseScreenAlbums:
		if ab, ok := nb.prov.(provider.ArtistBrowser); ok {
			nb.loading = true
			return fetchNavArtistAlbumsCmd(ab, nb.selArtist.ID, gen)
		}
	case nb.screen == navBrowseScreenList && nb.mode == navBrowseModeByAlbum:
		if ab, ok := nb.prov.(provider.AlbumBrowser); ok {
			nb.albums, nb.albumDone, nb.albumLoading = nil, false, true
			return fetchNavAlbumListCmd(ab, nb.sortType, 0, gen)
		}
	case nb.screen == navBrowseScreenList:
		if ab, ok := nb.prov.(provider.ArtistBrowser); ok {
			nb.loading, nb.albumDone = true, false
			return fetchNavArtistsCmd(ab, 0, gen)
		}
	}
	return nil
}

// hideTrackArtist rates the artist of a song "not interested" and skips the
// song when it plays.
func (m *Model) hideTrackArtist(track playlist.Track) tea.Cmd {
	target := interestTarget{kind: taste.Artist, id: "artist:" + track.Artist, name: track.Artist}
	if r, _ := findCapable(m, func(provider.TrackArtistResolver) bool { return true }); r != nil {
		if a, ok := r.ArtistForTrack(track); ok && a.ID != "" {
			target.id, target.name = a.ID, trimHeart(a.Name)
		}
	}
	if cur, idx := m.currentPlaybackTrack(); idx >= 0 && cur.Path == track.Path && m.player.IsPlaying() {
		target.hide = (*Model).skipNext
	}
	return m.applyInterest(target, taste.NotInterested)
}

// ratingKindLabels name the rated kinds in the ratings menu.
var ratingKindLabels = map[taste.Kind]string{
	taste.Song: "Song", taste.Album: "Album", taste.Artist: "Artist", taste.Genre: "Genre",
}

// openRatingsMenu lists what was rated "not interested" and "interested" (P).
// Choosing a row takes its rating back, so a mistaken H can be undone.
func (m *Model) openRatingsMenu() {
	ratings := m.taste.Ratings()
	if len(ratings) == 0 {
		m.status.Show(L("Nothing rated yet: I and H rate the highlighted song, album, artist or genre"), statusTTLMedium)
		return
	}
	var entries []keymapEntry
	group := 0
	for _, r := range ratings {
		if r.Interest != group {
			group = r.Interest
			title := L("Not interested")
			if group > 0 {
				title = L("Interested")
			}
			entries = append(entries, keymapEntry{action: "— " + title + " —", divider: true})
		}
		entries = append(entries, keymapEntry{key: L(ratingKindLabels[r.Item.Kind]), action: r.Item.Name, do: func(m *Model) tea.Cmd {
			if _, err := m.taste.Rate(r.ID, r.Item, taste.Neutral); err != nil {
				m.status.Errorf(statusTTLDefault, "%s: %s", L("Rating not saved"), err)
				return nil
			}
			m.openRatingsMenu()
			m.status.Successf(statusTTLMedium, "%s: %s", L("Rating undone"), r.Item.Name)
			// The pane shows what was hidden again; the browser on F5.
			if m.provider != nil && !m.provPane.loading {
				return m.fetchProviderPlaylists()
			}
			return nil
		}})
	}
	m.showKeymap(entries, L("Your ratings: Enter undoes one"))
}

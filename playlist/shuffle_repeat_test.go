package playlist

import "testing"

func TestToggleShuffle(t *testing.T) {
	p := makePlaylist(5, false)
	p.SetIndex(2) // C

	if p.Shuffled() {
		t.Fatal("Shuffled() initially should be false")
	}

	p.ToggleShuffle()
	if !p.Shuffled() {
		t.Fatal("Shuffled() after toggle should be true")
	}

	// Current track should still be the same
	cur, _ := p.Current()
	if cur.Title != "C" {
		t.Fatalf("Current after shuffle = %q, want C", cur.Title)
	}

	// Position should be 0 in shuffled order (current track moves to front)
	if p.pos != 0 {
		t.Fatalf("pos after shuffle = %d, want 0", p.pos)
	}
}

func TestToggleShuffleOff(t *testing.T) {
	p := makePlaylist(5, true) // start shuffled
	p.SetIndex(p.order[0])

	curTrack, curIdx := p.Current()

	p.ToggleShuffle() // turn off

	if p.Shuffled() {
		t.Fatal("Shuffled() after toggle off should be false")
	}

	// Order should be sequential again
	for i, idx := range p.order {
		if idx != i {
			t.Fatalf("order[%d] = %d, want %d", i, idx, i)
		}
	}

	// Position should track the same track
	cur2, _ := p.Current()
	if cur2.Title != curTrack.Title {
		t.Fatalf("Current after unshuffle = %q, want %q", cur2.Title, curTrack.Title)
	}

	// pos should equal the original track index
	if p.pos != curIdx {
		t.Fatalf("pos after unshuffle = %d, want %d", p.pos, curIdx)
	}
}

func TestToggleShuffleEmpty(t *testing.T) {
	p := New()
	p.ToggleShuffle() // should not panic
	if !p.Shuffled() {
		t.Fatal("Shuffled() should be true even when empty")
	}
}

func TestCycleRepeat(t *testing.T) {
	p := New()

	if p.Repeat() != RepeatOff {
		t.Fatalf("initial repeat = %v, want Off", p.Repeat())
	}

	p.CycleRepeat()
	if p.Repeat() != RepeatAll {
		t.Fatalf("after 1 cycle = %v, want All", p.Repeat())
	}

	p.CycleRepeat()
	if p.Repeat() != RepeatOne {
		t.Fatalf("after 2 cycles = %v, want One", p.Repeat())
	}

	p.CycleRepeat()
	if p.Repeat() != RepeatOff {
		t.Fatalf("after 3 cycles = %v, want Off", p.Repeat())
	}
}

func TestSetRepeat(t *testing.T) {
	p := New()

	p.SetRepeat(RepeatOne)
	if p.Repeat() != RepeatOne {
		t.Fatalf("Repeat() = %v, want One", p.Repeat())
	}

	p.SetRepeat(RepeatAll)
	if p.Repeat() != RepeatAll {
		t.Fatalf("Repeat() = %v, want All", p.Repeat())
	}
}

func TestShufflePreservesAllTracks(t *testing.T) {
	p := makePlaylist(10, false)
	p.ToggleShuffle()

	// All track indices should appear exactly once in the order
	seen := make(map[int]bool)
	for _, idx := range p.order {
		if seen[idx] {
			t.Fatalf("duplicate index %d in shuffle order", idx)
		}
		seen[idx] = true
	}
	if len(seen) != 10 {
		t.Fatalf("shuffle order has %d entries, want 10", len(seen))
	}
}

func TestSetTrack(t *testing.T) {
	p := makePlaylist(3, false)

	p.SetTrack(1, Track{Title: "NEW"})

	tracks := p.Tracks()
	if tracks[1].Title != "NEW" {
		t.Fatalf("tracks[1].Title = %q, want NEW", tracks[1].Title)
	}
}

func TestSetTrackOutOfBounds(t *testing.T) {
	p := makePlaylist(3, false)

	// Should be no-op, not panic
	p.SetTrack(-1, Track{Title: "X"})
	p.SetTrack(5, Track{Title: "X"})

	if p.Tracks()[0].Title != "A" {
		t.Fatal("tracks were modified by out-of-bounds SetTrack")
	}
}

func TestToggleFavorite(t *testing.T) {
	p := makePlaylist(3, false)

	p.ToggleFavorite(0)
	if !p.Tracks()[0].Favorite {
		t.Fatal("track 0 should be favorited")
	}
	if p.FavoriteCount() != 1 {
		t.Fatalf("FavoriteCount() = %d, want 1", p.FavoriteCount())
	}

	p.ToggleFavorite(0) // toggle off
	if p.Tracks()[0].Favorite {
		t.Fatal("track 0 should be unfavorited")
	}
	if p.FavoriteCount() != 0 {
		t.Fatalf("FavoriteCount() = %d, want 0", p.FavoriteCount())
	}
}

func TestToggleFavoriteOutOfBounds(t *testing.T) {
	p := makePlaylist(3, false)

	// Should be no-op, not panic
	p.ToggleFavorite(-1)
	p.ToggleFavorite(5)

	if p.FavoriteCount() != 0 {
		t.Fatal("favorites were modified by out-of-bounds ToggleFavorite")
	}
}

func TestFavoriteCount(t *testing.T) {
	p := makePlaylist(5, false)

	p.ToggleFavorite(0)
	p.ToggleFavorite(2)
	p.ToggleFavorite(4)

	if got := p.FavoriteCount(); got != 3 {
		t.Fatalf("FavoriteCount() = %d, want 3", got)
	}
}

package ytmusic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/fileutil"
)

// snapshotFile keeps the browse lists between runs, so the provider pane
// fills at once at start. The lists are refreshed in the background on their
// first read, so they are never older than one run.
const snapshotFile = "browse.json"

// browseSnapshot is the content of snapshotFile.
type browseSnapshot struct {
	// SignedIn is the sign-in state the lists belong to. A snapshot of the
	// other state is ignored, so signing out never shows a library.
	SignedIn  bool
	Home      []Shelf
	HomeAt    time.Time
	Library   []Item
	LibraryAt time.Time
	Cats      []Category
	CatsAt    time.Time
}

func (p *Provider) snapshotPath() string {
	if p.opts.DataDir == "" {
		return ""
	}
	return filepath.Join(p.opts.DataDir, snapshotFile)
}

// loadSnapshotLocked fills the empty lists from the snapshot, once. The
// caller holds p.mu.
func (p *Provider) loadSnapshotLocked() {
	if p.snapLoaded {
		return
	}
	p.snapLoaded = true
	path := p.snapshotPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var snap browseSnapshot
	if json.Unmarshal(data, &snap) != nil || snap.SignedIn != p.client.SignedIn() {
		return
	}
	if p.home == nil && snap.Home != nil {
		p.home, p.homeAt, p.homeDisk = snap.Home, snap.HomeAt, true
	}
	if p.library == nil && snap.Library != nil {
		p.library, p.libraryAt, p.libraryDisk = snap.Library, snap.LibraryAt, true
	}
	if p.cats == nil && snap.Cats != nil {
		p.cats, p.catsAt, p.catsDisk = snap.Cats, snap.CatsAt, true
	}
}

// saveSnapshotLocked writes the lists in the background, so the encoding
// and the synced write never hold p.mu. A failed write only costs a slower
// start. The caller holds p.mu.
func (p *Provider) saveSnapshotLocked() {
	path := p.snapshotPath()
	if path == "" {
		return
	}
	p.snapSeq++
	go p.writeSnapshot(path, p.snapSeq, browseSnapshot{
		SignedIn: p.client.SignedIn(),
		Home:     p.home, HomeAt: p.homeAt,
		Library: p.library, LibraryAt: p.libraryAt,
		Cats: p.cats, CatsAt: p.catsAt,
	})
}

// writeSnapshot writes snap unless a newer one was written already.
func (p *Provider) writeSnapshot(path string, seq uint64, snap browseSnapshot) {
	p.snapWriteMu.Lock()
	defer p.snapWriteMu.Unlock()
	if seq <= p.snapWritten {
		return
	}
	p.snapWritten = seq
	if data, err := json.Marshal(snap); err == nil {
		_ = fileutil.WriteFileAtomic(path, data, 0o600)
	}
}

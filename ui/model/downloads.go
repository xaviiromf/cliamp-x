package model

import (
	"context"
	"errors"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/xaviiromf/cliamp-x/internal/offline"
	"github.com/xaviiromf/cliamp-x/playlist"
	"github.com/xaviiromf/cliamp-x/provider"
	"github.com/xaviiromf/cliamp-x/tracksave"
)

// Offline downloads: D queues the highlighted song, album, playlist or
// artist for download. Songs download one at a time with yt-dlp; the status
// line shows the progress. A downloaded song plays from its file, also
// without a connection, and joins the local music menus.

// downloadState is the download queue.
type downloadState struct {
	manifest *offline.Manifest
	dir      string // the downloads directory, or "" for ~/Music/cliamp-x
	queue    []offline.Job
	queued   map[string]bool // paths queued or downloading
	running  bool
	current  offline.Job
	percent  float64
	done     int
	failed   int
	events   chan downloadEvent
	cancel   context.CancelFunc
	// failedJobs are the songs that failed, kept for a retry from the
	// downloads screen.
	failedJobs []failedDownload
	visible    bool // the downloads screen (Ctrl+O) is open
	scroll     int
}

// failedDownload is a song whose download failed, with the reason.
type failedDownload struct {
	job offline.Job
	err string
}

// downloadEvent is one step of the running download.
type downloadEvent struct {
	percent float64
	file    string
	err     error
	final   bool
}

type downloadEventMsg struct{ ev downloadEvent }

// downloadResolvedMsg carries the songs of a row queued for download.
type downloadResolvedMsg struct {
	label string
	jobs  []offline.Job
	err   error
}

// SetOffline turns downloads on: manifest records downloaded songs and dir
// is the downloads directory ("" for ~/Music/cliamp-x).
func (m *Model) SetOffline(manifest *offline.Manifest, dir string) {
	m.downloads.manifest = manifest
	m.downloads.dir = dir
	m.downloads.queued = map[string]bool{}
}

// isDownloaded reports whether a track has a downloaded copy.
func (m Model) isDownloaded(t playlist.Track) bool {
	return m.downloads.manifest != nil && m.downloads.manifest.Has(t.Path)
}

// downloadHighlighted queues the highlighted row for download.
func (m *Model) downloadHighlighted() tea.Cmd {
	if m.downloads.manifest == nil {
		return nil
	}
	seed, ok := m.discoverTarget()
	if !ok {
		m.status.Show(L("Highlight a song, album or playlist to download"), statusTTLMedium)
		return nil
	}
	if seed.itemID == "" {
		return m.queueDownloads(seed.label, []offline.Job{{Track: seed.track}})
	}
	prov := seed.prov
	if prov == nil {
		prov = m.provider
	}
	id, label := seed.itemID, seed.label
	m.status.Activityf(statusTTLLong, "%s %s…", L("Preparing the download of"), label)
	return func() tea.Msg {
		tracks, album, err := rowTracks(prov, id)
		if err != nil {
			return downloadResolvedMsg{label: label, err: err}
		}
		jobs := make([]offline.Job, 0, len(tracks))
		for i, t := range tracks {
			job := offline.Job{Track: t}
			if album {
				job.Album, job.TrackNo = t.Album, i+1
			}
			jobs = append(jobs, job)
		}
		return downloadResolvedMsg{label: label, jobs: jobs}
	}
}

// rowTracks loads the songs of a browse row, and reports whether the row is
// an album, whose songs get track numbers.
func rowTracks(prov playlist.Provider, id string) ([]playlist.Track, bool, error) {
	if loader, ok := prov.(provider.AlbumTrackLoader); ok && isAlbumID(id) {
		tracks, err := loader.AlbumTracks(strings.TrimPrefix(id, "al:"))
		return tracks, true, err
	}
	tracks, err := prov.Tracks(id)
	return tracks, false, err
}

func isAlbumID(id string) bool {
	return strings.HasPrefix(id, "MPREb_") || strings.HasPrefix(id, "al:MPREb_") || strings.HasPrefix(id, "local:al:")
}

func (m *Model) handleDownloadResolved(msg downloadResolvedMsg) tea.Cmd {
	if msg.err != nil {
		m.status.Warningf(statusTTLMedium, "%s: %s", L("Download"), msg.err)
		return nil
	}
	return m.queueDownloads(msg.label, msg.jobs)
}

// queueDownloads adds jobs that need downloading and starts the queue.
func (m *Model) queueDownloads(label string, jobs []offline.Job) tea.Cmd {
	added := 0
	for _, j := range jobs {
		p := j.Track.Path
		if !tracksave.NeedsDownload(j.Track) || m.downloads.queued[p] || m.isDownloaded(j.Track) {
			continue
		}
		m.downloads.queued[p] = true
		m.downloads.queue = append(m.downloads.queue, j)
		added++
	}
	if added == 0 {
		m.status.Show(L("Already downloaded (or not downloadable)"), statusTTLMedium)
		return nil
	}
	m.status.Activityf(statusTTLMedium, "↓ %s: %d %s", label, added, L("queued"))
	if m.downloads.running {
		return nil
	}
	return m.startNextDownload()
}

// startNextDownload starts the first queued job.
func (m *Model) startNextDownload() tea.Cmd {
	d := &m.downloads
	if len(d.queue) == 0 {
		d.running = false
		return nil
	}
	job := d.queue[0]
	d.queue = d.queue[1:]
	d.running, d.current, d.percent = true, job, 0
	dir, err := tracksave.Directory(d.dir)
	if err != nil {
		d.running = false
		m.status.Warningf(statusTTLMedium, "%s: %s", L("Download"), err)
		return nil
	}
	events := make(chan downloadEvent, 16)
	ctx, cancel := context.WithCancel(context.Background())
	d.events, d.cancel = events, cancel
	manifest := d.manifest
	go func() {
		defer cancel()
		file, err := offline.Download(ctx, job, dir, func(p float64) {
			select {
			case events <- downloadEvent{percent: p}:
			default: // the UI catches up with the next step
			}
		})
		if err == nil {
			err = manifest.Add(job.Track.Path, file)
		}
		events <- downloadEvent{file: file, err: err, final: true}
	}()
	m.showDownloadProgress()
	return waitDownloadEvent(events)
}

func waitDownloadEvent(ch chan downloadEvent) tea.Cmd {
	return func() tea.Msg { return downloadEventMsg{ev: <-ch} }
}

func (m *Model) showDownloadProgress() {
	d := m.downloads
	n := d.done + d.failed + 1
	m.status.Activityf(statusTTLLong, "↓ %d/%d · %s %.0f%%", n, n+len(d.queue), d.current.Track.DisplayName(), d.percent)
}

// handleDownloadEvent shows progress, and on a finished song starts the next.
func (m *Model) handleDownloadEvent(msg downloadEventMsg) tea.Cmd {
	d := &m.downloads
	if !msg.ev.final {
		d.percent = msg.ev.percent
		m.showDownloadProgress()
		return waitDownloadEvent(d.events)
	}
	delete(d.queued, d.current.Track.Path)
	switch {
	case errors.Is(msg.ev.err, context.Canceled):
		// Stopped by x on the downloads screen: not a failure.
	case msg.ev.err != nil:
		d.failed++
		d.failedJobs = append(d.failedJobs, failedDownload{job: d.current, err: msg.ev.err.Error()})
	default:
		d.done++
	}
	if len(d.queue) > 0 {
		return m.startNextDownload()
	}
	d.running = false
	switch {
	case d.done == 0 && d.failed == 0:
		// Canceled before a song finished: nothing to report.
	case d.failed > 0:
		m.status.Warningf(statusTTLLong, "↓ %d %s, %d %s: %s", d.done, L("downloaded"), d.failed, L("failed"), d.failedJobs[len(d.failedJobs)-1].err)
	default:
		dir, _ := tracksave.Directory(d.dir)
		m.status.Successf(statusTTLLong, "↓ %d %s %s", d.done, L("downloaded to"), dir)
	}
	d.done, d.failed = 0, 0
	// The local music menus pick the new files up on their next read.
	for _, pe := range m.providers {
		if r, ok := pe.Provider.(interface{ Refresh() }); ok {
			r.Refresh()
		}
	}
	return nil
}

// downloadLine is the preview pane's note for a downloaded song.
func downloadLine() string {
	return "↓ " + L("Downloaded: plays offline")
}

// cancelDownloads stops the running download and empties the queue.
func (m *Model) cancelDownloads() {
	d := &m.downloads
	if d.cancel != nil {
		d.cancel()
	}
	d.queue = nil
	clear(d.queued)
}

// retryFailedDownloads queues the failed songs again.
func (m *Model) retryFailedDownloads() tea.Cmd {
	d := &m.downloads
	if len(d.failedJobs) == 0 {
		m.status.Show(L("No failed downloads"), statusTTLShort)
		return nil
	}
	jobs := make([]offline.Job, len(d.failedJobs))
	for i, f := range d.failedJobs {
		jobs[i] = f.job
	}
	d.failedJobs = nil
	return m.queueDownloads(L("Retry"), jobs)
}

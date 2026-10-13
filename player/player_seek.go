package player

import (
	"fmt"
	"time"

	"github.com/gopxl/beep/v2/speaker"
)

// Seeking: in a decoder that can seek, by restarting yt-dlp at an offset for
// YouTube, and the position and duration the UI reads.

// Seek moves the playback position by the given duration (positive or negative).
// For seekable local files, the decoder's Seek method is used directly.
// Returns nil immediately for non-seekable streams (e.g., Icecast radio).
// FFmpeg replacements are prepared before the speaker lock is acquired. The
// lock only covers decoder commit and preload invalidation.
// Clears the preloaded next pipeline to prevent a stale gapless transition.
func (p *Player) Seek(d time.Duration) error {
	p.mu.Lock()
	cur := p.current
	p.mu.Unlock()
	if cur == nil {
		return nil
	}

	// yt-dlp seek-by-restart: handled outside the speaker lock via SeekYTDL.
	if cur.ytdlSeek {
		return p.SeekYTDL(d)
	}

	if seeker, ok := cur.decoder.(preparedFFmpegSeeker); ok {
		newSample := relativeSeekSample(cur, d)
		prepared, err := seeker.prepareSeek(newSample)
		if err != nil {
			return err
		}
		return p.commitPreparedSeek(cur, seeker, prepared)
	}

	// Other local decoders use their native Seek implementation.
	if !cur.seekable {
		return nil
	}

	p.lifecycleMu.Lock()
	speaker.Lock()
	p.mu.Lock()
	if p.current != cur {
		p.mu.Unlock()
		speaker.Unlock()
		p.lifecycleMu.Unlock()
		return nil
	}
	p.mu.Unlock()
	newSample := relativeSeekSample(cur, d)
	if err := cur.decoder.Seek(newSample); err != nil {
		speaker.Unlock()
		p.lifecycleMu.Unlock()
		return err
	}
	// Invalidate the preloaded next pipeline — the gapless transition point
	// has moved and the old preload may be stale. The speaker lock is already
	// held, so we can safely clear the gapless next stream.
	p.gapless.SetNext(nil)
	p.mu.Lock()
	old := p.nextPipeline
	p.nextPipeline = nil
	p.mu.Unlock()
	speaker.Unlock()
	p.lifecycleMu.Unlock()
	if old != nil {
		// Seek runs on the UI goroutine, and a close can wait for an ffmpeg
		// or yt-dlp process to exit, as in ClearPreload.
		go old.close()
	}
	return nil
}

type preparedFFmpegSeeker interface {
	prepareSeek(int) (*preparedFFmpegSeek, error)
	seekMatches(*preparedFFmpegSeek) bool
	commitPreparedSeek(*preparedFFmpegSeek) (ffmpegPipe, bool)
	interrupt()
}

func relativeSeekSample(cur *trackPipeline, d time.Duration) int {
	curDur := cur.format.SampleRate.D(cur.decoder.Position())
	newSample := max(cur.format.SampleRate.N(curDur+d), 0)
	if length := cur.decoder.Len(); length > 0 && newSample >= length {
		return length - 1
	}
	return newSample
}

func (p *Player) commitPreparedSeek(cur *trackPipeline, seeker preparedFFmpegSeeker, prepared *preparedFFmpegSeek) error {
	p.lifecycleMu.Lock()
	p.mu.Lock()
	current := p.current == cur
	p.mu.Unlock()
	if !current || !seeker.seekMatches(prepared) {
		p.lifecycleMu.Unlock()
		_ = prepared.close()
		return nil
	}

	// Prevent interruption-driven EOF from advancing to a stale preload while
	// the speaker finishes its active Stream call.
	p.gapless.SetNext(nil)
	seeker.interrupt()

	speaker.Lock()
	p.mu.Lock()
	current = p.current == cur && seeker.seekMatches(prepared)
	if !current {
		p.mu.Unlock()
		speaker.Unlock()
		p.lifecycleMu.Unlock()
		_ = prepared.close()
		return nil
	}

	oldPipe, _ := seeker.commitPreparedSeek(prepared)
	oldNext := p.nextPipeline
	p.nextPipeline = nil
	p.mu.Unlock()
	p.gapless.Replace(cur.stream)
	p.gaplessAdvance.Store(false)
	speaker.Unlock()
	p.lifecycleMu.Unlock()

	_ = oldPipe.stop()
	// A local track seeks on the UI goroutine, and the preload close can wait
	// for an ffmpeg or yt-dlp process to exit, as in ClearPreload.
	go closePipelines(oldNext)
	return nil
}

// CancelSeekYTDL increments the seek generation, causing any in-flight
// SeekYTDL to discard its result instead of swapping streams.
func (p *Player) CancelSeekYTDL() {
	p.seekGen.Add(1)
}

// SeekYTDL seeks a yt-dlp stream by restarting the pipeline at the target
// position. Must NOT be called with the speaker lock held.
// If a newer seek is requested (via CancelSeekYTDL) while this one is
// building, the result is discarded.
func (p *Player) SeekYTDL(d time.Duration) error {
	gen := p.seekGen.Load()

	// Snapshot current state without speaker lock.
	p.mu.Lock()
	cur := p.current
	p.mu.Unlock()
	if cur == nil || !cur.ytdlSeek {
		return nil
	}

	// Read position, then mute the current stream so the speaker outputs
	// silence while the new pipeline is being built (which blocks on Peek
	// waiting for yt-dlp data). Without this, the old audio keeps playing
	// at the pre-seek position during the rebuild.
	curPos, ok := p.muteYTDLSeekSource(cur, gen)
	if !ok {
		// Another track or a newer seek replaced cur after the snapshot.
		return nil
	}

	newPos := max(curPos+d, 0)
	if cur.knownDuration > 0 && newPos >= cur.knownDuration {
		newPos = cur.knownDuration - time.Second
	}
	startSec := int(newPos.Seconds())

	// Build pipeline WITHOUT speaker lock (this is the slow part — spawns yt-dlp).
	tp, err := p.buildYTDLPipeline(cur.path, startSec)
	if err != nil {
		p.restoreYTDLSeekSource(cur, gen)
		return fmt.Errorf("yt-dlp seek: %w", err)
	}
	tp.knownDuration = cur.knownDuration
	tp.ytdlSeek = true
	p.levelPipeline(tp, cur.level)

	if !p.commitYTDLSeek(cur, tp, gen) {
		// The seek was cancelled or another track started while the new pipeline
		// was being built.
		go closePipelines(tp)
	}
	return nil
}

// muteYTDLSeekSource reads the position of cur and silences it while SeekYTDL
// builds the replacement. It returns false and leaves the source alone when
// another track or a newer seek replaced cur after the snapshot. It checks
// under the same locks that commitYTDLSeek uses.
func (p *Player) muteYTDLSeekSource(cur *trackPipeline, gen int64) (time.Duration, bool) {
	speaker.Lock()
	defer speaker.Unlock()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current != cur || p.seekGen.Load() != gen {
		return 0, false
	}
	p.gapless.Replace(nil)
	p.gaplessAdvance.Store(false)
	// The decoder of a prefetched pipeline reads ahead of the speaker. Seek
	// from the audio that played, as the Position that the caller read does.
	pos, _ := cur.positionAndDuration()
	return pos, true
}

// commitYTDLSeek swaps in a rebuilt seek pipeline only when it still belongs
// to the active track. It must validate under the same locks used for the swap
// because playback can change while yt-dlp is starting.
func (p *Player) commitYTDLSeek(cur, replacement *trackPipeline, gen int64) bool {
	speaker.Lock()
	p.mu.Lock()
	if p.seekGen.Load() != gen || p.current != cur {
		p.mu.Unlock()
		speaker.Unlock()
		return false
	}
	p.gapless.Replace(replacement.stream)
	p.gapless.SetNext(nil)
	p.gaplessAdvance.Store(false)
	old := p.current
	oldNext := p.nextPipeline
	p.current = replacement
	p.nextPipeline = nil
	p.mu.Unlock()
	speaker.Unlock()
	// Clean up old pipelines async to avoid blocking on process wait.
	go closePipelines(old, oldNext)
	return true
}

// restoreYTDLSeekSource puts the original stream back after a replacement
// pipeline fails to start. The generation and current-pipeline checks prevent
// an obsolete seek from overwriting a newer seek or track change.
func (p *Player) restoreYTDLSeekSource(cur *trackPipeline, gen int64) {
	if cur == nil || p.seekGen.Load() != gen {
		return
	}
	speaker.Lock()
	p.mu.Lock()
	stillCurrent := p.current == cur && p.seekGen.Load() == gen
	if stillCurrent {
		p.gapless.Replace(cur.stream)
		p.gaplessAdvance.Store(false)
	}
	p.mu.Unlock()
	speaker.Unlock()
}

// IsYTDLSeek reports whether the current track uses yt-dlp seek-by-restart.
func (p *Player) IsYTDLSeek() bool {
	p.mu.Lock()
	cur := p.current
	p.mu.Unlock()
	return cur != nil && cur.ytdlSeek
}

// YTDLResumable reports whether the current yt-dlp track keeps its download
// in memory, so an unpause after a long pause needs no reconnect.
func (p *Player) YTDLResumable() bool {
	p.mu.Lock()
	cur := p.current
	p.mu.Unlock()
	if cur == nil {
		return false
	}
	y, ok := cur.decoder.(*ytdlPipeStreamer)
	return ok && y.relay.resumable()
}

// Position returns the current playback position.
// streamOffset is added for yt-dlp streams restarted at a time offset.
func (p *Player) Position() time.Duration {
	speaker.Lock()
	defer speaker.Unlock()
	pos, _ := p.positionAndDurationLocked()
	return pos
}

// Duration returns the total duration of the current track.
// For seekable local files it is derived from the decoder's sample count.
// For HTTP streams where the decoder reports Len()==0, the metadata hint
// stored at pipeline build time (knownDuration) is returned instead.
func (p *Player) Duration() time.Duration {
	speaker.Lock()
	defer speaker.Unlock()
	_, dur := p.positionAndDurationLocked()
	return dur
}

// PositionAndDuration returns both position and duration under a single
// speaker lock, avoiding two separate lock acquisitions per tick.
func (p *Player) PositionAndDuration() (time.Duration, time.Duration) {
	speaker.Lock()
	defer speaker.Unlock()
	return p.positionAndDurationLocked()
}

// positionAndDurationLocked is the rule of Position, Duration and
// PositionAndDuration: the clock of the current pipeline. The caller holds
// the speaker lock.
func (p *Player) positionAndDurationLocked() (time.Duration, time.Duration) {
	p.mu.Lock()
	cur := p.current
	p.mu.Unlock()
	if cur == nil {
		return 0, 0
	}
	return cur.positionAndDuration()
}

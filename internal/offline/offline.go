// Package offline downloads songs for listening without a connection. A
// download keeps YouTube's own AAC (m4a) audio when it exists, so nothing is
// re-encoded, and gets the cover, title, artist, album, track number and
// year written in. Files land in <dir>/<Artist>/<Album>/. A manifest maps
// each YouTube video to its file, so the player can play the file instead of
// streaming.
package offline

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/extbin"
	"github.com/xaviiromf/cliamp-x/internal/httpclient"
	"github.com/xaviiromf/cliamp-x/internal/sponsorblock"
	"github.com/xaviiromf/cliamp-x/internal/ytdlcookies"
	"github.com/xaviiromf/cliamp-x/playlist"
)

// Job is one song to download.
type Job struct {
	Track   playlist.Track
	Album   string // album tag; empty uses the track's album
	TrackNo int    // track number in the album, or 0
}

// Manifest maps YouTube video IDs to downloaded files. It is safe for
// concurrent use.
type Manifest struct {
	path  string
	mu    sync.Mutex
	files map[string]string
}

// OpenManifest loads the manifest at path; a missing file starts empty.
func OpenManifest(path string) *Manifest {
	m := &Manifest{path: path, files: map[string]string{}}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &m.files)
		if m.files == nil {
			m.files = map[string]string{}
		}
	}
	return m
}

// LocalFile returns the downloaded file of a YouTube URL when it still
// exists.
func (m *Manifest) LocalFile(path string) (string, bool) {
	file, ok := m.lookup(path)
	if !ok {
		return "", false
	}
	if _, err := os.Stat(file); err != nil {
		return "", false
	}
	return file, true
}

// Recorded reports whether path was downloaded, from memory only. Unlike Has
// it does not check that the file is still there, so drawing a list row on
// every frame costs no file system call.
func (m *Manifest) Recorded(path string) bool {
	_, ok := m.lookup(path)
	return ok
}

// lookup returns the recorded file of path, without checking it exists.
func (m *Manifest) lookup(path string) (string, bool) {
	if m == nil {
		return "", false
	}
	id := sponsorblock.VideoID(path)
	if id == "" {
		return "", false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	file, ok := m.files[id]
	return file, ok
}

// Len returns the number of downloads recorded.
func (m *Manifest) Len() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.files)
}

// Has reports whether path has a downloaded file.
func (m *Manifest) Has(path string) bool {
	_, ok := m.LocalFile(path)
	return ok
}

// Add records a download and saves the manifest.
func (m *Manifest) Add(path, file string) error {
	id := sponsorblock.VideoID(path)
	if id == "" || m == nil {
		return nil
	}
	m.mu.Lock()
	m.files[id] = file
	data, err := json.Marshal(m.files)
	m.mu.Unlock()
	if err != nil || m.path == "" {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.path)
}

// sanitize makes a name safe as one path element.
func sanitize(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '_'
		}
		return r
	}, strings.TrimSpace(name))
	name = strings.Trim(name, ". ")
	if len(name) > 120 {
		name = name[:120]
	}
	if name == "" {
		name = "_"
	}
	return name
}

// Destination returns the folder and file name (without extension) of a
// job under dir.
func Destination(dir string, job Job) (folder, base string) {
	t := job.Track
	artist := t.Artist
	if artist == "" {
		artist = "Unknown artist"
	}
	album := job.Album
	if album == "" {
		album = t.Album
	}
	if album == "" {
		album = "Singles"
	}
	title := t.Title
	if title == "" {
		title = sponsorblock.VideoID(t.Path)
	}
	base = sanitize(title)
	if job.TrackNo > 0 {
		base = fmt.Sprintf("%02d - %s", job.TrackNo, base)
	}
	return filepath.Join(dir, sanitize(artist), sanitize(album)), base
}

// progressLine matches the progress template's output.
var progressLine = regexp.MustCompile(`^CLIAMP-PROGRESS\s+([\d.]+)%`)

// Download fetches job into dir and returns the file. progress, when set,
// gets the download's percentage as it advances.
func Download(ctx context.Context, job Job, dir string, progress func(percent float64)) (string, error) {
	if !extbin.Have("yt-dlp") {
		return "", errors.New("yt-dlp not found in PATH")
	}
	folder, base := Destination(dir, job)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return "", err
	}
	tmpDir, err := os.MkdirTemp(folder, ".cliamp-dl-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	args := []string{
		"-f", "bestaudio[ext=m4a]/bestaudio",
		"--no-playlist", "--newline", "--no-part", "--quiet", "--progress",
		"--progress-template", "download:CLIAMP-PROGRESS %(progress._percent_str)s",
		"--print", "after_move:CLIAMP-FILE %(filepath)s",
		"-o", filepath.Join(tmpDir, "audio.%(ext)s"),
	}
	args = append(args, ytdlcookies.Args(ytdlcookies.ForURL(job.Track.Path))...)
	args = append(args, "--", job.Track.Path)
	cmd := extbin.Command(ctx, "yt-dlp", args...)
	cmd.WaitDelay = 3 * time.Second
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var file string
	var lastErr string
	done := make(chan struct{})
	go func() {
		defer close(done)
		sc := bufio.NewScanner(pr)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			switch {
			case strings.HasPrefix(line, "CLIAMP-FILE "):
				file = strings.TrimPrefix(line, "CLIAMP-FILE ")
			case progressLine.MatchString(line):
				if v, err := strconv.ParseFloat(progressLine.FindStringSubmatch(line)[1], 64); err == nil && progress != nil {
					progress(v)
				}
			case strings.HasPrefix(line, "ERROR:"):
				lastErr = line
			}
		}
	}()
	err = cmd.Wait()
	pw.Close()
	<-done
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if lastErr != "" {
			return "", fmt.Errorf("yt-dlp: %s", strings.TrimPrefix(lastErr, "ERROR: "))
		}
		return "", fmt.Errorf("yt-dlp: %w", err)
	}
	if file == "" {
		return "", errors.New("yt-dlp: no file downloaded")
	}
	return finalize(ctx, job, file, folder, base)
}

// finalize writes the tags and the cover with ffmpeg and moves the file to
// its place. Without ffmpeg the file is moved as it is.
func finalize(ctx context.Context, job Job, file, folder, base string) (string, error) {
	ext := strings.ToLower(filepath.Ext(file))
	if !extbin.Have("ffmpeg") {
		dest := filepath.Join(folder, base+ext)
		return dest, os.Rename(file, dest)
	}
	outExt := ext
	if ext != ".m4a" && ext != ".mp4" {
		// WebM audio is Opus: an Ogg container holds it with tags.
		outExt = ".opus"
	}
	dest := filepath.Join(folder, base+outExt)
	t := job.Track
	album := job.Album
	if album == "" {
		album = t.Album
	}
	args := []string{"-y", "-loglevel", "error", "-i", file}
	cover := ""
	if outExt == ".m4a" {
		cover = fetchCover(ctx, t.AlbumArtURL, filepath.Dir(file))
	}
	if cover != "" {
		args = append(args, "-i", cover, "-map", "0:a", "-map", "1", "-disposition:v:0", "attached_pic")
	} else {
		args = append(args, "-map", "0:a")
	}
	args = append(args, "-c", "copy")
	meta := map[string]string{"title": t.Title, "artist": t.Artist, "album": album, "genre": t.Genre}
	if t.Year > 0 {
		meta["date"] = strconv.Itoa(t.Year)
	}
	if job.TrackNo > 0 {
		meta["track"] = strconv.Itoa(job.TrackNo)
	}
	meta["comment"] = t.Path
	for k, v := range meta {
		if v != "" {
			args = append(args, "-metadata", k+"="+v)
		}
	}
	tmp := filepath.Join(filepath.Dir(file), "tagged"+outExt)
	args = append(args, tmp)
	cmd := extbin.Command(ctx, "ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Keep the audio even when tagging fails.
		_ = out
		return dest, os.Rename(file, dest)
	}
	return dest, os.Rename(tmp, dest)
}

// fetchCover downloads the cover image next to the audio and returns its
// path, or "" when there is none.
func fetchCover(ctx context.Context, url, dir string) string {
	if url == "" || !strings.HasPrefix(url, "http") {
		return ""
	}
	url = largeCover(url)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "image/jpeg,image/png")
	resp, err := httpclient.NewAPI(20 * time.Second).Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	ext := ".jpg"
	if strings.Contains(resp.Header.Get("Content-Type"), "png") {
		ext = ".png"
	}
	path := filepath.Join(dir, "cover"+ext)
	f, err := os.Create(path)
	if err != nil {
		return ""
	}
	_, err = io.Copy(f, io.LimitReader(resp.Body, 8<<20))
	f.Close()
	if err != nil {
		return ""
	}
	return path
}

// coverSize matches the "=w400-h400" size of a googleusercontent image URL.
var coverSize = regexp.MustCompile(`=w\d+-h\d+`)

// largeCover asks for a 1200 px cover, the size of a typical album scan.
// Video thumbnails ask for the largest still.
func largeCover(url string) string {
	if strings.Contains(url, "googleusercontent.com/") || strings.Contains(url, "ggpht.com/") {
		if loc := coverSize.FindStringIndex(url); loc != nil {
			return url[:loc[0]] + "=w1200-h1200" + url[loc[1]:]
		}
	}
	if strings.Contains(url, "i.ytimg.com/vi/") {
		return strings.Replace(url, "/mqdefault.jpg", "/hqdefault.jpg", 1)
	}
	return url
}

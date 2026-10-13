// Package upgrade implements self-upgrade by downloading the latest
// release binary from GitHub, mirroring the install.sh mechanism.
package upgrade

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/xaviiromf/cliamp-x/internal/httpclient"
)

const repo = "xaviiromf/cliamp-x"

// httpClient serves the GitHub API and checksum requests.
var httpClient = httpclient.NewAPI(30 * time.Second)

// downloadClient fetches the release binary. A client timeout also covers the
// body read, so this client has none and a slow connection can finish the
// download. downloadIdleTimeout ends a download that stops sending.
var downloadClient = httpclient.NewAPI(0)

// downloadIdleTimeout is the longest wait of the binary download for the
// response headers or for the next body bytes.
var downloadIdleTimeout = 30 * time.Second

// errDownloadStalled ends a binary download that got no data within
// downloadIdleTimeout.
var errDownloadStalled = errors.New("download stalled")

// releaseMaxBytes limits the releases/latest response. releaseListMaxBytes
// limits the list of up to 100 releases. Each release lists its assets, so
// the list is larger than 1 MiB.
const (
	releaseMaxBytes     = 1 << 20
	releaseListMaxBytes = 16 << 20
)

type release struct {
	TagName string  `json:"tag_name"`
	Draft   bool    `json:"draft"`
	Assets  []asset `json:"assets"`
}

// asset is a release file. URL is its API address, which serves the file
// to an authorized client even when the repository is private.
type asset struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// apiHost is the only host that gets the GitHub token. Asset downloads
// redirect to a storage host with a signed URL, and the HTTP client drops
// the Authorization header on that cross-host redirect.
const apiHost = "api.github.com"

// tokenSource returns a GitHub token, or "" for anonymous requests. A token
// is needed while the repository is private.
var tokenSource = githubToken

// githubToken reads GITHUB_TOKEN or GH_TOKEN, then asks the GitHub CLI.
func githubToken() string {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if tok := strings.TrimSpace(os.Getenv(key)); tok != "" {
			return tok
		}
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "gh", "auth", "token", "--hostname", "github.com")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}

// newRequest builds a GET request for url. API requests carry the token,
// and asset requests ask for the raw file instead of its JSON description.
func newRequest(ctx context.Context, url string, token string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if req.URL.Host == apiHost {
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if strings.Contains(req.URL.Path, "/releases/assets/") {
			req.Header.Set("Accept", "application/octet-stream")
		} else {
			req.Header.Set("Accept", "application/vnd.github+json")
		}
	}
	return req, nil
}

// privateHint explains a 404 from GitHub when no token was sent: a private
// repository answers 404 to anonymous requests.
func privateHint(status int, token string) string {
	if status == http.StatusNotFound && token == "" {
		return " (if the repository is private, run `gh auth login` or set GITHUB_TOKEN)"
	}
	return ""
}

// Run checks for a newer release and replaces the current binary if one is found.
func Run(currentVersion string, prerelease bool) error {
	// Android runs only the binaries that the installed app holds.
	if runtime.GOOS == "android" {
		return errors.New("cliamp-x upgrade cannot replace the Android app: update it from the app menu")
	}
	token := tokenSource()
	rel, err := latestRelease(prerelease, token)
	if err != nil {
		return fmt.Errorf("checking latest version: %w", err)
	}
	latest := rel.TagName

	// With prerelease, the user can run a build newer than every listed
	// release. An older release must not replace it.
	if currentVersion != "" && (currentVersion == latest || prerelease && notNewer(latest, currentVersion)) {
		fmt.Printf("Already up to date (%s)\n", currentVersion)
		return nil
	}

	if currentVersion == "" {
		releaseType := "release"
		if prerelease {
			releaseType = "prerelease"
		}
		fmt.Printf("Latest %s is %s, downloading...\n", releaseType, latest)
	} else {
		fmt.Printf("Upgrading %s → %s\n", currentVersion, latest)
	}

	binaryName := fmt.Sprintf("cliamp-x-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	// The API asset URLs work for private repositories too. A release
	// response without assets falls back to the public download URLs.
	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", repo, latest)
	checksumURL := baseURL + "/checksums.txt"
	binaryURL := baseURL + "/" + binaryName
	for _, a := range rel.Assets {
		switch a.Name {
		case "checksums.txt":
			checksumURL = a.URL
		case binaryName:
			binaryURL = a.URL
		}
	}

	fmt.Printf("Downloading %s...\n", binaryName)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating current binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	expectedHash, err := releaseChecksumWith(checksumURL, binaryName, token)
	if err != nil {
		return fmt.Errorf("verifying release checksum: %w", err)
	}
	if err := downloadAndReplaceWith(binaryURL, exe, expectedHash, token); err != nil {
		return err
	}

	fmt.Printf("Upgraded to %s\n", latest)
	return nil
}

func latestVersion(prerelease bool) (string, error) {
	r, err := latestRelease(prerelease, tokenSource())
	return r.TagName, err
}

// latestRelease returns the newest release with its assets.
func latestRelease(prerelease bool, token string) (release, error) {
	endpoint := "releases/latest"
	if prerelease {
		endpoint = "releases?per_page=100"
	}
	url := fmt.Sprintf("https://%s/repos/%s/%s", apiHost, repo, endpoint)
	req, err := newRequest(context.Background(), url, token)
	if err != nil {
		return release{}, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("GitHub API returned %s%s", resp.Status, privateHint(resp.StatusCode, token))
	}

	if !prerelease {
		var r release
		if err := httpclient.ReadJSON(resp.Body, releaseMaxBytes, &r); err != nil {
			return release{}, fmt.Errorf("parsing response: %w", err)
		}
		tag, err := validTag(r.TagName)
		r.TagName = tag
		return r, err
	}

	var releases []release
	if err := httpclient.ReadJSON(resp.Body, releaseListMaxBytes, &releases); err != nil {
		return release{}, fmt.Errorf("parsing response: %w", err)
	}
	return newestRelease(releases)
}

// newestRelease returns the tag of the release with the highest SemVer
// version that is not a draft. Stable releases count too, so a stable release
// that follows a prerelease wins over it. Tags that are not SemVer versions
// are skipped.
func newestRelease(releases []release) (release, error) {
	var best release
	var bestVersion version
	for _, r := range releases {
		v, ok := parseVersion(r.TagName)
		if r.Draft || !ok {
			continue
		}
		if best.TagName == "" || v.compare(bestVersion) > 0 {
			best, bestVersion = r, v
		}
	}
	if best.TagName == "" {
		return release{}, errors.New("no releases found")
	}
	tag, err := validTag(best.TagName)
	best.TagName = tag
	return best, err
}

func validTag(tag string) (string, error) {
	if strings.TrimSpace(tag) == "" || strings.ContainsAny(tag, "/\\") {
		return "", errors.New("release response contains an invalid tag")
	}
	return tag, nil
}

func releaseChecksum(url, binaryName string) (string, error) {
	return releaseChecksumWith(url, binaryName, "")
}

func releaseChecksumWith(url, binaryName, token string) (string, error) {
	req, err := newRequest(context.Background(), url, token)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download failed: %s%s", resp.Status, privateHint(resp.StatusCode, token))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return "", err
	}
	if len(data) > 1<<20 {
		return "", errors.New("checksum file is too large")
	}
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != binaryName {
			continue
		}
		if len(fields[0]) != sha256.Size*2 {
			return "", fmt.Errorf("invalid SHA-256 entry for %s", binaryName)
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			return "", fmt.Errorf("invalid SHA-256 entry for %s", binaryName)
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", fmt.Errorf("no SHA-256 entry for %s", binaryName)
}

func downloadAndReplace(url, destPath, expectedHash string) error {
	return downloadAndReplaceWith(url, destPath, expectedHash, "")
}

func downloadAndReplaceWith(url, destPath, expectedHash, token string) error {
	if len(expectedHash) != sha256.Size*2 {
		return errors.New("valid expected SHA-256 is required")
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	idle := time.AfterFunc(downloadIdleTimeout, func() {
		cancel(fmt.Errorf("%w: no data for %s", errDownloadStalled, downloadIdleTimeout))
	})
	defer idle.Stop()
	req, err := newRequest(ctx, url, token)
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		if stall := stallError(ctx); stall != nil {
			err = stall
		}
		return fmt.Errorf("downloading: %w", err)
	}
	defer resp.Body.Close()
	// The headers arrived, so the first body bytes get a full idle window.
	idle.Reset(downloadIdleTimeout)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: %s%s", resp.Status, privateHint(resp.StatusCode, token))
	}

	// Write to a temp file in the same directory as the target so
	// os.Rename works (same filesystem).
	dir := filepath.Dir(destPath)
	tmp, err := os.CreateTemp(dir, "cliamp-upgrade-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w (try running with sudo)", err)
	}
	tmpPath := tmp.Name()

	// Limit download to 200 MB to prevent unbounded disk usage from a
	// rogue redirect or compromised CDN.
	const maxBinarySize = 200 << 20
	h := sha256.New()
	body := &idleReader{r: resp.Body, timer: idle, timeout: downloadIdleTimeout}
	written, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(body, maxBinarySize+1))
	if err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		if stall := stallError(ctx); stall != nil {
			return fmt.Errorf("downloading: %w", stall)
		}
		return fmt.Errorf("writing binary: %w", err)
	}
	if written == 0 || written > maxBinarySize {
		tmp.Close()
		os.Remove(tmpPath)
		return errors.New("download is empty or exceeds maximum size")
	}
	if resp.ContentLength >= 0 && written != resp.ContentLength {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("truncated download: received %d of %d bytes", written, resp.ContentLength)
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, expectedHash) {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("SHA-256 mismatch: got %s", got)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("syncing binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing binary: %w", err)
	}

	if err := os.Chmod(tmpPath, 0o755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := replaceExecutable(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		if runtime.GOOS != "windows" {
			return fmt.Errorf("replacing binary: %w (try running with sudo)", err)
		}
		return fmt.Errorf("replacing binary: %w", err)
	}

	return nil
}

// idleReader restarts timer each time a read returns data, so the timer
// fires only when the body stops sending for timeout.
type idleReader struct {
	r       io.Reader
	timer   *time.Timer
	timeout time.Duration
}

func (r *idleReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if n > 0 {
		r.timer.Reset(r.timeout)
	}
	return n, err
}

// stallError returns the stall error when the idle timer canceled ctx, and
// nil otherwise. The canceled request reports only a generic context error.
func stallError(ctx context.Context) error {
	if cause := context.Cause(ctx); errors.Is(cause, errDownloadStalled) {
		return cause
	}
	return nil
}

func replaceExecutable(sourcePath, destPath string) error {
	if runtime.GOOS == "windows" {
		return replaceExecutableOnWindows(sourcePath, destPath)
	}
	return os.Rename(sourcePath, destPath)
}

func replaceExecutableOnWindows(sourcePath, destPath string) error {
	backupPath := filepath.Join(filepath.Dir(destPath), "."+filepath.Base(destPath)+".old")
	if err := os.Remove(backupPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing previous binary backup: %w", err)
	}

	if err := os.Rename(destPath, backupPath); err != nil {
		return fmt.Errorf("moving current binary aside: %w", err)
	}

	if err := os.Rename(sourcePath, destPath); err != nil {
		if rollbackErr := os.Rename(backupPath, destPath); rollbackErr != nil {
			return fmt.Errorf("installing new binary: %w; restoring current binary: %w", err, rollbackErr)
		}
		return fmt.Errorf("installing new binary: %w", err)
	}

	// Windows keeps the running executable locked. A leftover backup is
	// harmless and will be removed before the next upgrade.
	_ = os.Remove(backupPath)
	return nil
}

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
	"slices"
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
	Name    string  `json:"name"`
	Draft   bool    `json:"draft"`
	Assets  []asset `json:"assets"`
}

// version returns the tag, or the release name when the tag is not a
// version: the Android nightly moves the tag "nightly" and names the
// release after its version.
func (r release) version() string {
	if _, ok := parseVersion(r.TagName); !ok {
		if _, ok := parseVersion(r.Name); ok {
			return r.Name
		}
	}
	return r.TagName
}

// asset is a release file. URL is its API address, which serves the file
// to an authorized client even when the repository is private.
// DownloadURL is its public address on github.com.
type asset struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	DownloadURL string `json:"browser_download_url"`
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
	binaryName := fmt.Sprintf("cliamp-x-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	token := tokenSource()
	rel, err := latestRelease(prerelease, token, binaryName)
	if err != nil {
		return fmt.Errorf("checking latest version: %w", err)
	}
	latest := rel.version()

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

	// The API asset URLs work for private repositories too. A release
	// response without assets falls back to the public download URLs.
	baseURL := fmt.Sprintf("https://github.com/%s/releases/download/%s", repo, rel.TagName)
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

// Report is what `cliamp-x upgrade --check --json` prints: the newest
// release and its files. Prerelease is true when Latest has a SemVer
// prerelease part, as a release candidate or an Android nightly has. Newer
// is false when Latest is the version that runs or an older one.
type Report struct {
	Current    string        `json:"current"`
	Latest     string        `json:"latest"`
	Prerelease bool          `json:"prerelease"`
	Newer      bool          `json:"newer"`
	Assets     []ReportAsset `json:"assets"`
}

// ReportAsset is a release file. URL is its public download address. SHA256
// comes from the checksums.txt of the release, and is "" for a file that
// checksums.txt does not list.
type ReportAsset struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// Check finds the newest release as Run does, for any platform, and reports
// it with the SHA-256 of each file. It reads the release and its
// checksums.txt and downloads nothing else, so the Android app can use it.
func Check(current string, prerelease bool) (Report, error) {
	token := tokenSource()
	rel, err := latestRelease(prerelease, token, "")
	if err != nil {
		return Report{}, fmt.Errorf("checking latest version: %w", err)
	}
	latest := rel.version()
	v, ok := parseVersion(latest)
	report := Report{
		Current:    current,
		Latest:     latest,
		Prerelease: ok && len(v.pre) > 0,
		Newer:      latest != current && !notNewer(latest, current),
		Assets:     []ReportAsset{},
	}
	var sums map[string]string
	for _, a := range rel.Assets {
		if a.Name == "checksums.txt" {
			if sums, err = checksums(a.URL, token); err != nil {
				return Report{}, fmt.Errorf("reading release checksums: %w", err)
			}
		}
	}
	for _, a := range rel.Assets {
		report.Assets = append(report.Assets, ReportAsset{Name: a.Name, URL: a.DownloadURL, SHA256: sums[a.Name]})
	}
	return report, nil
}

func latestVersion(prerelease bool) (string, error) {
	r, err := latestRelease(prerelease, tokenSource(), "")
	return r.version(), err
}

// latestRelease returns the newest release with its assets. With
// prerelease, a release that lists its files without binary is skipped.
func latestRelease(prerelease bool, token, binary string) (release, error) {
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
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return release{}, errors.New("GitHub API rate limit exceeded: try again in an hour, or set GITHUB_TOKEN")
		}
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
	return newestRelease(releases, binary)
}

// newestRelease returns the release with the highest SemVer version that is
// not a draft. Stable releases count too, so a stable release that follows a
// prerelease wins over it. Releases without a SemVer version are skipped, and
// so is a release that lists its files without binary, when binary is set:
// the Android nightly has no desktop build.
func newestRelease(releases []release, binary string) (release, error) {
	var best release
	var bestVersion version
	for _, r := range releases {
		v, ok := parseVersion(r.version())
		if r.Draft || !ok || binary != "" && len(r.Assets) > 0 && !slices.ContainsFunc(r.Assets, func(a asset) bool { return a.Name == binary }) {
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
	sums, err := checksums(url, token)
	if err != nil {
		return "", err
	}
	hash, ok := sums[binaryName]
	if !ok {
		return "", fmt.Errorf("no SHA-256 entry for %s", binaryName)
	}
	if hash == "" {
		return "", fmt.Errorf("invalid SHA-256 entry for %s", binaryName)
	}
	return hash, nil
}

// checksums downloads a sha256sum file and maps each file name to its
// SHA-256 in lower case. A malformed hash maps to "", so only that file
// fails to verify. The first line of a name wins.
func checksums(url, token string) (map[string]string, error) {
	req, err := newRequest(context.Background(), url, token)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checksum download failed: %s%s", resp.Status, privateHint(resp.StatusCode, token))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("checksum file is too large")
	}
	sums := map[string]string{}
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if _, seen := sums[name]; seen {
			continue
		}
		hash := ""
		if _, err := hex.DecodeString(fields[0]); err == nil && len(fields[0]) == sha256.Size*2 {
			hash = strings.ToLower(fields[0])
		}
		sums[name] = hash
	}
	return sums, nil
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

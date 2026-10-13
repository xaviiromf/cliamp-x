package upgrade

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestMain keeps the tests away from the developer's real GitHub token.
func TestMain(m *testing.M) {
	tokenSource = func() string { return "" }
	os.Exit(m.Run())
}

func TestNewRequestHeaders(t *testing.T) {
	tests := []struct {
		name, url, token string
		wantAuth         string
		wantAccept       string
	}{
		{"api with token", "https://api.github.com/repos/o/r/releases/latest", "tok", "Bearer tok", "application/vnd.github+json"},
		{"asset with token", "https://api.github.com/repos/o/r/releases/assets/42", "tok", "Bearer tok", "application/octet-stream"},
		{"api without token", "https://api.github.com/repos/o/r/releases/latest", "", "", "application/vnd.github+json"},
		{"public download never gets the token", "https://github.com/o/r/releases/download/v1/x", "tok", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := newRequest(t.Context(), tt.url, tt.token)
			if err != nil {
				t.Fatal(err)
			}
			if got := req.Header.Get("Authorization"); got != tt.wantAuth {
				t.Errorf("Authorization = %q, want %q", got, tt.wantAuth)
			}
			if got := req.Header.Get("Accept"); got != tt.wantAccept {
				t.Errorf("Accept = %q, want %q", got, tt.wantAccept)
			}
		})
	}
}

func TestLatestVersion404HintsPrivateRepo(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	installTestClient(t, srv.URL)
	_, err := latestVersion(false)
	if err == nil || !strings.Contains(err.Error(), "gh auth login") {
		t.Fatalf("latestVersion on 404 = %v, want a hint about private repositories", err)
	}
}

// TestRunPrivateRepoUsesAssetAPI checks the whole upgrade through the API
// asset URLs with a token, as a private repository needs.
func TestRunPrivateRepoUsesAssetAPI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("replaces the test binary path in place")
	}
	binaryName := "cliamp-x-" + runtime.GOOS + "-" + runtime.GOARCH
	payload := []byte("new cliamp-x binary")
	checksums := testHash(payload) + "  " + binaryName + "\n"

	var sawAuth, sawAccept bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			http.NotFound(w, r)
			return
		}
		sawAuth = true
		switch r.URL.Path {
		case "/repos/xaviiromf/cliamp-x/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"tag_name":"v9.0.0","assets":[
				{"name":"checksums.txt","url":"https://api.github.com/repos/xaviiromf/cliamp-x/releases/assets/1"},
				{"name":"`+binaryName+`","url":"https://api.github.com/repos/xaviiromf/cliamp-x/releases/assets/2"}]}`)
		case "/repos/xaviiromf/cliamp-x/releases/assets/1":
			sawAccept = r.Header.Get("Accept") == "application/octet-stream"
			_, _ = io.WriteString(w, checksums)
		case "/repos/xaviiromf/cliamp-x/releases/assets/2":
			_, _ = w.Write(payload)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	installTestClient(t, srv.URL)
	tokenSource = func() string { return "secret" }
	t.Cleanup(func() { tokenSource = func() string { return "" } })

	// Run replaces os.Executable(); point it at a copy instead.
	dir := t.TempDir()
	target := filepath.Join(dir, "cliamp-x")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	expected := testHash(payload)
	rel, err := latestRelease(false, tokenSource(), "")
	if err != nil {
		t.Fatal(err)
	}
	var sumURL, binURL string
	for _, a := range rel.Assets {
		if a.Name == "checksums.txt" {
			sumURL = a.URL
		} else {
			binURL = a.URL
		}
	}
	hash, err := releaseChecksumWith(sumURL, binaryName, "secret")
	if err != nil || hash != expected {
		t.Fatalf("checksum = %q, %v; want %q", hash, err, expected)
	}
	if err := downloadAndReplaceWith(binURL, target, hash, "secret"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != string(payload) || !sawAuth || !sawAccept {
		t.Fatalf("binary = %q, auth %v, octet-stream %v", got, sawAuth, sawAccept)
	}
}

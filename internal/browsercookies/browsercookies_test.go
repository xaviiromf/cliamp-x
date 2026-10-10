package browsercookies

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// encryptChromium encrypts a cookie value the way Chromium does on Linux,
// with the host hash prefix of cookie database version 24.
func encryptChromium(t *testing.T, prefix, password, host, value string) []byte {
	t.Helper()
	sum := sha256.Sum256([]byte(host))
	plain := append(sum[:], value...)
	pad := aes.BlockSize - len(plain)%aes.BlockSize
	plain = append(plain, bytes.Repeat([]byte{byte(pad)}, pad)...)
	block, err := aes.NewCipher(deriveKey(password))
	if err != nil {
		t.Fatal(err)
	}
	out := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, bytes.Repeat([]byte{' '}, aes.BlockSize)).CryptBlocks(out, plain)
	return append([]byte(prefix), out...)
}

func createChromiumDB(t *testing.T, path string, rows func(db *sql.DB)) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stmt := range []string{
		`CREATE TABLE meta (key TEXT, value TEXT)`,
		`INSERT INTO meta VALUES ('version', '24')`,
		`CREATE TABLE cookies (host_key TEXT, name TEXT, value TEXT, encrypted_value BLOB, path TEXT, expires_utc INTEGER, is_secure INTEGER, is_httponly INTEGER)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	rows(db)
}

func TestReadChromiumWithKeyringAndPeanutsKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	keyringLookup = func([]string) []string { return []string{"wrong", "test-keyring-pw"} }
	t.Cleanup(func() { keyringLookup = keyringPasswords })

	db := filepath.Join(home, ".config/BraveSoftware/Brave-Origin/Default/Cookies")
	// Chromium time: microseconds since 1601. 13_400_000_000_000_000 is in 2025.
	const expires = 13_400_000_000_000_000
	createChromiumDB(t, db, func(db *sql.DB) {
		ins := `INSERT INTO cookies VALUES (?, ?, '', ?, '/', ?, 1, 0)`
		for _, c := range []struct{ prefix, pw, host, name, value string }{
			{"v11", "test-keyring-pw", ".youtube.com", "SAPISID", "sapi-value"},
			{"v11", "test-keyring-pw", ".youtube.com", "SID", "sid-value"},
			{"v10", "peanuts", ".youtube.com", "PREF", "f6=40000000"},
			{"v11", "test-keyring-pw", ".example.com", "OTHER", "x"},
		} {
			if _, err := db.Exec(ins, c.host, c.name, encryptChromium(t, c.prefix, c.pw, c.host, c.value), expires); err != nil {
				t.Fatal(err)
			}
		}
	})

	cookies, src, err := Read("", []string{"youtube.com"}, "SAPISID")
	if err != nil {
		t.Fatal(err)
	}
	if src.Browser != "brave-origin" {
		t.Fatalf("source = %+v", src)
	}
	got := map[string]string{}
	for _, c := range cookies {
		got[c.Name] = c.Value
		if c.Expires != expires/1_000_000-chromeEpochOffset || !c.Secure {
			t.Errorf("cookie %s expiry %d secure %v", c.Name, c.Expires, c.Secure)
		}
	}
	want := map[string]string{"SAPISID": "sapi-value", "SID": "sid-value", "PREF": "f6=40000000"}
	if len(got) != len(want) {
		t.Fatalf("cookies = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}

	// A named browser that has no profile fails clearly.
	if _, _, err := Read("chrome", []string{"youtube.com"}, "SAPISID"); err == nil {
		t.Error("Read(chrome) without a Chrome profile succeeded")
	}
	if _, _, err := Read("netscape", nil, ""); err == nil {
		t.Error("an unsupported browser was accepted")
	}
}

func TestReadReportsNotSignedIn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	keyringLookup = func([]string) []string { return nil }
	t.Cleanup(func() { keyringLookup = keyringPasswords })
	createChromiumDB(t, filepath.Join(home, ".config/chromium/Default/Cookies"), func(db *sql.DB) {
		if _, err := db.Exec(`INSERT INTO cookies VALUES ('.youtube.com', 'YSC', 'plain', x'', '/', 0, 1, 0)`); err != nil {
			t.Fatal(err)
		}
	})
	if _, _, err := Read("", []string{"youtube.com"}, "SAPISID"); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("err = %v, want ErrNotSignedIn", err)
	}
}

func TestReadFirefox(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".mozilla/firefox/abcd.default-release/cookies.sqlite")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE moz_cookies (host TEXT, name TEXT, value TEXT, path TEXT, expiry INTEGER, isSecure INTEGER, isHttpOnly INTEGER)`,
		`INSERT INTO moz_cookies VALUES ('.youtube.com', 'SAPISID', 'ff-sapi', '/', 1900000000000, 1, 0)`,
		`INSERT INTO moz_cookies VALUES ('.other.org', 'X', 'y', '/', 0, 0, 0)`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()

	cookies, src, err := Read("firefox", []string{"youtube.com"}, "SAPISID")
	if err != nil {
		t.Fatal(err)
	}
	if src.Browser != "firefox" || len(cookies) != 1 || cookies[0].Value != "ff-sapi" || cookies[0].Expires != 1900000000 {
		t.Fatalf("cookies = %+v src = %+v", cookies, src)
	}
}

func TestNetscapeRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jar", "cookies.txt")
	in := []Cookie{
		{Host: ".youtube.com", Name: "SAPISID", Value: "v1", Path: "/", Secure: true, Expires: 1900000000},
		{Host: "music.youtube.com", Name: "A", Value: "b", Path: "/x", HTTPOnly: true},
		{Host: ".youtube.com", Name: "BAD", Value: "tab\there"},
	}
	if err := WriteNetscape(path, in); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("jar stat = %v, %v", info, err)
	}
	out, err := ReadNetscape(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0] != in[0] || out[1] != in[1] {
		t.Fatalf("round trip = %+v", out)
	}
}

func TestParseSpec(t *testing.T) {
	for in, want := range map[string][2]string{
		"brave":                      {"brave", ""},
		"brave+gnomekeyring:Default": {"brave", "Default"},
		"chrome:Profile 1":           {"chrome", "Profile 1"},
		" firefox:~/.config/zen ":    {"firefox", "~/.config/zen"},
		"":                           {"", ""},
	} {
		name, prof := ParseSpec(in)
		if name != want[0] || prof != want[1] {
			t.Errorf("ParseSpec(%q) = %q %q, want %q", in, name, prof, want)
		}
	}
	if _, ok := findBrowser("google-chrome"); !ok {
		t.Error("the yt-dlp alias google-chrome is not accepted")
	}
}

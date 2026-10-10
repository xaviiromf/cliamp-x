package browsercookies

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// chromeEpochOffset is the number of seconds between 1601-01-01, the epoch
// of Chromium timestamps, and the Unix epoch.
const chromeEpochOffset = 11644473600

type chromiumRow struct {
	host, name, path string
	value            string
	encrypted        []byte
	expiresUTC       int64
	secure, httpOnly bool
}

// readChromium reads and decrypts the cookies of a Chromium profile.
func readChromium(p profile, domains []string) ([]Cookie, error) {
	db, err := openSQLite(p.db)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	version := 0
	var raw string
	if err := db.QueryRow(`SELECT value FROM meta WHERE key = 'version'`).Scan(&raw); err == nil {
		version, _ = strconv.Atoi(raw)
	}
	rows, err := db.Query(`SELECT host_key, name, value, encrypted_value, path, expires_utc, is_secure, is_httponly FROM cookies`)
	if err != nil {
		return nil, fmt.Errorf("read cookies: %w", err)
	}
	defer rows.Close()
	var all []chromiumRow
	for rows.Next() {
		var r chromiumRow
		var secure, httpOnly int
		if err := rows.Scan(&r.host, &r.name, &r.value, &r.encrypted, &r.path, &r.expiresUTC, &secure, &httpOnly); err != nil {
			return nil, fmt.Errorf("read cookies: %w", err)
		}
		if !matchesAny(r.host, domains) {
			continue
		}
		r.secure, r.httpOnly = secure != 0, httpOnly != 0
		all = append(all, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read cookies: %w", err)
	}
	if len(all) == 0 {
		return nil, nil
	}

	hashPrefix := version >= 24
	keys := chromiumKeys(p.browser.apps)
	v11, ok := pickKey(all, keys, hashPrefix)
	if !ok && needsV11(all) {
		return nil, errors.New("cannot decrypt the cookies: no keyring key fits (is the keyring unlocked?)")
	}
	v10 := deriveKey("peanuts")

	out := make([]Cookie, 0, len(all))
	for _, r := range all {
		value := r.value
		if value == "" && len(r.encrypted) > 0 {
			key := v11
			if bytes.HasPrefix(r.encrypted, []byte("v10")) {
				key = v10
			}
			plain, ok := decrypt(key, r, hashPrefix)
			if !ok {
				continue
			}
			value = plain
		}
		out = append(out, Cookie{
			Host: r.host, Name: r.name, Value: value, Path: r.path,
			Secure: r.secure, HTTPOnly: r.httpOnly, Expires: chromeExpiry(r.expiresUTC),
		})
	}
	return out, nil
}

func matchesAny(host string, domains []string) bool {
	if len(domains) == 0 {
		return true
	}
	for _, d := range domains {
		if domainMatch(host, d) {
			return true
		}
	}
	return false
}

func chromeExpiry(expiresUTC int64) int64 {
	if expiresUTC <= 0 {
		return 0
	}
	unix := expiresUTC/1_000_000 - chromeEpochOffset
	if unix < 0 {
		return 0
	}
	return unix
}

func needsV11(rows []chromiumRow) bool {
	for _, r := range rows {
		if r.value == "" && bytes.HasPrefix(r.encrypted, []byte("v11")) {
			return true
		}
	}
	return false
}

// pickKey returns the candidate key that decrypts the most v11 cookies.
func pickKey(rows []chromiumRow, keys [][]byte, hashPrefix bool) ([]byte, bool) {
	var best []byte
	bestCount := 0
	for _, key := range keys {
		count := 0
		tried := 0
		for _, r := range rows {
			if r.value != "" || !bytes.HasPrefix(r.encrypted, []byte("v11")) {
				continue
			}
			tried++
			if _, ok := decrypt(key, r, hashPrefix); ok {
				count++
			}
			if tried >= 8 {
				break
			}
		}
		if count > bestCount {
			best, bestCount = key, count
		}
	}
	return best, bestCount > 0
}

// deriveKey turns a keyring password into the AES key Chromium uses on Linux.
// It is PBKDF2-HMAC-SHA1 with the salt "saltysalt" and one iteration.
func deriveKey(password string) []byte {
	m := hmac.New(sha1.New, []byte(password))
	m.Write([]byte("saltysalt"))
	m.Write([]byte{0, 0, 0, 1})
	return m.Sum(nil)[:16]
}

// decrypt decrypts one v10 or v11 cookie value. Since cookie database version
// 24 the plaintext starts with the SHA-256 of the host, which also proves the
// key is right.
func decrypt(key []byte, r chromiumRow, hashPrefix bool) (string, bool) {
	if len(key) != 16 || len(r.encrypted) < 3+aes.BlockSize {
		return "", false
	}
	data := r.encrypted[3:]
	if len(data)%aes.BlockSize != 0 {
		return "", false
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", false
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, bytes.Repeat([]byte{' '}, aes.BlockSize)).CryptBlocks(out, data)
	pad := int(out[len(out)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(out) {
		return "", false
	}
	for _, b := range out[len(out)-pad:] {
		if int(b) != pad {
			return "", false
		}
	}
	out = out[:len(out)-pad]
	if hashPrefix {
		sum := sha256.Sum256([]byte(r.host))
		if len(out) < len(sum) || !bytes.Equal(out[:len(sum)], sum[:]) {
			return "", false
		}
		out = out[len(sum):]
	}
	if !utf8.Valid(out) {
		return "", false
	}
	return string(out), true
}

// chromiumKeys returns the candidate AES keys: the keyring passwords of the
// browser's own applications first, then every other Chromium keyring entry,
// then the fixed key Chromium uses when no keyring is available.
func chromiumKeys(apps []string) [][]byte {
	var keys [][]byte
	seen := map[string]bool{}
	add := func(password string) {
		if seen[password] {
			return
		}
		seen[password] = true
		keys = append(keys, deriveKey(password))
	}
	for _, pw := range keyringLookup(apps) {
		add(pw)
	}
	add("peanuts")
	add("")
	return keys
}

// keyringLookup reads the keyring passwords; tests replace it.
var keyringLookup = keyringPasswords

// keyringPasswords collects Chromium cookie passwords from the Secret Service
// (GNOME Keyring, KeePassXC) and from KWallet.
func keyringPasswords(apps []string) []string {
	var out []string
	out = append(out, secretServicePasswords(apps)...)
	out = append(out, kwalletPasswords(apps)...)
	return out
}

// kwalletEntry returns the KWallet folder and entry of a Chromium app.
func kwalletEntry(app string) (folder, entry string) {
	name := app
	switch strings.ToLower(app) {
	case "chrome":
		name = "Chrome"
	case "chromium":
		name = "Chromium"
	case "brave":
		name = "Brave"
	case "microsoft-edge", "microsoft edge":
		name = "Microsoft Edge"
	case "vivaldi":
		name = "Vivaldi"
	case "opera":
		name = "Opera"
	}
	return name + " Keys", name + " Safe Storage"
}

func openSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteURI(path))
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	return db, nil
}

package browsercookies

import (
	"context"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver
)

const (
	secretsDest = "org.freedesktop.secrets"
	secretsPath = dbus.ObjectPath("/org/freedesktop/secrets")
)

// chromeSchemas are the Secret Service schemas Chromium stores its cookie
// password under.
var chromeSchemas = []string{
	"chrome_libsecret_os_crypt_password_v2",
	"chrome_libsecret_os_crypt_password_v1",
}

// secretValue is the Secret Service Secret struct (oayays).
type secretValue struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

// secretServicePasswords reads the Chromium cookie passwords from the Secret
// Service. The entries of apps come first, then every other Chromium entry.
// It never shows a prompt: a locked item is skipped.
func secretServicePasswords(apps []string) []string {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil
	}
	defer conn.Close()
	svc := conn.Object(secretsDest, secretsPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var output dbus.Variant
	var session dbus.ObjectPath
	if err := svc.CallWithContext(ctx, "org.freedesktop.Secret.Service.OpenSession", 0, "plain", dbus.MakeVariant("")).Store(&output, &session); err != nil {
		return nil
	}
	defer conn.Object(secretsDest, session).CallWithContext(ctx, "org.freedesktop.Secret.Session.Close", 0)

	var queries []map[string]string
	for _, app := range apps {
		queries = append(queries, map[string]string{"application": app})
	}
	for _, schema := range chromeSchemas {
		queries = append(queries, map[string]string{"xdg:schema": schema})
	}

	var out []string
	seen := map[dbus.ObjectPath]bool{}
	for _, attrs := range queries {
		var unlocked, locked []dbus.ObjectPath
		if err := svc.CallWithContext(ctx, "org.freedesktop.Secret.Service.SearchItems", 0, attrs).Store(&unlocked, &locked); err != nil {
			continue
		}
		for _, item := range unlocked {
			if seen[item] {
				continue
			}
			seen[item] = true
			var secret secretValue
			if err := conn.Object(secretsDest, item).CallWithContext(ctx, "org.freedesktop.Secret.Item.GetSecret", 0, session).Store(&secret); err != nil {
				continue
			}
			if pw := strings.TrimSpace(string(secret.Value)); pw != "" {
				out = append(out, pw)
			}
		}
	}
	return out
}

// kwalletPasswords reads the Chromium cookie passwords from KWallet with
// kwallet-query, when it is installed.
func kwalletPasswords(apps []string) []string {
	bin, err := exec.LookPath("kwallet-query")
	if err != nil {
		return nil
	}
	var out []string
	for _, app := range apps {
		folder, entry := kwalletEntry(app)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		data, err := exec.CommandContext(ctx, bin, "--read-password", entry, "--folder", folder, "kdewallet").Output()
		cancel()
		if err != nil {
			continue
		}
		pw := strings.TrimSpace(string(data))
		if pw != "" && !strings.HasPrefix(pw, "Failed to read") {
			out = append(out, pw)
		}
	}
	return out
}

// sqliteURI opens a browser database read-only and immutable, so a running
// browser's lock does not block the read and nothing is ever written.
func sqliteURI(path string) string {
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro&immutable=1"}
	return u.String()
}

package browsercookies

import "fmt"

// readFirefox reads the cookies of a Firefox-family profile. Firefox stores
// cookie values in plain text.
func readFirefox(path string, domains []string) ([]Cookie, error) {
	db, err := openSQLite(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT host, name, value, path, expiry, isSecure, isHttpOnly FROM moz_cookies`)
	if err != nil {
		return nil, fmt.Errorf("read cookies: %w", err)
	}
	defer rows.Close()
	var out []Cookie
	for rows.Next() {
		var c Cookie
		var secure, httpOnly int
		if err := rows.Scan(&c.Host, &c.Name, &c.Value, &c.Path, &c.Expires, &secure, &httpOnly); err != nil {
			return nil, fmt.Errorf("read cookies: %w", err)
		}
		if !matchesAny(c.Host, domains) {
			continue
		}
		// Newer Firefox releases store the expiry in milliseconds.
		if c.Expires > 100_000_000_000 {
			c.Expires /= 1000
		}
		c.Secure, c.HTTPOnly = secure != 0, httpOnly != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

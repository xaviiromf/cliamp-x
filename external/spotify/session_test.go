//go:build !windows

package spotify

import (
	"testing"

	"golang.org/x/oauth2"
)

func TestUsingFallbackToken(t *testing.T) {
	t.Run("no token source", func(t *testing.T) {
		s := &Session{}
		if !s.usingFallbackToken() {
			t.Error("usingFallbackToken() = false, want true with nil tokenSource")
		}
	})

	t.Run("with token source", func(t *testing.T) {
		conf := spotifyOAuthConfig("test-client-id")
		s := &Session{tokenSource: conf.TokenSource(t.Context(), &oauth2.Token{})}
		if s.usingFallbackToken() {
			t.Error("usingFallbackToken() = true, want false with non-nil tokenSource")
		}
	})
}

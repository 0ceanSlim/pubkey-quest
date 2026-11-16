package utils

import (
	"net/http/httptest"
	"testing"
)

// The forwarded address counts only from our own proxy, and only the hop it
// added (beta exploit audit A6).
func TestClientIP(t *testing.T) {
	cases := []struct {
		remote, xff, want string
	}{
		{"203.0.113.9:5000", "127.0.0.1", "203.0.113.9"},             // straight in: the header is the client's own word
		{"127.0.0.1:5000", "127.0.0.1, 198.51.100.7", "198.51.100.7"}, // via the proxy: its hop, not the forged first one
		{"127.0.0.1:5000", "", "127.0.0.1"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = c.remote
		if c.xff != "" {
			r.Header.Set("X-Forwarded-For", c.xff)
		}
		if got := ClientIP(r); got != c.want {
			t.Errorf("%s with %q: %s, want %s", c.remote, c.xff, got, c.want)
		}
	}
}

// Player-given addresses must be public (E4).
func TestPublicURL(t *testing.T) {
	for _, u := range []string{"wss://127.0.0.1/", "ws://localhost:7777", "wss://10.0.0.5", "wss://169.254.169.254/", "wss://[::1]/", "https://relay.example", "wss://printer.local"} {
		if PublicURL(u, "wss", "ws") {
			t.Errorf("%s allowed", u)
		}
	}
	if !PublicURL("wss://203.0.113.9/", "wss") {
		t.Error("a public address refused")
	}
}

package utils

import (
	"reflect"
	"testing"
)

func TestRelayURL(t *testing.T) {
	for in, want := range map[string]string{
		"wss://wheat.oslim.dev/":       "wss://wheat.oslim.dev",
		"wss://Relay.Example.com":      "wss://relay.example.com",
		" wss://relay.example/nostr/ ": "wss://relay.example/nostr",
		"wss://relay.example/#frag":    "wss://relay.example",
	} {
		if got := RelayURL(in); got != want {
			t.Errorf("RelayURL(%q) = %q, want %q", in, got, want)
		}
	}
}

// A relay written with and without its trailing slash is one relay, written
// the way Grain keys it; private hosts stay out.
func TestPublicRelaysNormalizes(t *testing.T) {
	got := PublicRelays([]string{"wss://1.1.1.1/", "wss://1.1.1.1", "ws://127.0.0.1:7777/", "https://1.1.1.1/"})
	if want := []string{"wss://1.1.1.1"}; !reflect.DeepEqual(got, want) {
		t.Errorf("PublicRelays = %v, want %v", got, want)
	}
}

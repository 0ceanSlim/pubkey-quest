package api

import (
	"testing"

	"pubkey-quest/cmd/server/utils"
)

func TestDefaultRelaysOverride(t *testing.T) {
	prev := utils.AppConfig.Relays
	t.Cleanup(func() { utils.AppConfig.Relays = prev })

	utils.AppConfig.Relays.Defaults = nil
	if got := DefaultRelays(); len(got) != len(builtinDefaultRelays) || got[0] != builtinDefaultRelays[0] {
		t.Errorf("no override: want the built-in set, got %v", got)
	}
	utils.AppConfig.Relays.Defaults = []string{"wss://relay.example"}
	if got := DefaultRelays(); len(got) != 1 || got[0] != "wss://relay.example" {
		t.Errorf("config override ignored: got %v", got)
	}
	for _, u := range builtinDefaultRelays {
		if normRelay(u) == "wss://relay.damus.io" {
			t.Error("damus stays out of the default set")
		}
	}
}

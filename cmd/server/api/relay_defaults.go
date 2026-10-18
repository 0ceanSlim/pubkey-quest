package api

import (
	"strings"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/utils"
)

// builtinDefaultRelays is where a player with no relay list of their own (a new
// key, someone who isn't on Nostr yet) keeps their saves: free, open, general
// relays that NIP-66 monitors report healthy (checked 2026-10-18). Without
// these, outbox routing falls back to the indexer relays, which only take
// profiles and relay lists, and the save is refused. The official game relay
// takes the fifth slot once it runs. config.yml `relays.defaults` overrides.
var builtinDefaultRelays = []string{
	"wss://offchain.pub",
	"wss://relay.ditto.pub",
	"wss://relay.nostr.net",
	"wss://nos.lol",
}

// DefaultRelays is the game's default relay set.
func DefaultRelays() []string {
	if d := utils.AppConfig.Relays.Defaults; len(d) > 0 {
		return d
	}
	return builtinDefaultRelays
}

// hasOwnRelays reports whether the player has published write relays (or
// pinned a list) the outbox model can route to.
func hasOwnRelays(uc *core.UserContext, pubkey string) bool {
	if uc != nil && uc.FixedRelaysEnabled() {
		return true
	}
	c := connection.GetCoreClient()
	return c != nil && len(c.ResolveRelays(pubkey).Outbox) > 0
}

// routeWrites is where an event from the signed-in player goes: their outbox
// as usual, or — when they have no relays of their own — the game's default
// set instead of the indexers (which still get profiles and relay lists).
func routeWrites(uc *core.UserContext, ev *nostr.Event) []string {
	relays := uc.RoutePublish(ev)
	if hasOwnRelays(uc, ev.PubKey) {
		return relays
	}
	metadata := ev.Kind == 0 || ev.Kind == 10002 || ev.Kind == 10050
	index := map[string]bool{}
	for _, u := range connection.GetIndexRelays() {
		index[normRelay(u)] = true
	}
	var out []string
	seen := map[string]bool{}
	add := func(u string) {
		if k := normRelay(u); !seen[k] {
			seen[k] = true
			out = append(out, u)
		}
	}
	for _, u := range relays {
		if metadata || !index[normRelay(u)] {
			add(u)
		}
	}
	for _, u := range DefaultRelays() {
		add(u)
	}
	return out
}

func normRelay(u string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(u)), "/")
}

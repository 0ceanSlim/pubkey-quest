// Package dm is the Dungeon Master: the server's own Nostr identity
// (docs/draft/server-stats.md §6). On the official server he signs and
// publishes what play earns: badge definitions and awards (NIP-58), scores
// (Gamestr), and each player's lifetime record, mirrored to relays as its
// store of record (competition-plan.md).
//
// He signs nothing anywhere else. Test shares his key but never has
// saves.validation on, so what's earned there stays on the server.
package dm

import (
	"context"
	"log"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core"
	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/cmd/server/utils"
)

// ScoreRelays are where Gamestr reads scores, besides the DM's own relays.
var ScoreRelays = []string{"wss://relay.gamestr.io"}

var (
	signer *core.EventSigner
	pubHex string
)

// publishFn sends a signed event to relays and returns those that took it
// (the server's Grain client; tests replace it).
var publishFn = func(ctx context.Context, ev *nostr.Event, relays []string) []string {
	c := connection.GetCoreClient()
	if c == nil {
		return nil
	}
	results, err := c.PublishEvent(ctx, ev, utils.PublicRelays(relays))
	if err != nil {
		return nil
	}
	var ok []string
	for _, r := range results {
		if r.Success {
			ok = append(ok, r.RelayURL)
		}
	}
	return ok
}

// Init loads the DM's key from config. On the official server (stats on and
// saves.validation on) he starts signing: the badge definitions now, then a
// sweep that publishes whatever's waiting. It reports whether he signs.
func Init() bool {
	priv, pub, ok, err := utils.AppConfig.DM.DMKey()
	if err != nil || !ok {
		return false
	}
	if !stats.Enabled || !utils.AppConfig.Saves.Validation {
		return false
	}
	s, err := core.NewEventSigner(priv)
	if err != nil {
		log.Printf("❌ Dungeon Master: %v", err)
		return false
	}
	signer, pubHex = s, pub
	stats.Rebuild = rebuild
	go func() {
		PublishDefinitions()
		for {
			SweepAll()
			time.Sleep(10 * time.Minute)
		}
	}()
	log.Printf("✅ The Dungeon Master signs badges and scores")
	return true
}

// Signs reports whether the DM signs and publishes on this server.
func Signs() bool { return signer != nil }

// Pubkey is the DM's public key (hex), "" when he doesn't sign here.
func Pubkey() string { return pubHex }

// relays are the DM's own relays (config dm.relays).
func relays() []string { return utils.AppConfig.DM.Relays }

// send signs ev as the DM and publishes it to his relays plus extra. It
// returns the event's id once a relay has it, "" if none took it.
func send(ev *nostr.Event, extra ...string) string {
	if signer == nil {
		return ""
	}
	if err := signer.SignEvent(ev); err != nil {
		log.Printf("⚠️ Dungeon Master: signing kind %d: %v", ev.Kind, err)
		return ""
	}
	targets := append(append([]string{}, relays()...), extra...)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if len(publishFn(ctx, ev, dedupe(targets))) == 0 {
		return ""
	}
	return ev.ID
}

func dedupe(urls []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, u := range urls {
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	return out
}

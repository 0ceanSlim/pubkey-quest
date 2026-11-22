package dm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/0ceanslim/grain/client/connection"
	"github.com/0ceanslim/grain/client/core/tools"
	nostr "github.com/0ceanslim/grain/server/types"

	"pubkey-quest/cmd/server/game/badges"
	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/cmd/server/utils"
	"pubkey-quest/types"
)

// sweepMu keeps one sweep at a time, so nothing is signed twice.
var sweepMu sync.Mutex

// SweepAll publishes whatever's waiting for every player.
func SweepAll() {
	for _, npub := range stats.Players() {
		SweepPlayer(npub)
	}
}

// SweepPlayer publishes what's waiting for one player: the awards of badges
// they've earned, their scores that changed, and their record if it changed
// since it was last mirrored. Whatever no relay takes waits for the next
// sweep.
func SweepPlayer(npub string) {
	if !Signs() {
		return
	}
	sweepMu.Lock()
	defer sweepMu.Unlock()
	player, err := tools.DecodeNpub(npub)
	if err != nil {
		return
	}
	r, err := stats.Load(npub)
	if err != nil {
		log.Printf("⚠️ Dungeon Master: %s's record: %v", npub, err)
		return
	}

	awarded := map[string]string{}
	for id := range r.Badges {
		if r.Awarded[id] == "" {
			if ev := send(Award(id, pubHex, player), inboxOf(player)...); ev != "" {
				awarded[id] = ev
			}
		}
	}
	// The player's one Gamestr score, once their scores have changed.
	scored, key := "", ScoreKey(r)
	if len(r.Scores) > 0 && key != r.Scored {
		if r.Gamestr == "" {
			retireBoards(r, player)
		}
		scored = send(Score(r, player), ScoreRelays...)
	}
	if len(awarded) > 0 || scored != "" {
		stats.Note(npub, func(r *stats.Record) {
			if r.Awarded == nil {
				r.Awarded = map[string]string{}
			}
			for id, ev := range awarded {
				r.Awarded[id] = ev
			}
			if scored != "" {
				r.Gamestr = scored
				r.Scored = key // a score that changed since waits for the next sweep
			}
		})
	}

	// The record, mirrored, once it has changed.
	r, err = stats.Load(npub)
	if err != nil || r.Updated <= r.Mirrored {
		return
	}
	ev, err := Mirror(r, player)
	if err != nil || send(ev) == "" {
		return
	}
	stats.Note(npub, func(rec *stats.Record) {
		if rec.Updated == r.Updated {
			rec.Mirrored = r.Updated
		}
	})
}

// retireBoards takes down the scores the DM posted a board at a time, before
// one per player, so Gamestr doesn't rank them against each other.
func retireBoards(r stats.Record, player string) {
	for board, s := range r.Scores {
		if s.Event != "" && send(Retired(board, s, player), ScoreRelays...) == "" {
			log.Printf("⚠️ Dungeon Master: the old %s score %s reached no relay", board, s.Event)
		}
	}
}

// inboxOf is a player's read relays, where their awards are delivered.
func inboxOf(player string) []string {
	if c := connection.GetCoreClient(); c != nil {
		if ur := c.ResolveRelays(player); ur != nil {
			return ur.Inbox
		}
	}
	return nil
}

// rebuild fetches a player's record from relays when the server has no copy
// (stats.Rebuild): the newest the DM published.
func rebuild(npub string) (stats.Record, bool) {
	var r stats.Record
	player, err := tools.DecodeNpub(npub)
	c := connection.GetCoreClient()
	if err != nil || c == nil || pubHex == "" {
		return r, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	filter := nostr.Filter{Authors: []string{pubHex}, Kinds: []int{KindAppData}, Tags: map[string][]string{"d": {recordD(player)}}}
	var newest *nostr.Event
	for ev := range c.StreamEvents(ctx, filter, utils.PublicRelays(relays())) {
		if ev.PubKey == pubHex && (newest == nil || ev.CreatedAt > newest.CreatedAt) {
			newest = ev
		}
	}
	if newest == nil || json.Unmarshal([]byte(newest.Content), &r) != nil {
		return stats.Record{}, false
	}
	r.Mirrored = r.Updated // it came from the relays
	log.Printf("📂 Rebuilt %s's record from the Dungeon Master's relays", npub)
	return r, true
}

// DefinitionsFile remembers which badge definitions are published, so only a
// new or changed badge is signed again.
var DefinitionsFile = filepath.Join("data", "dm", "definitions.json")

// PublishDefinitions signs and publishes each badge's definition that's new
// or changed since it was last published.
func PublishDefinitions() {
	if !Signs() {
		return
	}
	published := map[string]string{} // badge id → hash of what was published
	if data, err := os.ReadFile(DefinitionsFile); err == nil {
		json.Unmarshal(data, &published)
	}
	media := badges.Media().Badges
	changed, failed := 0, 0
	for _, b := range badges.Config().Badges {
		h := definitionHash(b, media[b.ID])
		if published[b.ID] == h {
			continue
		}
		if send(Definition(b, media[b.ID])) != "" {
			published[b.ID] = h
			changed++
		} else {
			failed++
		}
	}
	if failed > 0 {
		log.Printf("⚠️ Dungeon Master: %d badge definition(s) reached no relay; retrying at the next sweep", failed)
	}
	if changed == 0 {
		return
	}
	if data, err := json.Marshal(published); err == nil {
		os.MkdirAll(filepath.Dir(DefinitionsFile), 0o755)
		os.WriteFile(DefinitionsFile, data, 0o644)
	}
	log.Printf("🏅 The Dungeon Master published %d badge definition(s)", changed)
}

func definitionHash(b types.BadgeDef, m types.BadgeMedia) string {
	data, _ := json.Marshal([]string{b.Name, b.Description, m.Image, m.Thumb256, m.Thumb64})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

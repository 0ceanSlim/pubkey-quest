package world

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pubkey-quest/types"
)

// ─── The ground ─────────────────────────────────────────────────────────────
//
// What's dropped lies where it fell, for anyone there to take
// (docs/draft/shared-ground.md). Two stores hold it:
//
//   - a session's own GroundStore: what that player has dropped since they
//     last saved. Only they see it. Their save still holds the item, so it
//     goes when the play it came from goes: a reload, quitting without saving.
//   - World: everyone's. A save moves the saver's own drops here (Publish),
//     as the save no longer holds them. Piles despawn DespawnAfter after
//     they were left, and a spot keeps at most MaxPilesPerSpot.
//
// Both are server-authoritative: pickup takes only what's really there.

// GroundDrop is a pile on the ground as the client sees it.
type GroundDrop struct {
	ID       string `json:"id"`
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
	// State is the item's condition (a food's made-time); a pile with state
	// never merges with another.
	State map[string]interface{} `json:"state,omitempty"`
	// Yours: your own drop since you last saved, seen by no one else yet.
	Yours bool `json:"yours,omitempty"`
}

// Pile is a pile and where it lies.
type Pile struct {
	GroundDrop
	Key string    `json:"key"`
	Pos float64   `json:"pos,omitempty"` // position along a road, 0–1 from its first end
	At  time.Time `json:"at"`            // when it was left (made public)
	// Owner: an npub only it shows to (a quest item); "" for anyone.
	Owner string `json:"owner,omitempty"`
	seq   int
}

// Spot is a place on the ground: a town spot (location|district|building|room,
// as presence names it) or, on a road, the road and a position along it.
type Spot struct {
	Key  string
	Road bool
	Pos  float64
}

// RoadReach is how far along a road (a fraction of it) you see what's lying
// there: presence's RoadNearby, the distance travellers see each other.
const RoadReach = 0.05

// Despawning and crowding.
const (
	DespawnAfter    = 2 * time.Hour
	MaxPilesPerSpot = 60
)

// GroundKey is the raw place key of a save (location|district|building|room).
// The ground itself is keyed by Spot, which fills in a building's default
// room and places a traveller along the road.
func GroundKey(state *types.SaveFile) string {
	if state == nil {
		return ""
	}
	return strings.Join([]string{state.Location, state.District, state.Building, state.Room}, "|")
}

// GroundStore is a set of piles. Its zero value is not usable; use
// NewGroundStore.
type GroundStore struct {
	mu     sync.Mutex
	prefix string // pile ids: "m" a session's own, "g" the world's
	seq    int
	sealed int // Mark's last answer: piles up to it belong to a save in hand
	piles  []Pile
	dirty  bool
}

// NewGroundStore returns an empty store of a session's own drops.
func NewGroundStore() *GroundStore { return newStore("m") }

func newStore(prefix string) *GroundStore { return &GroundStore{prefix: prefix} }

// NewSharedGround returns an empty shared ground (World is the one in play).
func NewSharedGround() *GroundStore { return newStore("g") }

// World is the shared ground.
var World = NewSharedGround()

// at reports whether a pile lies at spot.
func (p *Pile) at(s Spot) bool {
	if p.Key != s.Key {
		return false
	}
	if !s.Road {
		return true
	}
	d := p.Pos - s.Pos
	return d >= -RoadReach && d <= RoadReach
}

// sees reports whether npub may see and take a pile.
func (p *Pile) sees(npub string) bool { return p.Owner == "" || p.Owner == npub }

// Add drops quantity of itemID at spot, stacking onto a pile of the same
// item lying in the same place.
func (g *GroundStore) Add(s Spot, itemID string, quantity int) {
	g.AddWithState(s, itemID, quantity, nil)
}

// AddWithState drops items that carry condition: they get a pile of their own
// (only stateless piles of the same item stack).
func (g *GroundStore) AddWithState(s Spot, itemID string, quantity int, state map[string]interface{}) {
	if g == nil || itemID == "" || quantity <= 0 || s.Key == "" {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	if len(state) == 0 {
		for i := range g.piles {
			p := &g.piles[i]
			if p.Key == s.Key && p.Pos == s.Pos && p.Item == itemID && len(p.State) == 0 && p.Owner == "" && p.seq > g.sealed {
				p.Quantity += quantity
				p.seq = g.seq // a save after this covers the whole pile
				g.dirty = true
				return
			}
		}
	}
	g.put(Pile{GroundDrop: GroundDrop{Item: itemID, Quantity: quantity, State: state},
		Key: s.Key, Pos: s.Pos, At: time.Now()})
}

// put adds a pile with a fresh id (g.mu held).
func (g *GroundStore) put(p Pile) {
	p.ID = fmt.Sprintf("%s%d", g.prefix, g.seq)
	p.seq = g.seq
	g.piles = append(g.piles, p)
	g.dirty = true
	// A crowded spot loses its oldest pile.
	n, oldest := 0, -1
	for i := range g.piles {
		if g.piles[i].Key == p.Key {
			if n++; oldest < 0 || g.piles[i].At.Before(g.piles[oldest].At) {
				oldest = i
			}
		}
	}
	if n > MaxPilesPerSpot {
		g.piles = append(g.piles[:oldest], g.piles[oldest+1:]...)
	}
}

// List returns the piles at spot that npub sees, oldest first. Expired piles
// are swept first.
func (g *GroundStore) List(s Spot, npub string) []GroundDrop {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sweep(time.Now())
	var out []GroundDrop
	for i := range g.piles {
		if p := &g.piles[i]; p.at(s) && p.sees(npub) {
			d := p.GroundDrop
			d.Yours = g.prefix == "m" // a session's own drops
			out = append(out, d)
		}
	}
	return out
}

// Take removes up to quantity (all of it: quantity < 0) from a pile at spot
// that npub sees: the pile with id pileID, or else the first of itemID. It
// returns the pile as it was and how many were taken (0: none there).
func (g *GroundStore) Take(s Spot, npub, pileID, itemID string, quantity int) (Pile, int) {
	if g == nil || quantity == 0 {
		return Pile{}, 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sweep(time.Now())
	for i := range g.piles {
		p := &g.piles[i]
		if !p.at(s) || !p.sees(npub) || (pileID != "" && p.ID != pileID) || (pileID == "" && p.Item != itemID) {
			continue
		}
		taken := p.Quantity
		if quantity > 0 && quantity < taken {
			taken = quantity
		}
		was := *p
		if p.Quantity -= taken; p.Quantity <= 0 {
			g.piles = append(g.piles[:i], g.piles[i+1:]...)
		}
		g.dirty = true
		return was, taken
	}
	return Pile{}, 0
}

// Return puts back what was taken but couldn't be carried, as the pile it
// came from: same place, same age, same owner.
func (g *GroundStore) Return(from Pile, quantity int) {
	if g == nil || quantity <= 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.seq++
	from.Quantity = quantity
	g.put(from)
	if last := &g.piles[len(g.piles)-1]; from.seq > 0 && last.seq == g.seq {
		last.seq = from.seq // still covered by the save it was dropped before
	}
}

// sweep drops expired piles (g.mu held).
func (g *GroundStore) sweep(now time.Time) {
	kept := g.piles[:0]
	for _, p := range g.piles {
		if now.Sub(p.At) < DespawnAfter {
			kept = append(kept, p)
		} else {
			g.dirty = true
		}
	}
	g.piles = kept
}

// Mark is where the store stands now: a save made now covers every drop up
// to it. Nothing is added to those piles after (a later drop makes its own),
// so a save still being signed publishes just what it left out.
func (g *GroundStore) Mark() int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sealed = g.seq
	return g.seq
}

// Drain removes and returns the piles dropped up to mark.
func (g *GroundStore) Drain(mark int) []Pile {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []Pile
	kept := g.piles[:0]
	for _, p := range g.piles {
		if p.seq <= mark {
			out = append(out, p)
		} else {
			kept = append(kept, p)
		}
	}
	g.piles = kept
	return out
}

// Publish lays piles on the shared ground as of now: a save no longer holds
// them.
func (g *GroundStore) Publish(piles []Pile) {
	if g == nil || len(piles) == 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	for _, p := range piles {
		g.seq++
		p.Yours, p.At = false, now
		g.put(p)
	}
}

// Snapshot copies every pile (a session's journal keeps its own drops).
func (g *GroundStore) Snapshot() []Pile {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]Pile, len(g.piles))
	copy(out, g.piles)
	return out
}

// Restore puts back piles from a snapshot, as they were.
func (g *GroundStore) Restore(piles []Pile) {
	if g == nil || len(piles) == 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.restore(piles)
}

// restore puts piles back (g.mu held).
func (g *GroundStore) restore(piles []Pile) {
	for _, p := range piles {
		g.seq++
		g.put(p)
	}
}

// GroundFile is where the world's ground is kept across restarts.
var GroundFile = "data/ground.json"

// SaveWorldGround writes the world's ground to GroundFile if it changed.
// It's written after the sessions' journals (session.JournalAllSessions): a
// crash between the two can lose a pile, never make two of one.
func SaveWorldGround() {
	World.mu.Lock()
	if !World.dirty {
		World.mu.Unlock()
		return
	}
	World.sweep(time.Now())
	data, err := json.Marshal(World.piles)
	World.dirty = false
	World.mu.Unlock()
	if err != nil {
		log.Printf("⚠️ Failed to encode the ground: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(GroundFile), 0o755); err == nil {
		tmp := GroundFile + ".tmp"
		if err = os.WriteFile(tmp, data, 0o644); err == nil {
			err = os.Rename(tmp, GroundFile)
		}
		if err != nil {
			log.Printf("⚠️ Failed to keep the ground: %v", err)
		}
	}
}

// LoadWorldGround reads the world's ground back from GroundFile, if there is one.
func LoadWorldGround() {
	data, err := os.ReadFile(GroundFile)
	if err != nil {
		return
	}
	var piles []Pile
	if err := json.Unmarshal(data, &piles); err != nil {
		log.Printf("⚠️ Couldn't read %s, the ground starts empty: %v", GroundFile, err)
		return
	}
	World.mu.Lock()
	World.piles = nil
	World.restore(piles)
	World.sweep(time.Now())
	World.dirty = false
	World.mu.Unlock()
	log.Printf("📂 %d pile(s) on the ground", len(World.piles))
}

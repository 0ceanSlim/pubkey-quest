package world

import (
	"strings"
	"sync"

	"pubkey-quest/types"
)

// GroundDrop is a stack of a dropped item lying on the ground at one location.
type GroundDrop struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
}

// GroundStore holds items the player has dropped, keyed by location, for a single
// session. It is memory-only and never persisted — items left on the ground vanish
// when the session ends (you left them behind, by design). It is authoritative: the
// server owns what's on the ground, so pickup can't be spoofed by the client (the
// old flow destroyed the item on drop and re-spawned it via the debug add_item
// action, which lost items on reload and let the client conjure anything).
type GroundStore struct {
	mu    sync.Mutex
	piles map[string][]GroundDrop
}

// NewGroundStore returns an empty ground store.
func NewGroundStore() *GroundStore {
	return &GroundStore{piles: make(map[string][]GroundDrop)}
}

// GroundKey identifies the player's current spot (city/district/building/room, or
// the environment while travelling) so a drop is picked up where it was left.
func GroundKey(state *types.SaveFile) string {
	if state == nil {
		return ""
	}
	return strings.Join([]string{state.Location, state.District, state.Building, state.Room}, "|")
}

// Add drops quantity of itemID at key, stacking onto an existing pile of the same item.
func (g *GroundStore) Add(key, itemID string, quantity int) {
	if g == nil || itemID == "" || quantity <= 0 {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	pile := g.piles[key]
	for i := range pile {
		if pile[i].Item == itemID {
			pile[i].Quantity += quantity
			g.piles[key] = pile
			return
		}
	}
	g.piles[key] = append(pile, GroundDrop{Item: itemID, Quantity: quantity})
}

// Take removes up to quantity of itemID at key and returns how many were actually
// removed (0 if none are on the ground there).
func (g *GroundStore) Take(key, itemID string, quantity int) int {
	if g == nil || itemID == "" || quantity <= 0 {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	pile := g.piles[key]
	for i := range pile {
		if pile[i].Item == itemID {
			taken := quantity
			if taken > pile[i].Quantity {
				taken = pile[i].Quantity
			}
			pile[i].Quantity -= taken
			if pile[i].Quantity <= 0 {
				pile = append(pile[:i], pile[i+1:]...)
			}
			g.piles[key] = pile
			return taken
		}
	}
	return 0
}

// List returns a copy of the drops at key (nil-safe).
func (g *GroundStore) List(key string) []GroundDrop {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	pile := g.piles[key]
	out := make([]GroundDrop, len(pile))
	copy(out, pile)
	return out
}

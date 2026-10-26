// Package condition is item condition (docs/draft/degradation-system.md): state
// that changes as an item is used or as time passes, kept on the item's
// inventory slot next to "item" and "quantity". Food spoils (a timestamp), and
// later weapons and armor wear, lights burn and charges run down (counters).
//
// Condition can't be derived from game data, so it lives in the save (the
// codec keeps extra slot fields). A missing field means new: pristine, full,
// or, for food, "made the first time we see it".
//
// An item that carries condition never stacks: one item per slot, so moving,
// splitting and banking never have to merge two states.
package condition

import (
	"encoding/json"
	"sync"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// Slot keys. Short, since every one is written into saves.
const (
	KeyMade = "t" // food: the absolute game minute it was made
	KeyWear = "w" // weapons, armor: wear points taken (0 = pristine)
	KeyBurn = "b" // a lit light: burn minutes left
	KeyUses = "u" // charges: uses spent (missing = full)
)

// StateKeys are every condition key a slot can carry.
var StateKeys = []string{KeyMade, KeyWear, KeyBurn, KeyUses}

// SpoiledFood is the one item every spoiled food becomes (it stacks, no state).
const SpoiledFood = "spoiled-food"

// Now is the save's absolute game minute (CurrentDay*1440 + TimeOfDay), the
// clock food timestamps are written in.
func Now(save *types.SaveFile) int {
	return save.CurrentDay*1440 + save.TimeOfDay
}

// HasState reports whether a slot carries any condition.
func HasState(slot map[string]interface{}) bool {
	for _, k := range StateKeys {
		if _, ok := slot[k]; ok {
			return true
		}
	}
	return false
}

// State returns a copy of a slot's condition fields (nil when it has none).
func State(slot map[string]interface{}) map[string]interface{} {
	var out map[string]interface{}
	for _, k := range StateKeys {
		if v, ok := slot[k]; ok {
			if out == nil {
				out = map[string]interface{}{}
			}
			out[k] = v
		}
	}
	return out
}

// CopyState copies src's condition fields onto dst, replacing any dst had.
func CopyState(dst, src map[string]interface{}) {
	ClearState(dst)
	for k, v := range State(src) {
		dst[k] = v
	}
}

// ApplyState writes a saved state map (from State, a vault entry or a ground
// item) onto a slot, replacing any it had.
func ApplyState(slot, state map[string]interface{}) {
	ClearState(slot)
	for _, k := range StateKeys {
		if v, ok := state[k]; ok {
			slot[k] = v
		}
	}
}

// ClearState removes every condition field from a slot: an emptied slot must
// not keep the last item's freshness or wear.
func ClearState(slot map[string]interface{}) {
	for _, k := range StateKeys {
		delete(slot, k)
	}
}

// ─── item data ────────────────────────────────────────────────────────────────

// Info is what condition needs to know about an item, from its game data.
type Info struct {
	// Name is the item's display name (its id if it has none).
	Name string
	// ShelfLife is how long food keeps, in minutes (0: it never spoils).
	ShelfLife int
	// Vanish: when its time is up it disappears rather than spoiling
	// (goodberries lose their magic).
	Vanish bool
}

// Stateful reports whether an item carries condition, and so stacks 1.
func (i Info) Stateful() bool { return i.ShelfLife > 0 }

var (
	infoMu    sync.RWMutex
	infoCache = map[string]Info{}
)

// ItemInfo reads (and caches) an item's condition data. Items are fixed for a
// server's lifetime (a migration means a restart), so the cache never goes
// stale; ResetCache is for tests that rebuild the database.
func ItemInfo(itemID string) Info {
	infoMu.RLock()
	info, ok := infoCache[itemID]
	infoMu.RUnlock()
	if ok {
		return info
	}
	info.Name = itemID
	if item, err := db.GetItemByID(itemID); err == nil && item != nil {
		if item.Name != "" {
			info.Name = item.Name
		}
		var props struct {
			Spoils *struct {
				ShelfLife int  `json:"shelf_life"`
				Vanish    bool `json:"vanish"`
			} `json:"spoils"`
		}
		if json.Unmarshal([]byte(item.Properties), &props) == nil && props.Spoils != nil {
			info.ShelfLife, info.Vanish = props.Spoils.ShelfLife, props.Spoils.Vanish
		}
	}
	infoMu.Lock()
	infoCache[itemID] = info
	infoMu.Unlock()
	return info
}

// ResetCache forgets the cached item data.
func ResetCache() {
	infoMu.Lock()
	infoCache = map[string]Info{}
	infoMu.Unlock()
	ResetLightCache()
	ResetWearCache()
	ResetRepairCache()
	ResetChargeCache()
}

// ItemName is an item's display name.
func ItemName(itemID string) string { return ItemInfo(itemID).Name }

// Stateful reports whether an item carries condition (and so stacks 1).
func Stateful(itemID string) bool {
	return itemID != "" && ItemInfo(itemID).Stateful()
}

// CanMerge reports whether two slots' items may share a stack: never when
// either carries condition (one item per slot, so there's nothing to merge).
func CanMerge(a, b map[string]interface{}) bool {
	id, _ := a["item"].(string)
	return !Stateful(id) && !HasState(a) && !HasState(b)
}

// NewState is the condition a brand-new item starts with: food is made now.
// Nil when the item carries none.
func NewState(itemID string, now int) map[string]interface{} {
	if ItemInfo(itemID).ShelfLife > 0 {
		return map[string]interface{}{KeyMade: now}
	}
	return nil
}

// intField reads a numeric slot field (JSON numbers arrive as float64).
func intField(slot map[string]interface{}, key string) (int, bool) {
	switch v := slot[key].(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case int64:
		return int(v), true
	}
	return 0, false
}

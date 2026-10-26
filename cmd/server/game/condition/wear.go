package condition

import (
	"encoding/json"
	"sync"

	"pubkey-quest/cmd/server/db"
)

// Wear (degradation §4): weapons wear as they deal damage and armour as it's
// hit. Wear never breaks anything; its tier cuts a weapon's damage and an
// armour piece's AC, and what it sells for. The slot's "w" counts the wear
// points taken (missing: pristine); an item's durability is its
// "durability_max" (see MaxDurability).

// Tier is how worn an item is.
type Tier int

const (
	Pristine Tier = iota // 75–100% left: full effect
	Worn                 // 50–75%: −10% damage, −1 AC
	Damaged              // 25–50%: −25% damage, −2 AC
	Ruined               // 0–25%: −50% damage, half its AC
)

var tierNames = [...]string{"Pristine", "Worn", "Damaged", "Ruined"}

func (t Tier) String() string { return tierNames[t] }

var (
	maxMu    sync.RWMutex
	maxCache = map[string]int{}
)

// MaxDurability is how much wear an item takes from pristine to ruined, or 0
// when it doesn't wear. The codex migration works it out from the item and the
// category defaults (types.DurabilityFor) and stores it as "durability_max".
func MaxDurability(itemID string) int {
	if itemID == "" {
		return 0
	}
	maxMu.RLock()
	n, ok := maxCache[itemID]
	maxMu.RUnlock()
	if ok {
		return n
	}
	n = 0
	if item, err := db.GetItemByID(itemID); err == nil && item != nil {
		var props struct {
			DurabilityMax int `json:"durability_max"`
		}
		if json.Unmarshal([]byte(item.Properties), &props) == nil {
			n = props.DurabilityMax
		}
	}
	maxMu.Lock()
	maxCache[itemID] = n
	maxMu.Unlock()
	return n
}

// ResetWearCache forgets cached durability (tests).
func ResetWearCache() {
	maxMu.Lock()
	maxCache = map[string]int{}
	maxMu.Unlock()
}

// WearOf is the wear a slot's item has taken.
func WearOf(slot map[string]interface{}) int {
	n, _ := intField(slot, KeyWear)
	return n
}

// WearTier is how worn a slot's item is, and the share of its durability left
// (1 for anything that doesn't wear).
func WearTier(slot map[string]interface{}) (Tier, float64) {
	id, _ := slot["item"].(string)
	max := MaxDurability(id)
	if max <= 0 {
		return Pristine, 1
	}
	left := 1 - float64(min(WearOf(slot), max))/float64(max)
	switch {
	case left >= 0.75:
		return Pristine, left
	case left >= 0.5:
		return Worn, left
	case left >= 0.25:
		return Damaged, left
	}
	return Ruined, left
}

// AddWear wears a slot's item by n points (up to its durability). It returns
// the tier before and after, so the caller can say when it drops.
func AddWear(slot map[string]interface{}, n int) (before, after Tier) {
	id, _ := slot["item"].(string)
	max := MaxDurability(id)
	before, _ = WearTier(slot)
	if max <= 0 || n <= 0 {
		return before, before
	}
	slot[KeyWear] = min(WearOf(slot)+n, max)
	after, _ = WearTier(slot)
	return before, after
}

// DamagePercent is the share of its damage a weapon in this tier deals.
func DamagePercent(t Tier) int {
	return [...]int{100, 90, 75, 50}[t]
}

// ACPenalty is what an armour piece in this tier loses from its AC value.
func ACPenalty(t Tier, value int) int {
	switch t {
	case Worn:
		return min(1, value)
	case Damaged:
		return min(2, value)
	case Ruined:
		return value / 2
	}
	return 0
}

// WearSellPercent is the share of its price a worn item sells for.
func WearSellPercent(t Tier) int {
	return [...]int{100, 75, 50, 25}[t]
}

// SetWear sets a slot's wear (0 makes it pristine).
func SetWear(slot map[string]interface{}, n int) {
	if n <= 0 {
		delete(slot, KeyWear)
		return
	}
	slot[KeyWear] = n
}

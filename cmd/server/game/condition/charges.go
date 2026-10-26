package condition

import (
	"encoding/json"
	"sync"

	"pubkey-quest/cmd/server/db"
)

// Charges (degradation §7): an item with "charges": {"max": N, "refill": kind}
// has N uses. The slot's "u" counts the uses spent (missing: full). Without a
// refill it's used up on its last charge (a bottle of ink); with one it's just
// empty until refilled (a waterskin, refill "water").

// ChargeInfo is an item's charges block.
type ChargeInfo struct {
	Max    int    `json:"max"`
	Refill string `json:"refill,omitempty"`
	Noun   string `json:"noun,omitempty"` // "drinks", "scrolls", "sharpenings"
}

var (
	chargeMu    sync.RWMutex
	chargeCache = map[string]*ChargeInfo{}
)

// ChargesOf is an item's charges block, nil when it has none.
func ChargesOf(itemID string) *ChargeInfo {
	chargeMu.RLock()
	c, ok := chargeCache[itemID]
	chargeMu.RUnlock()
	if ok {
		return c
	}
	c = nil
	if item, err := db.GetItemByID(itemID); err == nil && item != nil {
		var props struct {
			Charges *ChargeInfo `json:"charges"`
		}
		if json.Unmarshal([]byte(item.Properties), &props) == nil && props.Charges != nil && props.Charges.Max > 0 {
			c = props.Charges
		}
	}
	chargeMu.Lock()
	chargeCache[itemID] = c
	chargeMu.Unlock()
	return c
}

// ResetChargeCache forgets cached charges (tests).
func ResetChargeCache() {
	chargeMu.Lock()
	chargeCache = map[string]*ChargeInfo{}
	chargeMu.Unlock()
}

// ChargesLeft is how many uses a slot's item has left (0 for no charges).
func ChargesLeft(slot map[string]interface{}) int {
	id, _ := slot["item"].(string)
	c := ChargesOf(id)
	if c == nil {
		return 0
	}
	spent, _ := intField(slot, KeyUses)
	return max(c.Max-spent, 0)
}

// UseCharge spends one of a slot's charges. It returns the uses left, and
// usedUp when an item that can't be refilled has none left (the caller
// removes it).
func UseCharge(slot map[string]interface{}) (left int, usedUp bool) {
	id, _ := slot["item"].(string)
	c := ChargesOf(id)
	if c == nil {
		return 0, false
	}
	spent, _ := intField(slot, KeyUses)
	spent = min(spent+1, c.Max)
	slot[KeyUses] = spent
	left = c.Max - spent
	return left, left == 0 && c.Refill == ""
}

// RefillCharges fills a slot's item back up.
func RefillCharges(slot map[string]interface{}) { delete(slot, KeyUses) }

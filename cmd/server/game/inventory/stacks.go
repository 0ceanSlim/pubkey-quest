package inventory

import (
	"encoding/json"
	"log"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/types"
)

// SplitStacks breaks up carried stacks bigger than their item now stacks to.
// Items that gained a condition went to stack 1 (raw and large fish, the
// whetstone: degradation §2), and saves from before still hold them in
// stacks. On load the extras move into empty slots, the same container
// first, then the belt, then the backpack, each keeping the stack's
// condition. When there's no room the rest stay where they are. Reports
// whether it changed anything.
func SplitStacks(save *types.SaveFile) bool {
	general, _ := save.Inventory["general_slots"].([]interface{})
	var pack []interface{}
	if gear, ok := save.Inventory["gear_slots"].(map[string]interface{}); ok {
		if b, ok := gear["bag"].(map[string]interface{}); ok {
			pack, _ = b["contents"].([]interface{})
		}
	}
	changed := false
	for _, slots := range gameutil.CarriedSlotArrays(save) {
		for _, raw := range slots {
			slot, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := slot["item"].(string)
			qty := GetSlotQuantity(slot)
			if id == "" || qty <= 1 {
				continue
			}
			limit := stackLimit(id)
			if qty <= limit {
				continue
			}
			extra := qty - limit
			for extra > 0 {
				n := min(extra, limit)
				if !placeSplit(slot, id, n, slots, general, pack) {
					break
				}
				extra -= n
			}
			slot["quantity"] = limit + extra
			changed = true
			if extra > 0 {
				log.Printf("⚠️ %d %s stacked past %d, no room to split %d", qty, id, limit, extra)
			} else {
				log.Printf("🔧 Split a stack of %d %s into stacks of %d", qty, id, limit)
			}
		}
	}
	return changed
}

// stackLimit is how many of an item fit in one slot: its "stack", or 1 when
// it carries a condition (as AddItemToInventory has it).
func stackLimit(itemID string) int {
	if info := condition.ItemInfo(itemID); info.Stateful() && !info.Vanish {
		return 1
	}
	limit := 1
	if item, err := db.GetItemByID(itemID); err == nil && item.Properties != "" {
		var props map[string]interface{}
		if json.Unmarshal([]byte(item.Properties), &props) == nil {
			if v, ok := props["stack"].(float64); ok && v > 1 {
				limit = int(v)
			}
		}
	}
	return limit
}

// placeSplit puts n of a stack into the first empty slot of the arrays given,
// with the stack's condition.
func placeSplit(from map[string]interface{}, itemID string, n int, arrays ...[]interface{}) bool {
	for _, slots := range arrays {
		for i, raw := range slots {
			s, ok := raw.(map[string]interface{})
			if raw != nil && !ok {
				continue
			}
			if s != nil {
				if id, _ := s["item"].(string); id != "" {
					continue
				}
			}
			moved := map[string]interface{}{"item": itemID, "quantity": n}
			if s != nil {
				if idx, ok := s["slot"]; ok {
					moved["slot"] = idx
				}
			}
			condition.CopyState(moved, from)
			slots[i] = moved
			return true
		}
	}
	return false
}

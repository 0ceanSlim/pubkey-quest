package character

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"

	gameinventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// ─── Claiming the starting gear in game (docs/draft/intro-and-tutorial.md) ───
//
// A new character's gear used to be picked in the intro and posted with the
// creation request. It's now picked in game, from their vault's keeper, and
// claimed here once. The choices arrive in the same shape the intro sent
// ("choice-N" → an item id, or a JSON list of [item, quantity] for bundles and
// weapon picks), but unlike the old creation path every pick is checked
// against the class's options in starting-gear.json: a claim can only ever
// hand out what the class was offered.

// ResolveStartingGear checks a class's picks and returns every item they come
// to: the given items, one option per choice, and the pack.
func ResolveStartingGear(gear *StartingGearData, choices map[string]string, pack string) ([]ItemWithQty, error) {
	items := append([]ItemWithQty{}, gear.StartingGear.GivenItems...)
	for i, choice := range gear.StartingGear.EquipmentChoices {
		picked := choices[fmt.Sprintf("choice-%d", i)]
		if picked == "" {
			return nil, fmt.Errorf("choice %d has no pick", i+1)
		}
		got, err := resolveChoice(choice, picked)
		if err != nil {
			return nil, fmt.Errorf("choice %d: %v", i+1, err)
		}
		items = append(items, got...)
	}
	if pc := gear.StartingGear.PackChoice; pc != nil && len(pc.Options) > 0 {
		if !slices.Contains(pc.Options, pack) {
			return nil, fmt.Errorf("%q isn't one of the packs on offer", pack)
		}
		items = append(items, ItemWithQty{Item: pack, Quantity: 1})
	}
	return items, nil
}

// resolveChoice matches one pick to one of the choice's options.
func resolveChoice(choice EquipmentChoice, picked string) ([]ItemWithQty, error) {
	if picked[0] == '[' {
		var raw [][]any
		if err := json.Unmarshal([]byte(picked), &raw); err != nil {
			return nil, fmt.Errorf("unreadable pick")
		}
		var list []ItemWithQty
		for _, pair := range raw {
			if len(pair) < 2 {
				return nil, fmt.Errorf("unreadable pick")
			}
			id, ok1 := pair[0].(string)
			qty, ok2 := pair[1].(float64)
			if !ok1 || !ok2 || qty < 1 {
				return nil, fmt.Errorf("unreadable pick")
			}
			list = append(list, ItemWithQty{Item: id, Quantity: int(qty)})
		}
		for _, opt := range choice.Options {
			switch opt.Type {
			case "bundle":
				if sameItems(opt.Items, list) {
					return list, nil
				}
			case "multi_slot":
				if fillsSlots(opt.Slots, list) {
					return list, nil
				}
			}
		}
		return nil, fmt.Errorf("that combination isn't on offer")
	}
	for _, opt := range choice.Options {
		switch {
		case opt.Type == "single" && opt.Item == picked:
			return []ItemWithQty{{Item: opt.Item, Quantity: max(opt.Quantity, 1)}}, nil
		case opt.Type == "bundle" && len(opt.Items) > 0 && opt.Items[0].Item == picked:
			return append([]ItemWithQty{}, opt.Items...), nil
		}
	}
	return nil, fmt.Errorf("%q isn't on offer", picked)
}

// sameItems reports whether two item lists hold the same items in the same
// quantities, in any order.
func sameItems(a, b []ItemWithQty) bool {
	count := func(l []ItemWithQty) map[string]int {
		m := map[string]int{}
		for _, it := range l {
			m[it.Item] += max(it.Quantity, 1)
		}
		return m
	}
	ca, cb := count(a), count(b)
	if len(ca) != len(cb) {
		return false
	}
	for id, n := range ca {
		if cb[id] != n {
			return false
		}
	}
	return true
}

// fillsSlots reports whether a pick fills a multi-slot option exactly: every
// fixed item present in its quantity, and one weapon from each weapon slot's
// list. Matched as a set, so the order the client sends doesn't matter.
func fillsSlots(slots []MultiSlotItem, list []ItemWithQty) bool {
	if len(list) != len(slots) {
		return false
	}
	used := make([]bool, len(list))
	take := func(ok func(ItemWithQty) bool) bool {
		for i, it := range list {
			if !used[i] && ok(it) {
				used[i] = true
				return true
			}
		}
		return false
	}
	// Fixed items first, so a weapon slot can't claim the shield.
	for _, s := range slots {
		if s.Type == "fixed" && !take(func(it ItemWithQty) bool { return it.Item == s.Item && it.Quantity == max(s.Quantity, 1) }) {
			return false
		}
	}
	for _, s := range slots {
		if s.Type == "weapon_choice" && !take(func(it ItemWithQty) bool { return it.Quantity == 1 && slices.Contains(s.Options, it.Item) }) {
			return false
		}
	}
	return true
}

// ClaimStartingGear gives a character their class's starting gear from checked
// picks. The gear is laid out exactly as character creation did it (worn
// pieces equipped, the pack on the back with its contents), and everything the
// character already carried is put back on top, so the letter, the gold and
// the Homeward Stone aren't lost. Returns the items granted.
func ClaimStartingGear(database *sql.DB, save *types.SaveFile, choices map[string]string, pack string) ([]ItemWithQty, error) {
	gear, err := loadStartingGearForClass(database, save.Class)
	if err != nil {
		return nil, err
	}
	items, err := ResolveStartingGear(gear, choices, pack)
	if err != nil {
		return nil, err
	}
	stacked, err := stackItems(database, items)
	if err != nil {
		return nil, err
	}
	built, err := createInventoryStructure(database, stacked)
	if err != nil {
		return nil, err
	}
	// The builder makes Go-typed slices ([]map[string]any for the pack's
	// contents) that the inventory code doesn't read ("invalid backpack"). At
	// creation that never showed, because the save went to disk and back first;
	// a claim goes straight into the live session, so normalise it the same way.
	var inv map[string]interface{}
	raw, err := json.Marshal(built)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &inv); err != nil {
		return nil, err
	}
	carried := carriedItems(save.Inventory)
	save.Inventory = inv
	for _, it := range carried {
		if n, err := gameinventory.AddItemToInventory(save, it.Item, it.Quantity); err != nil || n < it.Quantity {
			return nil, fmt.Errorf("no room to keep your %s", it.Item)
		}
	}
	return items, nil
}

// carriedItems lists every item in an inventory: the general slots, the gear
// slots and anything inside containers.
func carriedItems(inv map[string]interface{}) []ItemWithQty {
	var out []ItemWithQty
	var walk func(slot map[string]interface{})
	walk = func(slot map[string]interface{}) {
		if id, _ := slot["item"].(string); id != "" {
			qty := 1
			switch q := slot["quantity"].(type) {
			case float64:
				qty = max(int(q), 1)
			case int:
				qty = max(q, 1)
			}
			out = append(out, ItemWithQty{Item: id, Quantity: qty})
		}
		if contents, ok := slot["contents"].([]interface{}); ok {
			for _, c := range contents {
				if m, ok := c.(map[string]interface{}); ok {
					walk(m)
				}
			}
		}
		if contents, ok := slot["contents"].([]map[string]interface{}); ok {
			for _, m := range contents {
				walk(m)
			}
		}
	}
	if gs, ok := inv["general_slots"].([]interface{}); ok {
		for _, s := range gs {
			if m, ok := s.(map[string]interface{}); ok {
				walk(m)
			}
		}
	}
	if gear, ok := inv["gear_slots"].(map[string]interface{}); ok {
		for _, s := range gear {
			if m, ok := s.(map[string]interface{}); ok {
				walk(m)
			}
		}
	}
	return out
}

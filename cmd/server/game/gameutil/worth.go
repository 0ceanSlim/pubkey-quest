package gameutil

import (
	"encoding/json"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// GetItemValue is an item's base value from the database (0 for none).
func GetItemValue(itemID string) int64 {
	item, err := db.GetItemByID(itemID)
	if err != nil {
		return 0
	}
	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(item.Properties), &properties); err != nil {
		return 0
	}
	if v, ok := properties["value"].(float64); ok {
		return int64(v)
	}
	return 0
}

// NetWorth is the base value of everything a character owns: their gear,
// their pack and what's inside its containers, and their vault (the net
// worth board, docs/draft/server-stats.md §7). Gold is an item (gold-piece,
// worth 1 each), so it counts like anything else. Base value, not shop
// prices, so it's the same in every town.
func NetWorth(state *types.SaveFile) int64 {
	values := map[string]int64{}
	value := func(id string) int64 {
		v, ok := values[id]
		if !ok {
			v = GetItemValue(id)
			values[id] = v
		}
		return v
	}
	var slotWorth func(slot map[string]interface{}) int64
	slotWorth = func(slot map[string]interface{}) int64 {
		id, _ := slot["item"].(string)
		if id == "" || id == "null" {
			return 0
		}
		qty := int64(GetIntValue(slot, "quantity", 1))
		if qty <= 0 {
			return 0
		}
		worth := value(id)
		if contents, ok := slot["contents"].([]interface{}); ok {
			for _, c := range contents {
				if inner, ok := c.(map[string]interface{}); ok {
					worth += slotWorth(inner)
				}
			}
		}
		return worth * qty
	}

	var total int64
	if general, ok := state.Inventory["general_slots"].([]interface{}); ok {
		for _, s := range general {
			if slot, ok := s.(map[string]interface{}); ok {
				total += slotWorth(slot)
			}
		}
	}
	if gear, ok := state.Inventory["gear_slots"].(map[string]interface{}); ok {
		// A two-handed weapon sits in both hand slots; count it once.
		mainID := ""
		if m, ok := gear["mainhand"].(map[string]interface{}); ok {
			mainID, _ = m["item"].(string)
		}
		for name, s := range gear {
			slot, ok := s.(map[string]interface{})
			if !ok {
				continue
			}
			if id, _ := slot["item"].(string); name == "offhand" && id == mainID && IsTwoHanded(id) {
				continue
			}
			total += slotWorth(slot)
		}
	}
	for _, e := range state.Vault {
		if e.Quantity > 0 {
			total += value(e.ItemID) * int64(e.Quantity)
		}
	}
	return total
}

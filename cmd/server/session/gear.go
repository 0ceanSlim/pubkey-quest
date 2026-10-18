package session

import "pubkey-quest/types"

// normalizeGearSlots moves equipment filed under an old slot name to the slot
// the game uses: equipping a neck item used to file it as "necklace", which
// nothing shows or saves, instead of "neck". Runs when a save loads.
func normalizeGearSlots(save *types.SaveFile) {
	if save == nil || save.Inventory == nil {
		return
	}
	gear, ok := save.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return
	}
	legacy, ok := gear["necklace"].(map[string]interface{})
	if !ok {
		return
	}
	if item, _ := legacy["item"].(string); item != "" {
		neck, _ := gear["neck"].(map[string]interface{})
		if cur, _ := neck["item"].(string); cur != "" {
			return // both worn: leave it rather than lose an item
		}
		gear["neck"] = legacy
	}
	delete(gear, "necklace")
}

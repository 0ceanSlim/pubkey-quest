package session

import (
	"testing"

	"pubkey-quest/types"
)

func TestNormalizeGearSlotsMovesNecklace(t *testing.T) {
	save := &types.SaveFile{Inventory: map[string]interface{}{
		"gear_slots": map[string]interface{}{
			"neck":     map[string]interface{}{"item": nil, "quantity": 0},
			"necklace": map[string]interface{}{"item": "amulet", "quantity": 1},
		},
	}}
	normalizeGearSlots(save)
	gear := save.Inventory["gear_slots"].(map[string]interface{})
	if gear["neck"].(map[string]interface{})["item"] != "amulet" {
		t.Fatalf("amulet not moved to neck: %v", gear)
	}
	if _, ok := gear["necklace"]; ok {
		t.Fatal("the old necklace slot should be gone")
	}

	// A neck already worn keeps its item, and the stray one stays put rather
	// than being lost.
	save.Inventory["gear_slots"] = map[string]interface{}{
		"neck":     map[string]interface{}{"item": "emblem", "quantity": 1},
		"necklace": map[string]interface{}{"item": "amulet", "quantity": 1},
	}
	normalizeGearSlots(save)
	gear = save.Inventory["gear_slots"].(map[string]interface{})
	if gear["neck"].(map[string]interface{})["item"] != "emblem" {
		t.Fatalf("worn neck item replaced: %v", gear)
	}
	if _, ok := gear["necklace"]; !ok {
		t.Fatal("the stray amulet must not be deleted")
	}
}

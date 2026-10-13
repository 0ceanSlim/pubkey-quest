package inventory_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

func handsSave(main, off string) *types.SaveFile {
	slot := func(id string) map[string]interface{} {
		if id == "" {
			return map[string]interface{}{"item": nil, "quantity": 0}
		}
		return map[string]interface{}{"item": id, "quantity": 1}
	}
	return &types.SaveFile{Inventory: map[string]interface{}{
		"general_slots": []interface{}{
			map[string]interface{}{"slot": 0, "item": nil, "quantity": 0},
			map[string]interface{}{"slot": 1, "item": nil, "quantity": 0},
		},
		"gear_slots": map[string]interface{}{"mainhand": slot(main), "offhand": slot(off)},
	}}
}

func hand(save *types.SaveFile, slot string) interface{} {
	return save.Inventory["gear_slots"].(map[string]interface{})[slot].(map[string]interface{})["item"]
}

// A druid made by older starting gear held a bow in the main hand and a staff
// in the off hand. On load the bow takes both hands and the staff goes to the pack.
func TestNormalizeHandsRepairsABowWithAStaff(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := db.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	save := handsSave("shortbow", "quarterstaff")
	if !inventory.NormalizeHands(save) {
		t.Fatal("expected a repair")
	}
	if hand(save, "mainhand") != "shortbow" || hand(save, "offhand") != "shortbow" {
		t.Errorf("hands: %v / %v, want the bow in both", hand(save, "mainhand"), hand(save, "offhand"))
	}
	if !gameutil.PlayerHasItem(save, "quarterstaff") {
		t.Error("the staff should be in the pack")
	}
	// Weighed once, not twice.
	bowOnly := handsSave("shortbow", "shortbow")
	single := handsSave("shortbow", "")
	if gameutil.CalculateTotalWeight(bowOnly) != gameutil.CalculateTotalWeight(single) {
		t.Errorf("a two-hander in both hands weighs %v, one hand %v", gameutil.CalculateTotalWeight(bowOnly), gameutil.CalculateTotalWeight(single))
	}
	// Fine hands are left alone: two one-handers, or a two-hander already in both.
	for _, s := range []*types.SaveFile{handsSave("dagger", "dagger"), handsSave("mace", "shield"), handsSave("greataxe", "greataxe")} {
		if inventory.NormalizeHands(s) {
			t.Errorf("changed legal hands %v / %v", hand(s, "mainhand"), hand(s, "offhand"))
		}
	}
}

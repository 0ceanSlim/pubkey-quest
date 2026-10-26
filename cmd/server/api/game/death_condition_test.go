package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/tests/helpers"
)

// The items you keep when you die keep their condition: a worn sword stays
// worn. A light you held goes out.
func TestDeathKeepsCondition(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	condition.ResetCache()
	inv := map[string]interface{}{
		"general_slots": []interface{}{
			map[string]interface{}{"slot": 0, "item": nil, "quantity": 0},
			map[string]interface{}{"slot": 1, "item": nil, "quantity": 0},
			map[string]interface{}{"slot": 2, "item": nil, "quantity": 0},
			map[string]interface{}{"slot": 3, "item": nil, "quantity": 0},
		},
		"gear_slots": map[string]interface{}{
			"mainhand": map[string]interface{}{"item": "longsword", "quantity": 1, condition.KeyWear: 120},
			"chest":    map[string]interface{}{"item": "chainmail-hauberk", "quantity": 1},
			"offhand":  map[string]interface{}{"item": "lantern-bullseye", "quantity": 1, condition.KeyLit: 1, condition.KeyBurn: 200},
		},
	}
	kept := stripInventoryForDeath(inv)
	if len(kept) != 3 {
		t.Fatalf("kept %+v", kept)
	}
	found := map[string]map[string]interface{}{}
	for _, raw := range inv["general_slots"].([]interface{}) {
		if m, ok := raw.(map[string]interface{}); ok && m["item"] != nil {
			found[m["item"].(string)] = m
		}
	}
	if s := found["longsword"]; s == nil || condition.WearOf(s) != 120 {
		t.Errorf("kept longsword %v", s)
	}
	if s := found["lantern-bullseye"]; s == nil || condition.IsLit(s) || condition.BurnLeft(s) != 200 {
		t.Errorf("kept lantern %v", s)
	}
}

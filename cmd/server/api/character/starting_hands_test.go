package character

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
)

// Starting gear lands in the 5e places: the weapon in the main hand, a focus or
// shield in the off hand, a two-handed weapon keeps the off hand free, and a
// spellbook is carried in the pack rather than held.
func TestStartingHands(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		class         string
		items         []ItemWithQty
		main, offhand any
	}{
		{"Wizard", []ItemWithQty{{Item: "spellbook", Quantity: 1}, {Item: "quarterstaff", Quantity: 1}, {Item: "staff", Quantity: 1}}, "quarterstaff", "staff"},
		{"Wizard (wand)", []ItemWithQty{{Item: "spellbook", Quantity: 1}, {Item: "dagger", Quantity: 1}, {Item: "wand", Quantity: 1}}, "dagger", "wand"},
		{"Cleric", []ItemWithQty{{Item: "shield", Quantity: 1}, {Item: "mace", Quantity: 1}, {Item: "emblem", Quantity: 1}}, "mace", "shield"},
		{"Barbarian", []ItemWithQty{{Item: "javelin", Quantity: 4}, {Item: "greataxe", Quantity: 1}, {Item: "handaxe", Quantity: 2}}, "greataxe", nil},
	}
	for _, c := range cases {
		inv, err := createInventoryStructure(serverdb.GetDB(), c.items)
		if err != nil {
			t.Fatal(err)
		}
		g := inv["gear_slots"].(map[string]interface{})
		main := g["mainhand"].(map[string]interface{})["item"]
		off := g["offhand"].(map[string]interface{})["item"]
		if main != c.main || off != c.offhand {
			t.Errorf("%s: mainhand=%v offhand=%v, want %v / %v", c.class, main, off, c.main, c.offhand)
		}
		if c.class[:6] == "Wizard" && !carries(inv, "spellbook") {
			t.Errorf("%s: the spellbook should be in the pack", c.class)
		}
	}
}

// carries reports whether an item sits in the general slots or the backpack.
func carries(inv map[string]interface{}, id string) bool {
	for _, s := range inv["general_slots"].([]any) {
		if m, ok := s.(map[string]any); ok && m["item"] == id {
			return true
		}
	}
	if bag, ok := inv["gear_slots"].(map[string]interface{})["bag"].(map[string]interface{}); ok {
		if contents, ok := bag["contents"].([]interface{}); ok {
			for _, s := range contents {
				if m, ok := s.(map[string]interface{}); ok && m["item"] == id {
					return true
				}
			}
		}
	}
	return false
}

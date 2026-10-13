package character

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
)

// Starting gear lands in sensible hands: the weapon in the main hand, a
// two-handed weapon keeps the off hand free, a shield in the off hand.
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
		{"Wizard", []ItemWithQty{{Item: "spellbook", Quantity: 1}, {Item: "quarterstaff", Quantity: 1}, {Item: "staff", Quantity: 1}}, "quarterstaff", "spellbook"},
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
	}
}

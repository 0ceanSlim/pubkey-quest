package character_test

import (
	"testing"

	"pubkey-quest/cmd/server/api/character"
)

// The inheritance claim only hands out what the class was offered: every pick
// is checked against starting-gear.json, unlike the old creation path, which
// trusted any JSON list of items the client sent.

func fighterGear() *character.StartingGearData {
	g := &character.StartingGearData{Class: "Fighter"}
	g.StartingGear.GivenItems = []character.ItemWithQty{{Item: "rope", Quantity: 1}}
	g.StartingGear.EquipmentChoices = []character.EquipmentChoice{
		{Options: []character.EquipmentOption{
			{Type: "single", Item: "chainmail-set", Quantity: 1},
			{Type: "bundle", Items: []character.ItemWithQty{{Item: "leather-set", Quantity: 1}, {Item: "longbow", Quantity: 1}, {Item: "arrows", Quantity: 20}}},
		}},
		{Options: []character.EquipmentOption{
			{Type: "multi_slot", Slots: []character.MultiSlotItem{
				{Type: "weapon_choice", Options: []string{"longsword", "battleaxe"}},
				{Type: "fixed", Item: "shield", Quantity: 1},
			}},
		}},
	}
	g.StartingGear.PackChoice = &struct {
		Description string   `json:"description"`
		Options     []string `json:"options"`
	}{Options: []string{"explorers-pack", "dungeoneers-pack"}}
	return g
}

func TestStartingGearAcceptsWhatsOffered(t *testing.T) {
	items, err := character.ResolveStartingGear(fighterGear(), map[string]string{
		"choice-0": `[["arrows",20],["longbow",1],["leather-set",1]]`, // a bundle, in any order
		"choice-1": `[["shield",1],["battleaxe",1]]`,                  // weapon + fixed shield, in any order
	}, "explorers-pack")
	if err != nil {
		t.Fatalf("a valid claim was refused: %v", err)
	}
	if len(items) != 1+3+2+1 { // given + bundle + weapon slots + pack
		t.Fatalf("got %d items: %+v", len(items), items)
	}
	if _, err := character.ResolveStartingGear(fighterGear(), map[string]string{
		"choice-0": "chainmail-set", "choice-1": `[["longsword",1],["shield",1]]`,
	}, "dungeoneers-pack"); err != nil {
		t.Fatalf("a single pick was refused: %v", err)
	}
}

func TestStartingGearRefusesWhatIsnt(t *testing.T) {
	cases := map[string]map[string]string{
		"an item not on offer":       {"choice-0": "plate-set", "choice-1": `[["longsword",1],["shield",1]]`},
		"a padded bundle":            {"choice-0": `[["leather-set",1],["longbow",1],["arrows",99]]`, "choice-1": `[["longsword",1],["shield",1]]`},
		"a weapon not in the slot":   {"choice-0": "chainmail-set", "choice-1": `[["greatsword",1],["shield",1]]`},
		"a slot left empty":          {"choice-0": "chainmail-set", "choice-1": `[["shield",1]]`},
		"two weapons for one slot":   {"choice-0": "chainmail-set", "choice-1": `[["longsword",1],["battleaxe",1]]`},
		"a choice not made":          {"choice-0": "chainmail-set"},
		"an extra item in the slots": {"choice-0": "chainmail-set", "choice-1": `[["longsword",1],["shield",1],["gold-piece",1000]]`},
	}
	for name, picks := range cases {
		if _, err := character.ResolveStartingGear(fighterGear(), picks, "explorers-pack"); err == nil {
			t.Errorf("%s: should be refused", name)
		}
	}
	if _, err := character.ResolveStartingGear(fighterGear(), map[string]string{
		"choice-0": "chainmail-set", "choice-1": `[["longsword",1],["shield",1]]`,
	}, "bag-of-holding"); err == nil {
		t.Error("a pack not on offer should be refused")
	}
}

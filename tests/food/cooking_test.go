package food_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// Cooking and salting (food-system §5).

func camper(items ...string) *types.SaveFile {
	s := player(2)
	s.Location, s.TravelStopped = "darkwood-forest", true
	for i, id := range items {
		slot(s, i)["item"], slot(s, i)["quantity"] = id, 1
		if st := condition.NewState(id, condition.Now(s)-600); st != nil {
			condition.ApplyState(slot(s, i), st)
		}
	}
	return s
}

func TestCookAtAFire(t *testing.T) {
	setup(t)
	s := camper("tinderbox", "raw-fish", "raw-meat")
	before := s.TimeOfDay
	resp := inventory.CookAll(s)
	if !resp.Success {
		t.Fatalf("cook: %s", resp.Message)
	}
	if slot(s, 1)["item"] != "cooked-fish" || slot(s, 2)["item"] != "cooked-meat" {
		t.Errorf("cooked into %v and %v", slot(s, 1)["item"], slot(s, 2)["item"])
	}
	// Cooked food is freshly made.
	if f, _ := condition.FoodFreshness(slot(s, 1), condition.Now(s)); f != condition.Fresh {
		t.Errorf("cooked fish is %v", f)
	}
	if s.TimeOfDay-before != inventory.CookMinutes {
		t.Errorf("cooking took %d minutes", s.TimeOfDay-before)
	}

	for name, s := range map[string]*types.SaveFile{
		"no tinderbox": camper("raw-fish"),
		"in town":      func() *types.SaveFile { s := camper("tinderbox", "raw-fish"); s.Location = "kingdom"; return s }(),
		"walking":      func() *types.SaveFile { s := camper("tinderbox", "raw-fish"); s.TravelStopped = false; return s }(),
		"nothing raw":  camper("tinderbox", "bread"),
	} {
		if resp := inventory.CookAll(s); resp.Success {
			t.Errorf("%s: cooked anyway (%s)", name, resp.Message)
		}
	}
}

func TestSaltIntoJerky(t *testing.T) {
	setup(t)
	s := camper("tinderbox", "raw-meat", "salt")
	slot(s, 2)["quantity"] = 3
	targets := inventory.UseTargets(s, "salt")
	if len(targets) != 1 || targets[0].ItemID != "raw-meat" {
		t.Fatalf("salt targets: %+v", targets)
	}
	if resp, _ := inventory.UseOn(s, "salt", targets[0].SlotRef); !resp.Success {
		t.Fatalf("salt: %s", resp.Message)
	}
	if slot(s, 1)["item"] != "jerky" || gameutil.CountItem(s, "salt") != 2 {
		t.Errorf("salted into %v, %d salt left", slot(s, 1)["item"], gameutil.CountItem(s, "salt"))
	}
}

func TestInnCooksForGold(t *testing.T) {
	setup(t)
	s := camper("raw-fish", "large-fish")
	if n := inventory.CountRawFood(s); n != 2 {
		t.Fatalf("raw food %d", n)
	}
	made := inventory.CookAllAtInn(s)
	if made["Cooked Fish"] != 1 || made["Cooked Large Fish"] != 1 {
		t.Errorf("inn cooked %v", made)
	}
}

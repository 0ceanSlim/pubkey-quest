package food_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
)

// Charges (degradation §7): uses spent in "u", used up or refilled.

func TestInkRunsOut(t *testing.T) {
	setup(t)
	s := player(2)
	slot(s, 0)["item"], slot(s, 0)["quantity"] = "ink", 1
	for i := 1; i <= 9; i++ {
		left, gone, found := gameutil.SpendCharge(s, "ink")
		if !found || gone || left != 10-i {
			t.Fatalf("scroll %d: left %d gone %v found %v", i, left, gone, found)
		}
	}
	if _, gone, _ := gameutil.SpendCharge(s, "ink"); !gone {
		t.Fatal("the 10th scroll should finish the bottle")
	}
	if slot(s, 0)["item"] != nil || condition.HasState(slot(s, 0)) {
		t.Errorf("used-up ink left %v", slot(s, 0))
	}
	if _, _, found := gameutil.SpendCharge(s, "ink"); found {
		t.Error("spent a charge from nothing")
	}
}

// A part-used bottle is finished before a fresh one is opened.
func TestSpendsThePartUsedOneFirst(t *testing.T) {
	setup(t)
	s := player(2)
	slot(s, 0)["item"], slot(s, 0)["quantity"] = "ink", 1
	slot(s, 1)["item"], slot(s, 1)["quantity"], slot(s, 1)[condition.KeyUses] = "ink", 1, 7
	gameutil.SpendCharge(s, "ink")
	if condition.HasState(slot(s, 0)) || condition.ChargesLeft(slot(s, 1)) != 2 {
		t.Errorf("fresh %v, part-used %v", slot(s, 0), slot(s, 1))
	}
}

func TestWhetstoneWearsSmooth(t *testing.T) {
	setup(t)
	s := armed("mainhand", "longsword", 250, "whetstone")
	slot(s, 0)[condition.KeyUses] = 9 // its last sharpening
	resp, _ := inventory.UseOn(s, "whetstone", inventory.SlotRef{Type: "equipment", Gear: "mainhand"})
	if !resp.Success || slot(s, 0)["item"] != nil {
		t.Errorf("last sharpening: %s, slot %v", resp.Message, slot(s, 0))
	}
}

func TestConsumeClearsState(t *testing.T) {
	setup(t)
	s := player(2)
	slot(s, 0)["item"], slot(s, 0)["quantity"], slot(s, 0)[condition.KeyMade] = "raw-fish", 1, 100
	gameutil.ConsumeItem(s, "raw-fish", 1)
	if condition.HasState(slot(s, 0)) {
		t.Errorf("consumed fish left its state: %v", slot(s, 0))
	}
}

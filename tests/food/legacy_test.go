package food_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
)

// Saves from before degradation: stacks of things that now stack 1.

// A stack of raw fish splits into singles, each keeping the stack's made-time;
// stacks that still fit (rations) are left alone.
func TestOldStacksSplitOnLoad(t *testing.T) {
	setup(t)
	s := player(2)
	slot(s, 0)["item"], slot(s, 0)["quantity"], slot(s, 0)[condition.KeyMade] = "raw-fish", 4, 100
	slot(s, 1)["item"], slot(s, 1)["quantity"] = "rations", 6
	if !inventory.SplitStacks(s) {
		t.Fatal("nothing split")
	}
	if gameutil.CountItem(s, "raw-fish") != 4 || slot(s, 0)["quantity"] != 1 {
		t.Fatalf("fish: %d carried, first slot %v", gameutil.CountItem(s, "raw-fish"), slot(s, 0))
	}
	for _, i := range []int{2, 3} {
		if sl := slot(s, i); sl["item"] != "raw-fish" || sl[condition.KeyMade] != 100 {
			t.Errorf("belt slot %d: %v", i, sl)
		}
	}
	if sl := bag(s)[0].(map[string]interface{}); sl["item"] != "raw-fish" || sl["slot"] != 0 {
		t.Errorf("the belt is full, so the last goes to the pack: %v", sl)
	}
	if slot(s, 1)["quantity"] != 6 {
		t.Errorf("rations still stack: %v", slot(s, 1))
	}
	if inventory.SplitStacks(s) {
		t.Error("a second load split again")
	}
}

// With no room the rest stay stacked rather than being lost.
func TestOldStacksKeptWithoutRoom(t *testing.T) {
	setup(t)
	s := player(2)
	slot(s, 0)["item"], slot(s, 0)["quantity"] = "whetstone", 3
	for i := 1; i < 4; i++ {
		slot(s, i)["item"], slot(s, i)["quantity"] = "rations", 1
	}
	for i := range bag(s) {
		bag(s)[i].(map[string]interface{})["item"] = "rations"
		bag(s)[i].(map[string]interface{})["quantity"] = 1
	}
	inventory.SplitStacks(s)
	if slot(s, 0)["quantity"] != 3 {
		t.Errorf("whetstones: %v", slot(s, 0))
	}
}

// Wearing out the last of a stacked whetstone takes one, not the stack.
func TestUsedUpTakesOneFromAStack(t *testing.T) {
	setup(t)
	s := player(2)
	slot(s, 0)["item"], slot(s, 0)["quantity"], slot(s, 0)[condition.KeyUses] = "whetstone", 3, 9
	if _, gone, _ := gameutil.SpendCharge(s, "whetstone"); !gone {
		t.Fatal("the 10th sharpening should wear one smooth")
	}
	if slot(s, 0)["item"] != "whetstone" || slot(s, 0)["quantity"] != 2 || condition.HasState(slot(s, 0)) {
		t.Errorf("left %v, want 2 new whetstones", slot(s, 0))
	}
}

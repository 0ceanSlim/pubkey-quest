package food_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// Lights (degradation §6): a tinderbox lights them into your off hand, they
// burn while lit, torches are spent when they go out, lamps keep their oil and
// refill with a flask.

func gear(s *types.SaveFile) map[string]interface{} {
	g, _ := s.Inventory["gear_slots"].(map[string]interface{})
	if g == nil {
		g = map[string]interface{}{}
		s.Inventory["gear_slots"] = g
	}
	return g
}

func offhand(s *types.SaveFile) map[string]interface{} {
	h, _ := gear(s)["offhand"].(map[string]interface{})
	return h
}

func TestTorchLightsIntoTheHandAndBurnsOut(t *testing.T) {
	setup(t)
	s := player(2)
	inventory.AddItemToInventory(s, "torch", 3)

	_, torches := find(s, "torch")
	if r, _ := inventory.UseOn(s, "tinderbox", torches); r == nil || r.Success {
		t.Fatal("no tinderbox, no light")
	}
	inventory.AddItemToInventory(s, "tinderbox", 1)

	targets := inventory.UseTargets(s, "tinderbox")
	if len(targets) != 1 || targets[0].Label != "Light Torch" {
		t.Fatalf("the tinderbox should offer the torches: %+v", targets)
	}
	if r, err := inventory.UseOn(s, "tinderbox", targets[0].SlotRef); err != nil || !r.Success {
		t.Fatalf("light: %v %+v", err, r)
	}
	if h := offhand(s); h["item"] != "torch" || !condition.IsLit(h) || condition.BurnLeft(h) != 60 {
		t.Fatalf("a lit torch should be in the off hand with an hour left: %+v", h)
	}
	if st, _ := find(s, "torch"); st == nil || st["quantity"] != 2 {
		t.Errorf("one torch comes off the stack: %+v", st)
	}

	condition.BurnHeld(s, 30)
	if condition.BurnLeft(offhand(s)) != 30 {
		t.Errorf("after 30 minutes: %d left", condition.BurnLeft(offhand(s)))
	}
	if msgs := condition.BurnHeld(s, 30); len(msgs) != 1 || offhand(s)["item"] != nil {
		t.Errorf("the torch should burn out and be gone: %v %+v", msgs, offhand(s))
	}

	// A lit torch taken out of the hand goes out and is spent.
	_, torches = find(s, "torch")
	inventory.UseOn(s, "tinderbox", torches)
	r, err := inventory.HandleUnequipItemAction(s, map[string]interface{}{"item_id": "torch", "equipment_slot": "offhand"})
	if err != nil || r == nil || offhand(s)["item"] != nil {
		t.Fatalf("unequip a lit torch: %v %+v, hand %+v", err, r, offhand(s))
	}
	if st, _ := find(s, "torch"); st == nil || st["quantity"] != 1 {
		t.Errorf("the lit torch is spent, not returned to the stack: %+v", st)
	}
}

func TestLampKeepsItsOilAndRefills(t *testing.T) {
	setup(t)
	s := player(2)
	inventory.AddItemToInventory(s, "lamp", 1)
	inventory.AddItemToInventory(s, "tinderbox", 1)
	inventory.AddItemToInventory(s, "oil", 1)

	_, lamp := find(s, "lamp")
	if r, _ := inventory.UseOn(s, "tinderbox", lamp); r == nil || !r.Success {
		t.Fatalf("light the lamp: %+v", r)
	}
	condition.BurnHeld(s, 100)
	if r, _ := inventory.UseOn(s, "tinderbox", inventory.SlotRef{Type: "equipment", Gear: "offhand"}); r == nil || !r.Success {
		t.Fatalf("put it out: %+v", r)
	}
	h := offhand(s)
	if h["item"] != "lamp" || condition.IsLit(h) || condition.BurnLeft(h) != 260 {
		t.Fatalf("a lamp put out keeps its oil: %+v", h)
	}

	// Unequip it and back: the oil goes with it.
	inventory.HandleUnequipItemAction(s, map[string]interface{}{"item_id": "lamp", "equipment_slot": "offhand"})
	if sl, _ := find(s, "lamp"); sl == nil || condition.BurnLeft(sl) != 260 {
		t.Fatalf("the lamp should come off the hand with its oil: %+v", sl)
	}

	targets := inventory.UseTargets(s, "oil")
	if len(targets) != 1 {
		t.Fatalf("oil should offer the lamp: %+v", targets)
	}
	if r, _ := inventory.UseOn(s, "oil", targets[0].SlotRef); r == nil || !r.Success {
		t.Fatalf("refill: %+v", r)
	}
	if sl := slotAtRef(s, targets[0].SlotRef); condition.BurnLeft(sl) != 360 {
		t.Errorf("refilled lamp has %d minutes", condition.BurnLeft(sl))
	}
	if len(inventory.UseTargets(s, "oil")) != 0 {
		t.Error("a full lamp doesn't need oil")
	}
}

func slotAtRef(s *types.SaveFile, ref inventory.SlotRef) map[string]interface{} {
	switch ref.Type {
	case "equipment":
		return offhand(s)
	case "inventory":
		return bag(s)[ref.Index].(map[string]interface{})
	}
	return slot(s, ref.Index)
}

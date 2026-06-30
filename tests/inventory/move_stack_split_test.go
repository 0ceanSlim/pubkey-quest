package inventory_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/inventory"
)

func TestMoveToEmptySlot(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "longsword", 1)

	resp, err := inventory.HandleMoveItemAction(s, p(map[string]interface{}{
		"item_id": "longsword", "from_slot": float64(0), "to_slot": float64(1),
		"from_slot_type": "general", "to_slot_type": "general",
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("move: resp=%+v err=%v", resp, err)
	}

	if got := slotItem(general(s), 1); got != "longsword" {
		t.Errorf("general[1] = %q, want longsword", got)
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want empty", got)
	}
}

func TestMoveSwapsOccupiedSlots(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "longsword", 1)
	general(s)[1] = slot(1, "dagger", 1)

	resp, err := inventory.HandleMoveItemAction(s, p(map[string]interface{}{
		"item_id": "longsword", "from_slot": float64(0), "to_slot": float64(1),
		"from_slot_type": "general", "to_slot_type": "general",
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("move: resp=%+v err=%v", resp, err)
	}

	if got := slotItem(general(s), 1); got != "longsword" {
		t.Errorf("general[1] = %q, want longsword", got)
	}
	if got := slotItem(general(s), 0); got != "dagger" {
		t.Errorf("general[0] = %q, want dagger (swapped)", got)
	}
}

func TestStackCombinesStacks(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "rations", 3)
	general(s)[1] = slot(1, "rations", 4)

	resp, err := inventory.HandleStackItemAction(s, p(map[string]interface{}{
		"item_id": "rations", "from_slot": float64(0), "to_slot": float64(1),
		"from_slot_type": "general", "to_slot_type": "general",
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("stack: resp=%+v err=%v", resp, err)
	}

	if got := slotQty(general(s), 1); got != 7 {
		t.Errorf("general[1] qty = %d, want 7", got)
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want empty (fully merged)", got)
	}
}

// rations cap at 10 (stack field); the overflow stays in the source slot.
func TestStackRespectsMaxStack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "rations", 8)
	general(s)[1] = slot(1, "rations", 5)

	resp, err := inventory.HandleStackItemAction(s, p(map[string]interface{}{
		"item_id": "rations", "from_slot": float64(0), "to_slot": float64(1),
		"from_slot_type": "general", "to_slot_type": "general",
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("stack: resp=%+v err=%v", resp, err)
	}

	if got := slotQty(general(s), 1); got != 10 {
		t.Errorf("general[1] qty = %d, want 10 (capped)", got)
	}
	if got := slotQty(general(s), 0); got != 3 {
		t.Errorf("general[0] qty = %d, want 3 (overflow remains)", got)
	}
}

func TestSplitCreatesSecondStack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "rations", 5)

	resp, err := inventory.HandleSplitItemAction(s, p(map[string]interface{}{
		"item_id": "rations", "from_slot": float64(0), "to_slot": float64(1),
		"from_slot_type": "general", "to_slot_type": "general", "quantity": float64(2),
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("split: resp=%+v err=%v", resp, err)
	}

	if got := slotQty(general(s), 0); got != 3 {
		t.Errorf("general[0] qty = %d, want 3", got)
	}
	if got := slotItem(general(s), 1); got != "rations" {
		t.Errorf("general[1] = %q, want rations", got)
	}
	if got := slotQty(general(s), 1); got != 2 {
		t.Errorf("general[1] qty = %d, want 2", got)
	}
}

// Containers may not be stuffed into the backpack (it is itself a container).
func TestMoveContainerToBackpackRejected(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "component-pouch", 1)

	resp, err := inventory.HandleMoveItemAction(s, p(map[string]interface{}{
		"item_id": "component-pouch", "from_slot": float64(0), "to_slot": float64(0),
		"from_slot_type": "general", "to_slot_type": "inventory",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || resp.Success {
		t.Errorf("expected rejection moving a container into the backpack, got %+v", resp)
	}
	// The pouch must remain where it was.
	if got := slotItem(general(s), 0); got != "component-pouch" {
		t.Errorf("general[0] = %q, want component-pouch (unchanged)", got)
	}
}

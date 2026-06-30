package inventory_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/inventory"
)

// Vault deposit/withdraw are really HandleMoveItemAction with a vault endpoint.
func TestVaultRoundTrip(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	makeVault(s, "bank", 40)
	general(s)[0] = slot(0, "longsword", 1)

	deposit := func(fromType, toType string, from, to int) {
		t.Helper()
		resp, err := inventory.HandleMoveItemAction(s, p(map[string]interface{}{
			"item_id": "longsword", "from_slot": float64(from), "to_slot": float64(to),
			"from_slot_type": fromType, "to_slot_type": toType, "vault_building": "bank",
		}))
		if err != nil || resp == nil || !resp.Success {
			t.Fatalf("move %s->%s: resp=%+v err=%v", fromType, toType, resp, err)
		}
	}

	// Deposit general[0] -> vault[0]
	deposit("general", "vault", 0, 0)
	if got := slotItem(vaultSlots(s, "bank"), 0); got != "longsword" {
		t.Errorf("vault[0] = %q, want longsword after deposit", got)
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want empty after deposit", got)
	}

	// Withdraw vault[0] -> general[0]
	deposit("vault", "general", 0, 0)
	if got := slotItem(general(s), 0); got != "longsword" {
		t.Errorf("general[0] = %q, want longsword after withdraw", got)
	}
}

func TestDropRemovesEntireStack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "longsword", 1)

	resp, err := inventory.HandleDropItemAction(s, p(map[string]interface{}{
		"item_id": "longsword", "slot": float64(0), "slot_type": "general",
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("drop: resp=%+v err=%v", resp, err)
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want empty after drop", got)
	}
}

func TestDropPartialStackKeepsRemainder(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "rations", 5)

	resp, err := inventory.HandleDropItemAction(s, p(map[string]interface{}{
		"item_id": "rations", "slot": float64(0), "slot_type": "general", "quantity": float64(2),
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("drop: resp=%+v err=%v", resp, err)
	}
	if got := slotItem(general(s), 0); got != "rations" {
		t.Errorf("general[0] = %q, want rations (partial drop)", got)
	}
	if got := slotQty(general(s), 0); got != 3 {
		t.Errorf("general[0] qty = %d, want 3 after dropping 2", got)
	}
}

func TestUseConsumableDecrementsStack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "rations", 3)

	resp, err := inventory.HandleUseItemAction(s, p(map[string]interface{}{
		"item_id": "rations", "slot": float64(0),
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("use: resp=%+v err=%v", resp, err)
	}
	if got := slotItem(general(s), 0); got != "rations" {
		t.Errorf("general[0] = %q, want rations still present", got)
	}
	if got := slotQty(general(s), 0); got != 2 {
		t.Errorf("general[0] qty = %d, want 2 after using one", got)
	}
}

func TestUseSingleConsumableClearsSlotAndHeals(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	s.HP = 5
	general(s)[0] = slot(0, "healing", 1)

	resp, err := inventory.HandleUseItemAction(s, p(map[string]interface{}{
		"item_id": "healing", "slot": float64(0),
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("use: resp=%+v err=%v", resp, err)
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want empty after using the last potion", got)
	}
	if s.HP <= 5 {
		t.Errorf("HP = %d, want > 5 after a healing potion", s.HP)
	}
}

// Regression: splitting a stack stores quantity as int; consuming from that
// freshly-split stack must read the int and decrement, not see 0 and wipe it.
func TestSplitThenUseKeepsStack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "rations", 5)

	if _, err := inventory.HandleSplitItemAction(s, p(map[string]interface{}{
		"item_id": "rations", "from_slot": float64(0), "to_slot": float64(1),
		"from_slot_type": "general", "to_slot_type": "general", "quantity": float64(2),
	})); err != nil {
		t.Fatalf("split: %v", err)
	}

	// general[0] now holds an int quantity of 3.
	resp, err := inventory.HandleUseItemAction(s, p(map[string]interface{}{
		"item_id": "rations", "slot": float64(0),
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("use after split: resp=%+v err=%v", resp, err)
	}
	if got := slotItem(general(s), 0); got != "rations" {
		t.Errorf("general[0] = %q, want rations still present (not wiped)", got)
	}
	if got := slotQty(general(s), 0); got != 2 {
		t.Errorf("general[0] qty = %d, want 2 after consuming one of the split stack", got)
	}
}

package inventory_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// equip runs HandleEquipItemAction with slot auto-determination (no
// equipment_slot param) — the realistic path that reads gear_slot + the
// two-handed tag from the item.
func equip(t *testing.T, s *types.SaveFile, itemID string, fromSlot int, fromType string) {
	t.Helper()
	resp, err := inventory.HandleEquipItemAction(s, p(map[string]interface{}{
		"item_id": itemID, "from_slot": float64(fromSlot), "from_slot_type": fromType,
	}))
	if err != nil {
		t.Fatalf("equip %s: %v", itemID, err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("equip %s not successful: %+v", itemID, resp)
	}
}

func TestEquipWeaponFromGeneralToMainhand(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "longsword", 1)

	equip(t, s, "longsword", 0, "general")

	if got := gearItem(s, "mainhand"); got != "longsword" {
		t.Errorf("mainhand = %q, want longsword", got)
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want empty after equip", got)
	}
}

func TestEquipShieldToOffhand(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "shield", 1)

	equip(t, s, "shield", 0, "general")

	if got := gearItem(s, "offhand"); got != "shield" {
		t.Errorf("offhand = %q, want shield", got)
	}
}

func TestEquipArmorToChest(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "breastplate", 1)

	equip(t, s, "breastplate", 0, "general")

	if got := gearItem(s, "chest"); got != "breastplate" {
		t.Errorf("chest = %q, want breastplate", got)
	}
}

func TestEquipFromBackpack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	backpack(s)[0] = slot(0, "dagger", 1)

	equip(t, s, "dagger", 0, "inventory")

	if got := gearItem(s, "mainhand"); got != "dagger" {
		t.Errorf("mainhand = %q, want dagger", got)
	}
	if got := slotItem(backpack(s), 0); got != "" {
		t.Errorf("backpack[0] = %q, want empty after equip", got)
	}
}

// A two-handed weapon must occupy both hands and displace whatever was in them
// back into inventory.
func TestEquipTwoHandedClearsBothHands(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	gs := gearSlots(s)
	gs["mainhand"] = map[string]interface{}{"item": "dagger", "quantity": float64(1)}
	gs["offhand"] = map[string]interface{}{"item": "shield", "quantity": float64(1)}
	general(s)[0] = slot(0, "greatsword", 1)

	equip(t, s, "greatsword", 0, "general")

	if got := gearItem(s, "mainhand"); got != "greatsword" {
		t.Errorf("mainhand = %q, want greatsword", got)
	}
	if got := gearItem(s, "offhand"); got != "greatsword" {
		t.Errorf("offhand = %q, want greatsword (two-handed occupies both)", got)
	}
	// The displaced dagger + shield must both be back in general slots.
	found := map[string]bool{}
	for i := range general(s) {
		if id := slotItem(general(s), i); id != "" {
			found[id] = true
		}
	}
	if !found["dagger"] || !found["shield"] {
		t.Errorf("displaced items not returned to inventory: found %v", found)
	}
}

func TestUnequipReturnsToInventory(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	gearSlots(s)["mainhand"] = map[string]interface{}{"item": "longsword", "quantity": float64(1)}

	resp, err := inventory.HandleUnequipItemAction(s, p(map[string]interface{}{
		"equipment_slot": "mainhand",
	}))
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("unequip: resp=%+v err=%v", resp, err)
	}

	if got := gearItem(s, "mainhand"); got != "" {
		t.Errorf("mainhand = %q, want empty after unequip", got)
	}
	// Non-bag unequip targets the backpack first.
	if got := slotItem(backpack(s), 0); got != "longsword" {
		t.Errorf("backpack[0] = %q, want longsword", got)
	}
}

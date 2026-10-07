package inventory_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/types"
)

// The vault is one shared, slot-less pool that ignores stack limits, reached
// through any keeper who has accepted the player (schema v4). Deposit and
// withdraw are their own actions, not slot swaps.
func TestVaultRoundTrip(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	registerVault(s, "bank")
	general(s)[0] = slot(0, "longsword", 1)

	if _, err := inventory.HandleVaultDeposit(s, "general", 0, 0); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if got := vaultQty(s, "longsword"); got != 1 {
		t.Errorf("vault holds %d longswords, want 1 after deposit", got)
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want empty after deposit", got)
	}

	if _, err := inventory.HandleVaultWithdraw(s, "longsword", 1); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if got := vaultQty(s, "longsword"); got != 0 {
		t.Errorf("vault holds %d longswords, want 0 after withdraw", got)
	}
	// Placement is AddItemToInventory's business (it fills the backpack first
	// for anything that may nest), so assert it came back, not where it sits.
	if !carrying(s, "longsword") {
		t.Error("longsword is not in the inventory after withdraw")
	}
}

// carrying reports whether the player holds itemID in a general slot or the
// backpack.
func carrying(s *types.SaveFile, itemID string) bool {
	for _, arr := range [][]interface{}{general(s), backpack(s)} {
		for _, raw := range arr {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if id, _ := m["item"].(string); id == itemID {
				return true
			}
		}
	}
	return false
}

// The whole point of the rework: the vault ignores per-item stack limits, so
// deposits of the same item pile into one unbounded entry.
func TestVaultStacksBeyondItemStackLimit(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	registerVault(s, "bank")

	// Rations cap well below this in inventory; the vault should not care.
	for i := 0; i < 3; i++ {
		general(s)[i] = slot(i, "rations", 20)
		if _, err := inventory.HandleVaultDeposit(s, "general", i, 0); err != nil {
			t.Fatalf("deposit %d: %v", i, err)
		}
	}

	if got := vaultQty(s, "rations"); got != 60 {
		t.Errorf("vault holds %d rations, want 60 merged into one entry", got)
	}
	if got := len(s.Vault); got != 1 {
		t.Errorf("vault has %d entries, want 1 (same item merges)", got)
	}
}

// Depositing part of a stack leaves the remainder in the slot.
func TestVaultPartialDepositKeepsRemainder(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	registerVault(s, "bank")
	general(s)[0] = slot(0, "rations", 5)

	if _, err := inventory.HandleVaultDeposit(s, "general", 0, 2); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if got := vaultQty(s, "rations"); got != 2 {
		t.Errorf("vault holds %d rations, want 2", got)
	}
	if got := slotQty(general(s), 0); got != 3 {
		t.Errorf("general[0] qty = %d, want 3 left behind", got)
	}
}

// A withdrawal bigger than the carrying room comes back partial, and only what
// was actually carried leaves the vault.
func TestVaultWithdrawPartialWhenInventoryTight(t *testing.T) {
	setup(t)
	s := newSave(1, 0) // one general slot, no backpack
	registerVault(s, "bank")
	if err := vault.Deposit(s, "longsword", 5); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp, err := inventory.HandleVaultWithdraw(s, "longsword", 5)
	if err != nil || resp == nil || !resp.Success {
		t.Fatalf("withdraw: resp=%+v err=%v", resp, err)
	}
	carried := 5 - vaultQty(s, "longsword")
	if carried <= 0 || carried >= 5 {
		t.Errorf("carried %d of 5, want a partial withdrawal", carried)
	}
	if resp.Message == "" {
		t.Error("want a message explaining the partial withdrawal")
	}
}

// Vault containers are stored flat, so a container still holding things is
// refused rather than silently unpacked.
func TestVaultRefusesFullContainer(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	registerVault(s, "bank")
	pouch := slot(0, "pouch", 1)
	pouch["contents"] = []interface{}{slot(0, "rations", 1)}
	general(s)[0] = pouch

	resp, err := inventory.HandleVaultDeposit(s, "general", 0, 0)
	if err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if resp.Success {
		t.Error("deposit succeeded, want refusal for a full container")
	}
	if got := vaultQty(s, "pouch"); got != 0 {
		t.Errorf("vault holds %d pouches, want 0 (refused)", got)
	}
	if got := slotItem(general(s), 0); got != "pouch" {
		t.Errorf("general[0] = %q, want the pouch left alone", got)
	}
}

// An emptied container deposits fine, and empty containers stack like anything
// else in the vault.
func TestVaultAcceptsEmptyContainersAndStacksThem(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	registerVault(s, "bank")
	general(s)[0] = slot(0, "pouch", 1)
	general(s)[1] = slot(1, "pouch", 1)

	for i := 0; i < 2; i++ {
		resp, err := inventory.HandleVaultDeposit(s, "general", i, 0)
		if err != nil || !resp.Success {
			t.Fatalf("deposit %d: resp=%+v err=%v", i, resp, err)
		}
	}
	if got := vaultQty(s, "pouch"); got != 2 {
		t.Errorf("vault holds %d pouches, want 2 stacked", got)
	}
}

// Withdrawing a container must not smuggle it into the backpack — containers
// never nest.
func TestVaultWithdrawnContainerAvoidsBackpack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	registerVault(s, "bank")
	if err := vault.Deposit(s, "pouch", 1); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := inventory.HandleVaultWithdraw(s, "pouch", 1); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	for i, raw := range backpack(s) {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := m["item"].(string); id == "pouch" {
			t.Fatalf("pouch landed in backpack[%d]; containers must not nest", i)
		}
	}
	if got := slotItem(general(s), 0); got != "pouch" {
		t.Errorf("general[0] = %q, want the withdrawn pouch in a general slot", got)
	}
}

func TestDropRemovesEntireStack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "longsword", 1)

	resp, _, err := inventory.HandleDropItemAction(s, p(map[string]interface{}{
		"item_id": "longsword", "from_slot": float64(0), "from_slot_type": "general",
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

	resp, _, err := inventory.HandleDropItemAction(s, p(map[string]interface{}{
		"item_id": "rations", "from_slot": float64(0), "from_slot_type": "general", "quantity": float64(2),
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
		"item_id": "rations", "from_slot": float64(0),
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
		"item_id": "healing", "from_slot": float64(0),
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
		"item_id": "rations", "from_slot": float64(0),
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

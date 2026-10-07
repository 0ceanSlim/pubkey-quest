package inventory

import (
	"fmt"
	"log"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/types"
)

// Vault transfers. The vault is a flat, infinitely stacking pool with no slots
// (schema v4), so moving items in and out is not a slot swap — it is "take this
// stack out of a slot and pour it in" and "pour some back into whatever slots
// will hold it". That asymmetry is why these live apart from move_item.

// HandleVaultDeposit moves a whole stack (or qty of it, when qty > 0) out of an
// inventory slot and into the shared vault.
//
// The only refusal is a container that still has something in it: vault
// containers are stored flat, with no contents of their own, so the player
// empties it first. We say so rather than quietly unpacking it for them.
func HandleVaultDeposit(state *types.SaveFile, slotType string, slotIndex, qty int) (*types.GameActionResponse, error) {
	slots, err := inventorySlots(state, slotType)
	if err != nil {
		return nil, err
	}
	if slotIndex < 0 || slotIndex >= len(slots) {
		return nil, fmt.Errorf("slot %d out of range", slotIndex)
	}
	slot, ok := slots[slotIndex].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid slot data at %s[%d]", slotType, slotIndex)
	}
	itemID, _ := slot["item"].(string)
	if itemID == "" {
		return nil, fmt.Errorf("that slot is empty")
	}

	held := GetSlotQuantity(slot)
	if held <= 0 {
		held = 1
	}
	move := held
	if qty > 0 && qty < held {
		move = qty
	}

	if reason := containerBlocksDeposit(slot, itemID); reason != "" {
		return &types.GameActionResponse{Success: false, Error: reason, Color: "red"}, nil
	}

	if err := vault.Deposit(state, itemID, move); err != nil {
		return nil, err
	}

	if move >= held {
		clearSlot(slot)
	} else {
		slot["quantity"] = held - move
	}
	slots[slotIndex] = slot

	log.Printf("🏦 Deposited %dx %s into the shared vault (now %d held)", move, itemID, vault.Quantity(state, itemID))
	return &types.GameActionResponse{
		Success: true,
		Delta:   map[string]interface{}{"vault": vault.Response(state)},
	}, nil
}

// HandleVaultWithdraw takes qty of itemID out of the shared vault and puts it
// into the player's inventory. Inventory stack limits and free slots still
// apply, so a withdrawal can come back partial — in which case only what
// actually fit leaves the vault.
func HandleVaultWithdraw(state *types.SaveFile, itemID string, qty int) (*types.GameActionResponse, error) {
	if qty <= 0 {
		qty = 1
	}
	have := vault.Quantity(state, itemID)
	if have <= 0 {
		return nil, fmt.Errorf("the vault holds no %s", itemID)
	}
	if qty > have {
		qty = have
	}

	added, err := AddItemToInventory(state, itemID, qty)
	if err != nil {
		// Nothing fit — the vault is untouched, so this is a plain refusal.
		return &types.GameActionResponse{
			Success: false,
			Error:   "No room in your inventory for that",
			Color:   "red",
		}, nil
	}
	if added <= 0 {
		return &types.GameActionResponse{
			Success: false,
			Error:   "No room in your inventory for that",
			Color:   "red",
		}, nil
	}

	// Only remove what the player actually managed to carry away.
	if err := vault.Withdraw(state, itemID, added); err != nil {
		return nil, err
	}

	message := ""
	if added < qty {
		message = fmt.Sprintf("Only %d of %d would fit — the rest stays in the vault.", added, qty)
	}
	log.Printf("🏦 Withdrew %dx %s from the shared vault (%d left)", added, itemID, vault.Quantity(state, itemID))
	return &types.GameActionResponse{
		Success: true,
		Message: message,
		Color:   "yellow",
		Delta:   map[string]interface{}{"vault": vault.Response(state)},
	}, nil
}

// inventorySlots resolves a player-side slot array by slot type. The vault is
// not a valid slot type here — it has no slots.
func inventorySlots(state *types.SaveFile, slotType string) ([]interface{}, error) {
	switch slotType {
	case "general":
		slots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid general slots")
		}
		return slots, nil
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		contents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack")
		}
		return contents, nil
	default:
		return nil, fmt.Errorf("cannot deposit from slot type %q", slotType)
	}
}

// containerBlocksDeposit returns a player-facing reason when the slot holds a
// container that still has contents, and "" when the deposit may proceed.
func containerBlocksDeposit(slot map[string]interface{}, itemID string) string {
	itemData, err := db.GetItemByID(itemID)
	if err != nil || !itemHasTag(itemData.Tags, "container") {
		return ""
	}
	contents, ok := slot["contents"].([]interface{})
	if !ok {
		return ""
	}
	for _, raw := range contents {
		inner, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := inner["item"].(string); id != "" {
			return "The keeper won't take a full container — empty it first."
		}
	}
	return ""
}

// clearSlot empties a slot in place, keeping its index so the UI's slot
// numbering survives.
func clearSlot(slot map[string]interface{}) {
	slot["item"] = nil
	slot["quantity"] = 0
	delete(slot, "contents")
}

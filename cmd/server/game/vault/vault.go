// Package vault is the player's shared store. There is exactly one vault per
// character, reached from any keeper's door the player has been accepted at:
// the keepers hold keys to the same warded space, so what you leave in Ironpeak
// you collect in Goldenhaven.
//
// Two facts live in the save and neither is derivable (roadmap §4):
//   - SaveFile.Vault — the contents, one entry per item ID, quantity unbounded.
//     The vault deliberately ignores per-item stack limits; stacks are an
//     inventory-carrying constraint, not a storage one. An item with condition
//     (a food's made-time) gets an entry of its own and keeps it while banked.
//   - SaveFile.VaultKeepers — the building IDs whose keeper has accepted the
//     player. Access is per-city (you still have to make amends with each
//     keeper); the contents behind every door are the same.
package vault

import (
	"fmt"
	"log"
	"slices"

	"pubkey-quest/types"
)

// IsVaultRegistered reports whether the keeper at buildingID has accepted the
// player, i.e. whether this door opens onto the shared vault.
func IsVaultRegistered(state *types.SaveFile, buildingID string) bool {
	return buildingID != "" && slices.Contains(state.VaultKeepers, buildingID)
}

// RegisterVault records that the keeper at buildingID has accepted the player.
// Registering a second door does not create a second vault — it just adds
// another way into the one that already exists.
func RegisterVault(state *types.SaveFile, buildingID string) {
	if buildingID == "" || IsVaultRegistered(state, buildingID) {
		return
	}
	state.VaultKeepers = append(state.VaultKeepers, buildingID)
	log.Printf("✅ Vault keeper at %s accepted the player (%d door(s) now open)", buildingID, len(state.VaultKeepers))
}

// Contents returns the shared vault's entries, skipping anything that decayed
// to a non-positive quantity.
func Contents(state *types.SaveFile) []types.VaultEntry {
	out := make([]types.VaultEntry, 0, len(state.Vault))
	for _, e := range state.Vault {
		if e.ItemID != "" && e.Quantity > 0 {
			out = append(out, e)
		}
	}
	return out
}

// Quantity returns how many of itemID the vault holds (every entry of it).
func Quantity(state *types.SaveFile, itemID string) int {
	n := 0
	for _, e := range state.Vault {
		if e.ItemID == itemID {
			n += e.Quantity
		}
	}
	return n
}

// Deposit adds qty of itemID to the shared vault, merging into the existing
// entry when there is one. There is no capacity limit and no stack ceiling, so
// this only fails on nonsense input.
func Deposit(state *types.SaveFile, itemID string, qty int) error {
	return DepositWithState(state, itemID, qty, nil)
}

// DepositWithState banks items that carry condition: they get an entry of their
// own, so a stale fish stays stale. Without state it merges as Deposit does.
func DepositWithState(state *types.SaveFile, itemID string, qty int, itemState map[string]interface{}) error {
	if itemID == "" {
		return fmt.Errorf("no item to deposit")
	}
	if qty <= 0 {
		return fmt.Errorf("cannot deposit %d of %s", qty, itemID)
	}
	if len(itemState) == 0 {
		for i := range state.Vault {
			if state.Vault[i].ItemID == itemID && len(state.Vault[i].State) == 0 {
				state.Vault[i].Quantity += qty
				return nil
			}
		}
	}
	state.Vault = append(state.Vault, types.VaultEntry{ItemID: itemID, Quantity: qty, State: itemState})
	return nil
}

// Withdraw removes qty of itemID from the shared vault's plain (stateless)
// entry, dropping the entry when it empties. It fails rather than partially
// withdrawing when the vault holds less than asked.
func Withdraw(state *types.SaveFile, itemID string, qty int) error {
	if qty <= 0 {
		return fmt.Errorf("cannot withdraw %d of %s", qty, itemID)
	}
	for i := range state.Vault {
		if state.Vault[i].ItemID != itemID || len(state.Vault[i].State) > 0 {
			continue
		}
		if state.Vault[i].Quantity < qty {
			return fmt.Errorf("vault holds only %d of %s", state.Vault[i].Quantity, itemID)
		}
		state.Vault[i].Quantity -= qty
		if state.Vault[i].Quantity == 0 {
			state.Vault = slices.Delete(state.Vault, i, i+1)
		}
		return nil
	}
	return fmt.Errorf("vault holds no %s", itemID)
}

// TakeStateful removes the first entry of itemID that carries condition and
// returns it (ok false when there's none).
func TakeStateful(state *types.SaveFile, itemID string) (types.VaultEntry, bool) {
	for i, e := range state.Vault {
		if e.ItemID == itemID && len(e.State) > 0 {
			state.Vault = slices.Delete(state.Vault, i, i+1)
			return e, true
		}
	}
	return types.VaultEntry{}, false
}

// PutBack returns an entry taken with TakeStateful (when it didn't fit).
func PutBack(state *types.SaveFile, e types.VaultEntry) {
	state.Vault = append(state.Vault, e)
}

// Response is the wire shape the client renders: the shared contents plus the
// doors that open onto them. There are no slots — the UI grows to fit.
func Response(state *types.SaveFile) map[string]interface{} {
	entries := Contents(state)
	wire := make([]map[string]interface{}, 0, len(entries))
	for _, e := range entries {
		w := map[string]interface{}{
			"item_id":  e.ItemID,
			"quantity": e.Quantity,
		}
		if len(e.State) > 0 {
			w["state"] = e.State // the client draws its freshness bar
		}
		wire = append(wire, w)
	}
	return map[string]interface{}{
		"entries":  wire,
		"keepers":  state.VaultKeepers,
		"building": state.Building,
	}
}

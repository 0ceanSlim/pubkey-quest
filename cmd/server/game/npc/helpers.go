package npc

import (
	"slices"

	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/types"
)

// IsNativeRaceForLocation checks if a race is native to a location
func IsNativeRaceForLocation(race, location string) bool {
	nativeRaces := map[string][]string{
		"kingdom":           {"Human", "Half-Elf", "Half-Orc", "Tiefling"},
		"village-southwest": {"Orc"},
		"forest-kingdom":    {"Elf"},
		"hill-kingdom":      {"Dwarf"},
		"village-west":      {"Halfling"},
	}

	races, ok := nativeRaces[location]
	if !ok {
		return false
	}

	return slices.Contains(races, race)
}

// CheckDialogueRequirements checks if dialogue requirements are met
func CheckDialogueRequirements(state *types.SaveFile, requirements map[string]interface{}) bool {
	if requirements == nil {
		return true
	}

	if notNative, ok := requirements["not_native"].(bool); ok && notNative {
		if IsNativeRaceForLocation(state.Race, state.Location) {
			return false
		}
	}

	if notRegistered, ok := requirements["not_registered"].(bool); ok && notRegistered {
		if vault.IsVaultRegistered(state, state.Building) {
			return false
		}
	}

	if registered, ok := requirements["registered"].(bool); ok && registered {
		if !vault.IsVaultRegistered(state, state.Building) {
			return false
		}
	}

	// native: true — the opposite of not_native, for the birthright paths where
	// a local is let in at no cost (storage_config.free_for_races).
	if native, ok := requirements["native"].(bool); ok && native {
		if !IsNativeRaceForLocation(state.Race, state.Location) {
			return false
		}
	}

	if goldReq, ok := requirements["gold"].(float64); ok {
		if gameutil.GetGoldQuantity(state) < int(goldReq) {
			return false
		}
	}

	// items: [{"id": "iron-ore", "quantity": 5}] — the player must be carrying
	// all of them. Used by keepers who want goods rather than coin.
	for _, req := range ParseItemRequirements(requirements["items"]) {
		if gameutil.CountItem(state, req.ID) < req.Quantity {
			return false
		}
	}

	// quest_completed: "quest-id" — gates an option behind a finished quest.
	if questID, ok := requirements["quest_completed"].(string); ok && questID != "" {
		if !slices.Contains(state.QuestsCompleted, questID) {
			return false
		}
	}

	return true
}

// ItemRequirement is a quantity of one item a dialogue option asks for, either
// to check for or to take.
type ItemRequirement struct {
	ID       string
	Quantity int
}

// ParseItemRequirements reads an items list out of dialogue JSON. Entries are
// {"id": ..., "quantity": ...}; a missing or unreadable quantity means one.
// Anything malformed is skipped rather than failing the whole check, matching
// how the rest of the dialogue reader treats bad data.
func ParseItemRequirements(raw interface{}) []ItemRequirement {
	list, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var out []ItemRequirement
	for _, entry := range list {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		qty := 1
		switch v := m["quantity"].(type) {
		case float64:
			qty = int(v)
		case int:
			qty = v
		}
		if qty < 1 {
			qty = 1
		}
		out = append(out, ItemRequirement{ID: id, Quantity: qty})
	}
	return out
}

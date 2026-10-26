package types

import (
	"encoding/json"
	"strings"
)

// DurabilityFor is how much wear an item takes from pristine to ruined
// (docs/draft/degradation-system.md §4), from its own "durability" field and
// the category defaults in game-data/systems/condition.json. 0: it doesn't wear
// ("durability": false, quest items, ammunition, thrown-only weapons, anything
// that isn't a weapon or armour). The codex migration bakes the result into
// each item as "durability_max", so the server and the client just read it.
func DurabilityFor(raw json.RawMessage, itemType string, tags []string, defaults map[string]int) int {
	has := func(t string) bool {
		for _, tag := range tags {
			if strings.EqualFold(tag, t) {
				return true
			}
		}
		return false
	}
	if strings.TrimSpace(string(raw)) == "false" || has("quest_item") {
		return 0
	}
	var own struct {
		Max int `json:"max"`
	}
	if len(raw) > 0 && json.Unmarshal(raw, &own) == nil && own.Max > 0 {
		return own.Max
	}
	// Spent when used, so they don't wear.
	if has("ammunition") && !has("weapon") {
		return 0
	}
	if has("thrown") && !has("martial-melee") && !has("simple-melee") {
		return 0
	}
	if has("shield") {
		return defaults["shield"]
	}
	return defaults[itemType]
}

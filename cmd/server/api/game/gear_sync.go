package game

import (
	"sort"
	"strings"

	"pubkey-quest/types"
)

// gearSignature is what's equipped, slot by slot, so an action that changes
// the gear can be told from one that doesn't.
func gearSignature(state *types.SaveFile) string {
	gear, _ := state.Inventory["gear_slots"].(map[string]interface{})
	parts := make([]string, 0, len(gear))
	for slot, raw := range gear {
		m, _ := raw.(map[string]interface{})
		id, _ := m["item"].(string)
		parts = append(parts, slot+"="+id)
	}
	sort.Strings(parts)
	return strings.Join(parts, ";")
}

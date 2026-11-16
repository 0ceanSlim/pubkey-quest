package inventory

import (
	"fmt"
	"strings"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/types"
)

// Cracking a geode (mining-system.md §6): a pick's "use_on": "crack". It
// takes a few minutes and wears the pick a little, and the geode gives up one
// rough gem, rolled as a face of gems.json's geode_tier would hold.

// geodeTargets is every geode you carry.
func geodeTargets(state *types.SaveFile) []UseTarget {
	var out []UseTarget
	eachCarried(state, func(ref SlotRef, slot map[string]interface{}) {
		if id, _ := slot["item"].(string); id == types.GeodeID {
			out = append(out, UseTarget{SlotRef: ref, ItemID: id, Label: "Crack the " + condition.ItemName(id)})
		}
	})
	return out
}

// crackGeode cracks the geode in a slot with a pick.
func crackGeode(state *types.SaveFile, pickID string, slot map[string]interface{}) *types.GameActionResponse {
	if id, _ := slot["item"].(string); id != types.GeodeID {
		return refuse("There's nothing in that to crack.")
	}
	gc := gems.Config()
	g, found := gems.ForFace(gc, max(gc.Mining.GeodeTier, 1), nil, -1, gems.Global)
	if !found {
		return refuse("You can't make out anything inside it.")
	}
	if added, _ := AddItemToInventory(state, g.ID(), 1); added == 0 {
		return refuse("No room in your pack for what's inside.")
	}
	gameutil.ConsumeItem(state, types.GeodeID, 1)
	gametime.AdvanceTime(state, max(gc.Mining.CrackMinutes, 1), true)
	msg := fmt.Sprintf("You set the geode on a rock and tap round it with your %s until it splits. Inside, in a nest of crystal: a %s!",
		strings.ToLower(condition.ItemName(pickID)), strings.ToLower(condition.ItemName(g.ID())))
	if gc.Mining.CrackWear > 0 {
		var pick map[string]interface{}
		EachGear(state, func(_ SlotRef, s map[string]interface{}) {
			if id, _ := s["item"].(string); id == pickID && pick == nil {
				pick = s
			}
		})
		if pick != nil {
			if before, after := condition.AddWear(pick, gc.Mining.CrackWear); after > before {
				msg += fmt.Sprintf(" Your %s is %s now.", strings.ToLower(condition.ItemName(pickID)), strings.ToLower(after.String()))
			}
		}
	}
	return ok(msg)
}

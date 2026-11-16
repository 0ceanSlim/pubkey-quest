package inventory

import (
	"fmt"
	"strings"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/types"
)

// Pearls (gem-system.md §2): a shellfish gems.json lists (an oyster, mussels)
// may hold one. A dagger's "use_on": "shuck" opens it for the pearl and tosses
// the rest; eating it raw may turn one up too. Either way the pearl comes with
// its quality: pearls are never cut.

// shuckTargets is every shellfish you carry.
func shuckTargets(state *types.SaveFile) []UseTarget {
	var out []UseTarget
	gc := gems.Config()
	eachCarried(state, func(ref SlotRef, slot map[string]interface{}) {
		if id, _ := slot["item"].(string); gems.IsShellfish(gc, id) {
			out = append(out, UseTarget{SlotRef: ref, ItemID: id, Label: "Shuck the " + strings.ToLower(condition.ItemName(id))})
		}
	})
	return out
}

// shuck opens the shellfish in a slot with a dagger.
func shuck(state *types.SaveFile, toolID string, slot map[string]interface{}) *types.GameActionResponse {
	id, _ := slot["item"].(string)
	if !gems.IsShellfish(gems.Config(), id) {
		return refuse("There's nothing in that to open.")
	}
	gameutil.ConsumeItem(state, id, 1)
	msg := fmt.Sprintf("You work your %s into the %s and prise it open.", strings.ToLower(condition.ItemName(toolID)), strings.ToLower(condition.ItemName(id)))
	if found := pearlIn(state, id); found != "" {
		return ok(msg + " Inside: " + found)
	}
	return ok(msg + " No pearl. You toss the shell.")
}

// pearlIn rolls the pearl a shellfish (gone now) held and puts it in your
// pack: "a fine pearl (small)!" (or that it's lost for want of room), or ""
// for none.
func pearlIn(state *types.SaveFile, shellfishID string) string {
	g, found := gems.ForShellfish(gems.Config(), shellfishID, gems.Global)
	if !found {
		return ""
	}
	name := "a " + strings.ToLower(condition.ItemName(g.ID()))
	if added, _ := AddItemToInventory(state, g.ID(), 1); added == 0 {
		return name + ", but your pack's too full to keep it and it rolls away."
	}
	return name + "!"
}

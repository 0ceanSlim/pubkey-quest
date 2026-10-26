package inventory

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/types"
)

// Cooking and salting (food-system §1, §5): simple transforms, one item in and
// one out. A raw food's "cook" names what it cooks into and its "salt" what one
// salt preserves it as. Out in the wild, a tinderbox makes a fire: Cook cooks
// every raw food you carry in one go (30 minutes), and salt is used on one raw
// food at a time at the same fire. In town an innkeeper cooks for you.

// CookMinutes is how long a cooking fire takes.
const CookMinutes = 30

// Salt is the item salting uses up, one per food.
const Salt = "salt"

type transforms struct{ Cook, Salt string }

var (
	transformMu    sync.RWMutex
	transformCache = map[string]transforms{}
)

// transformOf is what a raw food cooks and salts into ("" for neither).
func transformOf(itemID string) transforms {
	transformMu.RLock()
	t, ok := transformCache[itemID]
	transformMu.RUnlock()
	if ok {
		return t
	}
	if item, err := db.GetItemByID(itemID); err == nil && item != nil {
		var props struct {
			Cook string `json:"cook"`
			Salt string `json:"salt"`
		}
		if json.Unmarshal([]byte(item.Properties), &props) == nil {
			t = transforms{props.Cook, props.Salt}
		}
	}
	transformMu.Lock()
	transformCache[itemID] = t
	transformMu.Unlock()
	return t
}

// CooksInto is what an item cooks into ("" when it doesn't cook).
func CooksInto(itemID string) string { return transformOf(itemID).Cook }

// FireHere says why you can't make a cooking fire here ("" when you can): out
// in the wild, stopped, with a tinderbox.
func FireHere(state *types.SaveFile) string {
	switch {
	case state.Building != "" || travel.GetEnvironmentData(state.Location) == nil:
		return "You need to be out in the wild to make a fire. In town, an innkeeper will cook for you."
	case !state.TravelStopped:
		return "Stop walking first to make a fire."
	case !gameutil.PlayerHasItem(state, "tinderbox"):
		return "You need a tinderbox to make a fire."
	}
	return ""
}

// eachRaw visits every carried slot holding a food that transforms.
func eachRaw(state *types.SaveFile, salt bool, visit func(ref SlotRef, slot map[string]interface{}, into string)) {
	eachCarried(state, func(ref SlotRef, slot map[string]interface{}) {
		id, _ := slot["item"].(string)
		t := transformOf(id)
		into := t.Cook
		if salt {
			into = t.Salt
		}
		if into != "" {
			visit(ref, slot, into)
		}
	})
}

// transform turns the food in a slot into what it becomes, freshly made.
func transform(state *types.SaveFile, slot map[string]interface{}, into string) {
	slot["item"] = into
	condition.ApplyState(slot, condition.NewState(into, condition.Now(state)))
}

// CookAll cooks every raw food you carry, over a fire you make here.
func CookAll(state *types.SaveFile) *types.GameActionResponse {
	if why := FireHere(state); why != "" {
		return refuse(why)
	}
	cooked := map[string]int{}
	var slots []map[string]interface{}
	var into []string
	eachRaw(state, false, func(_ SlotRef, slot map[string]interface{}, to string) {
		slots, into = append(slots, slot), append(into, to)
	})
	if len(slots) == 0 {
		return refuse("You've nothing raw to cook.")
	}
	gametime.AdvanceTime(state, CookMinutes, true)
	for i, slot := range slots {
		// The time at the fire may have spoiled it: spoiled food doesn't cook.
		if id, _ := slot["item"].(string); CooksInto(id) != into[i] {
			continue
		}
		cooked[condition.ItemName(into[i])] += max(GetSlotQuantity(slot), 1)
		transform(state, slot, into[i])
	}
	return ok("You build a small fire and cook. " + madeList(cooked) + ".")
}

// CookAllAtInn is the innkeeper cooking your raw food for you (no fire, no
// wait); the caller has charged for it.
func CookAllAtInn(state *types.SaveFile) map[string]int {
	cooked := map[string]int{}
	eachRaw(state, false, func(_ SlotRef, slot map[string]interface{}, to string) {
		cooked[condition.ItemName(to)] += max(GetSlotQuantity(slot), 1)
		transform(state, slot, to)
	})
	return cooked
}

// CountRawFood is how many raw foods you carry (what an innkeeper charges by).
func CountRawFood(state *types.SaveFile) int {
	n := 0
	eachRaw(state, false, func(_ SlotRef, slot map[string]interface{}, _ string) { n += max(GetSlotQuantity(slot), 1) })
	return n
}

// madeList is "2 Cooked Fish, 1 Cooked Meat".
func madeList(made map[string]int) string {
	var parts []string
	for name, n := range made {
		parts = append(parts, fmt.Sprintf("%d %s", n, name))
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

// MadeList is madeList for callers outside the package.
func MadeList(made map[string]int) string { return madeList(made) }

// saltTargets lists the raw foods salt would preserve.
func saltTargets(state *types.SaveFile) []UseTarget {
	var out []UseTarget
	eachRaw(state, true, func(ref SlotRef, slot map[string]interface{}, to string) {
		id, _ := slot["item"].(string)
		out = append(out, UseTarget{SlotRef: ref, ItemID: id,
			Label: fmt.Sprintf("Salt %s → %s", condition.ItemName(id), condition.ItemName(to))})
	})
	return out
}

// saltIt salts the raw food in a slot, at a fire, with one salt.
func saltIt(state *types.SaveFile, slot map[string]interface{}) *types.GameActionResponse {
	id, _ := slot["item"].(string)
	into := transformOf(id).Salt
	if into == "" {
		return refuse(fmt.Sprintf("You can't salt the %s.", condition.ItemName(id)))
	}
	if why := FireHere(state); why != "" {
		return refuse(strings.Replace(why, "make a fire", "dry it over a fire", 1))
	}
	gameutil.ConsumeItem(state, Salt, 1)
	transform(state, slot, into)
	return ok(fmt.Sprintf("You pack the %s in salt and dry it by the fire: %s.", condition.ItemName(id), condition.ItemName(into)))
}

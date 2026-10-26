package condition

import (
	"fmt"

	"pubkey-quest/types"
)

// Freshness tiers (degradation §3).
type Freshness int

const (
	Fresh   Freshness = iota // under half its shelf life: full nourishment and any bonus
	Stale                    // half to all of it: half the hours, no bonus
	Spoiled                  // past it: it becomes spoiled food
)

func (f Freshness) String() string {
	switch f {
	case Stale:
		return "stale"
	case Spoiled:
		return "spoiled"
	}
	return "fresh"
}

// FoodFreshness says how fresh a food slot is at game minute now, and how many
// minutes it has left before it spoils. Food that never spoils (or a slot not
// yet stamped) is fresh.
func FoodFreshness(slot map[string]interface{}, now int) (Freshness, int) {
	id, _ := slot["item"].(string)
	life := ItemInfo(id).ShelfLife
	made, ok := intField(slot, KeyMade)
	if life <= 0 || !ok {
		return Fresh, life
	}
	age := now - made
	left := life - age
	switch {
	case age >= life:
		return Spoiled, 0
	case age*2 >= life:
		return Stale, left
	}
	return Fresh, left
}

// SpoilSlots ages the food in some slot arrays to game minute now: food seen
// for the first time is stamped as made now, and food past its shelf life
// becomes one stack of spoiled food (merged into a spoiled stack with room in
// the same array), or vanishes if it's magic (goodberries). It only writes
// when something changes, so it's cheap to run on every time advance. The
// messages name what spoiled.
func SpoilSlots(arrays [][]interface{}, now int) []string {
	var msgs []string
	for _, slots := range arrays {
		for i, raw := range slots {
			slot, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := slot["item"].(string)
			if id == "" || id == SpoiledFood {
				continue
			}
			info := ItemInfo(id)
			if info.ShelfLife <= 0 {
				continue
			}
			if _, stamped := intField(slot, KeyMade); !stamped {
				slot[KeyMade] = now
				continue
			}
			if f, _ := FoodFreshness(slot, now); f != Spoiled {
				continue
			}
			name := ItemName(id)
			qty, _ := intField(slot, "quantity")
			qty = max(qty, 1)
			ClearState(slot)
			if info.Vanish {
				slot["item"], slot["quantity"] = nil, 0
				msgs = append(msgs, fmt.Sprintf("Your %s lost its magic and is gone.", name))
				continue
			}
			slot["item"], slot["quantity"] = SpoiledFood, qty
			mergeSpoiled(slots, i)
			msgs = append(msgs, fmt.Sprintf("Your %s has spoiled.", name))
		}
	}
	return msgs
}

// spoiledStack is how many spoiled food fit in one slot.
const spoiledStack = 10

// mergeSpoiled folds the freshly spoiled slot at index i into another
// spoiled-food stack in the same array that has room, emptying it if it fits.
func mergeSpoiled(slots []interface{}, i int) {
	fresh, _ := slots[i].(map[string]interface{})
	for j, raw := range slots {
		other, ok := raw.(map[string]interface{})
		if !ok || j == i || other["item"] != SpoiledFood {
			continue
		}
		have, _ := intField(other, "quantity")
		add, _ := intField(fresh, "quantity")
		room := spoiledStack - have
		if room <= 0 {
			continue
		}
		move := min(room, add)
		other["quantity"] = have + move
		if add-move <= 0 {
			fresh["item"], fresh["quantity"] = nil, 0
			return
		}
		fresh["quantity"] = add - move
	}
}

// SpoilVault ages the food banked in a vault the same way. Vault entries keep
// their own state (types.VaultEntry.State).
func SpoilVault(vault []types.VaultEntry, now int) ([]types.VaultEntry, []string) {
	var msgs []string
	out := vault[:0]
	for _, e := range vault {
		info := ItemInfo(e.ItemID)
		if info.ShelfLife > 0 {
			slot := map[string]interface{}{"item": e.ItemID, "quantity": e.Quantity}
			ApplyState(slot, e.State)
			if _, stamped := intField(slot, KeyMade); !stamped {
				e.State = map[string]interface{}{KeyMade: now}
			} else if f, _ := FoodFreshness(slot, now); f == Spoiled {
				name := ItemName(e.ItemID)
				if info.Vanish {
					msgs = append(msgs, fmt.Sprintf("The %s in your vault lost its magic.", name))
					continue
				}
				msgs = append(msgs, fmt.Sprintf("The %s in your vault has spoiled.", name))
				e = types.VaultEntry{ItemID: SpoiledFood, Quantity: e.Quantity}
			}
		}
		out = append(out, e)
	}
	return out, msgs
}

// SellShare is the percentage of its price an item sells for in its condition:
// fresh food at full price, stale at half, spoiled for nothing; worn gear at
// its tier's share (Worn 75%, Damaged 50%, Ruined 25%).
func SellShare(itemID string, state map[string]interface{}, now int) int {
	slot := map[string]interface{}{"item": itemID}
	ApplyState(slot, state)
	share := 100
	switch f, _ := FoodFreshness(slot, now); f {
	case Stale:
		share = 50
	case Spoiled:
		share = 0
	}
	tier, _ := WearTier(slot)
	return share * WearSellPercent(tier) / 100
}

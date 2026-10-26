package combat

import (
	"fmt"
	"strings"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/types"
)

// Wear in a fight (degradation §4). Only the players' gear wears; monster gear
// doesn't track.

// gearSlotMap is a gear slot's map ("mainhand", "chest"), nil when empty.
func gearSlotMap(inventory map[string]interface{}, name string) map[string]interface{} {
	gear, _ := inventory["gear_slots"].(map[string]interface{})
	slot, _ := gear[strings.ToLower(name)].(map[string]interface{})
	if slot == nil || slot["item"] == nil || slot["item"] == "" {
		return nil
	}
	return slot
}

// weaponWear cuts a hit's damage by the weapon's tier (after the roll, never
// below 1), then wears the weapon: 1 point, 2 on a crit. Two-handed weapons
// keep their wear on the main hand.
func weaponWear(save *types.SaveFile, weaponSlot, name string, dmg int, crit bool) (int, []string) {
	slot := gearSlotMap(save.Inventory, weaponSlot)
	if slot == nil || dmg <= 0 {
		return dmg, nil
	}
	var log []string
	tier, _ := condition.WearTier(slot)
	if pct := condition.DamagePercent(tier); pct < 100 {
		cut := max(1, dmg*pct/100)
		log = append(log, fmt.Sprintf("  ⚙ Your %s is %s: %d → %d damage (−%d%%).", name, strings.ToLower(tier.String()), dmg, cut, 100-pct))
		dmg = cut
	}
	n := 1
	if crit {
		n = 2
	}
	if before, after := condition.AddWear(slot, n); after > before {
		log = append(log, fmt.Sprintf("  ⚙ Your %s is %s now.", name, strings.ToLower(after.String())))
	}
	return dmg, log
}

// armorWear wears one piece of the armour a hit landed on: body armour first,
// then the legs, then a shield. It says so when a piece drops a tier.
func armorWear(save *types.SaveFile) []string {
	for _, name := range []string{"chest", "legs", "offhand"} {
		slot := gearSlotMap(save.Inventory, name)
		if slot == nil {
			continue
		}
		id, _ := slot["item"].(string)
		if condition.MaxDurability(id) <= 0 {
			continue
		}
		// In the off hand only a shield takes the blow (not a dagger or a torch).
		if name == "offhand" {
			if item, err := loadItemProps(serverdb.GetDB(), id); err != nil || !hasTag(item["tags"], "shield") {
				continue
			}
		}
		if before, after := condition.AddWear(slot, 1); after > before {
			return []string{fmt.Sprintf("  ⚙ Your %s is %s now.", condition.ItemName(id), strings.ToLower(after.String()))}
		}
		return nil
	}
	return nil
}

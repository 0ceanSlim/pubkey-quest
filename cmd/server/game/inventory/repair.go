package inventory

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// Field repair (degradation §5): a whetstone sharpens edged melee weapons and
// a crafting kit mends armour, shields and the rest of your weapons, both only
// back up to Worn. Tools "use_on": "sharpen" and "mend".

// SkillOf is the player's effective skill score (base stats + effects + carried
// tools). The api layer sets it; inventory can't build a quest context itself.
var SkillOf = func(state *types.SaveFile, skill string) int { return 10 }

// repairKind is the field repair an item takes: "sharpen", "mend" or "".
func repairKind(itemID string) string {
	item, err := db.GetItemByID(itemID)
	if err != nil || item == nil || condition.MaxDurability(itemID) == 0 {
		return ""
	}
	var props struct {
		DamageType string `json:"damage_type"`
	}
	_ = json.Unmarshal([]byte(item.Properties), &props)
	if strings.Contains(item.Type, "Melee Weapons") {
		for _, t := range condition.Repair().Whetstone.DamageTypes {
			if t == props.DamageType {
				return "sharpen"
			}
		}
	}
	return "mend"
}

// EachGear visits what you carry and everything you wear.
func EachGear(state *types.SaveFile, visit func(ref SlotRef, slot map[string]interface{})) {
	eachCarried(state, visit)
	gear, _ := state.Inventory["gear_slots"].(map[string]interface{})
	for _, name := range []string{"mainhand", "chest", "legs"} {
		if slot, ok := gear[name].(map[string]interface{}); ok && slot["item"] != nil && slot["item"] != "" {
			visit(SlotRef{Type: "equipment", Gear: name}, slot)
		}
	}
}

// repairTargets lists the worn gear a repair tool would mend.
func repairTargets(state *types.SaveFile, use string) []UseTarget {
	var out []UseTarget
	EachGear(state, func(ref SlotRef, slot map[string]interface{}) {
		id, _ := slot["item"].(string)
		if repairKind(id) != use || !condition.FieldRepairable(slot) {
			return
		}
		tier, _ := condition.WearTier(slot)
		verb := "Sharpen"
		if use == "mend" {
			verb = "Mend"
		}
		out = append(out, UseTarget{SlotRef: ref, ItemID: id,
			Label: fmt.Sprintf("%s %s (%s)", verb, condition.ItemName(id), tier)})
	})
	return out
}

// fieldRepair mends one item with a whetstone or crafting kit: time passes and,
// for the kit, a Crafting check decides whether it comes back to Worn or only
// halfway there.
func fieldRepair(state *types.SaveFile, toolID, use string, slot map[string]interface{}) *types.GameActionResponse {
	id, _ := slot["item"].(string)
	name := condition.ItemName(id)
	if repairKind(id) != use {
		if use == "sharpen" {
			return refuse(fmt.Sprintf("Your %s has no edge to hone.", name))
		}
		return refuse(fmt.Sprintf("Your %s needs a whetstone.", name))
	}
	if !condition.FieldRepairable(slot) {
		return refuse(fmt.Sprintf("Your %s is as good as you can make it out here. A smith could do better.", name))
	}
	rules := condition.Repair()
	if use == "sharpen" {
		gametime.AdvanceTime(state, max(rules.Whetstone.Minutes, 1), true)
		condition.FieldRepair(slot, 1)
		tier, _ := condition.WearTier(slot)
		msg := fmt.Sprintf("You work the %s along your %s's edge. It's %s now.", condition.ItemName(toolID), name, strings.ToLower(tier.String()))
		// A whetstone wears smooth after its sharpenings (degradation §7).
		if left, gone, found := gameutil.SpendCharge(state, toolID); gone {
			msg += fmt.Sprintf(" The %s is worn smooth.", condition.ItemName(toolID))
		} else if found && left <= 2 {
			msg += fmt.Sprintf(" The %s is wearing thin (%d left).", condition.ItemName(toolID), left)
		}
		return ok(msg)
	}

	kit := rules.CraftingKit
	gametime.AdvanceTime(state, max(kit.Minutes, 1), true)
	check := skillcheck.Resolve(SkillOf(state, kit.Skill), kit.DC, rand.New(rand.NewSource(time.Now().UnixNano())))
	roll := fmt.Sprintf("(Crafting: rolled %d%+d vs DC %d)", check.Roll, check.Modifier, check.DC)
	if check.Success {
		condition.FieldRepair(slot, 1)
		tier, _ := condition.WearTier(slot)
		return ok(fmt.Sprintf("You patch and refit your %s. It's %s now. %s", name, strings.ToLower(tier.String()), roll))
	}
	condition.FieldRepair(slot, 0.5)
	tier, _ := condition.WearTier(slot)
	return ok(fmt.Sprintf("Your repair on the %s is rough: it helps, a little. It's %s. %s", name, strings.ToLower(tier.String()), roll))
}

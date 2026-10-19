package combat

import (
	"encoding/json"
	"fmt"

	"pubkey-quest/types"
)

// throwItem resolves a throwable item (an item with a "throw" block) against
// the chosen monster: a ranged attack with the item's range — Dexterity, no
// proficiency (5e treats these as improvised weapons) — then its damage and
// any condition it lands (a net restrains, alchemist's fire keeps burning).
// It costs the action, and the item is used up whether it hits or not.
//
//	"throw": {"damage": "2d6", "damage_type": "acid",
//	          "condition": {"condition": "restrained", "save_each_turn": "strength"}, "dc": 10}
func throwItem(cs *types.CombatSession, save *types.SaveFile, item, throw map[string]interface{}, itemID, name string, slot map[string]interface{}, fromPack bool, targetID string) ([]string, error) {
	me := actor(cs)
	state := &me.CombatState
	monster, err := ResolveTarget(cs, targetID)
	if err != nil {
		return nil, err
	}
	r := chebyshev(me.Pos, monster.Pos)
	normal, long := getRangedReach(item)
	if long < normal {
		long = normal
	}
	if r > long {
		return nil, fmt.Errorf("%s is out of throwing range for the %s", monster.Name, name)
	}

	var log []string
	if fromPack {
		log = append(log, fmt.Sprintf("  You dig the %s out of your pack (bonus action).", name))
	}
	adv := ConditionAttackAdvantage(state.Conditions, monster.Conditions)
	if r <= 1 {
		adv-- // point-blank
	}
	if r > normal {
		adv-- // long range
	}
	if inDarkness(cs, save) {
		adv--
		log = append(log, "  🌑 It's dark — disadvantage (no darkvision, no light in hand).")
	}
	bonus := StatMod(GetStatFromMap(effectiveStats(save), "dexterity"))
	result := ResolveAttackRoll(bonus, monster.ArmorClass, adv)
	log = append(log, fmt.Sprintf("  You throw the %s at %s: rolled %d%s%s", name, monster.Name, result.Roll, formatModifier(result.Modifier), AdvNote(result)), outcomeLine(result))

	decrementSlotStack(slot)
	state.ActionUsed = true
	if fromPack {
		state.BonusActionUsed = true
	}
	if !result.IsHit {
		return log, nil
	}

	// The condition lands first, so a net on a creature that also takes damage
	// still holds it if the hit doesn't kill it.
	if raw, ok := throw["condition"].(map[string]interface{}); ok {
		var c types.SpellCondition
		if b, err := json.Marshal(raw); err == nil && json.Unmarshal(b, &c) == nil && c.Condition != "" {
			dc := 10
			if v, ok := throw["dc"].(float64); ok && v > 0 {
				dc = int(v)
			}
			if line, _ := landCondition(monster, &c, itemID, me.ID, dc); line != "" {
				log = append(log, line)
			}
		}
	}
	if dice, _ := throw["damage"].(string); dice != "" {
		dtype, _ := throw["damage_type"].(string)
		dmg := ResolveDamageToMonster(dice, 0, dtype, result.IsCrit, monster)
		crit := ""
		if result.IsCrit {
			crit = " Critical hit!"
		}
		log = append(log, fmt.Sprintf("  You deal %d %s damage (%s).%s", dmg, dtype, DiceLabel(dice, 0, result.IsCrit), crit))
		log = append(log, spellDamageToMonster(cs, me, save, monster, dmg)...)
	}
	return log, nil
}

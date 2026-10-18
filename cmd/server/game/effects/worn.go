package effects

import "strings"

// WornEffectIDs lists the effects an equipped item grants its wearer: its
// effects_when_worn, plus any effects_when_worn_by_race entry for the wearer's
// race (the Axe of the Ancestors' dwarf-only fury), matched case-insensitively.
// Equip, unequip and load all use this, so the three agree.
func WornEffectIDs(properties map[string]interface{}, race string) []string {
	var ids []string
	add := func(raw interface{}) {
		list, _ := raw.([]interface{})
		for _, v := range list {
			if s, ok := v.(string); ok && s != "" {
				ids = append(ids, s)
			}
		}
	}
	add(properties["effects_when_worn"])
	if byRace, ok := properties["effects_when_worn_by_race"].(map[string]interface{}); ok {
		for r, list := range byRace {
			if strings.EqualFold(r, race) {
				add(list)
			}
		}
	}
	return ids
}

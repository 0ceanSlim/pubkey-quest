package combat

import (
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// takeCoatings turns the save's weapon coatings (active effects carrying a
// combat_buff: basic poison, a honed edge) into fight buffs and uses the
// effects up. Called when a fight starts, and right after a coating is applied
// mid-fight.
func takeCoatings(save *types.SaveFile) []types.SpellBuff {
	if save == nil {
		return nil
	}
	var buffs []types.SpellBuff
	var spent []string
	seen := map[string]bool{}
	for _, ae := range save.ActiveEffects {
		if seen[ae.EffectID] {
			continue // honed twice is still one honed edge
		}
		data, err := effects.LoadEffectData(ae.EffectID)
		if err != nil || data.CombatBuff == nil || data.Removal.Type == "equipment" {
			continue // a worn piece's buff isn't a coating (wornBuffs)
		}
		seen[ae.EffectID] = true
		b := *data.CombatBuff
		b.SpellID = data.ID
		if b.Name == "" {
			b.Name = data.Name
		}
		buffs = append(buffs, b)
		spent = append(spent, ae.EffectID)
	}
	for _, id := range spent {
		effects.RemoveEffect(save, id)
	}
	return buffs
}

// wornBuffs are the fight buffs worn gear carries (an enchanted ring's fire on
// a hit, its resistances): every worn effect with a combat_buff, for the whole
// fight, never used up. The same buff from two pieces (one spell_id) counts
// once, the higher rank winning, so a ruby ring and a ruby necklace give one
// fire rider. Called when a fight starts.
func wornBuffs(save *types.SaveFile) []types.SpellBuff {
	if save == nil {
		return nil
	}
	type pick struct {
		buff types.SpellBuff
		rank int
	}
	best := map[string]pick{}
	var order []string
	for _, ae := range save.ActiveEffects {
		data, err := effects.LoadEffectData(ae.EffectID)
		if err != nil || data.CombatBuff == nil || data.Removal.Type != "equipment" {
			continue
		}
		b := *data.CombatBuff
		if b.SpellID == "" {
			b.SpellID = data.ID
		}
		if b.Name == "" {
			b.Name = data.Name
		}
		old, seen := best[b.SpellID]
		if !seen {
			order = append(order, b.SpellID)
		}
		if !seen || data.Rank > old.rank {
			best[b.SpellID] = pick{b, data.Rank}
		}
	}
	buffs := make([]types.SpellBuff, 0, len(order))
	for _, id := range order {
		buffs = append(buffs, best[id].buff)
	}
	return buffs
}

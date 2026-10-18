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
		if err != nil || data.CombatBuff == nil {
			continue
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

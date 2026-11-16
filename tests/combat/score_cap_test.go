package combat_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// No ability gets past 20, whatever an effect adds; penalties still bite.
func TestAbilitiesStopAtTwenty(t *testing.T) {
	combatSetup(t)

	save := fighterSave()
	save.Stats = map[string]interface{}{
		"Strength": 20.0, "Dexterity": 12.0, "Constitution": 14.0,
		"Intelligence": 10.0, "Wisdom": 19.0, "Charisma": 20.0,
	}
	save.ActiveEffects = []types.ActiveEffect{
		{EffectID: "blessed", EffectIndex: 0}, // WIS +2
		{EffectID: "blessed", EffectIndex: 1}, // CHA +1
		{EffectID: "ancestral-might"},         // STR +1
	}
	eff := effects.EffectiveStats(save)
	for _, a := range []string{"strength", "wisdom", "charisma"} {
		if got := combat.GetStatFromMap(eff, a); got != types.ScoreCap {
			t.Errorf("%s = %d, want the cap of %d", a, got, types.ScoreCap)
		}
	}
	if got := effects.Ability(save, "charisma"); got != types.ScoreCap {
		t.Errorf("Ability(charisma) = %d, want %d (stats are keyed \"Charisma\")", got, types.ScoreCap)
	}

	save.ActiveEffects = []types.ActiveEffect{{EffectID: "fatigued", EffectIndex: 1}} // STR -2
	if got := effects.Ability(save, "Strength"); got >= 20 {
		t.Errorf("a penalty should still lower a capped score: STR %d", got)
	}
}

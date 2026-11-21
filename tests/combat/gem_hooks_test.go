package combat_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// Moonwell's mana comes only at night (23:00–05:00).
func TestNightOnlyTick(t *testing.T) {
	combatSetup(t)
	save := fighterSave()
	save.Mana, save.MaxMana = 0, 50
	save.ActiveEffects = []types.ActiveEffect{{EffectID: "jewel-moonstone-huge", EffectIndex: 2}} // the mana tick
	save.TimeOfDay = 720
	effects.TickEffects(save, 120, true)
	if save.Mana != 0 {
		t.Fatalf("mana %d at noon", save.Mana)
	}
	save.TimeOfDay = 60
	effects.TickEffects(save, 120, true)
	if save.Mana != 4 {
		t.Fatalf("mana %d after two hours of night, want 4", save.Mana)
	}
}

// Unbroken: fatigue builds half as fast.
func TestFatigueSlow(t *testing.T) {
	combatSetup(t)
	tired := func(worn bool) int {
		save := fighterSave()
		save.Fatigue = 0
		save.Stats = map[string]interface{}{"strength": 10, "dexterity": 10, "constitution": 10, "intelligence": 10, "wisdom": 10, "charisma": 10}
		save.ActiveEffects = []types.ActiveEffect{{EffectID: "fatigue-accumulation"}}
		if worn {
			save.ActiveEffects = append(save.ActiveEffects, types.ActiveEffect{EffectID: "jewel-jasper-huge", EffectIndex: 3}) // fatigue_slow
		}
		for i := 0; i < 8; i++ {
			effects.TickEffects(save, 60, true)
		}
		return save.Fatigue
	}
	plain, steady := tired(false), tired(true)
	if plain == 0 || steady*2 > plain+1 {
		t.Fatalf("fatigue over 8 hours: %d plain, %d with Unbroken", plain, steady)
	}
}

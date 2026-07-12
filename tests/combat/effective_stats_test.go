package combat_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// Combat now reads effective (effect-modified) stats, so fatigue/exhaustion make a
// player fight worse. Verify a "fatigued" debuff (DEX-3/STR-2/WIS-1) lowers both the
// effective DEX combat rolls read and the resulting AC.
// (combatSetup + fighterSave live in abilities_integration_test.go — same package.)
func TestFatigueLowersCombatStats(t *testing.T) {
	combatSetup(t)

	save := fighterSave() // DEX 12 (mod +1), unarmored

	// Baseline: no effects → effective == base.
	baseDEX := combat.GetStatFromMap(effects.EffectiveStats(save), "dexterity")
	baseAC := combat.CalculatePlayerAC(db.GetDB(), save.Inventory, effects.EffectiveStats(save))

	// Apply the fatigued condition (what fatigue ≥9 grants: DEX-3, STR-2, WIS-1).
	save.ActiveEffects = []types.ActiveEffect{{EffectID: "fatigued"}}
	tiredStats := effects.EffectiveStats(save)
	tiredDEX := combat.GetStatFromMap(tiredStats, "dexterity")
	tiredAC := combat.CalculatePlayerAC(db.GetDB(), save.Inventory, tiredStats)

	if tiredDEX >= baseDEX {
		t.Errorf("fatigued should lower effective DEX: base %d, fatigued %d", baseDEX, tiredDEX)
	}
	if tiredAC >= baseAC {
		t.Errorf("fatigued should lower AC (via effective DEX): base %d, fatigued %d", baseAC, tiredAC)
	}
}

package combat

import (
	"testing"

	"pubkey-quest/types"
)

// Elemental Adept: spells of the chosen type ignore the target's resistance but
// still respect immunity, and every damage die counts at least 2.
func TestElementalAdeptIgnoresResistance(t *testing.T) {
	resistFire := &types.MonsterInstance{Data: types.MonsterData{DamageResistances: []string{"fire"}}}

	// 2d6 with 1s→2 is at least 4; if resistance were (wrongly) applied it would
	// halve to at least 2. Over many rolls the minimum must stay ≥ 4.
	minAdept := 1 << 30
	for i := 0; i < 300; i++ {
		if d := resolveElementalAdeptDamage("2d6", 0, "fire", false, resistFire); d < minAdept {
			minAdept = d
		}
	}
	if minAdept < 4 {
		t.Errorf("Elemental Adept fire vs a fire-resistant monster should not be halved (min %d, want ≥4)", minAdept)
	}

	// Immunity still zeroes it.
	immuneFire := &types.MonsterInstance{Data: types.MonsterData{DamageImmunities: []string{"fire"}}}
	if got := resolveElementalAdeptDamage("2d6", 0, "fire", false, immuneFire); got != 0 {
		t.Errorf("immunity should still zero the damage, got %d", got)
	}
}

func TestRollDiceMinTwo(t *testing.T) {
	// Every die counts at least 2, so 4d4 is always ≥ 8.
	for i := 0; i < 200; i++ {
		if got := rollDiceMinTwo("4d4", false); got < 8 {
			t.Fatalf("rollDiceMinTwo(4d4) = %d, want ≥ 8", got)
		}
	}
	// Crit doubles the dice count → 4d4 crit is always ≥ 16.
	if got := rollDiceMinTwo("4d4", true); got < 16 {
		t.Errorf("rollDiceMinTwo(4d4, crit) = %d, want ≥ 16", got)
	}
}

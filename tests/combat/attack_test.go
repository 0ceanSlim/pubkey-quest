package combat_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/combat"
)

// Proficiency bonus must scale correctly across the alpha level band (and
// beyond). UnarmedAttackBonus = STR mod + proficiency; with STR 10 (mod 0) it
// equals the proficiency bonus, isolating that scaling.
func TestProficiencyScaling(t *testing.T) {
	stats := map[string]interface{}{"strength": float64(10)} // mod 0
	cases := []struct{ level, want int }{
		{1, 2}, {4, 2}, {5, 3}, {8, 3}, {9, 4}, {13, 5}, {17, 6},
	}
	for _, c := range cases {
		if got := combat.UnarmedAttackBonus(stats, "Fighter", c.level); got != c.want {
			t.Errorf("proficiency at level %d = %d, want %d", c.level, got, c.want)
		}
	}
}

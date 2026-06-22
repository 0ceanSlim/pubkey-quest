package character_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// The per-level XP multiplier (advancement.json XPMultiplier) must apply to base
// XP from any source — not only combat. BonusedXP is the shared helper non-combat
// sources use before GrantXP.
func TestBonusedXP(t *testing.T) {
	adv := testAdvancement() // XPMultiplier = 1.0 + 0.05*(level-1)

	cases := []struct {
		name  string
		xp    int   // total XP → sets level
		base  int
		want  int
	}{
		{"level 1 ×1.00", xpForLevel(1), 100, 100},
		{"level 2 ×1.05", xpForLevel(2), 100, 105},
		{"level 5 ×1.20", xpForLevel(5), 100, 120},
		{"level 20 ×1.95", xpForLevel(20), 200, 390},
		{"rounds to nearest", xpForLevel(2), 10, 11}, // 10 * 1.05 = 10.5 → 11
	}
	for _, c := range cases {
		save := &types.SaveFile{Experience: c.xp}
		if got := character.BonusedXP(save, c.base, adv); got != c.want {
			t.Errorf("%s: BonusedXP(base=%d) = %d, want %d", c.name, c.base, got, c.want)
		}
	}

	// Nil save returns the base unchanged (defensive).
	if got := character.BonusedXP(nil, 50, adv); got != 50 {
		t.Errorf("BonusedXP(nil) = %d, want 50", got)
	}
}

// BonusXP is the single level-based multiplier the combat per-hit path and the
// save-based BonusedXP both route through.
func TestBonusXP(t *testing.T) {
	adv := testAdvancement() // XPMultiplier = 1.0 + 0.05*(level-1)
	cases := []struct{ level, base, want int }{
		{1, 100, 100},  // ×1.00
		{5, 100, 120},  // ×1.20
		{20, 200, 390}, // ×1.95
		{2, 10, 11},    // round(10.5)
	}
	for _, c := range cases {
		if got := character.BonusXP(c.level, c.base, adv); got != c.want {
			t.Errorf("BonusXP(level=%d, base=%d) = %d, want %d", c.level, c.base, got, c.want)
		}
	}
}

// The guide surfaces the multiplier so the player can see the bonus per level.
func TestBuildLevelGuide_XPMultiplier(t *testing.T) {
	adv := testAdvancement()
	save := &types.SaveFile{Class: "Fighter", Experience: xpForLevel(1),
		Stats: map[string]interface{}{"Constitution": float64(12)}}
	guide := character.BuildLevelGuide(save, adv, nil, nil)

	if guide[1].XPMultiplier <= guide[0].XPMultiplier {
		t.Errorf("XP multiplier should climb with level: L1=%v L2=%v", guide[0].XPMultiplier, guide[1].XPMultiplier)
	}
	if guide[4].XPMultiplier < 1.19 || guide[4].XPMultiplier > 1.21 { // level 5 ≈ 1.20
		t.Errorf("level 5 XP multiplier = %v, want ~1.20", guide[4].XPMultiplier)
	}
}

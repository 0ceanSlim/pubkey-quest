package combat_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/types"
)

func TestKillBonusXP(t *testing.T) {
	// No kill bonus defined → 0.
	if got := combat.KillBonusXP(&types.MonsterData{XP: 100}, 1.0); got != 0 {
		t.Errorf("no kill bonus = %d, want 0", got)
	}
	// Defined bonus, scaled by the advancement XP multiplier.
	boss := &types.MonsterData{XP: 100, KillBonusXP: 50}
	if got := combat.KillBonusXP(boss, 1.0); got != 50 {
		t.Errorf("kill bonus = %d, want 50", got)
	}
	if got := combat.KillBonusXP(boss, 1.2); got != 60 {
		t.Errorf("scaled kill bonus = %d, want 60", got)
	}
	// Nil-safe.
	if got := combat.KillBonusXP(nil, 1.0); got != 0 {
		t.Errorf("nil monster = %d, want 0", got)
	}
}

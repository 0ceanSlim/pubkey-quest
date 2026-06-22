package combat_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/types"
)

func TestKillBonusXP(t *testing.T) {
	// No kill bonus defined → 0.
	if got := combat.KillBonusXP(&types.MonsterData{XP: 100}); got != 0 {
		t.Errorf("no kill bonus = %d, want 0", got)
	}
	// Returns the flat base bonus; the level multiplier is applied separately
	// (character.BonusXP), not here.
	boss := &types.MonsterData{XP: 100, KillBonusXP: 50}
	if got := combat.KillBonusXP(boss); got != 50 {
		t.Errorf("kill bonus = %d, want 50", got)
	}
	// Nil-safe.
	if got := combat.KillBonusXP(nil); got != 0 {
		t.Errorf("nil monster = %d, want 0", got)
	}
}

func TestXPForDamage(t *testing.T) {
	m := &types.MonsterInstance{MaxHP: 10, Data: types.MonsterData{XP: 100}}
	// 100 XP / 10 HP = 10 per HP; 3 damage, day → 30 base XP (level multiplier
	// applied elsewhere).
	if got := combat.XPForDamage(m, 3, 1.0); got != 30 {
		t.Errorf("XPForDamage = %d, want 30", got)
	}
	// Night ×1.25 → int(37.5) = 37.
	if got := combat.XPForDamage(m, 3, 1.25); got != 37 {
		t.Errorf("night XPForDamage = %d, want 37", got)
	}
	// Minimum 1 XP on any damaging hit, even for low-value monsters.
	weak := &types.MonsterInstance{MaxHP: 100, Data: types.MonsterData{XP: 5}}
	if got := combat.XPForDamage(weak, 1, 1.0); got != 1 {
		t.Errorf("min XP = %d, want 1", got)
	}
	// No XP from a zero-value monster.
	none := &types.MonsterInstance{MaxHP: 10, Data: types.MonsterData{XP: 0}}
	if got := combat.XPForDamage(none, 5, 1.0); got != 0 {
		t.Errorf("zero-value monster XP = %d, want 0", got)
	}
}

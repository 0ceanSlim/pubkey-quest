package combat

import (
	"pubkey-quest/types"
)

// XPForDamage computes the BASE XP for dealing damage to a monster — proportional
// to damage dealt relative to the monster's max HP (monster value ÷ max HP ×
// damage), with the night bonus. The player's per-level XP multiplier is applied
// separately via character.BonusXP, so this stays the raw "xp per hit" value.
// nightMultiplier: 1.25 at night, 1.0 during the day.
func XPForDamage(monster *types.MonsterInstance, damageDealt int, nightMultiplier float64) int {
	if monster.MaxHP <= 0 || monster.Data.XP <= 0 {
		return 0
	}

	xpPerHP := float64(monster.Data.XP) / float64(monster.MaxHP)
	raw := float64(damageDealt) * xpPerHP * nightMultiplier

	// Minimum 1 XP per hit if monster has XP value
	if raw < 1 && damageDealt > 0 {
		raw = 1
	}

	return int(raw)
}

// KillBonusXP returns the flat BASE kill-bonus XP defined on a monster. Most
// monsters have none (field 0); tougher monsters — and POI/dungeon steps via the
// node walker (M3) — set it as a reward for the kill itself, on top of the
// proportional damage XP. The per-level multiplier is applied by the caller via
// character.BonusXP.
func KillBonusXP(monster *types.MonsterData) int {
	if monster == nil || monster.KillBonusXP <= 0 {
		return 0
	}
	return monster.KillBonusXP
}

// IsNight returns true if the current in-game time falls in the night window (23:00–05:00).
// timeOfDay is minutes since midnight (0–1439).
func IsNight(timeOfDay int) bool {
	return timeOfDay >= 1380 || timeOfDay < 300 // 23:00–04:59
}

// NightMultiplier returns 1.25 at night, 1.0 otherwise.
func NightMultiplier(timeOfDay int) float64 {
	if IsNight(timeOfDay) {
		return 1.25
	}
	return 1.0
}

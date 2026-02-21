package combat

import (
	"pubkey-quest/types"
)

// XPForDamage computes the XP awarded to the player for dealing damage to a monster.
// XP is proportional to damage dealt relative to the monster's max HP.
// nightMultiplier: 1.25 at night, 1.0 during the day.
// xpMultiplier: from the player's current advancement level entry.
func XPForDamage(monster *types.MonsterInstance, damageDealt int, nightMultiplier, xpMultiplier float64) int {
	if monster.MaxHP <= 0 || monster.Data.XP <= 0 {
		return 0
	}

	xpPerHP := float64(monster.Data.XP) / float64(monster.MaxHP)
	raw := float64(damageDealt) * xpPerHP * nightMultiplier * xpMultiplier

	// Minimum 1 XP per hit if monster has XP value
	if raw < 1 && damageDealt > 0 {
		raw = 1
	}

	return int(raw)
}

// KillBonusXP returns any kill bonus defined on the monster, scaled by xpMultiplier.
func KillBonusXP(monster *types.MonsterData, xpMultiplier float64) int {
	// Kill bonus is defined on standard MonsterData — parsed from the JSON.
	// For Phase 1, not all monsters have kill_bonus_xp; it's optional.
	// If the field is zero, no bonus is awarded.
	return 0 // Will be wired up once kill_bonus_xp is added to MonsterData struct
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

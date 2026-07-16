package status

import (
	"math"

	"pubkey-quest/types"
)

// FullRestMinutes is the amount of resting/sleeping (8 in-game hours) that
// restores HP and mana in full. Restore is proportional to time slept AND to Max,
// so a short wait nudges vitals, a full night restores everything, and the rate
// scales naturally with level — higher MaxHP means more HP per hour at the same
// fraction, so a night still restores most/all without healing "too fast".
const FullRestMinutes = 480

// RestoreVitalsForRest adds HP and mana in proportion to the minutes spent
// resting (capped at one full rest), never overhealing past Max. Returns the HP
// and mana actually restored. This is the shared engine behind waiting, resting,
// and sleeping — vitals come back over time, not instantly.
func RestoreVitalsForRest(save *types.SaveFile, minutes int) (int, int) {
	if save == nil || minutes <= 0 {
		return 0, 0
	}
	frac := float64(minutes) / float64(FullRestMinutes)
	if frac > 1 {
		frac = 1
	}
	hpBefore, manaBefore := save.HP, save.Mana

	save.HP += int(math.Round(float64(save.MaxHP) * frac))
	if save.HP > save.MaxHP {
		save.HP = save.MaxHP
	}
	save.Mana += int(math.Round(float64(save.MaxMana) * frac))
	if save.Mana > save.MaxMana {
		save.Mana = save.MaxMana
	}
	return save.HP - hpBefore, save.Mana - manaBefore
}

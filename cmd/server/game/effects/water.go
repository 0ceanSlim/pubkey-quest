package effects

import "pubkey-quest/types"

// WateredEffect is shown while a drink keeps the desert's heat off; its timer
// is how long that lasts (docs/draft/survival-gear.md).
const WateredEffect = "watered"

// Drink notes a drink of water, ago minutes before now: a waterskin, a potion,
// Create Water (an effect's instant "water" modifier). The heat's count of dry
// hours starts again, and Watered shows for what's left of it.
func Drink(state *types.SaveFile, ago int) {
	l := state.Lim()
	l.Drank, l.Dry = state.CurrentDay*1440+state.TimeOfDay-ago, 0
	RemoveEffect(state, WateredEffect)
	if ApplyEffect(state, WateredEffect) != nil || ago <= 0 {
		return
	}
	kept := state.ActiveEffects[:0]
	for _, e := range state.ActiveEffects {
		if e.EffectID == WateredEffect {
			if e.DurationRemaining -= float64(ago); e.DurationRemaining <= 0 {
				continue
			}
		}
		kept = append(kept, e)
	}
	state.ActiveEffects = kept
}

// WateredMinutes is how long a drink keeps the heat off: the Watered effect's
// timer.
func WateredMinutes() int {
	if d, err := LoadEffectData(WateredEffect); err == nil {
		return d.Removal.Timer
	}
	return 0
}

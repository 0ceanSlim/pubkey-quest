package status

import (
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// Exert adds hard work (landing fish, working a rock face) to a running tally
// and turns every perFatigue points of it into one point of fatigue, so the
// gathering loops cost you something beyond the clock. The tally is session
// memory: a leftover remainder is not worth saving. Returns the fatigue added.
func Exert(state *types.SaveFile, tally *int, points, perFatigue int) int {
	if state == nil || tally == nil || points <= 0 || perFatigue <= 0 {
		return 0
	}
	if effects.ModifierTotal(state, "fatigue_slow") > 0 {
		perFatigue *= 2 // jasper's Unbroken: fatigue builds half as fast
	}
	*tally += points
	n := *tally / perFatigue
	*tally %= perFatigue
	if n == 0 || state.Fatigue >= 10 {
		return 0
	}
	n = min(n, 10-state.Fatigue)
	state.Fatigue += n
	HandleFatigueChange(state)
	return n
}

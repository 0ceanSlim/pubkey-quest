package status_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/types"
)

// Regression test for the starvation bug: the "starving" system effect declares a
// periodic -1 HP every 240 min, but UpdateHungerPenaltyEffects used to tear it down
// and rebuild it every ~1-minute tick, resetting its accumulator so the drain never
// fired. Simulate real play (many 1-minute ticks) and confirm HP now actually drops.
// (setup + baseStats live in fatigue_freeze_test.go — same package.)
func TestStarvationDrainsHP(t *testing.T) {
	setup(t)

	state := &types.SaveFile{
		HP: 20, MaxHP: 20, Hunger: 0, Fatigue: 0, Stats: baseStats(),
	}

	// Simulate 300 one-minute world ticks (the frontend drives ~1 in-game min/tick).
	// The starving effect is applied on the first tick, then must accumulate 240 min.
	for i := 0; i < 300; i++ {
		gametime.AdvanceTime(state, 1, true)
	}

	if state.HP == 20 {
		t.Fatalf("starvation never drained HP over 300 minutes — the accumulator-reset bug is back")
	}
	if state.HP != 19 {
		t.Errorf("expected exactly one starvation tick (HP 20 → 19) after 300 min, got HP %d", state.HP)
	}
}

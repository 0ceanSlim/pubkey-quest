package status_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// The `removal.type:"action"` lifecycle: an effect authored to "last until you
// rest" (battle-focus) clears on the rest action and only that action.
// (setup() lives in fatigue_freeze_test.go — same package.)
func TestEffectActionRemoval(t *testing.T) {
	setup(t)

	state := &types.SaveFile{ActiveEffects: []types.ActiveEffect{{EffectID: "battle-focus"}}}

	// A non-matching action leaves it in place.
	if cleared := effects.RemoveEffectsByAction(state, "eat"); len(cleared) != 0 {
		t.Errorf("an unrelated action shouldn't clear a rest-removal effect, got %v", cleared)
	}
	if len(state.ActiveEffects) != 1 {
		t.Fatalf("effect wrongly removed by a non-matching action")
	}

	// The matching action clears it.
	cleared := effects.RemoveEffectsByAction(state, "rest")
	if len(cleared) != 1 || cleared[0] != "Battle Focus" {
		t.Errorf("rest should clear Battle Focus, got %v", cleared)
	}
	if len(state.ActiveEffects) != 0 {
		t.Errorf("effect should be gone after rest, still %d active", len(state.ActiveEffects))
	}
}

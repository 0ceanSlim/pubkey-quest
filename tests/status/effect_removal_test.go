package status_test

import (
	"fmt"
	"testing"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// The `removal.type:"action"` lifecycle: an effect authored to "last until you
// rest" clears on the rest action and only that action. No shipped effect uses
// it right now, so the test brings its own definition.
func TestEffectActionRemoval(t *testing.T) {
	restEffect := &types.EffectData{ID: "test-rest-focus", Name: "Rest Focus"}
	restEffect.Removal.Type = "action"
	restEffect.Removal.Action = "rest"
	load := func(id string) (*types.EffectData, error) {
		if id == restEffect.ID {
			return restEffect, nil
		}
		return nil, fmt.Errorf("unknown effect %s", id)
	}
	state := &types.SaveFile{ActiveEffects: []types.ActiveEffect{{EffectID: restEffect.ID}}}

	// A non-matching action leaves it in place.
	if cleared := effects.RemoveEffectsByActionWith(state, "eat", load); len(cleared) != 0 {
		t.Errorf("an unrelated action shouldn't clear a rest-removal effect, got %v", cleared)
	}
	if len(state.ActiveEffects) != 1 {
		t.Fatalf("effect wrongly removed by a non-matching action")
	}

	// The matching action clears it.
	cleared := effects.RemoveEffectsByActionWith(state, "rest", load)
	if len(cleared) != 1 || cleared[0] != "Rest Focus" {
		t.Errorf("rest should clear Rest Focus, got %v", cleared)
	}
	if len(state.ActiveEffects) != 0 {
		t.Errorf("effect should be gone after rest, still %d active", len(state.ActiveEffects))
	}
}

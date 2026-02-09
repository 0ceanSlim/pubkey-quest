package status

import (
	"log"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// UpdateHungerPenaltyEffects applies appropriate penalty effects based on hunger level
// Now uses data-driven system - effect activation defined in JSON system_check
func UpdateHungerPenaltyEffects(state *types.SaveFile) (*types.EffectMessage, error) {
	// Clamp hunger to valid range
	if state.Hunger < 0 {
		state.Hunger = 0
	} else if state.Hunger > 3 {
		state.Hunger = 3
	}

	// Use generic data-driven system
	return UpdateSystemStatusEffects(state, "hunger")
}

// RemoveHungerPenaltyEffects - DEPRECATED: Now handled by UpdateSystemStatusEffects
// Kept for backward compatibility but no longer used
func RemoveHungerPenaltyEffects(state *types.SaveFile) {
	// This function is no longer needed - UpdateSystemStatusEffects handles removal
	log.Printf("⚠️ RemoveHungerPenaltyEffects called but is deprecated - use UpdateSystemStatusEffects")
}

// EnsureHungerAccumulation ensures hunger accumulation effect is present (no swapping needed)
func EnsureHungerAccumulation(state *types.SaveFile) error {
	// Check if hunger accumulation effect already exists
	for _, activeEffect := range state.ActiveEffects {
		if activeEffect.EffectID == "hunger-accumulation-stuffed" ||
			activeEffect.EffectID == "hunger-accumulation-wellfed" ||
			activeEffect.EffectID == "hunger-accumulation-hungry" {
			// Already present - don't remove/re-add (preserves tick_accumulator)
			return nil
		}
	}

	// Apply initial hunger accumulation effect based on current hunger level
	var effectID string
	switch state.Hunger {
	case 3:
		effectID = "hunger-accumulation-stuffed"
	case 2:
		effectID = "hunger-accumulation-wellfed"
	case 1:
		effectID = "hunger-accumulation-hungry"
	case 0:
		// Don't apply hunger decrease accumulation when famished (hunger stays at 0)
		return nil
	default:
		return nil
	}

	return effects.ApplyEffect(state, effectID)
}

// RemoveHungerAccumulation removes all hunger accumulation effects
func RemoveHungerAccumulation(state *types.SaveFile) {
	var remainingEffects []types.ActiveEffect
	for _, activeEffect := range state.ActiveEffects {
		// Keep non-hunger-accumulation effects
		if activeEffect.EffectID != "hunger-accumulation-stuffed" &&
			activeEffect.EffectID != "hunger-accumulation-wellfed" &&
			activeEffect.EffectID != "hunger-accumulation-hungry" {
			remainingEffects = append(remainingEffects, activeEffect)
		}
	}
	state.ActiveEffects = remainingEffects
}

// ResetHungerAccumulator resets the tick accumulator for hunger accumulation effects
func ResetHungerAccumulator(state *types.SaveFile) {
	for i, activeEffect := range state.ActiveEffects {
		if activeEffect.EffectID == "hunger-accumulation-stuffed" ||
			activeEffect.EffectID == "hunger-accumulation-wellfed" ||
			activeEffect.EffectID == "hunger-accumulation-hungry" {
			state.ActiveEffects[i].TickAccumulator = 0
			return
		}
	}
}

// HandleHungerChange processes a hunger change and updates related effects
func HandleHungerChange(state *types.SaveFile) {
	// Update penalty effects
	if _, err := UpdateHungerPenaltyEffects(state); err != nil {
		log.Printf("⚠️ Failed to update hunger penalty effects: %v", err)
	}

	// Ensure accumulation is active
	if err := EnsureHungerAccumulation(state); err != nil {
		log.Printf("⚠️ Failed to ensure hunger accumulation: %v", err)
	}
}

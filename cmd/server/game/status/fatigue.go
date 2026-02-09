package status

import (
	"log"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// UpdateFatiguePenaltyEffects applies appropriate penalty effects based on fatigue level
// Now uses data-driven system - effect activation defined in JSON system_check
func UpdateFatiguePenaltyEffects(state *types.SaveFile) (*types.EffectMessage, error) {
	// Clamp fatigue to valid range
	if state.Fatigue < 0 {
		state.Fatigue = 0
	} else if state.Fatigue > 10 {
		state.Fatigue = 10
	}

	// Use generic data-driven system
	return UpdateSystemStatusEffects(state, "fatigue")
}

// RemoveFatiguePenaltyEffects - DEPRECATED: Now handled by UpdateSystemStatusEffects
// Kept for backward compatibility but no longer used
func RemoveFatiguePenaltyEffects(state *types.SaveFile) {
	// This function is no longer needed - UpdateSystemStatusEffects handles removal
	log.Printf("⚠️ RemoveFatiguePenaltyEffects called but is deprecated - use UpdateSystemStatusEffects")
}

// EnsureFatigueAccumulation ensures the fatigue accumulation effect is active
// Only adds if fatigue < 10 (stops accumulation at max)
func EnsureFatigueAccumulation(state *types.SaveFile) error {
	// Don't accumulate if already at max fatigue
	if state.Fatigue >= 10 {
		RemoveFatigueAccumulation(state)
		return nil
	}

	// Check if already present
	for _, activeEffect := range state.ActiveEffects {
		if activeEffect.EffectID == "fatigue-accumulation" {
			return nil // Already present
		}
	}

	// Apply it
	return effects.ApplyEffect(state, "fatigue-accumulation")
}

// RemoveFatigueAccumulation removes the fatigue accumulation effect
func RemoveFatigueAccumulation(state *types.SaveFile) {
	var remainingEffects []types.ActiveEffect
	for _, activeEffect := range state.ActiveEffects {
		if activeEffect.EffectID != "fatigue-accumulation" {
			remainingEffects = append(remainingEffects, activeEffect)
		}
	}
	state.ActiveEffects = remainingEffects
}

// ResetFatigueAccumulator resets the tick accumulator for fatigue accumulation effect
func ResetFatigueAccumulator(state *types.SaveFile) {
	for i, activeEffect := range state.ActiveEffects {
		if activeEffect.EffectID == "fatigue-accumulation" {
			state.ActiveEffects[i].TickAccumulator = 0
			return
		}
	}
}

// HandleFatigueChange processes a fatigue change and updates related effects
func HandleFatigueChange(state *types.SaveFile) {
	// Stop accumulation if we've reached max fatigue
	if state.Fatigue >= 10 {
		RemoveFatigueAccumulation(state)
	} else {
		// Ensure accumulation is active if below max
		if err := EnsureFatigueAccumulation(state); err != nil {
			log.Printf("⚠️ Failed to ensure fatigue accumulation: %v", err)
		}
	}

	// Update penalty effects
	if _, err := UpdateFatiguePenaltyEffects(state); err != nil {
		log.Printf("⚠️ Failed to update fatigue penalty effects: %v", err)
	}
}

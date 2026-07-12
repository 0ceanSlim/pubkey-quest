package status

import (
	"log"
	"sort"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// UpdateSystemStatusEffects evaluates all system status effects and applies/removes as needed
// This is the generic, data-driven replacement for hardcoded switch statements
func UpdateSystemStatusEffects(state *types.SaveFile, category string) (*types.EffectMessage, error) {
	// Get all system status effects for this category
	categoryEffects, err := effects.GetSystemStatusEffectsByCategory(category)
	if err != nil {
		return nil, err
	}

	// Debug: Only log if no effects found (indicates database issue)
	if len(categoryEffects) == 0 {
		log.Printf("⚠️ No %s effects found in database - check source_type column", category)
		return nil, nil
	}

	// Sort by priority (for overlapping ranges)
	// Exact matches (==) checked before ranges (>= or >)
	// Within each type, higher values checked first
	sort.Slice(categoryEffects, func(i, j int) bool {
		if categoryEffects[i].SystemCheck == nil || categoryEffects[j].SystemCheck == nil {
			return false
		}

		sci := categoryEffects[i].SystemCheck
		scj := categoryEffects[j].SystemCheck

		// Only compare effects checking the same stat
		if sci.Stat != scj.Stat {
			return false
		}

		// Prioritize exact matches (==) over ranges (>= or >)
		iIsExact := sci.Operator == "=="
		jIsExact := scj.Operator == "=="

		if iIsExact && !jIsExact {
			return true // i (exact) comes before j (range)
		}
		if !iIsExact && jIsExact {
			return false // j (exact) comes before i (range)
		}

		// Both are same type (both exact or both range)
		// Sort by value descending (higher values first)
		return sci.Value > scj.Value
	})

	var appliedMsg *types.EffectMessage
	var effectToApply *types.EffectData

	// Find the first effect that should be active (due to reverse priority sorting)
	for _, effectData := range categoryEffects {
		if effectData.SystemCheck == nil {
			continue
		}

		shouldBeActive := effects.EvaluateSystemCheck(effectData, state)
		if shouldBeActive {
			effectToApply = effectData
			break // Only apply one effect per category
		}
	}

	// Remove every effect in this category EXCEPT the one that should be active.
	// Leaving the active one untouched is critical: re-applying it (below) rebuilds
	// the ActiveEffect with TickAccumulator = 0, and this runs on every ~1-minute
	// world tick — so a periodic modifier like starving's "1 HP per 240 min" would
	// have its accumulator reset before it could ever reach the interval, and never
	// fire. Preserving the active effect lets its accumulator carry across ticks.
	for _, effectData := range categoryEffects {
		if effectToApply != nil && effectData.ID == effectToApply.ID {
			continue // this one should stay active — don't disturb its accumulator
		}
		if effects.HasActiveEffect(state, effectData.ID) {
			effects.RemoveEffect(state, effectData.ID)
		}
	}

	// Apply the target effect only on a genuine transition (it isn't already active).
	// A fresh application resets the accumulator, which is correct only when the
	// player actually crosses into this state, not every tick they remain in it.
	if effectToApply != nil && !effects.HasActiveEffect(state, effectToApply.ID) {
		msg, err := effects.ApplyEffectWithMessage(state, effectToApply.ID)
		if err != nil {
			log.Printf("⚠️ Failed to apply %s effect '%s': %v", category, effectToApply.Name, err)
		} else {
			appliedMsg = msg
		}
	}

	return appliedMsg, nil
}

// UpdateHungerEffects updates hunger-related system status effects
func UpdateHungerEffects(state *types.SaveFile) (*types.EffectMessage, error) {
	return UpdateSystemStatusEffects(state, "hunger")
}

// UpdateFatigueEffects updates fatigue-related system status effects
func UpdateFatigueEffects(state *types.SaveFile) (*types.EffectMessage, error) {
	return UpdateSystemStatusEffects(state, "fatigue")
}

// UpdateEncumbranceEffects updates encumbrance-related system status effects
func UpdateEncumbranceEffects(state *types.SaveFile) (*types.EffectMessage, error) {
	return UpdateSystemStatusEffects(state, "encumbrance")
}

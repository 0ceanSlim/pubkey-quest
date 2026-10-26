package status

import (
	"log"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// HungerNames are the four hunger levels by value (0 Starving … 3 Stuffed).
var HungerNames = []string{"Starving", "Hungry", "Well Fed", "Stuffed"}

// HungerName names a hunger level.
func HungerName(level int) string {
	if level < 0 || level >= len(HungerNames) {
		return "Unknown"
	}
	return HungerNames[level]
}

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

// ─── Hours of fullness (docs/draft/food-system.md) ────────────────────────────
//
// Food is measured in minutes of fullness. Hunger is a level (0 Starving …
// 3 Stuffed) and a tick accumulator counting toward the next drop, so the time
// until you're starving is what's left of this level's interval plus every
// interval below it. Eating adds to that total and converts it back: past the
// level's bar it spills into the next level up, and it stops at Stuffed with a
// full bar.

// hungerEffects are the accumulation effect per level (1–3); their tick
// intervals are the level's length, configurable in the codex.
var hungerEffects = map[int]string{1: "hunger-accumulation-hungry", 2: "hunger-accumulation-wellfed", 3: "hunger-accumulation-stuffed"}

// levelMinutes is how long a hunger level lasts before it drops.
func levelMinutes(level int) int {
	fallback := 240
	if level == 3 {
		fallback = 360
	}
	if id, ok := hungerEffects[level]; ok {
		if _, _, interval, _, err := effects.GetEffectTemplate(id, 0); err == nil && interval > 0 {
			return interval
		}
	}
	return fallback
}

// hungerAccumulator is the live accumulation effect (nil when starving).
func hungerAccumulator(state *types.SaveFile) *types.ActiveEffect {
	for i := range state.ActiveEffects {
		switch state.ActiveEffects[i].EffectID {
		case "hunger-accumulation-stuffed", "hunger-accumulation-wellfed", "hunger-accumulation-hungry":
			return &state.ActiveEffects[i]
		}
	}
	return nil
}

// Fullness is the minutes of food in you: the time until you're Starving.
func Fullness(state *types.SaveFile) int {
	if state.Hunger <= 0 {
		return 0
	}
	elapsed := 0
	if a := hungerAccumulator(state); a != nil {
		elapsed = int(a.TickAccumulator)
	}
	total := max(levelMinutes(state.Hunger)-elapsed, 0)
	for l := 1; l < state.Hunger; l++ {
		total += levelMinutes(l)
	}
	return total
}

// MaxFullness is the most food you can hold: Stuffed with a full bar.
func MaxFullness() int {
	return levelMinutes(1) + levelMinutes(2) + levelMinutes(3)
}

// IsFull reports whether you couldn't manage another bite.
func IsFull(state *types.SaveFile) bool {
	return Fullness(state) >= MaxFullness()
}

// setFullness puts the hunger level and accumulator at a total of minutes.
func setFullness(state *types.SaveFile, total int) {
	total = min(max(total, 0), MaxFullness())
	if total == 0 {
		state.Hunger = 0
		HandleHungerChange(state)
		return
	}
	level, base := 1, 0
	for level < 3 && total > base+levelMinutes(level) {
		base += levelMinutes(level)
		level++
	}
	state.Hunger = level
	HandleHungerChange(state) // penalties, and the accumulation effect when coming back from starving
	if a := hungerAccumulator(state); a != nil {
		a.TickAccumulator = float64(levelMinutes(level) - (total - base))
	}
}

// Feed adds minutes of fullness. It returns how many took and how many were
// wasted past full.
func Feed(state *types.SaveFile, minutes int) (gained, wasted int) {
	if minutes <= 0 {
		return 0, 0
	}
	before := Fullness(state)
	after := min(before+minutes, MaxFullness())
	setFullness(state, after)
	return after - before, before + minutes - after
}

// WakeWellFed is an inn's breakfast: Well Fed with a full bar, unless you
// went to bed fuller than that.
func WakeWellFed(state *types.SaveFile) {
	wellFed := levelMinutes(1) + levelMinutes(2)
	if Fullness(state) < wellFed {
		setFullness(state, wellFed)
	}
}

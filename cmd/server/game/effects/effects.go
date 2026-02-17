package effects

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/types"
)

// skillRatio holds a skill's stat weights (loaded from DB to avoid import cycle with data package)
type skillRatio struct {
	Ratio map[string]float64 `json:"ratio"`
}

// Cached skill ratios (loaded once on first use)
var cachedSkillRatios map[string]skillRatio

// getSkillRatio returns the stat ratio for a skill, loading from DB on first call
func getSkillRatio(skillID string) (map[string]float64, bool) {
	if cachedSkillRatios == nil {
		database := db.GetDB()
		if database == nil {
			return nil, false
		}

		var propsJSON string
		err := database.QueryRow("SELECT properties FROM systems WHERE id = 'skills'").Scan(&propsJSON)
		if err != nil {
			log.Printf("⚠️ Failed to load skill definitions for scaling: %v", err)
			return nil, false
		}

		var skills map[string]skillRatio
		if err := json.Unmarshal([]byte(propsJSON), &skills); err != nil {
			log.Printf("⚠️ Failed to parse skill definitions: %v", err)
			return nil, false
		}
		cachedSkillRatios = skills
	}

	skill, ok := cachedSkillRatios[skillID]
	if !ok {
		return nil, false
	}
	return skill.Ratio, true
}

// calculateSkillValue computes a skill value from character stats and ratios
func calculateSkillValue(stats map[string]interface{}, ratio map[string]float64) int {
	// Normalize stat keys to lowercase (save files use "Strength", ratios use "strength")
	normalizedStats := make(map[string]interface{}, len(stats))
	for k, v := range stats {
		normalizedStats[strings.ToLower(k)] = v
	}

	var value float64
	for stat, weight := range ratio {
		statVal := 10.0 // default
		if v, ok := normalizedStats[strings.ToLower(stat)]; ok {
			switch n := v.(type) {
			case float64:
				statVal = n
			case int:
				statVal = float64(n)
			case json.Number:
				if f, err := n.Float64(); err == nil {
					statVal = f
				}
			}
		}
		value += statVal * weight
	}
	return int(math.Round(value))
}

// GetEffectSkillScaling loads an effect's skill scaling config (or nil if none)
func GetEffectSkillScaling(effectID string) *types.SkillScaling {
	effectData, err := LoadEffectData(effectID)
	if err != nil {
		return nil
	}
	return effectData.SkillScaling
}

// applySkillScaling adjusts a tick interval based on skill scaling and character stats
func applySkillScaling(tickInterval int, scaling *types.SkillScaling, stats map[string]interface{}) int {
	if scaling == nil {
		return tickInterval
	}

	ratio, ok := getSkillRatio(scaling.Skill)
	if !ok {
		return tickInterval
	}

	skillValue := calculateSkillValue(stats, ratio)

	// No bonus at or below base level
	if skillValue <= scaling.BaseLevel {
		return tickInterval
	}

	// Calculate bonus levels (capped)
	bonusLevels := skillValue - scaling.BaseLevel
	if bonusLevels > scaling.MaxBonusLevels {
		bonusLevels = scaling.MaxBonusLevels
	}

	// Apply multiplier: higher skill = longer interval = slower accumulation
	multiplier := 1.0 + float64(bonusLevels)*scaling.BonusPerLevel
	return int(math.Floor(float64(tickInterval) * multiplier))
}

// ApplyEffect applies an effect to the character (from game-data/effects/{effectID}.json)
func ApplyEffect(state *types.SaveFile, effectID string) error {
	_, err := ApplyEffectWithMessage(state, effectID)
	return err
}

// ApplyEffectWithMessage applies an effect and returns a message to display
func ApplyEffectWithMessage(state *types.SaveFile, effectID string) (*types.EffectMessage, error) {
	// Load effect data from file
	effectData, err := LoadEffectData(effectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load effect %s: %v", effectID, err)
	}

	// Initialize active_effects if nil
	if state.ActiveEffects == nil {
		state.ActiveEffects = []types.ActiveEffect{}
	}

	// Calculate total duration from removal config
	duration := 0.0
	if effectData.Removal.Type == "timed" || effectData.Removal.Type == "hybrid" {
		duration = float64(effectData.Removal.Timer)
	}

	// Apply each modifier
	for idx, modifier := range effectData.Modifiers {
		// Determine how to apply based on type
		switch modifier.Type {
		case "instant":
			// Instant: Apply once to resources (hp, mana, hunger, fatigue) then done
			// Only add to active effects if there's a delay
			if modifier.Delay > 0 {
				activeEffect := types.ActiveEffect{
					EffectID:          effectID,
					EffectIndex:       idx,
					DurationRemaining: duration,
					TotalDuration:     duration,
					DelayRemaining:    float64(modifier.Delay),
					TickAccumulator:   0.0,
					AppliedAt:         state.TimeOfDay,
				}
				state.ActiveEffects = append(state.ActiveEffects, activeEffect)
			} else {
				// Apply immediately
				ApplyImmediateEffect(state, modifier.Stat, modifier.Value)
			}

		case "constant":
			// Constant: Stat modifiers that apply while effect is active
			// Always add to active effects (duration tracks when effect ends)
			activeEffect := types.ActiveEffect{
				EffectID:          effectID,
				EffectIndex:       idx,
				DurationRemaining: duration,
				TotalDuration:     duration,
				DelayRemaining:    float64(modifier.Delay),
				TickAccumulator:   0.0,
				AppliedAt:         state.TimeOfDay,
			}
			state.ActiveEffects = append(state.ActiveEffects, activeEffect)

		case "periodic":
			// Periodic: Apply repeatedly every tick_interval
			activeEffect := types.ActiveEffect{
				EffectID:          effectID,
				EffectIndex:       idx,
				DurationRemaining: duration,
				TotalDuration:     duration,
				DelayRemaining:    float64(modifier.Delay),
				TickAccumulator:   0.0,
				AppliedAt:         state.TimeOfDay,
			}
			state.ActiveEffects = append(state.ActiveEffects, activeEffect)

		default:
			// Unknown type, treat as constant for backward compatibility
			log.Printf("⚠️ Unknown modifier type '%s', treating as constant", modifier.Type)
			activeEffect := types.ActiveEffect{
				EffectID:          effectID,
				EffectIndex:       idx,
				DurationRemaining: duration,
				TotalDuration:     duration,
				DelayRemaining:    float64(modifier.Delay),
				TickAccumulator:   0.0,
				AppliedAt:         state.TimeOfDay,
			}
			state.ActiveEffects = append(state.ActiveEffects, activeEffect)
		}
	}

	// Return effect message (convert visible to silent for backward compatibility)
	effectMsg := &types.EffectMessage{
		Message:  effectData.Message,
		Color:    "", // Frontend will determine color based on category
		Category: effectData.Category,
		Silent:   !effectData.Visible,
	}

	return effectMsg, nil
}

// ApplyImmediateEffect applies an instant effect (no duration)
// Note: This function modifies fatigue/hunger but does NOT call status update functions
// to avoid circular dependencies. The caller is responsible for updating penalty effects.
func ApplyImmediateEffect(state *types.SaveFile, effectType string, value int) {
	switch effectType {
	case "hp":
		state.HP += value
		if state.HP > state.MaxHP {
			state.HP = state.MaxHP
		}
		if state.HP < 0 {
			state.HP = 0
		}
	case "mana":
		state.Mana += value
		if state.Mana > state.MaxMana {
			state.Mana = state.MaxMana
		}
		if state.Mana < 0 {
			state.Mana = 0
		}
	case "fatigue":
		// Adjust fatigue level (penalty effects handled by caller)
		state.Fatigue += value
		if state.Fatigue < 0 {
			state.Fatigue = 0
		}
		if state.Fatigue > 10 {
			state.Fatigue = 10
		}
	case "hunger":
		// Adjust hunger level (penalty effects handled by caller)
		state.Hunger += value
		if state.Hunger < 0 {
			state.Hunger = 0
		}
		if state.Hunger > 3 {
			state.Hunger = 3
		}
	}
}

// GetEffectTemplate loads effect template data and returns the specific modifier at index
func GetEffectTemplate(effectID string, effectIndex int) (stat string, value int, tickInterval int, name string, err error) {
	effectData, err := LoadEffectData(effectID)
	if err != nil {
		return "", 0, 0, "", fmt.Errorf("failed to load effect %s: %v", effectID, err)
	}

	if effectIndex >= len(effectData.Modifiers) {
		return "", 0, 0, effectData.Name, fmt.Errorf("invalid modifier index %d for effect %s", effectIndex, effectID)
	}

	modifier := effectData.Modifiers[effectIndex]
	return modifier.Stat, modifier.Value, modifier.TickInterval, effectData.Name, nil
}

// TickEffects processes all active effects, applying stat modifiers and ticking down durations
// Returns a slice of messages from effects that triggered (like starvation damage)
func TickEffects(state *types.SaveFile, minutesElapsed int) []types.EffectMessage {
	if len(state.ActiveEffects) == 0 {
		return nil
	}

	var remainingEffects []types.ActiveEffect
	var messages []types.EffectMessage

	for _, activeEffect := range state.ActiveEffects {
		// Load effect template to get stat, value, tick_interval
		stat, value, tickInterval, name, err := GetEffectTemplate(activeEffect.EffectID, activeEffect.EffectIndex)
		if err != nil {
			log.Printf("⚠️ Failed to load effect template for %s: %v", activeEffect.EffectID, err)
			continue
		}

		// Tick down delay first
		if activeEffect.DelayRemaining > 0 {
			activeEffect.DelayRemaining -= float64(minutesElapsed)
			if activeEffect.DelayRemaining > 0 {
				remainingEffects = append(remainingEffects, activeEffect)
				continue
			}
		}

		// Process tick-based effects (damage/healing over time)
		if tickInterval > 0 {
			// Hunger accumulation effects keep the same effect ID when hunger level changes
			// (to preserve the tick accumulator). Look up the tick interval from the database
			// for the effect that matches the current hunger level, so rates are configurable
			// via the codex without code changes.
			if activeEffect.EffectID == "hunger-accumulation-stuffed" ||
				activeEffect.EffectID == "hunger-accumulation-wellfed" ||
				activeEffect.EffectID == "hunger-accumulation-hungry" {
				var lookupID string
				switch state.Hunger {
				case 3:
					lookupID = "hunger-accumulation-stuffed"
				case 2:
					lookupID = "hunger-accumulation-wellfed"
				case 1:
					lookupID = "hunger-accumulation-hungry"
				case 0:
					tickInterval = 0 // starving — no decrease (handled by starving penalty effect)
				}
				if lookupID != "" {
					if _, _, interval, _, err := GetEffectTemplate(lookupID, 0); err == nil {
						tickInterval = interval
					} else {
						log.Printf("⚠️ Failed to load hunger tick interval for %s: %v", lookupID, err)
					}
				}
			}

			// Apply skill scaling to tick interval (e.g., Athletics slows fatigue)
			if scaling := GetEffectSkillScaling(activeEffect.EffectID); scaling != nil {
				tickInterval = applySkillScaling(tickInterval, scaling, state.Stats)
			}

			if tickInterval > 0 {
				activeEffect.TickAccumulator += float64(minutesElapsed)
				for activeEffect.TickAccumulator >= float64(tickInterval) {
					ApplyImmediateEffect(state, stat, value)
					activeEffect.TickAccumulator -= float64(tickInterval)

					// For starvation damage, show message
					if activeEffect.EffectID == "starving" && stat == "hp" {
						messages = append(messages, types.EffectMessage{
							Message:  "You're starving! You lose 1 HP from lack of food.",
							Color:    "red",
							Category: "debuff",
							Silent:   false,
						})
						log.Printf("💀 Starvation damage: Player lost 1 HP (current HP: %d)", state.HP)
					}
				}
			}
		}

		// Tick down duration (but don't tick permanent effects with duration == 0)
		if activeEffect.DurationRemaining > 0 {
			activeEffect.DurationRemaining -= float64(minutesElapsed)
		}

		// Keep effect if duration remains or is permanent (0)
		// BUT skip accumulation effects if we've hit the cap
		shouldKeep := activeEffect.DurationRemaining > 0 || activeEffect.DurationRemaining == 0

		// Don't keep fatigue-accumulation if fatigue is maxed
		if activeEffect.EffectID == "fatigue-accumulation" && state.Fatigue >= 10 {
			shouldKeep = false
			log.Printf("🛑 Removing fatigue-accumulation: fatigue at max (10)")
		}

		// Don't keep hunger-accumulation effects if starving
		if (activeEffect.EffectID == "hunger-accumulation-stuffed" ||
			activeEffect.EffectID == "hunger-accumulation-wellfed" ||
			activeEffect.EffectID == "hunger-accumulation-hungry") && state.Hunger <= 0 {
			shouldKeep = false
			log.Printf("🛑 Removing hunger-accumulation: hunger at min (0)")
		}

		if shouldKeep {
			remainingEffects = append(remainingEffects, activeEffect)
		} else if activeEffect.DurationRemaining < 0 {
			log.Printf("⏱️ Effect '%s' expired", name)
		}
	}

	state.ActiveEffects = remainingEffects
	return messages
}

// TickDownEffectDurations reduces duration_remaining for all timed effects
// Used during sleep and other time jumps to properly expire buffs/debuffs
func TickDownEffectDurations(state *types.SaveFile, minutes int) {
	if state.ActiveEffects == nil || minutes <= 0 {
		return
	}

	var remainingEffects []types.ActiveEffect
	for _, effect := range state.ActiveEffects {
		// Skip permanent effects (duration == 0) and system effects
		if effect.DurationRemaining == 0 {
			remainingEffects = append(remainingEffects, effect)
			continue
		}

		// Tick down the duration
		effect.DurationRemaining -= float64(minutes)

		if effect.DurationRemaining > 0 {
			// Effect still active
			remainingEffects = append(remainingEffects, effect)
		} else {
			// Effect expired
			log.Printf("⏱️ Effect '%s' expired during time skip (%d minutes)", effect.EffectID, minutes)
		}
	}

	state.ActiveEffects = remainingEffects
}

// GetActiveStatModifiers calculates total stat modifiers from all active effects
func GetActiveStatModifiers(state *types.SaveFile) map[string]int {
	modifiers := make(map[string]int)

	if state.ActiveEffects == nil {
		return modifiers
	}

	for _, activeEffect := range state.ActiveEffects {
		// Skip effects that haven't started yet (still in delay)
		if activeEffect.DelayRemaining > 0 {
			continue
		}

		// Load effect template to get stat and value
		stat, value, _, _, err := GetEffectTemplate(activeEffect.EffectID, activeEffect.EffectIndex)
		if err != nil {
			log.Printf("⚠️ Failed to load effect template for %s: %v", activeEffect.EffectID, err)
			continue
		}

		// Only apply stat modifiers (not instant effects like hp/mana)
		switch stat {
		case "strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma":
			modifiers[stat] += value
		}
	}

	return modifiers
}

// LoadEffectData loads effect data from database
func LoadEffectData(effectID string) (*types.EffectData, error) {
	// Normalize old effect IDs for backward compatibility
	effectID = NormalizeEffectID(effectID)

	database := db.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}

	// Query effect properties from database
	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM effects WHERE id = ?", effectID).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("effect not found in database: %s", effectID)
	}

	// Parse properties JSON into strongly typed struct
	var effectData types.EffectData
	if err := json.Unmarshal([]byte(propertiesJSON), &effectData); err != nil {
		return nil, fmt.Errorf("failed to parse effect properties: %v", err)
	}

	return &effectData, nil
}

// EvaluateSystemCheck checks if an effect's system check condition is met
func EvaluateSystemCheck(effectData *types.EffectData, state *types.SaveFile) bool {
	if effectData.SystemCheck == nil {
		return false
	}

	sc := effectData.SystemCheck
	condition := fmt.Sprintf("%s %s %d", sc.Stat, sc.Operator, sc.Value)

	return EvaluateCondition(condition, state)
}

// GetSystemStatusEffectsByCategory returns all system_status effects, optionally filtered by category
func GetSystemStatusEffectsByCategory(category string) ([]*types.EffectData, error) {
	database := db.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}

	// Query all system_status effects
	rows, err := database.Query(`
		SELECT id, properties
		FROM effects
		WHERE source_type = 'system_status'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var effects []*types.EffectData
	for rows.Next() {
		var id, properties string
		if err := rows.Scan(&id, &properties); err != nil {
			continue
		}

		var effectData types.EffectData
		if err := json.Unmarshal([]byte(properties), &effectData); err != nil {
			log.Printf("⚠️ Failed to parse effect %s: %v", id, err)
			continue
		}

		// Filter by category if specified
		// Category maps to the stat being checked in SystemCheck
		if category == "" || matchesCategory(&effectData, category) {
			effects = append(effects, &effectData)
		}
	}

	return effects, nil
}

// matchesCategory checks if an effect belongs to a category based on its SystemCheck stat
func matchesCategory(effect *types.EffectData, category string) bool {
	if effect.SystemCheck == nil {
		return false
	}

	// Map category to stat name
	switch category {
	case "fatigue":
		return effect.SystemCheck.Stat == "fatigue"
	case "hunger":
		return effect.SystemCheck.Stat == "hunger"
	case "encumbrance":
		return effect.SystemCheck.Stat == "weight_percent"
	default:
		// Fallback: check if effect ID contains category (for other cases)
		return strings.Contains(strings.ToLower(effect.ID), category)
	}
}

// HasActiveEffect checks if an effect is currently active
func HasActiveEffect(state *types.SaveFile, effectID string) bool {
	if state.ActiveEffects == nil {
		return false
	}

	for _, ae := range state.ActiveEffects {
		if ae.EffectID == effectID {
			return true
		}
	}
	return false
}

// RemoveEffect removes an effect from active effects
func RemoveEffect(state *types.SaveFile, effectID string) {
	if state.ActiveEffects == nil {
		return
	}

	var remaining []types.ActiveEffect
	for _, ae := range state.ActiveEffects {
		if ae.EffectID != effectID {
			remaining = append(remaining, ae)
		}
	}
	state.ActiveEffects = remaining
}

// EnrichActiveEffects adds template data (name, category, stat_modifiers) to active effects
// Used when sending active_effects to the frontend for display.
// Stats parameter is used to apply skill scaling to tick intervals for accurate UI display.
func EnrichActiveEffects(activeEffects []types.ActiveEffect, stats map[string]interface{}) []types.EnrichedEffect {
	enriched := make([]types.EnrichedEffect, 0, len(activeEffects))

	for _, ae := range activeEffects {
		ee := types.EnrichedEffect{
			ActiveEffect:  ae,
			Name:          ae.EffectID, // Default to ID
			Description:   "",
			Category:      "modifier",
			StatModifiers: make(map[string]int),
			TickInterval:  0,
		}

		// Load template data
		effectData, err := LoadEffectData(ae.EffectID)
		if err == nil {
			ee.Name = effectData.Name
			ee.Description = effectData.Description
			ee.Category = effectData.Category

			// Extract stat modifiers and tick interval from modifiers
			for _, modifier := range effectData.Modifiers {
				switch modifier.Stat {
				case "strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma":
					ee.StatModifiers[modifier.Stat] = modifier.Value
				}

				if modifier.Type == "periodic" && modifier.TickInterval > 0 {
					interval := modifier.TickInterval
					// Apply skill scaling so the frontend circle matches the actual tick rate
					if effectData.SkillScaling != nil {
						interval = applySkillScaling(interval, effectData.SkillScaling, stats)
					}
					ee.TickInterval = float64(interval)
				}
			}
		}

		enriched = append(enriched, ee)
	}

	return enriched
}

// NormalizeEffectID converts old effect IDs to new ones for backward compatibility
func NormalizeEffectID(effectID string) string {
	oldToNew := map[string]string{
		"hunger-accumulation-well-fed":  "hunger-accumulation-wellfed",
		"hunger-accumulation-full":      "hunger-accumulation-stuffed",
		"hunger-accumulation-satisfied": "hunger-accumulation-wellfed",
		"famished":                      "starving",
	}
	if newID, exists := oldToNew[effectID]; exists {
		return newID
	}
	return effectID
}

// MigrateOldEffectIDs updates all effect IDs in ActiveEffects to use new naming conventions
func MigrateOldEffectIDs(state *types.SaveFile) {
	if state.ActiveEffects == nil {
		return
	}
	for i := range state.ActiveEffects {
		oldID := state.ActiveEffects[i].EffectID
		newID := NormalizeEffectID(oldID)
		if newID != oldID {
			log.Printf("🔄 Migrating effect ID: %s -> %s", oldID, newID)
			state.ActiveEffects[i].EffectID = newID
		}
	}
}

// EvaluateCondition evaluates a simple condition string against game state
// Supports conditions like: "hunger < 2", "fatigue >= 6", "weight_percent != 50"
// Returns true if condition is met, false otherwise
func EvaluateCondition(condition string, state *types.SaveFile) bool {
	if condition == "" {
		return false
	}

	// Parse condition: "stat operator value"
	var stat, operator string
	var value int

	// Try different operators
	if n, _ := fmt.Sscanf(condition, "%s != %d", &stat, &value); n == 2 {
		operator = "!="
	} else if n, _ := fmt.Sscanf(condition, "%s == %d", &stat, &value); n == 2 {
		operator = "=="
	} else if n, _ := fmt.Sscanf(condition, "%s <= %d", &stat, &value); n == 2 {
		operator = "<="
	} else if n, _ := fmt.Sscanf(condition, "%s >= %d", &stat, &value); n == 2 {
		operator = ">="
	} else if n, _ := fmt.Sscanf(condition, "%s < %d", &stat, &value); n == 2 {
		operator = "<"
	} else if n, _ := fmt.Sscanf(condition, "%s > %d", &stat, &value); n == 2 {
		operator = ">"
	} else {
		log.Printf("⚠️ Failed to parse condition: %s", condition)
		return false
	}

	// Get actual value from game state
	var actualValue int
	switch stat {
	case "hunger":
		actualValue = state.Hunger
	case "fatigue":
		actualValue = state.Fatigue
	case "hp_percent":
		if state.MaxHP > 0 {
			actualValue = (state.HP * 100) / state.MaxHP
		}
	case "mana_percent":
		if state.MaxMana > 0 {
			actualValue = (state.Mana * 100) / state.MaxMana
		}
	case "weight_percent":
		totalWeight := gameutil.CalculateTotalWeight(state)
		capacity := gameutil.CalculateWeightCapacity(state)
		if capacity > 0 {
			// Use ceiling to round up (100.1% = 101%, not 100%)
			percentage := (totalWeight / capacity) * 100
			actualValue = int(percentage)
			if percentage > float64(actualValue) {
				actualValue++ // Round up if there's any decimal
			}
		} else {
			actualValue = 0
		}
	default:
		log.Printf("⚠️ Unknown condition stat: %s", stat)
		return false
	}

	// Evaluate operator
	switch operator {
	case "==":
		return actualValue == value
	case "!=":
		return actualValue != value
	case "<":
		return actualValue < value
	case "<=":
		return actualValue <= value
	case ">":
		return actualValue > value
	case ">=":
		return actualValue >= value
	default:
		log.Printf("⚠️ Unknown operator '%s' in condition '%s'", operator, condition)
		return false
	}
}



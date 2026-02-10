package status

import (
	"encoding/json"
	"fmt"
	"log"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// InitializeFatigueHungerEffects ensures all accumulation and penalty effects are properly set
// This should be called when loading a save or after modifying fatigue/hunger values
func InitializeFatigueHungerEffects(state *types.SaveFile) error {
	// Migrate old effect IDs to new ones for backward compatibility
	effects.MigrateOldEffectIDs(state)

	// Ensure fatigue accumulation effect is present
	if err := EnsureFatigueAccumulation(state); err != nil {
		return fmt.Errorf("failed to ensure fatigue accumulation: %w", err)
	}

	// Ensure hunger accumulation effect is present
	if err := EnsureHungerAccumulation(state); err != nil {
		return fmt.Errorf("failed to ensure hunger accumulation: %w", err)
	}

	// Initialize equipment effects FIRST (before calculating penalties that depend on them)
	// This must run before encumbrance penalties, as backpack affects weight capacity
	if err := InitializeEquipmentEffects(state); err != nil {
		return fmt.Errorf("failed to initialize equipment effects: %w", err)
	}

	// Apply penalty effects based on current levels
	if _, err := UpdateFatiguePenaltyEffects(state); err != nil {
		return fmt.Errorf("failed to update fatigue penalty effects: %w", err)
	}

	if _, err := UpdateHungerPenaltyEffects(state); err != nil {
		return fmt.Errorf("failed to update hunger penalty effects: %w", err)
	}

	// Apply encumbrance effects based on current weight
	// This now runs AFTER equipment effects are initialized, so capacity calculation is correct
	if _, err := UpdateEncumbrancePenaltyEffects(state); err != nil {
		return fmt.Errorf("failed to update encumbrance penalty effects: %w", err)
	}

	return nil
}

// InitializeEquipmentEffects scans all equipped items and applies their effects_when_worn
// This is needed when loading a save file, as effects are only normally applied during equip action
func InitializeEquipmentEffects(state *types.SaveFile) error {
	if state.Inventory == nil {
		return nil
	}

	gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return nil
	}

	database := db.GetDB()
	if database == nil {
		log.Printf("⚠️ Database unavailable, cannot initialize equipment effects")
		return nil
	}

	// Scan all equipment slots
	for slotName, slotData := range gearSlots {
		slotMap, ok := slotData.(map[string]interface{})
		if !ok {
			continue
		}

		itemID, ok := slotMap["item"].(string)
		if !ok || itemID == "" {
			continue
		}

		// Skip if this is the same item in both hands (two-handed weapon, only process once)
		if slotName == "offhand" {
			if mainhandSlot, ok := gearSlots["mainhand"].(map[string]interface{}); ok {
				if mainhandItem, _ := mainhandSlot["item"].(string); mainhandItem == itemID {
					continue // Already processed in mainhand
				}
			}
		}

		// Load item properties
		var propertiesJSON string
		err := database.QueryRow("SELECT properties FROM items WHERE id = ?", itemID).Scan(&propertiesJSON)
		if err != nil {
			continue
		}

		var properties map[string]interface{}
		if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
			continue
		}

		// Check for effects_when_worn
		effectsWhenWorn, ok := properties["effects_when_worn"].([]interface{})
		if !ok {
			continue
		}

		// Apply each effect
		for _, effectID := range effectsWhenWorn {
			if effectIDStr, ok := effectID.(string); ok {
				// Check if effect is already active (don't duplicate)
				alreadyActive := false
				for _, ae := range state.ActiveEffects {
					if ae.EffectID == effectIDStr {
						alreadyActive = true
						break
					}
				}

				if !alreadyActive {
					// Apply effect silently
					if err := effects.ApplyEffect(state, effectIDStr); err != nil {
						log.Printf("⚠️ Failed to initialize equipment effect '%s' from %s: %v", effectIDStr, itemID, err)
					} else {
						log.Printf("🔄 Initialized equipment effect: %s from %s", effectIDStr, itemID)
					}
				}
			}
		}
	}

	return nil
}

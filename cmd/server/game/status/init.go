package status

import (
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

	if _, ok := state.Inventory["gear_slots"].(map[string]interface{}); !ok {
		return nil
	}

	database := db.GetDB()
	if database == nil {
		log.Printf("⚠️ Database unavailable, cannot initialize equipment effects")
		return nil
	}

	// One instance per piece that grants it (two ruby rings, two), and none
	// left over from gear that's gone.
	effects.SyncWorn(state)
	return nil
}

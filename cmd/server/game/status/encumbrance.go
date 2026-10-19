package status

import (
	"log"

	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/types"
)

// CalculateTotalWeight delegates to the canonical gameutil implementation
func CalculateTotalWeight(state *types.SaveFile) float64 {
	return gameutil.CalculateTotalWeight(state)
}

// CalculateWeightCapacity delegates to the canonical gameutil implementation
func CalculateWeightCapacity(state *types.SaveFile) float64 {
	return gameutil.CalculateWeightCapacity(state)
}

// GetItemWeight delegates to the canonical gameutil implementation
func GetItemWeight(itemID string) float64 {
	return gameutil.GetItemWeight(itemID)
}

// GetItemWeightRecursive delegates to the canonical gameutil implementation
func GetItemWeightRecursive(slot map[string]interface{}) float64 {
	return gameutil.GetItemWeightRecursive(slot)
}

// UpdateEncumbrancePenaltyEffects applies appropriate penalty effects based on encumbrance level
func UpdateEncumbrancePenaltyEffects(state *types.SaveFile) (*types.EffectMessage, error) {
	return UpdateSystemStatusEffects(state, "encumbrance")
}

// RemoveEncumbrancePenaltyEffects - DEPRECATED: Now handled by UpdateSystemStatusEffects
func RemoveEncumbrancePenaltyEffects(state *types.SaveFile) {
	log.Printf("⚠️ RemoveEncumbrancePenaltyEffects called but is deprecated - use UpdateSystemStatusEffects")
}

// HandleEncumbranceChange processes encumbrance change after inventory modifications
func HandleEncumbranceChange(state *types.SaveFile) {
	if msg, err := UpdateEncumbrancePenaltyEffects(state); err != nil {
		log.Printf("⚠️ Failed to update encumbrance effects: %v", err)
	} else if msg != nil && !msg.Silent {
		log.Printf("📦 Encumbrance changed: %s", msg.Message)
	}
}

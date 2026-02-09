package status

import (
	"encoding/json"
	"log"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// CalculateTotalWeight calculates the total weight of all items in inventory
func CalculateTotalWeight(state *types.SaveFile) float64 {
	totalWeight := 0.0

	if state.Inventory == nil {
		return 0
	}

	gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return 0
	}

	// Calculate weight from equipped items
	for slotName, slotData := range gearSlots {
		if slotName == "bag" {
			// Handle bag contents separately
			if bagData, ok := slotData.(map[string]interface{}); ok {
				if contents, ok := bagData["contents"].([]interface{}); ok {
					for _, contentItem := range contents {
						if item, ok := contentItem.(map[string]interface{}); ok {
							itemID, _ := item["item"].(string)
							quantity, _ := item["quantity"].(float64)
							if itemID != "" && quantity > 0 {
								totalWeight += GetItemWeight(itemID) * quantity
							}
						}
					}
				}
				// Add the bag's own weight
				if bagItemID, ok := bagData["item"].(string); ok && bagItemID != "" {
					totalWeight += GetItemWeight(bagItemID)
				}
			}
		} else {
			// Regular equipment slot
			if slotData, ok := slotData.(map[string]interface{}); ok {
				itemID, _ := slotData["item"].(string)
				quantity, _ := slotData["quantity"].(float64)
				if itemID != "" && quantity > 0 {
					totalWeight += GetItemWeight(itemID) * quantity
				}
			}
		}
	}

	// Calculate weight from general slots
	if generalSlots, ok := state.Inventory["general_slots"].([]interface{}); ok {
		for _, slotData := range generalSlots {
			if slot, ok := slotData.(map[string]interface{}); ok {
				itemID, _ := slot["item"].(string)
				quantity, _ := slot["quantity"].(float64)
				if itemID != "" && quantity > 0 {
					totalWeight += GetItemWeight(itemID) * quantity
				}
			}
		}
	}

	return totalWeight
}

// GetItemWeight retrieves weight from an item's properties
func GetItemWeight(itemID string) float64 {
	item, err := db.GetItemByID(itemID)
	if err != nil {
		return 0
	}

	// Parse properties JSON to get weight
	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(item.Properties), &properties); err != nil {
		return 0
	}

	if weight, ok := properties["weight"].(float64); ok {
		return weight
	}

	return 0
}

// CalculateWeightCapacity calculates max carrying capacity based on STR and equipment
func CalculateWeightCapacity(state *types.SaveFile) float64 {
	// Base capacity = 5 * STR (as per encumbrance.json formula)
	strength := 10.0 // Default
	if state.Stats != nil {
		if str, ok := state.Stats["Strength"].(float64); ok {
			strength = str
		} else if str, ok := state.Stats["strength"].(float64); ok {
			strength = str
		}
	}

	baseCapacity := 5.0 * strength

	// Add weight_increase from equipped containers (like backpack)
	if state.Inventory != nil {
		if gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{}); ok {
			if bagData, ok := gearSlots["bag"].(map[string]interface{}); ok {
				if bagItemID, ok := bagData["item"].(string); ok && bagItemID != "" {
					item, err := db.GetItemByID(bagItemID)
					if err == nil {
						var properties map[string]interface{}
						if err := json.Unmarshal([]byte(item.Properties), &properties); err == nil {
							if weightIncrease, ok := properties["weight_increase"].(float64); ok {
								baseCapacity += weightIncrease
							}
						}
					}
				}
			}
		}
	}

	return baseCapacity
}

// GetEncumbranceLevel returns the encumbrance category based on weight percentage
// Categories: "light" (0-50%), "normal" (51-100%), "overweight" (101-150%), "encumbered" (151-200%), "overloaded" (201%+)
func GetEncumbranceLevel(state *types.SaveFile) string {
	totalWeight := CalculateTotalWeight(state)
	capacity := CalculateWeightCapacity(state)

	if capacity <= 0 {
		return "normal"
	}

	percentage := (totalWeight / capacity) * 100

	switch {
	case percentage <= 50:
		return "light"
	case percentage <= 100:
		return "normal"
	case percentage <= 150:
		return "overweight"
	case percentage <= 200:
		return "encumbered"
	default:
		return "overloaded"
	}
}

// UpdateEncumbrancePenaltyEffects applies appropriate penalty effects based on encumbrance level
// Now uses data-driven system - effect activation defined in JSON system_check
func UpdateEncumbrancePenaltyEffects(state *types.SaveFile) (*types.EffectMessage, error) {
	// Use generic data-driven system
	return UpdateSystemStatusEffects(state, "encumbrance")
}

// RemoveEncumbrancePenaltyEffects - DEPRECATED: Now handled by UpdateSystemStatusEffects
// Kept for backward compatibility but no longer used
func RemoveEncumbrancePenaltyEffects(state *types.SaveFile) {
	// This function is no longer needed - UpdateSystemStatusEffects handles removal
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

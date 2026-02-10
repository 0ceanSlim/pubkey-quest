package gameutil

import (
	"encoding/json"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// GetItemWeight retrieves the base weight of an item from the database
func GetItemWeight(itemID string) float64 {
	item, err := db.GetItemByID(itemID)
	if err != nil {
		return 0
	}
	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(item.Properties), &properties); err != nil {
		return 0
	}
	if weight, ok := properties["weight"].(float64); ok {
		return weight
	}
	return 0
}

// GetItemWeightRecursive calculates the weight of a slot including any container contents
func GetItemWeightRecursive(slot map[string]interface{}) float64 {
	itemID, ok := slot["item"].(string)
	if !ok || itemID == "" || itemID == "null" {
		return 0
	}

	baseWeight := GetItemWeight(itemID)

	contents, ok := slot["contents"].([]interface{})
	if !ok || len(contents) == 0 {
		return baseWeight
	}

	contentsWeight := 0.0
	for _, contentItem := range contents {
		if item, ok := contentItem.(map[string]interface{}); ok {
			contentID, _ := item["item"].(string)
			if contentID == "" || contentID == "null" {
				continue
			}
			qty := 1.0
			if q, ok := item["quantity"].(float64); ok {
				qty = q
			} else if q, ok := item["quantity"].(int); ok {
				qty = float64(q)
			}
			if qty > 0 {
				contentsWeight += GetItemWeightRecursive(item) * qty
			}
		}
	}

	return baseWeight + contentsWeight
}

// CalculateTotalWeight calculates the total weight of all items in inventory,
// including contents of any containers in any slot.
func CalculateTotalWeight(state *types.SaveFile) float64 {
	if state.Inventory == nil {
		return 0
	}

	total := 0.0

	// General slots (can contain containers with contents)
	if generalSlots, ok := state.Inventory["general_slots"].([]interface{}); ok {
		for _, slotData := range generalSlots {
			if slot, ok := slotData.(map[string]interface{}); ok {
				itemID, _ := slot["item"].(string)
				if itemID == "" || itemID == "null" {
					continue
				}
				qty := 1.0
				if q, ok := slot["quantity"].(float64); ok {
					qty = q
				} else if q, ok := slot["quantity"].(int); ok {
					qty = float64(q)
				}
				if qty > 0 {
					total += GetItemWeightRecursive(slot) * qty
				}
			}
		}
	}

	// Equipment slots (can also contain containers — bag, ammo pouch, etc.)
	if gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{}); ok {
		for _, slotData := range gearSlots {
			slotMap, ok := slotData.(map[string]interface{})
			if !ok {
				continue
			}
			itemID, _ := slotMap["item"].(string)
			if itemID == "" || itemID == "null" {
				continue
			}
			qty := 1.0
			if q, ok := slotMap["quantity"].(float64); ok {
				qty = q
			} else if q, ok := slotMap["quantity"].(int); ok {
				qty = float64(q)
			}
			if qty > 0 {
				total += GetItemWeightRecursive(slotMap) * qty
			}
		}
	}

	return total
}

// CalculateWeightCapacity calculates max carrying capacity based on STR and active effects
func CalculateWeightCapacity(state *types.SaveFile) float64 {
	baseMultiplier := getBaseWeightMultiplier()

	strength := 10.0
	if state.Stats != nil {
		if str, ok := state.Stats["Strength"].(float64); ok {
			strength = str
		} else if str, ok := state.Stats["strength"].(float64); ok {
			strength = str
		}
	}

	capacity := baseMultiplier * strength

	database := db.GetDB()
	if database != nil {
		for _, activeEffect := range state.ActiveEffects {
			var propertiesJSON string
			err := database.QueryRow("SELECT properties FROM effects WHERE id = ?", activeEffect.EffectID).Scan(&propertiesJSON)
			if err != nil {
				continue
			}
			var fullEffect types.EffectData
			if err := json.Unmarshal([]byte(propertiesJSON), &fullEffect); err != nil {
				continue
			}
			for _, modifier := range fullEffect.Modifiers {
				if modifier.Stat == "weight_capacity" && modifier.Type == "constant" {
					capacity += float64(modifier.Value)
				}
			}
		}
	}

	return capacity
}

// getBaseWeightMultiplier loads the base weight multiplier from encumbrance config
func getBaseWeightMultiplier() float64 {
	database := db.GetDB()
	if database == nil {
		return 5.0
	}
	var configJSON string
	if err := database.QueryRow("SELECT properties FROM systems WHERE id = 'encumbrance'").Scan(&configJSON); err != nil {
		return 5.0
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return 5.0
	}
	if enc, ok := config["encumbrance_system"].(map[string]interface{}); ok {
		if wc, ok := enc["weight_calculation"].(map[string]interface{}); ok {
			if m, ok := wc["base_weight_multiplier"].(float64); ok {
				return m
			}
		}
	}
	return 5.0
}

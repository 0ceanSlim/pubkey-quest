package inventory

import (
	"encoding/json"
	"fmt"
	"log"
	"slices"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// AddItemToInventory adds items to player inventory with intelligent stacking and slot priority
// Returns: (itemsAdded int, error)
func AddItemToInventory(save *types.SaveFile, itemID string, quantity int) (int, error) {
	log.Printf("🔧 AddItemToInventory called: itemID=%s, quantity=%d", itemID, quantity)

	inventory := save.Inventory

	// Get item data to check max stack size
	itemData, err := db.GetItemByID(itemID)
	if err != nil {
		return 0, fmt.Errorf("item not found: %s", itemID)
	}

	// Parse stack size from properties JSON
	maxStack := 1 // Default to 1
	if itemData.Properties != "" {
		var properties map[string]interface{}
		if err := json.Unmarshal([]byte(itemData.Properties), &properties); err == nil {
			if val, ok := properties["stack"].(float64); ok {
				maxStack = int(val)
			}
		}
	}

	// Containers never nest: a backpack can't go inside the backpack. move_item
	// enforces this for drags, so honour it here too — otherwise withdrawing a
	// pouch from the vault (or looting one) smuggles it into the bag.
	isContainer := itemHasTag(itemData.Tags, "container")

	log.Printf("📦 Adding %dx %s to inventory (max stack: %d, container: %t)", quantity, itemID, maxStack, isContainer)

	// Get inventory slots
	generalSlots, ok := inventory["general_slots"].([]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid inventory structure")
	}

	gearSlots, ok := inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid gear slots")
	}

	bag, _ := gearSlots["bag"].(map[string]interface{})
	backpackSlots, _ := bag["contents"].([]interface{})

	remaining := quantity
	totalAdded := 0

	log.Printf("🔍 Inventory state: %d general slots, %d backpack slots", len(generalSlots), len(backpackSlots))

	// STEP 1: Try to stack with existing items in backpack first
	if backpackSlots != nil && !isContainer {
		log.Printf("🔍 Checking backpack for existing %s stacks...", itemID)
		for i, slotData := range backpackSlots {
			if remaining <= 0 {
				break
			}

			slot, ok := slotData.(map[string]interface{})
			if !ok {
				continue
			}

			if slot["item"] != itemID {
				log.Printf("  🔍 backpack[%d]: %v (not a match)", i, slot["item"])
				continue
			}

			log.Printf("  ✨ backpack[%d]: Found existing %s!", i, itemID)
			currentQty := GetSlotQuantity(slot)
			if currentQty >= maxStack {
				continue // Already at max stack
			}

			canAdd := maxStack - currentQty
			if canAdd > remaining {
				canAdd = remaining
			}

			slot["quantity"] = currentQty + canAdd
			backpackSlots[i] = slot
			remaining -= canAdd
			totalAdded += canAdd
			log.Printf("  ✅ Stacked %d in backpack[%d] (now %d)", canAdd, i, currentQty+canAdd)
		}
	}

	// STEP 2: Try to stack with existing items in general slots
	for i, slotData := range generalSlots {
		if remaining <= 0 {
			break
		}

		slot, ok := slotData.(map[string]interface{})
		if !ok || slot["item"] != itemID {
			continue
		}

		currentQty := GetSlotQuantity(slot)
		if currentQty >= maxStack {
			continue // Already at max stack
		}

		canAdd := maxStack - currentQty
		if canAdd > remaining {
			canAdd = remaining
		}

		slot["quantity"] = currentQty + canAdd
		generalSlots[i] = slot
		remaining -= canAdd
		totalAdded += canAdd
		log.Printf("  ✅ Stacked %d in general[%d] (now %d)", canAdd, i, currentQty+canAdd)
	}

	// STEP 3: Fill empty backpack slots (never for containers — see above)
	for i, slotData := range backpackSlots {
		if remaining <= 0 || isContainer {
			break
		}

		slot, ok := slotData.(map[string]interface{})
		if !ok {
			continue
		}

		if slot["item"] == nil || slot["item"] == "" {
			toAdd := remaining
			if toAdd > maxStack {
				toAdd = maxStack
			}

			slot["item"] = itemID
			slot["quantity"] = toAdd
			backpackSlots[i] = slot
			remaining -= toAdd
			totalAdded += toAdd
			log.Printf("  ✅ Added %d to empty backpack[%d]", toAdd, i)
		}
	}

	// STEP 4: Fill empty general slots
	for i, slotData := range generalSlots {
		if remaining <= 0 {
			break
		}

		slot, ok := slotData.(map[string]interface{})
		if !ok {
			continue
		}

		if slot["item"] == nil || slot["item"] == "" {
			toAdd := remaining
			if toAdd > maxStack {
				toAdd = maxStack
			}

			slot["item"] = itemID
			slot["quantity"] = toAdd
			generalSlots[i] = slot
			remaining -= toAdd
			totalAdded += toAdd
			log.Printf("  ✅ Added %d to empty general[%d]", toAdd, i)
		}
	}

	// Update inventory
	if backpackSlots != nil {
		bag["contents"] = backpackSlots
		gearSlots["bag"] = bag
	}
	inventory["general_slots"] = generalSlots
	inventory["gear_slots"] = gearSlots

	if remaining > 0 {
		log.Printf("⚠️ Inventory full - added %d/%d items (%d couldn't fit)", totalAdded, quantity, remaining)
		if totalAdded == 0 {
			return 0, fmt.Errorf("no room in inventory")
		}
		return totalAdded, nil // Partial success
	}

	log.Printf("✅ Successfully added all %d items", totalAdded)
	return totalAdded, nil
}

// GetSlotQuantity extracts quantity from a slot map (handles both int and float64)
func GetSlotQuantity(slot map[string]interface{}) int {
	switch v := slot["quantity"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}

// itemHasTag reports whether an item's tags JSON (as stored in the items table)
// contains tag. A tags blob that won't parse is treated as untagged.
func itemHasTag(tagsJSON, tag string) bool {
	if tagsJSON == "" {
		return false
	}
	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return false
	}
	return slices.Contains(tags, tag)
}

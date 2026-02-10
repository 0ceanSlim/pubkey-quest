package inventory

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/types"
)

// HandleUseItemAction uses an item from inventory
func HandleUseItemAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	itemID, ok := params["item_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid item_id parameter")
	}

	// Validate item is consumable
	database := db.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}

	var propertiesJSON string
	var tagsJSON string
	err := database.QueryRow("SELECT properties, tags FROM items WHERE id = ?", itemID).Scan(&propertiesJSON, &tagsJSON)
	if err != nil {
		return nil, fmt.Errorf("item '%s' not found in database: %v", itemID, err)
	}

	// Check for "consumable" tag
	isConsumable := false
	var tags []interface{}
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err == nil {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok && tagStr == "consumable" {
				isConsumable = true
				break
			}
		}
	}

	if !isConsumable {
		return nil, fmt.Errorf("item '%s' is not consumable", itemID)
	}

	slot := -1
	if s, ok := params["slot"].(float64); ok {
		slot = int(s)
	}

	// Find and remove item from inventory (check both general and backpack)
	var itemFound bool
	var effects []string

	// Check general slots
	generalSlots, ok := state.Inventory["general_slots"].([]interface{})
	if ok {
		for i, slotData := range generalSlots {
			slotMap, ok := slotData.(map[string]interface{})
			if !ok {
				continue
			}

			if slotMap["item"] == itemID && (slot < 0 || i == slot) {
				itemFound = true

				// Apply item effects based on item ID
				effects = ApplyItemEffects(state, itemID)

				// Remove/reduce item quantity
				qty, _ := slotMap["quantity"].(float64)
				if qty > 1 {
					slotMap["quantity"] = qty - 1
				} else {
					slotMap["item"] = nil
					slotMap["quantity"] = 0
				}
				break
			}
		}
	}

	// If not found in general, check backpack
	if !itemFound {
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		contents, ok := bag["contents"].([]interface{})
		if ok {
			for i, slotData := range contents {
				slotMap, ok := slotData.(map[string]interface{})
				if !ok {
					continue
				}

				if slotMap["item"] == itemID && (slot < 0 || i == slot) {
					itemFound = true

					// Apply item effects
					effects = ApplyItemEffects(state, itemID)

					// Remove/reduce item quantity
					qty, _ := slotMap["quantity"].(float64)
					if qty > 1 {
						slotMap["quantity"] = qty - 1
					} else {
						slotMap["item"] = nil
						slotMap["quantity"] = 0
					}
					break
				}
			}
		}
	}

	if !itemFound {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}

	effectMsg := "Used item"
	if len(effects) > 0 {
		effectMsg = fmt.Sprintf("Used %s: %s", itemID, effects[0])
	}

	return &types.GameActionResponse{
		Success: true,
		Message: effectMsg,
	}, nil
}

// ApplyItemEffects applies item effects to the character state dynamically from item data
func ApplyItemEffects(state *types.SaveFile, itemID string) []string {
	var effectMessages []string

	// Get database connection
	database := db.GetDB()
	if database == nil {
		log.Printf("⚠️ Database not available, cannot apply effects for %s", itemID)
		return []string{"Used"}
	}

	// Query item from database to get properties
	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM items WHERE id = ?", itemID).Scan(&propertiesJSON)
	if err != nil {
		log.Printf("⚠️ Could not find item %s in database: %v", itemID, err)
		return []string{"Used"}
	}

	// Parse properties JSON
	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
		log.Printf("⚠️ Could not parse properties for item %s: %v", itemID, err)
		return []string{"Used"}
	}

	// Check if item has effects array
	effectsRaw, hasEffects := properties["effects"]
	if !hasEffects {
		log.Printf("⚠️ Item %s has no effects defined", itemID)
		return []string{"Used"}
	}

	// Parse effects array
	effectsArray, ok := effectsRaw.([]interface{})
	if !ok {
		log.Printf("⚠️ Item %s has invalid effects format", itemID)
		return []string{"Used"}
	}

	// Apply each effect
	for _, effectRaw := range effectsArray {
		effectMap, ok := effectRaw.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for named effect with chance (new format)
		if applyEffectID, ok := effectMap["apply_effect"].(string); ok {
			// Get chance (default 100%)
			chance := 100
			if chanceVal, ok := effectMap["chance"].(float64); ok {
				chance = int(chanceVal)
			}

			// Roll random number (1-100)
			roll := rand.Intn(100) + 1

			if roll <= chance {
				// Apply the named effect
				msg, err := effects.ApplyEffectWithMessage(state, applyEffectID)
				if err == nil && msg != nil {
					effectMessages = append(effectMessages, msg.Message)
					log.Printf("✅ Applied consumable effect '%s' (rolled %d <= %d)", applyEffectID, roll, chance)
				} else {
					log.Printf("⚠️ Failed to apply effect '%s': %v", applyEffectID, err)
				}
			} else {
				log.Printf("❌ Effect '%s' did not apply (rolled %d > %d)", applyEffectID, roll, chance)
			}

			continue
		}

		// Inline effect handling (legacy format - type, value)
		effectType, _ := effectMap["type"].(string)
		effectValue, _ := effectMap["value"].(float64) // JSON numbers are float64

		switch effectType {
		case "hp", "health":
			oldHP := state.HP
			state.HP = min(state.MaxHP, state.HP+int(effectValue))
			actualHealed := state.HP - oldHP
			if actualHealed > 0 {
				effectMessages = append(effectMessages, fmt.Sprintf("Healed %d HP", actualHealed))
			}

		case "mana":
			oldMana := state.Mana
			state.Mana = min(state.MaxMana, state.Mana+int(effectValue))
			actualRestored := state.Mana - oldMana
			if actualRestored > 0 {
				effectMessages = append(effectMessages, fmt.Sprintf("Restored %d mana", actualRestored))
			}

		case "hunger":
			oldHunger := state.Hunger
			state.Hunger = min(3, max(0, state.Hunger+int(effectValue)))
			status.HandleHungerChange(state)
			if state.Hunger > oldHunger {
				effectMessages = append(effectMessages, "Hunger restored")
			} else if state.Hunger < oldHunger {
				effectMessages = append(effectMessages, "Hunger decreased")
			}

		case "fatigue":
			oldFatigue := state.Fatigue
			state.Fatigue = max(0, min(10, state.Fatigue+int(effectValue)))
			status.HandleFatigueChange(state)
			if state.Fatigue < oldFatigue {
				fatigueReduced := oldFatigue - state.Fatigue
				effectMessages = append(effectMessages, fmt.Sprintf("Fatigue reduced by %d", fatigueReduced))
			} else if state.Fatigue > oldFatigue {
				fatigueIncreased := state.Fatigue - oldFatigue
				effectMessages = append(effectMessages, fmt.Sprintf("Fatigue increased by %d", fatigueIncreased))
			}

		default:
			log.Printf("⚠️ Unknown effect type: %s", effectType)
		}
	}

	if len(effectMessages) == 0 {
		return []string{"Used"}
	}

	return effectMessages
}

// HandleDropItemAction drops an item from inventory
func HandleDropItemAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	itemID, ok := params["item_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid item_id parameter")
	}

	slot, _ := params["slot"].(float64)
	slotType, _ := params["slot_type"].(string)
	if slotType == "" {
		slotType = "general" // Default to general slots
	}

	// Get the quantity to drop (default to all if not specified)
	dropQuantity := -1
	if qty, ok := params["quantity"].(float64); ok {
		dropQuantity = int(qty)
	}

	log.Printf("📤 Dropping %s: quantity=%d from %s[%d]", itemID, dropQuantity, slotType, int(slot))

	// Find item in appropriate inventory
	var itemFound bool
	var inventory []interface{}

	switch slotType {
	case "general":
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid inventory structure")
		}
		inventory = generalSlots
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		backpackContents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack structure")
		}
		inventory = backpackContents
	default:
		return nil, fmt.Errorf("invalid slot_type: %s", slotType)
	}

	// Search for item
	for i, slotData := range inventory {
		slotMap, ok := slotData.(map[string]interface{})
		if !ok {
			continue
		}

		if slotMap["item"] == itemID && (slot < 0 || i == int(slot)) {
			itemFound = true

			// Get current quantity
			currentQty := 1
			if qty, ok := slotMap["quantity"].(float64); ok {
				currentQty = int(qty)
			}

			// Determine how much to drop
			if dropQuantity <= 0 || dropQuantity >= currentQty {
				// Drop entire stack
				slotMap["item"] = nil
				slotMap["quantity"] = 0
				log.Printf("✅ Dropped entire stack of %s (%d items)", itemID, currentQty)
			} else {
				// Drop partial stack (store as int)
				slotMap["quantity"] = currentQty - dropQuantity
				log.Printf("✅ Dropped %d %s (keeping %d)", dropQuantity, itemID, currentQty-dropQuantity)
			}
			break
		}
	}

	if !itemFound {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Dropped %s", itemID),
	}, nil
}

// HandleRemoveFromInventoryAction removes an item from inventory (for sell staging)
func HandleRemoveFromInventoryAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	itemID, ok := params["item_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid item_id parameter")
	}

	fromSlot, _ := params["from_slot"].(float64)
	fromSlotType, _ := params["from_slot_type"].(string)
	if fromSlotType == "" {
		fromSlotType = "general" // Default to general slots
	}

	// Get the quantity to remove (default to 1)
	removeQuantity := 1
	if qty, ok := params["quantity"].(float64); ok {
		removeQuantity = int(qty)
	}

	log.Printf("🛒 Removing %dx %s from %s[%d] for sell staging", removeQuantity, itemID, fromSlotType, int(fromSlot))

	// Find item in appropriate inventory
	var itemFound bool
	var inventory []interface{}

	switch fromSlotType {
	case "general":
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid inventory structure")
		}
		inventory = generalSlots
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		backpackContents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack structure")
		}
		inventory = backpackContents
	default:
		return nil, fmt.Errorf("invalid slot_type: %s", fromSlotType)
	}

	// Search for item at specific slot
	if int(fromSlot) < 0 || int(fromSlot) >= len(inventory) {
		return nil, fmt.Errorf("invalid slot index: %d", int(fromSlot))
	}

	slotMap, ok := inventory[int(fromSlot)].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid slot data at index %d", int(fromSlot))
	}

	if slotMap["item"] != itemID {
		return nil, fmt.Errorf("item mismatch: expected %s, found %v", itemID, slotMap["item"])
	}

	itemFound = true

	// Get current quantity (ensure it's an integer)
	currentQty := 1
	if qty, ok := slotMap["quantity"].(float64); ok {
		currentQty = int(qty)
	} else if qty, ok := slotMap["quantity"].(int); ok {
		currentQty = qty
	}

	log.Printf("🔢 Current quantity at slot: %d (type: %T)", currentQty, slotMap["quantity"])

	if currentQty < removeQuantity {
		return nil, fmt.Errorf("not enough items: have %d, trying to remove %d", currentQty, removeQuantity)
	}

	// Remove from stack (ALWAYS store as int, not float64)
	newQty := currentQty - removeQuantity
	if newQty <= 0 {
		// Remove entire stack
		slotMap["item"] = nil
		slotMap["quantity"] = 0
		log.Printf("✅ Removed entire stack of %s (%d items)", itemID, currentQty)
	} else {
		// Remove partial stack - store as int
		slotMap["quantity"] = newQty
		log.Printf("✅ Removed %d %s (keeping %d) - stored as %T", removeQuantity, itemID, newQty, slotMap["quantity"])
	}

	if !itemFound {
		return nil, fmt.Errorf("item not found: %s at slot %d", itemID, int(fromSlot))
	}

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Removed %dx %s from inventory", removeQuantity, itemID),
	}, nil
}

// HandleMoveItemAction moves items between inventory slots
func HandleMoveItemAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	itemID, _ := params["item_id"].(string)
	fromSlot := int(params["from_slot"].(float64))
	toSlot := int(params["to_slot"].(float64))
	fromSlotType, _ := params["from_slot_type"].(string)
	toSlotType, _ := params["to_slot_type"].(string)

	// Get the appropriate slot arrays
	var fromSlots, toSlots []interface{}
	var vaultBuilding string

	// Get vault building ID if dealing with vault
	if params["vault_building"] != nil {
		vaultBuilding, _ = params["vault_building"].(string)
	}

	// Get from slots
	switch fromSlotType {
	case "general":
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid general slots")
		}
		fromSlots = generalSlots
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		contents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack")
		}
		fromSlots = contents
	case "vault":
		vaultData := vault.GetVaultForLocation(state, vaultBuilding)
		if vaultData == nil {
			return nil, fmt.Errorf("vault not found for building: %s", vaultBuilding)
		}
		slots, ok := vaultData["slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid vault slots")
		}
		fromSlots = slots
	}

	// Get to slots
	switch toSlotType {
	case "general":
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid general slots")
		}
		toSlots = generalSlots
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		contents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack")
		}
		toSlots = contents
	case "vault":
		vaultData := vault.GetVaultForLocation(state, vaultBuilding)
		if vaultData == nil {
			return nil, fmt.Errorf("vault not found for building: %s", vaultBuilding)
		}
		slots, ok := vaultData["slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid vault slots")
		}
		toSlots = slots
	}

	// CRITICAL VALIDATION: Containers cannot go into backpack
	if toSlotType == "inventory" {
		log.Printf("🔍 VALIDATION CHECK: Is '%s' a container? (destination: backpack)", itemID)

		database := db.GetDB()
		if database == nil {
			log.Printf("❌ CRITICAL: Database not available")
			return &types.GameActionResponse{
				Success: false,
				Error:   "System error: Cannot validate item restrictions",
				Color:   "red",
			}, nil
		}

		// Query tags from database
		var tagsJSON string
		err := database.QueryRow("SELECT tags FROM items WHERE id = ?", itemID).Scan(&tagsJSON)
		if err != nil {
			log.Printf("❌ CRITICAL: Failed to query tags for %s: %v", itemID, err)
			return &types.GameActionResponse{
				Success: false,
				Error:   fmt.Sprintf("System error: Cannot find item %s", itemID),
				Color:   "red",
			}, nil
		}

		log.Printf("📦 Raw tags JSON from database for '%s': %s", itemID, tagsJSON)

		var tags []interface{}
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
			log.Printf("❌ CRITICAL: Failed to parse tags JSON for %s: %v", itemID, err)
			return &types.GameActionResponse{
				Success: false,
				Error:   "System error: Invalid item data format",
				Color:   "red",
			}, nil
		}

		log.Printf("📦 Parsed tags array for '%s': %v", itemID, tags)

		// Check each tag
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				log.Printf("   🏷️ Found tag: '%s'", tagStr)
				if tagStr == "container" {
					log.Printf("❌ BLOCKED: '%s' has 'container' tag - CANNOT go in backpack!", itemID)
					return &types.GameActionResponse{
						Success: false,
						Error:   "Containers cannot be stored in the backpack",
						Color:   "red",
					}, nil
				}
			}
		}

		log.Printf("✅ VALIDATION PASSED: '%s' is NOT a container - allowing move to backpack", itemID)
	}

	// ADDITIONAL VALIDATION: Check displaced item in swap scenarios
	log.Printf("🔍 Checking swap validation: fromSlotType=%s, toSlotType=%s", fromSlotType, toSlotType)
	if fromSlotType == "inventory" && toSlotType != "inventory" {
		log.Printf("🔍 Condition met: dragging FROM backpack TO %s", toSlotType)
		// Check if we're swapping (destination slot is not empty)
		if toSlots != nil && toSlot < len(toSlots) {
			log.Printf("🔍 Checking destination slot %d (toSlots length: %d)", toSlot, len(toSlots))
			if destSlot, ok := toSlots[toSlot].(map[string]interface{}); ok {
				log.Printf("🔍 Destination slot data: %+v", destSlot)
				if destItem, ok := destSlot["item"].(string); ok && destItem != "" {
					// There's an item in the destination - this is a swap
					log.Printf("🔍 SWAP VALIDATION: Checking if displaced item '%s' is a container (would go to backpack)", destItem)

					database := db.GetDB()
					if database != nil {
						var tagsJSON string
						err := database.QueryRow("SELECT tags FROM items WHERE id = ?", destItem).Scan(&tagsJSON)
						if err == nil {
							var tags []interface{}
							if err := json.Unmarshal([]byte(tagsJSON), &tags); err == nil {
								for _, tag := range tags {
									if tagStr, ok := tag.(string); ok && tagStr == "container" {
										log.Printf("❌ BLOCKED: Displaced item '%s' is a container and cannot go in backpack via swap!", destItem)
										return &types.GameActionResponse{
											Success: false,
											Error:   "Containers cannot be stored in the backpack",
											Color:   "red",
										}, nil
									}
								}
							}
						}
					}
					log.Printf("✅ Swap validated: Displaced item '%s' is not a container", destItem)
				}
			}
		}
	}

	// Swap items
	if fromSlots != nil && toSlots != nil && fromSlot >= 0 && toSlot >= 0 {
		if fromSlotType == toSlotType {
			// Same array, just swap within it
			fromSlots[fromSlot], fromSlots[toSlot] = fromSlots[toSlot], fromSlots[fromSlot]

			// Update the "slot" field in each swapped item
			if fromSlotMap, ok := fromSlots[fromSlot].(map[string]interface{}); ok {
				fromSlotMap["slot"] = fromSlot
			}
			if toSlotMap, ok := fromSlots[toSlot].(map[string]interface{}); ok {
				toSlotMap["slot"] = toSlot
			}
		} else {
			// Different arrays, swap between them
			temp := fromSlots[fromSlot]
			fromSlots[fromSlot] = toSlots[toSlot]
			toSlots[toSlot] = temp

			// Update the "slot" field in each swapped item
			if fromSlotMap, ok := fromSlots[fromSlot].(map[string]interface{}); ok {
				fromSlotMap["slot"] = fromSlot
			}
			if toSlotMap, ok := toSlots[toSlot].(map[string]interface{}); ok {
				toSlotMap["slot"] = toSlot
			}
		}

		log.Printf("✅ Swapped slots: %s[%d] ↔ %s[%d]", fromSlotType, fromSlot, toSlotType, toSlot)
	}

	// If vault was involved, return updated vault data
	delta := map[string]interface{}{}
	if fromSlotType == "vault" || toSlotType == "vault" {
		log.Printf("🏦 Vault involved: from=%s, to=%s, building=%s", fromSlotType, toSlotType, vaultBuilding)
		vaultData := vault.GetVaultForLocation(state, vaultBuilding)
		if vaultData != nil {
			delta["vault_data"] = vaultData
			log.Printf("✅ Returning updated vault data with %d slots", len(vaultData["slots"].([]interface{})))
		} else {
			log.Printf("⚠️ Vault not found for building: %s", vaultBuilding)
		}
	}

	response := &types.GameActionResponse{
		Success: true,
		Message: "", // Suppressed - no need to show success message for moves
	}
	if len(delta) > 0 {
		response.Delta = delta
	}

	return response, nil
}

// HandleStackItemAction stacks items together
func HandleStackItemAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	itemID, _ := params["item_id"].(string)
	fromSlot := int(params["from_slot"].(float64))
	toSlot := int(params["to_slot"].(float64))
	fromSlotType, _ := params["from_slot_type"].(string)
	toSlotType, _ := params["to_slot_type"].(string)

	// Get source slots
	var fromSlots []interface{}
	switch fromSlotType {
	case "general":
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid general slots")
		}
		fromSlots = generalSlots
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		contents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack")
		}
		fromSlots = contents
	default:
		return nil, fmt.Errorf("invalid source slot type: %s", fromSlotType)
	}

	// Get destination slots
	var toSlots []interface{}
	switch toSlotType {
	case "general":
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid general slots")
		}
		toSlots = generalSlots
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		contents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack")
		}
		toSlots = contents
	default:
		return nil, fmt.Errorf("invalid destination slot type: %s", toSlotType)
	}

	// Get source and destination items
	fromSlotMap, ok := fromSlots[fromSlot].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid source slot data")
	}

	toSlotMap, ok := toSlots[toSlot].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid destination slot data")
	}

	// Verify both slots have the same item
	if fromSlotMap["item"] != itemID || toSlotMap["item"] != itemID {
		return nil, fmt.Errorf("items don't match for stacking")
	}

	// Get item data to check max stack
	itemData, err := db.GetItemByID(itemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}

	// Parse max stack from properties
	maxStack := 1
	if itemData.Properties != "" {
		var properties map[string]interface{}
		if err := json.Unmarshal([]byte(itemData.Properties), &properties); err == nil {
			if val, ok := properties["stack"].(float64); ok {
				maxStack = int(val)
			}
		}
	}

	// Get quantities - handle both int and float64 types
	var fromQty, toQty int

	// Convert from slot quantity (handle both int and float64)
	switch v := fromSlotMap["quantity"].(type) {
	case float64:
		fromQty = int(v)
	case int:
		fromQty = v
	default:
		fromQty = 0
	}

	// Convert to slot quantity (handle both int and float64)
	switch v := toSlotMap["quantity"].(type) {
	case float64:
		toQty = int(v)
	case int:
		toQty = v
	default:
		toQty = 0
	}

	log.Printf("📊 Stack quantities - From slot: qty=%v (type=%T), To slot: qty=%v (type=%T), max stack: %d",
		fromSlotMap["quantity"], fromSlotMap["quantity"],
		toSlotMap["quantity"], toSlotMap["quantity"], maxStack)
	log.Printf("📊 Converted - fromQty=%d, toQty=%d", fromQty, toQty)

	// Check if destination is already at max
	if toQty >= maxStack {
		return nil, fmt.Errorf("destination stack is full (max %d)", maxStack)
	}

	// Calculate how much can be added
	canAdd := maxStack - toQty
	if canAdd > fromQty {
		canAdd = fromQty
	}

	// Update destination slot
	toSlotMap["quantity"] = toQty + canAdd

	// Update or clear source slot
	remaining := fromQty - canAdd
	if remaining > 0 {
		fromSlotMap["quantity"] = remaining
		log.Printf("✅ Stacked %s: moved %d from %d to %d (now %d total, %d remaining in source)", itemID, canAdd, fromQty, toQty, toQty+canAdd, remaining)
	} else {
		fromSlotMap["item"] = nil
		fromSlotMap["quantity"] = 0
		log.Printf("✅ Stacked %s: %d + %d = %d (source cleared)", itemID, fromQty, toQty, toQty+canAdd)
	}

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Stacked items (%d total)", toQty+canAdd),
	}, nil
}

// HandleSplitItemAction splits a stack into two stacks
func HandleSplitItemAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	itemID, _ := params["item_id"].(string)
	fromSlot := int(params["from_slot"].(float64))
	toSlot := int(params["to_slot"].(float64))
	fromSlotType, _ := params["from_slot_type"].(string)
	toSlotType, _ := params["to_slot_type"].(string)
	splitQuantity := int(params["quantity"].(float64))

	log.Printf("✂️ Splitting %s: %d from %s[%d] to %s[%d]", itemID, splitQuantity, fromSlotType, fromSlot, toSlotType, toSlot)

	// Get source slot
	var fromSlots []interface{}
	switch fromSlotType {
	case "general":
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid general slots")
		}
		fromSlots = generalSlots
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		contents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack")
		}
		fromSlots = contents
	default:
		return nil, fmt.Errorf("invalid source slot type: %s", fromSlotType)
	}

	// Get destination slot
	var toSlots []interface{}
	switch toSlotType {
	case "general":
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid general slots")
		}
		toSlots = generalSlots
	case "inventory":
		gearSlots, _ := state.Inventory["gear_slots"].(map[string]interface{})
		bag, _ := gearSlots["bag"].(map[string]interface{})
		contents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack")
		}
		toSlots = contents
	default:
		return nil, fmt.Errorf("invalid destination slot type: %s", toSlotType)
	}

	// Validate slots exist
	if fromSlot < 0 || fromSlot >= len(fromSlots) {
		return nil, fmt.Errorf("invalid from slot: %d", fromSlot)
	}
	if toSlot < 0 || toSlot >= len(toSlots) {
		return nil, fmt.Errorf("invalid to slot: %d", toSlot)
	}

	// Get source item
	fromSlotMap, ok := fromSlots[fromSlot].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid source slot data")
	}

	// Verify item ID matches
	if fromSlotMap["item"] != itemID {
		return nil, fmt.Errorf("item mismatch in source slot")
	}

	// Get current quantity (handle both int and float64 types)
	var currentQty int
	switch v := fromSlotMap["quantity"].(type) {
	case float64:
		currentQty = int(v)
	case int:
		currentQty = v
	default:
		return nil, fmt.Errorf("invalid quantity in source slot")
	}

	// Validate split quantity
	if splitQuantity <= 0 || splitQuantity >= currentQty {
		return nil, fmt.Errorf("invalid split quantity: %d (current: %d)", splitQuantity, currentQty)
	}

	// Check destination slot is empty
	toSlotMap, ok := toSlots[toSlot].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid destination slot data")
	}
	if toSlotMap["item"] != nil && toSlotMap["item"] != "" {
		return nil, fmt.Errorf("destination slot is not empty")
	}

	// Perform split
	remainingQty := currentQty - splitQuantity

	// Update source slot (store as int, not float64)
	fromSlotMap["quantity"] = remainingQty

	// Update destination slot (store as int, not float64)
	toSlotMap["item"] = itemID
	toSlotMap["quantity"] = splitQuantity
	toSlotMap["slot"] = toSlot

	log.Printf("✅ Split complete: %s (%d remaining in slot %d, %d in new slot %d)", itemID, remainingQty, fromSlot, splitQuantity, toSlot)

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Split %d items into new stack", splitQuantity),
	}, nil
}

// HandleAddItemAction adds an item to inventory
func HandleAddItemAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	itemID, _ := params["item_id"].(string)
	quantity := 1
	if q, ok := params["quantity"].(float64); ok {
		quantity = int(q)
	}

	log.Printf("➕ Adding %dx %s to inventory", quantity, itemID)

	// Try general slots first
	generalSlots, ok := state.Inventory["general_slots"].([]interface{})
	if ok {
		for i, slotData := range generalSlots {
			slotMap, ok := slotData.(map[string]interface{})
			if !ok {
				continue
			}

			// Check if slot is empty
			if slotMap["item"] == nil || slotMap["item"] == "" {
				// Add item to this slot (ensure quantity is int)
				slotMap["item"] = itemID
				slotMap["quantity"] = int(quantity)
				log.Printf("✅ Added %dx %s to general_slots[%d] (type: %T)", quantity, itemID, i, slotMap["quantity"])

				return &types.GameActionResponse{
					Success: true,
					Message: fmt.Sprintf("Added %dx %s", quantity, itemID),
				}, nil
			}
		}
	}

	// Try backpack if general slots are full
	gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
	if ok {
		bag, ok := gearSlots["bag"].(map[string]interface{})
		if ok {
			backpack, ok := bag["contents"].([]interface{})
			if ok {
				for i, slotData := range backpack {
					slotMap, ok := slotData.(map[string]interface{})
					if !ok {
						continue
					}

					// Check if slot is empty
					if slotMap["item"] == nil || slotMap["item"] == "" {
						// Add item to this slot (ensure quantity is int)
						slotMap["item"] = itemID
						slotMap["quantity"] = int(quantity)
						log.Printf("✅ Added %dx %s to backpack[%d] (type: %T)", quantity, itemID, i, slotMap["quantity"])

						return &types.GameActionResponse{
							Success: true,
							Message: fmt.Sprintf("Added %dx %s", quantity, itemID),
						}, nil
					}
				}
			}
		}
	}

	return nil, fmt.Errorf("inventory is full")
}

package inventory

import (
	"encoding/json"
	"fmt"
	"log"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// GetEquippedItemID returns the item ID in a named gear slot, or "" if empty.
// slot is the key used in gear_slots (e.g. "mainhand", "chest", "offhand").
func GetEquippedItemID(inventory map[string]interface{}, slot string) string {
	gearSlots, ok := inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return ""
	}
	slotData, exists := gearSlots[slot]
	if !exists || slotData == nil {
		return ""
	}
	slotMap, ok := slotData.(map[string]interface{})
	if !ok {
		return ""
	}
	itemID, _ := slotMap["item"].(string)
	return itemID
}

// HandleEquipItemAction equips an item from inventory to an equipment slot
func HandleEquipItemAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	itemID, ok := params["item_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid item_id parameter")
	}

	// Get from_slot as int (can come as float64 from JSON)
	var fromSlot int
	if fs, ok := params["from_slot"].(float64); ok {
		fromSlot = int(fs)
	} else if fs, ok := params["from_slot"].(int); ok {
		fromSlot = fs
	} else {
		return nil, fmt.Errorf("missing or invalid from_slot parameter")
	}

	fromSlotType, _ := params["from_slot_type"].(string)
	if fromSlotType == "" {
		fromSlotType = "general" // Default
	}

	// Equipment slot can be provided or auto-determined
	equipSlot, _ := params["equipment_slot"].(string)
	isTwoHanded := false

	log.Printf("⚔️ Equip action: %s from %s[%d] to equipment slot", itemID, fromSlotType, fromSlot)

	// Get equipment slots
	gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		gearSlots = make(map[string]interface{})
		state.Inventory["gear_slots"] = gearSlots
	}

	// If no equipment slot specified, determine from item properties
	if equipSlot == "" {
		database := db.GetDB()
		if database == nil {
			return nil, fmt.Errorf("database not available")
		}

		var propertiesJSON string
		var tagsJSON string
		var itemType string
		err := database.QueryRow("SELECT properties, tags, item_type FROM items WHERE id = ?", itemID).Scan(&propertiesJSON, &tagsJSON, &itemType)
		if err != nil {
			return nil, fmt.Errorf("item '%s' not found in database: %v", itemID, err)
		}

		var properties map[string]interface{}
		if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
			return nil, fmt.Errorf("failed to parse item properties")
		}

		// Check for two-handed
		var tags []interface{}
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err == nil {
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok && tagStr == "two-handed" {
					isTwoHanded = true
					break
				}
			}
		}

		// Determine equipment slot
		if gearSlotProp, ok := properties["gear_slot"].(string); ok {
			switch gearSlotProp {
			case "hands":
				if itemType == "Shield" {
					equipSlot = "offhand"
				} else {
					mainhandSlot := gearSlots["mainhand"]
					offhandSlot := gearSlots["offhand"]

					rightOccupied := false
					leftOccupied := false

					if rightMap, ok := mainhandSlot.(map[string]interface{}); ok {
						if rightMap["item"] != nil && rightMap["item"] != "" {
							rightOccupied = true
						}
					}
					if leftMap, ok := offhandSlot.(map[string]interface{}); ok {
						if leftMap["item"] != nil && leftMap["item"] != "" {
							leftOccupied = true
						}
					}

					if !rightOccupied {
						equipSlot = "mainhand"
					} else if !leftOccupied {
						equipSlot = "offhand"
					} else {
						equipSlot = "mainhand"
					}
				}
			case "armor", "body":
				equipSlot = "armor"
			case "neck", "necklace":
				equipSlot = "necklace"
			case "finger", "ring":
				equipSlot = "ring"
			case "ammunition", "ammo":
				equipSlot = "ammo"
			case "clothes", "clothing":
				equipSlot = "clothes"
			case "bag", "backpack":
				equipSlot = "bag"
			case "mainhand":
				equipSlot = "mainhand"
			case "offhand":
				equipSlot = "offhand"
			default:
				equipSlot = gearSlotProp
			}
		} else {
			return nil, fmt.Errorf("item does not have a gear_slot property")
		}
	}

	log.Printf("⚔️ Equipping to slot: %s (two-handed: %v)", equipSlot, isTwoHanded)

	// Handle two-handed weapons - unequip both hands
	var itemsToUnequip []map[string]interface{}
	if isTwoHanded {
		if rightMap, ok := gearSlots["mainhand"].(map[string]interface{}); ok {
			if rightMap["item"] != nil && rightMap["item"] != "" {
				itemsToUnequip = append(itemsToUnequip, map[string]interface{}{
					"item":     rightMap["item"],
					"quantity": rightMap["quantity"],
					"from":     "mainhand",
				})
			}
		}
		if leftMap, ok := gearSlots["offhand"].(map[string]interface{}); ok {
			if leftMap["item"] != nil && leftMap["item"] != "" {
				itemsToUnequip = append(itemsToUnequip, map[string]interface{}{
					"item":     leftMap["item"],
					"quantity": leftMap["quantity"],
					"from":     "offhand",
				})
			}
		}
	} else {
		if existing := gearSlots[equipSlot]; existing != nil {
			if existingMap, ok := existing.(map[string]interface{}); ok {
				if existingMap["item"] != nil && existingMap["item"] != "" {
					existingItemID := existingMap["item"].(string)

					itemsToUnequip = append(itemsToUnequip, map[string]interface{}{
						"item":     existingItemID,
						"quantity": existingMap["quantity"],
						"from":     equipSlot,
					})

					// Check if existing item is two-handed
					log.Printf("🔍 Checking if existing item '%s' is two-handed", existingItemID)
					database := db.GetDB()
					if database != nil {
						var tagsJSON string
						err := database.QueryRow("SELECT tags FROM items WHERE id = ?", existingItemID).Scan(&tagsJSON)
						if err == nil {
							var tags []interface{}
							if err := json.Unmarshal([]byte(tagsJSON), &tags); err == nil {
								for _, tag := range tags {
									if tagStr, ok := tag.(string); ok && tagStr == "two-handed" {
										var otherSlot string
										switch equipSlot {
										case "mainhand":
											otherSlot = "offhand"
										case "offhand":
											otherSlot = "mainhand"
										}

										if otherSlot != "" {
											if otherHand, ok := gearSlots[otherSlot].(map[string]interface{}); ok {
												if otherHand["item"] == existingItemID {
													log.Printf("🗡️ Existing two-handed weapon detected - also clearing %s", otherSlot)
													gearSlots[otherSlot] = map[string]interface{}{
														"item":     nil,
														"quantity": 0,
													}
												}
											}
										}
										break
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Find item in inventory
	var itemData map[string]interface{}
	var sourceInventory []interface{}
	var sourceArrayIndex int

	if fromSlotType == "general" {
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid general_slots structure")
		}
		sourceInventory = generalSlots

		if fromSlot < 0 || fromSlot >= len(sourceInventory) {
			return nil, fmt.Errorf("invalid source slot")
		}

		itemMap, ok := sourceInventory[fromSlot].(map[string]interface{})
		if !ok || itemMap["item"] != itemID {
			return nil, fmt.Errorf("item not found in specified slot")
		}
		itemData = itemMap
		sourceArrayIndex = fromSlot

	} else if fromSlotType == "inventory" {
		bag, ok := gearSlots["bag"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("no backpack found")
		}
		backpackContents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid backpack contents")
		}
		sourceInventory = backpackContents

		found := false
		for i, slotData := range sourceInventory {
			if slotMap, ok := slotData.(map[string]interface{}); ok {
				var slotNum int
				if sn, ok := slotMap["slot"].(float64); ok {
					slotNum = int(sn)
				} else if sn, ok := slotMap["slot"].(int); ok {
					slotNum = sn
				}
				if slotNum == fromSlot && slotMap["item"] == itemID {
					itemData = slotMap
					sourceArrayIndex = i
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("item not found in specified slot")
		}
	} else {
		return nil, fmt.Errorf("invalid source slot type")
	}

	// Add unequipped items back to inventory (swapped items)
	for i, unequipData := range itemsToUnequip {
		var targetSlot int
		var targetInventory []interface{}

		if i == 0 {
			targetSlot = sourceArrayIndex
			targetInventory = sourceInventory
		} else {
			foundEmpty := false
			for j, slot := range sourceInventory {
				if slotMap, ok := slot.(map[string]interface{}); ok {
					if slotMap["item"] == nil || slotMap["item"] == "" {
						targetSlot = j
						targetInventory = sourceInventory
						foundEmpty = true
						break
					}
				}
			}

			if !foundEmpty {
				if fromSlotType == "general" {
					bag, ok := gearSlots["bag"].(map[string]interface{})
					if ok {
						backpackContents, ok := bag["contents"].([]interface{})
						if ok {
							for j := 0; j < 20; j++ {
								if j >= len(backpackContents) {
									backpackContents = append(backpackContents, map[string]interface{}{
										"item":     nil,
										"quantity": 0,
										"slot":     j,
									})
									bag["contents"] = backpackContents
								}
								if slotMap, ok := backpackContents[j].(map[string]interface{}); ok {
									if slotMap["item"] == nil || slotMap["item"] == "" {
										targetSlot = j
										targetInventory = backpackContents
										foundEmpty = true
										break
									}
								}
							}
						}
					}
				} else {
					generalSlots, ok := state.Inventory["general_slots"].([]interface{})
					if ok {
						for j := 0; j < 4; j++ {
							if j >= len(generalSlots) {
								generalSlots = append(generalSlots, map[string]interface{}{
									"item":     nil,
									"quantity": 0,
									"slot":     j,
								})
								state.Inventory["general_slots"] = generalSlots
							}
							if slotMap, ok := generalSlots[j].(map[string]interface{}); ok {
								if slotMap["item"] == nil || slotMap["item"] == "" {
									targetSlot = j
									targetInventory = generalSlots
									foundEmpty = true
									break
								}
							}
						}
					}
				}

				if !foundEmpty {
					return nil, fmt.Errorf("inventory is full, cannot unequip both items")
				}
			}
		}

		targetInventory[targetSlot] = map[string]interface{}{
			"item":     unequipData["item"],
			"quantity": unequipData["quantity"],
			"slot":     targetSlot,
		}

		slotName := unequipData["from"].(string)
		gearSlots[slotName] = map[string]interface{}{
			"item":     nil,
			"quantity": 0,
		}

		log.Printf("✅ Unequipped %s from %s to inventory slot %d", unequipData["item"], slotName, targetSlot)
	}

	// Move item to equipment slot
	if isTwoHanded {
		gearSlots["mainhand"] = map[string]interface{}{
			"item":     itemData["item"],
			"quantity": itemData["quantity"],
		}
		gearSlots["offhand"] = map[string]interface{}{
			"item":     itemData["item"],
			"quantity": itemData["quantity"],
		}
		log.Printf("✅ Equipped two-handed %s to both hands", itemData["item"])
	} else {
		equippedItem := map[string]interface{}{
			"item":     itemData["item"],
			"quantity": itemData["quantity"],
		}

		// Preserve a container's own contents when equipping it (backpack → bag,
		// quiver → ammo, etc.). Without this a filled container dropped
		// everything inside on equip.
		if contents, ok := itemData["contents"].([]interface{}); ok {
			equippedItem["contents"] = contents
			log.Printf("📦 Preserving container contents on equip (%d slots)", len(contents))
		} else if equipSlot == "bag" {
			equippedItem["contents"] = make([]interface{}, 0, 20)
			log.Printf("📦 Initializing empty bag contents")
		}

		gearSlots[equipSlot] = equippedItem
	}

	// Empty the source slot if no swap occurred
	if len(itemsToUnequip) == 0 {
		sourceInventory[sourceArrayIndex] = map[string]interface{}{
			"item":     nil,
			"quantity": 0,
			"slot":     fromSlot,
		}
	}

	// Save back to correct location
	if fromSlotType == "general" {
		state.Inventory["general_slots"] = sourceInventory
	} else {
		bag := gearSlots["bag"].(map[string]interface{})
		bag["contents"] = sourceInventory
	}

	if isTwoHanded {
		log.Printf("✅ Equipped %s to both hands (swapped %d items)", itemID, len(itemsToUnequip))
	} else {
		log.Printf("✅ Equipped %s to %s (swapped %d items)", itemID, equipSlot, len(itemsToUnequip))
	}

	// Apply effects_when_worn
	database := db.GetDB()
	if database != nil {
		var propertiesJSON string
		err := database.QueryRow("SELECT properties FROM items WHERE id = ?", itemID).Scan(&propertiesJSON)
		if err == nil {
			var properties map[string]interface{}
			if err := json.Unmarshal([]byte(propertiesJSON), &properties); err == nil {
				if effectsWhenWorn, ok := properties["effects_when_worn"].([]interface{}); ok {
					for _, effectID := range effectsWhenWorn {
						if effectIDStr, ok := effectID.(string); ok {
							// Apply effect silently (no message)
							if err := effects.ApplyEffect(state, effectIDStr); err != nil {
								log.Printf("⚠️ Failed to apply equipment effect '%s': %v", effectIDStr, err)
							} else {
								log.Printf("⚙️ Applied equipment effect: %s from %s", effectIDStr, itemID)
							}
						}
					}
				}
			}
		}
	}

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Equipped %s", itemID),
	}, nil
}

// HandleUnequipItemAction unequips an item from equipment to inventory
func HandleUnequipItemAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	equipSlot, ok := params["equipment_slot"].(string)
	if !ok || equipSlot == "" {
		equipSlot, ok = params["from_equip"].(string)
		if !ok || equipSlot == "" {
			return nil, fmt.Errorf("missing or invalid equipment_slot/from_equip parameter")
		}
	}

	log.Printf("🛡️ Unequip action from equipment slot: %s", equipSlot)

	gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid equipment structure")
	}

	itemData, exists := gearSlots[equipSlot]
	if !exists || itemData == nil {
		return nil, fmt.Errorf("no item in equipment slot '%s'", equipSlot)
	}

	itemMap, ok := itemData.(map[string]interface{})
	if !ok || itemMap["item"] == nil || itemMap["item"] == "" {
		return nil, fmt.Errorf("no item in equipment slot '%s'", equipSlot)
	}

	itemID := itemMap["item"].(string)
	quantity := 1
	if qty, ok := itemMap["quantity"].(float64); ok {
		quantity = int(qty)
	} else if qty, ok := itemMap["quantity"].(int); ok {
		quantity = qty
	}

	log.Printf("🛡️ Unequipping: %s (quantity: %d)", itemID, quantity)

	// Containers (backpack, quiver, …) may only be unequipped into general
	// slots — they can't live in the backpack — and must keep their contents.
	itemIsContainer := false
	if _, ok := itemMap["contents"]; ok {
		itemIsContainer = true
	} else if database := db.GetDB(); database != nil {
		var tagsJSON string
		if err := database.QueryRow("SELECT tags FROM items WHERE id = ?", itemID).Scan(&tagsJSON); err == nil {
			var tags []interface{}
			if json.Unmarshal([]byte(tagsJSON), &tags) == nil {
				for _, t := range tags {
					if ts, ok := t.(string); ok && ts == "container" {
						itemIsContainer = true
						break
					}
				}
			}
		}
	}

	emptySlotIndex := -1
	emptySlotType := ""

	if equipSlot == "bag" || itemIsContainer {
		log.Printf("🎒 Unequipping container %s → general slots only", itemID)

		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			generalSlots = make([]interface{}, 0, 4)
			state.Inventory["general_slots"] = generalSlots
		}

		for i := 0; i < 4; i++ {
			if i >= len(generalSlots) {
				generalSlots = append(generalSlots, map[string]interface{}{
					"item":     nil,
					"quantity": 0,
					"slot":     i,
				})
				state.Inventory["general_slots"] = generalSlots
			}

			if generalSlots[i] == nil {
				emptySlotIndex = i
				emptySlotType = "general"
				break
			}

			if slotMap, ok := generalSlots[i].(map[string]interface{}); ok {
				if slotMap["item"] == nil || slotMap["item"] == "" {
					emptySlotIndex = i
					emptySlotType = "general"
					break
				}
			}
		}

		if emptySlotIndex == -1 {
			return &types.GameActionResponse{
				Success: false,
				Error:   "There isn't enough room in your general slots to take this off",
				Color:   "red",
			}, nil
		}

		log.Printf("✅ Found empty general slot at index %d for bag", emptySlotIndex)
	} else {
		bag, ok := gearSlots["bag"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("no backpack found")
		}

		backpackContents, ok := bag["contents"].([]interface{})
		if !ok {
			backpackContents = make([]interface{}, 0, 20)
			bag["contents"] = backpackContents
		}

		for i := 0; i < 20; i++ {
			if i >= len(backpackContents) {
				backpackContents = append(backpackContents, map[string]interface{}{
					"item":     nil,
					"quantity": 0,
					"slot":     i,
				})
				bag["contents"] = backpackContents
			}

			if backpackContents[i] == nil {
				emptySlotIndex = i
				emptySlotType = "inventory"
				break
			}

			if slotMap, ok := backpackContents[i].(map[string]interface{}); ok {
				if slotMap["item"] == nil || slotMap["item"] == "" {
					emptySlotIndex = i
					emptySlotType = "inventory"
					break
				}
			}
		}
	}

	if emptySlotIndex == -1 && equipSlot != "bag" && !itemIsContainer {
		generalSlots, ok := state.Inventory["general_slots"].([]interface{})
		if !ok {
			generalSlots = make([]interface{}, 0, 4)
			state.Inventory["general_slots"] = generalSlots
		}

		for i := 0; i < 4; i++ {
			if i >= len(generalSlots) {
				generalSlots = append(generalSlots, map[string]interface{}{
					"item":     nil,
					"quantity": 0,
					"slot":     i,
				})
				state.Inventory["general_slots"] = generalSlots
			}

			if generalSlots[i] == nil {
				emptySlotIndex = i
				emptySlotType = "general"
				break
			}

			if slotMap, ok := generalSlots[i].(map[string]interface{}); ok {
				if slotMap["item"] == nil || slotMap["item"] == "" {
					emptySlotIndex = i
					emptySlotType = "general"
					break
				}
			}
		}
	}

	if emptySlotIndex == -1 {
		return nil, fmt.Errorf("your inventory is full")
	}

	newItem := map[string]interface{}{
		"item":     itemID,
		"quantity": quantity,
		"slot":     emptySlotIndex,
	}

	if itemIsContainer {
		if contents, ok := itemMap["contents"].([]interface{}); ok {
			newItem["contents"] = contents
			log.Printf("📦 Preserving container contents on unequip (%d slots)", len(contents))
		}
	}

	if emptySlotType == "inventory" {
		bag := gearSlots["bag"].(map[string]interface{})
		backpackContents := bag["contents"].([]interface{})
		backpackContents[emptySlotIndex] = newItem
		bag["contents"] = backpackContents
		log.Printf("✅ Moved to backpack slot %d", emptySlotIndex)
	} else {
		generalSlots := state.Inventory["general_slots"].([]interface{})
		generalSlots[emptySlotIndex] = newItem
		state.Inventory["general_slots"] = generalSlots
		log.Printf("✅ Moved to general slot %d", emptySlotIndex)
	}

	// Remove effects_when_worn before unequipping
	database := db.GetDB()
	if database != nil {
		var propertiesJSON string
		err := database.QueryRow("SELECT properties FROM items WHERE id = ?", itemID).Scan(&propertiesJSON)
		if err == nil {
			var properties map[string]interface{}
			if err := json.Unmarshal([]byte(propertiesJSON), &properties); err == nil {
				if effectsWhenWorn, ok := properties["effects_when_worn"].([]interface{}); ok {
					for _, effectID := range effectsWhenWorn {
						if effectIDStr, ok := effectID.(string); ok {
							// Remove effect
							effects.RemoveEffect(state, effectIDStr)
							log.Printf("⚙️ Removed equipment effect: %s from %s", effectIDStr, itemID)
						}
					}
				}
			}
		}
	}

	gearSlots[equipSlot] = map[string]interface{}{
		"item":     nil,
		"quantity": 0,
	}

	// Check if this is a two-handed weapon
	switch equipSlot {
	case "mainhand":
		if offhandSlot, ok := gearSlots["offhand"].(map[string]interface{}); ok {
			if offhandSlot["item"] == itemID {
				gearSlots["offhand"] = map[string]interface{}{
					"item":     nil,
					"quantity": 0,
				}
				log.Printf("✅ Also cleared offhand (two-handed weapon)")
			}
		}
	case "offhand":
		if mainhandSlot, ok := gearSlots["mainhand"].(map[string]interface{}); ok {
			if mainhandSlot["item"] == itemID {
				gearSlots["mainhand"] = map[string]interface{}{
					"item":     nil,
					"quantity": 0,
				}
				log.Printf("✅ Also cleared mainhand (two-handed weapon)")
			}
		}
	}

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Unequipped %s", itemID),
	}, nil
}

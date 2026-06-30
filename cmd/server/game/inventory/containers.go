package inventory

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// HandleAddToContainerAction adds an item to a container (backpack, pouch, etc.)
func HandleAddToContainerAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	// Extract parameters
	itemID, _ := params["item_id"].(string)
	fromSlot := -1
	if fs, ok := params["from_slot"].(float64); ok {
		fromSlot = int(fs)
	}
	fromSlotType, _ := params["from_slot_type"].(string)
	containerSlot := -1
	if cs, ok := params["container_slot"].(float64); ok {
		containerSlot = int(cs)
	}
	toSlotType, _ := params["container_slot_type"].(string)
	toContainerSlot := -1
	if tcs, ok := params["to_container_slot"].(float64); ok {
		toContainerSlot = int(tcs)
	}

	log.Printf("📦 Add to container: %s from %s[%d] to container at %s[%d], container slot %d",
		itemID, fromSlotType, fromSlot, toSlotType, containerSlot, toContainerSlot)

	var containerSlotMap map[string]interface{}
	var containerIndex int

	// Get general slots - needed for later source inventory lookup
	generalSlots, ok := state.Inventory["general_slots"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("general slots not found")
	}

	// Get the container based on toSlotType
	if toSlotType == "equipment" || toSlotType == "" {
		// Container is in equipment slot (backpack)
		gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("equipment slots not found")
		}
		if bag, ok := gearSlots["bag"].(map[string]interface{}); ok {
			if bag["item"] != nil && bag["item"] != "" {
				containerSlotMap = bag
				log.Printf("📦 Found container in equipped bag slot")
			}
		}
	} else if toSlotType == "general" {
		// Container is in general slots
		for i, slot := range generalSlots {
			if slotMap, ok := slot.(map[string]interface{}); ok {
				if slotNum, ok := slotMap["slot"].(float64); ok && int(slotNum) == containerSlot {
					containerSlotMap = slotMap
					containerIndex = i
					log.Printf("📦 Found container in general slot %d", containerSlot)
					break
				}
			}
		}
	}

	if containerSlotMap == nil {
		return nil, fmt.Errorf("container not found at %s[%d]", toSlotType, containerSlot)
	}

	containerID := containerSlotMap["item"].(string)

	// Get container properties from database
	database := db.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}

	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM items WHERE id = ?", containerID).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("container item not found in database")
	}

	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
		return nil, fmt.Errorf("failed to parse container properties")
	}

	// Check if item is a container
	tags, ok := properties["tags"].([]interface{})
	isContainer := false
	if ok {
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok && tagStr == "container" {
				isContainer = true
				break
			}
		}
	}

	if !isContainer {
		return nil, fmt.Errorf("item is not a container")
	}

	// Get container slots limit
	containerSlots := 10 // default
	if val, ok := properties["container_slots"].(float64); ok {
		containerSlots = int(val)
	}

	// Get allowed types. The item JSON may store this as a single string or an
	// array of strings (e.g. component-pouch: ["Spell Component"]). Empty or
	// "any" means unrestricted. Reading it as a string only — as before — meant
	// the array form failed the assertion and silently degraded to "any",
	// letting anything into a type-restricted container.
	var allowedTypes []string
	switch v := properties["allowed_types"].(type) {
	case string:
		if v != "" && !strings.EqualFold(v, "any") {
			allowedTypes = append(allowedTypes, v)
		}
	case []interface{}:
		for _, t := range v {
			if ts, ok := t.(string); ok && ts != "" && !strings.EqualFold(ts, "any") {
				allowedTypes = append(allowedTypes, ts)
			}
		}
	}

	// Get container contents
	contents, ok := containerSlotMap["contents"].([]interface{})
	if !ok {
		contents = make([]interface{}, 0)
	}

	// Ensure contents array has enough slots
	for len(contents) < containerSlots {
		contents = append(contents, nil)
	}

	// Check if container is full (count non-null items)
	usedSlots := 0
	for _, item := range contents {
		if item != nil {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemMap["item"] != nil {
					usedSlots++
				}
			}
		}
	}

	if usedSlots >= containerSlots {
		return nil, fmt.Errorf("container is full")
	}

	// Get item properties to check if it's a container
	var itemPropertiesJSON string
	err = database.QueryRow("SELECT properties FROM items WHERE id = ?", itemID).Scan(&itemPropertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("item not found in database")
	}

	var itemProperties map[string]interface{}
	if err := json.Unmarshal([]byte(itemPropertiesJSON), &itemProperties); err != nil {
		return nil, fmt.Errorf("failed to parse item properties")
	}

	// Check if item being added is itself a container - containers cannot go in containers
	if itemTags, ok := itemProperties["tags"].([]interface{}); ok {
		for _, tag := range itemTags {
			if tagStr, ok := tag.(string); ok && tagStr == "container" {
				return nil, fmt.Errorf("containers cannot be stored inside other containers")
			}
		}
	}

	// Validate the item against the container's allowed types (if restricted).
	if len(allowedTypes) > 0 {
		// Candidate type identifiers from the item: its tags, plus item_type and
		// type (the item JSON uses "type"; some rows also carry "item_type").
		var candidates []string
		if itemTags, ok := itemProperties["tags"].([]interface{}); ok {
			for _, tag := range itemTags {
				if tagStr, ok := tag.(string); ok {
					candidates = append(candidates, normalizeContainerType(tagStr))
				}
			}
		}
		for _, key := range []string{"item_type", "type"} {
			if itemType, ok := itemProperties[key].(string); ok && itemType != "" {
				candidates = append(candidates, normalizeContainerType(itemType))
			}
		}

		itemAllowed := false
		for _, allowed := range allowedTypes {
			na := normalizeContainerType(allowed)
			for _, c := range candidates {
				if c == na {
					itemAllowed = true
					break
				}
			}
			if itemAllowed {
				break
			}
		}

		if !itemAllowed {
			return nil, fmt.Errorf("this item cannot be stored in this container (requires %s)", strings.Join(allowedTypes, ", "))
		}
	}

	// Find the item in source inventory
	var sourceInventory []interface{}
	var sourceItem map[string]interface{}
	var sourceIndex int

	switch fromSlotType {
	case "general":
		sourceInventory = generalSlots
	case "inventory":
		gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("gear slots not found")
		}
		bag, ok := gearSlots["bag"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("backpack not found")
		}
		backpackContents, ok := bag["contents"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("backpack contents not found")
		}
		sourceInventory = backpackContents
	default:
		return nil, fmt.Errorf("invalid source slot type")
	}

	// Find source item by matching slot index
	for i, slot := range sourceInventory {
		if slotMap, ok := slot.(map[string]interface{}); ok {
			if slotNum, ok := slotMap["slot"].(float64); ok && int(slotNum) == fromSlot {
				if slotMap["item"] == itemID {
					sourceItem = slotMap
					sourceIndex = i
					break
				}
			}
		}
	}

	if sourceItem == nil {
		return nil, fmt.Errorf("item not found in source inventory")
	}

	// Get item quantity
	quantity := 1
	if qty, ok := sourceItem["quantity"].(float64); ok {
		quantity = int(qty)
	} else if qty, ok := sourceItem["quantity"].(int); ok {
		quantity = qty
	}

	// Get item name for message
	var itemName string
	err = database.QueryRow("SELECT name FROM items WHERE id = ?", itemID).Scan(&itemName)
	if err != nil {
		itemName = itemID
	}

	// Create item entry for container
	containerItem := map[string]interface{}{
		"item":     itemID,
		"quantity": quantity,
	}

	// Find first empty slot in container
	placedAt := -1
	for i := 0; i < containerSlots; i++ {
		if contents[i] == nil || (contents[i] != nil && contents[i].(map[string]interface{})["item"] == nil) {
			contents[i] = containerItem
			placedAt = i
			break
		}
	}

	if placedAt == -1 {
		return nil, fmt.Errorf("container is full")
	}

	// Remove item from source (set to empty slot)
	sourceInventory[sourceIndex] = map[string]interface{}{
		"item":     nil,
		"quantity": 0,
		"slot":     fromSlot,
	}

	// Update container contents based on where it is
	containerSlotMap["contents"] = contents

	if toSlotType == "general" {
		generalSlots[containerIndex] = containerSlotMap
		state.Inventory["general_slots"] = generalSlots
	}

	// Save source inventory changes
	if fromSlotType == "inventory" {
		gearSlots := state.Inventory["gear_slots"].(map[string]interface{})
		bag := gearSlots["bag"].(map[string]interface{})
		bag["contents"] = sourceInventory
	}

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Added %dx %s to container", quantity, itemName),
		Color:   "green",
	}, nil
}

// HandleRemoveFromContainerAction removes an item from a container back to inventory
func HandleRemoveFromContainerAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	// Extract parameters
	itemID, _ := params["item_id"].(string)
	containerSlot := -1
	if cs, ok := params["container_slot"].(float64); ok {
		containerSlot = int(cs)
	}
	fromContainerSlot := -1
	if fcs, ok := params["from_container_slot"].(float64); ok {
		fromContainerSlot = int(fcs)
	}
	fromSlotType, _ := params["container_slot_type"].(string)

	// Use from_slot if from_container_slot not provided
	if fromContainerSlot < 0 {
		if fs, ok := params["from_slot"].(float64); ok {
			fromContainerSlot = int(fs)
		}
	}

	log.Printf("📤 Remove from container: slot %d from container at %s[%d]",
		fromContainerSlot, fromSlotType, containerSlot)

	var containerSlotMap map[string]interface{}
	var containerIndex int
	containerLocation := "" // "general" or "equipped"

	// Load general slots early
	generalSlots, ok := state.Inventory["general_slots"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("general slots not found")
	}

	// Use fromSlotType to determine where to look for the container
	if fromSlotType == "equipment" || fromSlotType == "" {
		gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
		if ok {
			if bag, ok := gearSlots["bag"].(map[string]interface{}); ok {
				if bag["item"] != nil && bag["item"] != "" {
					containerSlotMap = bag
					containerLocation = "equipped"
					log.Printf("📦 Found container in equipped bag slot")
				}
			}
		}
	} else if fromSlotType == "general" {
		log.Printf("📦 Searching %d general slots for container at slot %d", len(generalSlots), containerSlot)

		for i, slot := range generalSlots {
			if slotMap, ok := slot.(map[string]interface{}); ok {
				var slotNum int
				if sn, ok := slotMap["slot"].(float64); ok {
					slotNum = int(sn)
				} else if sn, ok := slotMap["slot"].(int); ok {
					slotNum = sn
				} else {
					slotNum = i
				}

				log.Printf("🔍 Slot %d (index %d): item=%v, hasContents=%v", slotNum, i, slotMap["item"], slotMap["contents"] != nil)

				if slotNum == containerSlot {
					if slotMap["item"] != nil && slotMap["item"] != "" {
						containerSlotMap = slotMap
						containerIndex = i
						containerLocation = "general"
						log.Printf("📦 Found container '%v' in general slot %d (array index %d)", slotMap["item"], containerSlot, i)
						break
					} else {
						log.Printf("⚠️ Slot %d is empty, not a container", slotNum)
					}
				}
			}
		}
	}

	if containerSlotMap == nil {
		return nil, fmt.Errorf("container not found")
	}

	// Get container contents
	contents, ok := containerSlotMap["contents"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("container has no contents")
	}

	if fromContainerSlot >= len(contents) {
		return nil, fmt.Errorf("invalid container slot")
	}

	containerItem, ok := contents[fromContainerSlot].(map[string]interface{})
	if !ok || containerItem["item"] == nil {
		return nil, fmt.Errorf("container slot is empty")
	}

	actualItemID := containerItem["item"].(string)
	_ = itemID // itemID from params is for validation if needed
	quantity := 1
	if qty, ok := containerItem["quantity"].(float64); ok {
		quantity = int(qty)
	} else if qty, ok := containerItem["quantity"].(int); ok {
		quantity = qty
	}

	// Try to find empty slot in general_slots first
	emptySlotFound := false

	for i, slot := range generalSlots {
		if slotMap, ok := slot.(map[string]interface{}); ok {
			if slotMap["item"] == nil {
				generalSlots[i] = map[string]interface{}{
					"item":     actualItemID,
					"quantity": quantity,
					"slot":     slotMap["slot"],
				}
				emptySlotFound = true
				break
			}
		}
	}

	// If no space in general slots, try backpack
	if !emptySlotFound {
		gearSlots, ok := state.Inventory["gear_slots"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("gear slots not found")
		}
		bag, ok := gearSlots["bag"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("backpack not found")
		}
		backpackContents, ok := bag["contents"].([]interface{})
		if !ok {
			backpackContents = make([]interface{}, 0)
		}

		// Extend backpack if needed
		for len(backpackContents) < 20 {
			backpackContents = append(backpackContents, map[string]interface{}{
				"item":     nil,
				"quantity": 0,
				"slot":     len(backpackContents),
			})
		}

		// Find empty backpack slot
		for i := 0; i < 20; i++ {
			if backpackContents[i] == nil || backpackContents[i].(map[string]interface{})["item"] == nil {
				backpackContents[i] = map[string]interface{}{
					"item":     actualItemID,
					"quantity": quantity,
					"slot":     i,
				}
				emptySlotFound = true
				break
			}
		}

		if emptySlotFound {
			bag["contents"] = backpackContents
		}
	}

	if !emptySlotFound {
		return nil, fmt.Errorf("inventory is full - cannot remove from container")
	}

	// Remove item from container
	contents[fromContainerSlot] = map[string]interface{}{
		"item":     nil,
		"quantity": 0,
		"slot":     fromContainerSlot,
	}

	// Update container in the correct location
	if containerLocation == "equipped" {
		containerSlotMap["contents"] = contents
		log.Printf("📦 Updated contents of equipped bag")
	} else {
		generalSlots[containerIndex].(map[string]interface{})["contents"] = contents
		state.Inventory["general_slots"] = generalSlots
		log.Printf("📦 Updated contents of container in general slot %d", containerIndex)
	}

	// Get item name for message
	database := db.GetDB()
	var itemName string
	if database != nil {
		err := database.QueryRow("SELECT name FROM items WHERE id = ?", actualItemID).Scan(&itemName)
		if err != nil {
			itemName = actualItemID
		}
	} else {
		itemName = actualItemID
	}

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Removed %dx %s from container", quantity, itemName),
		Color:   "green",
	}, nil
}

// normalizeContainerType canonicalizes a type/tag string for allowed_types
// comparison: lowercased, spaces and hyphens to underscores, trailing plural
// "s" trimmed. So the container's "Spell Component" and an item's
// "spell_component" tag (or "Spell Component" type) compare equal.
func normalizeContainerType(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return strings.TrimSuffix(s, "s")
}

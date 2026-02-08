package validation

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// CleanupResult holds the results of a cleanup operation
type CleanupResult struct {
	FilesProcessed int      `json:"files_processed"`
	FilesModified  int      `json:"files_modified"`
	Changes        []Change `json:"changes"`
}

// Change represents a change made to a file
type Change struct {
	File    string `json:"file"`
	Type    string `json:"type"` // "added", "removed", "fixed", "reordered"
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

// CleanupAllItems runs cleanup on all item files
func CleanupAllItems(dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{
		Changes: []Change{},
	}

	itemsPath := "game-data/items"

	err := filepath.WalkDir(itemsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			changes, modified := cleanupItemFile(path, dryRun)
			result.FilesProcessed++
			if modified {
				result.FilesModified++
			}
			result.Changes = append(result.Changes, changes...)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	log.Printf("Cleanup complete: %d files processed, %d modified", result.FilesProcessed, result.FilesModified)
	return result, nil
}

// cleanupItemFile cleans up a single item file
func cleanupItemFile(filePath string, dryRun bool) ([]Change, bool) {
	changes := []Change{}
	filename := filepath.Base(filePath)
	idFromFilename := strings.TrimSuffix(filename, ".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading %s: %v", filename, err)
		return changes, false
	}

	var item map[string]interface{}
	if err := json.Unmarshal(data, &item); err != nil {
		log.Printf("Error parsing %s: %v", filename, err)
		return changes, false
	}

	modified := false

	// STEP 0: NORMALIZE PROPERTY NAMES (fix casing issues)
	normalizedItem, nameChanges := normalizePropertyNames(item, filename)
	if len(nameChanges) > 0 {
		changes = append(changes, nameChanges...)
		item = normalizedItem
		modified = true
	}

	// 1. ADD MISSING REQUIRED FIELDS
	requiredDefaults := map[string]interface{}{
		"id":             idFromFilename,
		"name":           "NEEDS NAME",
		"description":    "NEEDS DESCRIPTION",
		"rarity":         "common",
		"price":          0,
		"weight":         1.0,
		"stack":          1,
		"type":           "Adventuring Gear",
		"image":          fmt.Sprintf("/res/img/items/%s.png", idFromFilename),
		"tags":           []interface{}{},
		"notes":          []interface{}{},
	}

	for field, defaultValue := range requiredDefaults {
		if _, exists := item[field]; !exists {
			item[field] = defaultValue
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   field,
				Message: fmt.Sprintf("Added missing required field '%s'", field),
			})
			modified = true
		} else if field == "description" {
			// Check for empty strings
			if val, ok := item[field].(string); ok && val == "" {
				item[field] = defaultValue
				changes = append(changes, Change{
					File:    filename,
					Type:    "fixed",
					Field:   field,
					Message: fmt.Sprintf("Fixed empty '%s' field", field),
				})
				modified = true
			}
		}
	}

	// 2. GET TAGS AND TYPE FOR CONDITIONAL CLEANUP
	tags := []string{}
	if tagsArray, ok := item["tags"].([]interface{}); ok {
		for _, tag := range tagsArray {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	itemType := ""
	if t, ok := item["type"].(string); ok {
		itemType = strings.ToLower(t)
	}

	// 3. REMOVE UNNECESSARY NULL/EMPTY FIELDS
	fieldsToCheckForRemoval := []string{"ac", "damage", "heal", "ammunition", "range", "range-long", "damage-type"}

	for _, field := range fieldsToCheckForRemoval {
		if val, exists := item[field]; exists {
			shouldRemove := false

			// Check if it's null or empty
			switch v := val.(type) {
			case nil:
				shouldRemove = true
			case string:
				if v == "" || v == "null" {
					shouldRemove = true
				}
			}

			// Don't remove if it's required for this item type
			if shouldRemove {
				if field == "ac" && strings.Contains(itemType, "armor") && !contains(tags, "pack") {
					shouldRemove = false // AC is required for armor
				}
				if field == "damage" && strings.Contains(itemType, "melee") {
					shouldRemove = false // Damage is required for melee weapons
				}
				if (field == "ammunition" || field == "range" || field == "range-long") && strings.Contains(itemType, "ranged") && !contains(tags, "thrown") {
					shouldRemove = false // These are required for ranged weapons
				}
			}

			if shouldRemove {
				delete(item, field)
				changes = append(changes, Change{
					File:    filename,
					Type:    "removed",
					Field:   field,
					Message: fmt.Sprintf("Removed unnecessary null/empty '%s' field", field),
				})
				modified = true
			}
		}
	}

	// 4. ADD CONDITIONAL FIELDS

	// If item has gear_slot, ensure it has equipment tag
	if gearSlot, hasGearSlot := item["gear_slot"]; hasGearSlot {
		if !contains(tags, "equipment") {
			// Add equipment tag instead of removing gear_slot
			tags = append(tags, "equipment")
			item["tags"] = interfaceSlice(tags)
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "tags",
				Message: "Added 'equipment' tag (item has gear_slot)",
			})
			modified = true
		}

		// Validate gear_slot value
		validGearSlots := map[string]bool{
			"hands": true, "mainhand": true, "offhand": true,
			"chest": true, "head": true, "legs": true,
			"gloves": true, "boots": true,
			"neck": true, "ring": true,
			"ammo": true, "bag": true,
		}
		if gearSlotStr, ok := gearSlot.(string); ok {
			if !validGearSlots[gearSlotStr] {
				// Invalid gear_slot - need manual fix
				changes = append(changes, Change{
					File:    filename,
					Type:    "fixed",
					Field:   "gear_slot",
					Message: fmt.Sprintf("⚠️ MANUAL FIX REQUIRED: Invalid gear_slot '%s' (must be: hands, mainhand, offhand, chest, head, legs, gloves, boots, neck, ring, ammo, bag)", gearSlotStr),
				})
			}
		}
	}

	// If equipment tag exists, ensure gear_slot exists
	if contains(tags, "equipment") {
		if _, exists := item["gear_slot"]; !exists {
			// Default to "hands" for weapons, "chest" for armor, "ammo" for ammunition
			defaultSlot := "hands"
			if strings.Contains(itemType, "armor") {
				defaultSlot = "chest"
			} else if contains(tags, "ammunition") || strings.Contains(itemType, "ammunition") {
				defaultSlot = "ammo"
			}

			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "gear_slot",
				Message: fmt.Sprintf("⚠️ MANUAL FIX REQUIRED: Added 'gear_slot' (defaulted to '%s', may need adjustment)", defaultSlot),
			})
			item["gear_slot"] = defaultSlot
			modified = true
		}
	}

	// If consumable tag exists, add effects if missing (as placeholder)
	if contains(tags, "consumable") {
		if _, exists := item["effects"]; !exists {
			item["effects"] = []interface{}{}
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "effects",
				Message: "Added empty 'effects' array for consumable",
			})
			modified = true
		}
	}

	// If container tag exists, ensure container_slots and allowed_types exist
	if contains(tags, "container") {
		if _, exists := item["container_slots"]; !exists {
			item["container_slots"] = 20 // Default container size
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "container_slots",
				Message: "Added missing 'container_slots' (defaulted to 20)",
			})
			modified = true
		}
		if _, exists := item["allowed_types"]; !exists {
			item["allowed_types"] = "any"
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "allowed_types",
				Message: "Added missing 'allowed_types' (defaulted to 'any')",
			})
			modified = true
		}
	} else {
		// Remove container properties if container tag is missing
		if _, exists := item["container_slots"]; exists {
			delete(item, "container_slots")
			changes = append(changes, Change{
				File:    filename,
				Type:    "removed",
				Field:   "container_slots",
				Message: "Removed 'container_slots' (item is not tagged as 'container')",
			})
			modified = true
		}
		if _, exists := item["allowed_types"]; exists {
			delete(item, "allowed_types")
			changes = append(changes, Change{
				File:    filename,
				Type:    "removed",
				Field:   "allowed_types",
				Message: "Removed 'allowed_types' (item is not tagged as 'container')",
			})
			modified = true
		}
	}

	// If pack tag exists, ensure contents property exists
	if contains(tags, "pack") {
		if _, exists := item["contents"]; !exists {
			item["contents"] = []interface{}{}
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "contents",
				Message: "⚠️ MANUAL FIX REQUIRED: Added empty 'contents' array for pack (needs items)",
			})
			modified = true
		}
	}

	// If focus tag exists, ensure equipment tag and provides property exist
	if contains(tags, "focus") {
		// Add equipment tag if missing
		if !contains(tags, "equipment") {
			tags = append(tags, "equipment")
			item["tags"] = interfaceSlice(tags)
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "tags",
				Message: "Added 'equipment' tag (required for focus items)",
			})
			modified = true
		}

		// Check for provides property
		if _, exists := item["provides"]; !exists {
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "provides",
				Message: "⚠️ MANUAL FIX REQUIRED: Focus items must have 'provides' property with spell component item ID",
			})
			// Don't add a default value - this needs manual input
		}
	}

	// 5. ENSURE TYPE-SPECIFIC FIELDS EXIST

	// Armor needs AC (but not armor sets/packs)
	if strings.Contains(itemType, "armor") && !contains(tags, "pack") {
		if _, exists := item["ac"]; !exists {
			item["ac"] = "10"
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "ac",
				Message: "Added missing 'ac' for armor (needs manual value)",
			})
			modified = true
		}
	}

	// Melee weapons need damage
	if strings.Contains(itemType, "melee") {
		if _, exists := item["damage"]; !exists {
			item["damage"] = "1d6"
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "damage",
				Message: "Added missing 'damage' for melee weapon (needs manual value)",
			})
			modified = true
		}
		if _, exists := item["damage-type"]; !exists {
			item["damage-type"] = "slashing"
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "damage-type",
				Message: "Added missing 'damage-type' for melee weapon",
			})
			modified = true
		}
	}

	// Ranged weapons need ammunition, range, and range-long
	if strings.Contains(itemType, "ranged") {
		if _, exists := item["ammunition"]; !exists {
			item["ammunition"] = "arrows"
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "ammunition",
				Message: "Added missing 'ammunition' for ranged weapon",
			})
			modified = true
		}
		if _, exists := item["range"]; !exists {
			item["range"] = "80"
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "range",
				Message: "Added missing 'range' for ranged weapon (needs manual value)",
			})
			modified = true
		}
		if _, exists := item["range-long"]; !exists {
			item["range-long"] = "320"
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "range-long",
				Message: "Added missing 'range-long' for ranged weapon (needs manual value)",
			})
			modified = true
		}
		if _, exists := item["damage-type"]; !exists {
			item["damage-type"] = "piercing"
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "damage-type",
				Message: "Added missing 'damage-type' for ranged weapon",
			})
			modified = true
		}
	}

	// 6. STANDARDIZE PROPERTY ORDER
	orderedItem := orderItemProperties(item)

	// 7. WRITE BACK TO FILE (if not dry run and modified)
	if modified && !dryRun {
		// Marshal with indentation
		output, err := json.MarshalIndent(orderedItem, "", "  ")
		if err != nil {
			log.Printf("Error marshaling %s: %v", filename, err)
			return changes, false
		}

		// Write to file
		if err := os.WriteFile(filePath, output, 0644); err != nil {
			log.Printf("Error writing %s: %v", filename, err)
			return changes, false
		}

		changes = append(changes, Change{
			File:    filename,
			Type:    "reordered",
			Message: "Standardized property ordering",
		})
	}

	return changes, modified
}

// orderItemProperties returns a new map with properties in the standard order
func orderItemProperties(item map[string]interface{}) map[string]interface{} {
	ordered := make(map[string]interface{})

	// Define the standard order
	propertyOrder := []string{
		"id",
		"name",
		"description",
		"rarity",
		"price",
		"weight",
		"stack",
		"type",
		"gear_slot",
		"container_slots",
		"allowed_types",
		"contents",
		"provides",
		"ac",
		"damage",
		"damage-type",
		"heal",
		"ammunition",
		"range",
		"range-long",
		"effects",
		"tags",
		"notes",
		"image",
		"img",
	}

	// Add properties in order if they exist
	for _, key := range propertyOrder {
		if val, exists := item[key]; exists {
			ordered[key] = val
		}
	}

	// Add any remaining properties that aren't in the standard order
	for key, val := range item {
		if _, exists := ordered[key]; !exists {
			ordered[key] = val
		}
	}

	return ordered
}

// interfaceSlice converts []string to []interface{} for JSON encoding
func interfaceSlice(strings []string) []interface{} {
	result := make([]interface{}, len(strings))
	for i, s := range strings {
		result[i] = s
	}
	return result
}

// normalizePropertyNames converts all property names to their correct lowercase format
func normalizePropertyNames(item map[string]interface{}, filename string) (map[string]interface{}, []Change) {
	changes := []Change{}
	normalized := make(map[string]interface{})

	// Map of incorrect property names to correct ones
	propertyNameMap := map[string]string{
		// Lowercase everything and handle common variations
		"ID":             "id",
		"Id":             "id",
		"NAME":           "name",
		"Name":           "name",
		"DESCRIPTION":    "description",
		"Description":    "description",
		"RARITY":         "rarity",
		"Rarity":         "rarity",
		"PRICE":          "price",
		"Price":          "price",
		"WEIGHT":         "weight",
		"Weight":         "weight",
		"STACK":          "stack",
		"Stack":          "stack",
		"TYPE":           "type",
		"Type":           "type",
		"GEAR_SLOT":      "gear_slot",
		"Gear_Slot":      "gear_slot",
		"Gear_slot":      "gear_slot",
		"gear_Slot":      "gear_slot",
		"GearSlot":       "gear_slot",
		"gearSlot":       "gear_slot",
		"CONTAINER_SLOTS": "container_slots",
		"Container_Slots": "container_slots",
		"Container_slots": "container_slots",
		"ContainerSlots":  "container_slots",
		"containerSlots":  "container_slots",
		"ALLOWED_TYPES":   "allowed_types",
		"Allowed_Types":   "allowed_types",
		"Allowed_types":   "allowed_types",
		"AllowedTypes":    "allowed_types",
		"allowedTypes":    "allowed_types",
		"AC":              "ac",
		"Ac":              "ac",
		"DAMAGE":          "damage",
		"Damage":          "damage",
		"DAMAGE-TYPE":     "damage-type",
		"Damage-Type":     "damage-type",
		"Damage-type":     "damage-type",
		"damage-Type":     "damage-type",
		"HEAL":            "heal",
		"Heal":            "heal",
		"AMMUNITION":      "ammunition",
		"Ammunition":      "ammunition",
		"RANGE":           "range",
		"Range":           "range",
		"RANGE-LONG":      "range-long",
		"Range-Long":      "range-long",
		"Range-long":      "range-long",
		"range-Long":      "range-long",
		"EFFECTS":         "effects",
		"Effects":         "effects",
		"TAGS":            "tags",
		"Tags":            "tags",
		"NOTES":           "notes",
		"Notes":           "notes",
		"IMAGE":           "image",
		"Image":           "image",
		"IMG":             "img",
		"Img":             "img",
		"CONTENTS":        "contents",
		"Contents":        "contents",
	}

	for key, value := range item {
		correctKey := key

		// Check if this key needs normalization
		if mappedKey, exists := propertyNameMap[key]; exists {
			correctKey = mappedKey
			changes = append(changes, Change{
				File:    filename,
				Type:    "fixed",
				Field:   key,
				Message: fmt.Sprintf("Normalized property name '%s' → '%s'", key, correctKey),
			})
		}

		// Also handle values that need normalization
		if correctKey == "gear_slot" {
			if strVal, ok := value.(string); ok {
				// Normalize gear_slot values to lowercase
				normalizedValue := strings.ToLower(strVal)
				if normalizedValue != strVal {
					changes = append(changes, Change{
						File:    filename,
						Type:    "fixed",
						Field:   "gear_slot",
						Message: fmt.Sprintf("Normalized gear_slot value '%s' → '%s'", strVal, normalizedValue),
					})
					value = normalizedValue
				}
			}
		}

		normalized[correctKey] = value
	}

	return normalized, changes
}

// CleanupStartingGear reorders fields in starting gear to maintain consistency
// Field order: given_items, equipment_choices, pack_choice
func CleanupStartingGear(dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{
		Changes: []Change{},
	}

	filePath := "game-data/systems/new-character/starting-gear.json"

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return result, fmt.Errorf("failed to read starting-gear.json: %w", err)
	}

	var gearData []interface{}
	if err := json.Unmarshal(data, &gearData); err != nil {
		return result, fmt.Errorf("failed to parse starting-gear.json: %w", err)
	}

	modified := false
	result.FilesProcessed = 1

	// Process each class entry
	for i, classEntry := range gearData {
		classMap, ok := classEntry.(map[string]interface{})
		if !ok {
			continue
		}

		className, _ := classMap["class"].(string)
		startingGear, ok := classMap["starting_gear"].(map[string]interface{})
		if !ok {
			continue
		}

		// Create new ordered map
		orderedGear := make(map[string]interface{})

		// Field order: given_items, equipment_choices, pack_choice
		fieldOrder := []string{"given_items", "equipment_choices", "pack_choice"}

		// Track if order changed by comparing first non-empty field
		firstField := ""
		for _, key := range fieldOrder {
			if _, exists := startingGear[key]; exists && firstField == "" {
				firstField = key
				break
			}
		}

		// Get actual first key in the map
		actualFirstKey := ""
		for key := range startingGear {
			actualFirstKey = key
			break
		}

		needsReorder := (firstField != "" && actualFirstKey != firstField)

		// Copy fields in the correct order
		for _, field := range fieldOrder {
			if val, exists := startingGear[field]; exists {
				orderedGear[field] = val
			}
		}

		// Copy any other fields that might exist (shouldn't be any, but just in case)
		for key, val := range startingGear {
			if _, exists := orderedGear[key]; !exists {
				orderedGear[key] = val
				result.Changes = append(result.Changes, Change{
					File:    "starting-gear.json",
					Type:    "fixed",
					Field:   className,
					Message: fmt.Sprintf("Found unexpected field '%s'", key),
				})
			}
		}

		// Update if order changed
		if needsReorder {
			modified = true
			result.Changes = append(result.Changes, Change{
				File:    "starting-gear.json",
				Type:    "reordered",
				Field:   className,
				Message: "Reordered fields to: given_items, equipment_choices, pack_choice",
			})
		}

		// Update the entry
		classMap["starting_gear"] = orderedGear
		gearData[i] = classMap
	}

	// Write back if modified and not dry run
	if modified {
		result.FilesModified = 1
		if !dryRun {
			cleanedData, err := json.MarshalIndent(gearData, "", "  ")
			if err != nil {
				return result, fmt.Errorf("failed to marshal cleaned data: %w", err)
			}

			if err := os.WriteFile(filePath, cleanedData, 0644); err != nil {
				return result, fmt.Errorf("failed to write cleaned file: %w", err)
			}
		}
	}

	log.Printf("Starting gear cleanup complete: 1 file processed, %d modified", result.FilesModified)
	return result, nil
}

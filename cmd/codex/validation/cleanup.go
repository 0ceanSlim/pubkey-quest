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

// CleanupEffects migrates effect files from old structure to new structure
func CleanupEffects(dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{
		Changes: []Change{},
	}

	effectsPath := "game-data/effects"

	err := filepath.WalkDir(effectsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			changes, modified := cleanupEffectFile(path, dryRun)
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

	log.Printf("Effects cleanup complete: %d files processed, %d modified", result.FilesProcessed, result.FilesModified)
	return result, nil
}

func cleanupEffectFile(filePath string, dryRun bool) ([]Change, bool) {
	changes := []Change{}
	filename := filepath.Base(filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Error reading %s: %v", filename, err)
		return changes, false
	}

	var effect map[string]interface{}
	if err := json.Unmarshal(data, &effect); err != nil {
		log.Printf("Error parsing %s: %v", filename, err)
		return changes, false
	}

	modified := false

	// STEP 1: Rename 'effects' to 'modifiers'
	if oldEffects, exists := effect["effects"]; exists {
		effect["modifiers"] = oldEffects
		delete(effect, "effects")
		changes = append(changes, Change{
			File:    filename,
			Type:    "fixed",
			Field:   "effects → modifiers",
			Message: "Renamed 'effects' field to 'modifiers'",
		})
		modified = true
	}

	// STEP 2: Remove deprecated fields
	deprecatedFields := []string{"icon", "color", "silent"}
	for _, field := range deprecatedFields {
		if _, exists := effect[field]; exists {
			delete(effect, field)
			changes = append(changes, Change{
				File:    filename,
				Type:    "removed",
				Field:    field,
				Message:  fmt.Sprintf("Removed deprecated field '%s'", field),
			})
			modified = true
		}
	}

	// STEP 3: Infer and add source_type if missing
	if _, exists := effect["source_type"]; !exists {
		sourceType := inferSourceType(effect, filename)
		effect["source_type"] = sourceType
		changes = append(changes, Change{
			File:    filename,
			Type:    "added",
			Field:   "source_type",
			Message:  fmt.Sprintf("Added inferred source_type: '%s'", sourceType),
		})
		modified = true
	}

	// STEP 4: Add visible field based on source_type
	if _, exists := effect["visible"]; !exists {
		sourceType, _ := effect["source_type"].(string)
		visible := sourceType != "system_ticker"
		effect["visible"] = visible
		changes = append(changes, Change{
			File:    filename,
			Type:    "added",
			Field:   "visible",
			Message:  fmt.Sprintf("Added visible: %v (based on source_type)", visible),
		})
		modified = true
	}

	// STEP 5: Ensure category field exists and is correct
	if category, exists := effect["category"]; !exists || category == "system" {
		sourceType, _ := effect["source_type"].(string)
		newCategory := inferCategory(effect, sourceType)
		effect["category"] = newCategory
		if !exists {
			changes = append(changes, Change{
				File:    filename,
				Type:    "added",
				Field:   "category",
				Message:  fmt.Sprintf("Added inferred category: '%s'", newCategory),
			})
		} else {
			changes = append(changes, Change{
				File:    filename,
				Type:    "fixed",
				Field:   "category",
				Message:  fmt.Sprintf("Fixed category: 'system' → '%s'", newCategory),
			})
		}
		modified = true
	}

	// STEP 6: Add removal field if missing
	if _, exists := effect["removal"]; !exists {
		removal := inferRemoval(effect)
		effect["removal"] = removal
		changes = append(changes, Change{
			File:    filename,
			Type:    "added",
			Field:   "removal",
			Message:  "Added inferred removal condition",
		})
		modified = true
	}

	// STEP 7: Convert modifiers to new structure
	if modifiers, ok := effect["modifiers"].([]interface{}); ok {
		newModifiers := []interface{}{}
		for i, mod := range modifiers {
			modMap, ok := mod.(map[string]interface{})
			if !ok {
				newModifiers = append(newModifiers, mod)
				continue
			}

			newMod := make(map[string]interface{})

			// Copy stat
			if stat, ok := modMap["type"].(string); ok {
				newMod["stat"] = stat
			}

			// Copy value
			if value, ok := modMap["value"]; ok {
				newMod["value"] = value
			}

			// Determine type (instant vs periodic)
			if tickInterval, ok := modMap["tick_interval"]; ok && tickInterval != nil {
				if ti, ok := tickInterval.(float64); ok && ti > 0 {
					newMod["type"] = "periodic"
					newMod["tick_interval"] = tickInterval
				} else {
					newMod["type"] = "instant"
				}
			} else {
				newMod["type"] = "instant"
			}

			// Copy delay if present and non-zero
			if delay, ok := modMap["delay"].(float64); ok && delay > 0 {
				newMod["delay"] = delay
			}

			// Remove duration field (moved to removal)
			if _, exists := modMap["duration"]; exists {
				changes = append(changes, Change{
					File:    filename,
					Type:    "removed",
					Field:   fmt.Sprintf("modifiers[%d].duration", i),
					Message:  "Removed 'duration' from modifier (moved to removal.timer)",
				})
			}

			newModifiers = append(newModifiers, newMod)
		}

		effect["modifiers"] = newModifiers
		changes = append(changes, Change{
			File:    filename,
			Type:    "fixed",
			Field:   "modifiers",
			Message:  "Restructured modifiers to new format",
		})
		modified = true
	}

	// STEP 8: Reorder fields to standard order
	orderedEffect := orderEffectProperties(effect)

	// STEP 9: Write back to file (if not dry run and modified)
	if modified && !dryRun {
		output, err := json.MarshalIndent(orderedEffect, "", "  ")
		if err != nil {
			log.Printf("Error marshaling %s: %v", filename, err)
			return changes, false
		}

		if err := os.WriteFile(filePath, output, 0644); err != nil {
			log.Printf("Error writing %s: %v", filename, err)
			return changes, false
		}
	}

	return changes, modified
}

// inferSourceType infers the source_type from the old structure
func inferSourceType(effect map[string]interface{}, filename string) string {
	// Check if it's a system ticker (silent + system category + has tick_interval)
	if category, ok := effect["category"].(string); ok && category == "system" {
		if silent, ok := effect["silent"].(bool); ok && silent {
			return "system_ticker"
		}
	}

	// Check filenames for known patterns
	if strings.Contains(filename, "accumulation") {
		return "system_ticker"
	}

	if strings.Contains(filename, "encumbrance") || strings.Contains(filename, "hungry") ||
		strings.Contains(filename, "stuffed") || strings.Contains(filename, "tired") ||
		strings.Contains(filename, "exhausted") || strings.Contains(filename, "fatigued") ||
		strings.Contains(filename, "starving") {
		return "system_status"
	}

	// Default to applied
	return "applied"
}

// inferCategory infers the category from the old structure
func inferCategory(effect map[string]interface{}, sourceType string) string {
	if sourceType == "system_ticker" || sourceType == "system_status" {
		return "status"
	}

	// Check if it's a buff or debuff based on modifiers
	if modifiers, ok := effect["modifiers"].([]interface{}); ok {
		for _, mod := range modifiers {
			if modMap, ok := mod.(map[string]interface{}); ok {
				if value, ok := modMap["value"].(float64); ok {
					if value > 0 {
						return "buff"
					} else if value < 0 {
						return "debuff"
					}
				}
			}
		}
	}

	// Default to buff
	return "buff"
}

// inferRemoval infers the removal condition from the old structure
func inferRemoval(effect map[string]interface{}) map[string]interface{} {
	removal := make(map[string]interface{})

	// Check if effect has duration from modifiers
	duration := 0
	if modifiers, ok := effect["modifiers"].([]interface{}); ok {
		for _, mod := range modifiers {
			if modMap, ok := mod.(map[string]interface{}); ok {
				if d, ok := modMap["duration"].(float64); ok {
					duration = int(d)
					break
				}
			}
		}
	}

	sourceType, _ := effect["source_type"].(string)

	if sourceType == "system_ticker" {
		removal["type"] = "permanent"
	} else if sourceType == "system_status" {
		removal["type"] = "conditional"
		removal["condition"] = "AUTO-INFERRED: Needs manual specification"
	} else if duration > 0 {
		removal["type"] = "timed"
		removal["timer"] = duration
	} else {
		removal["type"] = "timed"
		removal["timer"] = 0 // Will need manual fix
	}

	return removal
}

// orderEffectProperties returns a new map with properties in the standard order
func orderEffectProperties(effect map[string]interface{}) map[string]interface{} {
	ordered := make(map[string]interface{})

	// Define the standard order
	propertyOrder := []string{
		"id",
		"name",
		"description",
		"source_type",
		"category",
		"removal",
		"modifiers",
		"message",
		"visible",
	}

	// Add properties in order if they exist
	for _, key := range propertyOrder {
		if val, exists := effect[key]; exists {
			ordered[key] = val
		}
	}

	// Add any remaining properties that aren't in the standard order
	for key, val := range effect {
		if _, exists := ordered[key]; !exists {
			ordered[key] = val
		}
	}

	return ordered
}

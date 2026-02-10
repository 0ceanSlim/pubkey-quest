package validation

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Issue represents a validation issue found in game data
type Issue struct {
	Type     string `json:"type"`     // "error", "warning", "info"
	Category string `json:"category"` // "items", "spells", "monsters", etc.
	File     string `json:"file"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

// Result holds the validation results
type Result struct {
	Issues []Issue `json:"issues"`
	Stats  Stats   `json:"stats"`
}

// Stats holds statistics about validation
type Stats struct {
	TotalFiles   int `json:"total_files"`
	ErrorCount   int `json:"error_count"`
	WarningCount int `json:"warning_count"`
	InfoCount    int `json:"info_count"`
}

// ValidateAll runs all validation checks on game data
func ValidateAll() (*Result, error) {
	result := &Result{
		Issues: []Issue{},
	}

	// Validate items
	if itemIssues, err := ValidateItems(); err != nil {
		return nil, err
	} else {
		result.Issues = append(result.Issues, itemIssues...)
	}

	// Validate monsters
	if monsterIssues, err := ValidateMonsters(); err != nil {
		return nil, err
	} else {
		result.Issues = append(result.Issues, monsterIssues...)
	}

	// Validate locations
	if locationIssues, err := ValidateLocations(); err != nil {
		return nil, err
	} else {
		result.Issues = append(result.Issues, locationIssues...)
	}

	// Validate NPCs
	if npcIssues, err := ValidateNPCs(); err != nil {
		return nil, err
	} else {
		result.Issues = append(result.Issues, npcIssues...)
	}

	// Validate starting gear
	if gearIssues, err := ValidateStartingGear(); err != nil {
		return nil, err
	} else {
		result.Issues = append(result.Issues, gearIssues...)
	}

	// Validate effects
	if effectIssues, err := ValidateEffects(); err != nil {
		return nil, err
	} else {
		result.Issues = append(result.Issues, effectIssues...)
	}

	// Calculate stats
	for _, issue := range result.Issues {
		result.Stats.TotalFiles++
		switch issue.Type {
		case "error":
			result.Stats.ErrorCount++
		case "warning":
			result.Stats.WarningCount++
		case "info":
			result.Stats.InfoCount++
		}
	}

	return result, nil
}

// ValidateItems validates all item files
func ValidateItems() ([]Issue, error) {
	issues := []Issue{}
	itemsPath := "game-data/items"

	// First, build a set of all valid item IDs for reference checking
	validItemIDs := make(map[string]bool)
	filepath.WalkDir(itemsPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			filename := strings.TrimSuffix(filepath.Base(path), ".json")
			validItemIDs[filename] = true
		}
		return nil
	})

	// Now validate each item
	err := filepath.WalkDir(itemsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			itemIssues := validateItemFile(path, validItemIDs)
			issues = append(issues, itemIssues...)
		}
		return nil
	})

	return issues, err
}

func validateItemFile(filePath string, validItemIDs map[string]bool) []Issue {
	issues := []Issue{}
	filename := filepath.Base(filePath)
	idFromFilename := strings.TrimSuffix(filename, ".json")

	data, err := os.ReadFile(filePath)
	if err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "items",
			File:     filename,
			Message:  fmt.Sprintf("Failed to read file: %v", err),
		})
		return issues
	}

	var item map[string]interface{}
	if err := json.Unmarshal(data, &item); err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "items",
			File:     filename,
			Message:  fmt.Sprintf("Invalid JSON: %v", err),
		})
		return issues
	}

	// Check ALL required fields
	requiredFields := []string{"id", "name", "description", "rarity", "price", "weight", "stack", "type", "image", "tags", "notes"}
	for _, field := range requiredFields {
		if _, exists := item[field]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    field,
				Message:  fmt.Sprintf("Missing required field: %s", field),
			})
		} else {
			// Check for empty strings in string fields
			if field == "description" {
				if val, ok := item[field].(string); ok && val == "" {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "items",
						File:     filename,
						Field:    field,
						Message:  fmt.Sprintf("Required field '%s' cannot be empty", field),
					})
				}
			}
		}
	}

	// Check ID matches filename
	if id, ok := item["id"].(string); ok {
		if id != idFromFilename {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "id",
				Message:  fmt.Sprintf("ID '%s' doesn't match filename '%s'", id, idFromFilename),
			})
		}
	}

	// Check price is non-negative
	if price, ok := item["price"].(float64); ok {
		if price < 0 {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "price",
				Message:  "Price cannot be negative",
			})
		}
	}

	// Check weight is non-negative
	if weight, ok := item["weight"].(float64); ok {
		if weight < 0 {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "weight",
				Message:  "Weight cannot be negative",
			})
		}
	}

	// Check stack is positive
	if stack, ok := item["stack"].(float64); ok {
		if stack < 1 {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "stack",
				Message:  "Stack must be at least 1",
			})
		}
	}

	// Check rarity is valid
	validRarities := map[string]bool{
		"common": true, "uncommon": true, "rare": true,
		"legendary": true, "mythical": true,
	}
	if rarity, ok := item["rarity"].(string); ok {
		if !validRarities[strings.ToLower(rarity)] {
			issues = append(issues, Issue{
				Type:     "warning",
				Category: "items",
				File:     filename,
				Field:    "rarity",
				Message:  fmt.Sprintf("Non-standard rarity: %s", rarity),
			})
		}
	}

	// Get tags for conditional validation
	tags := []string{}
	if tagsArray, ok := item["tags"].([]interface{}); ok {
		for _, tag := range tagsArray {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	// TAG-BASED CONDITIONAL VALIDATION

	// Equipment tag requires gear_slot
	if contains(tags, "equipment") {
		if gearSlot, exists := item["gear_slot"]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "gear_slot",
				Message:  "Items with 'equipment' tag must have 'gear_slot' property",
			})
		} else if gearSlotStr, ok := gearSlot.(string); ok {
			validGearSlots := map[string]bool{
				"hands": true, "mainhand": true, "offhand": true,
				"chest": true, "head": true, "legs": true,
				"gloves": true, "boots": true,
				"neck": true, "ring": true,
				"ammo": true, "bag": true,
			}
			if !validGearSlots[gearSlotStr] {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "items",
					File:     filename,
					Field:    "gear_slot",
					Message:  fmt.Sprintf("Invalid gear_slot '%s'. Must be one of: hands, mainhand, offhand, chest, head, legs, gloves, boots, neck, ring, ammo, bag", gearSlotStr),
				})
			}
		}
	} else {
		// Items with gear_slot should have equipment tag
		if _, exists := item["gear_slot"]; exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "tags",
				Message:  "Item has 'gear_slot' but is missing 'equipment' tag - should add 'equipment' to tags",
			})
		}
	}

	// Consumable tag requires effects property
	if contains(tags, "consumable") {
		if _, exists := item["effects"]; !exists {
			issues = append(issues, Issue{
				Type:     "warning",
				Category: "items",
				File:     filename,
				Field:    "effects",
				Message:  "Items with 'consumable' tag should have 'effects' property (not yet enforced)",
			})
		}
	}

	// Container tag requires container_slots and allowed_types
	if contains(tags, "container") {
		if _, exists := item["container_slots"]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "container_slots",
				Message:  "Items with 'container' tag must have 'container_slots' property",
			})
		} else {
			// Validate container_slots is a positive number
			if slots, ok := item["container_slots"].(float64); ok {
				if slots < 1 {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "items",
						File:     filename,
						Field:    "container_slots",
						Message:  "container_slots must be at least 1",
					})
				}
			}
		}

		if allowedTypes, exists := item["allowed_types"]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "allowed_types",
				Message:  "Items with 'container' tag must have 'allowed_types' property",
			})
		} else {
			// Validate allowed_types format
			switch v := allowedTypes.(type) {
			case string:
				// Can be "any" or a specific type/item ID
				if v != "any" && v == "" {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "items",
						File:     filename,
						Field:    "allowed_types",
						Message:  "allowed_types cannot be empty string",
					})
				}
			case []interface{}:
				// Array of types or IDs
				if len(v) == 0 {
					issues = append(issues, Issue{
						Type:     "warning",
						Category: "items",
						File:     filename,
						Field:    "allowed_types",
						Message:  "allowed_types array is empty",
					})
				}
			default:
				issues = append(issues, Issue{
					Type:     "error",
					Category: "items",
					File:     filename,
					Field:    "allowed_types",
					Message:  "allowed_types must be a string ('any', type, or item ID) or array of strings",
				})
			}
		}
	} else {
		// Non-container items should NOT have container properties
		if _, exists := item["container_slots"]; exists {
			issues = append(issues, Issue{
				Type:     "warning",
				Category: "items",
				File:     filename,
				Field:    "container_slots",
				Message:  "Item has 'container_slots' but is not tagged as 'container'",
			})
		}
		if _, exists := item["allowed_types"]; exists {
			issues = append(issues, Issue{
				Type:     "warning",
				Category: "items",
				File:     filename,
				Field:    "allowed_types",
				Message:  "Item has 'allowed_types' but is not tagged as 'container'",
			})
		}
	}

	// TYPE-BASED CONDITIONAL VALIDATION
	itemType := ""
	if t, ok := item["type"].(string); ok {
		itemType = t
	}

	// Armor must have AC (but armor sets are packs containing pieces, not equippable themselves)
	if strings.Contains(strings.ToLower(itemType), "armor") && !contains(tags, "pack") {
		if ac, exists := item["ac"]; !exists || ac == nil || ac == "" {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "ac",
				Message:  "Armor items must have 'ac' property",
			})
		}
	} else if !strings.Contains(strings.ToLower(itemType), "armor") {
		// Non-armor should not have AC
		if ac, exists := item["ac"]; exists && ac != nil && ac != "" {
			issues = append(issues, Issue{
				Type:     "warning",
				Category: "items",
				File:     filename,
				Field:    "ac",
				Message:  "Non-armor item has 'ac' property (should be removed)",
			})
		}
	}

	// Melee weapons must have damage
	if strings.Contains(strings.ToLower(itemType), "melee") {
		if damage, exists := item["damage"]; !exists || damage == nil || damage == "" {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "damage",
				Message:  "Melee weapons must have 'damage' property",
			})
		}
	}

	// Ranged weapons need ammunition (unless thrown - they are their own ammo), range, and range-long
	if strings.Contains(strings.ToLower(itemType), "ranged") {
		if !contains(tags, "thrown") {
			if ammunition, exists := item["ammunition"]; !exists || ammunition == nil || ammunition == "" {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "items",
					File:     filename,
					Field:    "ammunition",
					Message:  "Ranged weapons must have 'ammunition' property (unless thrown)",
				})
			}
		}
		if rng, exists := item["range"]; !exists || rng == nil || rng == "" || rng == "null" {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "range",
				Message:  "Ranged weapons must have 'range' property",
			})
		}
		if rngLong, exists := item["range-long"]; !exists || rngLong == nil || rngLong == "" || rngLong == "null" {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "range-long",
				Message:  "Ranged weapons must have 'range-long' property",
			})
		}
	} else {
		// Non-ranged weapons should not have these properties
		checkUnnecessaryField(item, "ammunition", filename, "Non-ranged item", &issues)
		checkUnnecessaryField(item, "range", filename, "Non-ranged item", &issues)
		checkUnnecessaryField(item, "range-long", filename, "Non-ranged item", &issues)
	}

	// Check for unnecessary null/empty fields
	unnecessaryIfNull := []string{"ac", "damage", "heal", "ammunition", "range", "range-long"}
	for _, field := range unnecessaryIfNull {
		checkUnnecessaryField(item, field, filename, "Item", &issues)
	}

	// PACK TAG VALIDATION
	if contains(tags, "pack") {
		if contents, exists := item["contents"]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "contents",
				Message:  "Items with 'pack' tag must have 'contents' property",
			})
		} else if contentsArray, ok := contents.([]interface{}); ok {
			// Validate contents array format and item IDs
			for i, entry := range contentsArray {
				if entryArray, ok := entry.([]interface{}); ok {
					if len(entryArray) != 2 {
						issues = append(issues, Issue{
							Type:     "error",
							Category: "items",
							File:     filename,
							Field:    "contents",
							Message:  fmt.Sprintf("Pack contents entry %d must be [item_id, quantity] format", i),
						})
						continue
					}

					// Validate item ID exists
					if itemID, ok := entryArray[0].(string); ok {
						if !validItemIDs[itemID] {
							issues = append(issues, Issue{
								Type:     "error",
								Category: "items",
								File:     filename,
								Field:    "contents",
								Message:  fmt.Sprintf("Pack contains invalid item ID '%s' at index %d", itemID, i),
							})
						}
					} else {
						issues = append(issues, Issue{
							Type:     "error",
							Category: "items",
							File:     filename,
							Field:    "contents",
							Message:  fmt.Sprintf("Pack contents entry %d: item ID must be a string", i),
						})
					}

					// Validate quantity is a number
					if quantity, ok := entryArray[1].(float64); ok {
						if quantity < 1 {
							issues = append(issues, Issue{
								Type:     "error",
								Category: "items",
								File:     filename,
								Field:    "contents",
								Message:  fmt.Sprintf("Pack contents entry %d: quantity must be at least 1", i),
							})
						}
					} else {
						issues = append(issues, Issue{
							Type:     "error",
							Category: "items",
							File:     filename,
							Field:    "contents",
							Message:  fmt.Sprintf("Pack contents entry %d: quantity must be a number", i),
						})
					}
				} else {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "items",
						File:     filename,
						Field:    "contents",
						Message:  fmt.Sprintf("Pack contents entry %d must be an array", i),
					})
				}
			}
		} else if contents != nil {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "contents",
				Message:  "Pack 'contents' property must be an array",
			})
		}
	}

	// FOCUS TAG VALIDATION
	if contains(tags, "focus") {
		// Must have equipment tag
		if !contains(tags, "equipment") {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "tags",
				Message:  "Items with 'focus' tag must also have 'equipment' tag",
			})
		}

		// Must have provides property
		if provides, exists := item["provides"]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "provides",
				Message:  "Items with 'focus' tag must have 'provides' property",
			})
		} else if providesStr, ok := provides.(string); ok {
			// Validate that the provided item ID exists
			if providesStr == "" {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "items",
					File:     filename,
					Field:    "provides",
					Message:  "Focus 'provides' property cannot be empty",
				})
			} else if !validItemIDs[providesStr] {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "items",
					File:     filename,
					Field:    "provides",
					Message:  fmt.Sprintf("Focus provides invalid item ID '%s'", providesStr),
				})
			}
		} else {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "items",
				File:     filename,
				Field:    "provides",
				Message:  "Focus 'provides' property must be a string (spell component item ID)",
			})
		}
	}

	// Check if image file exists (warning, not error)
	if image, ok := item["image"].(string); ok && image != "" {
		imagePath := filepath.Join("www/res/img/items", idFromFilename+".png")
		if _, err := os.Stat(imagePath); os.IsNotExist(err) {
			issues = append(issues, Issue{
				Type:     "warning",
				Category: "items",
				File:     filename,
				Field:    "image",
				Message:  "Image file not found",
			})
		}
	}

	return issues
}

// Helper function to check if a slice contains a string
func contains(slice []string, str string) bool {
	for _, item := range slice {
		if item == str {
			return true
		}
	}
	return false
}

// Helper to check for unnecessary null/empty fields
func checkUnnecessaryField(item map[string]interface{}, field, filename, context string, issues *[]Issue) {
	if val, exists := item[field]; exists {
		isEmpty := false
		switch v := val.(type) {
		case nil:
			isEmpty = true
		case string:
			isEmpty = v == "" || v == "null"
		}
		if isEmpty {
			*issues = append(*issues, Issue{
				Type:     "warning",
				Category: "items",
				File:     filename,
				Field:    field,
				Message:  fmt.Sprintf("%s has unnecessary null/empty '%s' property (should be removed)", context, field),
			})
		}
	}
}

// ValidateOneItem validates a single item by its ID (filename without .json)
func ValidateOneItem(itemID string) ([]Issue, error) {
	issues := []Issue{}
	itemsPath := "game-data/items"

	// Build valid item IDs set for reference checking
	validItemIDs := make(map[string]bool)
	filepath.WalkDir(itemsPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			filename := strings.TrimSuffix(filepath.Base(path), ".json")
			validItemIDs[filename] = true
		}
		return nil
	})

	filePath := filepath.Join(itemsPath, itemID+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("item file not found: %s", itemID)
	}

	issues = validateItemFile(filePath, validItemIDs)
	return issues, nil
}

// ValidateMonsters validates all monster files
func ValidateMonsters() ([]Issue, error) {
	issues := []Issue{}
	monstersPath := "game-data/monsters"

	err := filepath.WalkDir(monstersPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			monsterIssues := validateMonsterFile(path)
			issues = append(issues, monsterIssues...)
		}
		return nil
	})

	return issues, err
}

func validateMonsterFile(filePath string) []Issue {
	issues := []Issue{}
	filename := filepath.Base(filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "monsters",
			File:     filename,
			Message:  fmt.Sprintf("Failed to read file: %v", err),
		})
		return issues
	}

	var monster map[string]interface{}
	if err := json.Unmarshal(data, &monster); err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "monsters",
			File:     filename,
			Message:  fmt.Sprintf("Invalid JSON: %v", err),
		})
		return issues
	}

	// Check required fields
	requiredFields := []string{"id", "name"}
	for _, field := range requiredFields {
		if _, exists := monster[field]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "monsters",
				File:     filename,
				Field:    field,
				Message:  fmt.Sprintf("Missing required field: %s", field),
			})
		}
	}

	return issues
}

// ValidateLocations validates all location files
func ValidateLocations() ([]Issue, error) {
	issues := []Issue{}
	locationsPath := "game-data/locations"

	// Check cities and environments
	subDirs := []string{"cities", "environments"}
	for _, subDir := range subDirs {
		dirPath := filepath.Join(locationsPath, subDir)
		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && strings.HasSuffix(path, ".json") {
				locationIssues := validateLocationFile(path)
				issues = append(issues, locationIssues...)
			}
			return nil
		})

		if err != nil {
			return issues, err
		}
	}

	return issues, nil
}

func validateLocationFile(filePath string) []Issue {
	issues := []Issue{}
	filename := filepath.Base(filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "locations",
			File:     filename,
			Message:  fmt.Sprintf("Failed to read file: %v", err),
		})
		return issues
	}

	var location map[string]interface{}
	if err := json.Unmarshal(data, &location); err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "locations",
			File:     filename,
			Message:  fmt.Sprintf("Invalid JSON: %v", err),
		})
		return issues
	}

	// Check required fields
	requiredFields := []string{"id", "name"}
	for _, field := range requiredFields {
		if _, exists := location[field]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "locations",
				File:     filename,
				Field:    field,
				Message:  fmt.Sprintf("Missing required field: %s", field),
			})
		}
	}

	return issues
}

// ValidateNPCs validates all NPC files
func ValidateNPCs() ([]Issue, error) {
	issues := []Issue{}
	npcsPath := "game-data/npcs"

	err := filepath.WalkDir(npcsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			npcIssues := validateNPCFile(path)
			issues = append(issues, npcIssues...)
		}
		return nil
	})

	return issues, err
}

func validateNPCFile(filePath string) []Issue {
	issues := []Issue{}
	filename := filepath.Base(filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "npcs",
			File:     filename,
			Message:  fmt.Sprintf("Failed to read file: %v", err),
		})
		return issues
	}

	var npc map[string]interface{}
	if err := json.Unmarshal(data, &npc); err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "npcs",
			File:     filename,
			Message:  fmt.Sprintf("Invalid JSON: %v", err),
		})
		return issues
	}

	// Check required fields
	requiredFields := []string{"id", "name"}
	for _, field := range requiredFields {
		if _, exists := npc[field]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "npcs",
				File:     filename,
				Field:    field,
				Message:  fmt.Sprintf("Missing required field: %s", field),
			})
		}
	}

	return issues
}

// ValidateStartingGear validates the starting gear configuration file
// ValidateStartingGear validates the starting gear configuration file
func ValidateStartingGear() ([]Issue, error) {
	issues := []Issue{}
	filePath := "game-data/systems/new-character/starting-gear.json"

	// Build valid item IDs set for reference checking
	validItemIDs := make(map[string]bool)
	packItemIDs := make(map[string]bool)
	itemsPath := "game-data/items"

	// Walk through items to build both maps
	filepath.WalkDir(itemsPath, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			filename := strings.TrimSuffix(filepath.Base(path), ".json")
			validItemIDs[filename] = true

			// Check if this item is a pack
			data, err := os.ReadFile(path)
			if err == nil {
				var item map[string]interface{}
				if json.Unmarshal(data, &item) == nil {
					if itemType, ok := item["type"].(string); ok {
						if itemType == "Pack" {
							packItemIDs[filename] = true
						}
					}
				}
			}
		}
		return nil
	})

	// Read starting gear file
	data, err := os.ReadFile(filePath)
	if err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "starting-gear",
			File:     "starting-gear.json",
			Message:  fmt.Sprintf("Failed to read file: %v", err),
		})
		return issues, err
	}

	var gearData []map[string]interface{}
	if err := json.Unmarshal(data, &gearData); err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "starting-gear",
			File:     "starting-gear.json",
			Message:  fmt.Sprintf("Invalid JSON: %v", err),
		})
		return issues, err
	}

	// Validate each class entry
	for i, classEntry := range gearData {
		className := "unknown"
		if class, ok := classEntry["class"].(string); ok {
			className = class
		} else {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "starting-gear",
				File:     "starting-gear.json",
				Field:    fmt.Sprintf("entry[%d].class", i),
				Message:  "Missing or invalid 'class' field",
			})
			continue
		}

		// Check starting_gear exists
		startingGear, ok := classEntry["starting_gear"].(map[string]interface{})
		if !ok {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "starting-gear",
				File:     "starting-gear.json",
				Field:    className + ".starting_gear",
				Message:  "Missing 'starting_gear' object",
			})
			continue
		}

		// Check for pack in given_items
		hasPackInGivenItems := false
		if givenItems, ok := startingGear["given_items"].([]interface{}); ok {
			for _, item := range givenItems {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemID, ok := itemMap["item"].(string); ok {
						if packItemIDs[itemID] {
							hasPackInGivenItems = true
							break
						}
					}
				}
			}
		}

		// Check for pack_choice
		hasPackChoice := false
		if _, ok := startingGear["pack_choice"]; ok {
			hasPackChoice = true
		}

		// Validate mutual exclusivity: either pack_choice OR pack in given_items, not both
		if hasPackChoice && hasPackInGivenItems {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "starting-gear",
				File:     "starting-gear.json",
				Field:    className + ".starting_gear",
				Message:  "Cannot have both 'pack_choice' and a pack in 'given_items'. Use one or the other.",
			})
		}

		// Validate equipment_choices
		if equipmentChoices, ok := startingGear["equipment_choices"].([]interface{}); ok {
			for choiceIdx, choice := range equipmentChoices {
				if choiceMap, ok := choice.(map[string]interface{}); ok {
					validateEquipmentChoice(choiceMap, className, choiceIdx, validItemIDs, &issues)
				} else {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "starting-gear",
						File:     "starting-gear.json",
						Field:    fmt.Sprintf("%s.equipment_choices[%d]", className, choiceIdx),
						Message:  "Equipment choice must be an object",
					})
				}
			}
		} else {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "starting-gear",
				File:     "starting-gear.json",
				Field:    className + ".equipment_choices",
				Message:  "Missing or invalid 'equipment_choices' array",
			})
		}

		// Validate pack_choice (must reference actual pack items)
		if packChoice, ok := startingGear["pack_choice"].(map[string]interface{}); ok {
			if options, ok := packChoice["options"].([]interface{}); ok {
				for optIdx, opt := range options {
					if packID, ok := opt.(string); ok {
						if !validItemIDs[packID] {
							issues = append(issues, Issue{
								Type:     "error",
								Category: "starting-gear",
								File:     "starting-gear.json",
								Field:    fmt.Sprintf("%s.pack_choice.options[%d]", className, optIdx),
								Message:  fmt.Sprintf("Invalid pack ID '%s'", packID),
							})
						} else if !packItemIDs[packID] {
							issues = append(issues, Issue{
								Type:     "error",
								Category: "starting-gear",
								File:     "starting-gear.json",
								Field:    fmt.Sprintf("%s.pack_choice.options[%d]", className, optIdx),
								Message:  fmt.Sprintf("Item '%s' is not a pack (type must be 'Pack')", packID),
							})
						}
					} else {
						issues = append(issues, Issue{
							Type:     "error",
							Category: "starting-gear",
							File:     "starting-gear.json",
							Field:    fmt.Sprintf("%s.pack_choice.options[%d]", className, optIdx),
							Message:  "Pack option must be a string (pack ID)",
						})
					}
				}
			}
		}

		// Validate given_items
		if givenItems, ok := startingGear["given_items"].([]interface{}); ok {
			for itemIdx, item := range givenItems {
				if itemMap, ok := item.(map[string]interface{}); ok {
					if itemID, ok := itemMap["item"].(string); ok {
						if !validItemIDs[itemID] {
							issues = append(issues, Issue{
								Type:     "error",
								Category: "starting-gear",
								File:     "starting-gear.json",
								Field:    fmt.Sprintf("%s.given_items[%d].item", className, itemIdx),
								Message:  fmt.Sprintf("Invalid item ID '%s'", itemID),
							})
						}
					} else {
						issues = append(issues, Issue{
							Type:     "error",
							Category: "starting-gear",
							File:     "starting-gear.json",
							Field:    fmt.Sprintf("%s.given_items[%d].item", className, itemIdx),
							Message:  "Given item must have 'item' field with string value",
						})
					}

					if quantity, ok := itemMap["quantity"].(float64); ok {
						if quantity < 1 {
							issues = append(issues, Issue{
								Type:     "error",
								Category: "starting-gear",
								File:     "starting-gear.json",
								Field:    fmt.Sprintf("%s.given_items[%d].quantity", className, itemIdx),
								Message:  "Quantity must be at least 1",
							})
						}
					} else {
						issues = append(issues, Issue{
							Type:     "error",
							Category: "starting-gear",
							File:     "starting-gear.json",
							Field:    fmt.Sprintf("%s.given_items[%d].quantity", className, itemIdx),
							Message:  "Missing or invalid 'quantity' field",
						})
					}
				} else {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "starting-gear",
						File:     "starting-gear.json",
						Field:    fmt.Sprintf("%s.given_items[%d]", className, itemIdx),
						Message:  "Given item must be an object with 'item' and 'quantity' fields",
					})
				}
			}
		}
	}

	return issues, nil
}
// Helper function to validate an equipment choice
func validateEquipmentChoice(choice map[string]interface{}, className string, choiceIdx int, validItemIDs map[string]bool, issues *[]Issue) {
	options, ok := choice["options"].([]interface{})
	if !ok {
		*issues = append(*issues, Issue{
			Type:     "error",
			Category: "starting-gear",
			File:     "starting-gear.json",
			Field:    fmt.Sprintf("%s.equipment_choices[%d].options", className, choiceIdx),
			Message:  "Missing or invalid 'options' array",
		})
		return
	}

	for optIdx, option := range options {
		if optMap, ok := option.(map[string]interface{}); ok {
			optType, _ := optMap["type"].(string)

			switch optType {
			case "single":
				validateSingleOption(optMap, className, choiceIdx, optIdx, validItemIDs, issues)
			case "bundle":
				validateBundleOption(optMap, className, choiceIdx, optIdx, validItemIDs, issues)
			case "multi_slot":
				validateMultiSlotOption(optMap, className, choiceIdx, optIdx, validItemIDs, issues)
			default:
				*issues = append(*issues, Issue{
					Type:     "error",
					Category: "starting-gear",
					File:     "starting-gear.json",
					Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].type", className, choiceIdx, optIdx),
					Message:  fmt.Sprintf("Invalid option type '%s'. Must be 'single', 'bundle', or 'multi_slot'", optType),
				})
			}
		} else {
			*issues = append(*issues, Issue{
				Type:     "error",
				Category: "starting-gear",
				File:     "starting-gear.json",
				Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d]", className, choiceIdx, optIdx),
				Message:  "Option must be an object",
			})
		}
	}
}

// Validate single option
func validateSingleOption(opt map[string]interface{}, className string, choiceIdx, optIdx int, validItemIDs map[string]bool, issues *[]Issue) {
	itemID, ok := opt["item"].(string)
	if !ok {
		*issues = append(*issues, Issue{
			Type:     "error",
			Category: "starting-gear",
			File:     "starting-gear.json",
			Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].item", className, choiceIdx, optIdx),
			Message:  "Single option must have 'item' field with string value",
		})
		return
	}

	if !validItemIDs[itemID] {
		*issues = append(*issues, Issue{
			Type:     "error",
			Category: "starting-gear",
			File:     "starting-gear.json",
			Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].item", className, choiceIdx, optIdx),
			Message:  fmt.Sprintf("Invalid item ID '%s'", itemID),
		})
	}

	if quantity, ok := opt["quantity"].(float64); ok {
		if quantity < 1 {
			*issues = append(*issues, Issue{
				Type:     "error",
				Category: "starting-gear",
				File:     "starting-gear.json",
				Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].quantity", className, choiceIdx, optIdx),
				Message:  "Quantity must be at least 1",
			})
		}
	} else {
		*issues = append(*issues, Issue{
			Type:     "error",
			Category: "starting-gear",
			File:     "starting-gear.json",
			Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].quantity", className, choiceIdx, optIdx),
			Message:  "Missing or invalid 'quantity' field",
		})
	}
}

// Validate bundle option
func validateBundleOption(opt map[string]interface{}, className string, choiceIdx, optIdx int, validItemIDs map[string]bool, issues *[]Issue) {
	items, ok := opt["items"].([]interface{})
	if !ok {
		*issues = append(*issues, Issue{
			Type:     "error",
			Category: "starting-gear",
			File:     "starting-gear.json",
			Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].items", className, choiceIdx, optIdx),
			Message:  "Bundle option must have 'items' array",
		})
		return
	}

	for itemIdx, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if itemID, ok := itemMap["item"].(string); ok {
				if !validItemIDs[itemID] {
					*issues = append(*issues, Issue{
						Type:     "error",
						Category: "starting-gear",
						File:     "starting-gear.json",
						Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].items[%d].item", className, choiceIdx, optIdx, itemIdx),
						Message:  fmt.Sprintf("Invalid item ID '%s'", itemID),
					})
				}
			} else {
				*issues = append(*issues, Issue{
					Type:     "error",
					Category: "starting-gear",
					File:     "starting-gear.json",
					Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].items[%d].item", className, choiceIdx, optIdx, itemIdx),
					Message:  "Bundle item must have 'item' field with string value",
				})
			}

			if quantity, ok := itemMap["quantity"].(float64); ok {
				if quantity < 1 {
					*issues = append(*issues, Issue{
						Type:     "error",
						Category: "starting-gear",
						File:     "starting-gear.json",
						Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].items[%d].quantity", className, choiceIdx, optIdx, itemIdx),
						Message:  "Quantity must be at least 1",
					})
				}
			} else {
				*issues = append(*issues, Issue{
					Type:     "error",
					Category: "starting-gear",
					File:     "starting-gear.json",
					Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].items[%d].quantity", className, choiceIdx, optIdx, itemIdx),
					Message:  "Missing or invalid 'quantity' field",
				})
			}
		} else {
			*issues = append(*issues, Issue{
				Type:     "error",
				Category: "starting-gear",
				File:     "starting-gear.json",
				Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].items[%d]", className, choiceIdx, optIdx, itemIdx),
				Message:  "Bundle item must be an object with 'item' and 'quantity' fields",
			})
		}
	}
}

// Validate multi_slot option
func validateMultiSlotOption(opt map[string]interface{}, className string, choiceIdx, optIdx int, validItemIDs map[string]bool, issues *[]Issue) {
	slots, ok := opt["slots"].([]interface{})
	if !ok {
		*issues = append(*issues, Issue{
			Type:     "error",
			Category: "starting-gear",
			File:     "starting-gear.json",
			Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots", className, choiceIdx, optIdx),
			Message:  "Multi-slot option must have 'slots' array",
		})
		return
	}

	for slotIdx, slot := range slots {
		if slotMap, ok := slot.(map[string]interface{}); ok {
			slotType, _ := slotMap["type"].(string)

			switch slotType {
			case "weapon_choice":
				if options, ok := slotMap["options"].([]interface{}); ok {
					for wepIdx, wep := range options {
						if wepID, ok := wep.(string); ok {
							if !validItemIDs[wepID] {
								*issues = append(*issues, Issue{
									Type:     "error",
									Category: "starting-gear",
									File:     "starting-gear.json",
									Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d].options[%d]", className, choiceIdx, optIdx, slotIdx, wepIdx),
									Message:  fmt.Sprintf("Invalid weapon ID '%s'", wepID),
								})
							}
						} else {
							*issues = append(*issues, Issue{
								Type:     "error",
								Category: "starting-gear",
								File:     "starting-gear.json",
								Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d].options[%d]", className, choiceIdx, optIdx, slotIdx, wepIdx),
								Message:  "Weapon choice option must be a string (item ID)",
							})
						}
					}
				} else {
					*issues = append(*issues, Issue{
						Type:     "error",
						Category: "starting-gear",
						File:     "starting-gear.json",
						Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d].options", className, choiceIdx, optIdx, slotIdx),
						Message:  "Weapon choice must have 'options' array of item IDs",
					})
				}

			case "fixed":
				if itemID, ok := slotMap["item"].(string); ok {
					if !validItemIDs[itemID] {
						*issues = append(*issues, Issue{
							Type:     "error",
							Category: "starting-gear",
							File:     "starting-gear.json",
							Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d].item", className, choiceIdx, optIdx, slotIdx),
							Message:  fmt.Sprintf("Invalid item ID '%s'", itemID),
						})
					}
				} else {
					*issues = append(*issues, Issue{
						Type:     "error",
						Category: "starting-gear",
						File:     "starting-gear.json",
						Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d].item", className, choiceIdx, optIdx, slotIdx),
						Message:  "Fixed slot must have 'item' field with string value",
					})
				}

				if quantity, ok := slotMap["quantity"].(float64); ok {
					if quantity < 1 {
						*issues = append(*issues, Issue{
							Type:     "error",
							Category: "starting-gear",
							File:     "starting-gear.json",
							Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d].quantity", className, choiceIdx, optIdx, slotIdx),
							Message:  "Quantity must be at least 1",
						})
					}
				} else {
					*issues = append(*issues, Issue{
						Type:     "error",
						Category: "starting-gear",
						File:     "starting-gear.json",
						Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d].quantity", className, choiceIdx, optIdx, slotIdx),
						Message:  "Missing or invalid 'quantity' field",
					})
				}

			default:
				*issues = append(*issues, Issue{
					Type:     "error",
					Category: "starting-gear",
					File:     "starting-gear.json",
					Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d].type", className, choiceIdx, optIdx, slotIdx),
					Message:  fmt.Sprintf("Invalid slot type '%s'. Must be 'weapon_choice' or 'fixed'", slotType),
				})
			}
		} else {
			*issues = append(*issues, Issue{
				Type:     "error",
				Category: "starting-gear",
				File:     "starting-gear.json",
				Field:    fmt.Sprintf("%s.equipment_choices[%d].options[%d].slots[%d]", className, choiceIdx, optIdx, slotIdx),
				Message:  "Slot must be an object",
			})
		}
	}
}

// ValidateEffects validates all effect files against the new schema
func ValidateEffects() ([]Issue, error) {
	issues := []Issue{}
	effectsPath := "game-data/effects"

	// Load effect types for validation
	effectTypes, err := loadEffectTypes("game-data/systems/effects.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load effect types: %w", err)
	}

	// Find all effect files
	err = filepath.WalkDir(effectsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			effectIssues := validateEffectFile(path, effectTypes)
			issues = append(issues, effectIssues...)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return issues, nil
}

func loadEffectTypes(path string) (map[string]effectTypeInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var wrapper struct {
		EffectTypes map[string]effectTypeInfo `json:"effect_types"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	return wrapper.EffectTypes, nil
}

type effectTypeInfo struct {
	ID             string `json:"id"`
	Property       string `json:"property"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	AllowsPeriodic bool   `json:"allows_periodic"`
}

func validateEffectFile(filePath string, effectTypes map[string]effectTypeInfo) []Issue {
	issues := []Issue{}
	filename := filepath.Base(filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "effects",
			File:     filename,
			Message:  fmt.Sprintf("Failed to read file: %v", err),
		})
		return issues
	}

	var effect map[string]interface{}
	if err := json.Unmarshal(data, &effect); err != nil {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "effects",
			File:     filename,
			Message:  fmt.Sprintf("Failed to parse JSON: %v", err),
		})
		return issues
	}

	// Rule 1: Required fields
	requiredFields := []string{"id", "name", "description"}
	for _, field := range requiredFields {
		if _, exists := effect[field]; !exists {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "effects",
				File:     filename,
				Field:    field,
				Message:  fmt.Sprintf("Required field '%s' is missing", field),
			})
		}
	}

	// Check for new structure vs old structure
	hasModifiers := false
	if modifiers, ok := effect["modifiers"].([]interface{}); ok && len(modifiers) > 0 {
		hasModifiers = true
	}

	hasOldStructure := false
	if effects, ok := effect["effects"].([]interface{}); ok && len(effects) > 0 {
		hasOldStructure = true
	}

	// Check for new required fields
	if !hasModifiers {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "effects",
			File:     filename,
			Field:    "modifiers",
			Message:  "Required field 'modifiers' is missing or empty (use 'modifiers' instead of 'effects')",
		})
	}

	if _, exists := effect["source_type"]; !exists {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "effects",
			File:     filename,
			Field:    "source_type",
			Message:  "Required field 'source_type' is missing (must be 'system_ticker', 'system_status', or 'applied')",
		})
	}

	if _, exists := effect["category"]; !exists {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "effects",
			File:     filename,
			Field:    "category",
			Message:  "Required field 'category' is missing (must be 'buff', 'debuff', or 'status')",
		})
	}

	if _, exists := effect["removal"]; !exists {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "effects",
			File:     filename,
			Field:    "removal",
			Message:  "Required field 'removal' is missing",
		})
	}

	if _, exists := effect["visible"]; !exists {
		issues = append(issues, Issue{
			Type:     "error",
			Category: "effects",
			File:     filename,
			Field:    "visible",
			Message:  "Required field 'visible' is missing",
		})
	}

	// Rule 2: source_type validation
	if sourceType, ok := effect["source_type"].(string); ok {
		validSourceTypes := map[string]bool{
			"system_ticker": true,
			"system_status": true,
			"applied":       true,
		}
		if !validSourceTypes[sourceType] {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "effects",
				File:     filename,
				Field:    "source_type",
				Message:  fmt.Sprintf("Invalid source_type '%s' (must be 'system_ticker', 'system_status', or 'applied')", sourceType),
			})
		}

		// NEW RULE: system_status effects must have system_check
		if sourceType == "system_status" {
			if systemCheck, ok := effect["system_check"].(map[string]interface{}); !ok {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    "system_check",
					Message:  "system_status effects must have 'system_check' defined",
				})
			} else {
				// Validate system_check fields
				validStats := map[string]bool{"hunger": true, "fatigue": true, "weight_percent": true, "hp_percent": true, "mana_percent": true}
				validOperators := map[string]bool{"==": true, "!=": true, "<": true, "<=": true, ">": true, ">=": true}

				if stat, ok := systemCheck["stat"].(string); !ok || stat == "" {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "effects",
						File:     filename,
						Field:    "system_check.stat",
						Message:  "system_check must have 'stat' field",
					})
				} else if !validStats[stat] {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "effects",
						File:     filename,
						Field:    "system_check.stat",
						Message:  fmt.Sprintf("Invalid stat '%s' (must be: hunger, fatigue, weight_percent, hp_percent, mana_percent)", stat),
					})
				} else {
					// Validate value range based on stat
					if value, ok := systemCheck["value"].(float64); ok {
						intValue := int(value)
						switch stat {
						case "hunger":
							if intValue < 0 || intValue > 3 {
								issues = append(issues, Issue{
									Type:     "error",
									Category: "effects",
									File:     filename,
									Field:    "system_check.value",
									Message:  "hunger value must be 0-3",
								})
							}
						case "fatigue":
							if intValue < 0 || intValue > 10 {
								issues = append(issues, Issue{
									Type:     "error",
									Category: "effects",
									File:     filename,
									Field:    "system_check.value",
									Message:  "fatigue value must be 0-10",
								})
							}
						case "weight_percent":
							if intValue < 0 || intValue > 300 {
								issues = append(issues, Issue{
									Type:     "error",
									Category: "effects",
									File:     filename,
									Field:    "system_check.value",
									Message:  "weight_percent value must be 0-300",
								})
							}
						case "hp_percent", "mana_percent":
							if intValue < 0 || intValue > 100 {
								issues = append(issues, Issue{
									Type:     "error",
									Category: "effects",
									File:     filename,
									Field:    "system_check.value",
									Message:  fmt.Sprintf("%s value must be 0-100", stat),
								})
							}
						}
					}
				}

				if operator, ok := systemCheck["operator"].(string); !ok || operator == "" {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "effects",
						File:     filename,
						Field:    "system_check.operator",
						Message:  "system_check must have 'operator' field",
					})
				} else if !validOperators[operator] {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "effects",
						File:     filename,
						Field:    "system_check.operator",
						Message:  fmt.Sprintf("Invalid operator '%s' (must be: ==, !=, <, <=, >, >=)", operator),
					})
				}

				if _, ok := systemCheck["value"]; !ok {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "effects",
						File:     filename,
						Field:    "system_check.value",
						Message:  "system_check must have 'value' field",
					})
				}
			}
		}

		// Rule 4 & 5: visible field validation
		if visible, ok := effect["visible"].(bool); ok {
			if sourceType == "system_ticker" && visible {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    "visible",
					Message:  "system_ticker effects must have visible=false",
				})
			}
			if (sourceType == "system_status" || sourceType == "applied") && !visible {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    "visible",
					Message:  fmt.Sprintf("%s effects must have visible=true", sourceType),
				})
			}
		}

		// Rule 10: Applied effects should have message
		if sourceType == "applied" {
			if message, ok := effect["message"].(string); !ok || strings.TrimSpace(message) == "" {
				issues = append(issues, Issue{
					Type:     "warning",
					Category: "effects",
					File:     filename,
					Field:    "message",
					Message:  "Applied effects should have a message field",
				})
			}
		}
	}

	// Rule 3: category validation
	if category, ok := effect["category"].(string); ok {
		validCategories := map[string]bool{
			"buff":   true,
			"debuff": true,
			"status": true,
		}
		if !validCategories[category] {
			issues = append(issues, Issue{
				Type:     "error",
				Category: "effects",
				File:     filename,
				Field:    "category",
				Message:  fmt.Sprintf("Invalid category '%s' (must be 'buff', 'debuff', or 'status')", category),
			})
		}
	}

	// Rule 6-9: Modifier validation
	if modifiers, ok := effect["modifiers"].([]interface{}); ok {
		periodicCount := make(map[string]int) // Track periodic modifiers per stat

		for i, mod := range modifiers {
			modMap, ok := mod.(map[string]interface{})
			if !ok {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    fmt.Sprintf("modifiers[%d]", i),
					Message:  "Modifier must be an object",
				})
				continue
			}

			// Validate stat type exists
			stat, ok := modMap["stat"].(string)
			if !ok {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    fmt.Sprintf("modifiers[%d].stat", i),
					Message:  "Modifier must have 'stat' field",
				})
				continue
			}

			effectType, exists := effectTypes[stat]
			if !exists {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    fmt.Sprintf("modifiers[%d].stat", i),
					Message:  fmt.Sprintf("Unknown stat type '%s'", stat),
				})
				continue
			}

			// Validate modifier type
			modType, ok := modMap["type"].(string)
			if !ok {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    fmt.Sprintf("modifiers[%d].type", i),
					Message:  "Modifier must have 'type' field",
				})
				continue
			}

			validModTypes := map[string]bool{
				"instant":  true,
				"constant": true,
				"periodic": true,
			}
			if !validModTypes[modType] {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    fmt.Sprintf("modifiers[%d].type", i),
					Message:  fmt.Sprintf("Invalid modifier type '%s' (must be 'instant', 'constant', or 'periodic')", modType),
				})
			}

			// Rule 7: Validate modifier type based on category
			switch effectType.Category {
			case "stat", "capacity":
				// Stats and capacities should ONLY use constant type
				if modType != "constant" {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "effects",
						File:     filename,
						Field:    fmt.Sprintf("modifiers[%d].type", i),
						Message:  fmt.Sprintf("Stat/capacity '%s' should use type 'constant' (not '%s')", stat, modType),
					})
				}
			case "resource":
				// Resources can use instant, constant, or periodic
				// (already validated above that it's one of these three)

				// Resources should NOT use constant for direct modifications
				// (instant for one-time, periodic for repeating)
				if modType == "constant" {
					issues = append(issues, Issue{
						Type:     "warning",
						Category: "effects",
						File:     filename,
						Field:    fmt.Sprintf("modifiers[%d].type", i),
						Message:  fmt.Sprintf("Resource '%s' using type 'constant' - are you sure? Usually resources use 'instant' or 'periodic'", stat),
					})
				}
			}

			// Rule 8: Periodic modifiers need tick_interval
			if modType == "periodic" {
				if tickInterval, ok := modMap["tick_interval"].(float64); !ok || tickInterval <= 0 {
					issues = append(issues, Issue{
						Type:     "error",
						Category: "effects",
						File:     filename,
						Field:    fmt.Sprintf("modifiers[%d].tick_interval", i),
						Message:  "Periodic modifiers must have tick_interval > 0",
					})
				}
			}

			// Rule 9: Non-periodic modifiers should NOT have tick_interval
			if modType != "periodic" {
				if _, hasTick := modMap["tick_interval"]; hasTick {
					issues = append(issues, Issue{
						Type:     "warning",
						Category: "effects",
						File:     filename,
						Field:    fmt.Sprintf("modifiers[%d].tick_interval", i),
						Message:  fmt.Sprintf("Modifier type '%s' should not have tick_interval (only periodic modifiers need this)", modType),
					})
				}
			}

			// Count periodic modifiers per stat
			if modType == "periodic" {
				periodicCount[stat]++
			}

			// Rule 13: Delay only makes sense for applied effects
			if delay, ok := modMap["delay"].(float64); ok && delay > 0 {
				if sourceType, ok := effect["source_type"].(string); ok && sourceType != "applied" {
					issues = append(issues, Issue{
						Type:     "warning",
						Category: "effects",
						File:     filename,
						Field:    fmt.Sprintf("modifiers[%d].delay", i),
						Message:  fmt.Sprintf("Delay field is unusual for %s effects (typically only used for applied effects)", sourceType),
					})
				}
			}
		}

		// Rule 6: Only one periodic modifier per stat per effect
		for stat, count := range periodicCount {
			if count > 1 {
				issues = append(issues, Issue{
					Type:     "error",
					Category: "effects",
					File:     filename,
					Field:    "modifiers",
					Message:  fmt.Sprintf("Multiple periodic modifiers for stat '%s' (max: 1 per stat)", stat),
				})
			}
		}
	}

	// Rule 11-14: Deprecated fields
	if _, exists := effect["icon"]; exists {
		issues = append(issues, Issue{
			Type:     "warning",
			Category: "effects",
			File:     filename,
			Field:    "icon",
			Message:  "Deprecated field 'icon' found (frontend derives icon from effect ID)",
		})
	}

	if _, exists := effect["color"]; exists {
		issues = append(issues, Issue{
			Type:     "warning",
			Category: "effects",
			File:     filename,
			Field:    "color",
			Message:  "Deprecated field 'color' found (frontend uses hardcoded colors)",
		})
	}

	if _, exists := effect["silent"]; exists {
		issues = append(issues, Issue{
			Type:     "warning",
			Category: "effects",
			File:     filename,
			Field:    "silent",
			Message:  "Deprecated field 'silent' found (use 'visible' instead)",
		})
	}

	if hasOldStructure {
		issues = append(issues, Issue{
			Type:     "warning",
			Category: "effects",
			File:     filename,
			Field:    "effects",
			Message:  "Deprecated field 'effects' found (use 'modifiers' instead)",
		})
	}

	return issues
}

package character

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/types"
)

// ============================================================================
// STARTING GEAR LOADING
// ============================================================================

func loadStartingGearForClass(database *sql.DB, class string) (*StartingGearData, error) {
	var dataJSON string
	err := database.QueryRow("SELECT data FROM starting_gear WHERE id = 'starting-gear'").Scan(&dataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query starting gear from database: %v", err)
	}

	var allGear []StartingGearData
	if err := json.Unmarshal([]byte(dataJSON), &allGear); err != nil {
		return nil, fmt.Errorf("failed to parse starting gear data: %v", err)
	}

	for _, gear := range allGear {
		if gear.Class == class {
			return &gear, nil
		}
	}

	return nil, fmt.Errorf("no starting gear found for class: %s", class)
}

// ============================================================================
// INVENTORY BUILDING
// ============================================================================

func buildInventoryFromChoices(database *sql.DB, startingGear *StartingGearData, choices map[string]string, packChoice string) (map[string]interface{}, error) {
	// 1. Collect all items from given_items + player choices
	allItems := []ItemWithQty{}

	// Add given items
	allItems = append(allItems, startingGear.StartingGear.GivenItems...)

	// Log what we received
	log.Printf("📦 Received equipment choices: %+v", choices)
	log.Printf("📦 Pack choice: %s", packChoice)

	// Add selected equipment
	for i, equipChoice := range startingGear.StartingGear.EquipmentChoices {
		choiceKey := fmt.Sprintf("choice-%d", i)
		selectedID := choices[choiceKey]

		if selectedID == "" {
			log.Printf("⚠️  No selection for %s (available keys: %v)", choiceKey, getKeys(choices))
			continue
		}

		// Check if it's a JSON array (complex weapon choice OR bundle)
		if len(selectedID) > 0 && selectedID[0] == '[' {
			log.Printf("📦 Parsing JSON array for %s: %s", choiceKey, selectedID)
			var itemList [][]interface{}
			if err := json.Unmarshal([]byte(selectedID), &itemList); err == nil {
				// Successfully parsed as array
				log.Printf("✅ Parsed %d items from JSON array", len(itemList))
				for _, itemPair := range itemList {
					if len(itemPair) >= 2 {
						itemID, ok1 := itemPair[0].(string)
						qty, ok2 := itemPair[1].(float64)
						if ok1 && ok2 {
							log.Printf("  - Adding %s x%d", itemID, int(qty))
							allItems = append(allItems, ItemWithQty{Item: itemID, Quantity: int(qty)})
						}
					}
				}
				continue
			} else {
				log.Printf("❌ Failed to parse JSON array: %v", err)
			}
		}

		// Find the selected option
		for _, option := range equipChoice.Options {
			if option.Type == "single" && option.Item == selectedID {
				allItems = append(allItems, ItemWithQty{Item: option.Item, Quantity: option.Quantity})
				break
			} else if option.Type == "bundle" {
				// Check if this bundle's first item matches (simplified check)
				if len(option.Items) > 0 && option.Items[0].Item == selectedID {
					allItems = append(allItems, option.Items...)
					break
				}
			}
		}
	}

	// Add pack choice
	if packChoice != "" {
		allItems = append(allItems, ItemWithQty{Item: packChoice, Quantity: 1})
	}

	// 2. Stack items (respect stack limits)
	stackedItems, err := stackItems(database, allItems)
	if err != nil {
		return nil, fmt.Errorf("failed to stack items: %v", err)
	}

	// 3. Build inventory structure with auto-equipping
	inventory, err := createInventoryStructure(database, stackedItems)
	if err != nil {
		return nil, fmt.Errorf("failed to create inventory: %v", err)
	}

	return inventory, nil
}

// ============================================================================
// ITEM STACKING
// ============================================================================

func stackItems(database *sql.DB, items []ItemWithQty) ([]ItemWithQty, error) {
	stacked := []ItemWithQty{}

	for _, item := range items {
		// Get item data to check stack limit
		stackLimit, err := getItemStackLimit(database, item.Item)
		if err != nil {
			log.Printf("⚠️  Failed to get stack limit for %s: %v", item.Item, err)
			stackLimit = 1 // Default
		}

		remainingQty := item.Quantity

		// Try to add to existing stacks
		for i := range stacked {
			if stacked[i].Item == item.Item && stacked[i].Quantity < stackLimit {
				canAdd := min(remainingQty, stackLimit-stacked[i].Quantity)
				stacked[i].Quantity += canAdd
				remainingQty -= canAdd

				if remainingQty <= 0 {
					break
				}
			}
		}

		// Create new stacks for remaining quantity
		for remainingQty > 0 {
			stackSize := min(remainingQty, stackLimit)
			stacked = append(stacked, ItemWithQty{Item: item.Item, Quantity: stackSize})
			remainingQty -= stackSize
		}
	}

	return stacked, nil
}

func getItemStackLimit(database *sql.DB, itemID string) (int, error) {
	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM items WHERE id = ?", itemID).Scan(&propertiesJSON)
	if err != nil {
		return 1, err
	}

	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
		return 1, err
	}

	if stack, ok := properties["stack"].(float64); ok {
		return int(stack), nil
	}

	return 1, nil
}

// ============================================================================
// INVENTORY STRUCTURE CREATION
// ============================================================================

func createInventoryStructure(database *sql.DB, items []ItemWithQty) (map[string]interface{}, error) {
	inventory := map[string]interface{}{
		"general_slots": []any{
			map[string]any{"slot": 0, "item": nil, "quantity": 0},
			map[string]any{"slot": 1, "item": nil, "quantity": 0},
			map[string]any{"slot": 2, "item": nil, "quantity": 0},
			map[string]any{"slot": 3, "item": nil, "quantity": 0},
		},
		"gear_slots": map[string]interface{}{
			"neck":     map[string]interface{}{"item": nil, "quantity": 0},
			"head":     map[string]interface{}{"item": nil, "quantity": 0},
			"ammo":     map[string]interface{}{"item": nil, "quantity": 0},
			"mainhand": map[string]interface{}{"item": nil, "quantity": 0},
			"chest":    map[string]interface{}{"item": nil, "quantity": 0},
			"offhand":  map[string]interface{}{"item": nil, "quantity": 0},
			"ring1":    map[string]interface{}{"item": nil, "quantity": 0},
			"legs":     map[string]interface{}{"item": nil, "quantity": 0},
			"ring2":    map[string]interface{}{"item": nil, "quantity": 0},
			"gloves":   map[string]interface{}{"item": nil, "quantity": 0},
			"boots":    map[string]interface{}{"item": nil, "quantity": 0},
			"bag":      map[string]interface{}{"item": nil, "quantity": 0},
		},
	}

	gearSlots := inventory["gear_slots"].(map[string]interface{})
	generalSlots := inventory["general_slots"].([]any)

	remainingItems := []ItemWithQty{}
	twoHandedEquipped := false

	// 0. Unpack armor sets into individual pieces first
	expandedItems := []ItemWithQty{}
	for _, item := range items {
		if isArmorSet(database, item.Item) {
			pieces, err := unpackArmorSet(database, item.Item)
			if err != nil {
				log.Printf("⚠️  Failed to unpack armor set %s: %v", item.Item, err)
				expandedItems = append(expandedItems, item)
			} else {
				expandedItems = append(expandedItems, pieces...)
			}
		} else {
			expandedItems = append(expandedItems, item)
		}
	}
	items = expandedItems

	// 1. Handle packs first (auto-unpack to bag slot)
	for _, item := range items {
		if isPackItem(item.Item) {
			contents, err := unpackItem(database, item.Item)
			if err != nil {
				log.Printf("⚠️  Failed to unpack %s: %v", item.Item, err)
				continue
			}

			gearSlots["bag"] = map[string]interface{}{
				"item":     "backpack",
				"quantity": 1,
				"contents": contents,
			}
		} else {
			remainingItems = append(remainingItems, item)
		}
	}

	// 2. Auto-equip equipment items (process in forward order - earlier choices have priority)
	equippedIndices := []int{} // Track which items were equipped
	for i := 0; i < len(remainingItems); i++ {
		item := remainingItems[i]
		itemData, err := getItemData(database, item.Item)
		if err != nil {
			log.Printf("⚠️  Failed to get item data for %s: %v", item.Item, err)
			continue
		}

		gearSlot := getGearSlot(itemData)
		if gearSlot == "" {
			continue // Not equipment
		}

		equipped := false

		switch gearSlot {
		case "chest", "neck", "head", "legs", "gloves", "boots", "ammo":
			if gearSlots[gearSlot].(map[string]interface{})["item"] == nil {
				gearSlots[gearSlot] = map[string]interface{}{
					"item":     item.Item,
					"quantity": item.Quantity,
				}
				log.Printf("✅ Equipped %s to %s slot", item.Item, gearSlot)
				equipped = true
			}

		case "ring":
			// Try ring1 first, then ring2
			if gearSlots["ring1"].(map[string]interface{})["item"] == nil {
				gearSlots["ring1"] = map[string]interface{}{
					"item":     item.Item,
					"quantity": item.Quantity,
				}
				log.Printf("✅ Equipped %s to ring1 slot", item.Item)
				equipped = true
			} else if gearSlots["ring2"].(map[string]interface{})["item"] == nil {
				gearSlots["ring2"] = map[string]interface{}{
					"item":     item.Item,
					"quantity": item.Quantity,
				}
				log.Printf("✅ Equipped %s to ring2 slot", item.Item)
				equipped = true
			}

		case "hands":
			isTwoHanded := hasTags(itemData, []string{"two-handed"})

			if isTwoHanded {
				if gearSlots["mainhand"].(map[string]interface{})["item"] == nil {
					gearSlots["mainhand"] = map[string]interface{}{
						"item":     item.Item,
						"quantity": item.Quantity,
					}
					log.Printf("✅ Equipped two-handed %s to mainhand", item.Item)
					twoHandedEquipped = true
					equipped = true
				}
			} else {
				if gearSlots["mainhand"].(map[string]interface{})["item"] == nil {
					gearSlots["mainhand"] = map[string]interface{}{
						"item":     item.Item,
						"quantity": item.Quantity,
					}
					log.Printf("✅ Equipped %s to mainhand", item.Item)
					equipped = true
				} else if gearSlots["offhand"].(map[string]interface{})["item"] == nil && !twoHandedEquipped {
					gearSlots["offhand"] = map[string]interface{}{
						"item":     item.Item,
						"quantity": item.Quantity,
					}
					log.Printf("✅ Equipped %s to offhand", item.Item)
					equipped = true
				}
			}

		case "mainhand":
			if gearSlots["mainhand"].(map[string]interface{})["item"] == nil {
				gearSlots["mainhand"] = map[string]interface{}{
					"item":     item.Item,
					"quantity": item.Quantity,
				}
				log.Printf("✅ Equipped %s to mainhand slot", item.Item)
				equipped = true
			}

		case "offhand":
			if gearSlots["offhand"].(map[string]interface{})["item"] == nil && !twoHandedEquipped {
				gearSlots["offhand"] = map[string]interface{}{
					"item":     item.Item,
					"quantity": item.Quantity,
				}
				log.Printf("✅ Equipped %s to offhand slot", item.Item)
				equipped = true
			}
		}

		if equipped {
			equippedIndices = append(equippedIndices, i)
		}
	}

	// Remove equipped items in reverse order to maintain indices
	for i := len(equippedIndices) - 1; i >= 0; i-- {
		idx := equippedIndices[i]
		remainingItems = append(remainingItems[:idx], remainingItems[idx+1:]...)
	}

	// 3. Put remaining items in general slots or backpack
	generalSlotIndex := 0
	for _, item := range remainingItems {
		itemData, _ := getItemData(database, item.Item)
		isContainer := hasTags(itemData, []string{"container"})

		// Containers go in general slots, not in bag
		if isContainer {
			if generalSlotIndex < 4 {
				// Initialize container with empty contents array
				containerSlots := 10 // default

				// Check for container_slots in properties first, then root level
				// JSON numbers unmarshal as float64, but also check int for robustness
				if properties, ok := itemData["properties"].(map[string]interface{}); ok {
					if val, ok := properties["container_slots"].(float64); ok {
						containerSlots = int(val)
					} else if val, ok := properties["container_slots"].(int); ok {
						containerSlots = val
					}
				} else if val, ok := itemData["container_slots"].(float64); ok {
					containerSlots = int(val)
				} else if val, ok := itemData["container_slots"].(int); ok {
					containerSlots = val
				}

				contents := make([]map[string]interface{}, containerSlots)
				for i := 0; i < containerSlots; i++ {
					contents[i] = map[string]interface{}{
						"slot":     i,
						"item":     nil,
						"quantity": 0,
					}
				}

				generalSlots[generalSlotIndex] = map[string]interface{}{
					"slot":     generalSlotIndex,
					"item":     item.Item,
					"quantity": item.Quantity,
					"contents": contents,
				}
				generalSlotIndex++
			}
		} else {
			// Try to add to backpack first
			if bag, ok := gearSlots["bag"].(map[string]interface{}); ok && bag["item"] != nil {
				if contents, ok := bag["contents"].([]map[string]interface{}); ok {
					placed := false
					for i, slot := range contents {
						if slot["item"] == nil {
							contents[i] = map[string]interface{}{
								"slot":     slot["slot"],
								"item":     item.Item,
								"quantity": item.Quantity,
							}
							placed = true
							break
						}
					}
					if placed {
						continue
					}
				}
			}

			// Backpack full or doesn't exist, use general slots
			if generalSlotIndex < 4 {
				generalSlots[generalSlotIndex] = map[string]interface{}{
					"slot":     generalSlotIndex,
					"item":     item.Item,
					"quantity": item.Quantity,
				}
				generalSlotIndex++
			}
		}
	}

	inventory["general_slots"] = generalSlots
	inventory["gear_slots"] = gearSlots

	return inventory, nil
}

// ============================================================================
// PACK UNPACKING
// ============================================================================

func isPackItem(itemID string) bool {
	return itemID == "explorers-pack" || itemID == "priests-pack" ||
		itemID == "scholars-pack" || itemID == "dungeoneers-pack" ||
		itemID == "diplomats-pack" || itemID == "entertainers-pack" ||
		itemID == "burglars-pack" || itemID == "druid-pack"
}

func isArmorSet(database *sql.DB, itemID string) bool {
	itemData, err := getItemData(database, itemID)
	if err != nil {
		return false
	}
	return hasTags(itemData, []string{"armor-set"})
}

func unpackArmorSet(database *sql.DB, setID string) ([]ItemWithQty, error) {
	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM items WHERE id = ?", setID).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query armor set: %v", err)
	}

	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
		return nil, fmt.Errorf("failed to parse properties: %v", err)
	}

	contentsRaw, ok := properties["contents"]
	if !ok {
		return nil, fmt.Errorf("armor set has no contents field")
	}

	contentsArray, ok := contentsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("contents is not an array")
	}

	items := []ItemWithQty{}
	for _, itemRaw := range contentsArray {
		itemArray, ok := itemRaw.([]interface{})
		if !ok || len(itemArray) < 2 {
			continue
		}

		itemID, ok1 := itemArray[0].(string)
		qty, ok2 := itemArray[1].(float64)
		if !ok1 || !ok2 {
			continue
		}

		items = append(items, ItemWithQty{Item: itemID, Quantity: int(qty)})
	}

	log.Printf("📦 Unpacked armor set %s into %d pieces", setID, len(items))
	return items, nil
}

func unpackItem(database *sql.DB, packID string) ([]map[string]interface{}, error) {
	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM items WHERE id = ?", packID).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query pack: %v", err)
	}

	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
		return nil, fmt.Errorf("failed to parse properties: %v", err)
	}

	contentsRaw, ok := properties["contents"]
	if !ok {
		return nil, fmt.Errorf("pack has no contents field")
	}

	// Convert contents to [][]interface{}
	contentsArray, ok := contentsRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("contents is not an array")
	}

	slots := []map[string]interface{}{}
	slotIndex := 0

	for _, contentItem := range contentsArray {
		itemArray, ok := contentItem.([]interface{})
		if !ok || len(itemArray) < 2 {
			continue
		}

		itemID, ok1 := itemArray[0].(string)
		qty, ok2 := itemArray[1].(float64)
		if !ok1 || !ok2 {
			continue
		}

		// Skip backpack and pack items
		if itemID == "backpack" || isPackItem(itemID) {
			continue
		}

		slots = append(slots, map[string]interface{}{
			"slot":     slotIndex,
			"item":     itemID,
			"quantity": int(qty),
		})
		slotIndex++
	}

	// Fill remaining slots to 20
	for slotIndex < 20 {
		slots = append(slots, map[string]interface{}{
			"slot":     slotIndex,
			"item":     nil,
			"quantity": 0,
		})
		slotIndex++
	}

	return slots, nil
}

// ============================================================================
// ITEM HELPERS
// ============================================================================

func getItemData(database *sql.DB, itemID string) (map[string]interface{}, error) {
	var propertiesJSON, tagsJSON string
	var name, description, itemType, rarity string

	err := database.QueryRow(`
		SELECT name, description, item_type, properties, tags, rarity
		FROM items WHERE id = ?
	`, itemID).Scan(&name, &description, &itemType, &propertiesJSON, &tagsJSON, &rarity)

	if err != nil {
		return nil, err
	}

	var properties map[string]interface{}
	var tags []interface{}

	json.Unmarshal([]byte(propertiesJSON), &properties)
	json.Unmarshal([]byte(tagsJSON), &tags)

	return map[string]interface{}{
		"id":          itemID,
		"name":        name,
		"description": description,
		"item_type":   itemType,
		"properties":  properties,
		"tags":        tags,
		"rarity":      rarity,
	}, nil
}

func getGearSlot(itemData map[string]interface{}) string {
	if properties, ok := itemData["properties"].(map[string]interface{}); ok {
		if gearSlot, ok := properties["gear_slot"].(string); ok {
			return gearSlot
		}
	}
	return ""
}

func hasTags(itemData map[string]interface{}, searchTags []string) bool {
	tags, ok := itemData["tags"].([]interface{})
	if !ok {
		return false
	}

	for _, tag := range tags {
		tagStr, ok := tag.(string)
		if !ok {
			continue
		}
		for _, search := range searchTags {
			if tagStr == search {
				return true
			}
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getKeys(m map[string]string) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ============================================================================
// SPELL SLOTS AND KNOWN SPELLS
// ============================================================================

func generateSpellSlots(database *sql.DB, class string) (map[string]interface{}, error) {
	var dataJSON string
	err := database.QueryRow("SELECT data FROM spell_slots_progression WHERE id = 'spell-slots'").Scan(&dataJSON)
	if err != nil {
		return nil, err
	}

	// Parse as generic map first to skip the "description" field
	var rawData map[string]interface{}
	if err := json.Unmarshal([]byte(dataJSON), &rawData); err != nil {
		return nil, err
	}

	classKey := strings.ToLower(class)
	classDataRaw, ok := rawData[classKey]
	if !ok {
		return make(map[string]interface{}), nil // Not a caster
	}

	classData, ok := classDataRaw.(map[string]interface{})
	if !ok {
		return make(map[string]interface{}), nil
	}

	level1SlotsRaw, ok := classData["1"]
	if !ok {
		return make(map[string]interface{}), nil
	}

	level1Slots, ok := level1SlotsRaw.(map[string]interface{})
	if !ok {
		return make(map[string]interface{}), nil
	}

	spellSlots := make(map[string]interface{})

	for slotType, countRaw := range level1Slots {
		count, ok := countRaw.(float64) // JSON numbers are float64
		if !ok || count <= 0 {
			continue
		}

		slots := []map[string]interface{}{}
		for i := 0; i < int(count); i++ {
			slots = append(slots, map[string]interface{}{
				"slot":     i,
				"spell":    nil,
				"quantity": 0,
			})
		}
		spellSlots[slotType] = slots
	}

	return spellSlots, nil
}

func loadKnownSpells(database *sql.DB, class string) ([]string, error) {
	var dataJSON string
	err := database.QueryRow("SELECT data FROM starting_spells WHERE id = 'starting-spells'").Scan(&dataJSON)
	if err != nil {
		return nil, err
	}

	var allSpells map[string]map[string][]string
	if err := json.Unmarshal([]byte(dataJSON), &allSpells); err != nil {
		return nil, err
	}

	classKey := strings.ToLower(class)
	classSpells, ok := allSpells[classKey]
	if !ok {
		return []string{}, nil // Not a caster
	}

	knownSpells := []string{}

	if cantrips, ok := classSpells["cantrips"]; ok {
		knownSpells = append(knownSpells, cantrips...)
	}

	if level1, ok := classSpells["level1"]; ok {
		knownSpells = append(knownSpells, level1...)
	}

	return knownSpells, nil
}

// ============================================================================
// LOCATION HELPERS
// ============================================================================

func getStartingCityForRace(database *sql.DB, race string) (string, error) {
	var dataJSON string
	err := database.QueryRow("SELECT data FROM starting_locations WHERE id = 'starting-locations'").Scan(&dataJSON)
	if err != nil {
		return "millhaven", err
	}

	var racialCities struct {
		RacialStartingCities map[string]string `json:"racial_starting_cities"`
	}

	if err := json.Unmarshal([]byte(dataJSON), &racialCities); err != nil {
		return "millhaven", err
	}

	if city, ok := racialCities.RacialStartingCities[race]; ok {
		return city, nil
	}

	return "millhaven", nil
}

func getMusicTrackForLocation(database *sql.DB, locationID string) string {
	var dataJSON string
	err := database.QueryRow("SELECT data FROM music_tracks WHERE id = 'music'").Scan(&dataJSON)
	if err != nil {
		log.Printf("⚠️  Failed to query music tracks from database: %v", err)
		return ""
	}

	var musicData struct {
		Tracks []struct {
			Title      string  `json:"title"`
			File       string  `json:"file"`
			UnlocksAt  *string `json:"unlocks_at"`
			AutoUnlock bool    `json:"auto_unlock"`
		} `json:"tracks"`
	}

	if err := json.Unmarshal([]byte(dataJSON), &musicData); err != nil {
		log.Printf("⚠️  Failed to parse music data: %v", err)
		return ""
	}

	// Find track for this location
	for _, track := range musicData.Tracks {
		if track.UnlocksAt != nil && *track.UnlocksAt == locationID {
			return track.Title
		}
	}

	return ""
}

func getAutoUnlockMusicTracks(database *sql.DB) []string {
	var dataJSON string
	err := database.QueryRow("SELECT data FROM music_tracks WHERE id = 'music'").Scan(&dataJSON)
	if err != nil {
		log.Printf("⚠️  Failed to query music tracks from database: %v", err)
		return []string{}
	}

	var musicData struct {
		Tracks []struct {
			Title      string  `json:"title"`
			File       string  `json:"file"`
			UnlocksAt  *string `json:"unlocks_at"`
			AutoUnlock bool    `json:"auto_unlock"`
		} `json:"tracks"`
	}

	if err := json.Unmarshal([]byte(dataJSON), &musicData); err != nil {
		log.Printf("⚠️  Failed to parse music data: %v", err)
		return []string{}
	}

	tracks := []string{}
	for _, track := range musicData.Tracks {
		if track.AutoUnlock {
			tracks = append(tracks, track.Title)
		}
	}

	return tracks
}

// ============================================================================
// VAULT GENERATION
// ============================================================================

// startingVaultKeepers returns the vault doors a new character can already open:
// just the one in their home city, whose keeper counts them a local. The vault
// behind it is shared and starts empty (schema v4). Returns nil when the city
// has no vault building, which simply means the player registers elsewhere.
func startingVaultKeepers(database *sql.DB, locationID string) []string {
	buildingID, err := building.FindBuildingIDByType(database, locationID, "vault")
	if err != nil {
		log.Printf("⚠️ No vault building in starting city %s: %v", locationID, err)
		return nil
	}
	log.Printf("✅ Home vault keeper registered for new character: %s", buildingID)
	return []string{buildingID}
}

// ============================================================================
// STARTING GOLD
// ============================================================================

// AddGoldToInventory adds gold to the first available slot in inventory
func AddGoldToInventory(inventory map[string]interface{}, goldAmount int) error {
	log.Printf("💰 Adding %dg to inventory", goldAmount)

	// Try general_slots first (type is []any, not []map[string]interface{})
	generalSlotsRaw, ok := inventory["general_slots"].([]any)
	if !ok {
		log.Printf("❌ general_slots type assertion failed, got type: %T", inventory["general_slots"])
		return fmt.Errorf("invalid general_slots format")
	}

	// Try to find existing gold stack or empty slot in general slots
	for i, slotData := range generalSlotsRaw {
		slot, ok := slotData.(map[string]any)
		if !ok {
			continue
		}

		// Check if slot has gold already - add to it
		if slot["item"] == "gold-piece" {
			currentQty := 0
			if qty, ok := slot["quantity"].(float64); ok {
				currentQty = int(qty)
			} else if qty, ok := slot["quantity"].(int); ok {
				currentQty = qty
			}
			slot["quantity"] = currentQty + goldAmount
			log.Printf("✅ Added %dg to existing gold stack in general_slots[%d] (new total: %d)", goldAmount, i, currentQty+goldAmount)
			return nil
		}

		// Found empty slot - add gold here
		if slot["item"] == nil || slot["item"] == "" {
			slot["item"] = "gold-piece"
			slot["quantity"] = goldAmount
			log.Printf("✅ Added %dg to general_slots[%d]", goldAmount, i)
			return nil
		}
	}

	// If general slots are full, try backpack
	gearSlots, ok := inventory["gear_slots"].(map[string]any)
	if ok {
		bag, ok := gearSlots["bag"].(map[string]any)
		if ok {
			contents, ok := bag["contents"].([]any)
			if ok {
				for i, slotData := range contents {
					slot, ok := slotData.(map[string]any)
					if !ok {
						continue
					}

					// Check if slot has gold already - add to it
					if slot["item"] == "gold-piece" {
						currentQty := 0
						if qty, ok := slot["quantity"].(float64); ok {
							currentQty = int(qty)
						} else if qty, ok := slot["quantity"].(int); ok {
							currentQty = qty
						}
						slot["quantity"] = currentQty + goldAmount
						log.Printf("✅ Added %dg to existing gold stack in backpack[%d] (new total: %d)", goldAmount, i, currentQty+goldAmount)
						return nil
					}

					// Found empty slot
					if slot["item"] == nil || slot["item"] == "" {
						slot["item"] = "gold-piece"
						slot["quantity"] = goldAmount
						log.Printf("✅ Added %dg to backpack[%d]", goldAmount, i)
						return nil
					}
				}
			}
		}
	}

	log.Printf("❌ No empty slots available for gold")
	return fmt.Errorf("no empty slots available for gold")
}

func getStartingGold(database *sql.DB, background string) (int, error) {
	var dataJSON string
	err := database.QueryRow("SELECT data FROM starting_gold WHERE id = 'starting-gold'").Scan(&dataJSON)
	if err != nil {
		return 1000, err
	}

	var goldData map[string][][]interface{}
	if err := json.Unmarshal([]byte(dataJSON), &goldData); err != nil {
		return 1000, err
	}

	goldList, ok := goldData["starting-gold"]
	if !ok {
		return 1000, fmt.Errorf("no starting-gold key")
	}

	for _, entry := range goldList {
		if len(entry) >= 2 {
			if bg, ok := entry[0].(string); ok && bg == background {
				if gold, ok := entry[1].(float64); ok {
					return int(gold), nil
				}
			}
		}
	}

	return 1000, nil
}

// ============================================================================
// SAVE TO DISK
// ============================================================================

func saveToDisk(npub string, saveFile *types.SaveFile) error {
	savesDir := filepath.Join(SavesDirectory, npub)
	if err := os.MkdirAll(savesDir, 0755); err != nil {
		return fmt.Errorf("failed to create saves directory: %v", err)
	}

	savePath := filepath.Join(savesDir, saveFile.InternalID+".json")
	data, err := json.MarshalIndent(saveFile, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(savePath, data, 0644)
}

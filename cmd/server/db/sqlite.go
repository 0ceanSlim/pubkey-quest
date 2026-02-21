package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// InitDatabase initializes the database connection and validates it exists
// Used by the server - expects database to already be created by migration tool
func InitDatabase() error {
	// Ensure www directory exists
	dataDir := "./www"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create www directory: %v", err)
	}

	dbPath := filepath.Join(dataDir, "game.db")

	// Check if database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return fmt.Errorf("database not found at %s - please run the migration tool first (cd game-data && go run migrate.go)", dbPath)
	}

	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	log.Printf("✅ Connected to SQLite database at %s", dbPath)

	// Validate that required tables exist
	if err = validateDatabase(); err != nil {
		return fmt.Errorf("database validation failed: %v\nPlease run the migration tool (cd game-data && go run migrate.go)", err)
	}

	return nil
}

// GetDB returns the database connection
func GetDB() *sql.DB {
	return db
}

// Close closes the database connection
func Close() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

// validateDatabase checks that all required tables exist
func validateDatabase() error {
	requiredTables := []string{
		"items",
		"spells",
		"monsters",
		"locations",
		"npcs",
		"effects",
		"generation_weights",
		"starting_gear",
		"starting_gold",
		"starting_locations",
		"starting_spells",
		"spell_slots_progression",
		"music_tracks",
		"shop_pricing",
		"advancement",
	}

	for _, table := range requiredTables {
		var exists int
		err := db.QueryRow(`
			SELECT COUNT(*)
			FROM sqlite_master
			WHERE type='table' AND name=?
		`, table).Scan(&exists)

		if err != nil {
			return fmt.Errorf("failed to check for table %s: %v", table, err)
		}

		if exists == 0 {
			return fmt.Errorf("required table '%s' not found", table)
		}
	}

	log.Println("✅ Database validation passed - all required tables exist")
	return nil
}

// Item represents an item from the database
type Item struct {
	ID          string
	Name        string
	Description string
	Type        string
	Value       int
	Properties  string
	Tags        string
	Rarity      string
}

// NPC represents an NPC from the database
type NPC struct {
	ID         string
	Name       string
	Title      string
	Race       string
	Location   string
	Building   string
	Description string
	Properties string
}

// GetItemByID retrieves an item by its ID
func GetItemByID(itemID string) (*Item, error) {
	var item Item
	var propertiesJSON string
	var tagsJSON string

	err := db.QueryRow(`
		SELECT id, name, description, item_type, properties, tags, rarity
		FROM items
		WHERE id = ?
	`, itemID).Scan(&item.ID, &item.Name, &item.Description, &item.Type, &propertiesJSON, &tagsJSON, &item.Rarity)

	if err != nil {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}

	item.Properties = propertiesJSON
	item.Tags = tagsJSON

	// Parse price from properties JSON
	var properties map[string]interface{}
	if err := parseJSON(propertiesJSON, &properties); err == nil {
		if val, ok := properties["price"].(float64); ok {
			item.Value = int(val)
		} else if val, ok := properties["value"].(float64); ok {
			// Fallback to "value" if "price" doesn't exist
			item.Value = int(val)
		}
	}

	return &item, nil
}

// GetNPCByID retrieves an NPC by its ID and returns a types.NPCData
func GetNPCByID(npcID string) (*NPCData, error) {
	var propertiesJSON string

	err := db.QueryRow(`
		SELECT properties
		FROM npcs
		WHERE id = ?
	`, npcID).Scan(&propertiesJSON)

	if err != nil {
		return nil, fmt.Errorf("NPC not found: %s", npcID)
	}

	// Parse properties JSON into NPCData
	var npcData NPCData
	if err := parseJSON(propertiesJSON, &npcData); err != nil {
		return nil, fmt.Errorf("failed to parse NPC data: %v", err)
	}

	return &npcData, nil
}

// ShopPricingRules represents pricing formulas from shop-pricing.json
type ShopPricingRules struct {
	BuyPricing struct {
		General struct {
			BaseMultiplier float64 `json:"base_multiplier"`
			CharismaRate   float64 `json:"charisma_rate"`
		} `json:"general"`
		Specialty struct {
			BaseMultiplier float64 `json:"base_multiplier"`
			CharismaRate   float64 `json:"charisma_rate"`
		} `json:"specialty"`
	} `json:"buy_pricing"`
	SellPricing struct {
		General struct {
			BaseMultiplier float64 `json:"base_multiplier"`
			CharismaRate   float64 `json:"charisma_rate"`
		} `json:"general"`
		Specialty struct {
			BaseMultiplier float64 `json:"base_multiplier"`
			CharismaRate   float64 `json:"charisma_rate"`
		} `json:"specialty"`
	} `json:"sell_pricing"`
	CharismaBase int `json:"charisma_base"`
}

// GetShopPricingRules retrieves shop pricing rules from the database
func GetShopPricingRules() (*ShopPricingRules, error) {
	var dataJSON string

	err := db.QueryRow(`SELECT data FROM shop_pricing WHERE id = 'shop-pricing' LIMIT 1`).Scan(&dataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query shop pricing: %v", err)
	}

	var rules ShopPricingRules
	if err := json.Unmarshal([]byte(dataJSON), &rules); err != nil {
		return nil, fmt.Errorf("failed to parse shop pricing JSON: %v", err)
	}

	return &rules, nil
}

// Helper to parse JSON strings
func parseJSON(jsonStr string, target interface{}) error {
	if jsonStr == "" {
		return fmt.Errorf("empty JSON string")
	}
	return json.Unmarshal([]byte(jsonStr), target)
}

// NPCData represents the full NPC structure (copied from types package to avoid circular import)
type NPCData struct {
	ID            string                 `json:"id"`
	Name          string                 `json:"name"`
	Title         string                 `json:"title,omitempty"`
	Race          string                 `json:"race,omitempty"`
	Location      string                 `json:"location,omitempty"`
	Building      string                 `json:"building,omitempty"`
	Description   string                 `json:"description,omitempty"`
	Greeting      map[string]string      `json:"greeting,omitempty"`
	Dialogue      map[string]interface{} `json:"dialogue,omitempty"`
	Schedule      []interface{}          `json:"schedule,omitempty"`
	ShopConfig    map[string]interface{} `json:"shop_config,omitempty"`
	StorageConfig map[string]interface{} `json:"storage_config,omitempty"`
	InnConfig     map[string]interface{} `json:"inn_config,omitempty"`
}

// GenerationWeights represents the character generation weight data
type GenerationWeights struct {
	Races                    []string                  `json:"Races"`
	RaceWeights              []int                     `json:"RaceWeights"`
	ClassWeightsByRace       map[string]map[string]int `json:"classWeightsByRace"`
	BackgroundWeightsByClass map[string]map[string]int `json:"BackgroundWeightsByClass"`
	Alignments               []string                  `json:"Alignments"`
	AlignmentWeights         []int                     `json:"AlignmentWeights"`
}

// GetGenerationWeights retrieves character generation weights from the database
func GetGenerationWeights() (*GenerationWeights, error) {
	var dataJSON string

	err := db.QueryRow(`SELECT data FROM generation_weights WHERE id = 'generation-weights' LIMIT 1`).Scan(&dataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query generation weights: %v", err)
	}

	var weights GenerationWeights
	if err := json.Unmarshal([]byte(dataJSON), &weights); err != nil {
		return nil, fmt.Errorf("failed to parse generation weights JSON: %v", err)
	}

	return &weights, nil
}
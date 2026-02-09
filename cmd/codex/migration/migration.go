package migration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var database *sql.DB

// Status holds migration progress information
type Status struct {
	Step     string `json:"step"`
	Progress int    `json:"progress"`
	Total    int    `json:"total"`
	Message  string `json:"message"`
	Error    string `json:"error,omitempty"`
}

// StatusCallback is called during migration to report progress
type StatusCallback func(status Status)

// Migrate performs the full database migration from JSON files
func Migrate(dbPath string, callback StatusCallback) error {
	log.Println("🔄 Starting database migration...")

	if callback != nil {
		callback(Status{Step: "init", Message: "Initializing database connection"})
	}

	// Initialize database connection
	if err := initDatabase(dbPath); err != nil {
		if callback != nil {
			callback(Status{Step: "init", Error: err.Error()})
		}
		return fmt.Errorf("failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create all tables first
	if callback != nil {
		callback(Status{Step: "tables", Message: "Creating database tables"})
	}
	if err := createTables(); err != nil {
		if callback != nil {
			callback(Status{Step: "tables", Error: err.Error()})
		}
		return fmt.Errorf("failed to create tables: %v", err)
	}

	// Populate tables with data from JSON files
	if err := migrateFromJSON(callback); err != nil {
		if callback != nil {
			callback(Status{Step: "data", Error: err.Error()})
		}
		return fmt.Errorf("migration failed: %v", err)
	}

	if callback != nil {
		callback(Status{Step: "complete", Message: "Database migration completed successfully!"})
	}

	log.Println("✅ Database migration completed successfully!")
	return nil
}

// initDatabase initializes the database connection for migration
func initDatabase(dbPath string) error {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create database directory: %v", err)
	}

	var err error
	database, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test the connection
	if err = database.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	log.Printf("✅ Connected to SQLite database at %s", dbPath)
	return nil
}

// migrateEffectsSchema adds source_type column to effects table if it doesn't exist
func migrateEffectsSchema() error {
	// Check if source_type column exists
	var columnExists bool
	err := database.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('effects')
		WHERE name = 'source_type'
	`).Scan(&columnExists)

	if err != nil {
		return fmt.Errorf("failed to check for source_type column: %v", err)
	}

	if columnExists {
		log.Println("✅ Effects table already has source_type column")
		return nil
	}

	log.Println("🔧 Adding source_type column to effects table...")

	// Add the column
	_, err = database.Exec(`ALTER TABLE effects ADD COLUMN source_type TEXT`)
	if err != nil {
		return fmt.Errorf("failed to add source_type column: %v", err)
	}

	// Update existing rows by reading their properties JSON
	rows, err := database.Query(`SELECT id, properties FROM effects`)
	if err != nil {
		return fmt.Errorf("failed to query effects: %v", err)
	}
	defer rows.Close()

	updateCount := 0
	for rows.Next() {
		var id, propsJSON string
		if err := rows.Scan(&id, &propsJSON); err != nil {
			continue
		}

		var effect map[string]interface{}
		if err := json.Unmarshal([]byte(propsJSON), &effect); err != nil {
			log.Printf("⚠️ Failed to parse effect %s: %v", id, err)
			continue
		}

		sourceType, _ := effect["source_type"].(string)
		if sourceType != "" {
			_, err = database.Exec(`UPDATE effects SET source_type = ? WHERE id = ?`, sourceType, id)
			if err != nil {
				log.Printf("⚠️ Failed to update source_type for %s: %v", id, err)
			} else {
				updateCount++
			}
		}
	}

	log.Printf("✅ Added source_type column and updated %d effects", updateCount)
	return nil
}

// createTables creates all the necessary database tables
func createTables() error {
	log.Println("📋 Creating database tables...")

	tables := []string{
		// Items table for weapons, armor, consumables, etc.
		`CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			item_type TEXT NOT NULL,
			properties TEXT,
			tags TEXT,
			rarity TEXT DEFAULT 'common',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Spells table
		`CREATE TABLE IF NOT EXISTS spells (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			level INTEGER NOT NULL,
			school TEXT NOT NULL,
			damage TEXT,
			mana_cost INTEGER,
			classes TEXT,
			properties TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Monsters table
		`CREATE TABLE IF NOT EXISTS monsters (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			challenge_rating REAL,
			stats TEXT,
			actions TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Locations table
		`CREATE TABLE IF NOT EXISTS locations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			location_type TEXT,
			description TEXT,
			image TEXT,
			music TEXT,
			properties TEXT,
			connections TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// NPCs table
		`CREATE TABLE IF NOT EXISTS npcs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			title TEXT,
			race TEXT,
			location TEXT,
			building TEXT,
			description TEXT,
			properties TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Equipment packs table
		`CREATE TABLE IF NOT EXISTS equipment_packs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			items TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Effects table
		`CREATE TABLE IF NOT EXISTS effects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			source_type TEXT,
			properties TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Abilities table (martial class abilities)
		`CREATE TABLE IF NOT EXISTS abilities (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			class TEXT NOT NULL,
			unlock_level INTEGER NOT NULL,
			resource_cost INTEGER DEFAULT 0,
			resource_type TEXT,
			cooldown TEXT,
			description TEXT,
			properties TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, table := range tables {
		if _, err := database.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	// Migrate existing effects table to add source_type column if it doesn't exist
	// This is needed for databases created before the data-driven refactor
	if err := migrateEffectsSchema(); err != nil {
		log.Printf("⚠️ Warning: failed to migrate effects schema: %v", err)
		// Don't fail - the table might already have the column
	}

	log.Println("✅ Database tables created successfully")
	return nil
}

// migrateFromJSON imports all JSON data into the database
func migrateFromJSON(callback StatusCallback) error {
	log.Println("📦 Starting JSON to database migration...")

	// Migrate character data
	if callback != nil {
		callback(Status{Step: "character", Message: "Migrating character data"})
	}
	if err := migrateCharacterData(); err != nil {
		return fmt.Errorf("failed to migrate character data: %v", err)
	}

	// Migrate items
	if callback != nil {
		callback(Status{Step: "items", Message: "Migrating items"})
	}
	if err := migrateItems(callback); err != nil {
		return fmt.Errorf("failed to migrate items: %v", err)
	}

	// Migrate spells
	if callback != nil {
		callback(Status{Step: "spells", Message: "Migrating spells"})
	}
	if err := migrateSpells(callback); err != nil {
		return fmt.Errorf("failed to migrate spells: %v", err)
	}

	// Migrate content data (monsters, locations, NPCs)
	if callback != nil {
		callback(Status{Step: "content", Message: "Migrating monsters, locations, and NPCs"})
	}
	if err := migrateContentData(callback); err != nil {
		return fmt.Errorf("failed to migrate content data: %v", err)
	}

	// Migrate effects
	if callback != nil {
		callback(Status{Step: "effects", Message: "Migrating effects"})
	}
	if err := migrateEffects(callback); err != nil {
		return fmt.Errorf("failed to migrate effects: %v", err)
	}

	// Migrate abilities
	if callback != nil {
		callback(Status{Step: "abilities", Message: "Migrating martial abilities"})
	}
	if err := migrateAbilities(callback); err != nil {
		return fmt.Errorf("failed to migrate abilities: %v", err)
	}

	// Migrate system data (spell slots, music)
	if callback != nil {
		callback(Status{Step: "system", Message: "Migrating system data"})
	}
	if err := migrateSystemData(); err != nil {
		return fmt.Errorf("failed to migrate system data: %v", err)
	}

	log.Println("✅ Data migration completed successfully!")
	return nil
}

// migrateCharacterData migrates character-related JSON files
func migrateCharacterData() error {
	log.Println("Migrating character data...")

	characterDataPath := filepath.Join("game-data/systems/new-character")

	// Define the files we want to migrate for character data
	characterFiles := map[string]string{
		"base-hp.json":              "character_base_hp",
		"generation-weights.json":   "generation_weights",
		"introductions.json":        "introductions",
		"starting-gear.json":        "starting_gear",
		"starting-gold.json":        "starting_gold",
		"starting-locations.json":   "starting_locations",
		"starting-spells.json":      "starting_spells",
	}

	for filename, tableName := range characterFiles {
		filePath := filepath.Join(characterDataPath, filename)
		if err := migrateGenericJSON(filePath, tableName); err != nil {
			log.Printf("Warning: failed to migrate %s: %v", filename, err)
		}
	}

	return nil
}

// migrateItems migrates all item JSON files
func migrateItems(callback StatusCallback) error {
	log.Println("Migrating items...")

	itemsPath := "game-data/items"

	// Clear existing items
	if _, err := database.Exec("DELETE FROM items"); err != nil {
		return fmt.Errorf("failed to clear items table: %v", err)
	}

	// Count total items first
	totalItems := 0
	filepath.WalkDir(itemsPath, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			totalItems++
		}
		return nil
	})

	count := 0
	err := filepath.WalkDir(itemsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			if err := migrateItemFile(path); err != nil {
				log.Printf("Warning: failed to migrate item file %s: %v", path, err)
			} else {
				count++
				if callback != nil && count%10 == 0 {
					callback(Status{
						Step:     "items",
						Progress: count,
						Total:    totalItems,
						Message:  fmt.Sprintf("Migrated %d/%d items", count, totalItems),
					})
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk items directory: %v", err)
	}

	log.Printf("Migrated %d items", count)
	return nil
}

// migrateItemFile migrates a single item JSON file
func migrateItemFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var item map[string]interface{}
	if err := json.Unmarshal(data, &item); err != nil {
		return err
	}

	// Extract base filename as ID
	id := strings.TrimSuffix(filepath.Base(filePath), ".json")

	// Convert item data to required fields
	name, _ := item["name"].(string)
	description, _ := item["description"].(string)
	itemType, _ := item["type"].(string)
	rarity, _ := item["rarity"].(string)

	// Extract tags as JSON
	tagsJSON, _ := json.Marshal(item["tags"])

	// Serialize all properties as JSON for the properties field
	propertiesJSON, _ := json.Marshal(item)

	stmt := `INSERT INTO items (id, name, description, item_type, properties, tags, rarity) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err = database.Exec(stmt, id, name, description, itemType, string(propertiesJSON), string(tagsJSON), rarity)
	return err
}

// migrateSpells migrates all spell JSON files
func migrateSpells(callback StatusCallback) error {
	log.Println("Migrating spells...")

	spellsPath := "game-data/magic/spells"

	// Clear existing spells
	if _, err := database.Exec("DELETE FROM spells"); err != nil {
		return fmt.Errorf("failed to clear spells table: %v", err)
	}

	// Count total spells first
	totalSpells := 0
	filepath.WalkDir(spellsPath, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			totalSpells++
		}
		return nil
	})

	count := 0
	err := filepath.WalkDir(spellsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			if err := migrateSpellFile(path); err != nil {
				log.Printf("Warning: failed to migrate spell file %s: %v", path, err)
			} else {
				count++
				if callback != nil && count%10 == 0 {
					callback(Status{
						Step:     "spells",
						Progress: count,
						Total:    totalSpells,
						Message:  fmt.Sprintf("Migrated %d/%d spells", count, totalSpells),
					})
				}
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk spells directory: %v", err)
	}

	log.Printf("Migrated %d spells", count)
	return nil
}

// migrateSpellFile migrates a single spell JSON file
func migrateSpellFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var spell map[string]interface{}
	if err := json.Unmarshal(data, &spell); err != nil {
		return err
	}

	// Extract base filename as ID
	id := strings.TrimSuffix(filepath.Base(filePath), ".json")

	// Convert spell data to required fields
	name, _ := spell["name"].(string)
	description, _ := spell["description"].(string)
	level, _ := spell["level"].(float64)
	school, _ := spell["school"].(string)
	damage, _ := spell["damage"].(string)
	manaCostFloat, _ := spell["mana_cost"].(float64)
	manaCost := int(manaCostFloat)

	// Extract classes as JSON
	classesJSON, _ := json.Marshal(spell["classes"])

	// Serialize all properties as JSON for the properties field
	propertiesJSON, _ := json.Marshal(spell)

	stmt := `INSERT INTO spells (id, name, description, level, school, damage, mana_cost, classes, properties)
	         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = database.Exec(stmt, id, name, description, int(level), school, damage, manaCost, string(classesJSON), string(propertiesJSON))
	return err
}

// migrateContentData migrates monsters, locations, and other content
func migrateContentData(callback StatusCallback) error {
	log.Println("Migrating content data...")

	// Migrate monsters
	if err := migrateMonsters(callback); err != nil {
		return fmt.Errorf("failed to migrate monsters: %v", err)
	}

	// Migrate locations
	if err := migrateLocations(callback); err != nil {
		return fmt.Errorf("failed to migrate locations: %v", err)
	}

	// Migrate NPCs
	if err := migrateNPCs(callback); err != nil {
		return fmt.Errorf("failed to migrate NPCs: %v", err)
	}

	return nil
}

// migrateMonsters migrates all monster JSON files
func migrateMonsters(callback StatusCallback) error {
	monstersPath := "game-data/monsters"

	// Clear existing monsters
	if _, err := database.Exec("DELETE FROM monsters"); err != nil {
		return fmt.Errorf("failed to clear monsters table: %v", err)
	}

	count := 0
	err := filepath.WalkDir(monstersPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			if err := migrateMonsterFile(path); err != nil {
				log.Printf("Warning: failed to migrate monster file %s: %v", path, err)
			} else {
				count++
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk monsters directory: %v", err)
	}

	log.Printf("Migrated %d monsters", count)
	return nil
}

// migrateMonsterFile migrates a single monster JSON file
func migrateMonsterFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var monster map[string]interface{}
	if err := json.Unmarshal(data, &monster); err != nil {
		return err
	}

	// Extract base filename as ID
	id := strings.TrimSuffix(filepath.Base(filePath), ".json")

	// Convert monster data to required fields
	name, _ := monster["name"].(string)
	challengeRating, _ := monster["challenge_rating"].(float64)

	// Serialize stats and actions as JSON
	statsJSON, _ := json.Marshal(monster)
	actionsJSON, _ := json.Marshal(map[string]interface{}{}) // Empty for now

	stmt := `INSERT INTO monsters (id, name, challenge_rating, stats, actions) VALUES (?, ?, ?, ?, ?)`
	_, err = database.Exec(stmt, id, name, challengeRating, string(statsJSON), string(actionsJSON))
	return err
}

// migrateLocations migrates location data
func migrateLocations(callback StatusCallback) error {
	locationsPath := "game-data/locations"

	// Clear existing locations
	if _, err := database.Exec("DELETE FROM locations"); err != nil {
		return fmt.Errorf("failed to clear locations table: %v", err)
	}

	count := 0

	// Walk through cities and environments
	subDirs := []string{"cities", "environments"}
	for _, subDir := range subDirs {
		dirPath := filepath.Join(locationsPath, subDir)
		err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if !d.IsDir() && strings.HasSuffix(path, ".json") {
				locationType := subDir[:len(subDir)-1] // Remove 's' from cities/environments
				if err := migrateLocationFile(path, locationType); err != nil {
					log.Printf("Warning: failed to migrate location file %s: %v", path, err)
				} else {
					count++
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("Warning: failed to walk %s directory: %v", subDir, err)
		}
	}

	log.Printf("Migrated %d locations", count)
	return nil
}

// migrateLocationFile migrates a single location JSON file
func migrateLocationFile(filePath, locationType string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var location map[string]interface{}
	if err := json.Unmarshal(data, &location); err != nil {
		return err
	}

	// Extract base filename as ID
	id := strings.TrimSuffix(filepath.Base(filePath), ".json")

	name, _ := location["name"].(string)
	description, _ := location["description"].(string)
	image, _ := location["image"].(string)
	music, _ := location["music"].(string)

	// Serialize all properties as JSON
	propertiesJSON, _ := json.Marshal(location)
	connectionsJSON, _ := json.Marshal(location["connections"])

	stmt := `INSERT INTO locations (id, name, location_type, description, image, music, properties, connections)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = database.Exec(stmt, id, name, locationType, description, image, music, string(propertiesJSON), string(connectionsJSON))
	return err
}

// migrateNPCs migrates all NPC JSON files from all location subdirectories
func migrateNPCs(callback StatusCallback) error {
	npcsPath := "game-data/npcs"

	// Clear existing NPCs
	if _, err := database.Exec("DELETE FROM npcs"); err != nil {
		return fmt.Errorf("failed to clear npcs table: %v", err)
	}

	count := 0

	// Walk through all subdirectories (kingdom, millhaven, etc.)
	err := filepath.WalkDir(npcsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			// Extract location from folder path (e.g., "game-data/npcs/kingdom/..." -> "kingdom")
			relPath, _ := filepath.Rel(npcsPath, path)
			locationFolder := filepath.Dir(relPath)

			if err := migrateNPCFile(path, locationFolder); err != nil {
				log.Printf("Warning: failed to migrate NPC file %s: %v", path, err)
			} else {
				count++
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk NPCs directory: %v", err)
	}

	log.Printf("Migrated %d NPCs", count)
	return nil
}

// migrateNPCFile migrates a single NPC JSON file
func migrateNPCFile(filePath, locationFromPath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var npc map[string]interface{}
	if err := json.Unmarshal(data, &npc); err != nil {
		return err
	}

	// Extract base filename as ID
	id := strings.TrimSuffix(filepath.Base(filePath), ".json")

	// Convert NPC data to required fields
	name, _ := npc["name"].(string)
	title, _ := npc["title"].(string)
	race, _ := npc["race"].(string)
	building, _ := npc["building"].(string)
	description, _ := npc["description"].(string)

	// Use location from folder path
	location := locationFromPath

	// Serialize all properties as JSON
	propertiesJSON, _ := json.Marshal(npc)

	stmt := `INSERT INTO npcs (id, name, title, race, location, building, description, properties)
	         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = database.Exec(stmt, id, name, title, race, location, building, description, string(propertiesJSON))
	return err
}

// migrateEffects migrates all effect JSON files
func migrateEffects(callback StatusCallback) error {
	effectsPath := "game-data/effects"

	// Clear existing effects
	if _, err := database.Exec("DELETE FROM effects"); err != nil {
		return fmt.Errorf("failed to clear effects table: %v", err)
	}

	count := 0
	err := filepath.WalkDir(effectsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			if err := migrateEffectFile(path); err != nil {
				log.Printf("Warning: failed to migrate effect file %s: %v", path, err)
			} else {
				count++
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk effects directory: %v", err)
	}

	log.Printf("Migrated %d effects", count)
	return nil
}

// migrateEffectFile migrates a single effect JSON file
func migrateEffectFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var effect map[string]interface{}
	if err := json.Unmarshal(data, &effect); err != nil {
		return err
	}

	// Extract base filename as ID
	id := strings.TrimSuffix(filepath.Base(filePath), ".json")

	// Convert effect data to required fields
	name, _ := effect["name"].(string)
	description, _ := effect["description"].(string)
	sourceType, _ := effect["source_type"].(string)

	// Serialize all properties as JSON
	propertiesJSON, _ := json.Marshal(effect)

	stmt := `INSERT INTO effects (id, name, description, source_type, properties) VALUES (?, ?, ?, ?, ?)`
	_, err = database.Exec(stmt, id, name, description, sourceType, string(propertiesJSON))
	return err
}

// migrateAbilities migrates all ability JSON files from class subdirectories
func migrateAbilities(callback StatusCallback) error {
	abilitiesPath := "game-data/systems/abilities"

	// Clear existing abilities
	if _, err := database.Exec("DELETE FROM abilities"); err != nil {
		return fmt.Errorf("failed to clear abilities table: %v", err)
	}

	count := 0

	// Walk through class subdirectories (fighter, barbarian, monk, rogue)
	err := filepath.WalkDir(abilitiesPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".json") {
			if err := migrateAbilityFile(path); err != nil {
				log.Printf("Warning: failed to migrate ability file %s: %v", path, err)
			} else {
				count++
			}
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk abilities directory: %v", err)
	}

	log.Printf("Migrated %d abilities", count)
	return nil
}

// migrateAbilityFile migrates a single ability JSON file
func migrateAbilityFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var ability map[string]interface{}
	if err := json.Unmarshal(data, &ability); err != nil {
		return err
	}

	// Extract base filename as ID
	id := strings.TrimSuffix(filepath.Base(filePath), ".json")

	// Convert ability data to required fields
	name, _ := ability["name"].(string)
	class, _ := ability["class"].(string)
	unlockLevelFloat, _ := ability["unlock_level"].(float64)
	unlockLevel := int(unlockLevelFloat)
	resourceCostFloat, _ := ability["resource_cost"].(float64)
	resourceCost := int(resourceCostFloat)
	resourceType, _ := ability["resource_type"].(string)
	cooldown, _ := ability["cooldown"].(string)
	description, _ := ability["description"].(string)

	// Serialize all properties as JSON (includes scaling_tiers, effects_applied, etc.)
	propertiesJSON, _ := json.Marshal(ability)

	stmt := `INSERT INTO abilities (id, name, class, unlock_level, resource_cost, resource_type, cooldown, description, properties)
	         VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = database.Exec(stmt, id, name, class, unlockLevel, resourceCost, resourceType, cooldown, description, string(propertiesJSON))
	return err
}

// migrateGenericJSON migrates a generic JSON file to a dynamically created table
func migrateGenericJSON(filePath, tableName string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Create table if it doesn't exist
	createSQL := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id VARCHAR PRIMARY KEY,
		data JSON,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`, tableName)

	if _, err := database.Exec(createSQL); err != nil {
		return fmt.Errorf("failed to create table %s: %v", tableName, err)
	}

	// Clear existing data
	if _, err := database.Exec(fmt.Sprintf("DELETE FROM %s", tableName)); err != nil {
		return fmt.Errorf("failed to clear table %s: %v", tableName, err)
	}

	// Insert data
	id := strings.TrimSuffix(filepath.Base(filePath), ".json")
	stmt := fmt.Sprintf(`INSERT INTO %s (id, data) VALUES (?, ?)`, tableName)
	_, err = database.Exec(stmt, id, string(data))

	if err != nil {
		return fmt.Errorf("failed to insert into %s: %v", tableName, err)
	}

	return nil
}

// migrateSystemData migrates system configuration files
func migrateSystemData() error {
	log.Println("Migrating system data...")

	systemDataPath := "game-data/systems"

	// Migrate music.json
	musicPath := filepath.Join(systemDataPath, "music.json")
	if err := migrateGenericJSON(musicPath, "music_tracks"); err != nil {
		log.Printf("Warning: failed to migrate music.json: %v", err)
	}

	// Migrate spell-slots.json
	spellSlotsPath := "game-data/magic/spell-slots.json"
	if err := migrateGenericJSON(spellSlotsPath, "spell_slots_progression"); err != nil {
		log.Printf("Warning: failed to migrate spell-slots.json: %v", err)
	}

	// Migrate shop-pricing.json
	shopPricingPath := filepath.Join(systemDataPath, "shop-pricing.json")
	if err := migrateGenericJSON(shopPricingPath, "shop_pricing"); err != nil {
		log.Printf("Warning: failed to migrate shop-pricing.json: %v", err)
	}

	return nil
}

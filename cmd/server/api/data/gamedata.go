package data

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"pubkey-quest/cmd/server/db"
)

// GameData represents the complete static game data bundle
// swagger:model GameData
type GameData struct {
	Items       []Item       `json:"items"`
	Spells      []Spell      `json:"spells"`
	Monsters    []Monster    `json:"monsters"`
	Locations   []Location   `json:"locations"`
	Packs       []Pack       `json:"packs"`
	MusicTracks []MusicTrack `json:"music_tracks"`
}

// Item represents game items
// swagger:model Item
type Item struct {
	ID          string                 `json:"id" example:"longsword"`
	Name        string                 `json:"name" example:"Longsword"`
	Description string                 `json:"description" example:"A versatile martial weapon"`
	ItemType    string                 `json:"item_type" example:"weapon"`
	Properties  map[string]interface{} `json:"properties"`
	Tags        []string               `json:"tags" example:"martial,slashing"`
	Rarity      string                 `json:"rarity" example:"common"`
}

// Spell represents game spells
// swagger:model Spell
type Spell struct {
	ID          string                 `json:"id" example:"fire-bolt"`
	Name        string                 `json:"name" example:"Fire Bolt"`
	Description string                 `json:"description" example:"A mote of fire streaks toward a creature"`
	Level       int                    `json:"level" example:"0"`
	School      string                 `json:"school" example:"evocation"`
	Damage      string                 `json:"damage" example:"1d10"`
	ManaCost    int                    `json:"mana_cost" example:"0"`
	Classes     []string               `json:"classes" example:"wizard,sorcerer"`
	Properties  map[string]interface{} `json:"properties"`
}

// Monster represents game monsters
// swagger:model Monster
type Monster struct {
	ID              string                 `json:"id" example:"goblin"`
	Name            string                 `json:"name" example:"Goblin"`
	ChallengeRating float64                `json:"challenge_rating" example:"0.25"`
	Stats           map[string]interface{} `json:"stats"`
	Actions         map[string]interface{} `json:"actions"`
}

// Location represents game locations
// swagger:model Location
type Location struct {
	ID           string                 `json:"id" example:"millhaven"`
	Name         string                 `json:"name" example:"Millhaven"`
	LocationType string                 `json:"location_type" example:"city"`
	Description  string                 `json:"description" example:"A bustling trade town"`
	Image        string                 `json:"image,omitempty" example:"millhaven.png"`
	Music        string                 `json:"music,omitempty" example:"town-theme"`
	Properties   map[string]interface{} `json:"properties"`
	Connections  []string               `json:"connections" example:"verdant,kingdom"`
}

// Pack represents equipment packs
// swagger:model Pack
type Pack struct {
	ID    string        `json:"id" example:"explorers-pack"`
	Name  string        `json:"name" example:"Explorer's Pack"`
	Items []interface{} `json:"items"`
}

// MusicTrack represents music tracks in the game
// swagger:model MusicTrack
type MusicTrack struct {
	Title      string `json:"title" example:"Town Theme"`
	File       string `json:"file" example:"town-theme.mp3"`
	UnlocksAt  string `json:"unlocks_at,omitempty" example:"millhaven"`
	AutoUnlock bool   `json:"auto_unlock,omitempty" example:"false"`
}

// NPC represents non-player characters
// swagger:model NPC
type NPC struct {
	ID          string                 `json:"id" example:"blacksmith-john"`
	Name        string                 `json:"name" example:"John"`
	Title       string                 `json:"title,omitempty" example:"Blacksmith"`
	Race        string                 `json:"race,omitempty" example:"Human"`
	Location    string                 `json:"location,omitempty" example:"millhaven"`
	Building    string                 `json:"building,omitempty" example:"forge"`
	Description string                 `json:"description,omitempty" example:"A skilled metalworker"`
	Properties  map[string]interface{} `json:"properties"`
}

// GameDataHandler godoc
// @Summary      Get all game data
// @Description  Returns all static game data in one request (items, spells, monsters, locations, packs, music tracks)
// @Tags         GameData
// @Produce      json
// @Success      200  {object}  GameData
// @Failure      500  {string}  string  "Database not available"
// @Router       /game-data [get]
func GameDataHandler(w http.ResponseWriter, r *http.Request) {
	database := db.GetDB()
	if database == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	gameData := GameData{}

	// Load all static data in parallel
	errChan := make(chan error, 6)

	go func() {
		var err error
		gameData.Items, err = LoadAllItems(database)
		errChan <- err
	}()

	go func() {
		var err error
		gameData.Spells, err = LoadAllSpells(database)
		errChan <- err
	}()

	go func() {
		var err error
		gameData.Monsters, err = LoadAllMonsters(database)
		errChan <- err
	}()

	go func() {
		var err error
		gameData.Locations, err = LoadAllLocations(database)
		errChan <- err
	}()

	go func() {
		var err error
		gameData.Packs, err = LoadAllPacks(database)
		errChan <- err
	}()

	go func() {
		var err error
		gameData.MusicTracks, err = LoadAllMusicTracks(database)
		errChan <- err
	}()

	// Wait for all operations to complete
	for i := 0; i < 6; i++ {
		if err := <-errChan; err != nil {
			log.Printf("Error loading game data: %v", err)
			http.Error(w, "Failed to load game data", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour

	if err := json.NewEncoder(w).Encode(gameData); err != nil {
		log.Printf("Error encoding game data: %v", err)
		http.Error(w, "Failed to encode game data", http.StatusInternalServerError)
		return
	}

	log.Printf("Served game data: %d items, %d spells, %d monsters, %d locations, %d packs, %d music tracks",
		len(gameData.Items), len(gameData.Spells), len(gameData.Monsters), len(gameData.Locations), len(gameData.Packs), len(gameData.MusicTracks))
}

// ItemsHandler godoc
// @Summary      Get items
// @Description  Returns all items, optionally filtered by item ID
// @Tags         GameData
// @Produce      json
// @Param        name  query     string  false  "Filter by item ID"
// @Success      200   {array}   Item
// @Failure      500   {string}  string  "Database error"
// @Router       /items [get]
func ItemsHandler(w http.ResponseWriter, r *http.Request) {
	database := db.GetDB()
	if database == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	items, err := LoadAllItems(database)
	if err != nil {
		log.Printf("Error loading items: %v", err)
		http.Error(w, "Failed to load items", http.StatusInternalServerError)
		return
	}

	// Filter by name if provided (name is actually the item ID from starting-gear.json)
	nameQuery := r.URL.Query().Get("name")
	if nameQuery != "" {
		log.Printf("Filtering items by ID: '%s'", nameQuery)
		var filteredItems []Item
		for _, item := range items {
			// Match by ID (the item filename without .json)
			if item.ID == nameQuery {
				log.Printf("  ✓ Match found: ID='%s', Name='%s'", item.ID, item.Name)
				filteredItems = append(filteredItems, item)
			}
		}
		if len(filteredItems) == 0 {
			log.Printf("  ⚠️ No items matched ID '%s'. Checking first 5 items in database:", nameQuery)
			for i := 0; i < 5 && i < len(items); i++ {
				log.Printf("    - ID: '%s', Name: '%s'", items[i].ID, items[i].Name)
			}
		}
		items = filteredItems
		log.Printf("Returning %d filtered items", len(items))
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// SpellsHandler godoc
// @Summary      Get spells
// @Description  Returns all spells, or a specific spell by ID if provided in path
// @Tags         GameData
// @Produce      json
// @Param        id   path      string  false  "Spell ID (e.g., fire-bolt)"
// @Success      200  {array}   Spell         "All spells or single spell"
// @Failure      404  {string}  string        "Spell not found"
// @Failure      500  {string}  string        "Database error"
// @Router       /spells/ [get]
// @Router       /spells/{id} [get]
func SpellsHandler(w http.ResponseWriter, r *http.Request) {
	database := db.GetDB()
	if database == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	// Extract spell ID from URL path if present (e.g., /api/spells/fire-bolt)
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/spells/"), "/")
	spellID := ""
	if len(pathParts) > 0 && pathParts[0] != "" {
		spellID = pathParts[0]
	}

	spells, err := LoadAllSpells(database)
	if err != nil {
		log.Printf("Error loading spells: %v", err)
		http.Error(w, "Failed to load spells", http.StatusInternalServerError)
		return
	}

	// Filter by ID if provided
	if spellID != "" {
		for _, spell := range spells {
			if spell.ID == spellID {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(spell)
				return
			}
		}
		http.Error(w, "Spell not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(spells)
}

// MonstersHandler godoc
// @Summary      Get monsters
// @Description  Returns all monsters in the game
// @Tags         GameData
// @Produce      json
// @Success      200  {array}   Monster
// @Failure      500  {string}  string  "Database error"
// @Router       /monsters [get]
func MonstersHandler(w http.ResponseWriter, r *http.Request) {
	database := db.GetDB()
	if database == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	monsters, err := LoadAllMonsters(database)
	if err != nil {
		log.Printf("Error loading monsters: %v", err)
		http.Error(w, "Failed to load monsters", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(monsters)
}

// LocationsHandler godoc
// @Summary      Get locations
// @Description  Returns all locations in the game world
// @Tags         GameData
// @Produce      json
// @Success      200  {array}   Location
// @Failure      500  {string}  string  "Database error"
// @Router       /locations [get]
func LocationsHandler(w http.ResponseWriter, r *http.Request) {
	database := db.GetDB()
	if database == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	locations, err := LoadAllLocations(database)
	if err != nil {
		log.Printf("Error loading locations: %v", err)
		http.Error(w, "Failed to load locations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(locations)
}

// NPCsHandler godoc
// @Summary      Get NPCs
// @Description  Returns all non-player characters in the game
// @Tags         GameData
// @Produce      json
// @Success      200  {array}   NPC
// @Failure      500  {string}  string  "Database error"
// @Router       /npcs [get]
func NPCsHandler(w http.ResponseWriter, r *http.Request) {
	database := db.GetDB()
	if database == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	npcs, err := LoadAllNPCs(database)
	if err != nil {
		log.Printf("Error loading NPCs: %v", err)
		http.Error(w, "Failed to load NPCs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(npcs)
}

// Database loading functions
func LoadAllItems(database *sql.DB) ([]Item, error) {
	rows, err := database.Query("SELECT id, name, description, item_type, properties, tags, rarity FROM items")
	if err != nil {
		return nil, fmt.Errorf("failed to query items: %v", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		var propertiesJSON, tagsJSON string

		err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.ItemType,
			&propertiesJSON, &tagsJSON, &item.Rarity)
		if err != nil {
			log.Printf("Error scanning item row: %v", err)
			continue
		}

		// Parse JSON fields
		if propertiesJSON != "" {
			json.Unmarshal([]byte(propertiesJSON), &item.Properties)
		}
		if tagsJSON != "" {
			json.Unmarshal([]byte(tagsJSON), &item.Tags)
		}

		items = append(items, item)
	}

	return items, nil
}

func LoadAllSpells(database *sql.DB) ([]Spell, error) {
	rows, err := database.Query("SELECT id, name, description, level, school, COALESCE(damage, ''), mana_cost, COALESCE(classes, ''), COALESCE(properties, '') FROM spells")
	if err != nil {
		return nil, fmt.Errorf("failed to query spells: %v", err)
	}
	defer rows.Close()

	var spells []Spell
	for rows.Next() {
		var spell Spell
		var propertiesJSON, classesJSON, damage string

		err := rows.Scan(&spell.ID, &spell.Name, &spell.Description, &spell.Level,
			&spell.School, &damage, &spell.ManaCost, &classesJSON, &propertiesJSON)
		if err != nil {
			log.Printf("Error scanning spell row: %v", err)
			continue
		}

		spell.Damage = damage

		// Parse JSON fields
		if propertiesJSON != "" {
			json.Unmarshal([]byte(propertiesJSON), &spell.Properties)
		}
		if classesJSON != "" {
			json.Unmarshal([]byte(classesJSON), &spell.Classes)
		}

		spells = append(spells, spell)
	}

	return spells, nil
}

func LoadAllMonsters(database *sql.DB) ([]Monster, error) {
	rows, err := database.Query("SELECT id, name, challenge_rating, stats, actions FROM monsters")
	if err != nil {
		return nil, fmt.Errorf("failed to query monsters: %v", err)
	}
	defer rows.Close()

	var monsters []Monster
	for rows.Next() {
		var monster Monster
		var statsJSON, actionsJSON string

		err := rows.Scan(&monster.ID, &monster.Name, &monster.ChallengeRating, &statsJSON, &actionsJSON)
		if err != nil {
			log.Printf("Error scanning monster row: %v", err)
			continue
		}

		// Parse JSON fields
		json.Unmarshal([]byte(statsJSON), &monster.Stats)
		json.Unmarshal([]byte(actionsJSON), &monster.Actions)

		monsters = append(monsters, monster)
	}

	return monsters, nil
}

func LoadAllLocations(database *sql.DB) ([]Location, error) {
	rows, err := database.Query("SELECT id, name, COALESCE(location_type, ''), COALESCE(description, ''), COALESCE(image, ''), COALESCE(music, ''), COALESCE(properties, ''), COALESCE(connections, '') FROM locations")
	if err != nil {
		return nil, fmt.Errorf("failed to query locations: %v", err)
	}
	defer rows.Close()

	var locations []Location
	for rows.Next() {
		var location Location
		var propertiesJSON, connectionsJSON string

		err := rows.Scan(&location.ID, &location.Name, &location.LocationType, &location.Description,
			&location.Image, &location.Music, &propertiesJSON, &connectionsJSON)
		if err != nil {
			log.Printf("Error scanning location row: %v", err)
			continue
		}

		// Parse JSON fields
		if propertiesJSON != "" {
			json.Unmarshal([]byte(propertiesJSON), &location.Properties)
		}
		if connectionsJSON != "" {
			json.Unmarshal([]byte(connectionsJSON), &location.Connections)
		}

		locations = append(locations, location)
	}

	return locations, nil
}

func LoadAllPacks(database *sql.DB) ([]Pack, error) {
	rows, err := database.Query("SELECT id, name, items FROM equipment_packs")
	if err != nil {
		return nil, fmt.Errorf("failed to query equipment packs: %v", err)
	}
	defer rows.Close()

	var packs []Pack
	for rows.Next() {
		var pack Pack
		var itemsJSON string

		err := rows.Scan(&pack.ID, &pack.Name, &itemsJSON)
		if err != nil {
			log.Printf("Error scanning pack row: %v", err)
			continue
		}

		// Parse JSON field
		json.Unmarshal([]byte(itemsJSON), &pack.Items)

		packs = append(packs, pack)
	}

	return packs, nil
}

func LoadAllNPCs(database *sql.DB) ([]NPC, error) {
	rows, err := database.Query("SELECT id, name, title, race, location, building, description, properties FROM npcs")
	if err != nil {
		return nil, fmt.Errorf("failed to query NPCs: %v", err)
	}
	defer rows.Close()

	var npcs []NPC
	for rows.Next() {
		var npc NPC
		var propertiesJSON string
		var title, race, location, building, description sql.NullString

		err := rows.Scan(&npc.ID, &npc.Name, &title, &race, &location, &building, &description, &propertiesJSON)
		if err != nil {
			log.Printf("Error scanning NPC row: %v", err)
			continue
		}

		// Handle nullable fields
		if title.Valid {
			npc.Title = title.String
		}
		if race.Valid {
			npc.Race = race.String
		}
		if location.Valid {
			npc.Location = location.String
		}
		if building.Valid {
			npc.Building = building.String
		}
		if description.Valid {
			npc.Description = description.String
		}

		// Parse JSON field
		if propertiesJSON != "" {
			json.Unmarshal([]byte(propertiesJSON), &npc.Properties)
		}

		npcs = append(npcs, npc)
	}

	return npcs, nil
}

// LoadItemByID loads the full item data for a single item by ID.
// The properties column stores the entire item JSON.
func LoadItemByID(database *sql.DB, id string) (map[string]interface{}, error) {
	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM items WHERE id = ?", id).Scan(&propertiesJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("item not found: %s", id)
		}
		return nil, fmt.Errorf("failed to query item %s: %v", id, err)
	}

	var item map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &item); err != nil {
		return nil, fmt.Errorf("failed to parse item %s: %v", id, err)
	}

	return item, nil
}

func LoadAllMusicTracks(database *sql.DB) ([]MusicTrack, error) {
	var dataJSON string
	err := database.QueryRow("SELECT data FROM music_tracks WHERE id = 'music'").Scan(&dataJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to query music_tracks: %v", err)
	}

	// Parse the JSON data
	var musicData struct {
		Tracks []MusicTrack `json:"tracks"`
	}

	if err := json.Unmarshal([]byte(dataJSON), &musicData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal music data: %v", err)
	}

	return musicData.Tracks, nil
}

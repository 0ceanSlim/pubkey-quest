package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// Type aliases for backward compatibility within api package
type SaveFile = types.SaveFile
type ActiveEffect = types.ActiveEffect
type EnrichedEffect = types.EnrichedEffect

// SavesDirectory re-exported from session package for backward compatibility
var SavesDirectory = session.SavesDirectory

// SaveListItem represents a save file in list responses
// swagger:model SaveListItem
type SaveListItem struct {
	ID                  string                 `json:"id" example:"save_1234567890"`
	D                   string                 `json:"d" example:"Hero Name"`
	CreatedAt           string                 `json:"created_at" example:"2025-01-15T12:00:00Z"`
	Race                string                 `json:"race" example:"Human"`
	Class               string                 `json:"class" example:"Fighter"`
	Background          string                 `json:"background" example:"Soldier"`
	Alignment           string                 `json:"alignment" example:"Neutral Good"`
	Experience          int                    `json:"experience" example:"0"`
	HP                  int                    `json:"hp" example:"12"`
	MaxHP               int                    `json:"max_hp" example:"12"`
	Mana                int                    `json:"mana" example:"0"`
	MaxMana             int                    `json:"max_mana" example:"0"`
	Fatigue             int                    `json:"fatigue" example:"0"`
	Hunger              int                    `json:"hunger" example:"2"`
	Stats               map[string]interface{} `json:"stats"`
	Location            string                 `json:"location" example:"millhaven"`
	District            string                 `json:"district" example:"center"`
	Building            string                 `json:"building" example:""`
	Inventory           map[string]interface{} `json:"inventory"`
	Vaults              []interface{}          `json:"vaults"`
	KnownSpells         []string               `json:"known_spells"`
	SpellSlots          map[string]interface{} `json:"spell_slots"`
	LocationsDiscovered []string               `json:"locations_discovered"`
	MusicTracksUnlocked []string               `json:"music_tracks_unlocked"`
	CurrentDay          int                    `json:"current_day" example:"1"`
	TimeOfDay           int                    `json:"time_of_day" example:"720"`
}

// SaveCreateResponse represents the response after creating/updating a save
// swagger:model SaveCreateResponse
type SaveCreateResponse struct {
	Success bool   `json:"success" example:"true"`
	SaveID  string `json:"save_id" example:"save_1234567890"`
	Message string `json:"message" example:"Game saved successfully"`
}

// SaveDeleteResponse represents the response after deleting a save
// swagger:model SaveDeleteResponse
type SaveDeleteResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Save deleted successfully"`
}

// SavesHandler godoc
// @Summary      Manage save files
// @Description  GET: List all saves for user, POST: Create/update save, DELETE: Delete save
// @Tags         Saves
// @Accept       json
// @Produce      json
// @Param        npub    path      string    true   "Nostr public key (npub format)"
// @Param        saveID  path      string    false  "Save ID (required for DELETE)"
// @Param        save    body      SaveFile  false  "Save data (for POST)"
// @Success      200     {array}   SaveListItem      "List of saves (GET)"
// @Success      200     {object}  SaveCreateResponse "Save created/updated (POST)"
// @Success      200     {object}  SaveDeleteResponse "Save deleted (DELETE)"
// @Failure      400     {string}  string  "Missing npub or invalid data"
// @Failure      404     {string}  string  "Save not found"
// @Failure      500     {string}  string  "Server error"
// @Router       /saves/{npub} [get]
// @Router       /saves/{npub} [post]
// @Router       /saves/{npub}/{saveID} [delete]
func SavesHandler(w http.ResponseWriter, r *http.Request) {
	// Extract npub from URL path: /api/saves/{npub}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/saves/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		http.Error(w, "Missing npub in URL", http.StatusBadRequest)
		return
	}

	npub := pathParts[0]

	switch r.Method {
	case "GET":
		handleGetSaves(w, r, npub)
	case "POST":
		handleCreateSave(w, r, npub)
	case "DELETE":
		if len(pathParts) < 2 {
			http.Error(w, "Missing save ID", http.StatusBadRequest)
			return
		}
		handleDeleteSave(w, r, npub, pathParts[1])
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Get all saves for a user
func handleGetSaves(w http.ResponseWriter, _ *http.Request, npub string) {
	log.Printf("📂 Loading saves for npub: %s", npub)

	savesDir := filepath.Join(SavesDirectory, npub)
	if _, err := os.Stat(savesDir); os.IsNotExist(err) {
		// No saves directory exists for this user
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]SaveFile{})
		return
	}

	files, err := ioutil.ReadDir(savesDir)
	if err != nil {
		log.Printf("❌ Error reading saves directory: %v", err)
		http.Error(w, "Failed to read saves", http.StatusInternalServerError)
		return
	}

	var saves []SaveFile
	for _, file := range files {
		if filepath.Ext(file.Name()) == ".json" {
			savePath := filepath.Join(savesDir, file.Name())
			if saveData, err := loadSaveFile(savePath); err == nil {
				saves = append(saves, *saveData)
			} else {
				log.Printf("⚠️ Failed to load save file %s: %v", file.Name(), err)
			}
		}
	}

	log.Printf("✅ Found %d saves for npub: %s", len(saves), npub)
	w.Header().Set("Content-Type", "application/json")

	// Convert saves to include id field in JSON output
	savesWithID := make([]map[string]interface{}, 0, len(saves))
	for _, save := range saves {
		saveMap := make(map[string]interface{})
		saveMap["id"] = save.InternalID
		saveMap["d"] = save.D
		saveMap["created_at"] = save.CreatedAt
		saveMap["race"] = save.Race
		saveMap["class"] = save.Class
		saveMap["background"] = save.Background
		saveMap["alignment"] = save.Alignment
		saveMap["experience"] = save.Experience
		saveMap["hp"] = save.HP
		saveMap["max_hp"] = save.MaxHP
		saveMap["mana"] = save.Mana
		saveMap["max_mana"] = save.MaxMana
		saveMap["fatigue"] = save.Fatigue
		saveMap["hunger"] = save.Hunger
		saveMap["stats"] = save.Stats
		saveMap["location"] = save.Location
		saveMap["district"] = save.District
		saveMap["building"] = save.Building
		saveMap["inventory"] = save.Inventory
		saveMap["vaults"] = save.Vaults
		saveMap["known_spells"] = save.KnownSpells
		saveMap["spell_slots"] = save.SpellSlots
		saveMap["locations_discovered"] = save.LocationsDiscovered
		saveMap["music_tracks_unlocked"] = save.MusicTracksUnlocked
		saveMap["current_day"] = save.CurrentDay
		saveMap["time_of_day"] = save.TimeOfDay
		savesWithID = append(savesWithID, saveMap)
	}

	json.NewEncoder(w).Encode(savesWithID)
}

// Create or update a save
func handleCreateSave(w http.ResponseWriter, r *http.Request, npub string) {
	// Block saves during active combat
	for _, sess := range session.GetSessionManager().GetAllSessions() {
		if sess.Npub == npub && sess.ActiveCombat != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]any{
				"success": false,
				"error":   "Cannot save during active combat",
			})
			return
		}
	}

	// First decode into a flexible map to handle any structure
	var rawData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawData); err != nil {
		log.Printf("❌ Error decoding save data: %v", err)
		http.Error(w, "Invalid save data", http.StatusBadRequest)
		return
	}


	// Convert back to JSON and then decode into SaveFile struct
	jsonData, err := json.Marshal(rawData)
	if err != nil {
		log.Printf("❌ Error marshaling save data: %v", err)
		http.Error(w, "Invalid save data", http.StatusInternalServerError)
		return
	}

	var saveData SaveFile
	if err := json.Unmarshal(jsonData, &saveData); err != nil {
		log.Printf("❌ Error unmarshaling save data: %v", err)
		http.Error(w, "Invalid save data", http.StatusBadRequest)
		return
	}

	// Set internal metadata (not serialized to JSON)
	saveData.InternalNpub = npub

	// Check if 'id' was provided in the request (for overwrites)
	if id, ok := rawData["id"].(string); ok && id != "" {
		saveData.InternalID = id
		log.Printf("📝 Overwriting existing save: %s", id)
	} else if saveData.InternalID == "" {
		// Generate new save ID only if none provided
		saveData.InternalID = fmt.Sprintf("save_%d", time.Now().Unix())
		saveData.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		log.Printf("✨ Creating new save: %s", saveData.InternalID)
	}

	// Ensure saves directory exists for this user
	userSavesDir := filepath.Join(SavesDirectory, npub)
	if err := os.MkdirAll(userSavesDir, 0755); err != nil {
		log.Printf("❌ Error creating saves directory: %v", err)
		http.Error(w, "Failed to create saves directory", http.StatusInternalServerError)
		return
	}

	// Write save file
	savePath := filepath.Join(userSavesDir, saveData.InternalID+".json")
	if err := WriteSaveFile(savePath, &saveData); err != nil {
		log.Printf("❌ Error writing save file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Saved game for npub: %s, save ID: %s", npub, saveData.InternalID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"save_id": saveData.InternalID,
		"message": "Game saved successfully",
	})
}

// Delete a save
func handleDeleteSave(w http.ResponseWriter, _ *http.Request, npub string, saveID string) {
	savePath := filepath.Join(SavesDirectory, npub, saveID+".json")

	if err := os.Remove(savePath); err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Save file not found", http.StatusNotFound)
		} else {
			log.Printf("❌ Error deleting save file: %v", err)
			http.Error(w, "Failed to delete save", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("🗑️ Deleted save: %s for npub: %s", saveID, npub)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Save deleted successfully",
	})
}

// LoadSaveByID re-exported from session package
func LoadSaveByID(npub, saveID string) (*SaveFile, error) {
	return session.LoadSaveByID(npub, saveID)
}

// WriteSaveFile re-exported from session package
func WriteSaveFile(path string, save *SaveFile) error {
	return session.WriteSaveFile(path, save)
}

// GetSaveInfo returns save file info for listings
func GetSaveInfo(npub, saveID string) (*SaveFile, error) {
	return LoadSaveByID(npub, saveID)
}

// loadSaveFile is a local helper for handlers in this file
func loadSaveFile(path string) (*SaveFile, error) {
	return session.LoadSaveFile(path)
}
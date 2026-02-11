package routes

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"pubkey-quest/cmd/server/auth"
	"pubkey-quest/cmd/server/utils"

	"github.com/0ceanslim/grain/client/core/tools"
)

// GameHandler serves the main game interface
func GameHandler(w http.ResponseWriter, r *http.Request) {
	// Extract query parameters
	saveID := r.URL.Query().Get("save")
	newGame := r.URL.Query().Get("new")

	data := utils.PageData{
		Title: "Pubkey Quest - Game",
		Theme: "dark",
		CustomData: map[string]interface{}{
			"GameMode":  true,
			"SaveID":    saveID,
			"NewGame":   newGame == "true",
			"DebugMode": utils.AppConfig.Server.DebugMode,
		},
	}

	utils.RenderTemplateWithLayout(w, data, "game.html", false, true)
}

// NewGameHandler serves the character generation page
func NewGameHandler(w http.ResponseWriter, r *http.Request) {
	data := utils.PageData{
		Title: "Pubkey Quest - New Adventure",
		Theme: "dark",
		CustomData: map[string]interface{}{
			"NewGameMode": true,
		},
	}

	utils.RenderTemplate(w, data, "game-intro.html", false)
}

// LoadSave serves the load save page
func LoadSave(w http.ResponseWriter, r *http.Request) {
	// Get current user session using grain
	session := auth.GetCurrentUser(r)
	if session == nil {
		http.Error(w, "No active session found", http.StatusUnauthorized)
		log.Println("❌ LoadSave: No active session found")
		return
	}

	// Generate npub from public key using grain tools
	npub, err := tools.EncodePubkey(session.PublicKey)
	if err != nil {
		http.Error(w, "Failed to encode npub", http.StatusInternalServerError)
		log.Printf("❌ LoadSave: Failed to encode npub: %v", err)
		return
	}

	// For now, create a basic profile from the session
	// In the future, this could fetch from Nostr relays
	content := map[string]interface{}{
		"name":         "Pubkey Quest Player",
		"display_name": "Player",
		"about":        "A brave adventurer in the Pubkey Quest realm",
		"npub":         npub,
	}

	// Fetch character data using the public key
	characterData, err := fetchCharacterData(npub)
	if err != nil {
		log.Printf("⚠️ LoadSave: Failed to fetch character data: %v", err)
		// Continue with empty character data if error occurs
		characterData = map[string]interface{}{}
	}

	// Prepare placeholder save data - this would be replaced with actual save data in future
	saveData := []map[string]interface{}{
		{
			"id":        "save1",
			"name":      "Adventure in the Dark Forest",
			"timestamp": "2025-04-01 14:30:22",
			"level":     5,
		},
		{
			"id":        "save2",
			"name":      "The Mountain Quest",
			"timestamp": "2025-04-05 18:12:45",
			"level":     8,
		},
	}

	// Create custom data for template
	customData := map[string]interface{}{
		"profile":   content,
		"character": characterData,
		"saves":     saveData,
		"npub":      npub,
	}

	// Render the template
	data := utils.PageData{
		Title:      "Load Game",
		Theme:      "dark", // Or get from user preferences
		CustomData: customData,
	}

	utils.RenderTemplate(w, data, "load-save.html", false)
}

// SavesHandler serves the save selection interface
func SavesHandler(w http.ResponseWriter, r *http.Request) {
	data := utils.PageData{
		Title: "Pubkey Quest - Select Save",
		Theme: "dark",
		CustomData: map[string]interface{}{
			"SavesMode": true,
		},
	}

	utils.RenderTemplate(w, data, "saves.html", false)
}

// fetchCharacterData is a helper function to fetch character data from API
func fetchCharacterData(npub string) (map[string]interface{}, error) {
	// Create the full URL with protocol, host and port
	port := utils.AppConfig.Server.Port
	apiURL := fmt.Sprintf("http://localhost:%d/api/character?npub=%s", port, npub)

	log.Printf("🔍 Fetching character data from: %s", apiURL)

	// Make API call to character endpoint
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to character API: %w", err)
	}
	defer resp.Body.Close()

	// Check if response is successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned non-OK status: %d", resp.StatusCode)
	}

	var characterData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&characterData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode character data: %w", err)
	}

	log.Printf("✅ Successfully retrieved character data for npub: %s", npub)
	return characterData, nil
}

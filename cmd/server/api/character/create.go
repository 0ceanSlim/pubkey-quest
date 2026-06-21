package character

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/0ceanslim/grain/client/core/tools"

	"pubkey-quest/cmd/server/db"
	gamecharacter "pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// SavesDirectory is the path to save files
const SavesDirectory = "data/saves"

// ============================================================================
// REQUEST/RESPONSE TYPES
// ============================================================================

// CreateCharacterRequest represents the frontend's simple equipment choices
// swagger:model CreateCharacterRequest
type CreateCharacterRequest struct {
	Npub             string            `json:"npub" example:"npub1..."`
	Name             string            `json:"name" example:"Aragorn"`
	EquipmentChoices map[string]string `json:"equipment_choices"` // e.g., {"choice-0": "scimitar", "choice-1": "shield"}
	PackChoice       string            `json:"pack_choice" example:"explorers-pack"`
}

// CreateCharacterResponse returns the save ID and full character data
// swagger:model CreateCharacterResponse
type CreateCharacterResponse struct {
	Success   bool        `json:"success" example:"true"`
	SaveID    string      `json:"save_id" example:"save_1234567890"`
	Character interface{} `json:"character,omitempty"`
	Error     string      `json:"error,omitempty" example:""`
}

// EquipmentChoice represents a single choice from starting-gear.json
type EquipmentChoice struct {
	Description string            `json:"description"`
	Options     []EquipmentOption `json:"options"`
}

// EquipmentOption represents one option in a choice
type EquipmentOption struct {
	Type     string          `json:"type"` // "single", "bundle", "multi_slot"
	Item     string          `json:"item,omitempty"`
	Quantity int             `json:"quantity,omitempty"`
	Items    []ItemWithQty   `json:"items,omitempty"` // For bundles
	Slots    []MultiSlotItem `json:"slots,omitempty"` // For multi_slot
}

// ItemWithQty represents an item with quantity
type ItemWithQty struct {
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
}

// MultiSlotItem represents a slot in a multi_slot choice
type MultiSlotItem struct {
	Type     string   `json:"type"` // "weapon_choice" or "fixed"
	Options  []string `json:"options,omitempty"`
	Item     string   `json:"item,omitempty"`
	Quantity int      `json:"quantity,omitempty"`
}

// StartingGearData represents the class-specific gear from JSON
type StartingGearData struct {
	Class        string `json:"class"`
	StartingGear struct {
		EquipmentChoices []EquipmentChoice `json:"equipment_choices"`
		PackChoice       *struct {
			Description string   `json:"description"`
			Options     []string `json:"options"`
		} `json:"pack_choice"`
		GivenItems []ItemWithQty `json:"given_items"`
	} `json:"starting_gear"`
}

// SaveFile type alias for backward compatibility
type SaveFile = types.SaveFile

// ActiveEffect type alias for backward compatibility
type ActiveEffect = types.ActiveEffect

// ============================================================================
// API HANDLER
// ============================================================================

// CreateCharacterHandler godoc
// @Summary      Create character
// @Description  Creates a new character with selected equipment and saves it to disk
// @Tags         Character
// @Accept       json
// @Produce      json
// @Param        request  body      CreateCharacterRequest  true  "Character creation data"
// @Success      200      {object}  CreateCharacterResponse
// @Failure      400      {object}  CreateCharacterResponse  "Invalid request"
// @Failure      405      {string}  string                   "Method not allowed"
// @Router       /character/create-save [post]
func CreateCharacterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateCharacterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ Error decoding request: %v", err)
		respondWithError(w, "Invalid request data")
		return
	}

	log.Printf("🎮 Creating character for npub: %s, name: %s", req.Npub, req.Name)

	// 1. Decode npub and generate character
	pubKey, err := tools.DecodeNpub(req.Npub)
	if err != nil {
		respondWithError(w, "Invalid npub")
		return
	}

	weightData, err := GetWeightsFromDB()
	if err != nil {
		respondWithError(w, "Failed to load weight data: "+err.Error())
		return
	}

	weightDataJSON, _ := json.Marshal(weightData)
	var weightDataStruct types.WeightData
	json.Unmarshal(weightDataJSON, &weightDataStruct)

	generatedChar := gamecharacter.GenerateCharacter(pubKey, &weightDataStruct)

	// 2. Get database connection
	database := db.GetDB()
	if database == nil {
		respondWithError(w, "Database not available")
		return
	}

	// 3. Load starting gear data
	startingGear, err := loadStartingGearForClass(database, generatedChar.Class)
	if err != nil {
		respondWithError(w, "Failed to load starting gear: "+err.Error())
		return
	}

	// 4. Get starting gold
	startingGold, err := getStartingGold(database, generatedChar.Background)
	if err != nil {
		log.Printf("⚠️  Failed to get starting gold: %v", err)
		startingGold = 1000 // Default
	}

	// 5. Build inventory from equipment choices
	inventory, err := buildInventoryFromChoices(database, startingGear, req.EquipmentChoices, req.PackChoice)
	if err != nil {
		respondWithError(w, "Failed to build inventory: "+err.Error())
		return
	}

	// 6. Add gold to inventory as an item
	err = AddGoldToInventory(inventory, startingGold)
	if err != nil {
		log.Printf("⚠️  Failed to add gold to inventory: %v", err)
	}

	// 7. Generate spell slots
	spellSlots, err := generateSpellSlots(database, generatedChar.Class)
	if err != nil {
		log.Printf("⚠️  Failed to generate spell slots: %v", err)
		spellSlots = make(map[string]interface{})
	}

	// 8. Load known spells
	knownSpells, err := loadKnownSpells(database, generatedChar.Class)
	if err != nil {
		log.Printf("⚠️  Failed to load known spells: %v", err)
		knownSpells = []string{}
	}

	// 9. Determine starting location based on race
	startingCity, err := getStartingCityForRace(database, generatedChar.Race)
	if err != nil {
		log.Printf("⚠️  Failed to get starting city: %v", err)
		startingCity = "millhaven"
	}

	// 10. Generate starting vault
	startingVault := generateStartingVault(startingCity)
	vaults := []map[string]interface{}{startingVault}

	// 12. Use location IDs directly (not display names)
	// startingCity is already an ID like "millhaven", "verdant", etc.
	locationID := startingCity
	districtKey := "center" // All characters start in the center district
	buildingID := ""        // Start outdoors

	// 13. Get music tracks (auto-unlock + location track)
	musicTracks := getAutoUnlockMusicTracks(database)
	locationMusic := getMusicTrackForLocation(database, startingCity)
	if locationMusic != "" {
		musicTracks = append(musicTracks, locationMusic)
	}

	// 14. Convert stats to interface{} map
	statsInterface := make(map[string]interface{})
	for k, v := range generatedChar.Stats {
		statsInterface[k] = v
	}

	// HP/Mana from the canonical derive functions (level 1) so a fresh character
	// matches exactly what Hydrate recomputes on every later load.
	hp := gamecharacter.DeriveMaxHP(generatedChar.Class, 1, statsInterface)
	mana := gamecharacter.DeriveMaxMana(generatedChar.Class, 1, statsInterface)

	// 15. Create save file
	saveFile := SaveFile{
		D:                   req.Name,
		CreatedAt:           time.Now().UTC().Format(time.RFC3339),
		Race:                generatedChar.Race,
		Class:               generatedChar.Class,
		Background:          generatedChar.Background,
		Alignment:           generatedChar.Alignment,
		SchemaVersion:       types.CurrentSchemaVersion,
		Experience:          0,
		HP:                  hp,
		MaxHP:               hp,
		Mana:                mana,
		MaxMana:             mana,
		Fatigue:             0,
		Hunger:              2, // Start satisfied (2 = Satisfied)
		Stats:               statsInterface,
		Location:            locationID,  // Use ID, not display name
		District:            districtKey, // Use key, not display name
		Building:            buildingID,  // Use ID, not display name
		CurrentDay:          1,
		TimeOfDay:           720, // Noon (12 PM) - stored in minutes (720 = 12*60)
		Inventory:           inventory,
		Vaults:              vaults,
		KnownSpells:         knownSpells,
		SpellSlots:          spellSlots,
		LocationsDiscovered: []string{startingCity}, // Only the city ID, not districts
		MusicTracksUnlocked: musicTracks,
		ActiveEffects: []ActiveEffect{
			{
				EffectID:          "fatigue-accumulation",
				EffectIndex:       0,
				DurationRemaining: 0, // Permanent effect
				DelayRemaining:    0,
				TickAccumulator:   0,
				AppliedAt:         720, // Noon (720 minutes = 12:00 PM)
			},
			{
				EffectID:          "hunger-accumulation-satisfied", // Start satisfied (Hunger=2)
				EffectIndex:       0,
				DurationRemaining: 0, // Permanent effect
				DelayRemaining:    0,
				TickAccumulator:   0,
				AppliedAt:         720, // Noon (720 minutes = 12:00 PM)
			},
		},
		InternalNpub: req.Npub,
		InternalID:   fmt.Sprintf("save_%d", time.Now().Unix()),
	}

	// 16. Save to disk
	if err := saveToDisk(req.Npub, &saveFile); err != nil {
		respondWithError(w, "Failed to save character: "+err.Error())
		return
	}

	log.Printf("✅ Character created successfully: %s", saveFile.InternalID)

	// 13. Respond with success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CreateCharacterResponse{
		Success: true,
		SaveID:  saveFile.InternalID,
		Character: map[string]interface{}{
			"name":       saveFile.D,
			"race":       saveFile.Race,
			"class":      saveFile.Class,
			"background": saveFile.Background,
			"alignment":  saveFile.Alignment,
			"hp":         saveFile.HP,
			"max_hp":     saveFile.MaxHP,
			"mana":       saveFile.Mana,
			"max_mana":   saveFile.MaxMana,
			"stats":      saveFile.Stats,
		},
	})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func respondWithError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(CreateCharacterResponse{
		Success: false,
		Error:   message,
	})
}

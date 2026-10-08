package character

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/0ceanslim/grain/client/core/tools"

	gamecharacter "pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// The roster a player can see from the very start.
//
// Characters derive from the npub, and each derived slot avoids the traits of
// the slots before it (see game/character/roster.go), so the whole roster is a
// pure function of the key. That means a locked slot can show the character
// actually waiting in it — "Level 10 · a Dwarf Barbarian" — which turns a grey
// box into something to play toward, and it needs nothing stored to do it.

// SlotUnlockLevels is the highest level, on any one of the player's characters,
// required to open each slot. Index 0 is slot 1.
var SlotUnlockLevels = []int{0, 5, 10, 15, 20}

// CharacterSlot is one row of the saves screen.
type CharacterSlot struct {
	Slot        int  `json:"slot"`
	UnlockLevel int  `json:"unlock_level"`
	Unlocked    bool `json:"unlocked"`
	// Custom marks the player-authored slot, which has no derived preview.
	Custom bool `json:"custom"`
	// Preview is who this slot would create. Absent for the custom slot.
	Preview *types.Character `json:"preview,omitempty"`
}

// CharacterSlotsResponse is the whole roster.
type CharacterSlotsResponse struct {
	Npub         string          `json:"npub"`
	HighestLevel int             `json:"highest_level"`
	Slots        []CharacterSlot `json:"slots"`
}

// CharacterSlotsHandler godoc
// @Summary      The player's character slots
// @Description  Every slot with the character it would create, whether it is
// @Description  unlocked, and the level that opens it. Derived slots avoid the
// @Description  traits of the slots before them, so a roster is several
// @Description  different people; the last slot is the player's own build.
// @Tags         Character
// @Produce      json
// @Param        npub           query     string  true   "Nostr public key"
// @Param        highest_level  query     int     false  "Highest level reached; defaults to 0"
// @Success      200  {object}  CharacterSlotsResponse
// @Failure      400  {string}  string  "Missing or invalid npub"
// @Router       /character/slots [get]
func CharacterSlotsHandler(w http.ResponseWriter, r *http.Request) {
	npub := r.URL.Query().Get("npub")
	if npub == "" {
		http.Error(w, "Missing npub parameter", http.StatusBadRequest)
		return
	}
	pubKey, err := tools.DecodeNpub(npub)
	if err != nil {
		http.Error(w, "Invalid npub", http.StatusBadRequest)
		return
	}

	highestLevel := 0
	if raw := r.URL.Query().Get("highest_level"); raw != "" {
		if _, err := fmt.Sscanf(raw, "%d", &highestLevel); err != nil || highestLevel < 0 {
			highestLevel = 0
		}
	}

	weightData, err := LoadWeightData()
	if err != nil {
		log.Printf("❌ Could not load generation weights: %v", err)
		http.Error(w, "Character generation data unavailable", http.StatusInternalServerError)
		return
	}

	roster := gamecharacter.GenerateRoster(pubKey, weightData)

	slots := make([]CharacterSlot, 0, len(SlotUnlockLevels))
	for i, unlockLevel := range SlotUnlockLevels {
		slotNumber := i + 1
		slot := CharacterSlot{
			Slot:        slotNumber,
			UnlockLevel: unlockLevel,
			Unlocked:    highestLevel >= unlockLevel,
			Custom:      slotNumber == gamecharacter.CustomSlot,
		}
		if !slot.Custom && i < len(roster) {
			preview := roster[i]
			slot.Preview = &preview
		}
		slots = append(slots, slot)
	}

	writeSlotsJSON(w, CharacterSlotsResponse{
		Npub:         npub,
		HighestLevel: highestLevel,
		Slots:        slots,
	})
}

// LoadWeightData reads the generation tables the roster derives from.
func LoadWeightData() (*types.WeightData, error) {
	weightDataMap, err := GetWeightsFromDB()
	if err != nil {
		return nil, err
	}
	blob, err := json.Marshal(weightDataMap)
	if err != nil {
		return nil, err
	}
	var weightData types.WeightData
	if err := json.Unmarshal(blob, &weightData); err != nil {
		return nil, err
	}
	return &weightData, nil
}

func writeSlotsJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("❌ Failed to encode slots response: %v", err)
	}
}

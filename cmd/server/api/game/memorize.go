package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	serverdb "pubkey-quest/cmd/server/db"
	gameSpells "pubkey-quest/cmd/server/game/spells"
)

// MemorizeScrollRequest names a scroll inside a spellbook container.
type MemorizeScrollRequest struct {
	Npub              string `json:"npub"`
	SaveID            string `json:"save_id"`
	ContainerSlot     int    `json:"container_slot"`
	ContainerSlotType string `json:"container_slot_type"` // "general" or "equipment"
	FromContainerSlot int    `json:"from_container_slot"`
	DryRun            bool   `json:"dry_run"` // report the cost only
}

// MemorizeScrollHandler godoc
// @Summary      Commit a scroll in your spellbook to memory (wizards)
// @Description  Learns the scroll's spell for good: costs gold and time per spell level and uses up the
// @Description  scroll. dry_run returns the cost without spending anything.
// @Tags         Progression
// @Accept       json
// @Produce      json
// @Param        request  body      MemorizeScrollRequest  true  "Scroll to memorize"
// @Success      200      {object}  map[string]interface{}
// @Router       /api/progression/memorize-scroll [post]
func MemorizeScrollHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req MemorizeScrollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Npub == "" || req.SaveID == "" {
		http.Error(w, "Missing required fields: npub, save_id", http.StatusBadRequest)
		return
	}
	sess := getSessionAndValidate(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if sess.ActiveCombat != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": "Not in the middle of a fight."})
		return
	}
	res, err := gameSpells.Memorize(serverdb.GetDB(), sess.GetSaveData(), req.ContainerSlot, req.ContainerSlotType, req.FromContainerSlot, req.DryRun)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	hours := fmt.Sprintf("%d hours", res.Minutes/60)
	if res.Minutes == 60 {
		hours = "1 hour"
	}
	if req.DryRun {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": res,
			"message": fmt.Sprintf("Commit %s to memory for %d gold and %s? The scroll is used up.", res.Name, res.Gold, hours)})
		return
	}
	log.Printf("📖 Memorized %s for %s (%d gold, %d min)", res.SpellID, req.Npub, res.Gold, res.Minutes)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": res, "color": "green",
		"message": fmt.Sprintf("You commit %s to memory. (%d gold, %s)", res.Name, res.Gold, hours)})
}

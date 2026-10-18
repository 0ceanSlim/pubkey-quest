package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	serverdb "pubkey-quest/cmd/server/db"
	gameSpells "pubkey-quest/cmd/server/game/spells"
)

// GetScribableHandler godoc
// @Summary      Scrolls this caster can scribe
// @Description  Every known spell up to their slot level, with gold, time, chance and what's missing.
// @Description  `here` is whether they stand at a station of their tradition (temple, herbary, mage tower).
// @Tags         Scrolls
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  map[string]interface{}
// @Router       /api/scrolls/scribable [get]
func GetScribableHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	npub, saveID := r.URL.Query().Get("npub"), r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		http.Error(w, "Missing query params: npub, save_id", http.StatusBadRequest)
		return
	}
	sess := getSessionAndValidate(w, npub, saveID)
	if sess == nil {
		return
	}
	st, err := gameSpells.Scribable(serverdb.GetDB(), sess.GetSaveData())
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": st})
}

// ScribeRequest scribes one scroll.
type ScribeRequest struct {
	Npub    string `json:"npub"`
	SaveID  string `json:"save_id"`
	SpellID string `json:"spell_id"`
}

// ScribeHandler godoc
// @Summary      Scribe a scroll
// @Description  Spends parchment, the spell's components, gold and time; rolls the casting ability. On a
// @Description  success the scroll is added to the inventory; on a failure everything is still spent.
// @Tags         Scrolls
// @Accept       json
// @Produce      json
// @Param        request  body      ScribeRequest  true  "Spell to scribe"
// @Success      200      {object}  map[string]interface{}
// @Router       /api/scrolls/scribe [post]
func ScribeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ScribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Npub == "" || req.SaveID == "" || req.SpellID == "" {
		http.Error(w, "Missing required fields: npub, save_id, spell_id", http.StatusBadRequest)
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
	res, err := gameSpells.Scribe(serverdb.GetDB(), sess.GetSaveData(), req.SpellID)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	msg := fmt.Sprintf("You scribe a Scroll of %s. (%d hours)", res.Name, res.Minutes/60)
	color := "green"
	if !res.Scribed {
		msg = fmt.Sprintf("The ink blots and the scroll of %s is ruined (rolled %d, needed %d or less). The materials are spent.", res.Name, res.Roll, res.Chance)
		color = "red"
	}
	log.Printf("📜 Scribe %s for %s: roll %d vs %d%% → scribed=%v", req.SpellID, req.Npub, res.Roll, res.Chance, res.Scribed)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": msg, "color": color, "data": res})
}

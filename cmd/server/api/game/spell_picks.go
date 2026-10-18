package game

import (
	"encoding/json"
	"log"
	"net/http"

	"pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/spellbook"
)

// SpellPickOption is one spell a character could learn now.
type SpellPickOption struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Level       int    `json:"level"`
	School      string `json:"school"`
	Description string `json:"description"`
	ManaCost    int    `json:"mana_cost"`
}

// SpellPicksResponse is how many spells a character may learn and which.
type SpellPicksResponse struct {
	Success bool              `json:"success"`
	Picks   int               `json:"picks"`
	Options []SpellPickOption `json:"options"`
}

// GetSpellPicksHandler godoc
// @Summary      Spells this character can learn now
// @Description  Known casters (bard, sorcerer, warlock, paladin, ranger) and wizards learn a spell for
// @Description  each step up their spells_known count; clerics and druids know their whole list (0 picks).
// @Tags         Progression
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  SpellPicksResponse
// @Router       /api/progression/spell-picks [get]
func GetSpellPicksHandler(w http.ResponseWriter, r *http.Request) {
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
	save := sess.GetSaveData()
	resp := SpellPicksResponse{Success: true, Picks: spellbook.Picks(save), Options: []SpellPickOption{}}
	if resp.Picks > 0 {
		byID := map[string]data.Spell{}
		if all, err := data.LoadAllSpells(serverdb.GetDB()); err == nil {
			for _, s := range all {
				byID[s.ID] = s
			}
		}
		for _, id := range spellbook.PickOptions(save) {
			s := byID[id]
			resp.Options = append(resp.Options, SpellPickOption{ID: id, Name: s.Name, Level: s.Level, School: s.School, Description: s.Description, ManaCost: s.ManaCost})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// LearnSpellRequest spends a pick on a spell.
type LearnSpellRequest struct {
	Npub    string `json:"npub"`
	SaveID  string `json:"save_id"`
	SpellID string `json:"spell_id"`
}

// LearnSpellHandler godoc
// @Summary      Learn a spell (spends a spell pick)
// @Tags         Progression
// @Accept       json
// @Produce      json
// @Param        request  body      LearnSpellRequest  true  "Spell to learn"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "No pick, not a class spell, too high, or already known"
// @Failure      409      {string}  string  "In active combat"
// @Router       /api/progression/learn-spell [post]
func LearnSpellHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LearnSpellRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.SpellID == "" {
		http.Error(w, "Missing required fields: npub, save_id, spell_id", http.StatusBadRequest)
		return
	}
	sess := getSessionAndValidate(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}
	if sess.ActiveCombat != nil {
		http.Error(w, "Cannot learn a spell during combat", http.StatusConflict)
		return
	}
	save := sess.GetSaveData()
	if err := spellbook.Learn(save, req.SpellID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("📖 Spell learned: %s for %s — %d picks left", req.SpellID, req.Npub, spellbook.Picks(save))
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"spell_id":     req.SpellID,
		"picks":        spellbook.Picks(save),
		"known_spells": spellbook.Known(save),
	})
}

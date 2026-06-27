package game

import (
	"encoding/json"
	"log"
	"net/http"

	"pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/session"
)

// SkillsHandler returns skill definitions with calculated values for a character session
// GET /api/skills?npub={npub}&save_id={save_id}
func SkillsHandler(w http.ResponseWriter, r *http.Request) {
	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")

	skills, err := data.LoadSkillDefinitions()
	if err != nil {
		log.Printf("❌ Failed to load skill definitions: %v", err)
		http.Error(w, "Failed to load skill definitions", http.StatusInternalServerError)
		return
	}

	// If no character context, return definitions only
	if npub == "" || saveID == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"skills": skills,
		})
		return
	}

	// Get session to read character stats
	sm := session.GetSessionManager()
	gameSession, err := sm.GetSession(npub, saveID)
	if err != nil || gameSession == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	// Effective stats (base + active effect modifiers) so the displayed skill
	// matches the value a check or gate uses.
	stats := effects.EffectiveStats(&gameSession.SaveData)

	// Calculate values for each skill
	result := make(map[string]data.SkillResponse)
	for id, skill := range skills {
		result[id] = data.SkillResponse{
			Name:        skill.Name,
			Description: skill.Description,
			Ratio:       skill.Ratio,
			Value:       data.CalculateSkillValue(stats, skill.Ratio),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"skills": result,
	})
}

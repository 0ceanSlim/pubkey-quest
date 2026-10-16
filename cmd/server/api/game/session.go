package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/cmd/server/ledger"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/utils"
	"pubkey-quest/types"
)

// SessionRequest represents a session identification request
// swagger:model SessionRequest
type SessionRequest struct {
	Npub   string `json:"npub" example:"npub1..."`
	SaveID string `json:"save_id" example:"save_1234567890"`
}

// SessionResponse represents a session operation response
// swagger:model SessionResponse
type SessionResponse struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Session initialized successfully"`
	Session struct {
		Npub     string `json:"npub" example:"npub1..."`
		SaveID   string `json:"save_id" example:"save_1234567890"`
		LoadedAt string `json:"loaded_at" example:"2025-01-15T12:00:00Z"`
	} `json:"session,omitempty"`
}

// InitSessionHandler godoc
// @Summary      Initialize session
// @Description  Loads a save file into memory for gameplay
// @Tags         Session
// @Accept       json
// @Produce      json
// @Param        request  body      SessionRequest  true  "Session identification"
// @Success      200      {object}  SessionResponse
// @Failure      400      {string}  string  "Missing npub or save_id"
// @Failure      405      {string}  string  "Method not allowed"
// @Failure      500      {string}  string  "Failed to initialize session"
// @Router       /session/init [post]
func InitSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Npub   string `json:"npub"`
		SaveID string `json:"save_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Npub == "" || request.SaveID == "" {
		http.Error(w, "Missing npub or save_id", http.StatusBadRequest)
		return
	}

	// Load session into memory
	sess, err := session.GetSessionManager().LoadSession(request.Npub, request.SaveID)
	if err != nil {
		log.Printf("❌ Failed to initialize session: %v", err)
		http.Error(w, fmt.Sprintf("Failed to initialize session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Session initialized successfully",
		"session": map[string]any{
			"npub":      sess.Npub,
			"save_id":   sess.SaveID,
			"loaded_at": sess.LoadedAt,
			// With relay saves a new character only lives on this server until
			// the player signs a first save; the game prompts for it.
			"needs_first_save": needsFirstSave(sess.Npub, sess.SaveID),
		},
	})
}

// needsFirstSave: relay saves are on and this character has never been signed
// (no cached save event, no ledger record).
func needsFirstSave(npub, saveID string) bool {
	if !utils.AppConfig.Saves.Relay {
		return false
	}
	if _, err := os.Stat(filepath.Join(session.SavesDirectory, npub, saveID+".event.json")); err == nil {
		return false
	}
	if _, ok := ledger.Get(npub, SaveDTag(saveID)); ok {
		return false
	}
	return true
}

// ReloadSessionHandler godoc
// @Summary      Reload session
// @Description  Force reload from disk, discarding all in-memory changes
// @Tags         Session
// @Accept       json
// @Produce      json
// @Param        request  body      SessionRequest  true  "Session identification"
// @Success      200      {object}  SessionResponse
// @Failure      400      {string}  string  "Missing npub or save_id"
// @Failure      405      {string}  string  "Method not allowed"
// @Failure      500      {string}  string  "Failed to reload session"
// @Router       /session/reload [post]
func ReloadSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Npub   string `json:"npub"`
		SaveID string `json:"save_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Npub == "" || request.SaveID == "" {
		http.Error(w, "Missing npub or save_id", http.StatusBadRequest)
		return
	}

	// Force reload session from disk
	sess, err := session.GetSessionManager().ReloadSession(request.Npub, request.SaveID)
	if err != nil {
		log.Printf("❌ Failed to reload session: %v", err)
		http.Error(w, fmt.Sprintf("Failed to reload session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Session reloaded from disk successfully",
		"session": map[string]any{
			"npub":       sess.Npub,
			"save_id":    sess.SaveID,
			"loaded_at":  sess.LoadedAt,
			"updated_at": sess.UpdatedAt,
		},
	})
}

// GetSessionHandler godoc
// @Summary      Get session state
// @Description  Retrieve current in-memory session state including character data and active effects
// @Tags         Session
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save file ID"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "Missing npub or save_id"
// @Failure      404      {string}  string  "Session not found"
// @Failure      405      {string}  string  "Method not allowed"
// @Router       /session/state [get]
func GetSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")

	if npub == "" || saveID == "" {
		http.Error(w, "Missing npub or save_id", http.StatusBadRequest)
		return
	}

	sessionMgr := session.GetSessionManager()

	// Get session from memory
	sess, err := sessionMgr.GetSession(npub, saveID)
	if err != nil {
		// If not in memory, try to load it
		sess, err = sessionMgr.LoadSession(npub, saveID)
		if err != nil {
			log.Printf("❌ Failed to get session: %v", err)
			http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
			return
		}
	}

	// Calculate weight and capacity (NOT stored in save file - calculated on-the-fly)
	totalWeight := status.CalculateTotalWeight(&sess.SaveData)
	weightCapacity := status.CalculateWeightCapacity(&sess.SaveData)

	// Create response with enriched active effects
	response := map[string]interface{}{
		"d":                     sess.SaveData.D,
		"created_at":            sess.SaveData.CreatedAt,
		"race":                  sess.SaveData.Race,
		"class":                 sess.SaveData.Class,
		"background":            sess.SaveData.Background,
		"alignment":             sess.SaveData.Alignment,
		"experience":            sess.SaveData.Experience,
		"hp":                    sess.SaveData.HP,
		"max_hp":                sess.SaveData.MaxHP,
		"mana":                  sess.SaveData.Mana,
		"max_mana":              sess.SaveData.MaxMana,
		"fatigue":               sess.SaveData.Fatigue,
		"hunger":                sess.SaveData.Hunger,
		"stats":                 sess.SaveData.Stats,
		"location":              sess.SaveData.Location,
		"district":              sess.SaveData.District,
		"building":              sess.SaveData.Building,
		"room":                  sess.SaveData.Room,
		"current_day":           sess.SaveData.CurrentDay,
		"time_of_day":           sess.SaveData.TimeOfDay,
		"inventory":             sess.SaveData.Inventory,
		"vault":                 vault.Response(&sess.SaveData),
		"known_spells":          sess.SaveData.KnownSpells,
		"spell_slots":           sess.SaveData.SpellSlots,
		"locations_discovered":  sess.SaveData.LocationsDiscovered,
		"music_tracks_unlocked": sess.SaveData.MusicTracksUnlocked,
		"active_effects":        effects.EnrichActiveEffects(sess.SaveData.ActiveEffects, &sess.SaveData),

		// Add calculated values (NOT persisted - calculated at runtime)
		"total_weight":    totalWeight,
		"weight_capacity": weightCapacity,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateSessionRequest represents a session update request
// swagger:model UpdateSessionRequest
type UpdateSessionRequest struct {
	Npub     string                 `json:"npub" example:"npub1..."`
	SaveID   string                 `json:"save_id" example:"save_1234567890"`
	SaveData map[string]interface{} `json:"save_data"`
}

// UpdateSessionHandler godoc
// @Summary      Update session
// @Description  Update in-memory game state with new data
// @Tags         Session
// @Accept       json
// @Produce      json
// @Param        request  body      UpdateSessionRequest  true  "Session update data"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "Invalid request"
// @Failure      405      {string}  string  "Method not allowed"
// @Failure      500      {string}  string  "Failed to update session"
// @Router       /session/update [post]
func UpdateSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Npub     string         `json:"npub"`
		SaveID   string         `json:"save_id"`
		SaveData map[string]any `json:"save_data"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Npub == "" || request.SaveID == "" {
		http.Error(w, "Missing npub or save_id", http.StatusBadRequest)
		return
	}

	// Convert map to SaveFile struct
	jsonData, err := json.Marshal(request.SaveData)
	if err != nil {
		http.Error(w, "Invalid save data", http.StatusBadRequest)
		return
	}

	var saveData types.SaveFile
	if err := json.Unmarshal(jsonData, &saveData); err != nil {
		http.Error(w, "Invalid save data format", http.StatusBadRequest)
		return
	}

	// Set internal metadata
	saveData.InternalNpub = request.Npub
	saveData.InternalID = request.SaveID

	// Update session in memory
	if err := session.GetSessionManager().UpdateSession(request.Npub, request.SaveID, saveData); err != nil {
		log.Printf("❌ Failed to update session: %v", err)
		http.Error(w, fmt.Sprintf("Failed to update session: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Session updated successfully",
	})
}

// SaveSessionHandler godoc
// @Summary      Save session
// @Description  Write in-memory session state to disk
// @Tags         Session
// @Accept       json
// @Produce      json
// @Param        request  body      SessionRequest  true  "Session identification"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "Missing npub or save_id"
// @Failure      404      {string}  string  "Session not found in memory"
// @Failure      405      {string}  string  "Method not allowed"
// @Failure      500      {string}  string  "Failed to write save file"
// @Router       /session/save [post]
func SaveSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Npub   string `json:"npub"`
		SaveID string `json:"save_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Npub == "" || request.SaveID == "" {
		http.Error(w, "Missing npub or save_id", http.StatusBadRequest)
		return
	}

	// Get session from memory
	sess, err := session.GetSessionManager().GetSession(request.Npub, request.SaveID)
	if err != nil {
		log.Printf("❌ Session not found in memory: %v", err)
		http.Error(w, "Session not found in memory", http.StatusNotFound)
		return
	}

	// Saves capture a resting state: never mid-fight or mid-something, in any
	// save mode (this used to be checked only on the relay path).
	if why := saveBlocked(sess); why != "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"success": false, "message": why})
		return
	}

	// Items mid-sale go back in the pack before anything is saved.
	ReturnStagedSales(sess)

	// Relay saves: hand the browser the event to sign; nothing is written until
	// a relay accepts it (POST /api/saves/publish).
	if utils.AppConfig.Saves.Relay {
		unsigned, err := prepareRelaySave(sess, request.SaveID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"success": false, "message": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "Sign your save to keep it",
			"save_id": request.SaveID,
			"sign":    unsigned,
		})
		return
	}

	// Write to disk using existing save logic
	savePath := session.GetSavePath(request.Npub, request.SaveID)
	if err := session.WriteSaveFile(savePath, &sess.SaveData); err != nil {
		log.Printf("❌ Failed to write save file: %v", err)
		http.Error(w, "Failed to write save file", http.StatusInternalServerError)
		return
	}

	// The deliberate save is now the authoritative state — drop the crash journal.
	session.RemoveJournal(request.Npub, request.SaveID)

	log.Printf("✅ Session saved to disk: %s:%s", request.Npub, request.SaveID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Game saved successfully",
		"save_id": request.SaveID,
	})
}

// DebugStateHandler godoc
// @Summary      Get session state (debug)
// @Description  Returns detailed session state for debugging (debug mode only)
// @Tags         Debug
// @Produce      json
// @Param        npub     query     string  false  "Nostr public key"
// @Param        save_id  query     string  false  "Save file ID"
// @Success      200      {object}  map[string]interface{}
// @Failure      403      {string}  string  "Debug mode disabled"
// @Failure      404      {string}  string  "Session not found"
// @Failure      405      {string}  string  "Method not allowed"
// @Router       /debug/state [get]
func DebugStateHandler(w http.ResponseWriter, r *http.Request, debugMode bool) {
	if !debugMode {
		http.Error(w, "Debug mode disabled", http.StatusForbidden)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get npub and save_id from URL or from active sessions
	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")

	sessionMgr := session.GetSessionManager()

	// If no parameters, return all sessions
	if npub == "" || saveID == "" {
		sessions := sessionMgr.GetAllSessions()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"success":       true,
			"session_count": len(sessions),
			"sessions":      sessions,
		})
		return
	}

	// Get specific session
	sess, err := sessionMgr.GetSession(npub, saveID)
	if err != nil {
		// Try to load it if not in memory
		sess, err = sessionMgr.LoadSession(npub, saveID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"session": sess,
	})
}

// CleanupSessionHandler godoc
// @Summary      Cleanup session
// @Description  Remove a session from memory without saving
// @Tags         Session
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save file ID"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "Missing npub or save_id"
// @Failure      405      {string}  string  "Method not allowed"
// @Router       /session/cleanup [delete]
func CleanupSessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")

	if npub == "" || saveID == "" {
		http.Error(w, "Missing npub or save_id", http.StatusBadRequest)
		return
	}

	leaveSocial(npub, saveID)
	session.GetSessionManager().UnloadSession(npub, saveID)
	// Clean quit: unsaved progress is intentionally discarded (revert to last
	// deliberate save), so the crash journal goes too.
	session.RemoveJournal(npub, saveID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Session unloaded from memory",
	})
}

// saveBlocked says why the game can't be saved right now ("" when it can):
// mid-fight, with something about to notice you, partway through a POI or
// encounter, or with a pick in a lock.
func saveBlocked(sess *session.GameSession) string {
	switch {
	case sess.ActiveCombat != nil:
		return "You can't save during a fight."
	case sess.PendingEncounter != nil:
		return "You can't save with something this close. Deal with it first."
	case sess.ActivePOI != nil:
		return "Finish what you're doing here before you save."
	case sess.LockSession != nil:
		return "Take your pick out of the lock before you save."
	}
	return ""
}

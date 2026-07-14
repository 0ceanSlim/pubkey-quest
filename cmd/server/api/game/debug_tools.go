package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/session"
)

// debug_tools.go holds the developer conveniences surfaced in the in-game Debug
// Console (debug builds only). Each handler mirrors DebugStateHandler's session
// resolution and, like every mutation, mutates the in-memory GameSession (the
// authoritative live state) and returns a delta so the client reflects it without
// a disk save. They are registered ONLY inside registerDebugRoutes.

// resolveDebugSession pulls the live (or freshly loaded) session for npub+save_id,
// writing the appropriate HTTP error on failure. Returns nil when it has handled
// the error.
func resolveDebugSession(w http.ResponseWriter, npub, saveID string) *session.GameSession {
	if npub == "" || saveID == "" {
		http.Error(w, "npub and save_id are required", http.StatusBadRequest)
		return nil
	}
	mgr := session.GetSessionManager()
	sess, err := mgr.GetSession(npub, saveID)
	if err != nil {
		if sess, err = mgr.LoadSession(npub, saveID); err != nil {
			http.Error(w, fmt.Sprintf("Session not found: %v", err), http.StatusNotFound)
			return nil
		}
	}
	return sess
}

// debugGuard enforces debug-only + POST + JSON decode. Returns false when it has
// already written a response.
func debugGuard(w http.ResponseWriter, r *http.Request, debugMode bool, body any) bool {
	if !debugMode {
		http.Error(w, "Debug mode disabled", http.StatusForbidden)
		return false
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	if err := json.NewDecoder(r.Body).Decode(body); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return false
	}
	return true
}

// DebugGrantXPHandler adds XP to the character via the canonical character.GrantXP
// (which re-derives level/MaxHP/MaxMana and full-heals on a level-up), then returns
// the delta + any level-up so the client updates and shows the level-up modal.
//
// POST /api/debug/grant-xp  { npub, save_id, amount }
func DebugGrantXPHandler(w http.ResponseWriter, r *http.Request, debugMode bool) {
	var req struct {
		Npub   string `json:"npub"`
		SaveID string `json:"save_id"`
		Amount int    `json:"amount"`
	}
	if !debugGuard(w, r, debugMode, &req) {
		return
	}
	if req.Amount == 0 {
		http.Error(w, "amount must be non-zero", http.StatusBadRequest)
		return
	}

	sess := resolveDebugSession(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}

	adv, err := loadAdvancement()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load advancement table: %v", err), http.StatusInternalServerError)
		return
	}

	result := character.GrantXP(&sess.SaveData, req.Amount, adv)
	log.Printf("🐛 Debug: granted %d XP to %s (leveled=%v)", req.Amount, req.Npub, result.Leveled)

	resp := map[string]any{
		"success": true,
		"message": fmt.Sprintf("Granted %d XP", req.Amount),
		"data":    map[string]any{"level_up": result},
	}
	if d := sess.UpdateSnapshotAndCalculateDelta(); d != nil && !d.IsEmpty() {
		resp["delta"] = d.ToMap()
	}
	writeDebugJSON(w, resp)
}

// DebugTeleportHandler drops the player into any settlement for testing. It sets
// the location and clears travel/building/room state so the scene re-renders
// cleanly. District defaults to "center" (every city has one).
//
// POST /api/debug/teleport  { npub, save_id, location, district? }
func DebugTeleportHandler(w http.ResponseWriter, r *http.Request, debugMode bool) {
	var req struct {
		Npub     string `json:"npub"`
		SaveID   string `json:"save_id"`
		Location string `json:"location"`
		District string `json:"district"`
	}
	if !debugGuard(w, r, debugMode, &req) {
		return
	}
	if req.Location == "" {
		http.Error(w, "location is required", http.StatusBadRequest)
		return
	}
	district := req.District
	if district == "" {
		district = "center"
	}

	sess := resolveDebugSession(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}

	save := &sess.SaveData
	save.Location = req.Location
	save.District = district
	save.Building = ""
	save.Room = ""
	save.TravelProgress = 0
	save.TravelStopped = false

	// Treat a teleport as a discovery so quest "explore" objectives can advance.
	events.Record(save, events.LocationDiscovered, req.Location, 1)
	log.Printf("🐛 Debug: teleported %s to %s/%s", req.Npub, req.Location, district)

	writeDebugJSON(w, map[string]any{
		"success":  true,
		"message":  fmt.Sprintf("Teleported to %s", req.Location),
		"location": req.Location,
		"district": district,
	})
}

func writeDebugJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

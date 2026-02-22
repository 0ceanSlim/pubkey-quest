package game

import (
	"encoding/json"
	"log"
	"net/http"

	"pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/db"
	gameSpells "pubkey-quest/cmd/server/game/spells"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// ============================================================================
// Request / Response types
// ============================================================================

// SpellPrepRequest is sent when the player starts preparing a spell.
type SpellPrepRequest struct {
	Npub      string `json:"npub"`
	SaveID    string `json:"save_id"`
	SpellID   string `json:"spell_id"`
	SlotLevel string `json:"slot_level"` // "cantrips", "level_1", etc.
	SlotIndex int    `json:"slot_index"`
}

// SpellPrepResponse is returned by POST /api/spells/prepare.
type SpellPrepResponse struct {
	Success        bool   `json:"success"`
	Message        string `json:"message,omitempty"`
	SpellID        string `json:"spell_id,omitempty"`
	SlotLevel      string `json:"slot_level,omitempty"`
	SlotIndex      int    `json:"slot_index,omitempty"`
	ReadyAtAbsolute int   `json:"ready_at_absolute,omitempty"`
	ReadyInMinutes int    `json:"ready_in_minutes,omitempty"`
}

// PrepQueueItem is a single task in the queue as returned to the client.
type PrepQueueItem struct {
	SpellID        string `json:"spell_id"`
	SlotLevel      string `json:"slot_level"`
	SlotIndex      int    `json:"slot_index"`
	ReadyAtAbsolute int   `json:"ready_at_absolute"`
	ReadyInMinutes int    `json:"ready_in_minutes"`
	IsReady        bool   `json:"is_ready"`
}

// PrepQueueResponse is returned by GET /api/spells/prep-queue.
type PrepQueueResponse struct {
	Success bool            `json:"success"`
	Tasks   []PrepQueueItem `json:"tasks"`
}

// SpellSlotRequest identifies a specific slot for cancel/unslot operations.
type SpellSlotRequest struct {
	Npub      string `json:"npub"`
	SaveID    string `json:"save_id"`
	SlotLevel string `json:"slot_level"`
	SlotIndex int    `json:"slot_index"`
}

// ============================================================================
// Helpers
// ============================================================================

// getSpellLevel fetches a spell's level from the database.
// Returns -1 and logs an error if the spell is not found.
func getSpellLevel(spellID string) int {
	database := db.GetDB()
	if database == nil {
		return -1
	}
	spellList, err := data.LoadAllSpells(database)
	if err != nil {
		return -1
	}
	for _, s := range spellList {
		if s.ID == spellID {
			return s.Level
		}
	}
	return -1
}

// getSessionAndValidate looks up a session and returns 404 if missing.
func getSessionAndValidate(w http.ResponseWriter, npub, saveID string) *session.GameSession {
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		http.Error(w, "Session not found — call /api/session/init first", http.StatusNotFound)
		return nil
	}
	return sess
}

// expectedSlotLevel maps a spell level integer to the slot-level string.
func expectedSlotLevel(spellLevel int) string {
	if sl, ok := gameSpells.SpellLevelToSlotLevel[spellLevel]; ok {
		return sl
	}
	return ""
}

// ============================================================================
// Handlers
// ============================================================================

// PrepareSpellHandler godoc
// @Summary      Prepare a spell for a slot
// @Description  Queues a leveled spell for preparation (level×60 minutes) or instantly places a cantrip.
//               The slot must exist in the player's spell_slots and not have an active prep task.
//               Cantrips (level 0) are placed immediately with ready_in_minutes=0.
// @Tags         Spells
// @Accept       json
// @Produce      json
// @Param        request  body      SpellPrepRequest   true  "Preparation request"
// @Success      200      {object}  SpellPrepResponse
// @Failure      400      {string}  string  "Validation error"
// @Failure      404      {string}  string  "Session or spell not found"
// @Router       /api/spells/prepare [post]
func PrepareSpellHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SpellPrepRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.SpellID == "" || req.SlotLevel == "" {
		http.Error(w, "Missing required fields: npub, save_id, spell_id, slot_level", http.StatusBadRequest)
		return
	}

	sess := getSessionAndValidate(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}

	// Validate spell is known
	knownFound := false
	for _, k := range sess.SaveData.KnownSpells {
		if k == req.SpellID {
			knownFound = true
			break
		}
	}
	if !knownFound {
		http.Error(w, "Spell not in known_spells", http.StatusBadRequest)
		return
	}

	// Fetch spell level from DB
	spellLevel := getSpellLevel(req.SpellID)
	if spellLevel < 0 {
		http.Error(w, "Spell not found in database", http.StatusNotFound)
		return
	}

	// Validate slot_level matches spell level
	want := expectedSlotLevel(spellLevel)
	if want == "" || want != req.SlotLevel {
		http.Error(w, "slot_level does not match spell level (spell level "+
			gameSpells.SpellLevelToSlotLevel[spellLevel]+" required)", http.StatusBadRequest)
		return
	}

	// Validate the slot exists in the player's spell_slots
	slots := sess.SaveData.SpellSlots
	if !gameSpells.HasSlot(slots, req.SlotLevel, req.SlotIndex) {
		http.Error(w, "Slot does not exist for this character", http.StatusBadRequest)
		return
	}

	// Cantrips: instant placement
	if spellLevel == 0 {
		if !gameSpells.SetSpellInSlot(slots, req.SlotLevel, req.SlotIndex, req.SpellID) {
			http.Error(w, "Failed to place cantrip in slot", http.StatusInternalServerError)
			return
		}
		log.Printf("🔮 Cantrip placed: %s → %s[%d] for %s", req.SpellID, req.SlotLevel, req.SlotIndex, req.Npub)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SpellPrepResponse{
			Success:        true,
			SpellID:        req.SpellID,
			SlotLevel:      req.SlotLevel,
			SlotIndex:      req.SlotIndex,
			ReadyAtAbsolute: gameSpells.AbsoluteMinutes(sess.SaveData.CurrentDay, sess.SaveData.TimeOfDay),
			ReadyInMinutes: 0,
		})
		return
	}

	// Check for duplicate prep task targeting the same slot
	if idx := gameSpells.FindPrepTask(sess.PrepQueue, req.SlotLevel, req.SlotIndex); idx >= 0 {
		// Cancel the existing task before adding a new one (re-slotting is allowed)
		sess.PrepQueue = gameSpells.RemovePrepTask(sess.PrepQueue, idx)
	}

	// Queue the prep task
	prepMins := gameSpells.PrepMinutes(spellLevel)
	readyAt := gameSpells.AbsoluteMinutes(sess.SaveData.CurrentDay, sess.SaveData.TimeOfDay) + prepMins
	task := types.SpellPrepTask{
		SpellID:         req.SpellID,
		SlotLevel:       req.SlotLevel,
		SlotIndex:       req.SlotIndex,
		ReadyAtAbsolute: readyAt,
	}
	sess.PrepQueue = append(sess.PrepQueue, task)

	log.Printf("🔮 Spell prep queued: %s → %s[%d], ready in %d min (at %d) for %s",
		req.SpellID, req.SlotLevel, req.SlotIndex, prepMins, readyAt, req.Npub)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(SpellPrepResponse{
		Success:        true,
		SpellID:        req.SpellID,
		SlotLevel:      req.SlotLevel,
		SlotIndex:      req.SlotIndex,
		ReadyAtAbsolute: readyAt,
		ReadyInMinutes: prepMins,
	})
}

// GetPrepQueueHandler godoc
// @Summary      Get spell preparation queue
// @Description  Returns all in-progress spell prep tasks and resolves any that are ready.
//               Lazy resolution: tasks that finished since the last check are completed here.
// @Tags         Spells
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  PrepQueueResponse
// @Failure      404      {string}  string  "Session not found"
// @Router       /api/spells/prep-queue [get]
func GetPrepQueueHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		http.Error(w, "Missing query params: npub, save_id", http.StatusBadRequest)
		return
	}

	sess := getSessionAndValidate(w, npub, saveID)
	if sess == nil {
		return
	}

	// Lazy-resolve any finished tasks
	gameSpells.ResolvePrepTimers(sess)

	// Build response
	currentAbs := gameSpells.AbsoluteMinutes(sess.SaveData.CurrentDay, sess.SaveData.TimeOfDay)
	items := make([]PrepQueueItem, 0, len(sess.PrepQueue))
	for _, t := range sess.PrepQueue {
		mins := gameSpells.MinutesRemaining(t, sess.SaveData.CurrentDay, sess.SaveData.TimeOfDay)
		items = append(items, PrepQueueItem{
			SpellID:        t.SpellID,
			SlotLevel:      t.SlotLevel,
			SlotIndex:      t.SlotIndex,
			ReadyAtAbsolute: t.ReadyAtAbsolute,
			ReadyInMinutes: mins,
			IsReady:        t.ReadyAtAbsolute <= currentAbs,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PrepQueueResponse{
		Success: true,
		Tasks:   items,
	})
}

// CancelPrepHandler godoc
// @Summary      Cancel a spell preparation task
// @Description  Removes the matching prep task from the queue. The slot is left unchanged.
// @Tags         Spells
// @Accept       json
// @Produce      json
// @Param        request  body      SpellSlotRequest  true  "Slot to cancel"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "Slot not in prep queue"
// @Failure      404      {string}  string  "Session not found"
// @Router       /api/spells/cancel-prep [post]
func CancelPrepHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SpellSlotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.SlotLevel == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	sess := getSessionAndValidate(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}

	idx := gameSpells.FindPrepTask(sess.PrepQueue, req.SlotLevel, req.SlotIndex)
	if idx < 0 {
		http.Error(w, "No prep task found for this slot", http.StatusBadRequest)
		return
	}

	cancelledID := sess.PrepQueue[idx].SpellID
	sess.PrepQueue = gameSpells.RemovePrepTask(sess.PrepQueue, idx)

	log.Printf("🔮 Spell prep cancelled: %s from %s[%d] for %s", cancelledID, req.SlotLevel, req.SlotIndex, req.Npub)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":           true,
		"cancelled_spell_id": cancelledID,
	})
}

// UnslotSpellHandler godoc
// @Summary      Remove a spell from a slot
// @Description  Sets the spell in the given slot to null and cancels any in-progress prep for that slot.
// @Tags         Spells
// @Accept       json
// @Produce      json
// @Param        request  body      SpellSlotRequest  true  "Slot to clear"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "Slot not found"
// @Failure      404      {string}  string  "Session not found"
// @Router       /api/spells/unslot [post]
func UnslotSpellHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SpellSlotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.SlotLevel == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	sess := getSessionAndValidate(w, req.Npub, req.SaveID)
	if sess == nil {
		return
	}

	slots := sess.SaveData.SpellSlots
	if !gameSpells.HasSlot(slots, req.SlotLevel, req.SlotIndex) {
		http.Error(w, "Slot does not exist for this character", http.StatusBadRequest)
		return
	}

	// Clear the slot
	gameSpells.ClearSpellInSlot(slots, req.SlotLevel, req.SlotIndex)

	// Cancel any in-progress prep for this slot
	if idx := gameSpells.FindPrepTask(sess.PrepQueue, req.SlotLevel, req.SlotIndex); idx >= 0 {
		sess.PrepQueue = gameSpells.RemovePrepTask(sess.PrepQueue, idx)
	}

	log.Printf("🔮 Spell unslotted: %s[%d] for %s", req.SlotLevel, req.SlotIndex, req.Npub)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"slot_level": req.SlotLevel,
		"slot_index": req.SlotIndex,
	})
}

package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/combat"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// ─── Request / Response models ───────────────────────────────────────────────

// CombatStartRequest is the body sent to POST /combat/start.
// swagger:model CombatStartRequest
type CombatStartRequest struct {
	Npub          string `json:"npub"           example:"npub1..."`
	SaveID        string `json:"save_id"        example:"save_1234567890"`
	MonsterID     string `json:"monster_id"     example:"goblin"`
	EnvironmentID string `json:"environment_id" example:"forest"`
}

// CombatActionRequest is the body sent to POST /combat/action.
// weapon_slot must be "mainHand", "offHand", or "unarmed".
// move_dir: -1 = step closer, 0 = hold position, +1 = step back.
// hand: "main" (default) or "off" to use the off-hand weapon as a bonus action.
// thrown: true to throw a melee weapon with the "thrown" tag as a ranged attack.
// swagger:model CombatActionRequest
type CombatActionRequest struct {
	Npub       string `json:"npub"        example:"npub1..."`
	SaveID     string `json:"save_id"     example:"save_1234567890"`
	WeaponSlot string `json:"weapon_slot" example:"mainHand"`
	MoveDir    int    `json:"move_dir"    example:"0"`
	Hand       string `json:"hand"        example:"main"`
	Thrown     bool   `json:"thrown"      example:"false"`
}

// CombatBaseRequest is reused by death-save and end-combat.
// swagger:model CombatBaseRequest
type CombatBaseRequest struct {
	Npub   string `json:"npub"    example:"npub1..."`
	SaveID string `json:"save_id" example:"save_1234567890"`
}

// CombatPlayerView is the player's combat state returned in responses.
// swagger:model CombatPlayerView
type CombatPlayerView struct {
	CurrentHP          int  `json:"current_hp"           example:"8"`
	MaxHP              int  `json:"max_hp"               example:"10"`
	DeathSaveSuccesses int  `json:"death_save_successes" example:"0"`
	DeathSaveFailures  int  `json:"death_save_failures"  example:"0"`
	IsUnconscious      bool `json:"is_unconscious"       example:"false"`
	IsStable           bool `json:"is_stable"            example:"false"`
}

// CombatMonsterView is the visible monster state returned in responses.
// Full stat blocks are never sent to the client.
// swagger:model CombatMonsterView
type CombatMonsterView struct {
	InstanceID string `json:"instance_id" example:"goblin"`
	Name       string `json:"name"        example:"Goblin"`
	CurrentHP  int    `json:"current_hp"  example:"5"`
	MaxHP      int    `json:"max_hp"      example:"7"`
	ArmorClass int    `json:"armor_class" example:"15"`
	IsAlive    bool   `json:"is_alive"    example:"true"`
}

// CombatStateResponse is returned by all combat action endpoints.
// swagger:model CombatStateResponse
type CombatStateResponse struct {
	Success              bool                    `json:"success"                   example:"true"`
	Phase                string                  `json:"phase"                     example:"active"`
	Round                int                     `json:"round"                     example:"1"`
	Range                int                     `json:"range"                     example:"2"`
	Player               CombatPlayerView        `json:"player"`
	Monsters             []CombatMonsterView     `json:"monsters"`
	Initiative           []types.InitiativeEntry `json:"initiative"`
	Log                  []string                `json:"log"`
	NewLog               []string                `json:"new_log,omitempty"`
	XPEarned             int                     `json:"xp_earned"                example:"12"`
	LootRolled           []types.LootDrop        `json:"loot_rolled,omitempty"`
	LevelUpPending       bool                    `json:"level_up_pending"          example:"false"`
	BonusAttackAvailable bool                    `json:"bonus_attack_available"    example:"false"`
	AmmoRemaining        int                     `json:"ammo_remaining"            example:"19"`
}

// CombatEndResponse is returned when the player calls POST /combat/end.
// swagger:model CombatEndResponse
type CombatEndResponse struct {
	Success     bool             `json:"success"               example:"true"`
	Outcome     string           `json:"outcome"               example:"victory"`
	XPApplied   int              `json:"xp_applied"            example:"47"`
	LootAdded   []types.LootDrop `json:"loot_added,omitempty"`
	LootDropped []types.LootDrop `json:"loot_dropped,omitempty"`
	Message     string           `json:"message"               example:"You defeated the Goblin and gained 47 XP."`
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// getSessionAndCombat retrieves the session and validates an active combat exists.
func getSessionAndCombat(npub, saveID string) (*session.GameSession, error) {
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		return nil, fmt.Errorf("session not found")
	}
	if sess.ActiveCombat == nil {
		return nil, fmt.Errorf("no active combat")
	}
	return sess, nil
}

// loadAdvancement retrieves the advancement table from the database.
func loadAdvancement() ([]types.AdvancementEntry, error) {
	return character.LoadAdvancement(serverdb.GetDB())
}

// buildStateResponse converts in-memory CombatSession into the API response shape.
func buildStateResponse(cs *types.CombatSession, save *types.SaveFile, newLog []string) CombatStateResponse {
	player := CombatPlayerView{}
	if len(cs.Party) > 0 {
		state := cs.Party[0].CombatState
		player = CombatPlayerView{
			CurrentHP:          state.CurrentHP,
			MaxHP:              state.MaxHP,
			DeathSaveSuccesses: state.DeathSaveSuccesses,
			DeathSaveFailures:  state.DeathSaveFailures,
			IsUnconscious:      state.IsUnconscious,
			IsStable:           state.IsStable,
		}
	}

	monsters := make([]CombatMonsterView, 0, len(cs.Monsters))
	for _, m := range cs.Monsters {
		monsters = append(monsters, CombatMonsterView{
			InstanceID: m.InstanceID,
			Name:       m.Name,
			CurrentHP:  m.CurrentHP,
			MaxHP:      m.MaxHP,
			ArmorClass: m.ArmorClass,
			IsAlive:    m.IsAlive,
		})
	}

	bonusAvail := false
	ammoLeft := 0
	if save != nil {
		bonusAvail = checkBonusAttackAvailable(cs, save)
		ammoLeft = getAmmoRemaining(save.Inventory)
	}

	return CombatStateResponse{
		Success:              true,
		Phase:                cs.Phase,
		Round:                cs.Round,
		Range:                cs.Range,
		Player:               player,
		Monsters:             monsters,
		Initiative:           cs.Initiative,
		Log:                  cs.Log,
		NewLog:               newLog,
		XPEarned:             cs.XPEarnedThisFight,
		LootRolled:           cs.LootRolled,
		LevelUpPending:       cs.LevelUpPending,
		BonusAttackAvailable: bonusAvail,
		AmmoRemaining:        ammoLeft,
	}
}

// checkBonusAttackAvailable returns true when the player currently meets the
// conditions for a two-weapon fighting bonus action: both hands hold light
// weapons, and the bonus action has not yet been used this turn.
func checkBonusAttackAvailable(cs *types.CombatSession, save *types.SaveFile) bool {
	if len(cs.Party) == 0 || cs.Party[0].CombatState.BonusActionUsed {
		return false
	}
	db := serverdb.GetDB()
	if db == nil {
		return false
	}

	mainHandID := getEquippedIDMulti(save.Inventory, "mainhand", "mainHand")
	offHandID := getEquippedIDMulti(save.Inventory, "offhand", "offHand")
	if mainHandID == "" || offHandID == "" {
		return false
	}

	mainItem, err := gamedata.LoadItemByID(db, mainHandID)
	if err != nil {
		return false
	}
	offItem, err := gamedata.LoadItemByID(db, offHandID)
	if err != nil {
		return false
	}

	return itemHasTag(mainItem["tags"], "light") &&
		itemHasTag(offItem["tags"], "light") &&
		!itemHasTag(mainItem["tags"], "loading")
}

// getAmmoRemaining reads the current quantity in the ammo gear slot.
func getAmmoRemaining(inventory map[string]interface{}) int {
	gearSlots, ok := inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return 0
	}
	for _, key := range []string{"ammunition", "ammo"} {
		if slotData, exists := gearSlots[key]; exists && slotData != nil {
			if slotMap, ok := slotData.(map[string]interface{}); ok {
				return int(slotFloat(slotMap, "quantity"))
			}
		}
	}
	return 0
}

// addAmmoToSlot restores recovered ammunition to the ammo gear slot after victory.
// If the slot is empty (ammo type unknown), no ammo is restored.
func addAmmoToSlot(save *types.SaveFile, qty int) {
	if qty <= 0 {
		return
	}
	gearSlots, ok := save.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return
	}

	var ammoMap map[string]interface{}
	var ammoKey string
	for _, key := range []string{"ammunition", "ammo"} {
		if slotData, exists := gearSlots[key]; exists && slotData != nil {
			if slotMap, ok := slotData.(map[string]interface{}); ok {
				if itemID, _ := slotMap["item"].(string); itemID != "" {
					ammoMap = slotMap
					ammoKey = key
					break
				}
			}
		}
	}

	if ammoMap == nil {
		return // Cannot restore ammo when slot has no item type
	}

	itemID, _ := ammoMap["item"].(string)
	existing := int(slotFloat(ammoMap, "quantity"))
	newQty := existing + qty

	// Cap at the item's stack limit
	item, err := gamedata.LoadItemByID(serverdb.GetDB(), itemID)
	if err == nil {
		if stackLimit, ok := item["stack"].(float64); ok && stackLimit > 0 {
			if newQty > int(stackLimit) {
				newQty = int(stackLimit)
			}
		}
	}

	ammoMap["quantity"] = newQty
	gearSlots[ammoKey] = ammoMap
}

// itemHasTag checks whether a raw tag list ([]interface{}) contains a tag string.
func itemHasTag(tags interface{}, tag string) bool {
	list, ok := tags.([]interface{})
	if !ok {
		return false
	}
	for _, t := range list {
		if s, ok := t.(string); ok && strings.EqualFold(s, tag) {
			return true
		}
	}
	return false
}

// getEquippedIDMulti tries multiple slot key variants and returns the first non-empty item ID.
func getEquippedIDMulti(inventory map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if id := gaminventory.GetEquippedItemID(inventory, key); id != "" {
			return id
		}
	}
	return ""
}

// decodeBaseRequest decodes npub + save_id from the request body.
func decodeBaseRequest(r *http.Request) (npub, saveID string, err error) {
	var req CombatBaseRequest
	if err = json.NewDecoder(r.Body).Decode(&req); err != nil {
		return "", "", fmt.Errorf("invalid request body")
	}
	if req.Npub == "" || req.SaveID == "" {
		return "", "", fmt.Errorf("missing npub or save_id")
	}
	return req.Npub, req.SaveID, nil
}

// writeJSON writes a JSON response with the given status code.
func writeCombatJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeCombatError writes a JSON error response.
func writeCombatError(w http.ResponseWriter, status int, msg string) {
	writeCombatJSON(w, status, map[string]any{"success": false, "error": msg})
}

// ─── StartCombatHandler ───────────────────────────────────────────────────────

// StartCombatHandler godoc
// @Summary      Start a combat encounter
// @Description  Initialises a new combat session for the given monster and environment.
//
//	Combat state lives in server memory only and is never written to the save file.
//	Returns the full initial combat state including initiative order and any opening
//	monster turn (if the monster wins initiative).
//
// @Tags         Combat
// @Accept       json
// @Produce      json
// @Param        request  body      CombatStartRequest  true  "Encounter parameters"
// @Success      200      {object}  CombatStateResponse       "Combat started"
// @Failure      400      {string}  string                    "Invalid request or already in combat"
// @Failure      404      {string}  string                    "Session not found"
// @Failure      405      {string}  string                    "Method not allowed"
// @Failure      500      {string}  string                    "Internal error"
// @Router       /combat/start [post]
func StartCombatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req CombatStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.MonsterID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub, save_id, or monster_id")
		return
	}

	sess, err := session.GetSessionManager().GetSession(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, "Session not found")
		return
	}
	if sess.ActiveCombat != nil {
		writeCombatError(w, http.StatusBadRequest, "Combat already in progress")
		return
	}

	advancement, err := loadAdvancement()
	if err != nil {
		log.Printf("❌ StartCombat: failed to load advancement: %v", err)
		writeCombatError(w, http.StatusInternalServerError, "Failed to load advancement data")
		return
	}

	cs, err := combat.StartCombat(serverdb.GetDB(), &sess.SaveData, req.Npub, req.MonsterID, req.EnvironmentID, advancement)
	if err != nil {
		log.Printf("❌ StartCombat: %v", err)
		writeCombatError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start combat: %v", err))
		return
	}

	sess.ActiveCombat = cs
	log.Printf("⚔️  Combat started: npub=%s monster=%s env=%s", req.Npub, req.MonsterID, req.EnvironmentID)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, cs.Log))
}

// ─── GetCombatStateHandler ────────────────────────────────────────────────────

// GetCombatStateHandler godoc
// @Summary      Get current combat state
// @Description  Returns the live combat state for an active encounter.
//
//	Use this to re-sync the frontend after a page refresh or reconnect.
//
// @Tags         Combat
// @Produce      json
// @Param        npub     query     string              true  "Nostr public key"
// @Param        save_id  query     string              true  "Save ID"
// @Success      200      {object}  CombatStateResponse       "Current combat state"
// @Failure      400      {string}  string                    "Missing parameters"
// @Failure      404      {string}  string                    "Session or combat not found"
// @Failure      405      {string}  string                    "Method not allowed"
// @Router       /combat/state [get]
func GetCombatStateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}

	sess, err := getSessionAndCombat(npub, saveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCombatJSON(w, http.StatusOK, buildStateResponse(sess.ActiveCombat, &sess.SaveData, nil))
}

// ─── CombatActionHandler ──────────────────────────────────────────────────────

// CombatActionHandler godoc
// @Summary      Execute a player attack action
// @Description  Resolves one full combat round: player movement, attack roll, damage,
//
//	XP award, and the monster's response turn. Returns the updated combat
//	state along with the new log entries for this round.
//	weapon_slot must be one of: "mainHand", "offHand", "unarmed".
//	move_dir: -1 step closer, 0 hold position, +1 step back.
//
// @Tags         Combat
// @Accept       json
// @Produce      json
// @Param        request  body      CombatActionRequest  true  "Attack action"
// @Success      200      {object}  CombatStateResponse        "Round resolved"
// @Failure      400      {string}  string                     "Invalid request or bad phase"
// @Failure      404      {string}  string                     "Session or combat not found"
// @Failure      405      {string}  string                     "Method not allowed"
// @Failure      500      {string}  string                     "Internal error"
// @Router       /combat/action [post]
func CombatActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req CombatActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}
	if req.WeaponSlot == "" {
		req.WeaponSlot = "mainHand" // Default to mainhand
	}
	if req.Hand == "" {
		req.Hand = "main" // Default to main-hand action
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	if cs.Phase != "active" {
		writeCombatError(w, http.StatusBadRequest,
			fmt.Sprintf("Cannot attack: combat phase is %q (must be \"active\")", cs.Phase))
		return
	}

	advancement, err := loadAdvancement()
	if err != nil {
		log.Printf("❌ CombatAction: failed to load advancement: %v", err)
		writeCombatError(w, http.StatusInternalServerError, "Failed to load advancement data")
		return
	}

	roundLog, err := combat.ProcessPlayerAttack(
		serverdb.GetDB(), cs, &sess.SaveData,
		req.WeaponSlot, req.MoveDir, req.Hand, req.Thrown, advancement,
	)
	if err != nil {
		log.Printf("❌ CombatAction: %v", err)
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Combat error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	cs.Round++

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// ─── CombatDeathSaveHandler ───────────────────────────────────────────────────

// CombatDeathSaveHandler godoc
// @Summary      Roll a death saving throw
// @Description  Rolls one death saving throw for the unconscious player and runs the
//
//	monster's response turn. Must be called when combat phase is "death_saves".
//	Natural 20 revives with 1 HP; natural 1 counts as two failures; 3 successes
//	stabilise; 3 failures end in defeat.
//
// @Tags         Combat
// @Accept       json
// @Produce      json
// @Param        request  body      CombatBaseRequest    true  "Session identifiers"
// @Success      200      {object}  CombatStateResponse        "Throw resolved"
// @Failure      400      {string}  string                     "Wrong phase or bad request"
// @Failure      404      {string}  string                     "Session or combat not found"
// @Failure      405      {string}  string                     "Method not allowed"
// @Router       /combat/death-save [post]
func CombatDeathSaveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	npub, saveID, err := decodeBaseRequest(r)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, err := getSessionAndCombat(npub, saveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	if cs.Phase != "death_saves" {
		writeCombatError(w, http.StatusBadRequest,
			fmt.Sprintf("Cannot roll death save: combat phase is %q", cs.Phase))
		return
	}

	roundLog := combat.ProcessDeathSave(cs, &sess.SaveData)
	cs.Log = append(cs.Log, roundLog...)
	cs.Round++

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// ─── CombatEndHandler ─────────────────────────────────────────────────────────

// CombatEndHandler godoc
// @Summary      End combat and apply results
// @Description  Resolves the outcome of the combat encounter and applies changes to the
//
//	player's save data in session memory. Must be called after combat reaches
//	a terminal phase ("loot", "victory", or "defeat").
//
//	Victory: Applies earned XP to session, adds loot to inventory, and updates
//	the player's HP to reflect damage taken during combat. If level_up_pending
//	is true the frontend should show the level-up screen before letting the
//	player save.
//
//	Defeat: Flattens the player's inventory into individual units sorted by
//	item cost, keeps the 3 most valuable, clears everything else, restores HP
//	and mana to full, and returns the player to their starting location. XP and
//	level are preserved. The frontend should show the death screen.
//
//	In both cases the active combat is cleared from session memory when this
//	endpoint returns successfully.
//
// @Tags         Combat
// @Accept       json
// @Produce      json
// @Param        request  body      CombatBaseRequest  true  "Session identifiers"
// @Success      200      {object}  CombatEndResponse        "Results applied"
// @Failure      400      {string}  string                   "Wrong phase or bad request"
// @Failure      404      {string}  string                   "Session or combat not found"
// @Failure      405      {string}  string                   "Method not allowed"
// @Router       /combat/end [post]
func CombatEndHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	npub, saveID, err := decodeBaseRequest(r)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, err.Error())
		return
	}

	sess, err := getSessionAndCombat(npub, saveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	terminalPhases := map[string]bool{"loot": true, "victory": true, "defeat": true}
	if !terminalPhases[cs.Phase] {
		writeCombatError(w, http.StatusBadRequest,
			fmt.Sprintf("Cannot end combat: phase is %q — combat must reach a terminal phase first", cs.Phase))
		return
	}

	var resp CombatEndResponse

	if cs.Phase == "defeat" {
		resp = applyDefeatOutcome(sess, cs)
	} else {
		resp = applyVictoryOutcome(sess, cs)
	}

	sess.ActiveCombat = nil
	log.Printf("✅ Combat ended: npub=%s outcome=%s xp=%d", npub, resp.Outcome, resp.XPApplied)

	writeCombatJSON(w, http.StatusOK, resp)
}

// applyVictoryOutcome applies XP + loot to the session and returns the response.
func applyVictoryOutcome(sess *session.GameSession, cs *types.CombatSession) CombatEndResponse {
	save := &sess.SaveData

	// Apply combat HP (player may have taken damage)
	if len(cs.Party) > 0 {
		save.HP = cs.Party[0].CombatState.CurrentHP
		if save.HP < 1 {
			save.HP = 1 // Stable players survive with at least 1 HP
		}
	}

	// Apply XP
	save.Experience += cs.XPEarnedThisFight

	// Recover 50% of ammo used this combat (floor)
	if cs.AmmoUsedThisCombat > 0 {
		recovered := cs.AmmoUsedThisCombat / 2
		if recovered > 0 {
			addAmmoToSlot(save, recovered)
		}
	}

	// Add loot to inventory
	placed, overflow := addLootToInventory(save.Inventory, cs.LootRolled)

	msg := fmt.Sprintf("You are victorious! +%d XP.", cs.XPEarnedThisFight)
	if len(placed) > 0 {
		msg += fmt.Sprintf(" %d item type(s) added to inventory.", len(placed))
	}
	if len(overflow) > 0 {
		msg += fmt.Sprintf(" %d item type(s) had no space and were lost.", len(overflow))
	}

	return CombatEndResponse{
		Success:     true,
		Outcome:     "victory",
		XPApplied:   cs.XPEarnedThisFight,
		LootAdded:   placed,
		LootDropped: overflow,
		Message:     msg,
	}
}

// applyDefeatOutcome strips inventory, restores vitals, and returns the player
// to their starting location.
func applyDefeatOutcome(sess *session.GameSession, cs *types.CombatSession) CombatEndResponse {
	save := &sess.SaveData

	// Apply XP earned before death (plan: XP and level are kept on death)
	save.Experience += cs.XPEarnedThisFight

	// Strip inventory — keep top 3 most valuable individual items
	kept := stripInventoryForDeath(save.Inventory)

	// Restore HP and mana to full
	save.HP = save.MaxHP
	save.Mana = save.MaxMana

	// Return to starting location
	save.Location = deathReturnLocation(save)
	save.District = ""
	save.Building = ""
	save.TravelProgress = 0
	save.TravelStopped = false

	msg := fmt.Sprintf(
		"You have fallen. You wake in %s, stripped of your belongings. XP and level are preserved.",
		save.Location,
	)

	return CombatEndResponse{
		Success:   true,
		Outcome:   "defeat",
		XPApplied: cs.XPEarnedThisFight,
		LootAdded: kept,
		Message:   msg,
	}
}

// deathReturnLocation returns the player's racial starting city.
// Falls back to the first discovered location, then to "kingdom".
func deathReturnLocation(save *types.SaveFile) string {
	if len(save.LocationsDiscovered) > 0 {
		return save.LocationsDiscovered[0]
	}
	return "kingdom"
}

// ─── Inventory helpers ────────────────────────────────────────────────────────

// addLootToInventory tries to place loot into general_slots (stacking where possible).
// Returns (placed, overflow) — overflow items had no space and were lost.
func addLootToInventory(inventory map[string]interface{}, loot []types.LootDrop) (placed, overflow []types.LootDrop) {
	generalSlots, ok := inventory["general_slots"].([]interface{})
	if !ok {
		return nil, loot
	}

	for _, drop := range loot {
		remaining := drop.Quantity

		// Stack onto matching slot
		for i, slot := range generalSlots {
			if slot == nil {
				continue
			}
			slotMap, ok := slot.(map[string]interface{})
			if !ok {
				continue
			}
			if slotMap["item"] == drop.Item {
				existing := int(slotFloat(slotMap, "quantity"))
				slotMap["quantity"] = existing + remaining
				generalSlots[i] = slotMap
				remaining = 0
				break
			}
		}

		// Place in an empty slot
		if remaining > 0 {
			for i, slot := range generalSlots {
				if slot == nil {
					generalSlots[i] = map[string]interface{}{
						"item":     drop.Item,
						"quantity": remaining,
					}
					remaining = 0
					break
				}
			}
		}

		if remaining > 0 {
			overflow = append(overflow, types.LootDrop{Item: drop.Item, Quantity: remaining})
		} else {
			placed = append(placed, drop)
		}
	}

	inventory["general_slots"] = generalSlots
	return placed, overflow
}

// stripInventoryForDeath flattens all inventory into individual units, keeps the
// 3 most valuable (by item cost), clears everything else, and returns the kept items.
func stripInventoryForDeath(inventory map[string]interface{}) []types.LootDrop {
	units := collectItemUnits(inventory)

	sort.Slice(units, func(i, j int) bool {
		return units[i].cost > units[j].cost
	})

	top3 := mergeUnitsIntoDrops(units, 3)
	clearEntireInventory(inventory)
	placeItemsInGeneralSlots(inventory, top3)

	return top3
}

// itemUnit is an individual item instance with its looked-up cost.
type itemUnit struct {
	itemID string
	cost   float64
}

// collectItemUnits expands all inventory slots into individual item units.
// Stacks of N become N units. Item costs are looked up from the database.
func collectItemUnits(inventory map[string]interface{}) []itemUnit {
	var units []itemUnit

	// General slots
	if slots, ok := inventory["general_slots"].([]interface{}); ok {
		for _, slot := range slots {
			units = append(units, unitsFromSlot(slot)...)
		}
	}

	gearSlots, ok := inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return units
	}

	// Equipped gear (excluding bag)
	for slotName, val := range gearSlots {
		if slotName == "bag" {
			continue
		}
		units = append(units, unitsFromSlot(val)...)
	}

	// Bag contents
	if bag, ok := gearSlots["bag"].(map[string]interface{}); ok {
		if contents, ok := bag["contents"].([]interface{}); ok {
			for _, slot := range contents {
				units = append(units, unitsFromSlot(slot)...)
			}
		}
	}

	return units
}

// unitsFromSlot extracts item units from a raw slot value (map or nil).
func unitsFromSlot(slot interface{}) []itemUnit {
	if slot == nil {
		return nil
	}
	slotMap, ok := slot.(map[string]interface{})
	if !ok {
		return nil
	}
	itemID, _ := slotMap["item"].(string)
	if itemID == "" {
		return nil
	}
	qty := int(slotFloat(slotMap, "quantity"))
	if qty <= 0 {
		qty = 1 // Equipped gear has no quantity field — count as 1
	}

	cost := lookupItemCost(itemID)
	units := make([]itemUnit, qty)
	for i := range units {
		units[i] = itemUnit{itemID: itemID, cost: cost}
	}
	return units
}

// lookupItemCost queries the item's cost field from the database.
func lookupItemCost(itemID string) float64 {
	item, err := gamedata.LoadItemByID(serverdb.GetDB(), itemID)
	if err != nil {
		return 0
	}
	if cost, ok := item["cost"].(float64); ok {
		return cost
	}
	return 0
}

// mergeUnitsIntoDrops takes the first N units and merges identical items into stacks.
func mergeUnitsIntoDrops(units []itemUnit, maxUnits int) []types.LootDrop {
	if len(units) > maxUnits {
		units = units[:maxUnits]
	}
	var drops []types.LootDrop
	for _, u := range units {
		found := false
		for i := range drops {
			if drops[i].Item == u.itemID {
				drops[i].Quantity++
				found = true
				break
			}
		}
		if !found {
			drops = append(drops, types.LootDrop{Item: u.itemID, Quantity: 1})
		}
	}
	return drops
}

// clearEntireInventory sets all inventory slots to nil/empty.
func clearEntireInventory(inventory map[string]interface{}) {
	if slots, ok := inventory["general_slots"].([]interface{}); ok {
		for i := range slots {
			slots[i] = nil
		}
		inventory["general_slots"] = slots
	}

	if gearSlots, ok := inventory["gear_slots"].(map[string]interface{}); ok {
		for slotName, val := range gearSlots {
			if slotName == "bag" {
				if bag, ok := val.(map[string]interface{}); ok {
					if contents, ok := bag["contents"].([]interface{}); ok {
						for i := range contents {
							contents[i] = nil
						}
						bag["contents"] = contents
					}
					gearSlots["bag"] = bag
				}
			} else {
				gearSlots[slotName] = nil
			}
		}
		inventory["gear_slots"] = gearSlots
	}
}

// placeItemsInGeneralSlots writes the kept items into the first N general slots.
func placeItemsInGeneralSlots(inventory map[string]interface{}, drops []types.LootDrop) {
	slots, ok := inventory["general_slots"].([]interface{})
	if !ok {
		return
	}
	for i, drop := range drops {
		if i >= len(slots) {
			break
		}
		slots[i] = map[string]interface{}{
			"item":     drop.Item,
			"quantity": drop.Quantity,
		}
	}
	inventory["general_slots"] = slots
}

// slotFloat safely reads a float64 value from a slot map.
func slotFloat(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

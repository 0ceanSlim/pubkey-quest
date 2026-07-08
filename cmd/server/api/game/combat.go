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
	"pubkey-quest/cmd/server/game/poi"
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

// CombatMoveRequest is the body sent to POST /combat/move.
// x, y: target grid cell coordinates (0-indexed, must be within grid bounds).
// swagger:model CombatMoveRequest
type CombatMoveRequest struct {
	Npub   string `json:"npub"    example:"npub1..."`
	SaveID string `json:"save_id" example:"save_1234567890"`
	X      int    `json:"x"       example:"3"`
	Y      int    `json:"y"       example:"3"`
}

// CombatActionRequest is the body sent to POST /combat/action.
// weapon_slot must be "mainHand", "offHand", or "unarmed".
// hand: "main" (default) or "off" to use the off-hand weapon as a bonus action.
// thrown: true to throw a melee weapon with the "thrown" tag as a ranged attack.
// swagger:model CombatActionRequest
type CombatActionRequest struct {
	Npub       string `json:"npub"        example:"npub1..."`
	SaveID     string `json:"save_id"     example:"save_1234567890"`
	WeaponSlot string `json:"weapon_slot" example:"mainHand"`
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
	Dodging            bool `json:"dodging"              example:"false"`
	Conditions         []string `json:"conditions"`
	// Resource is the martial class ability pool (Rage/Stamina/Ki/Cunning); nil for casters.
	Resource      *CombatResourceView `json:"resource,omitempty"`
	RageTurnsLeft int                 `json:"rage_turns_left,omitempty"`
	AbilitiesUsed []string            `json:"abilities_used,omitempty"` // once-per-combat abilities already spent
}

// CombatResourceView is the visible martial ability resource pool.
// swagger:model CombatResourceView
type CombatResourceView struct {
	Type    string `json:"type"    example:"rage"`
	Label   string `json:"label"   example:"Rage"`
	Current int    `json:"current" example:"50"`
	Max     int    `json:"max"     example:"100"`
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
	Conditions []string `json:"conditions"`
}

// CombatGridView describes the 2D combat grid dimensions.
// swagger:model CombatGridView
type CombatGridView struct {
	Width  int `json:"width"  example:"9"`
	Height int `json:"height" example:"7"`
}

// CombatStateResponse is returned by all combat action endpoints.
// swagger:model CombatStateResponse
type CombatStateResponse struct {
	Success              bool                    `json:"success"                example:"true"`
	Phase                string                  `json:"phase"                  example:"active"`
	Round                int                     `json:"round"                  example:"1"`
	Range                int                     `json:"range"                  example:"2"`
	Grid                 CombatGridView          `json:"grid"`
	PlayerPos            types.Position          `json:"player_pos"`
	MonsterPos           types.Position          `json:"monster_pos"`
	MovementBudget       int                     `json:"movement_budget"        example:"6"`
	MovementSpent        int                     `json:"movement_spent"         example:"0"`
	ActionUsed           bool                    `json:"action_used"            example:"false"`
	BonusActionUsed      bool                    `json:"bonus_action_used"      example:"false"`
	Disengaged           bool                    `json:"disengaged"             example:"false"`
	ReactionUsed         bool                    `json:"reaction_used"          example:"false"`
	MonsterMeleeReach    int                     `json:"monster_melee_reach"    example:"1"`
	PlayerMeleeReach     int                     `json:"player_melee_reach"     example:"1"`
	MonsterPosBefore     *types.Position         `json:"monster_pos_before,omitempty"`
	Player               CombatPlayerView        `json:"player"`
	Monsters             []CombatMonsterView     `json:"monsters"`
	Initiative           []types.InitiativeEntry `json:"initiative"`
	Log                  []string                `json:"log"`
	NewLog               []string                `json:"new_log,omitempty"`
	XPEarned             int                     `json:"xp_earned"              example:"12"`
	LootRolled           []types.LootDrop        `json:"loot_rolled,omitempty"`
	LevelUpPending       bool                    `json:"level_up_pending"       example:"false"`
	BonusAttackAvailable bool                    `json:"bonus_attack_available" example:"false"`
	AmmoRemaining        int                     `json:"ammo_remaining"         example:"19"`
	Difficulty           string                  `json:"difficulty,omitempty"   example:"tough"`
}

// CombatEndResponse is returned when the player calls POST /combat/end.
// swagger:model CombatEndResponse
type CombatEndResponse struct {
	Success     bool                 `json:"success"               example:"true"`
	Outcome     string               `json:"outcome"               example:"victory"`
	XPApplied   int                  `json:"xp_applied"            example:"47"`
	LootAdded   []types.LootDrop     `json:"loot_added,omitempty"`
	LootDropped []types.LootDrop     `json:"loot_dropped,omitempty"`
	Message     string               `json:"message"               example:"You defeated the Goblin and gained 47 XP."`
	LevelUp     *types.LevelUpResult `json:"level_up,omitempty"`
	// POIResumed is the next POI node when this fight happened inside a POI walk
	// (a monster node). The client reopens the exploration overlay on it. Nil for
	// ordinary fights and on defeat.
	POIResumed *poi.StepResult `json:"poi_resumed,omitempty"`
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
// conditionNames flattens combat conditions to their display names for the client.
func conditionNames(conds []types.CombatCondition) []string {
	out := make([]string, 0, len(conds))
	for _, c := range conds {
		out = append(out, c.Name)
	}
	return out
}

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
			Dodging:            state.Dodging,
			Conditions:         conditionNames(state.Conditions),
			RageTurnsLeft:      state.RageTurnsLeft,
			AbilitiesUsed:      state.AbilitiesUsed,
		}
		if state.Resource != nil {
			player.Resource = &CombatResourceView{
				Type:    state.Resource.Type,
				Label:   state.Resource.Label,
				Current: state.Resource.Current,
				Max:     state.Resource.Max,
			}
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
			Conditions: conditionNames(m.Conditions),
		})
	}

	bonusAvail := false
	ammoLeft := 0
	if save != nil {
		bonusAvail = checkBonusAttackAvailable(cs, save)
		ammoLeft = getAmmoRemaining(save.Inventory)
	}

	movBudget, movSpent, actionUsed, bonusUsed, disengaged, reactionUsed := 0, 0, false, false, false, false
	if len(cs.Party) > 0 {
		s := cs.Party[0].CombatState
		movBudget = s.MovementBudget
		movSpent = s.MovementSpent
		actionUsed = s.ActionUsed
		bonusUsed = s.BonusActionUsed
		disengaged = s.Disengaged
		reactionUsed = s.ReactionUsed
	}

	monsterReach := 0
	if len(cs.Monsters) > 0 {
		monsterReach = combat.MonsterMeleeReach(&cs.Monsters[0])
	}
	playerReach := 0
	if save != nil {
		playerReach = combat.PlayerMeleeReachForSave(serverdb.GetDB(), save)
	}

	return CombatStateResponse{
		Success:              true,
		Phase:                cs.Phase,
		Round:                cs.Round,
		Range:                combat.ChebyshevExported(cs.PlayerPos, cs.MonsterPos),
		Grid:                 CombatGridView{Width: cs.GridWidth, Height: cs.GridHeight},
		PlayerPos:            cs.PlayerPos,
		MonsterPos:           cs.MonsterPos,
		MovementBudget:       movBudget,
		MovementSpent:        movSpent,
		ActionUsed:           actionUsed,
		BonusActionUsed:      bonusUsed,
		Disengaged:           disengaged,
		ReactionUsed:         reactionUsed,
		MonsterMeleeReach:    monsterReach,
		PlayerMeleeReach:     playerReach,
		MonsterPosBefore:     cs.MonsterSpawnPos,
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
		Difficulty:           cs.Difficulty,
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

	resp := buildStateResponse(cs, &sess.SaveData, cs.Log)
	// Spawn position is only meaningful on the very first response — clear it
	// so subsequent state queries / rounds don't re-trigger the opening animation.
	cs.MonsterSpawnPos = nil
	writeCombatJSON(w, http.StatusOK, resp)
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
		req.WeaponSlot = "mainHand"
	}
	if req.Hand == "" {
		req.Hand = "main"
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	advancement, err := loadAdvancement()
	if err != nil {
		log.Printf("❌ CombatAction: failed to load advancement: %v", err)
		writeCombatError(w, http.StatusInternalServerError, "Failed to load advancement data")
		return
	}

	cs := sess.ActiveCombat
	roundLog, err := combat.ProcessPlayerAttack(
		serverdb.GetDB(), cs, &sess.SaveData,
		req.WeaponSlot, req.Hand, req.Thrown, advancement,
	)
	if err != nil {
		log.Printf("❌ CombatAction: %v", err)
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Combat error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	cs.Round++
	roundLog = append(roundLog, maybeAutoEndTurn(cs, &sess.SaveData)...)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// ─── CombatCastHandler (M4 Phase B) ───────────────────────────────────────────

// CombatCastRequest is the body sent to POST /combat/cast.
// swagger:model CombatCastRequest
type CombatCastRequest struct {
	Npub    string `json:"npub"     example:"npub1..."`
	SaveID  string `json:"save_id"  example:"save_1234567890"`
	SpellID string `json:"spell_id" example:"fire-bolt"`
}

// CombatCastHandler resolves the player casting a prepared spell at the monster.
// Like the attack handler it runs the cast, then auto-ends the turn if the player
// has nothing left to do. Casting gates on known + prepared + mana + components;
// a failure (e.g. missing component) returns 400 without spending the turn.
func CombatCastHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req CombatCastRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.SpellID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub, save_id, or spell_id")
		return
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	advancement, err := loadAdvancement()
	if err != nil {
		log.Printf("❌ CombatCast: failed to load advancement: %v", err)
		writeCombatError(w, http.StatusInternalServerError, "Failed to load advancement data")
		return
	}

	cs := sess.ActiveCombat
	roundLog, err := combat.ProcessPlayerCast(serverdb.GetDB(), cs, &sess.SaveData, req.SpellID, advancement)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Cast error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	cs.Round++
	roundLog = append(roundLog, maybeAutoEndTurn(cs, &sess.SaveData)...)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// ─── CombatUseItemHandler (M4 Phase C) ─────────────────────────────────────────

// CombatUseItemRequest is the body sent to POST /combat/use-item.
// swagger:model CombatUseItemRequest
type CombatUseItemRequest struct {
	Npub   string `json:"npub"    example:"npub1..."`
	SaveID string `json:"save_id" example:"save_1234567890"`
	ItemID string `json:"item_id" example:"healing"`
}

// CombatUseItemHandler drinks a potion / uses a consumable during the fight and
// auto-ends the turn if nothing meaningful remains. Healing lands on the combat
// HP pool (see combat.ProcessPlayerUseItem).
func CombatUseItemHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req CombatUseItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.ItemID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub, save_id, or item_id")
		return
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	roundLog, err := combat.ProcessPlayerUseItem(serverdb.GetDB(), cs, &sess.SaveData, req.ItemID)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Use-item error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	cs.Round++
	roundLog = append(roundLog, maybeAutoEndTurn(cs, &sess.SaveData)...)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// ─── CombatAbilityHandler (M5 Slice 3) ─────────────────────────────────────────

// CombatAbilityRequest is the body sent to POST /combat/ability.
// swagger:model CombatAbilityRequest
type CombatAbilityRequest struct {
	Npub      string `json:"npub"       example:"npub1..."`
	SaveID    string `json:"save_id"    example:"save_1234567890"`
	AbilityID string `json:"ability_id" example:"enter-rage"`
}

// CombatAbilityHandler resolves a martial class ability (Rage, Second Wind, Flurry,
// Sneak Attack, …), spending the class resource pool. Like the other combat
// endpoints it auto-ends the turn once the player is out of options — buffs that
// grant an extra action (Action Surge / Flurry) deliberately leave the turn open.
func CombatAbilityHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req CombatAbilityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.AbilityID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub, save_id, or ability_id")
		return
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	advancement, err := loadAdvancement()
	if err != nil {
		log.Printf("❌ CombatAbility: failed to load advancement: %v", err)
		writeCombatError(w, http.StatusInternalServerError, "Failed to load advancement data")
		return
	}

	cs := sess.ActiveCombat
	roundLog, err := combat.ProcessPlayerAbility(serverdb.GetDB(), cs, &sess.SaveData, req.AbilityID, advancement)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Ability error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	cs.Round++
	roundLog = append(roundLog, maybeAutoEndTurn(cs, &sess.SaveData)...)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// ─── CombatMoveHandler ────────────────────────────────────────────────────────

// CombatMoveHandler handles the player's movement sub-phase.
// The range updates and turn_phase transitions to "action" but the monster
// does NOT respond — it waits for the player to take an action first.
func CombatMoveHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req CombatMoveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	moveLog, err := combat.ProcessPlayerMove(serverdb.GetDB(), cs, &sess.SaveData, req.X, req.Y)
	if err != nil {
		log.Printf("❌ CombatMove: %v", err)
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Combat error: %v", err))
		return
	}

	cs.Log = append(cs.Log, moveLog...)
	moveLog = append(moveLog, maybeAutoEndTurn(cs, &sess.SaveData)...)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, moveLog))
}

// ─── CombatEndTurnHandler ────────────────────────────────────────────────────

// CombatEndTurnHandler finalises the player's turn and runs the monster's response.
// The player can move and act freely before calling this. Resets player turn state
// for the next round and increments the round counter.
func CombatEndTurnHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req CombatBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	roundLog, err := combat.ProcessEndTurn(serverdb.GetDB(), cs, &sess.SaveData)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Combat error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	cs.Round++

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// ─── CombatHoldHandler ────────────────────────────────────────────────────────

// CombatHoldHandler uses the player's action to brace into a readied stance.
// Consumes all remaining movement and sets HeldPosition=true so the readied
// counter-attack fires if the monster closes to melee reach next turn. Auto-
// ends the turn (Hold always fills both Action and Movement).
func CombatHoldHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req CombatBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	roundLog, err := combat.ProcessPlayerHold(cs)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Combat error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	roundLog = append(roundLog, maybeAutoEndTurn(cs, &sess.SaveData)...)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// CombatDisengageHandler spends the player's action to disengage — no
// opportunity attacks will be provoked by player movement this turn.
func CombatDisengageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req CombatBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	roundLog, err := combat.ProcessPlayerDisengage(cs)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Combat error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	roundLog = append(roundLog, maybeAutoEndTurn(cs, &sess.SaveData)...)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, roundLog))
}

// shouldAutoEndTurn reports whether the player has no meaningful moves left.
// True when: action used AND movement fully spent AND (bonus already used OR
// bonus attack not available). The end-turn handler callers use this to decide
// whether to run ProcessEndTurn before serialising the response.
func shouldAutoEndTurn(cs *types.CombatSession, save *types.SaveFile) bool {
	if cs.Phase != "active" || len(cs.Party) == 0 {
		return false
	}
	state := &cs.Party[0].CombatState
	if !state.ActionUsed {
		return false
	}
	if state.MovementSpent < state.MovementBudget {
		return false
	}
	if state.BonusActionUsed {
		return true
	}
	return !checkBonusAttackAvailable(cs, save)
}

// maybeAutoEndTurn runs the monster response if the player is out of meaningful
// options. Returns the generated log entries (empty slice if no auto-end fired).
// The cs.Log is updated with those entries so persisted state stays consistent.
func maybeAutoEndTurn(cs *types.CombatSession, save *types.SaveFile) []string {
	if !shouldAutoEndTurn(cs, save) {
		return nil
	}
	autoLog, err := combat.ProcessEndTurn(serverdb.GetDB(), cs, save)
	if err != nil {
		log.Printf("⚠️ auto end-turn failed: %v", err)
		return nil
	}
	cs.Log = append(cs.Log, autoLog...)
	return autoLog
}

// ─── CombatFleeHandler ────────────────────────────────────────────────────────

// CombatFleeHandler attempts a flee roll using the formula from §18 of the combat plan.
// Requires range ≥ 3. On success the combat ends (phase → "loot", empty loot).
// On failure the monster still responds and combat continues.
func CombatFleeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req CombatBaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCombatError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}

	sess, err := getSessionAndCombat(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, err.Error())
		return
	}

	cs := sess.ActiveCombat
	roundLog, err := combat.ProcessPlayerFlee(cs, &sess.SaveData)
	if err != nil {
		writeCombatError(w, http.StatusBadRequest, fmt.Sprintf("Combat error: %v", err))
		return
	}

	cs.Log = append(cs.Log, roundLog...)
	roundLog = append(roundLog, maybeAutoEndTurn(cs, &sess.SaveData)...)

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

	// If this fight happened inside a POI walk, bridge back: defeat ends the walk
	// (the player is teleported home), victory resumes it at the node past the
	// monster so the client can reopen the exploration overlay.
	if sess.ActivePOI != nil {
		if resp.Outcome == "defeat" {
			sess.ActivePOI = nil
		} else {
			resp.POIResumed = resumePOIAfterVictory(sess)
		}
	}

	// Combat mutated HP / XP / inventory (loot) outside the per-tick delta system,
	// so the session's delta snapshot is now stale (it predates the fight). Re-base
	// it to the post-combat state: the client already has the full picture from its
	// post-combat refresh, and this stops the next world tick from emitting a stale
	// inventory diff against a pre-loot baseline.
	sess.InitializeSnapshot()

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

	// Apply XP through the single grant path so any level-up is applied and
	// surfaced — XP comes from many sources, not just combat.
	var levelUp types.LevelUpResult
	if adv, err := character.LoadAdvancement(serverdb.GetDB()); err == nil {
		levelUp = character.GrantXP(save, cs.XPEarnedThisFight, adv)
	} else {
		log.Printf("⚠️ advancement load failed; XP applied without level-up check: %v", err)
		save.Experience += cs.XPEarnedThisFight
	}

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

	resp := CombatEndResponse{
		Success:     true,
		Outcome:     "victory",
		XPApplied:   cs.XPEarnedThisFight,
		LootAdded:   placed,
		LootDropped: overflow,
		Message:     msg,
	}
	if levelUp.Leveled {
		resp.LevelUp = &levelUp
		resp.Message += fmt.Sprintf(" You reached level %d!", levelUp.NewLevel)
	}
	return resp
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

// addLootToInventory places each drop into the first slot it fits, in order:
//   1. Stack onto a matching general_slots entry
//   2. Stack onto a matching gear_slots.bag.contents entry
//   3. Take an empty general_slots entry
//   4. Take an empty gear_slots.bag.contents entry
//
// Stack limits are read from the items DB (item["stack"]). If absent or 1, the
// item is treated as unstackable — extras spill into a fresh slot rather than
// piling onto the same entry. Anything that still doesn't fit ends up in overflow.
func addLootToInventory(inventory map[string]interface{}, loot []types.LootDrop) (placed, overflow []types.LootDrop) {
	generalSlots, _ := inventory["general_slots"].([]interface{})
	bagSlots := getBagContents(inventory)

	for _, drop := range loot {
		stackLimit := lookupItemStackLimit(drop.Item)
		remaining := drop.Quantity

		// 1) Stack onto matching general_slots entries
		if remaining > 0 && generalSlots != nil {
			remaining = stackOntoMatching(generalSlots, drop.Item, remaining, stackLimit)
		}
		// 2) Stack onto matching bag entries
		if remaining > 0 && bagSlots != nil {
			remaining = stackOntoMatching(bagSlots, drop.Item, remaining, stackLimit)
		}
		// 3) Empty general_slots entries
		if remaining > 0 && generalSlots != nil {
			remaining = placeInEmpty(generalSlots, drop.Item, remaining, stackLimit)
		}
		// 4) Empty bag entries
		if remaining > 0 && bagSlots != nil {
			remaining = placeInEmpty(bagSlots, drop.Item, remaining, stackLimit)
		}

		placedQty := drop.Quantity - remaining
		if placedQty > 0 {
			placed = append(placed, types.LootDrop{Item: drop.Item, Quantity: placedQty})
		}
		if remaining > 0 {
			overflow = append(overflow, types.LootDrop{Item: drop.Item, Quantity: remaining})
		}
	}

	if generalSlots != nil {
		inventory["general_slots"] = generalSlots
	}
	if bagSlots != nil {
		writeBagContents(inventory, bagSlots)
	}
	return placed, overflow
}

// getBagContents returns the backpack's contents slice, or nil if no bag is equipped.
func getBagContents(inventory map[string]interface{}) []interface{} {
	gearSlots, ok := inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return nil
	}
	bag, ok := gearSlots["bag"].(map[string]interface{})
	if !ok {
		return nil
	}
	contents, _ := bag["contents"].([]interface{})
	return contents
}

// writeBagContents writes the bag contents slice back into the inventory.
func writeBagContents(inventory map[string]interface{}, contents []interface{}) {
	gearSlots, ok := inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return
	}
	bag, ok := gearSlots["bag"].(map[string]interface{})
	if !ok {
		return
	}
	bag["contents"] = contents
	gearSlots["bag"] = bag
	inventory["gear_slots"] = gearSlots
}

// lookupItemStackLimit returns the item's stack limit; defaults to 1 (unstackable)
// when the field is missing or invalid.
func lookupItemStackLimit(itemID string) int {
	item, err := gamedata.LoadItemByID(serverdb.GetDB(), itemID)
	if err != nil {
		return 1
	}
	if v, ok := item["stack"].(float64); ok && v > 0 {
		return int(v)
	}
	return 1
}

// stackOntoMatching tops up existing stacks of itemID up to stackLimit.
// Returns the quantity that still needs to be placed.
func stackOntoMatching(slots []interface{}, itemID string, qty, stackLimit int) int {
	if stackLimit <= 1 {
		return qty // unstackable — never merge onto an existing entry
	}
	for i, slot := range slots {
		if qty == 0 {
			break
		}
		slotMap, ok := slot.(map[string]interface{})
		if !ok || slotMap == nil {
			continue
		}
		if id, _ := slotMap["item"].(string); id != itemID {
			continue
		}
		existing := int(slotFloat(slotMap, "quantity"))
		room := stackLimit - existing
		if room <= 0 {
			continue
		}
		add := qty
		if add > room {
			add = room
		}
		slotMap["quantity"] = existing + add
		slots[i] = slotMap
		qty -= add
	}
	return qty
}

// placeInEmpty drops the remaining quantity into empty slots, respecting stack
// limits. Returns the leftover quantity that didn't fit.
func placeInEmpty(slots []interface{}, itemID string, qty, stackLimit int) int {
	for i, slot := range slots {
		if qty == 0 {
			break
		}
		if slot != nil {
			if slotMap, ok := slot.(map[string]interface{}); ok {
				if id, _ := slotMap["item"].(string); id != "" {
					continue
				}
			}
		}
		take := qty
		if stackLimit > 0 && take > stackLimit {
			take = stackLimit
		}
		slots[i] = map[string]interface{}{
			"item":     itemID,
			"quantity": take,
		}
		qty -= take
	}
	return qty
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

	// Diagnostic: pin whether "only a backpack on death" is a strip bug (units
	// collected but not kept) or an already-empty inventory (loot lost earlier).
	sample := make([]string, 0, len(units))
	for i, u := range units {
		if i >= 5 {
			break
		}
		sample = append(sample, fmt.Sprintf("%s(%.0f)", u.itemID, u.cost))
	}
	log.Printf("💀 death strip: %d units collected %v → kept top %d %+v", len(units), sample, len(top3), top3)

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

	// Equipped gear, including the bag itself — a backpack is an item with a value
	// and competes for the kept slots like anything else (unitsFromSlot reads the
	// bag's own "item"/"quantity"; its contents are gathered separately below).
	for _, val := range gearSlots {
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

// lookupItemCost queries the item's base worth from the database — the canonical
// "value" field (with a legacy "price" fallback), the same fields db/sqlite.go
// reads into Item.Value. Reading the wrong key here made every item score 0, so
// the death "keep your 3 most valuable items" rule was keeping arbitrary items.
func lookupItemCost(itemID string) float64 {
	item, err := gamedata.LoadItemByID(serverdb.GetDB(), itemID)
	if err != nil {
		return 0
	}
	if value, ok := item["value"].(float64); ok {
		return value
	}
	if price, ok := item["price"].(float64); ok {
		return price
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
		// Clear every equipped slot, the bag included — it's an item like any
		// other and is only kept if it placed in the top 3 (as a general-slot
		// item). Its contents were already flattened into the valuation.
		for slotName := range gearSlots {
			gearSlots[slotName] = nil
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

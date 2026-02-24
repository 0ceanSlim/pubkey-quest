package game

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/session"
)

// debugMonsterPool is the curated list of starter monsters for debug combat.
var debugMonsterPool = []string{
	"goblin", "wolf", "skeleton", "zombie", "orc",
	"kobold", "bandit", "giant-rat", "blood-hawk",
}

// debugCombatRequest extends the base request with an optional monster selector.
type debugCombatRequest struct {
	Npub      string `json:"npub"`
	SaveID    string `json:"save_id"`
	MonsterID string `json:"monster_id"` // optional; empty = random pick
}

// DebugCombatStartHandler godoc
// @Summary      Start a random debug combat encounter
// @Description  Picks a random monster from a curated list and starts a combat session.
//
//	Only available in debug mode.
//
// @Tags         Debug
// @Accept       json
// @Produce      json
// @Param        request  body      CombatBaseRequest   true  "Session identifiers"
// @Success      200      {object}  CombatStateResponse       "Combat started"
// @Failure      400      {string}  string                    "Missing fields"
// @Failure      404      {string}  string                    "Session not found"
// @Failure      405      {string}  string                    "Method not allowed"
// @Failure      409      {string}  string                    "Combat already active"
// @Failure      500      {string}  string                    "Internal error"
// @Router       /combat/debug/start [post]
func DebugCombatStartHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req debugCombatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "missing npub or save_id")
		return
	}
	npub, saveID := req.Npub, req.SaveID

	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, "Session not found")
		return
	}

	if sess.ActiveCombat != nil {
		writeCombatError(w, http.StatusConflict, "Combat already active")
		return
	}

	// Use the caller's monster choice if it's in the allowed pool; else pick random.
	monsterID := ""
	for _, id := range debugMonsterPool {
		if id == req.MonsterID {
			monsterID = id
			break
		}
	}
	if monsterID == "" {
		monsterID = debugMonsterPool[rand.Intn(len(debugMonsterPool))]
	}

	advancement, err := loadAdvancement()
	if err != nil {
		log.Printf("❌ DebugCombatStart: failed to load advancement: %v", err)
		writeCombatError(w, http.StatusInternalServerError, "Failed to load advancement data")
		return
	}

	cs, err := combat.StartCombat(serverdb.GetDB(), &sess.SaveData, npub, monsterID, "", advancement)
	if err != nil {
		log.Printf("❌ DebugCombatStart: %v", err)
		writeCombatError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start combat: %v", err))
		return
	}

	sess.ActiveCombat = cs
	log.Printf("⚔️  Debug combat started: npub=%s monster=%s", npub, monsterID)

	writeCombatJSON(w, http.StatusOK, buildStateResponse(cs, &sess.SaveData, cs.Log))
}

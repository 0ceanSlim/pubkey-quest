package game

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// poi.go drives a discovered POI as a playable node walk. The walker engine
// (game/poi) is pure; this layer holds the in-flight walk on the session, builds
// the side-effecting Deps (where everything is importable), and bridges a
// monster node into the existing combat UI. Enter starts a walk at the POI's
// start node; advance moves to the next node the player chose; list feeds the
// travel-screen markers. State changes land directly on the live session save
// (the pattern combat uses) — the client calls /game/state afterwards to refresh.

// ─── Deps + node resolution ───────────────────────────────────────────────────

// buildPOIDeps assembles the walker's side-effecting collaborators, each routed
// to the system that owns it. This is the seam the engine deliberately leaves to
// the caller so resolution stays pure and unit-testable.
func buildPOIDeps(state *types.SaveFile) (poi.Deps, error) {
	advancement, err := loadAdvancement()
	if err != nil {
		return poi.Deps{}, err
	}
	return poi.Deps{
		Ctx:         buildQuestContext(state), // effective-stats aware (base + active effects)
		Rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		GrantReward: func(s *types.SaveFile, r *types.POIReward) { quest.GrantReward(s, r, advancement) },
		ApplyEffect: func(s *types.SaveFile, id string) { _ = effects.ApplyEffect(s, id) },
		AddItem:     func(s *types.SaveFile, id string, qty int) { _, _ = inventory.AddItemToInventory(s, id, qty) },
	}, nil
}

// stepPOI resolves one node of the active walk: it applies the node to the save,
// records a passing check so quest "check" objectives advance, updates the
// session's CurrentNode + anti-skip allowlist, bridges a monster node into
// combat (stashing the resume node), and clears the walk on a terminal node.
// When a fight starts the combat payload is written into data for the client.
func stepPOI(sess *session.GameSession, nodeID string, data map[string]any) (poi.StepResult, error) {
	if sess.ActivePOI == nil {
		return poi.StepResult{}, fmt.Errorf("no active walk")
	}
	node, ok := sess.ActivePOI.Nodes[nodeID]
	if !ok {
		return poi.StepResult{}, fmt.Errorf("node %q not found in %q", nodeID, sess.ActivePOI.POIID)
	}
	state := &sess.SaveData
	deps, err := buildPOIDeps(state)
	if err != nil {
		return poi.StepResult{}, err
	}

	res := poi.Resolve(node, nodeID, state, deps)

	// A POI/environment damage node can take HP to 0. Death is otherwise a
	// combat-only concept, so without this the walk (and travel) would keep going
	// with the player dead. Trigger the shared death flow and end the walk here —
	// the single choke point for all POI damage.
	if state.HP <= 0 {
		kept := ApplyDeath(state)
		sess.ActivePOI = nil
		res.Terminal = true
		res.Combat = ""
		res.Next = ""
		res.Outcome = append(res.Outcome, fmt.Sprintf(
			"You have fallen. You wake in %s, stripped of your belongings — but your experience endures.",
			state.Location,
		))
		data["death"] = map[string]any{"outcome": "defeat", "location": state.Location, "loot_kept": kept}
		return res, nil
	}

	// A passing check feeds the event recorder so quest "check" objectives tick.
	if res.CheckSkill != "" && res.CheckSuccess {
		events.Record(state, events.SkillCheckPassed, res.CheckSkill, 1)
	}

	sess.ActivePOI.CurrentNode = nodeID
	sess.ActivePOI.ValidNexts = poi.NextsFor(res)

	switch {
	case res.Combat != "":
		// Monster node — drop into the combat UI; the POI resumes at res.Next on
		// victory. If the monster can't be started (e.g. an unbuilt monster id),
		// don't dead-end the walk — fall through to the resume node as a Continue.
		if err := bridgePOICombat(sess, res.Combat); err != nil {
			log.Printf("⚠️ %q monster node: combat bridge failed: %v", sess.ActivePOI.POIID, err)
			res.Combat = ""
			sess.ActivePOI.ValidNexts = poi.NextsFor(res) // recompute now Combat is cleared
			break
		}
		sess.ActivePOI.ResumeNext = res.Next
		buildCombatPayload(sess, data)
	case res.Terminal:
		sess.ActivePOI = nil
	}
	return res, nil
}

// bridgePOICombat starts combat with the POI's monster, mirroring the biome
// travel-encounter entry (maybeRollTravelEncounter in actions.go).
func bridgePOICombat(sess *session.GameSession, monsterID string) error {
	if sess.ActiveCombat != nil {
		return fmt.Errorf("combat already in progress")
	}
	advancement, err := loadAdvancement()
	if err != nil {
		return err
	}
	state := &sess.SaveData
	cs, err := combat.StartCombat(serverdb.GetDB(), state, sess.Npub, monsterID, state.Location, advancement)
	if err != nil {
		return err
	}
	sess.ActiveCombat = cs
	return nil
}

// buildCombatPayload writes the opening combat state onto data so the client
// drops into the combat UI, then clears the spawn position so later state
// queries don't replay the opening animation (mirrors StartCombatHandler).
func buildCombatPayload(sess *session.GameSession, data map[string]any) {
	cs := sess.ActiveCombat
	if cs == nil {
		return
	}
	payload := buildStateResponse(cs, &sess.SaveData, cs.Log)
	cs.MonsterSpawnPos = nil
	data["combat_started"] = true
	data["combat"] = payload
}

// resumePOIAfterVictory advances a POI that was mid-fight when combat ended in
// victory. The combat handler calls this after clearing ActiveCombat; it returns
// the resumed step for the client to render (nil when there's nothing to resume).
func resumePOIAfterVictory(sess *session.GameSession) *poi.StepResult {
	if sess.ActivePOI == nil {
		return nil
	}
	resumeNode := sess.ActivePOI.ResumeNext
	sess.ActivePOI.ResumeNext = ""
	if resumeNode == "" {
		// The monster node was terminal — the POI ends on victory.
		sess.ActivePOI = nil
		return nil
	}
	// A chained monster node on resume restarts combat server-side; the client
	// re-enters it via /combat/state on seeing step.combat. data is discarded
	// because the combat-end response only carries the resumed step. Nodes live on
	// the session, so no reload is needed (works for POIs and encounters alike).
	res, err := stepPOI(sess, resumeNode, map[string]any{})
	if err != nil {
		sess.ActivePOI = nil
		return nil
	}
	return &res
}

// ─── Handlers ─────────────────────────────────────────────────────────────────

// POIEnterHandler begins a walk of a discovered POI at its start node.
// POST /api/poi/enter {npub, save_id, poi_id}
func POIEnterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writePOIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Npub   string `json:"npub"`
		SaveID string `json:"save_id"`
		POIID  string `json:"poi_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePOIError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.POIID == "" {
		writePOIError(w, http.StatusBadRequest, "missing npub, save_id, or poi_id")
		return
	}
	sess, err := session.GetSessionManager().GetSession(req.Npub, req.SaveID)
	if err != nil {
		writePOIError(w, http.StatusNotFound, "session not found")
		return
	}
	if !alreadyDiscovered(&sess.SaveData, req.POIID) {
		writePOIError(w, http.StatusBadRequest, "POI not discovered")
		return
	}
	poiData, err := serverdb.GetPOIByID(req.POIID)
	if err != nil {
		writePOIError(w, http.StatusNotFound, "POI not found")
		return
	}

	sess.ActivePOI = &poi.Session{POIID: req.POIID, Title: poiData.Name, Nodes: poiData.Nodes, CurrentNode: poiData.StartNode}
	data := map[string]any{}
	res, err := stepPOI(sess, poiData.StartNode, data)
	if err != nil {
		sess.ActivePOI = nil
		writePOIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	data["step"] = res
	writePOIJSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

// POIAdvanceHandler resolves the next node the player chose. The requested node
// must be on the current node's anti-skip allowlist.
// POST /api/poi/advance {npub, save_id, next}
func POIAdvanceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writePOIError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Npub   string `json:"npub"`
		SaveID string `json:"save_id"`
		Next   string `json:"next"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePOIError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Npub == "" || req.SaveID == "" || req.Next == "" {
		writePOIError(w, http.StatusBadRequest, "missing npub, save_id, or next")
		return
	}
	sess, err := session.GetSessionManager().GetSession(req.Npub, req.SaveID)
	if err != nil {
		writePOIError(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.ActivePOI == nil {
		writePOIError(w, http.StatusBadRequest, "no active POI")
		return
	}
	if !sess.ActivePOI.AllowsNext(req.Next) {
		writePOIError(w, http.StatusBadRequest, "invalid step")
		return
	}
	data := map[string]any{}
	res, err := stepPOI(sess, req.Next, data)
	if err != nil {
		writePOIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	data["step"] = res
	writePOIJSON(w, http.StatusOK, map[string]any{"success": true, "data": data})
}

// POIListHandler returns the POIs the player has discovered in their current
// environment, with positions, for the travel-screen markers.
// GET /api/poi/list?npub&save_id
func POIListHandler(w http.ResponseWriter, r *http.Request) {
	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		writePOIError(w, http.StatusBadRequest, "missing npub or save_id")
		return
	}
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		writePOIError(w, http.StatusNotFound, "session not found")
		return
	}
	state := &sess.SaveData
	pois, _ := serverdb.GetPOIsByEnvironment(state.Location)
	out := make([]map[string]any, 0)
	for _, p := range pois {
		if !alreadyDiscovered(state, p.ID) {
			continue
		}
		out = append(out, map[string]any{
			"id":          p.ID,
			"name":        p.Name,
			"category":    string(p.Category),
			"description": p.Description,
			"position":    p.Position,
		})
	}
	writePOIJSON(w, http.StatusOK, map[string]any{"success": true, "data": out})
}

// ─── JSON helpers ─────────────────────────────────────────────────────────────

func writePOIJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writePOIError(w http.ResponseWriter, status int, msg string) {
	writePOIJSON(w, status, map[string]any{"success": false, "error": msg})
}

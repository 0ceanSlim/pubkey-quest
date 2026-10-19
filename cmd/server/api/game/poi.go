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
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/cmd/server/game/spellbook"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/social"
	"pubkey-quest/cmd/server/world"
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
		Ctx: buildQuestContext(state), // effective-stats aware (base + active effects)
		Rng: rand.New(rand.NewSource(time.Now().UnixNano())),
		GrantReward: func(s *types.SaveFile, r *types.POIReward) {
			quest.GrantReward(s, r, advancement)
			for _, it := range r.Items {
				noteAcquired(s, it.ID)
			}
		},
		ApplyEffect: func(s *types.SaveFile, id string) { _ = effects.ApplyEffect(s, id) },
		AddItem: func(s *types.SaveFile, id string, qty int) {
			if added, _ := inventory.AddItemToInventory(s, id, qty); added > 0 {
				noteAcquired(s, id)
			}
		},
		PayCost: payPOICost,
		RandomScroll: func(f *types.ScrollFilter, rng *rand.Rand) string {
			return spellbook.RandomScroll(f, rng.Intn)
		},
		ChangeFatigue: func(s *types.SaveFile, n int) {
			s.Fatigue = min(max(s.Fatigue+n, 0), 10)
			status.HandleFatigueChange(s)
		},
	}, nil
}

// payPOICost takes a transaction node's price — gold and items — all or
// nothing: it checks everything is there before taking any of it.
func payPOICost(s *types.SaveFile, c *types.POICost) bool {
	if c.Gold > 0 && gameutil.GetGoldQuantity(s) < c.Gold {
		return false
	}
	for _, it := range c.Items {
		if gameutil.CountItem(s, it.ID) < max(it.Quantity, 1) {
			return false
		}
	}
	if c.Gold > 0 && !gameutil.DeductGold(s, c.Gold) {
		return false
	}
	for _, it := range c.Items {
		gameutil.ConsumeItem(s, it.ID, max(it.Quantity, 1))
	}
	return true
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
	// A lock node's sub-steps ("door#quick") resolve against the node itself.
	node, ok := sess.ActivePOI.Nodes[poi.BaseNode(nodeID)]
	if !ok {
		return poi.StepResult{}, fmt.Errorf("node %q not found in %q", nodeID, sess.ActivePOI.POIID)
	}
	state := &sess.SaveData
	deps, err := buildPOIDeps(state)
	if err != nil {
		return poi.StepResult{}, err
	}
	deps.Locks = poiLocks{sess}

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

	// A passing check feeds the event recorder so quest "check" objectives tick,
	// and reaching a named spot counts for "explore" objectives.
	if res.CheckSkill != "" && res.CheckSuccess {
		events.Record(state, events.SkillCheckPassed, res.CheckSkill, 1)
	}
	// A paid shortcut (the ferryman) carries a traveller further along the
	// road; arrival itself happens on the next travel tick.
	if res.TravelAhead > 0 && travel.IsInEnvironment(state) {
		state.TravelProgress = min(0.95, state.TravelProgress+res.TravelAhead)
	}
	if res.Waypoint != "" {
		// The first time a named spot is reached it's a discovery (XP); after
		// that it still counts for quest "explore" objectives, but pays nothing.
		if alreadyDiscovered(state, res.Waypoint) {
			events.Record(state, events.LocationVisited, res.Waypoint, 1)
		} else {
			state.LocationsDiscovered = append(state.LocationsDiscovered, res.Waypoint)
			events.Record(state, events.LocationDiscovered, res.Waypoint, 1)
		}
	}

	sess.ActivePOI.CurrentNode = nodeID
	sess.ActivePOI.ValidNexts = poi.NextsFor(res)

	if len(res.NPCs) > 0 {
		sess.POINPCs = map[string]bool{}
		for _, id := range res.NPCs {
			sess.POINPCs[id] = true
		}
		sess.POINPCPlace, sess.POINPCProgress = world.GroundKey(state), state.TravelProgress
	}

	switch {
	case res.Combat != "":
		// Monster node — drop into the combat UI; the POI resumes at res.Next on
		// victory. If the monster can't be started (e.g. an unbuilt monster id),
		// don't dead-end the walk — fall through to the resume node as a Continue.
		if err := bridgePOICombat(sess, &res); err != nil {
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

// bridgePOICombat starts combat with the monster node's group (its authored count,
// and an ambush if it's flagged surprise), mirroring the biome travel-encounter
// entry (maybeRollTravelEncounter in actions.go).
func bridgePOICombat(sess *session.GameSession, res *poi.StepResult) error {
	if sess.ActiveCombat != nil {
		return fmt.Errorf("combat already in progress")
	}
	advancement, err := loadAdvancement()
	if err != nil {
		return err
	}
	state := &sess.SaveData
	spec := combat.EncounterSpec{
		MonsterIDs:    combat.MonsterIDsForCount(res.Combat, res.CombatCount),
		EnvironmentID: state.Location,
		Surprise:      res.CombatSurprise,
		Arena:         res.CombatArena,
	}
	// A party leader's walk is the party's: everyone with them fights.
	if social.PartyOf(sess.Npub) != nil && !social.IsFollower(sess.Npub) {
		_, err := startFight(sess, spec, advancement, true)
		return err
	}
	cs, err := combat.StartEncounter(serverdb.GetDB(), state, sess.Npub, spec, advancement)
	if err != nil {
		return err
	}
	sess.ActiveCombat = cs
	return nil
}

// partyStepLog mirrors a party leader's POI step into the followers' logs.
func partyStepLog(leader *session.GameSession, res poi.StepResult) {
	if social.PartyOf(leader.Npub) == nil || social.IsFollower(leader.Npub) {
		return
	}
	text := res.Text
	if len(text) > 280 {
		text = text[:277] + "…"
	}
	lines := []string{}
	if text != "" {
		lines = append(lines, "📜 "+text)
	}
	for _, o := range res.Outcome {
		lines = append(lines, "   "+o)
	}
	partyLog(leader, lines...)
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
	combat.ClearSpawnPositions(cs)
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

// metInPOI reports whether the player is still with an NPC a POI walk handed
// them to — the only way to talk to someone who lives at a POI.
func metInPOI(sess *session.GameSession, npcID string) bool {
	return sess.POINPCs[npcID] &&
		sess.POINPCPlace == world.GroundKey(&sess.SaveData) &&
		sess.POINPCProgress == sess.SaveData.TravelProgress
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
	// One request at a time per session, after any moves the leader made.
	sess.Lock()
	defer sess.Unlock()
	applyPartySyncs(sess)
	if social.IsFollower(sess.Npub) {
		writePOIError(w, http.StatusForbidden, "Your party leader leads the way in.")
		return
	}
	if sess.ActiveCombat != nil || sess.ActivePOI != nil || sess.PendingEncounter != nil {
		writePOIError(w, http.StatusConflict, "You're busy.")
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
	if !poiHere(&sess.SaveData, poiData) {
		writePOIError(w, http.StatusBadRequest, "That place isn't here.")
		return
	}
	// A POI can be shut to those who don't qualify (a rope to descend, the will
	// to face what's inside); say why, in the author's words.
	if res := requirement.Evaluate(poiData.Requirements, buildQuestContext(&sess.SaveData)); !res.OK {
		msg := "You can't go in there yet."
		if d := res.Missing[0].Description; d != "" {
			msg = d
		}
		writePOIError(w, http.StatusForbidden, msg)
		return
	}

	// Going in counts as exploring it — a quest taken after the POI was first
	// found can still send you there. (Not a discovery: it was found already.)
	events.Record(&sess.SaveData, events.LocationVisited, req.POIID, 1)

	// A leader takes the party in: whoever hadn't found it has now.
	if social.PartyOf(sess.Npub) != nil {
		partyShare(sess, []string{req.POIID}, "🗺️ "+sess.SaveData.D+" leads the party into "+poiData.Name+".")
	}

	sess.ActivePOI = &poi.Session{POIID: req.POIID, Title: poiData.Name, Nodes: poiData.Nodes, CurrentNode: poiData.StartNode}
	data := map[string]any{}
	res, err := stepPOI(sess, poiData.StartNode, data)
	if err != nil {
		sess.ActivePOI = nil
		writePOIError(w, http.StatusInternalServerError, err.Error())
		return
	}
	partyStepLog(sess, res)
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
	sess.Lock()
	defer sess.Unlock()
	if sess.ActivePOI == nil {
		writePOIError(w, http.StatusBadRequest, "no active POI")
		return
	}
	sess.LockSession = nil // moving the walk on walks away from any lock being picked
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
	partyStepLog(sess, res)
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
	// On a road: the road's POIs. Outdoors in a city: the district's hidden places.
	where := state.Location
	if state.Building == "" && state.District != "" && isCityLocation(state.Location) {
		where = state.Location + "-" + state.District
	}
	pois, _ := serverdb.GetPOIsByEnvironment(where)
	env := travel.GetEnvironmentData(state.Location)
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
			// On the player's own bar: measured from where they set out.
			"position": travel.RouteFraction(env, state.District, p.Position),
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

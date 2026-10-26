package game

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// ─── Stealth openings (party-combat-plan §7) ─────────────────────────────────
//
// An encounter can start with the monsters unaware of you. The fight isn't
// started yet: the session holds a PendingEncounter and the client shows the
// choice — sneak away, sneak attack, or approach — answered through
// POST /api/combat/approach.

// UnawareMonsterView is one monster in an unaware group.
// swagger:model UnawareMonsterView
type UnawareMonsterView struct {
	TemplateID string `json:"template_id" example:"goblin"`
	Name       string `json:"name"        example:"Goblin"`
}

// UnawareView describes a pending encounter whose monsters haven't seen you.
// swagger:model UnawareView
type UnawareView struct {
	Text     string               `json:"text"     example:"A goblin hasn't noticed you."`
	Monsters []UnawareMonsterView `json:"monsters"`
	Log      []string             `json:"log,omitempty"`
}

// ApproachRequest answers an unaware encounter.
// swagger:model ApproachRequest
type ApproachRequest struct {
	Npub   string `json:"npub"    example:"npub1..."`
	SaveID string `json:"save_id" example:"save_1234567890"`
	Choice string `json:"choice"  example:"sneak_attack"` // sneak_away | sneak_attack | approach
}

// ApproachResponse is the outcome: either you slipped away (no fight), or the
// fight started and Combat carries its opening state.
// swagger:model ApproachResponse
type ApproachResponse struct {
	Success     bool                 `json:"success"      example:"true"`
	SlippedAway bool                 `json:"slipped_away" example:"false"`
	Message     string               `json:"message"      example:"You slip away unseen."`
	Log         []string             `json:"log,omitempty"`
	Combat      *CombatStateResponse `json:"combat,omitempty"`
}

// awarenessFrom returns the player's Stealth modifier (Thieving skill) and
// passive Perception from a skill context (the travel one on the road: tools and
// your light count).
func awarenessFrom(ctx requirement.Context) (stealthMod, passivePerception int) {
	return skillcheck.Modifier(ctx.SkillValue("thieving")),
		10 + skillcheck.Modifier(ctx.SkillValue("perception"))
}

// rollOpening rolls who noticed whom for an encounter met on the road. Travel
// tools count (a spyglass), and if you'd be ambushed a Survival roll can still
// catch the signs in time.
func rollOpening(save *types.SaveFile, monsterIDs []string) (combat.AwarenessRoll, error) {
	monsters, err := combat.LoadEncounterMonsters(serverdb.GetDB(), monsterIDs)
	if err != nil {
		return combat.AwarenessRoll{}, err
	}
	ctx := buildTravelContext(save)
	stealthMod, passive := awarenessFrom(ctx)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	aw := combat.RollAwareness(rng, stealthMod, passive, monsters)
	// Say what your light did (it's already in the modifiers).
	if _, _, notes := lightAtNight(save); len(notes) > 0 {
		aw.Log = append(notes, aw.Log...)
	}
	aw = combat.ReadTheSigns(rng, aw, skillcheck.Modifier(ctx.SkillValue("survival")), monsters)
	// A ward (alarm, detect evil) warns you before they're on you: no ambush.
	if aw.Opening == combat.OpeningAmbush && effects.ModifierTotal(save, "ambush_ward") > 0 {
		aw.Opening = combat.OpeningNormal
		aw.Log = append(aw.Log, "  🔔 Your ward rings out as they close in. They don't catch you off guard.")
	}
	return aw, nil
}

// holdUnawareEncounter parks the encounter as pending and describes it.
func holdUnawareEncounter(sess *session.GameSession, monsterIDs []string, environmentID string, rollLog []string) UnawareView {
	sess.PendingEncounter = &types.PendingEncounter{MonsterIDs: monsterIDs, EnvironmentID: environmentID}
	return unawareView(sess.PendingEncounter, rollLog)
}

// unawareView builds the client payload for a pending encounter.
func unawareView(p *types.PendingEncounter, rollLog []string) UnawareView {
	view := UnawareView{Log: rollLog}
	counts := map[string]int{}
	names := map[string]string{}
	var order []string
	for _, id := range p.MonsterIDs {
		name := id
		if m, err := combat.LoadMonsterByID(serverdb.GetDB(), id); err == nil {
			name = m.Name
		}
		if counts[id] == 0 {
			order = append(order, id)
		}
		counts[id]++
		names[id] = name
		view.Monsters = append(view.Monsters, UnawareMonsterView{TemplateID: id, Name: name})
	}
	var parts []string
	for _, id := range order {
		if counts[id] == 1 {
			parts = append(parts, article(names[id])+" "+strings.ToLower(names[id]))
		} else {
			parts = append(parts, fmt.Sprintf("%d %ss", counts[id], strings.ToLower(names[id])))
		}
	}
	verb := "hasn't"
	if len(p.MonsterIDs) > 1 {
		verb = "haven't"
	}
	who := strings.Join(parts, " and ")
	view.Text = strings.ToUpper(who[:1]) + who[1:] + " " + verb + " noticed you. You could slip away — or strike first."
	return view
}

// article picks "A" or "An" for a name.
func article(name string) string {
	if name != "" && strings.ContainsRune("AEIOUaeiou", rune(name[0])) {
		return "an"
	}
	return "a"
}

// startPendingFight turns the pending encounter into a live fight.
func startPendingFight(sess *session.GameSession, monstersSurprised bool, lead []string) (*CombatStateResponse, error) {
	p := sess.PendingEncounter
	advancement, err := loadAdvancement()
	if err != nil {
		return nil, err
	}
	spec := combat.EncounterSpec{MonsterIDs: p.MonsterIDs, EnvironmentID: p.EnvironmentID, MonstersSurprised: monstersSurprised}
	cs, err := combat.StartEncounter(serverdb.GetDB(), &sess.SaveData, sess.Npub, spec, advancement)
	if err != nil {
		return nil, err
	}
	sess.PendingEncounter = nil
	if len(lead) > 0 {
		cs.Log = append(append([]string{}, lead...), cs.Log...)
	}
	sess.ActiveCombat = cs
	resp := buildStateResponse(cs, &sess.SaveData, cs.Log)
	combat.ClearSpawnPositions(cs)
	return &resp, nil
}

// CombatApproachHandler godoc
// @Summary      Answer an encounter whose monsters haven't noticed you
// @Description  choice: sneak_away (Stealth vs their passive Perception — on a failure the fight starts normally),
// @Description  sneak_attack (the fight starts with the monsters surprised: you act first, they lose their first turn,
// @Description  your attacks that turn have advantage), or approach (a normal fight).
// @Tags         Combat
// @Accept       json
// @Produce      json
// @Param        request  body      ApproachRequest  true  "Choice"
// @Success      200      {object}  ApproachResponse
// @Failure      400      {string}  string  "No pending encounter or bad choice"
// @Failure      404      {string}  string  "Session not found"
// @Router       /api/combat/approach [post]
func CombatApproachHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeCombatError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	var req ApproachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Npub == "" || req.SaveID == "" {
		writeCombatError(w, http.StatusBadRequest, "Missing npub or save_id")
		return
	}
	sess, err := session.GetSessionManager().GetSession(req.Npub, req.SaveID)
	if err != nil {
		writeCombatError(w, http.StatusNotFound, "Session not found")
		return
	}
	if sess.PendingEncounter == nil || sess.ActiveCombat != nil {
		writeCombatError(w, http.StatusBadRequest, "Nothing is waiting for you")
		return
	}

	var resp ApproachResponse
	switch req.Choice {
	case "sneak_away":
		monsters, err := combat.LoadEncounterMonsters(serverdb.GetDB(), sess.PendingEncounter.MonsterIDs)
		if err != nil {
			writeCombatError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Sneaking off the road: your light counts against you here too.
		stealthMod, _ := awarenessFrom(buildTravelContext(&sess.SaveData))
		ok, line := combat.SneakAway(rand.New(rand.NewSource(time.Now().UnixNano())), stealthMod, monsters)
		if ok {
			sess.PendingEncounter = nil
			resp = ApproachResponse{Success: true, SlippedAway: true, Message: "You slip away unseen.", Log: []string{line}}
			break
		}
		cs, err := startPendingFight(sess, false, []string{line, "👀 A twig snaps — they've spotted you!"})
		if err != nil {
			writeCombatError(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp = ApproachResponse{Success: true, Message: "They've spotted you!", Combat: cs}
	case "sneak_attack":
		cs, err := startPendingFight(sess, true, []string{"🗡 You creep closer and strike from hiding!"})
		if err != nil {
			writeCombatError(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp = ApproachResponse{Success: true, Combat: cs}
	case "approach":
		cs, err := startPendingFight(sess, false, []string{"You step out to face them."})
		if err != nil {
			writeCombatError(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp = ApproachResponse{Success: true, Combat: cs}
	default:
		writeCombatError(w, http.StatusBadRequest, "choice must be sneak_away, sneak_attack or approach")
		return
	}
	log.Printf("👁 Unaware encounter: npub=%s choice=%s slipped=%v", req.Npub, req.Choice, resp.SlippedAway)
	writeCombatJSON(w, http.StatusOK, resp)
}

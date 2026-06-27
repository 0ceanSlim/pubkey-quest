package game

import (
	"encoding/json"
	"net/http"
	"strings"

	"pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// ─── requirement context ──────────────────────────────────────────────────────

// questContext adapts a save into a requirement.Context by composing the
// canonical derivations (skills, level, quest points, inventory, identity).
type questContext struct {
	save        *types.SaveFile
	skillDefs   map[string]data.SkillDefinition
	advancement []types.AdvancementEntry
	// effStats is the player's stat block with active effect modifiers folded in
	// (base stats already include spent ability points). Computed once so skill
	// and stat gates read the same effective values a check would roll against.
	effStats map[string]interface{}
}

func buildQuestContext(save *types.SaveFile) requirement.Context {
	defs, _ := data.LoadSkillDefinitions()
	adv, _ := loadAdvancement()
	return questContext{save: save, skillDefs: defs, advancement: adv, effStats: effects.EffectiveStats(save)}
}

func (c questContext) SkillValue(id string) int {
	def, ok := c.skillDefs[id]
	if !ok {
		return 0
	}
	return data.CalculateSkillValue(c.effStats, def.Ratio)
}
func (c questContext) StatValue(id string) int {
	for k, v := range c.effStats {
		if !strings.EqualFold(k, id) {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}
func (c questContext) Level() int                      { return character.GetLevelFromXP(c.save.Experience, c.advancement) }
func (c questContext) QuestPoints() int                { return quest.QuestPoints(c.save, serverdb.GetQuestByID) }
func (c questContext) HasItem(id string) bool          { return gameutil.PlayerHasItem(c.save, id) }
func (c questContext) Class() string                   { return c.save.Class }
func (c questContext) Race() string                    { return c.save.Race }
func (c questContext) Alignment() string               { return c.save.Alignment }
func (c questContext) IsQuestCompleted(id string) bool { return quest.IsCompleted(c.save, id) }

// ─── log view ─────────────────────────────────────────────────────────────────

type objectiveView struct {
	Description string `json:"description"`
	Count       int    `json:"count"`
	Target      int    `json:"target"`
	Done        bool   `json:"done"`
}

type rewardItemView struct {
	ID       string `json:"id"`
	Quantity int    `json:"quantity"`
}

type rewardView struct {
	XP          int              `json:"xp,omitempty"`
	Gold        int              `json:"gold,omitempty"`
	QuestPoints int              `json:"quest_points,omitempty"`
	Items       []rewardItemView `json:"items,omitempty"`
}

// questRewardView summarises a quest's payout: its quest points (the quest's
// TotalQP, awarded on completion) plus the XP/gold/items from the last stage
// that carries a reward.
func questRewardView(qd *types.QuestData) *rewardView {
	rv := &rewardView{QuestPoints: qd.TotalQP}
	for i := len(qd.Stages) - 1; i >= 0; i-- {
		if r := qd.Stages[i].Rewards; r != nil {
			rv.XP = r.XP
			rv.Gold = r.Gold
			for _, it := range r.Items {
				rv.Items = append(rv.Items, rewardItemView{ID: it.ID, Quantity: it.Quantity})
			}
			break
		}
	}
	if rv.QuestPoints == 0 && rv.XP == 0 && rv.Gold == 0 && len(rv.Items) == 0 {
		return nil
	}
	return rv
}

type activeQuestView struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Category    string          `json:"category"`
	Difficulty  string          `json:"difficulty,omitempty"`
	Stage       int             `json:"stage"`
	StageCount  int             `json:"stage_count"`
	Description string          `json:"description"`
	Objectives  []objectiveView `json:"objectives"`
	Rewards     *rewardView     `json:"rewards,omitempty"`
}

type namedQuestView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category,omitempty"`
	Difficulty  string `json:"difficulty,omitempty"`
	Description string      `json:"description,omitempty"`
	StartHint   string      `json:"start_hint,omitempty"`
	Rewards     *rewardView `json:"rewards,omitempty"`
}

type questLogView struct {
	Active      []activeQuestView `json:"active"`
	Completed   []namedQuestView  `json:"completed"`
	Available   []namedQuestView  `json:"available"`
	QuestPoints int               `json:"quest_points"`
}

func buildQuestLog(save *types.SaveFile, ctx requirement.Context) questLogView {
	view := questLogView{QuestPoints: quest.QuestPoints(save, serverdb.GetQuestByID)}

	for _, qp := range save.QuestsActive {
		qd, err := serverdb.GetQuestByID(qp.QuestID)
		if err != nil || qd == nil {
			continue
		}
		av := activeQuestView{
			ID: qd.ID, Name: qd.Name, Category: string(qd.Category), Difficulty: qd.Difficulty,
			Stage: qp.Stage, StageCount: len(qd.Stages), Rewards: questRewardView(qd),
		}
		if qp.Stage < len(qd.Stages) {
			stage := qd.Stages[qp.Stage]
			av.Description = stage.Description
			for j, obj := range stage.Objectives {
				target := obj.Count
				if target <= 0 {
					target = 1
				}
				count := 0
				if j < len(qp.ObjectiveCounts) {
					count = qp.ObjectiveCounts[j]
				}
				av.Objectives = append(av.Objectives, objectiveView{
					Description: obj.Description, Count: count, Target: target, Done: count >= target,
				})
			}
		}
		view.Active = append(view.Active, av)
	}

	for _, id := range save.QuestsCompleted {
		name := id
		if qd, err := serverdb.GetQuestByID(id); err == nil && qd != nil {
			name = qd.Name
		}
		view.Completed = append(view.Completed, namedQuestView{ID: id, Name: name})
	}

	all, _ := serverdb.GetAllQuests()
	for _, qd := range quest.Available(all, save, ctx) {
		view.Available = append(view.Available, namedQuestView{
			ID: qd.ID, Name: qd.Name, Category: string(qd.Category),
			Difficulty: qd.Difficulty, Description: qd.Description,
			StartHint: qd.StartCondition.StartHint, Rewards: questRewardView(&qd),
		})
	}
	return view
}

// injectQuestOffers adds the quests this NPC gives — those whose start
// condition is talking to it, and which the player can currently start — onto
// the dialogue delta as offered_quests, so the talk UI can present them.
func injectQuestOffers(resp *types.GameActionResponse, npcID string, state *types.SaveFile) {
	if resp.Delta == nil {
		return
	}
	dlg, ok := resp.Delta["npc_dialogue"].(map[string]interface{})
	if !ok {
		return
	}
	all, err := serverdb.GetAllQuests()
	if err != nil {
		return
	}
	ctx := buildQuestContext(state)

	var offers []map[string]interface{}
	for _, q := range all {
		if q.StartCondition.Type == "talk" && q.StartCondition.Target == npcID && quest.CanStart(q, state, ctx) {
			offers = append(offers, map[string]interface{}{
				"id":          q.ID,
				"name":        q.Name,
				"category":    string(q.Category),
				"difficulty":  q.Difficulty,
				"description": q.Description,
			})
		}
	}
	if len(offers) > 0 {
		dlg["offered_quests"] = offers
	}
}

// ─── handlers ─────────────────────────────────────────────────────────────────

type questActionRequest struct {
	Npub    string `json:"npub"`
	SaveID  string `json:"save_id"`
	QuestID string `json:"quest_id"`
}

// QuestLogHandler returns the player's quest log: active quests with objective
// progress, completed quests, currently-available quests, and the QP total.
func QuestLogHandler(w http.ResponseWriter, r *http.Request) {
	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		writeQuestError(w, http.StatusBadRequest, "missing npub or save_id")
		return
	}
	sess, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		writeQuestError(w, http.StatusNotFound, "session not found")
		return
	}
	ctx := buildQuestContext(&sess.SaveData)
	writeQuestJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    buildQuestLog(&sess.SaveData, ctx),
	})
}

// QuestAcceptHandler starts a quest for the player after availability checks.
func QuestAcceptHandler(w http.ResponseWriter, r *http.Request) {
	req, sess, ok := questActionSession(w, r)
	if !ok {
		return
	}
	qd, err := serverdb.GetQuestByID(req.QuestID)
	if err != nil || qd == nil {
		writeQuestError(w, http.StatusNotFound, "quest not found")
		return
	}
	ctx := buildQuestContext(&sess.SaveData)
	if err := quest.Accept(*qd, &sess.SaveData, ctx); err != nil {
		writeQuestError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeQuestJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Quest accepted: " + qd.Name,
		"data":    buildQuestLog(&sess.SaveData, ctx),
	})
}

// QuestAbandonHandler drops an in-progress quest.
func QuestAbandonHandler(w http.ResponseWriter, r *http.Request) {
	req, sess, ok := questActionSession(w, r)
	if !ok {
		return
	}
	if err := quest.Abandon(&sess.SaveData, req.QuestID); err != nil {
		writeQuestError(w, http.StatusBadRequest, err.Error())
		return
	}
	ctx := buildQuestContext(&sess.SaveData)
	writeQuestJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Quest abandoned",
		"data":    buildQuestLog(&sess.SaveData, ctx),
	})
}

// questActionSession parses a POST quest action and resolves its session.
func questActionSession(w http.ResponseWriter, r *http.Request) (questActionRequest, *session.GameSession, bool) {
	var req questActionRequest
	if r.Method != http.MethodPost {
		writeQuestError(w, http.StatusMethodNotAllowed, "method not allowed")
		return req, nil, false
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeQuestError(w, http.StatusBadRequest, "invalid request body")
		return req, nil, false
	}
	if req.Npub == "" || req.SaveID == "" || req.QuestID == "" {
		writeQuestError(w, http.StatusBadRequest, "missing npub, save_id, or quest_id")
		return req, nil, false
	}
	sess, err := session.GetSessionManager().GetSession(req.Npub, req.SaveID)
	if err != nil {
		writeQuestError(w, http.StatusNotFound, "session not found")
		return req, nil, false
	}
	return req, sess, true
}

func writeQuestJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeQuestError(w http.ResponseWriter, status int, msg string) {
	writeQuestJSON(w, status, map[string]interface{}{"success": false, "error": msg})
}

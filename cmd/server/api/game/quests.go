package game

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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
//
// Every quest in the log has the same shape, whatever its status, so the journal
// can sort and filter one list. Status is one of:
//
//	active    — in progress (stage, objectives)
//	available — startable now (start hint)
//	locked    — exists for this character but a requirement or prerequisite is
//	            unmet; Requirements says which
//	completed — done

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

// requirementView is one gate on starting a quest, phrased for the player, and
// whether this character currently meets it.
type requirementView struct {
	Description string `json:"description"`
	Met         bool   `json:"met"`
}

type recommendedView struct {
	Stats  []string `json:"stats,omitempty"`
	Danger string   `json:"danger,omitempty"`
}

type questView struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Status           string            `json:"status"`
	Category         string            `json:"category,omitempty"`
	Difficulty       string            `json:"difficulty,omitempty"`
	Description      string            `json:"description,omitempty"`
	StartHint        string            `json:"start_hint,omitempty"`
	Stage            int               `json:"stage"`
	StageCount       int               `json:"stage_count"`
	StageDescription string            `json:"stage_description,omitempty"`
	Objectives       []objectiveView   `json:"objectives,omitempty"`
	Requirements     []requirementView `json:"requirements,omitempty"`
	Recommended      *recommendedView  `json:"recommended,omitempty"`
	Rewards          *rewardView       `json:"rewards,omitempty"`
}

type questLogView struct {
	Active      []questView `json:"active"`
	Available   []questView `json:"available"`
	Locked      []questView `json:"locked"`
	Completed   []questView `json:"completed"`
	QuestPoints int         `json:"quest_points"`
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

// baseQuestView fills the fields every status shares.
func baseQuestView(qd *types.QuestData, status string) questView {
	v := questView{
		ID: qd.ID, Name: qd.Name, Status: status, Category: string(qd.Category),
		Difficulty: qd.Difficulty, Description: qd.Description,
		StartHint: qd.StartCondition.StartHint, StageCount: len(qd.Stages),
		Rewards: questRewardView(qd),
	}
	if rec := qd.Recommended; rec != nil && (len(rec.RecommendedStats) > 0 || rec.CombatDanger != "") {
		v.Recommended = &recommendedView{Stats: rec.RecommendedStats, Danger: rec.CombatDanger}
	}
	return v
}

// questRequirements lists every gate on starting the quest — prerequisite
// quests first, then requirements — each marked met or not for this character.
func questRequirements(qd *types.QuestData, save *types.SaveFile, ctx requirement.Context) []requirementView {
	var out []requirementView
	for _, pre := range qd.Prerequisites {
		name := pre
		if pq, err := serverdb.GetQuestByID(pre); err == nil && pq != nil {
			name = pq.Name
		}
		out = append(out, requirementView{Description: "Complete " + name, Met: quest.IsCompleted(save, pre)})
	}
	for _, req := range qd.Requirements {
		out = append(out, requirementView{Description: describeRequirement(req), Met: requirement.EvaluateOne(req, ctx)})
	}
	return out
}

// describeRequirement phrases a requirement for the player: the authored
// description when there is one, otherwise a short label built from the rule.
func describeRequirement(req types.POIRequirement) string {
	if req.Description != "" {
		return req.Description
	}
	title := func(s string) string {
		s = strings.ReplaceAll(s, "_", " ")
		s = strings.ReplaceAll(s, "-", " ")
		if s == "" {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	}
	list := func(vs []string) string {
		out := make([]string, len(vs))
		for i, v := range vs {
			out[i] = title(v)
		}
		return strings.Join(out, " or ")
	}
	switch req.Type {
	case "skill", "stat":
		return fmt.Sprintf("%s %d", title(req.ID), req.Min)
	case "level":
		return fmt.Sprintf("Level %d", req.Min)
	case "quest_points":
		return fmt.Sprintf("%d Quest Points", req.Min)
	case "item":
		return "Carry " + title(req.ID)
	case "class", "race", "alignment":
		return title(req.Type) + ": " + list(req.Values)
	case "quest_completed":
		name := req.ID
		if pq, err := serverdb.GetQuestByID(req.ID); err == nil && pq != nil {
			name = pq.Name
		}
		return "Complete " + name
	}
	return title(req.Type)
}

func buildQuestLog(save *types.SaveFile, ctx requirement.Context) questLogView {
	view := questLogView{
		Active: []questView{}, Available: []questView{}, Locked: []questView{}, Completed: []questView{},
		QuestPoints: quest.QuestPoints(save, serverdb.GetQuestByID),
	}

	for _, qp := range save.QuestsActive {
		qd, err := serverdb.GetQuestByID(qp.QuestID)
		if err != nil || qd == nil {
			continue
		}
		av := baseQuestView(qd, "active")
		av.Stage = qp.Stage
		if qp.Stage < len(qd.Stages) {
			stage := qd.Stages[qp.Stage]
			av.StageDescription = stage.Description
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
		if qd, err := serverdb.GetQuestByID(id); err == nil && qd != nil {
			view.Completed = append(view.Completed, baseQuestView(qd, "completed"))
		} else {
			view.Completed = append(view.Completed, questView{ID: id, Name: id, Status: "completed"})
		}
	}

	all, _ := serverdb.GetAllQuests()
	available := map[string]bool{}
	for _, qd := range quest.Available(all, save, ctx) {
		v := baseQuestView(&qd, "available")
		v.Requirements = questRequirements(&qd, save, ctx)
		view.Available = append(view.Available, v)
		available[qd.ID] = true
	}

	// Locked: quests this character could pursue but can't start yet. Daily and
	// weekly pools only show the current pick, and only when it's still to do.
	now := time.Now()
	for i := range all {
		qd := &all[i]
		if available[qd.ID] || quest.IsActive(save, qd.ID) || quest.IsCompleted(save, qd.ID) {
			continue
		}
		if quest.IsRepeatable(qd.Category) {
			if cur, ok := quest.CurrentRepeatable(all, qd.Category, now); !ok || cur.ID != qd.ID ||
				!quest.RepeatableAvailable(*qd, save, all, now) {
				continue
			}
		}
		if barredForLife(qd, ctx) {
			continue // e.g. an elves-only quest for a human — it can never open up
		}
		v := baseQuestView(qd, "locked")
		v.Requirements = questRequirements(qd, save, ctx)
		view.Locked = append(view.Locked, v)
	}
	return view
}

// permanentRequirementTypes are gates on traits fixed at character creation —
// nothing in play changes them — so failing one means the quest is never
// startable by this character.
var permanentRequirementTypes = map[string]bool{"race": true, "class": true, "alignment": true}

// barredForLife reports whether a quest fails one of those permanent gates.
// Such quests are left out of the journal entirely rather than shown as locked.
func barredForLife(qd *types.QuestData, ctx requirement.Context) bool {
	for _, req := range qd.Requirements {
		if permanentRequirementTypes[req.Type] && !requirement.EvaluateOne(req, ctx) {
			return true
		}
	}
	return false
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

	offer := func(q types.QuestData) map[string]interface{} {
		return map[string]interface{}{
			"id":          q.ID,
			"name":        q.Name,
			"category":    string(q.Category),
			"difficulty":  q.Difficulty,
			"description": q.Description,
		}
	}

	var offers []map[string]interface{}
	for _, q := range all {
		if q.StartCondition.Type == "talk" && q.StartCondition.Target == npcID && quest.CanStart(q, state, ctx) {
			offers = append(offers, offer(q))
		}
	}

	// Vault keepers (marked by storage_config) hand out the day's daily and the
	// week's weekly bounty — the current pick from each pool, if the player hasn't
	// done it this period and meets its requirements. Vault keepers are on duty
	// around the clock, so a bounty can be picked up at any hour.
	if npcData, err := serverdb.GetNPCByID(npcID); err == nil && len(npcData.StorageConfig) > 0 {
		now := time.Now()
		for _, cat := range []types.QuestCategory{types.QuestDaily, types.QuestWeekly} {
			if q, ok := quest.CurrentRepeatable(all, cat, now); ok &&
				quest.RepeatableAvailable(q, state, all, now) &&
				requirement.Evaluate(q.Requirements, ctx).OK {
				offers = append(offers, offer(q))
			}
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

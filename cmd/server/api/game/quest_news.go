package game

import (
	"slices"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// questMarks is where a save's quests stood (session.QuestMarks).
type questMarks = session.QuestMarks

func markQuests(save *types.SaveFile) questMarks {
	m := questMarks{Active: map[string]int{}, Repeatable: map[string]int{}}
	for _, qp := range save.QuestsActive {
		m.Active[qp.QuestID] = qp.Stage
	}
	m.Completed = append(m.Completed, save.QuestsCompleted...)
	for id, p := range save.RepeatableQuests {
		m.Repeatable[id] = p
	}
	return m
}

// questNews adds to an action's response the quests it finished
// (quests_completed) and moved on (quests_advanced), so the client can say so
// and refresh the journal and tracker at once instead of on the next reload.
// A repeatable counts as finished when its period mark moves.
func questNews(before questMarks, save *types.SaveFile, response *GameActionResponse) {
	now := markQuests(save)
	var done, advanced []map[string]any
	entry := func(id string) map[string]any {
		name := id
		if q, err := serverdb.GetQuestByID(id); err == nil && q != nil && q.Name != "" {
			name = q.Name
		}
		return map[string]any{"id": id, "name": name}
	}
	for id := range before.Active {
		if _, still := now.Active[id]; still {
			continue
		}
		if (slices.Contains(now.Completed, id) && !slices.Contains(before.Completed, id)) ||
			now.Repeatable[id] != before.Repeatable[id] {
			e := entry(id)
			e["rewards"] = questRewards(id)
			done = append(done, e)
		}
	}
	for id, stage := range now.Active {
		if was, ok := before.Active[id]; ok && stage > was {
			advanced = append(advanced, entry(id))
		}
	}
	if len(done) == 0 && len(advanced) == 0 {
		return
	}
	if response.Data == nil {
		response.Data = make(map[string]interface{})
	}
	if len(done) > 0 {
		response.Data["quests_completed"] = done
	}
	if len(advanced) > 0 {
		response.Data["quests_advanced"] = advanced
	}
}

// questNewsFields is questNews for handlers outside the game-action route (the
// shop, spell preparation, saving): the quests_completed / quests_advanced
// fields to merge into their own response, or nil when nothing moved.
func questNewsFields(before questMarks, save *types.SaveFile) map[string]any {
	resp := &GameActionResponse{}
	questNews(before, save, resp)
	return resp.Data
}

// questRewards is what finishing a quest gave, for the rewards screen: its last
// stage's reward (XP, gold, items by name) and its quest points.
func questRewards(questID string) map[string]any {
	out := map[string]any{}
	q, err := serverdb.GetQuestByID(questID)
	if err != nil || q == nil || len(q.Stages) == 0 {
		return out
	}
	if q.TotalQP > 0 {
		out["quest_points"] = q.TotalQP
	}
	r := q.Stages[len(q.Stages)-1].Rewards
	if r == nil {
		return out
	}
	if r.XP > 0 {
		out["xp"] = r.XP
	}
	if r.Gold > 0 {
		out["gold"] = r.Gold
	}
	var items []map[string]any
	for _, it := range r.Items {
		name := it.ID
		if item, err := serverdb.GetItemByID(it.ID); err == nil && item != nil && item.Name != "" {
			name = item.Name
		}
		items = append(items, map[string]any{"id": it.ID, "name": name, "quantity": max(it.Quantity, 1)})
	}
	if len(items) > 0 {
		out["items"] = items
	}
	return out
}

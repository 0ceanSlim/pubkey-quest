package game

import (
	"slices"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// questMarks is where a save's quests stood: the active stage of each, which
// are done for good, and the period each repeatable was last completed in.
type questMarks struct {
	active     map[string]int
	completed  []string
	repeatable map[string]int
}

func markQuests(save *types.SaveFile) questMarks {
	m := questMarks{active: map[string]int{}, repeatable: map[string]int{}}
	for _, qp := range save.QuestsActive {
		m.active[qp.QuestID] = qp.Stage
	}
	m.completed = append(m.completed, save.QuestsCompleted...)
	for id, p := range save.RepeatableQuests {
		m.repeatable[id] = p
	}
	return m
}

// questNews adds to an action's response the quests it finished
// (quests_completed) and moved on (quests_advanced), so the client can say so
// and refresh the journal and tracker at once instead of on the next reload.
// A repeatable counts as finished when its period mark moves.
func questNews(before questMarks, save *types.SaveFile, response *GameActionResponse) {
	now := markQuests(save)
	var done, advanced []map[string]string
	entry := func(id string) map[string]string {
		name := id
		if q, err := serverdb.GetQuestByID(id); err == nil && q != nil && q.Name != "" {
			name = q.Name
		}
		return map[string]string{"id": id, "name": name}
	}
	for id := range before.active {
		if _, still := now.active[id]; still {
			continue
		}
		if (slices.Contains(now.completed, id) && !slices.Contains(before.completed, id)) ||
			now.repeatable[id] != before.repeatable[id] {
			done = append(done, entry(id))
		}
	}
	for id, stage := range now.active {
		if was, ok := before.active[id]; ok && stage > was {
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

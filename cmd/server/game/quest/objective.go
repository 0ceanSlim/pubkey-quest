package quest

import (
	"time"

	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/types"
)

// QuestLookup fetches a quest definition by id (e.g. db.GetQuestByID). The
// engine takes it as a function so this package stays decoupled from the DB.
type QuestLookup func(id string) (*types.QuestData, error)

// Consumer returns an events.Consumer that drives quest progress: as gameplay
// events arrive it advances the matching objectives on each active quest,
// completes stages (granting their rewards), and moves finished quests to the
// completed list. Register it once at startup with the loaded advancement table:
//
//	events.Subscribe(quest.Consumer(db.GetQuestByID, advancement))
func Consumer(lookup QuestLookup, advancement []types.AdvancementEntry) events.Consumer {
	return func(save *types.SaveFile, ev events.Event) {
		advanceObjectives(save, ev, lookup, advancement)
	}
}

// advanceObjectives applies one event to every active quest, then prunes any
// that finished.
func advanceObjectives(save *types.SaveFile, ev events.Event, lookup QuestLookup, advancement []types.AdvancementEntry) {
	for i := range save.QuestsActive {
		qp := &save.QuestsActive[i]
		quest, err := lookup(qp.QuestID)
		if err != nil || quest == nil || qp.Stage >= len(quest.Stages) {
			continue
		}
		stage := quest.Stages[qp.Stage]
		ensureCounts(qp, len(stage.Objectives))

		changed := false
		for j, obj := range stage.Objectives {
			if !objectiveMatches(obj, ev) {
				continue
			}
			need := objectiveTarget(obj)
			if obj.Type == types.ObjectiveFetch {
				// Fetch counts what you're carrying, not how often an item
				// passed through your hands — so it can't be farmed by dropping
				// and re-taking the same thing, and it falls if you sell it.
				if held := min(gameutil.CountItem(save, obj.Target), need); held != qp.ObjectiveCounts[j] {
					qp.ObjectiveCounts[j] = held
					changed = true
				}
				continue
			}
			if qp.ObjectiveCounts[j] >= need {
				continue
			}
			qp.ObjectiveCounts[j] += ev.Count
			if qp.ObjectiveCounts[j] > need {
				qp.ObjectiveCounts[j] = need
			}
			changed = true
		}

		if changed && stageComplete(stage, qp.ObjectiveCounts) {
			completeStage(save, qp, quest, advancement)
		}
	}
	pruneCompleted(save, lookup)
}

// completeStage grants the finished stage's reward and advances to the next
// stage (resetting its objective counters). When the stage was the last one the
// quest is finished; pruneCompleted then moves it to the completed list.
func completeStage(save *types.SaveFile, qp *types.QuestProgress, quest *types.QuestData, advancement []types.AdvancementEntry) {
	stage := quest.Stages[qp.Stage]
	// Hand-ins: the items a "consume" fetch asked for go to whoever wanted them.
	for _, obj := range stage.Objectives {
		if obj.Type == types.ObjectiveFetch && obj.Consume {
			gameutil.ConsumeItem(save, obj.Target, objectiveTarget(obj))
		}
	}
	if stage.Rewards != nil {
		GrantReward(save, stage.Rewards, advancement)
	}
	qp.Stage++
	if qp.Stage < len(quest.Stages) {
		// NOTE: wait stages (Stage.WaitMinutes) advance immediately for now —
		// the ready_at clock gating is a later refinement.
		next := quest.Stages[qp.Stage]
		qp.ObjectiveCounts = make([]int, len(next.Objectives))
		PrimeFetch(save, qp, next)
		// Already holding everything the next stage asks for: it's done too.
		if len(next.Objectives) > 0 && stageComplete(next, qp.ObjectiveCounts) {
			completeStage(save, qp, quest, advancement)
		}
	}
}

// PrimeFetch fills a stage's fetch objectives from what the player already
// carries, so "bring me three pelts" counts the pelts in your pack.
func PrimeFetch(save *types.SaveFile, qp *types.QuestProgress, stage types.QuestStage) {
	ensureCounts(qp, len(stage.Objectives))
	for j, obj := range stage.Objectives {
		if obj.Type == types.ObjectiveFetch {
			qp.ObjectiveCounts[j] = min(gameutil.CountItem(save, obj.Target), objectiveTarget(obj))
		}
	}
}

// UnlocksPOI reports whether a quest stage the player has finished reveals the
// POI (types.QuestStage.UnlocksPOI). Derived from quest progress rather than
// stored, per the hydration rule.
func UnlocksPOI(save *types.SaveFile, poiID string, lookup QuestLookup) bool {
	unlockedBy := func(questID string, doneStages int) bool {
		q, err := lookup(questID)
		if err != nil || q == nil {
			return false
		}
		for i := 0; i < doneStages && i < len(q.Stages); i++ {
			if q.Stages[i].UnlocksPOI == poiID {
				return true
			}
		}
		return false
	}
	for _, qp := range save.QuestsActive {
		if unlockedBy(qp.QuestID, qp.Stage) {
			return true
		}
	}
	for _, id := range save.QuestsCompleted {
		if unlockedBy(id, 1<<30) {
			return true
		}
	}
	return false
}

// pruneCompleted moves any active quest whose stage index has run past its last
// stage onto the completed list, and drops it from the active list.
func pruneCompleted(save *types.SaveFile, lookup QuestLookup) {
	if len(save.QuestsActive) == 0 {
		return
	}
	kept := save.QuestsActive[:0]
	for _, qp := range save.QuestsActive {
		quest, err := lookup(qp.QuestID)
		if err == nil && quest != nil && qp.Stage >= len(quest.Stages) {
			if IsRepeatable(quest.Category) {
				// Daily/weekly: don't permanently complete — record the period so
				// it re-opens next reset, and drop it from the active list.
				markRepeatableDone(save, quest, time.Now())
			} else if !contains(save.QuestsCompleted, qp.QuestID) {
				save.QuestsCompleted = append(save.QuestsCompleted, qp.QuestID)
			}
			continue
		}
		kept = append(kept, qp)
	}
	save.QuestsActive = kept
}

// objectiveMatches reports whether an event satisfies progress on an objective.
// The event kinds line up one-to-one with the objective types.
func objectiveMatches(obj types.QuestObjective, ev events.Event) bool {
	switch obj.Type {
	case types.ObjectiveSlay:
		return ev.Kind == events.MonsterKilled && obj.Target == ev.Target
	case types.ObjectiveFetch:
		return ev.Kind == events.ItemAcquired && obj.Target == ev.Target
	case types.ObjectiveExplore:
		return ev.Kind == events.LocationDiscovered && obj.Target == ev.Target
	case types.ObjectiveTalk:
		return ev.Kind == events.NPCTalked && obj.Target == ev.Target
	case types.ObjectiveCheck:
		return ev.Kind == events.SkillCheckPassed && obj.Skill == ev.Target
	}
	return false
}

// objectiveTarget is the count an objective needs (default 1).
func objectiveTarget(obj types.QuestObjective) int {
	if obj.Count > 0 {
		return obj.Count
	}
	return 1
}

// stageComplete reports whether every objective in the stage has met its target.
func stageComplete(stage types.QuestStage, counts []int) bool {
	for j, obj := range stage.Objectives {
		if j >= len(counts) || counts[j] < objectiveTarget(obj) {
			return false
		}
	}
	return true
}

// ensureCounts grows a progress record's counter slice to cover all objectives
// (defensive against quest content gaining objectives between saves).
func ensureCounts(qp *types.QuestProgress, n int) {
	if len(qp.ObjectiveCounts) >= n {
		return
	}
	grown := make([]int, n)
	copy(grown, qp.ObjectiveCounts)
	qp.ObjectiveCounts = grown
}

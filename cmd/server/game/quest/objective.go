package quest

import (
	"strings"
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
		final := IsFinalStage(save, qp.Stage, quest)

		changed := false
		if ev.Kind == events.TurnedIn && ev.Target == quest.ID {
			// Handing in: count what's carried right now.
			PrimeFetch(save, qp, stage)
			changed = true
		}
		for j, obj := range stage.Objectives {
			if IsTurnIn(obj, final) {
				// The last step's "talk to them" is the turn-in: only choosing
				// Turn in meets it, not opening the conversation.
				if ev.Kind != events.TurnedIn || ev.Target != quest.ID {
					continue
				}
			} else if !objectiveMatches(obj, ev) {
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
	skipStages(save, qp, quest)
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

// IsFinalStage reports whether a stage is the last one this character will do
// (any later stages are for other classes).
func IsFinalStage(save *types.SaveFile, stageIdx int, quest *types.QuestData) bool {
	for k := stageIdx + 1; k < len(quest.Stages); k++ {
		if stageApplies(quest.Stages[k], save) {
			return false
		}
	}
	return true
}

// IsTurnIn reports whether an objective is a quest's turn-in: a "talk" objective
// on its final stage, met only by choosing Turn in (events.TurnedIn).
func IsTurnIn(obj types.QuestObjective, finalStage bool) bool {
	return finalStage && obj.Type == types.ObjectiveTalk
}

// ReadyToTurnIn reports whether an active quest is on its final stage, with a
// turn-in to targetNPC (or quest.AnyVaultKeeper, when keeper is true) and every
// other objective of the stage done. Fetch objectives count what's carried now.
func ReadyToTurnIn(save *types.SaveFile, quest *types.QuestData, npcID string, keeper bool) bool {
	for i := range save.QuestsActive {
		qp := &save.QuestsActive[i]
		if qp.QuestID != quest.ID || qp.Stage >= len(quest.Stages) || !IsFinalStage(save, qp.Stage, quest) {
			continue
		}
		stage := quest.Stages[qp.Stage]
		ensureCounts(qp, len(stage.Objectives))
		turnIn := false
		for j, obj := range stage.Objectives {
			switch {
			case IsTurnIn(obj, true):
				if obj.Target == npcID || (keeper && obj.Target == AnyVaultKeeper) {
					turnIn = true
				}
			case obj.Type == types.ObjectiveFetch:
				if gameutil.CountItem(save, obj.Target) < objectiveTarget(obj) {
					return false
				}
			case qp.ObjectiveCounts[j] < objectiveTarget(obj):
				return false
			}
		}
		return turnIn
	}
	return false
}

// stageApplies reports whether a stage is for this character: a stage limited
// to some classes (types.QuestStage.Classes) is skipped by everyone else.
func stageApplies(stage types.QuestStage, save *types.SaveFile) bool {
	if len(stage.Classes) == 0 {
		return true
	}
	for _, c := range stage.Classes {
		if strings.EqualFold(c, save.Class) {
			return true
		}
	}
	return false
}

// skipStages moves past any stages from the current one on that aren't for this
// character, without their objectives or rewards.
func skipStages(save *types.SaveFile, qp *types.QuestProgress, quest *types.QuestData) {
	for qp.Stage < len(quest.Stages) && !stageApplies(quest.Stages[qp.Stage], save) {
		qp.Stage++
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
		return (ev.Kind == events.LocationDiscovered || ev.Kind == events.LocationVisited) && obj.Target == ev.Target
	case types.ObjectiveTalk:
		return ev.Kind == events.NPCTalked && obj.Target == ev.Target
	case types.ObjectiveCheck:
		return ev.Kind == events.SkillCheckPassed && obj.Skill == ev.Target
	case types.ObjectiveDo:
		return ev.Kind == events.Did && obj.Target == ev.Target
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

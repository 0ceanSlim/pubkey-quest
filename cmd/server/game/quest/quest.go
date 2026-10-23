// Package quest is the M3 quest engine: it decides which quests a player can
// start, starts them, tracks objective progress fed by the event
// recorder, and pays out rewards on completion. Availability gating reuses the
// shared requirement evaluator; objective progress flows from gameplay events
// (see Consumer); quest points are always derived from the completed list.
package quest

import (
	"fmt"
	"time"

	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/types"
)

// IsCompleted reports whether the player has finished the quest.
func IsCompleted(save *types.SaveFile, questID string) bool {
	return contains(save.QuestsCompleted, questID)
}

// StageOf is the quest's active stage index, or requirement.StageCompleted /
// requirement.StageNotStarted — what quest_stage requirements gate on.
func StageOf(save *types.SaveFile, questID string) int {
	for _, qp := range save.QuestsActive {
		if qp.QuestID == questID {
			return qp.Stage
		}
	}
	if IsCompleted(save, questID) {
		return requirement.StageCompleted
	}
	return requirement.StageNotStarted
}

// IsActive reports whether the quest is currently in progress.
func IsActive(save *types.SaveFile, questID string) bool {
	for _, qp := range save.QuestsActive {
		if qp.QuestID == questID {
			return true
		}
	}
	return false
}

// CanStart reports whether a quest is startable right now: not already
// active/completed, every prerequisite quest completed, and all requirements
// satisfied (via the shared evaluator).
func CanStart(q types.QuestData, save *types.SaveFile, ctx requirement.Context) bool {
	if IsCompleted(save, q.ID) || IsActive(save, q.ID) {
		return false
	}
	// Creation quests (the tutorial) are started for a new character, never offered.
	if q.StartCondition.Type == StartAtCreation {
		return false
	}
	for _, pre := range q.Prerequisites {
		if !IsCompleted(save, pre) {
			return false
		}
	}
	return requirement.Evaluate(q.Requirements, ctx).OK
}

// Available filters a quest set down to those the player can start now.
func Available(all []types.QuestData, save *types.SaveFile, ctx requirement.Context) []types.QuestData {
	now := time.Now()
	var out []types.QuestData
	for _, q := range all {
		// Repeatables (daily/weekly) never enter the completed list, so CanStart
		// would always pass them — gate instead on "this period's pick, not yet
		// done this period," still honoring requirements.
		if IsRepeatable(q.Category) {
			if RepeatableAvailable(q, save, all, now) && requirement.Evaluate(q.Requirements, ctx).OK {
				out = append(out, q)
			}
			continue
		}
		if CanStart(q, save, ctx) {
			out = append(out, q)
		}
	}
	return out
}

// StartAtCreation is the start_condition type of a quest that a new character
// begins with (the tutorial): StartForNewCharacter starts it, nobody offers it.
const StartAtCreation = "creation"

// StartForNewCharacter begins a creation quest on a brand-new save. It skips
// the requirement checks Accept makes (there's no game context yet, and the
// quest is chosen by the character's starting city, not offered), and any
// opening stages that aren't for this character's class.
func StartForNewCharacter(q types.QuestData, save *types.SaveFile) {
	if IsActive(save, q.ID) || IsCompleted(save, q.ID) {
		return
	}
	qp := types.QuestProgress{QuestID: q.ID}
	skipStages(save, &qp, &q)
	if qp.Stage < len(q.Stages) {
		qp.ObjectiveCounts = make([]int, len(q.Stages[qp.Stage].Objectives))
		PrimeFetch(save, &qp, q.Stages[qp.Stage])
	}
	save.QuestsActive = append(save.QuestsActive, qp)
}

// FinishNow completes an active quest on the spot, granting only its last
// stage's reward: how the tutorial's "I've done this before" skips the lessons
// but keeps the welcome. A quest that isn't active is left alone.
func FinishNow(save *types.SaveFile, q *types.QuestData, advancement []types.AdvancementEntry) {
	for i := range save.QuestsActive {
		qp := &save.QuestsActive[i]
		if qp.QuestID != q.ID || len(q.Stages) == 0 {
			continue
		}
		if last := q.Stages[len(q.Stages)-1]; last.Rewards != nil {
			GrantReward(save, last.Rewards, advancement)
		}
		qp.Stage = len(q.Stages)
		pruneCompleted(save, func(id string) (*types.QuestData, error) {
			if id == q.ID {
				return q, nil
			}
			return nil, fmt.Errorf("not %s", q.ID) // other quests are left as they are
		})
		return
	}
}

// Accept starts a quest after revalidating that it can be started, recording a
// fresh progress entry (stage 0, zeroed objective counters).
func Accept(q types.QuestData, save *types.SaveFile, ctx requirement.Context) error {
	if IsCompleted(save, q.ID) {
		return fmt.Errorf("quest %q already completed", q.ID)
	}
	if IsActive(save, q.ID) {
		return fmt.Errorf("quest %q already active", q.ID)
	}
	for _, pre := range q.Prerequisites {
		if !IsCompleted(save, pre) {
			return fmt.Errorf("prerequisite not met: %s", pre)
		}
	}
	if res := requirement.Evaluate(q.Requirements, ctx); !res.OK {
		return fmt.Errorf("requirements not met for %q", q.ID)
	}

	objectives := 0
	if len(q.Stages) > 0 {
		objectives = len(q.Stages[0].Objectives)
	}
	save.QuestsActive = append(save.QuestsActive, types.QuestProgress{
		QuestID:         q.ID,
		Stage:           0,
		ObjectiveCounts: make([]int, objectives),
	})
	if len(q.Stages) > 0 {
		PrimeFetch(save, &save.QuestsActive[len(save.QuestsActive)-1], q.Stages[0])
	}
	return nil
}

// QuestPoints derives the player's total quest points from their completed
// quests — the canonical example of the hydration rule (the number is never
// stored, only the completed-quest list is).
func QuestPoints(save *types.SaveFile, lookup QuestLookup) int {
	total := 0
	for _, id := range save.QuestsCompleted {
		if q, err := lookup(id); err == nil && q != nil {
			total += q.TotalQP
		}
	}
	return total
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

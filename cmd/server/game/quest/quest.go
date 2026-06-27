// Package quest is the M3 quest engine: it decides which quests a player can
// start, starts and abandons them, tracks objective progress fed by the event
// recorder, and pays out rewards on completion. Availability gating reuses the
// shared requirement evaluator; objective progress flows from gameplay events
// (see Consumer); quest points are always derived from the completed list.
package quest

import (
	"fmt"

	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/types"
)

// IsCompleted reports whether the player has finished the quest.
func IsCompleted(save *types.SaveFile, questID string) bool {
	return contains(save.QuestsCompleted, questID)
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
	for _, pre := range q.Prerequisites {
		if !IsCompleted(save, pre) {
			return false
		}
	}
	return requirement.Evaluate(q.Requirements, ctx).OK
}

// Available filters a quest set down to those the player can start now.
func Available(all []types.QuestData, save *types.SaveFile, ctx requirement.Context) []types.QuestData {
	var out []types.QuestData
	for _, q := range all {
		if CanStart(q, save, ctx) {
			out = append(out, q)
		}
	}
	return out
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
	return nil
}

// Abandon drops an in-progress quest, discarding its progress.
func Abandon(save *types.SaveFile, questID string) error {
	kept := make([]types.QuestProgress, 0, len(save.QuestsActive))
	found := false
	for _, qp := range save.QuestsActive {
		if qp.QuestID == questID {
			found = true
			continue
		}
		kept = append(kept, qp)
	}
	if !found {
		return fmt.Errorf("quest not active: %s", questID)
	}
	save.QuestsActive = kept
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

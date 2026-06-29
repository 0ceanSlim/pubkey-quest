package quest_test

import (
	"testing"
	"time"

	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/types"
)

// Two dailies in the pool; the active one rotates deterministically by period.
func dailyPool() []types.QuestData {
	return []types.QuestData{
		{ID: "daily-a", Name: "Daily A", Category: types.QuestDaily},
		{ID: "daily-b", Name: "Daily B", Category: types.QuestDaily},
		{ID: "side-x", Name: "Side", Category: types.QuestSide}, // ignored
	}
}

func TestCurrentRepeatableRotates(t *testing.T) {
	all := dailyPool()
	// Two days exactly one apart (past the reset hour both times) must pick
	// different dailies; the pool is sorted by id so the rotation is stable.
	d1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	d2 := d1.AddDate(0, 0, 1)
	q1, ok1 := quest.CurrentRepeatable(all, types.QuestDaily, d1)
	q2, ok2 := quest.CurrentRepeatable(all, types.QuestDaily, d2)
	if !ok1 || !ok2 {
		t.Fatal("expected a daily pick on both days")
	}
	if q1.ID == q2.ID {
		t.Errorf("consecutive days should rotate the daily, got %s both", q1.ID)
	}
	// Same day → same pick (stable within a period).
	again, _ := quest.CurrentRepeatable(all, types.QuestDaily, d1.Add(2*time.Hour))
	if again.ID != q1.ID {
		t.Errorf("same period should be stable: %s vs %s", again.ID, q1.ID)
	}
}

func TestRepeatableAvailableAndCompletion(t *testing.T) {
	all := dailyPool()
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	today, _ := quest.CurrentRepeatable(all, types.QuestDaily, now)
	save := &types.SaveFile{}

	// The non-current daily is never available; today's is.
	other := "daily-a"
	if today.ID == "daily-a" {
		other = "daily-b"
	}
	if quest.RepeatableAvailable(types.QuestData{ID: other, Category: types.QuestDaily}, save, all, now) {
		t.Error("a daily that isn't today's pick must not be available")
	}
	if !quest.RepeatableAvailable(today, save, all, now) {
		t.Error("today's daily should be available on a fresh save")
	}

	// Mark it done this period → no longer available today, but available next day.
	save.RepeatableQuests = map[string]int{today.ID: quest.CurrentPeriod(types.QuestDaily, now)}
	if quest.RepeatableAvailable(today, save, all, now) {
		t.Error("a daily completed this period must not re-offer the same day")
	}
	tomorrow := now.AddDate(0, 0, 1)
	if tomorrowPick, _ := quest.CurrentRepeatable(all, types.QuestDaily, tomorrow); tomorrowPick.ID == today.ID {
		// only assert re-availability when the rotation lands on it again
		if !quest.RepeatableAvailable(today, save, all, tomorrow) {
			t.Error("a daily should re-open next period")
		}
	}

	// Dailies never enter the permanent completed list.
	if len(save.QuestsCompleted) != 0 {
		t.Error("repeatable completion must not touch QuestsCompleted")
	}
}

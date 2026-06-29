package quest

import (
	"sort"
	"time"

	"pubkey-quest/types"
)

// Repeatable (daily/weekly) quests. Unlike one-shot quests, these cycle: one is
// picked from the category pool per real-world period, offered by innkeepers,
// and on completion the period is recorded (not the permanent completed list) so
// it re-opens next reset. Selection is a deterministic rotation through the pool
// so it's stable within a period and cycles predictably across periods.

// DailyResetHour is the server-local hour at which the daily rolls over (and the
// weekly, on its week boundary). Configurable.
const DailyResetHour = 6

// dailyPeriod is a per-day index that increments at DailyResetHour local time.
func dailyPeriod(now time.Time) int {
	s := now.Add(-time.Duration(DailyResetHour) * time.Hour)
	y, m, d := s.Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, s.Location()).Unix() / 86400)
}

// weeklyPeriod is a per-calendar-week index (ISO year+week), rolling at the reset
// hour on the week's first day.
func weeklyPeriod(now time.Time) int {
	s := now.Add(-time.Duration(DailyResetHour) * time.Hour)
	y, w := s.ISOWeek()
	return y*100 + w
}

// IsRepeatable reports whether a category cycles (daily/weekly).
func IsRepeatable(cat types.QuestCategory) bool {
	return cat == types.QuestDaily || cat == types.QuestWeekly
}

// CurrentPeriod returns the active period index for a repeatable category.
func CurrentPeriod(cat types.QuestCategory, now time.Time) int {
	if cat == types.QuestWeekly {
		return weeklyPeriod(now)
	}
	return dailyPeriod(now)
}

// CurrentRepeatable returns the one quest active this period for the given
// repeatable category — a deterministic rotation through the (id-sorted) pool.
func CurrentRepeatable(all []types.QuestData, cat types.QuestCategory, now time.Time) (types.QuestData, bool) {
	var pool []types.QuestData
	for _, q := range all {
		if q.Category == cat {
			pool = append(pool, q)
		}
	}
	if len(pool) == 0 {
		return types.QuestData{}, false
	}
	sort.Slice(pool, func(i, j int) bool { return pool[i].ID < pool[j].ID })
	idx := ((CurrentPeriod(cat, now) % len(pool)) + len(pool)) % len(pool)
	return pool[idx], true
}

// RepeatableAvailable reports whether a repeatable quest can be picked up right
// now: it's this period's pick for its category, isn't already active, and hasn't
// been completed this period.
func RepeatableAvailable(q types.QuestData, save *types.SaveFile, all []types.QuestData, now time.Time) bool {
	if !IsRepeatable(q.Category) {
		return false
	}
	cur, ok := CurrentRepeatable(all, q.Category, now)
	if !ok || cur.ID != q.ID {
		return false
	}
	if IsActive(save, q.ID) {
		return false
	}
	return save.RepeatableQuests[q.ID] != CurrentPeriod(q.Category, now)
}

// markRepeatableDone records that a repeatable quest was completed this period,
// so it won't re-offer until the next reset.
func markRepeatableDone(save *types.SaveFile, q *types.QuestData, now time.Time) {
	if save.RepeatableQuests == nil {
		save.RepeatableQuests = map[string]int{}
	}
	save.RepeatableQuests[q.ID] = CurrentPeriod(q.Category, now)
}

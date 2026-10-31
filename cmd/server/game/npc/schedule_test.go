package npc

import (
	"testing"

	"pubkey-quest/types"
)

// A day's own entries win over the everyday ones; an entry that runs past
// midnight still belongs to the day it started.
func TestScheduleDays(t *testing.T) {
	sched := []types.NPCScheduleSlot{
		{Start: 0, End: 420, Location: "home", State: "sleeping"},
		{Start: 420, End: 1140, Location: "forge", State: "working"},
		{Start: 1140, End: 1440, Location: "home", State: "home"},
		{Start: 1200, End: 1380, Location: "tavern", State: "leisure", Days: []string{"thursday"}},
		{Start: 1380, End: 120, Location: "dance", State: "leisure", Days: []string{"saturday"}},
	}
	// Day 1 is Sunday, so day 5 is Thursday and day 7 Saturday.
	if types.Weekday(1) != "Sunday" || types.Weekday(5) != "Thursday" || types.Weekday(8) != "Sunday" {
		t.Fatalf("weekdays: %s %s %s", types.Weekday(1), types.Weekday(5), types.Weekday(8))
	}
	for _, tc := range []struct {
		day, at int
		want    string
	}{
		{1, 1260, "home"},   // Sunday evening: home
		{5, 1260, "tavern"}, // Thursday evening: the tavern
		{5, 1390, "home"},   // Thursday after the tavern
		{5, 600, "forge"},   // Thursday working hours are the everyday ones
		{7, 1400, "dance"},  // Saturday night, before midnight
		{8, 60, "dance"},    // …and after it, on the next day (Sunday)
		{7, 60, "home"},     // Saturday's own early morning is the everyday one
	} {
		if got := GetCurrentScheduleSlot(sched, tc.day, tc.at); got == nil || got.Location != tc.want {
			t.Errorf("day %d (%s) %02d:%02d: %v, want %s", tc.day, types.Weekday(tc.day), tc.at/60, tc.at%60, got, tc.want)
		}
	}
}

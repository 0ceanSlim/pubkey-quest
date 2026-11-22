package quest

import (
	"testing"
	"time"

	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
)

// A daily counts once a day for the player, however many of their characters
// turn it in; a weekly counts toward weeklies.
func TestRepeatablesCountOncePerPeriod(t *testing.T) {
	oldOn, oldDir := stats.Enabled, stats.Dir
	stats.Enabled, stats.Dir = true, t.TempDir()
	t.Cleanup(func() { stats.Enabled, stats.Dir = oldOn, oldDir })

	daily := &types.QuestData{ID: "daily-fish-order", Category: types.QuestDaily}
	weekly := &types.QuestData{ID: "weekly-hunt", Category: types.QuestWeekly}
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	for _, id := range []string{"save_1", "save_2"} {
		save := &types.SaveFile{InternalNpub: "npub_me", InternalID: id}
		p := stats.Open("npub_me", id, nil)
		markRepeatableDone(save, daily, now)
		markRepeatableDone(save, weekly, now)
		if _, err := stats.Commit("npub_me", id, p.Mark()); err != nil {
			t.Fatal(err)
		}
		stats.Close("npub_me", id)
	}
	r, _ := stats.Load("npub_me")
	if r.Counters[stats.Dailies] != 1 || r.Counters[stats.Weeklies] != 1 {
		t.Fatalf("dailies %d, weeklies %d; want 1 each", r.Counters[stats.Dailies], r.Counters[stats.Weeklies])
	}
}

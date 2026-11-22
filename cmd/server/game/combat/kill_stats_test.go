package combat

import (
	"testing"

	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
)

// A kill counts toward the player's monsters slain and toughest kill, once a
// save covers it.
func TestKillCountsTowardStats(t *testing.T) {
	oldOn, oldDir := stats.Enabled, stats.Dir
	stats.Enabled, stats.Dir = true, t.TempDir()
	t.Cleanup(func() { stats.Enabled, stats.Dir = oldOn, oldDir })

	save := &types.SaveFile{D: "Mara", InternalNpub: "npub_me", InternalID: "save_1"}
	p := stats.Open("npub_me", "save_1", nil)
	t.Cleanup(func() { stats.Close("npub_me", "save_1") })

	for _, xp := range []int{50, 200, 100} {
		d := goblinData()
		d.XP = xp
		m := types.MonsterInstance{TemplateID: d.ID, InstanceID: d.ID, Name: d.Name, Data: d}
		cs := boardWith([]types.MonsterInstance{m})
		killRewards(cs, &cs.Monsters[0], save, nil)
	}
	r, err := stats.Commit("npub_me", "save_1", p.Mark())
	if err != nil {
		t.Fatal(err)
	}
	if r.Counters[stats.Slain] != 3 {
		t.Errorf("monsters slain %d, want 3", r.Counters[stats.Slain])
	}
	if b := r.Bests[stats.ToughestKill]; b.Value != 200 || b.Label != "goblin" || b.Who != "Mara" {
		t.Errorf("toughest kill %+v, want the 200 XP goblin, by Mara", b)
	}
}

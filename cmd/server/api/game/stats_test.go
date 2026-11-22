package game

import (
	"testing"

	"pubkey-quest/cmd/server/game/fishing"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/stats"
)

// statsOn turns the stats store on for a test session.
func statsOn(t *testing.T, sess *session.GameSession) {
	t.Helper()
	oldOn, oldDir := stats.Enabled, stats.Dir
	stats.Enabled, stats.Dir = true, t.TempDir()
	sess.SaveData.InternalNpub, sess.SaveData.InternalID = sess.Npub, sess.SaveID
	sess.Stats = stats.Open(sess.Npub, sess.SaveID, nil)
	t.Cleanup(func() {
		stats.Close(sess.Npub, sess.SaveID)
		stats.Enabled, stats.Dir = oldOn, oldDir
	})
}

// A landed fish counts once the save is kept: landed, its species, and the
// biggest of that species in ounces, by the character who caught it.
func TestCatchCountsAtSave(t *testing.T) {
	sess := wharfSession(t)
	sess.SaveData.D = "Mara"
	statsOn(t, sess)

	addCatch(sess, fishing.Fish{Species: "river-pike", Size: "large", Lb: 19.375}, "", map[string]any{})
	addCatch(sess, fishing.Fish{Species: "river-pike", Size: "small", Lb: 2}, "", map[string]any{})
	if r, _ := stats.Load(sess.Npub); r.Counters[stats.FishLanded] != 0 {
		t.Fatal("a catch counted before any save")
	}
	commitStats(sess, sess.Stats.Mark(), &sess.SaveData)

	r, _ := stats.Load(sess.Npub)
	if r.Counters[stats.FishLanded] != 2 {
		t.Errorf("fish landed %d, want 2", r.Counters[stats.FishLanded])
	}
	if _, ok := r.Sets[stats.FishSpecies]["river-pike"]; !ok {
		t.Errorf("species caught: %v", r.Sets[stats.FishSpecies])
	}
	if b := r.Bests[stats.FishOunces+"river-pike"]; b.Value != 310 || b.Who != "Mara" {
		t.Errorf("biggest pike %+v, want 310 oz by Mara", b)
	}
}

// Gems and ores found out in the world count toward the collections; a set
// piece, or anything else, doesn't.
func TestNoteFound(t *testing.T) {
	sess := wharfSession(t)
	statsOn(t, sess)
	state := &sess.SaveData
	for _, id := range []string{"gem-ruby-small-uncut", "gem-black-pearl-tiny-fine", "iron-ore", "ring-ruby-small-fine", "longsword"} {
		noteFound(state, id)
	}
	commitStats(sess, sess.Stats.Mark(), &sess.SaveData)
	r, _ := stats.Load(sess.Npub)
	if g := r.Sets[stats.GemTypes]; len(g) != 2 || g["ruby"] == 0 || g["black-pearl"] == 0 {
		t.Errorf("gem types %v, want ruby and black-pearl", g)
	}
	if o := r.Sets[stats.OresMined]; len(o) != 1 || o["iron-ore"] == 0 {
		t.Errorf("ores %v, want iron-ore", o)
	}
}

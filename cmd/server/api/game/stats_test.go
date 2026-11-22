package game

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/game/boards"
	"pubkey-quest/cmd/server/game/fishing"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
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

// Net worth is the base value of everything owned: the pack and what's in its
// containers, the gear (a two-handed weapon in both hands once), the vault,
// and gold, which is an item.
func TestNetWorth(t *testing.T) {
	sess := wharfSession(t)
	state := &sess.SaveData
	state.Inventory = map[string]interface{}{
		"general_slots": []interface{}{
			map[string]interface{}{"item": "gold-piece", "quantity": 25.0},
			map[string]interface{}{"item": "backpack", "quantity": 1.0, "contents": []interface{}{
				map[string]interface{}{"item": "rations", "quantity": 2.0},
			}},
			map[string]interface{}{"item": "", "quantity": 0.0},
		},
		"gear_slots": map[string]interface{}{
			"mainhand": map[string]interface{}{"item": "greatsword", "quantity": 1.0},
			"offhand":  map[string]interface{}{"item": "greatsword", "quantity": 1.0},
		},
	}
	state.Vault = []types.VaultEntry{{ItemID: "longsword", Quantity: 2}}

	// 25 gold + backpack 200 + 2 rations 100 + greatsword 5000 once + 2 longswords 3000
	if got, want := gameutil.NetWorth(state), int64(25+200+100+5000+3000); got != want {
		t.Fatalf("net worth %d, want %d", got, want)
	}
}

// A kept save puts the character on the boards: their XP, net worth, quest
// points and places found, and the player's lifetime records.
func TestSaveStandsOnTheBoards(t *testing.T) {
	sess := wharfSession(t)
	sess.SaveData.D, sess.SaveData.Class, sess.SaveData.Experience = "Mara", "Druid", 300
	sess.SaveData.LocationsDiscovered = []string{"goldenhaven", "kingdom", "not-a-place"}
	statsOn(t, sess)
	addCatch(sess, fishing.Fish{Species: "carp", Size: "small", Lb: 3}, "", map[string]any{})
	commitStats(sess, sess.Stats.Mark(), &sess.SaveData)

	r, _ := stats.Load(sess.Npub)
	xp := r.Scores[boards.XP]
	if xp.Value != int64(sess.SaveData.Experience) || xp.Value < 300 || xp.Who != "Mara" || !strings.HasSuffix(xp.Detail, "Druid") {
		t.Errorf("xp score %+v", xp)
	}
	if r.Scores[boards.Places].Value != 2 {
		t.Errorf("places %d, want 2 (an unknown id doesn't count)", r.Scores[boards.Places].Value)
	}
	if r.Scores[boards.FishLanded].Value != 1 || r.Scores[boards.Biggest+"carp"].Value != 48 {
		t.Errorf("fish boards: landed %+v, biggest carp %+v", r.Scores[boards.FishLanded], r.Scores[boards.Biggest+"carp"])
	}
	if r.Scores[boards.NetWorth].Value <= 0 {
		t.Errorf("net worth %+v", r.Scores[boards.NetWorth])
	}

	DropFromBoards(sess.Npub, sess.SaveID)
	r, _ = stats.Load(sess.Npub)
	if _, ok := r.Scores[boards.XP]; ok {
		t.Error("a deleted character still stands on the xp board")
	}
	if r.Scores[boards.FishLanded].Value != 1 {
		t.Error("deleting a character took the player's lifetime record with it")
	}
}

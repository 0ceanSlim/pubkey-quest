package badges

import (
	"reflect"
	"testing"

	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
)

var testDefs = []types.BadgeDef{
	{ID: "quest-rats", Rule: types.BadgeRule{Quest: "rats-of-goldenhaven"}},
	{ID: "quest-letter", Rule: types.BadgeRule{AnyQuest: []string{"letter-a", "letter-b"}}},
	{ID: "every-fish", Rule: types.BadgeRule{Collection: types.CollectionFish}},
	{ID: "every-town", Rule: types.BadgeRule{Collection: types.CollectionTowns}},
	{ID: "level-5", Rule: types.BadgeRule{Level: 5}},
	{ID: "level-10", Rule: types.BadgeRule{Level: 10}},
	{ID: "dailies-10", Rule: types.BadgeRule{Counter: stats.Dailies, At: 10}},
	{ID: "every-song", Rule: types.BadgeRule{Collection: types.CollectionSongs}}, // an empty catalog: never
}

var testCatalog = map[string][]string{
	types.CollectionFish:  {"carp", "river-pike"},
	types.CollectionTowns: {"kingdom", "millhaven"},
}

func TestJudge(t *testing.T) {
	r := &stats.Record{
		Sets:     map[string]map[string]int64{stats.FishSpecies: {"carp": 1}},
		Counters: map[string]int64{stats.Dailies: 9},
	}
	save := &types.SaveFile{
		QuestsCompleted:     []string{"rats-of-goldenhaven", "letter-b"},
		LocationsDiscovered: []string{"kingdom", "millhaven", "darkwood-forest"},
	}

	got := Judge(testDefs, testCatalog, r, save, 7, 100)
	if want := []string{"quest-rats", "quest-letter", "every-town", "level-5"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("first save earned %v, want %v", got, want)
	}

	// Another character lands the pike: the lifetime set is complete. The
	// tenth daily lands too. Nothing is awarded twice.
	r.Sets[stats.FishSpecies]["river-pike"] = 2
	r.Counters[stats.Dailies] = 10
	got = Judge(testDefs, testCatalog, r, &types.SaveFile{}, 1, 200)
	if want := []string{"every-fish", "dailies-10"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("second save earned %v, want %v", got, want)
	}
	if r.Badges["quest-rats"] != 100 || r.Badges["every-fish"] != 200 {
		t.Errorf("award times: %v", r.Badges)
	}
	if len(Judge(testDefs, testCatalog, r, save, 7, 300)) != 0 {
		t.Error("a badge was awarded twice")
	}
}

// A kept save is judged even with no stats pending: a quest finished or a
// level reached is enough.
func TestCommitJudgesEverySave(t *testing.T) {
	oldOn, oldDir := stats.Enabled, stats.Dir
	stats.Enabled, stats.Dir = true, t.TempDir()
	t.Cleanup(func() { stats.Enabled, stats.Dir = oldOn, oldDir })
	stats.Open("npub_me", "save_1", nil)
	t.Cleanup(func() { stats.Close("npub_me", "save_1") })

	save := &types.SaveFile{QuestsCompleted: []string{"rats-of-goldenhaven"}}
	judge := func(r *stats.Record, now int64) bool {
		return len(Judge(testDefs, testCatalog, r, save, 5, now)) > 0
	}
	if _, err := stats.Commit("npub_me", "save_1", 0, judge); err != nil {
		t.Fatal(err)
	}
	r, _ := stats.Load("npub_me")
	if _, ok := r.Badges["quest-rats"]; !ok {
		t.Fatalf("badges after the save: %v", r.Badges)
	}
	if _, ok := r.Badges["level-5"]; !ok {
		t.Fatalf("level-5 missing: %v", r.Badges)
	}
}

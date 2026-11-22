package boards

import (
	"testing"

	"pubkey-quest/cmd/server/stats"
)

func stand(xp int64, who string) map[string]stats.Entry {
	return map[string]stats.Entry{XP: {Value: xp, Who: who}}
}

// The best character stands for the player; deleting them drops the player
// to the next best.
func TestBestCharacterStands(t *testing.T) {
	r := &stats.Record{}
	Stand(r, "save_1", stand(500, "Mara"), 10)
	Stand(r, "save_2", stand(900, "Bram"), 20)
	if s := r.Scores[XP]; s.Value != 900 || s.Who != "Bram" || s.At != 20 {
		t.Fatalf("xp %+v, want Bram's 900", s)
	}
	Drop(r, "save_2", 30)
	if s := r.Scores[XP]; s.Value != 500 || s.Who != "Mara" || s.At != 30 {
		t.Fatalf("after Bram's deleted: %+v, want Mara's 500", s)
	}
	Drop(r, "save_1", 40)
	if _, ok := r.Scores[XP]; ok {
		t.Fatal("no characters left, but still on the xp board")
	}
}

// A score keeps its time until it changes (an unchanged score isn't signed
// again).
func TestRescoreOnlyOnChange(t *testing.T) {
	r := &stats.Record{}
	Stand(r, "save_1", stand(500, "Mara"), 10)
	if Stand(r, "save_1", stand(500, "Mara"), 20) {
		t.Error("an unchanged standing reported a change")
	}
	if r.Scores[XP].At != 10 {
		t.Errorf("an unchanged score's time moved to %d", r.Scores[XP].At)
	}
	r.Scores[XP] = stats.Score{Entry: r.Scores[XP].Entry, At: 10, Event: "signed"}
	Stand(r, "save_1", stand(650, "Mara"), 30)
	if s := r.Scores[XP]; s.Value != 650 || s.Event != "" {
		t.Errorf("a new score %+v should wait to be signed again", s)
	}
}

// The record boards come from the lifetime record.
func TestRecordBoards(t *testing.T) {
	r := &stats.Record{
		Counters: map[string]int64{stats.Slain: 12, stats.FishLanded: 4},
		Bests: map[string]stats.BestEntry{
			stats.ToughestKill:              {Value: 1800, Label: "troll", Who: "Bram"},
			stats.FishOunces + "river-pike": {Value: 310, Label: "river-pike", Who: "Mara"},
		},
		Badges: map[string]int64{"level-5": 1, "every-fish": 2},
	}
	Rescore(r, 5)
	want := map[string]stats.Entry{
		Slain:                  {Value: 12},
		FishLanded:             {Value: 4},
		Badges:                 {Value: 2},
		Toughest:               {Value: 1800, Who: "Bram", Detail: "troll"},
		Biggest + "river-pike": {Value: 310, Who: "Mara", Detail: "river-pike"},
	}
	for b, e := range want {
		if r.Scores[b].Entry != e {
			t.Errorf("%s: %+v, want %+v", b, r.Scores[b].Entry, e)
		}
	}
}

package game

import (
	"testing"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/fishing"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/session"
)

// Fishing Goldenhaven's wharf through the real actions.

func wharfSession(t *testing.T, items ...string) *session.GameSession {
	t.Helper()
	sess := bobSession(t)
	sess.SaveData.Building, sess.SaveData.TimeOfDay = "", 720 // outside, at noon
	slots := []interface{}{}
	for i := 0; i < 6; i++ {
		slots = append(slots, map[string]interface{}{"item": "", "quantity": 0})
	}
	for i, id := range items {
		slots[i] = map[string]interface{}{"item": id, "quantity": 5}
	}
	sess.SaveData.Inventory = map[string]interface{}{"general_slots": slots, "gear_slots": map[string]interface{}{}}
	return sess
}

func fishAction(t *testing.T, sess *session.GameSession, typ string, params map[string]any) *GameActionResponse {
	t.Helper()
	resp, err := processGameAction(sess, GameAction{Type: typ, Params: params})
	if err != nil {
		t.Fatalf("%s: %v", typ, err)
	}
	return resp
}

func TestTheWharfIsAFishingSpot(t *testing.T) {
	sess := wharfSession(t)
	if got := townFishingSpot(&sess.SaveData); got != "goldenhaven-wharf" {
		t.Fatalf("spot in Goldenhaven's east district: %q", got)
	}
	sess.SaveData.Building = "sailors_tavern"
	if townFishingSpot(&sess.SaveData) != "" {
		t.Error("fishing from inside the tavern")
	}
}

func TestNoRodNoFishing(t *testing.T) {
	sess := wharfSession(t, "worms")
	if r := fishAction(t, sess, "fish_begin", map[string]any{"kind": "town", "bait": "worms"}); r.Success {
		t.Fatal("began fishing without a rod")
	}
	if r := fishAction(t, sess, "fish_begin", map[string]any{"kind": "town", "barehand": true}); r.Success {
		t.Fatal("fished barehanded without learning how")
	}
}

// Fish until something lands: the fish goes in the pack with its exact weight,
// and the spot's count for the day goes up.
func TestCastAndLand(t *testing.T) {
	sess := wharfSession(t, "fishing-rod", "worms")
	xp0 := sess.SaveData.Experience
	r := fishAction(t, sess, "fish_begin", map[string]any{"kind": "town", "bait": "worms"})
	if !r.Success || sess.FishSession == nil {
		t.Fatalf("fish_begin: %s", r.Message)
	}
	for range 400 {
		fs := sess.FishSession
		if fs == nil {
			t.Fatal("the session closed")
		}
		act := "wait"
		switch fs.Phase {
		case fishing.PhaseWatch:
			if fs.Float == fishing.FloatDive {
				act = "strike"
			}
		case fishing.PhaseFight:
			act = "reel"
			if fs.Tension >= 6 {
				act = "give"
			}
		case fishing.PhaseDone:
			if fs.View()["catch"] != nil {
				goto landed
			}
			act = "recast"
		}
		fishAction(t, sess, "fish_act", map[string]any{"session_id": fs.ID, "version": float64(fs.Version), "act": act})
	}
	t.Fatal("nothing landed in 400 moves")
landed:
	catch := sess.FishSession.View()["catch"].(map[string]any)
	id := catch["item_id"].(string)
	if gameutil.CountItem(&sess.SaveData, id) != 1 {
		t.Fatalf("%s isn't in the pack", id)
	}
	for _, raw := range sess.SaveData.Inventory["general_slots"].([]interface{}) {
		if s, _ := raw.(map[string]interface{}); s["item"] == id {
			if lb, _ := s[condition.KeyLb].(float64); lb <= 0 {
				t.Errorf("no weight on the catch: %v", s)
			}
		}
	}
	if sess.SaveData.Experience <= xp0 {
		t.Error("landing a fish gave no XP")
	}
	// Anything else walks away from the water.
	fishAction(t, sess, "wait", map[string]any{"minutes": float64(5)})
	if sess.FishSession != nil {
		t.Error("the fishing session survived another action")
	}
}

// A catch pays a little XP by size, and its exertion adds up to fatigue.
func TestCatchToll(t *testing.T) {
	sess := wharfSession(t)
	sess.SaveData.Fatigue = 0
	xp0 := sess.SaveData.Experience
	catchToll(sess, "huge", nil) // exertion 4 of 6
	if sess.SaveData.Fatigue != 0 {
		t.Fatalf("fatigue after one huge fish = %d, want 0", sess.SaveData.Fatigue)
	}
	catchToll(sess, "huge", nil) // 8: one fatigue, 2 carried
	if sess.SaveData.Fatigue != 1 || sess.SaveData.Lim().Exertion != 2 {
		t.Fatalf("fatigue %d, exertion %d; want 1 and 2", sess.SaveData.Fatigue, sess.SaveData.Lim().Exertion)
	}
	if gained := sess.SaveData.Experience - xp0; gained < 40 {
		t.Errorf("two huge fish gave %d XP, want at least 40", gained)
	}
}

func TestCutBait(t *testing.T) {
	sess := wharfSession(t, "fish-minnow-small", "fish-cod-large")
	if r := fishAction(t, sess, "cut_bait", map[string]any{"item_id": "fish-cod-large"}); r.Success {
		t.Error("cut a large cod into bait")
	}
	if r := fishAction(t, sess, "cut_bait", map[string]any{"item_id": "fish-minnow-small"}); !r.Success || gameutil.CountItem(&sess.SaveData, "cut-bait") != 2 {
		t.Errorf("cut bait: %s", r.Message)
	}
}

func TestCatchObjectiveMatches(t *testing.T) {
	wharfSession(t) // loads the database
	for _, c := range []struct {
		target, min, item string
		want              bool
	}{
		{"any", "", "fish-minnow-tiny", true},
		{"water-sea", "large", "fish-cod-huge", true},
		{"water-sea", "large", "fish-cod-small", false},
		{"water-river", "", "fish-cod-medium", false},
		{"leaping-salmon", "", "fish-leaping-salmon-small", true},
		{"any", "", "worms", false},
	} {
		if got := fishMatches(c.target, c.min, c.item); got != c.want {
			t.Errorf("catch %s (min %q) with %s: %v, want %v", c.target, c.min, c.item, got, c.want)
		}
	}
}

// Finn mounts a huge fish for a quarter of its value; it becomes a trophy.
func TestFinnMountsATrophy(t *testing.T) {
	sess := wharfSession(t)
	sess.SaveData.Location, sess.SaveData.District, sess.SaveData.Building = "saltwind", "center", "net_and_tackle"
	slots := sess.SaveData.Inventory["general_slots"].([]interface{})
	slots[0] = map[string]interface{}{"item": "fish-cod-huge", "quantity": 1}
	slots[1] = map[string]interface{}{"item": "gold-piece", "quantity": 500}
	resp := handleMountChoice(sess, trophyMounter, mountChoice)
	if resp == nil || !resp.Success {
		t.Fatalf("mount: %+v", resp)
	}
	if gameutil.CountItem(&sess.SaveData, "trophy-cod") != 1 || gameutil.CountItem(&sess.SaveData, "fish-cod-huge") != 0 {
		t.Error("the cod should be a trophy now")
	}
	if gold := gameutil.CountItem(&sess.SaveData, "gold-piece"); gold != 500-mountFee("fish-cod-huge") {
		t.Errorf("gold left %d", gold)
	}
}

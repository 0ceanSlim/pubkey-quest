package game

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/session"
)

// forgeSession is a player at the Anvilheart at noon, carrying slots of items.
func forgeSession(t *testing.T, slots ...map[string]interface{}) *session.GameSession {
	t.Helper()
	sess := wharfSession(t)
	state := &sess.SaveData
	state.Location, state.District, state.Building, state.TimeOfDay = "ironpeak", "west", "anvilheart_forge", 720
	general := state.Inventory["general_slots"].([]interface{})
	for i, s := range slots {
		general[i] = s
	}
	return sess
}

func carrying(id string, n int) map[string]interface{} {
	return map[string]interface{}{"item": id, "quantity": n}
}

// commission talks to the Forgemistress, asks about picks (when she offers
// it), then makes the choice, as a player would: only offered choices count.
func commission(t *testing.T, sess *session.GameSession, choice string) *GameActionResponse {
	t.Helper()
	fishAction(t, sess, "talk_to_npc", map[string]any{"npc_id": "forgemistress"})
	if strings.HasPrefix(choice, optKeyPrefix+"ask_about_picks") && sess.DialogueOffered["ask_about_picks"] {
		fishAction(t, sess, "npc_dialogue_choice", map[string]any{"npc_id": "forgemistress", "choice": "ask_about_picks"})
	}
	return fishAction(t, sess, "npc_dialogue_choice", map[string]any{"npc_id": "forgemistress", "choice": choice})
}

// The Forgemistress forges a dwarven pick from ore and gold, and nothing else
// gets you one: not asking for the finished pick directly, not coming up
// short, not after she's gone home.
func TestCommissionADwarvenPick(t *testing.T) {
	sess := forgeSession(t, carrying("iron-ore", 12), carrying("copper-ore", 5), carrying("gold-piece", 500))
	state := &sess.SaveData

	if r := commission(t, sess, "dwarven_pick_made"); r.Success || gameutil.CountItem(state, "dwarven-pick") != 0 {
		t.Fatalf("asked for the pick without paying: %+v", r)
	}

	r := commission(t, sess, optKey("ask_about_picks", 0))
	if !r.Success || !strings.Contains(r.Message, "Dwarven Pick") {
		t.Fatalf("commission: %+v", r)
	}
	if gameutil.CountItem(state, "dwarven-pick") != 1 || gameutil.CountItem(state, "iron-ore") != 2 ||
		gameutil.CountItem(state, "copper-ore") != 0 || gameutil.GetGoldQuantity(state) != 100 {
		t.Fatalf("after the commission: pick %d, iron %d, copper %d, gold %d",
			gameutil.CountItem(state, "dwarven-pick"), gameutil.CountItem(state, "iron-ore"),
			gameutil.CountItem(state, "copper-ore"), gameutil.GetGoldQuantity(state))
	}

	// Short of copper now: no second pick.
	if r := commission(t, sess, optKey("ask_about_picks", 0)); r.Success || gameutil.CountItem(state, "dwarven-pick") != 1 {
		t.Fatalf("a second pick without the copper: %+v", r)
	}

	// After hours she won't, even with everything she asks for.
	sess = forgeSession(t, carrying("iron-ore", 10), carrying("copper-ore", 5), carrying("gold-piece", 500))
	sess.SaveData.TimeOfDay = 1200
	if r := commission(t, sess, optKey("ask_about_picks", 0)); r.Success || gameutil.CountItem(&sess.SaveData, "dwarven-pick") != 0 ||
		gameutil.GetGoldQuantity(&sess.SaveData) != 500 {
		t.Fatalf("a commission after hours: %+v", r)
	}
}

// The deepiron pick takes the dwarven pick in trade.
func TestCommissionADeepironPick(t *testing.T) {
	sess := forgeSession(t, carrying("deepiron-ore", 8), carrying("dwarven-pick", 1), carrying("gold-piece", 2000))
	state := &sess.SaveData
	if r := commission(t, sess, optKey("ask_about_picks", 1)); !r.Success {
		t.Fatalf("commission: %+v", r)
	}
	if gameutil.CountItem(state, "deepiron-pick") != 1 || gameutil.CountItem(state, "dwarven-pick") != 0 ||
		gameutil.CountItem(state, "deepiron-ore") != 0 || gameutil.GetGoldQuantity(state) != 0 {
		t.Fatal("the deepiron pick should replace the dwarven one and take the ore and gold")
	}
}

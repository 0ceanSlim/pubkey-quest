package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// The rats in the Salty Anchor's cellar (rats-of-goldenhaven stage 0): standing
// in the cellar on that stage starts a fight, in waves of three, until the five
// kills are in; fleeing doesn't loop the fight until you leave and come back.

func cellarSession(t *testing.T) *session.GameSession {
	t.Helper()
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
	save := types.SaveFile{
		Race: "Human", Class: "Fighter", HP: 30, MaxHP: 30,
		Stats: map[string]interface{}{
			"strength": 14, "dexterity": 12, "constitution": 14,
			"intelligence": 10, "wisdom": 10, "charisma": 10,
		},
		Inventory: map[string]interface{}{},
		Location:  "goldenhaven", District: "east", Building: "sailors_tavern", Room: "cellar",
		QuestsActive: []types.QuestProgress{{QuestID: "rats-of-goldenhaven", Stage: 0}},
	}
	return &session.GameSession{Npub: "npub_test", SaveID: "s1", SaveData: save, PlaceKey: "cellar"}
}

func ratsIn(cs *types.CombatSession) int {
	n := 0
	for _, m := range cs.Monsters {
		if m.TemplateID == "giant-rat" {
			n++
		}
	}
	return n
}

func TestQuestEncounterWaves(t *testing.T) {
	sess := cellarSession(t)
	resp := &types.GameActionResponse{}

	maybeStartQuestEncounter(sess, resp)
	if sess.ActiveCombat == nil {
		t.Fatal("standing in the cellar on stage 0 should start the rat fight")
	}
	if got := ratsIn(sess.ActiveCombat); got != 1 {
		t.Errorf("first wave: %d rats, want 1 (one rat at a time)", got)
	}
	if resp.Data["combat_started"] != true {
		t.Error("response should hand the client the fight")
	}

	// Won the first wave: 1 of 5 killed. The next rat waits out a breather...
	sess.ActiveCombat = nil
	sess.SaveData.QuestsActive[0].ObjectiveCounts = []int{1}
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat != nil {
		t.Fatal("the next wave must not chain straight onto the last one")
	}
	sess.SaveData.TimeOfDay += questWaveBreatherMinutes - 1
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat != nil {
		t.Fatal("the next wave arrived before the breather was up")
	}
	// ...then comes.
	sess.SaveData.TimeOfDay++
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat == nil || ratsIn(sess.ActiveCombat) != 1 {
		t.Fatalf("the next wave should be one more rat, got %v", sess.ActiveCombat)
	}

	// Fled without a kill: staying put doesn't restart it...
	sess.ActiveCombat = nil
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat != nil {
		t.Error("after fleeing, the fight must not restart while you stay in the room")
	}
	// ...but stepping out and back in does.
	sess.PlaceKey = "taproom"
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	sess.PlaceKey = "cellar"
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat == nil {
		t.Error("coming back into the cellar should retry the fight")
	}

	// All five dead: the cellar is quiet.
	sess.ActiveCombat = nil
	sess.SaveData.QuestsActive[0].ObjectiveCounts = []int{5}
	sess.PlaceKey = "elsewhere"
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	sess.PlaceKey = "cellar"
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat != nil {
		t.Error("no fight once the stage's kills are done")
	}
}

func TestQuestEncounterOnlyInPlace(t *testing.T) {
	sess := cellarSession(t)
	sess.SaveData.Room = "taproom"
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat != nil {
		t.Error("the rats are in the cellar, not the taproom")
	}
}

// The Cragspire Survey's wolves wait on the road (an environment): they must
// not jump you the moment you load in or stand still, only while you travel.
func TestRoadQuestEncounterOnlyWhileTravelling(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
	newSess := func(stopped bool) *session.GameSession {
		save := types.SaveFile{
			Race: "Human", Class: "Fighter", HP: 30, MaxHP: 30,
			Stats: map[string]interface{}{
				"strength": 14, "dexterity": 12, "constitution": 14,
				"intelligence": 10, "wisdom": 10, "charisma": 10,
			},
			Inventory:     map[string]interface{}{},
			Location:      "cragspire-mountains",
			TravelStopped: stopped,
			QuestsActive:  []types.QuestProgress{{QuestID: "daily-mountain-survey", Stage: 2}},
		}
		return &session.GameSession{Npub: "npub_road", SaveID: "s1", SaveData: save, PlaceKey: "road"}
	}
	defer func(r func() float64) { questRoll = r }(questRoll)
	questRoll = func() float64 { return 0 } // every roll hits

	sess := newSess(false)
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat != nil {
		t.Fatal("a road quest fight must not start on load / on an ordinary action")
	}

	stopped := newSess(true)
	maybeStartRoadQuestEncounter(stopped, 5, &types.GameActionResponse{})
	if stopped.ActiveCombat != nil {
		t.Fatal("a road quest fight must not find you while you're stopped")
	}

	maybeStartRoadQuestEncounter(sess, 5, &types.GameActionResponse{})
	if sess.ActiveCombat == nil {
		t.Fatal("travelling the road on that stage should bring the wolves")
	}
	wolves := 0
	for _, m := range sess.ActiveCombat.Monsters {
		if m.TemplateID == "wolf" {
			wolves++
		}
	}
	if wolves != 2 {
		t.Errorf("got %d wolves, want 2", wolves)
	}

	questRoll = func() float64 { return 0.99 } // the roll misses
	missed := newSess(false)
	maybeStartRoadQuestEncounter(missed, 5, &types.GameActionResponse{})
	if missed.ActiveCombat != nil {
		t.Fatal("a missed roll means no fight this tick")
	}
}

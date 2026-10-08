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
	if got := ratsIn(sess.ActiveCombat); got != 3 {
		t.Errorf("first wave: %d rats, want 3 (count cap)", got)
	}
	if resp.Data["combat_started"] != true {
		t.Error("response should hand the client the fight")
	}

	// Won the first wave: 3 of 5 killed. The next action brings the last two.
	sess.ActiveCombat = nil
	sess.SaveData.QuestsActive[0].ObjectiveCounts = []int{3}
	maybeStartQuestEncounter(sess, &types.GameActionResponse{})
	if sess.ActiveCombat == nil || ratsIn(sess.ActiveCombat) != 2 {
		t.Fatalf("second wave should hold the 2 remaining rats, got %v", sess.ActiveCombat)
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

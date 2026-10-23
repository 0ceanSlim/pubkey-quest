package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

func TestQuestNews(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
	save := &types.SaveFile{
		QuestsActive:     []types.QuestProgress{{QuestID: "daily-monster-bounty", Stage: 1}, {QuestID: "the-missing-mule", Stage: 1}, {QuestID: "abandoned", Stage: 0}},
		RepeatableQuests: map[string]int{},
	}
	before := markQuests(save)

	// Claiming the bounty finishes it (a repeatable records its period), the
	// mule quest moves to its next stage, and one quest is simply dropped.
	save.QuestsActive = []types.QuestProgress{{QuestID: "the-missing-mule", Stage: 2}}
	save.RepeatableQuests["daily-monster-bounty"] = 20381

	resp := &GameActionResponse{}
	questNews(before, save, resp)

	done, _ := resp.Data["quests_completed"].([]map[string]any)
	if len(done) != 1 || done[0]["id"] != "daily-monster-bounty" {
		t.Fatalf("quests_completed = %v, want the bounty only (an abandoned quest isn't completed)", resp.Data["quests_completed"])
	}
	moved, _ := resp.Data["quests_advanced"].([]map[string]any)
	if len(moved) != 1 || moved[0]["id"] != "the-missing-mule" {
		t.Fatalf("quests_advanced = %v", resp.Data["quests_advanced"])
	}

	// Nothing changed: nothing reported.
	quiet := &GameActionResponse{}
	questNews(markQuests(save), save, quiet)
	if quiet.Data != nil {
		t.Fatalf("no change should add nothing, got %v", quiet.Data)
	}
}

package api_test

import (
	"fmt"
	"strings"
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/types"
)

// A quest's last "talk to them" step finishes only when the player chooses
// Turn in, and the response carries the rewards for the rewards screen.
func TestTurnInFinishesTheTutorialWithItsRewards(t *testing.T) {
	ts := setupTutorialServer(t)
	defer ts.Close()
	defer db.Close()

	saveID := newCharacter(t, ts, tutorialNpubA)
	save := liveSave(t, tutorialNpubA, saveID)
	questID, keeper, vault := keeperAndVault(t, save)
	q, _ := db.GetQuestByID(questID)

	// Skip ahead to the report-back step.
	last := len(q.Stages) - 1
	for i := range save.QuestsActive {
		if save.QuestsActive[i].QuestID == questID {
			save.QuestsActive[i] = types.QuestProgress{QuestID: questID, Stage: last, ObjectiveCounts: make([]int, len(q.Stages[last].Objectives))}
		}
	}

	if r := act(t, ts, tutorialNpubA, saveID, "enter_building", map[string]any{"building_id": vault}); r["success"] != true {
		t.Fatalf("enter %s: %v", vault, r)
	}
	talk := act(t, ts, tutorialNpubA, saveID, "talk_to_npc", map[string]any{"npc_id": keeper})
	if quest.IsCompleted(liveSave(t, tutorialNpubA, saveID), questID) {
		t.Fatal("opening the conversation mustn't finish the quest")
	}
	key := "quest:" + questID + ":turnin:@turnin"
	if !strings.Contains(fmt.Sprint(talk), key) {
		t.Fatalf("the keeper should offer the turn-in: %v", talk)
	}

	done := act(t, ts, tutorialNpubA, saveID, "npc_dialogue_choice", map[string]any{"npc_id": keeper, "choice": key})
	if !quest.IsCompleted(liveSave(t, tutorialNpubA, saveID), questID) {
		t.Fatalf("turning in should finish the quest: %v", done)
	}
	data, _ := done["data"].(map[string]any)
	list, _ := data["quests_completed"].([]any)
	if len(list) != 1 {
		t.Fatalf("quests_completed: %v", data["quests_completed"])
	}
	rewards, _ := list[0].(map[string]any)["rewards"].(map[string]any)
	if rewards["xp"] != float64(150) || rewards["gold"] != float64(100) {
		t.Errorf("rewards for the screen: %v", rewards)
	}

	// A forged turn-in for a quest that isn't ready does nothing.
	again := act(t, ts, tutorialNpubA, saveID, "npc_dialogue_choice", map[string]any{"npc_id": keeper, "choice": key})
	if d, _ := again["data"].(map[string]any); d["quests_completed"] != nil {
		t.Errorf("a second turn-in paid out again: %v", again)
	}
}

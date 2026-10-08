package game

import (
	"strings"
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// Taking up The Rats of Goldenhaven by talking to Bob at the Salty Anchor, through
// the same handlers the client's talk / choice actions use.

func bobSession(t *testing.T) *session.GameSession {
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
		Location:  "goldenhaven", District: "east", Building: "sailors_tavern",
		TimeOfDay: 1100, // the Anchor opens at 17:00
	}
	return &session.GameSession{Npub: "npub_test", SaveID: "s1", SaveData: save}
}

func dialogueOf(t *testing.T, resp *GameActionResponse) map[string]interface{} {
	t.Helper()
	if resp == nil || resp.Delta == nil {
		t.Fatalf("no dialogue in response: %+v", resp)
	}
	dlg, ok := resp.Delta["npc_dialogue"].(map[string]interface{})
	if !ok {
		t.Fatalf("no npc_dialogue in delta: %+v", resp.Delta)
	}
	return dlg
}

func questOption(dlg map[string]interface{}, questID string) string {
	opts, _ := dlg["options"].([]string)
	for _, o := range opts {
		if strings.HasPrefix(o, questKeyPrefix+questID+":") {
			return o
		}
	}
	return ""
}

func choose(t *testing.T, sess *session.GameSession, choice string) *GameActionResponse {
	t.Helper()
	resp, err := handleNPCDialogueChoiceAction(sess, map[string]any{"npc_id": "tavern-owner-bob", "choice": choice})
	if err != nil {
		t.Fatalf("choice %q: %v", choice, err)
	}
	return resp
}

func TestTakeUpQuestInConversation(t *testing.T) {
	sess := bobSession(t)

	talk, err := handleTalkToNPCAction(&sess.SaveData, map[string]any{"npc_id": "tavern-owner-bob"})
	if err != nil || talk == nil || !talk.Success {
		t.Fatalf("talk to Bob: %v %+v", err, talk)
	}
	menu := dialogueOf(t, talk)
	offer := questOption(menu, "rats-of-goldenhaven")
	if offer == "" {
		t.Fatalf("Bob's menu should offer the rats quest, got %v", menu["options"])
	}
	if labels, _ := menu["option_labels"].(map[string]string); labels[offer] == "" {
		t.Error("the quest option needs a label for its button")
	}
	opts, _ := menu["options"].([]string)
	if last := opts[len(opts)-1]; last != "goodbye" {
		t.Errorf("goodbye should stay the last option, got %q", last)
	}

	// Hear him out, then walk the thread to an accept node.
	node := dialogueOf(t, choose(t, sess, offer))
	var accept string
	for _, o := range node["options"].([]string) {
		if strings.HasSuffix(o, ":accept") {
			accept = o
		}
	}
	if accept == "" {
		t.Fatalf("the offer should lead to an accept option, got %v", node["options"])
	}
	resp := choose(t, sess, accept)
	if !quest.IsActive(&sess.SaveData, "rats-of-goldenhaven") {
		t.Fatal("accepting in conversation should start the quest")
	}
	if resp.Data["quest_accepted"] != "rats-of-goldenhaven" {
		t.Error("the response should tell the client a quest started")
	}
	// The thread ends with a way out instead of restarting the conversation.
	if opts := dialogueOf(t, resp)["options"].([]string); len(opts) == 0 {
		t.Error("after accepting, the player should be offered a way on (back / goodbye)")
	}

	// Back at the menu, the offer is gone — the quest is under way.
	back := dialogueOf(t, choose(t, sess, questKey("rats-of-goldenhaven", "offer", nodeBack)))
	if strings.Contains(questOption(back, "rats-of-goldenhaven"), ":offer:") {
		t.Error("an active quest must not be offered again")
	}
}

func TestForgedAcceptIsRefused(t *testing.T) {
	sess := bobSession(t)
	sess.SaveData.Race, sess.SaveData.Class = "Human", "Fighter"
	// Elven Heritage isn't Bob's to give, and a human can never take it.
	choose(t, sess, questKey("elven-heritage", "offer", "accept"))
	if quest.IsActive(&sess.SaveData, "elven-heritage") {
		t.Fatal("a forged accept key must not start a quest the NPC isn't offering")
	}
}

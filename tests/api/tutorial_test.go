package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"pubkey-quest/cmd/server/api/character"
	"pubkey-quest/cmd/server/api/game"
	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/events"
	gamechar "pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// The new start, end to end through the real handlers: a character is created
// with only the letter, their gold and their Homeward Stone, and the tutorial
// quest; their vault's keeper opens the inheritance; claiming it gives them
// their class's gear, once.

// Throwaway keys: nobody's, made for this test (sha256 of a phrase).
const (
	tutorialNpubA = "npub154smcj58kwmwqlfzlzq49ka2d7gmfx3rwrvuarsh0hh8qc7e9arqy0uhvs"
	tutorialNpubB = "npub17unhhtnvacfuxxpayg9wuhvs06hz7p8qc4q7zknk2hhs63he964quklfyr"
)

func setupTutorialServer(t *testing.T) *helpers.TestServer {
	t.Helper()
	helpers.SetupTestEnvironment(t)
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	events.Reset()
	adv, err := gamechar.LoadAdvancement(db.GetDB())
	if err != nil {
		t.Fatalf("advancement: %v", err)
	}
	events.Subscribe(quest.Consumer(db.GetQuestByID, adv))
	ts := helpers.NewTestServer()
	ts.Mux.HandleFunc("/api/character/create-save", character.CreateCharacterHandler)
	ts.Mux.HandleFunc("/api/game/action", game.GameActionHandler)
	return ts
}

// newCharacter creates a save through the creation endpoint and returns its id.
func newCharacter(t *testing.T, ts *helpers.TestServer, npub string) string {
	t.Helper()
	t.Cleanup(func() { os.RemoveAll(filepath.Join(session.SavesDirectory, npub)) })
	resp := ts.POST(t, "/api/character/create-save", map[string]any{"npub": npub, "name": "Tester"})
	helpers.AssertStatus(t, resp, http.StatusOK)
	out := helpers.AssertJSON(t, resp)
	id, _ := out["save_id"].(string)
	if id == "" {
		t.Fatalf("no save id: %v", out)
	}
	// Save ids are by the second: drop the session so a later test's character
	// can't be handed this one.
	t.Cleanup(func() { session.GetSessionManager().UnloadSession(npub, id) })
	return id
}

func act(t *testing.T, ts *helpers.TestServer, npub, saveID, kind string, params map[string]any) map[string]any {
	t.Helper()
	resp := ts.POST(t, "/api/game/action", map[string]any{"npub": npub, "save_id": saveID, "action": map[string]any{"type": kind, "params": params}})
	helpers.AssertStatus(t, resp, http.StatusOK)
	return helpers.AssertJSON(t, resp)
}

func liveSave(t *testing.T, npub, saveID string) *types.SaveFile {
	t.Helper()
	s, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		if s, err = session.GetSessionManager().LoadSession(npub, saveID); err != nil {
			t.Fatalf("session: %v", err)
		}
	}
	return &s.SaveData
}

// firstPicks builds a valid claim for a class: the first option of every choice.
func firstPicks(t *testing.T, class string) (map[string]any, string) {
	t.Helper()
	var raw string
	if err := db.GetDB().QueryRow("SELECT data FROM starting_gear WHERE id = 'starting-gear'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var all []character.StartingGearData
	if err := json.Unmarshal([]byte(raw), &all); err != nil {
		t.Fatal(err)
	}
	for _, g := range all {
		if g.Class != class {
			continue
		}
		picks := map[string]any{}
		for i, c := range g.StartingGear.EquipmentChoices {
			opt := c.Options[0]
			key := fmt.Sprintf("choice-%d", i)
			switch opt.Type {
			case "single":
				picks[key] = opt.Item
			case "bundle":
				var list [][]any
				for _, it := range opt.Items {
					list = append(list, []any{it.Item, it.Quantity})
				}
				b, _ := json.Marshal(list)
				picks[key] = string(b)
			case "multi_slot":
				var list [][]any
				for _, s := range opt.Slots {
					if s.Type == "fixed" {
						list = append(list, []any{s.Item, max(s.Quantity, 1)})
					} else {
						list = append(list, []any{s.Options[0], 1})
					}
				}
				b, _ := json.Marshal(list)
				picks[key] = string(b)
			}
		}
		pack := ""
		if g.StartingGear.PackChoice != nil && len(g.StartingGear.PackChoice.Options) > 0 {
			pack = g.StartingGear.PackChoice.Options[0]
		}
		return picks, pack
	}
	t.Fatalf("no starting gear for %s", class)
	return nil, ""
}

// keeperAndVault finds the tutorial's keeper and the building they work in.
func keeperAndVault(t *testing.T, save *types.SaveFile) (questID, keeper, vault string) {
	t.Helper()
	for _, qp := range save.QuestsActive {
		if strings.HasPrefix(qp.QuestID, "caretakers-letter-") {
			questID = qp.QuestID
		}
	}
	if questID == "" {
		t.Fatalf("no tutorial quest: %+v", save.QuestsActive)
	}
	q, _ := db.GetQuestByID(questID)
	keeper = q.StartCondition.Target
	npc, err := db.GetNPCByID(keeper)
	if err != nil || npc == nil || len(npc.Schedule) == 0 {
		t.Fatalf("keeper %s: %v", keeper, err)
	}
	vault, _ = npc.Schedule[0].(map[string]any)["location"].(string)
	return
}

func TestTutorialFromCreationToInheritance(t *testing.T) {
	ts := setupTutorialServer(t)
	defer ts.Close()
	defer db.Close()

	saveID := newCharacter(t, ts, tutorialNpubA)
	save := liveSave(t, tutorialNpubA, saveID)

	// Arrives carrying only the letter; the gold and the stone are in the vault.
	if gameutil.CountItem(save, character.CaretakersLetter) != 1 {
		t.Error("a new character should carry the letter")
	}
	if gameutil.CountItem(save, "gold-piece") != 0 {
		t.Error("the starting gold should be in the vault, not the bag")
	}
	if len(save.Vault) < 2 {
		t.Errorf("the vault should hold the gold and the Homeward Stone: %+v", save.Vault)
	}
	questID, keeper, vault := keeperAndVault(t, save)
	if quest.StageOf(save, questID) != 0 {
		t.Fatalf("the tutorial starts at stage 0, got %d", quest.StageOf(save, questID))
	}

	// First, read the letter.
	if r := act(t, ts, tutorialNpubA, saveID, "read_letter", nil); r["success"] != true {
		t.Fatalf("read_letter: %v", r)
	}
	if quest.StageOf(liveSave(t, tutorialNpubA, saveID), questID) != 1 {
		t.Fatal("reading the letter should move the tutorial on")
	}

	// Go in and talk to the keeper: stage 1, and the keeper offers the inheritance.
	if r := act(t, ts, tutorialNpubA, saveID, "enter_building", map[string]any{"building_id": vault}); r["success"] != true {
		t.Fatalf("enter %s: %v", vault, r)
	}
	talk := act(t, ts, tutorialNpubA, saveID, "talk_to_npc", map[string]any{"npc_id": keeper})
	if quest.StageOf(liveSave(t, tutorialNpubA, saveID), questID) != 2 {
		t.Fatalf("talking to %s should reach stage 2: %v", keeper, talk)
	}
	hub := "quest:" + questID + ":active:hub"
	if !strings.Contains(fmt.Sprint(talk), hub) {
		t.Fatalf("the keeper's menu should offer the letter thread: %v", talk)
	}
	act(t, ts, tutorialNpubA, saveID, "npc_dialogue_choice", map[string]any{"npc_id": keeper, "choice": hub})
	inherit := act(t, ts, tutorialNpubA, saveID, "npc_dialogue_choice", map[string]any{"npc_id": keeper, "choice": "quest:" + questID + ":active:inherit"})
	take := act(t, ts, tutorialNpubA, saveID, "npc_dialogue_choice", map[string]any{"npc_id": keeper, "choice": "quest:" + questID + ":active:take"})
	data, _ := take["data"].(map[string]any)
	if data == nil || data["open_starting_gear"] == nil {
		t.Fatalf("'take' should open the gear selection: inherit=%v take=%v", inherit, take)
	}

	// Claim the first option of every choice.
	save = liveSave(t, tutorialNpubA, saveID)
	picks, pack := firstPicks(t, save.Class)
	claim := act(t, ts, tutorialNpubA, saveID, "claim_starting_gear", map[string]any{"equipment_choices": picks, "pack_choice": pack})
	if claim["success"] != true {
		t.Fatalf("claim refused: %v", claim)
	}
	save = liveSave(t, tutorialNpubA, saveID)
	if gameutil.CountItem(save, character.CaretakersLetter) != 1 {
		t.Error("the letter must survive the claim")
	}
	if !slices.Contains(save.DialogueGifts, "starting_gear") {
		t.Error("the claim should be recorded")
	}
	// The pack's carrying capacity applies at once (worn effects come with the claim).
	if bag, _ := save.Inventory["gear_slots"].(map[string]interface{})["bag"].(map[string]interface{}); bag["item"] == "backpack" {
		has := false
		for _, e := range save.ActiveEffects {
			has = has || e.EffectID == "backpack-capacity"
		}
		if !has {
			t.Error("the claimed backpack's capacity effect should be active")
		}
	}
	if got := quest.StageOf(save, questID); got != 3 {
		t.Errorf("after the claim the tutorial should be on the vault (3), got %d", got)
	}

	// The claimed pack works like any other: the letter goes into it. (The
	// claim used to leave the pack's contents in a shape the inventory code
	// rejected: "invalid backpack".)
	letterSlot := -1
	for i, sl := range save.Inventory["general_slots"].([]interface{}) {
		if m, _ := sl.(map[string]interface{}); m["item"] == character.CaretakersLetter {
			letterSlot = i
		}
	}
	bag, _ := save.Inventory["gear_slots"].(map[string]interface{})["bag"].(map[string]interface{})
	contents, ok := bag["contents"].([]interface{})
	if !ok {
		t.Fatalf("the pack's contents should be a plain list, got %T", bag["contents"])
	}
	free := -1
	for i, c := range contents {
		if m, _ := c.(map[string]interface{}); m["item"] == nil {
			free = i
			break
		}
	}
	if letterSlot >= 0 && free >= 0 {
		mv := act(t, ts, tutorialNpubA, saveID, "move_item", map[string]any{
			"item_id": character.CaretakersLetter, "from_slot": letterSlot, "from_slot_type": "general",
			"to_slot": free, "to_slot_type": "inventory",
		})
		if mv["success"] != true {
			t.Fatalf("moving the letter into the pack failed: %v", mv)
		}
	}

	// Take the gold and the stone out of the vault: on to supplies.
	for _, e := range append([]types.VaultEntry{}, liveSave(t, tutorialNpubA, saveID).Vault...) {
		if r := act(t, ts, tutorialNpubA, saveID, "vault_withdraw", map[string]any{"item_id": e.ItemID, "quantity": e.Quantity}); r["success"] != true {
			t.Fatalf("withdraw %s: %v", e.ItemID, r)
		}
	}
	if got := quest.StageOf(liveSave(t, tutorialNpubA, saveID), questID); got != 4 {
		t.Errorf("with the gold and the stone in hand the tutorial should be on supplies (4), got %d", got)
	}

	// Once only.
	if again := act(t, ts, tutorialNpubA, saveID, "claim_starting_gear", map[string]any{"equipment_choices": picks, "pack_choice": pack}); again["success"] == true {
		t.Error("the inheritance can only be claimed once")
	}
}

func TestTutorialSkipAndForgedClaims(t *testing.T) {
	ts := setupTutorialServer(t)
	defer ts.Close()
	defer db.Close()

	saveID := newCharacter(t, ts, tutorialNpubB)
	save := liveSave(t, tutorialNpubB, saveID)
	questID, _, _ := keeperAndVault(t, save)
	picks, pack := firstPicks(t, save.Class)

	// A forged pick (an item the class wasn't offered) is refused and grants nothing.
	forged := map[string]any{}
	for k, v := range picks {
		forged[k] = v
	}
	forged["choice-0"] = `[["plate-set",1],["gold-piece",100000]]`
	if r := act(t, ts, tutorialNpubB, saveID, "claim_starting_gear", map[string]any{"equipment_choices": forged, "pack_choice": pack}); r["success"] == true {
		t.Fatalf("a forged claim was accepted: %v", r)
	}
	if slices.Contains(liveSave(t, tutorialNpubB, saveID).DialogueGifts, "starting_gear") {
		t.Fatal("a refused claim mustn't use up the inheritance")
	}

	// "I've done this before": the gear, then the tutorial is done.
	if r := act(t, ts, tutorialNpubB, saveID, "claim_starting_gear", map[string]any{"equipment_choices": picks, "pack_choice": pack, "skip": true}); r["success"] != true {
		t.Fatalf("claim with skip refused: %v", r)
	}
	if !quest.IsCompleted(liveSave(t, tutorialNpubB, saveID), questID) {
		t.Error("skipping should complete the tutorial")
	}
}

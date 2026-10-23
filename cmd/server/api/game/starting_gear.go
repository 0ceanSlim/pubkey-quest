package game

import (
	"fmt"
	"log"
	"slices"
	"strings"

	"pubkey-quest/cmd/server/api/character"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

// ─── The inheritance: starting gear from your vault's keeper ─────────────────
//
// A new character arrives with only the caretaker's letter; their gold and
// their Homeward Stone wait in their vault. Their gear is claimed in game from
// their people's vault keeper, inside the tutorial quest (docs/draft/intro-and-tutorial.md): the
// keeper's "inheritance" line opens the gear selection, and this action takes
// the picks. Once per character, recorded in DialogueGifts (no new save field).

// startingGearGift marks the inheritance as given in SaveFile.DialogueGifts.
const startingGearGift = "starting_gear"

// tutorialQuest is the character's active creation quest (their city's
// "The Caretaker's Letter"), or nil.
func tutorialQuest(save *types.SaveFile) *types.QuestData {
	for _, qp := range save.QuestsActive {
		if q, err := serverdb.GetQuestByID(qp.QuestID); err == nil && q != nil && q.StartCondition.Type == quest.StartAtCreation {
			return q
		}
	}
	return nil
}

// handleReadLetter notes that the caretaker's letter was read (the client
// shows it): the tutorial's first step.
func handleReadLetter(session *GameSession) (*GameActionResponse, error) {
	save := &session.SaveData
	if gameutil.CountItem(save, character.CaretakersLetter) == 0 {
		return &GameActionResponse{Success: false, Message: "You don't have the letter with you."}, nil
	}
	events.Record(save, events.Did, "read-letter", 1)
	return &GameActionResponse{Success: true}, nil
}

// handleClaimStartingGear grants the checked picks. Params: equipment_choices
// ({"choice-0": …}), pack_choice, and skip (true for "I've done this before":
// the gear, then the tutorial is finished on the spot).
func handleClaimStartingGear(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	save := &session.SaveData
	if slices.Contains(save.DialogueGifts, startingGearGift) {
		return &GameActionResponse{Success: false, Message: "You've already taken up your inheritance.", Color: "yellow"}, nil
	}
	tq := tutorialQuest(save)
	if tq == nil {
		return &GameActionResponse{Success: false, Message: "There's nothing held in trust for you here.", Color: "yellow"}, nil
	}

	choices := map[string]string{}
	if raw, ok := params["equipment_choices"].(map[string]any); ok {
		for k, v := range raw {
			if s, ok := v.(string); ok {
				choices[k] = s
			}
		}
	}
	pack, _ := params["pack_choice"].(string)
	skip, _ := params["skip"].(bool)

	items, err := character.ClaimStartingGear(serverdb.GetDB(), save, choices, pack)
	if err != nil {
		return &GameActionResponse{Success: false, Message: fmt.Sprintf("That won't do: %v.", err), Color: "yellow"}, nil
	}
	// Worn gear's effects (a backpack's carrying capacity) apply on equip, and
	// this gear was laid out rather than equipped: apply them as loading a save
	// does, then recompute the load.
	if err := status.InitializeEquipmentEffects(save); err != nil {
		log.Printf("⚠️ inheritance: worn effects: %v", err)
	}
	if _, err := status.UpdateEncumbrancePenaltyEffects(save); err != nil {
		log.Printf("⚠️ inheritance: encumbrance: %v", err)
	}
	save.DialogueGifts = append(save.DialogueGifts, startingGearGift)
	events.Record(save, events.Did, "claim-starting-gear", 1)
	if skip {
		if adv, err := loadAdvancement(); err == nil {
			quest.FinishNow(save, tq, adv)
		}
	}

	var names []string
	for _, it := range items {
		name := it.Item
		if item, err := serverdb.GetItemByID(it.Item); err == nil && item != nil && item.Name != "" {
			name = item.Name
		}
		if it.Quantity > 1 {
			name = fmt.Sprintf("%d× %s", it.Quantity, name)
		}
		names = append(names, name)
	}
	msg := "You take up your inheritance: " + strings.Join(names, ", ") + ". What you can wear, you're wearing. Your gold and your Homeward Stone are in your vault."
	return &GameActionResponse{
		Success: true,
		Message: msg,
		Color:   "green",
		Data:    map[string]interface{}{"starting_gear_claimed": true, "tutorial_skipped": skip},
	}, nil
}

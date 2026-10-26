package game

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

// The inn kitchen (food-system §4–§5): every innkeeper serves a hot meal, eaten
// on the spot, and cooks the raw food you carry for a few gold each. The menu
// and prices are game-data/systems/cooking.json.

const (
	mealChoicePrefix = "inn:meal:"
	cookChoice       = "inn:cook"
)

type innMeal struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Cost   int     `json:"cost"`
	Hours  float64 `json:"hours"`
	Effect string  `json:"effect"`
}

type cookingConfig struct {
	Inn struct {
		CookCost int       `json:"cook_cost"`
		Meals    []innMeal `json:"meals"`
	} `json:"inn"`
	MealEffects []string `json:"meal_effects"`
}

var (
	cookingOnce sync.Once
	cookingCfg  cookingConfig
)

func cooking() *cookingConfig {
	cookingOnce.Do(func() {
		database := serverdb.GetDB()
		if database == nil {
			return
		}
		var props string
		if database.QueryRow("SELECT properties FROM systems WHERE id = 'cooking'").Scan(&props) == nil {
			_ = json.Unmarshal([]byte(props), &cookingCfg)
		}
	})
	return &cookingCfg
}

// isInnkeeper reports whether an NPC rents rooms (any dialogue node does).
func isInnkeeper(n *serverdb.NPCData) bool {
	for _, raw := range n.Dialogue {
		if node, ok := raw.(map[string]interface{}); ok && node["action"] == "rent_room" {
			return true
		}
	}
	return false
}

// injectInnOptions adds the meals and "Cook my food" to an innkeeper's menu,
// before goodbye.
func injectInnOptions(resp *types.GameActionResponse, npcID string, state *types.SaveFile) {
	if resp == nil || resp.Delta == nil {
		return
	}
	dlg, ok := resp.Delta["npc_dialogue"].(map[string]interface{})
	if !ok {
		return
	}
	npcData, err := serverdb.GetNPCByID(npcID)
	if err != nil || npcData == nil || !isInnkeeper(npcData) || !npc.OnDuty(state, npcID, "rent_room") {
		return
	}
	cfg := cooking()
	var keys []string
	labels := map[string]string{}
	for _, m := range cfg.Inn.Meals {
		k := mealChoicePrefix + m.ID
		keys = append(keys, k)
		labels[k] = fmt.Sprintf("🍲 Order a hot meal: %s (%d gold)", m.Name, m.Cost)
	}
	if n := inventory.CountRawFood(state); n > 0 {
		keys = append(keys, cookChoice)
		labels[cookChoice] = fmt.Sprintf("🔥 Cook my food (%d gold)", n*cfg.Inn.CookCost)
	}
	if len(keys) == 0 {
		return
	}
	options, _ := dlg["options"].([]string)
	at := len(options)
	if at > 0 && closesDialogue(npcData, options[at-1]) {
		at--
	}
	dlg["options"] = append(append(append([]string{}, options[:at]...), keys...), options[at:]...)
	dlg["option_labels"] = mergeLabels(dlg["option_labels"], labels)
}

// handleInnChoice answers a meal or cooking choice; nil when the choice isn't one.
func handleInnChoice(session *GameSession, npcID, choice string) *GameActionResponse {
	if !strings.HasPrefix(choice, mealChoicePrefix) && choice != cookChoice {
		return nil
	}
	state := &session.SaveData
	npcData, err := serverdb.GetNPCByID(npcID)
	if err != nil || npcData == nil || !isInnkeeper(npcData) || !npc.OnDuty(state, npcID, "rent_room") {
		return &GameActionResponse{Success: false, Message: "There's no one here to serve you.", Color: "yellow"}
	}
	cfg := cooking()
	if choice == cookChoice {
		n := inventory.CountRawFood(state)
		if n == 0 {
			return &GameActionResponse{Success: false, Message: "You've nothing raw to cook.", Color: "yellow"}
		}
		cost := n * cfg.Inn.CookCost
		if gameutil.GetGoldQuantity(state) < cost {
			return &GameActionResponse{Success: false, Message: fmt.Sprintf("That's %d gold. You don't have enough.", cost), Color: "yellow"}
		}
		gameutil.DeductGold(state, cost)
		made := inventory.CookAllAtInn(state)
		return &GameActionResponse{Success: true, Color: "green",
			Message: fmt.Sprintf("%s takes your food back to the kitchen and returns it hot: %s. (%d gold)", npcData.Name, inventory.MadeList(made), cost),
			Delta:   map[string]interface{}{"npc_dialogue": map[string]interface{}{"action": "close"}}}
	}

	id := strings.TrimPrefix(choice, mealChoicePrefix)
	var meal *innMeal
	for i := range cfg.Inn.Meals {
		if cfg.Inn.Meals[i].ID == id {
			meal = &cfg.Inn.Meals[i]
		}
	}
	if meal == nil {
		return &GameActionResponse{Success: false, Message: "That's not on the menu.", Color: "yellow"}
	}
	if effects.HasActiveEffect(state, inventory.FoodPoisoning) {
		return &GameActionResponse{Success: false, Message: "Your stomach won't take anything until the food poisoning passes.", Color: "yellow"}
	}
	if status.IsFull(state) {
		return &GameActionResponse{Success: false, Message: "You couldn't manage another bite.", Color: "yellow"}
	}
	if gameutil.GetGoldQuantity(state) < meal.Cost {
		return &GameActionResponse{Success: false, Message: fmt.Sprintf("The %s is %d gold. You don't have enough.", meal.Name, meal.Cost), Color: "yellow"}
	}
	gameutil.DeductGold(state, meal.Cost)
	gained, _ := status.Feed(state, int(meal.Hours*60))
	msg := fmt.Sprintf("🍲 You eat a bowl of %s by the fire. It keeps you fed %s.", meal.Name, inventory.HoursLabel(gained))
	if meal.Effect != "" {
		// One meal buff at a time: a new meal replaces the last.
		for _, e := range cfg.MealEffects {
			effects.RemoveEffect(state, e)
		}
		if m, err := effects.ApplyEffectWithMessage(state, meal.Effect); err == nil && m != nil && m.Message != "" {
			msg += " " + m.Message
		}
	}
	return &GameActionResponse{Success: true, Color: "green", Message: msg,
		Delta: map[string]interface{}{"npc_dialogue": map[string]interface{}{"action": "close"}}}
}

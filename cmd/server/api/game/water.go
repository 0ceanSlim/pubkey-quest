package game

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/types"
)

// The waterskin (degradation §7): its charges are drinks, filled wherever
// there's water. condition.json "water" says where: every town, and the wild
// places listed. Drinking will matter in the desert once its heat does.

var (
	waterOnce sync.Once
	waterAt   struct {
		Towns        bool     `json:"towns"`
		Environments []string `json:"environments"`
	}
)

// waterHere reports whether there's water to fill a waterskin where you are.
func waterHere(state *types.SaveFile) bool {
	waterOnce.Do(func() {
		database := serverdb.GetDB()
		if database == nil {
			return
		}
		var props string
		if database.QueryRow("SELECT properties FROM systems WHERE id = 'condition'").Scan(&props) != nil {
			return
		}
		var cfg struct {
			Water json.RawMessage `json:"water"`
		}
		if json.Unmarshal([]byte(props), &cfg) == nil && cfg.Water != nil {
			_ = json.Unmarshal(cfg.Water, &waterAt)
		}
	})
	if travel.GetEnvironmentData(state.Location) == nil {
		return waterAt.Towns
	}
	return slices.Contains(waterAt.Environments, state.Location)
}

// handleDrink drinks from a waterskin. params: slot_type, slot, gear.
func handleDrink(state *types.SaveFile, params map[string]any) (*GameActionResponse, error) {
	ref := slotRefFrom(params)
	slot := inventory.SlotAt(state, ref)
	id, _ := slot["item"].(string)
	c := condition.ChargesOf(id)
	if slot == nil || c == nil || c.Refill != "water" {
		return &GameActionResponse{Success: false, Message: "You can't drink that.", Color: "yellow"}, nil
	}
	if condition.ChargesLeft(slot) == 0 {
		return &GameActionResponse{Success: false, Message: fmt.Sprintf("Your %s is empty.", condition.ItemName(id)), Color: "yellow"}, nil
	}
	left, _ := condition.UseCharge(slot)
	msg := fmt.Sprintf("You take a long drink from your %s. (%d left)", condition.ItemName(id), left)
	if left == 0 {
		msg = fmt.Sprintf("You drain the last of your %s.", condition.ItemName(id))
	}
	return &GameActionResponse{Success: true, Message: msg, Color: "blue"}, nil
}

// handleFillWater fills every waterskin you carry, where there's water.
func handleFillWater(state *types.SaveFile) (*GameActionResponse, error) {
	if !waterHere(state) {
		return &GameActionResponse{Success: false, Message: "There's no water here to fill it from.", Color: "yellow"}, nil
	}
	filled := 0
	for _, slots := range gameutil.CarriedSlotArrays(state) {
		for _, raw := range slots {
			slot, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := slot["item"].(string)
			if c := condition.ChargesOf(id); c != nil && c.Refill == "water" && condition.ChargesLeft(slot) < c.Max {
				condition.RefillCharges(slot)
				filled++
			}
		}
	}
	if filled == 0 {
		return &GameActionResponse{Success: false, Message: "Your waterskins are already full.", Color: "yellow"}, nil
	}
	return &GameActionResponse{Success: true, Message: "You fill your waterskin to the brim.", Color: "blue"}, nil
}

// slotRefFrom reads a SlotRef from action params.
func slotRefFrom(params map[string]any) inventory.SlotRef {
	ref := inventory.SlotRef{}
	ref.Type, _ = params["slot_type"].(string)
	ref.Gear, _ = params["gear"].(string)
	if n, ok := params["slot"].(float64); ok {
		ref.Index = int(n)
	}
	return ref
}

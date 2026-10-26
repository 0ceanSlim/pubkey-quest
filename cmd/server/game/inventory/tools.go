package inventory

import (
	"encoding/json"
	"fmt"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

// Using a tool on an item (degradation §6, "the Use on… picker"): a tool's
// "use_on" names what it does to other items: "light" for the tinderbox,
// "fuel" for oil, "sharpen" for the whetstone, "mend" for the crafting kit and "salt" for salt. UseTargets lists the items you carry it works on (the client
// shows them in a small box); UseOn does it. One pattern for every tool, and it
// never offers a pair that does nothing.

// SlotRef addresses one inventory slot: a general or backpack slot by index,
// or a gear slot by name.
type SlotRef struct {
	Type  string `json:"slot_type"`      // "general", "inventory" (the backpack) or "equipment"
	Index int    `json:"slot"`           // general / backpack index
	Gear  string `json:"gear,omitempty"` // gear slot name for "equipment"
}

// UseTarget is one item a tool can be used on, and what it would do.
type UseTarget struct {
	SlotRef
	ItemID string `json:"item_id"`
	Label  string `json:"label"` // "Light Torch", "Put out Lamp", "Refill Lamp (2h left)"
}

// toolUse is a tool's "use_on" ("" when it isn't a tool).
func toolUse(itemID string) string {
	item, err := db.GetItemByID(itemID)
	if err != nil || item == nil {
		return ""
	}
	var props struct {
		UseOn string `json:"use_on"`
	}
	_ = json.Unmarshal([]byte(item.Properties), &props)
	return props.UseOn
}

// SlotAt resolves a SlotRef to its slot map.
func SlotAt(state *types.SaveFile, ref SlotRef) map[string]interface{} {
	if ref.Type == "equipment" {
		gear, _ := state.Inventory["gear_slots"].(map[string]interface{})
		slot, _ := gear[ref.Gear].(map[string]interface{})
		return slot
	}
	slots, err := inventorySlots(state, ref.Type)
	if err != nil || ref.Index < 0 || ref.Index >= len(slots) {
		return nil
	}
	slot, _ := slots[ref.Index].(map[string]interface{})
	return slot
}

// eachCarried visits the general and backpack slots, then the off hand.
func eachCarried(state *types.SaveFile, visit func(ref SlotRef, slot map[string]interface{})) {
	for _, kind := range []string{"general", "inventory"} {
		slots, err := inventorySlots(state, kind)
		if err != nil {
			continue
		}
		for i, raw := range slots {
			if slot, ok := raw.(map[string]interface{}); ok && slot["item"] != nil && slot["item"] != "" {
				visit(SlotRef{Type: kind, Index: i}, slot)
			}
		}
	}
	gear, _ := state.Inventory["gear_slots"].(map[string]interface{})
	if slot, ok := gear["offhand"].(map[string]interface{}); ok && slot["item"] != nil && slot["item"] != "" {
		visit(SlotRef{Type: "equipment", Gear: "offhand"}, slot)
	}
}

// UseTargets lists what the tool can be used on right now.
func UseTargets(state *types.SaveFile, toolID string) []UseTarget {
	var out []UseTarget
	use := toolUse(toolID)
	if use == "sharpen" || use == "mend" {
		return repairTargets(state, use)
	}
	if use == "salt" {
		return saltTargets(state)
	}
	eachCarried(state, func(ref SlotRef, slot map[string]interface{}) {
		id, _ := slot["item"].(string)
		name := condition.ItemName(id)
		switch use {
		case "light":
			l := condition.LightOf(id)
			switch {
			case l == nil:
			case condition.IsLit(slot):
				out = append(out, UseTarget{SlotRef: ref, ItemID: id, Label: "Put out " + name})
			case condition.BurnLeft(slot) > 0:
				out = append(out, UseTarget{SlotRef: ref, ItemID: id, Label: "Light " + name})
			}
		case "fuel":
			if condition.NeedsFuel(slot, fuelOf(toolID)) {
				out = append(out, UseTarget{SlotRef: ref, ItemID: id,
					Label: fmt.Sprintf("Refill %s (%s left)", name, hoursLabel(condition.BurnLeft(slot)))})
			}
		}
	})
	return out
}

// fuelOf is the fuel a "fuel" tool is (its id: oil fills what burns oil).
func fuelOf(toolID string) string { return toolID }

// UseOn uses a tool on the item in a slot.
func UseOn(state *types.SaveFile, toolID string, ref SlotRef) (*types.GameActionResponse, error) {
	if !gameutil.PlayerHasItem(state, toolID) {
		return refuse(fmt.Sprintf("You need a %s.", condition.ItemName(toolID))), nil
	}
	slot := SlotAt(state, ref)
	if slot == nil || slot["item"] == nil || slot["item"] == "" {
		return nil, fmt.Errorf("nothing there")
	}
	id, _ := slot["item"].(string)
	switch use := toolUse(toolID); use {
	case "sharpen", "mend":
		return fieldRepair(state, toolID, use, slot), nil
	case "salt":
		return saltIt(state, slot), nil
	case "light":
		if condition.LightOf(id) == nil {
			return refuse("That doesn't light."), nil
		}
		if condition.IsLit(slot) {
			return putOut(state, ref, slot), nil
		}
		return lightIt(state, ref, slot)
	case "fuel":
		if !condition.NeedsFuel(slot, fuelOf(toolID)) {
			return refuse(fmt.Sprintf("The %s doesn't need it.", condition.ItemName(id))), nil
		}
		gameutil.ConsumeItem(state, toolID, 1)
		condition.Refill(slot)
		return ok(fmt.Sprintf("You fill the %s with %s.", condition.ItemName(id), condition.ItemName(toolID))), nil
	}
	return refuse("Nothing interesting happens."), nil
}

// lightIt lights a light and puts it in your off hand (degradation §6):
// whatever you held there goes back to your bag, and if there's no room for
// it, the light stays unlit. One torch comes off a stack.
func lightIt(state *types.SaveFile, ref SlotRef, slot map[string]interface{}) (*types.GameActionResponse, error) {
	id, _ := slot["item"].(string)
	name := condition.ItemName(id)
	gear, _ := state.Inventory["gear_slots"].(map[string]interface{})

	// Lighting the one already in your hand.
	if ref.Type == "equipment" {
		if qty := GetSlotQuantity(slot); qty > 1 {
			// A held stack: one stays lit in your hand, the rest go to your bag.
			if added, _ := AddItemWithState(state, id, qty-1, condition.State(slot)); added < qty-1 {
				return refuse("No room in your bag for the rest of the stack."), nil
			}
			slot["quantity"] = 1
		}
		if err := condition.Ignite(slot); err != nil {
			return refuse(err.Error() + "."), nil
		}
		return ok(fmt.Sprintf("You light the %s.", name)), nil
	}

	// Lighting one from your pack: it goes into your off hand.
	if main, _ := gear["mainhand"].(map[string]interface{}); main != nil {
		if mid, _ := main["item"].(string); mid != "" && gameutil.IsTwoHanded(mid) {
			return refuse(fmt.Sprintf("Your hands are full: put away your %s to hold a light.", condition.ItemName(mid))), nil
		}
	}
	lit := map[string]interface{}{"item": id, "quantity": 1}
	condition.CopyState(lit, slot)
	if err := condition.Ignite(lit); err != nil {
		return refuse(err.Error() + "."), nil
	}

	// Whatever's in your off hand goes back to your bag (a lit light goes out).
	if held, _ := gear["offhand"].(map[string]interface{}); held != nil {
		if hid, _ := held["item"].(string); hid != "" {
			heldState, keep := takenOutOfHand(held)
			if keep {
				if added, _ := AddItemWithState(state, hid, max(GetSlotQuantity(held), 1), heldState); added == 0 {
					return refuse(fmt.Sprintf("No room in your bag for your %s.", condition.ItemName(hid))), nil
				}
			}
		}
	}

	// Take one off the stack in your pack.
	if qty := GetSlotQuantity(slot); qty > 1 {
		slot["quantity"] = qty - 1
	} else {
		slot["item"], slot["quantity"] = nil, 0
		condition.ClearState(slot)
	}
	gear["offhand"] = lit
	_ = status.InitializeEquipmentEffects(state)
	return ok(fmt.Sprintf("You light the %s and hold it up.", name)), nil
}

// putOut puts the lit light out: a torch or candle is spent, a lamp keeps its oil.
func putOut(state *types.SaveFile, ref SlotRef, slot map[string]interface{}) *types.GameActionResponse {
	id, _ := slot["item"].(string)
	name := condition.ItemName(id)
	if condition.Douse(slot) {
		return ok(fmt.Sprintf("You put out the %s.", name))
	}
	empty := map[string]interface{}{"item": nil, "quantity": 0}
	if ref.Type == "equipment" {
		gear, _ := state.Inventory["gear_slots"].(map[string]interface{})
		gear[ref.Gear] = empty
	} else {
		slot["item"], slot["quantity"] = nil, 0
		condition.ClearState(slot)
	}
	return ok(fmt.Sprintf("You put out the %s. It's spent.", name))
}

func ok(msg string) *types.GameActionResponse {
	return &types.GameActionResponse{Success: true, Message: msg, Color: "yellow"}
}

func refuse(msg string) *types.GameActionResponse {
	return &types.GameActionResponse{Success: false, Message: msg, Color: "yellow"}
}

// ToggleShutter closes or opens a lit hooded lantern's shutter (the light in
// your off hand). Closed, it burns on but sheds no light and gives nothing away.
func ToggleShutter(state *types.SaveFile) *types.GameActionResponse {
	id, slot, light := condition.HeldLight(state)
	if slot == nil || light == nil || !light.Shutter {
		return refuse("You're not holding a lantern with a shutter.")
	}
	if !condition.IsLit(slot) {
		return refuse(fmt.Sprintf("Light the %s first.", condition.ItemName(id)))
	}
	if condition.IsShut(slot) {
		delete(slot, condition.KeyShut)
		return ok(fmt.Sprintf("You open the %s's shutter. Light spills out.", condition.ItemName(id)))
	}
	slot[condition.KeyShut] = 1
	return ok(fmt.Sprintf("You close the %s's shutter. It burns on in the dark.", condition.ItemName(id)))
}

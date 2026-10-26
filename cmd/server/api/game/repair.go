package game

import (
	"fmt"
	"strings"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/types"
)

// Smith repair (degradation §5): the smiths in condition.json's "repair" get a
// "🔨 Repair my gear" option. It opens a window of your worn gear with what each
// repair costs; "smith_repair" mends one.

// repairChoice is the dialogue key of the injected Repair option.
const repairChoice = "smith:repair"

func init() {
	// Field repair's Crafting check reads the same effective skill as quests.
	inventory.SkillOf = func(state *types.SaveFile, skill string) int {
		return buildQuestContext(state).SkillValue(skill)
	}
}

// injectRepairOption adds the Repair option to a smith's menu, before goodbye.
func injectRepairOption(resp *types.GameActionResponse, npcID string, state *types.SaveFile) {
	if _, ok := condition.Repair().Smiths[npcID]; !ok || resp == nil || resp.Delta == nil || !npc.OnDuty(state, npcID, "open_shop") {
		return
	}
	dlg, ok := resp.Delta["npc_dialogue"].(map[string]interface{})
	if !ok {
		return
	}
	npcData, err := serverdb.GetNPCByID(npcID)
	if err != nil || npcData == nil {
		return
	}
	options, _ := dlg["options"].([]string)
	at := len(options)
	if at > 0 && closesDialogue(npcData, options[at-1]) {
		at--
	}
	dlg["options"] = append(append(append([]string{}, options[:at]...), repairChoice), options[at:]...)
	dlg["option_labels"] = mergeLabels(dlg["option_labels"], map[string]string{repairChoice: "🔨 Repair my gear"})
}

// repairRow is one worn item in the smith's window.
type repairRow struct {
	inventory.SlotRef
	ItemID   string          `json:"item_id"`
	Name     string          `json:"name"`
	Tier     string          `json:"tier"`
	Left     int             `json:"left"` // % durability left
	Quote    condition.Quote `json:"quote"`
	Material string          `json:"material_name,omitempty"`
	Have     int             `json:"have,omitempty"` // of the material
	Missing  string          `json:"missing,omitempty"`
	Reason   string          `json:"reason,omitempty"` // why they won't
}

// smithPrice is what the smith charges once a carried discount item is counted.
func smithPrice(state *types.SaveFile, q condition.Quote) (gold int, discounted bool) {
	if q.Discount != "" && gameutil.CountItem(state, q.Discount) > 0 {
		return max(q.Gold*(100-q.Percent)/100, 1), true
	}
	return q.Gold, false
}

// repairWindow lists everything you carry or wear that has taken wear.
func repairWindow(state *types.SaveFile, npcID string) map[string]interface{} {
	deal := condition.Repair().Smiths[npcID]
	rows := []repairRow{}
	inventory.EachGear(state, func(ref inventory.SlotRef, slot map[string]interface{}) {
		id, _ := slot["item"].(string)
		if condition.MaxDurability(id) == 0 || condition.WearOf(slot) == 0 {
			return
		}
		value := 0
		if item, err := serverdb.GetItemByID(id); err == nil && item != nil {
			value = item.Value
		}
		tier, left := condition.WearTier(slot)
		row := repairRow{SlotRef: ref, ItemID: id, Name: condition.ItemName(id), Tier: tier.String(),
			Left: int(left*100 + 0.5), Quote: condition.SmithQuote(deal, slot, value)}
		switch {
		case row.Quote.CantWork:
			row.Reason = fmt.Sprintf("Doesn't work %s.", condition.MaterialOf(id))
		case row.Quote.Untouched:
			row.Reason = "Good enough as it is: they only fix it up to Worn."
		default:
			row.Quote.Gold, _ = smithPrice(state, row.Quote)
			if row.Quote.Material != "" {
				row.Material = condition.ItemName(row.Quote.Material)
				row.Have = gameutil.CountItem(state, row.Quote.Material)
				if row.Have < row.Quote.Count {
					row.Missing = fmt.Sprintf("Needs %d %s.", row.Quote.Count, row.Material)
				}
			}
		}
		rows = append(rows, row)
	})
	name := npcID
	if n, err := serverdb.GetNPCByID(npcID); err == nil && n != nil {
		name = n.Name
	}
	return map[string]interface{}{
		"npc_id": npcID, "npc_name": name, "items": rows,
		"gold": gameutil.GetGoldQuantity(state), "rough": deal.UpTo == "worn",
		"materials": deal.Materials, "discount": discountName(deal),
	}
}

func discountName(deal condition.SmithDeal) string {
	if deal.Discount == nil {
		return ""
	}
	return fmt.Sprintf("%d%% off with %s", deal.Discount.Percent, condition.ItemName(deal.Discount.Item))
}

// openRepair answers the Repair option: close the dialogue, open the window.
func openRepair(session *GameSession, npcID string) *GameActionResponse {
	if _, ok := condition.Repair().Smiths[npcID]; !ok || !npc.OnDuty(&session.SaveData, npcID, "open_shop") {
		return &GameActionResponse{Success: false, Message: "There's no one here to repair your gear.", Color: "yellow"}
	}
	return &GameActionResponse{Success: true,
		Delta: map[string]interface{}{"npc_dialogue": map[string]interface{}{"action": "close"}},
		Data:  map[string]interface{}{"open_repair": repairWindow(&session.SaveData, npcID)}}
}

// handleSmithRepair mends one item. params: npc_id, slot_type, slot, gear.
// With no slot it just returns the window (to refresh it).
func handleSmithRepair(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	state := &session.SaveData
	npcID, _ := params["npc_id"].(string)
	deal, ok := condition.Repair().Smiths[npcID]
	if !ok || !npc.OnDuty(state, npcID, "open_shop") {
		return &GameActionResponse{Success: false, Message: "There's no one here to repair your gear.", Color: "yellow"}, nil
	}
	ref := slotRefFrom(params)
	if ref.Type == "" {
		return &GameActionResponse{Success: true, Data: map[string]interface{}{"repair": repairWindow(state, npcID)}}, nil
	}
	refuse := func(msg string) (*GameActionResponse, error) {
		return &GameActionResponse{Success: false, Message: msg, Color: "yellow",
			Data: map[string]interface{}{"repair": repairWindow(state, npcID)}}, nil
	}
	slot := inventory.SlotAt(state, ref)
	if slot == nil || slot["item"] == nil || slot["item"] == "" {
		return refuse("That's not there any more.")
	}
	id, _ := slot["item"].(string)
	value := 0
	if item, err := serverdb.GetItemByID(id); err == nil && item != nil {
		value = item.Value
	}
	q := condition.SmithQuote(deal, slot, value)
	switch {
	case q.CantWork:
		return refuse(fmt.Sprintf("\"I don't work %s.\"", condition.MaterialOf(id)))
	case q.Untouched:
		return refuse(fmt.Sprintf("\"Your %s's fine as it is.\"", condition.ItemName(id)))
	}
	gold, discounted := smithPrice(state, q)
	if gameutil.GetGoldQuantity(state) < gold {
		return refuse(fmt.Sprintf("That's %d gold. You don't have enough.", gold))
	}
	if q.Material != "" && gameutil.CountItem(state, q.Material) < q.Count {
		return refuse(fmt.Sprintf("You need %d %s for that.", q.Count, condition.ItemName(q.Material)))
	}
	gameutil.DeductGold(state, gold)
	paid := []string{fmt.Sprintf("%d gold", gold)}
	if q.Material != "" {
		gameutil.ConsumeItem(state, q.Material, q.Count)
		paid = append(paid, fmt.Sprintf("%d %s", q.Count, condition.ItemName(q.Material)))
	}
	if discounted {
		gameutil.ConsumeItem(state, q.Discount, 1)
		paid = append(paid, "1 "+condition.ItemName(q.Discount))
	}
	condition.SetWear(slot, q.ToWear)
	tier, _ := condition.WearTier(slot)
	return &GameActionResponse{Success: true, Color: "green",
		Message: fmt.Sprintf("🔨 Your %s is %s again. (Paid %s.)", condition.ItemName(id), strings.ToLower(tier.String()), strings.Join(paid, ", ")),
		Data:    map[string]interface{}{"repair": repairWindow(state, npcID)}}, nil
}

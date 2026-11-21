package game

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/types"
)

// ─── Enchanters (docs/draft/enchanting.md) ───────────────────────────────────
//
// An enchanter wakes the gem in a plain ring or necklace: the piece becomes
// its enchanted twin and wears the gem's effect. Each enchanter is an NPC
// with an enchant_config (which pieces, which gems, how big, their chance and
// their price) and "enchant" on their working shift. The fee is paid either
// way; a failure leaves the piece as it was, so trying again costs only fees.
// The chance is rolled here. Their menu gets "Enchant jewelry" while they're
// at work, and it opens the enchanting window (src/ui/enchanter.js).

const enchanterChoice = "gem:enchant"

// enchantRoll is the success roll (tests replace it).
var enchantRoll = rand.Float64

// enchantConfig is an NPC's enchant_config, or nil.
func enchantConfig(npcID string) *types.EnchantConfig {
	var props string
	if npcID == "" || serverdb.GetDB() == nil ||
		serverdb.GetDB().QueryRow("SELECT properties FROM npcs WHERE id = ?", npcID).Scan(&props) != nil {
		return nil
	}
	var n types.NPCData
	if json.Unmarshal([]byte(props), &n) != nil {
		return nil
	}
	return n.EnchantConfig
}

// isEnchanter reports whether an NPC enchants and is at work.
func isEnchanter(state *types.SaveFile, npcID string) bool {
	return enchantConfig(npcID) != nil && npc.OnDuty(state, npcID, "enchant")
}

// injectEnchanterOption adds "Enchant jewelry" to an enchanter's menu, before
// goodbye.
func injectEnchanterOption(resp *types.GameActionResponse, npcID string, state *types.SaveFile) {
	if resp == nil || resp.Delta == nil || !isEnchanter(state, npcID) {
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
	dlg["options"] = append(append(append([]string{}, options[:at]...), enchanterChoice), options[at:]...)
	dlg["option_labels"] = mergeLabels(dlg["option_labels"], map[string]string{enchanterChoice: "✨ Enchant jewelry"})
}

// openEnchanter answers "Enchant jewelry": close the dialogue, open the window.
func openEnchanter(session *GameSession, npcID string) *GameActionResponse {
	if !isEnchanter(&session.SaveData, npcID) {
		return fishRefusal("There's no one here to enchant anything.")
	}
	return &GameActionResponse{Success: true,
		Delta: map[string]interface{}{"npc_dialogue": map[string]interface{}{"action": "close"}},
		Data:  map[string]interface{}{"open_enchanter": enchanterWindow(&session.SaveData, npcID)}}
}

// enchantRow is one plain piece you carry, and what the enchanter would do
// with it.
type enchantRow struct {
	ItemID  string `json:"item_id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Have    int    `json:"have"`
	Refusal string `json:"refusal,omitempty"` // why they won't
	Fee     int    `json:"fee,omitempty"`
	Chance  int    `json:"chance,omitempty"` // percent
	Makes   string `json:"makes,omitempty"`  // the enchanted piece's name
	Effect  string `json:"effect,omitempty"` // what it would do for you
}

// enchanterWindow lists the plain rings and necklaces you carry.
func enchanterWindow(state *types.SaveFile, npcID string) map[string]interface{} {
	c := gems.Config()
	cfg := enchantConfig(npcID)
	rows := []enchantRow{}
	for _, id := range gameutil.CarriedItemIDs(state) {
		g, ok := gems.Parse(c, id)
		if !ok || g.Piece == "" || g.Enchanted {
			continue
		}
		row := enchantRow{ItemID: id, Name: gems.Name(c, g), Image: itemImage(id), Have: gameutil.CountItem(state, id),
			Refusal: gems.Refusal(c, g, cfg)}
		if row.Refusal == "" {
			made := g
			made.Enchanted = true
			row.Fee = gems.EnchantFee(c, g, cfg)
			row.Chance = int(math.Round(gems.EnchantChance(c, g, cfg) * 100))
			row.Makes = itemName(made.ID())
			row.Effect = gems.PieceDescription(c, g, state.Class)
		}
		rows = append(rows, row)
	}
	name := npcID
	if n, err := serverdb.GetNPCByID(npcID); err == nil && n != nil {
		name = n.Name
	}
	out := map[string]interface{}{"npc_id": npcID, "npc_name": name, "pieces": rows, "gold": gameutil.GetGoldQuantity(state)}
	if cfg != nil {
		out["max_size"], out["gems"], out["works"] = cfg.MaxSize, cfg.Gems, cfg.Pieces
	}
	return out
}

// itemName is an item's display name, or its id.
func itemName(itemID string) string {
	if it, err := serverdb.GetItemByID(itemID); err == nil && it != nil && it.Name != "" {
		return it.Name
	}
	return itemID
}

// handleEnchant does one enchanting. params: npc_id, item_id. With no item it
// returns the window.
func handleEnchant(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	state := &session.SaveData
	c := gems.Config()
	npcID, _ := params["npc_id"].(string)
	if !isEnchanter(state, npcID) {
		return fishRefusal("There's no one here to enchant anything."), nil
	}
	cfg := enchantConfig(npcID)
	itemID, _ := params["item_id"].(string)
	window := func() map[string]interface{} { return map[string]interface{}{"enchanter": enchanterWindow(state, npcID)} }
	refuse := func(msg string) (*GameActionResponse, error) {
		return &GameActionResponse{Success: false, Message: msg, Color: "yellow", Data: window()}, nil
	}
	if itemID == "" {
		return &GameActionResponse{Success: true, Data: window()}, nil
	}
	g, ok := gems.Parse(c, itemID)
	if !ok || g.Piece == "" {
		return refuse("That isn't a ring or a necklace with a stone in it.")
	}
	if why := gems.Refusal(c, g, cfg); why != "" {
		return refuse(why)
	}
	if gameutil.CountItem(state, itemID) < 1 {
		return refuse("You don't have that piece any more.")
	}
	fee := gems.EnchantFee(c, g, cfg)
	if gameutil.GetGoldQuantity(state) < fee {
		return refuse(fmt.Sprintf("The work is %d gold. You don't have enough.", fee))
	}
	// The piece goes on the table; it comes back as it was, or enchanted.
	if !gameutil.ConsumeItem(state, itemID, 1) {
		return refuse("You don't have that piece any more.")
	}
	gameutil.DeductGold(state, fee)
	made := g
	msg := fmt.Sprintf("✨ The stone wakes: %s. (Paid %d gold.)", itemName(g.ID()+types.EnchantedSuffix), fee)
	failed := enchantRoll() >= gems.EnchantChance(c, g, cfg)
	if failed {
		msg = fmt.Sprintf("✨ The working slides off your %s: the stone won't wake this time. Your piece is unharmed. (Paid %d gold.)", gems.Name(c, g), fee)
	} else {
		made.Enchanted = true
	}
	if added, _ := inventory.AddItemToInventory(state, made.ID(), 1); added == 0 {
		session.Ground.Add(groundSpot(session), made.ID(), 1)
		msg += " Your pack is full: it's on the ground."
	}
	if failed {
		return &GameActionResponse{Success: true, Color: "yellow", Message: msg,
			Data: map[string]interface{}{"enchanter": enchanterWindow(state, npcID), "enchant_failed": true}}, nil
	}
	return &GameActionResponse{Success: true, Color: "green", Message: msg,
		Data: map[string]interface{}{"enchanter": enchanterWindow(state, npcID), "enchanter_made": made.ID()}}, nil
}

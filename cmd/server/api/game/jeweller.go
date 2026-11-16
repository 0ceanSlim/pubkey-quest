package game

import (
	"encoding/json"
	"fmt"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/world"
	"pubkey-quest/types"
)

// ─── The jeweller (gem-system.md §3) ─────────────────────────────────────────
//
// The jeweller cuts rough gems and sets cut ones in gold. Cutting costs a fee
// by size and reveals the gem's quality, which is rolled here, at the wheel:
// no uncut gem carries one, so nothing hidden sits in the save. Setting takes
// the piece's gold plus labour on the gem's worth; the piece is worth both.
// Her menu gets "Cut or set gems" while she's at work, and it opens the bench
// window (src/ui/jeweller.js); the jeweller action does the work.

const jewellerChoice = "gem:bench"

// isJeweller reports whether an NPC is the jeweller and at work.
func isJeweller(state *types.SaveFile, npcID string) bool {
	return npcID != "" && npcID == gems.Config().Jeweller.NPC && npc.OnDuty(state, npcID, "open_shop")
}

// injectJewellerOption adds "Cut or set gems" to the jeweller's menu, before
// goodbye.
func injectJewellerOption(resp *types.GameActionResponse, npcID string, state *types.SaveFile) {
	if resp == nil || resp.Delta == nil || !isJeweller(state, npcID) {
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
	dlg["options"] = append(append(append([]string{}, options[:at]...), jewellerChoice), options[at:]...)
	dlg["option_labels"] = mergeLabels(dlg["option_labels"], map[string]string{jewellerChoice: "💎 Cut or set gems"})
}

// openJeweller answers "Cut or set gems": close the dialogue, open the bench.
func openJeweller(session *GameSession, npcID string) *GameActionResponse {
	if !isJeweller(&session.SaveData, npcID) {
		return fishRefusal("There's no one here to cut gems.")
	}
	return &GameActionResponse{Success: true,
		Delta: map[string]interface{}{"npc_dialogue": map[string]interface{}{"action": "close"}},
		Data:  map[string]interface{}{"open_jeweller": jewellerWindow(&session.SaveData, npcID)}}
}

// benchPiece is one setting the jeweller offers for a cut gem.
type benchPiece struct {
	Piece  string `json:"piece"`
	Name   string `json:"name"`
	ItemID string `json:"item_id"`
	Gold   int    `json:"gold"`
	Labour int    `json:"labour"`
	Value  int    `json:"value"`
}

// benchRow is one gem you carry, and what she'd do with it.
type benchRow struct {
	ItemID string       `json:"item_id"`
	Name   string       `json:"name"`
	Image  string       `json:"image"`
	Have   int          `json:"have"`
	Value  int          `json:"value"`
	Cut    bool         `json:"cut"`
	Fee    int          `json:"fee,omitempty"`    // to cut it
	Pieces []benchPiece `json:"pieces,omitempty"` // to set it
}

// itemImage is an item's sprite path.
func itemImage(itemID string) string {
	it, err := serverdb.GetItemByID(itemID)
	if err != nil || it == nil {
		return ""
	}
	var props struct {
		Image string `json:"image"`
	}
	_ = json.Unmarshal([]byte(it.Properties), &props)
	return props.Image
}

// jewellerWindow lists the gems you carry: rough ones to cut, cut ones to set.
func jewellerWindow(state *types.SaveFile, npcID string) map[string]interface{} {
	c := gems.Config()
	rough, cut := []benchRow{}, []benchRow{}
	for _, id := range gameutil.CarriedItemIDs(state) {
		g, ok := gems.Parse(c, id)
		if !ok || g.Piece != "" {
			continue
		}
		row := benchRow{ItemID: id, Name: gems.Name(c, g), Image: itemImage(id), Have: gameutil.CountItem(state, id),
			Value: gems.Value(c, g), Cut: g.Cut()}
		if !g.Cut() {
			row.Fee = gems.CutFee(c, g.Size)
			rough = append(rough, row)
			continue
		}
		for _, p := range types.GemPieces {
			if c.Jeweller.Pieces[p].Gold <= 0 {
				continue
			}
			set := g
			set.Piece = p
			gold, labour := gems.SetFee(c, g, p)
			row.Pieces = append(row.Pieces, benchPiece{Piece: p, Name: c.Jeweller.Pieces[p].Name, ItemID: set.ID(),
				Gold: gold, Labour: labour, Value: gems.Value(c, set)})
		}
		cut = append(cut, row)
	}
	name := npcID
	if n, err := serverdb.GetNPCByID(npcID); err == nil && n != nil {
		name = n.Name
	}
	return map[string]interface{}{
		"npc_id": npcID, "npc_name": name, "rough": rough, "cut": cut,
		"gold": gameutil.GetGoldQuantity(state),
	}
}

// handleJeweller does one job at the bench. params: npc_id, op ("cut" or
// "set"), item_id, and piece for a setting. With no op it returns the window.
func handleJeweller(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	state := &session.SaveData
	c := gems.Config()
	npcID, _ := params["npc_id"].(string)
	if !isJeweller(state, npcID) {
		return fishRefusal("There's no one here to cut gems."), nil
	}
	op, _ := params["op"].(string)
	itemID, _ := params["item_id"].(string)
	refuse := func(msg string) (*GameActionResponse, error) {
		return &GameActionResponse{Success: false, Message: msg, Color: "yellow",
			Data: map[string]interface{}{"jeweller": jewellerWindow(state, npcID)}}, nil
	}
	if op == "" {
		return &GameActionResponse{Success: true, Data: map[string]interface{}{"jeweller": jewellerWindow(state, npcID)}}, nil
	}
	g, ok := gems.Parse(c, itemID)
	if !ok || g.Piece != "" {
		return refuse("That isn't a gem.")
	}
	if gameutil.CountItem(state, itemID) < 1 {
		return refuse("You don't have that gem any more.")
	}
	var made gems.Gem
	var paid int
	var msg string
	switch op {
	case "cut":
		if g.Cut() {
			return refuse("That one's already cut.")
		}
		paid = gems.CutFee(c, g.Size)
		if gameutil.GetGoldQuantity(state) < paid {
			return refuse(fmt.Sprintf("Cutting it is %d gold. You don't have enough.", paid))
		}
		made = g
		made.Quality = gems.RollQuality(c, gems.Global)
		msg = cutLine(c, g, made)
	case "set":
		piece, _ := params["piece"].(string)
		if !g.Cut() {
			return refuse("It has to be cut before it can be set.")
		}
		if c.Jeweller.Pieces[piece].Gold <= 0 {
			return refuse("She doesn't make those.")
		}
		gold, labour := gems.SetFee(c, g, piece)
		paid = gold + labour
		if gameutil.GetGoldQuantity(state) < paid {
			return refuse(fmt.Sprintf("That's %d gold for the %s and %d for the work. You don't have enough.",
				gold, c.Jeweller.Pieces[piece].Name, labour))
		}
		made = g
		made.Piece = piece
		msg = fmt.Sprintf("💎 She sets your %s in gold: %s, worth %d gold.", gems.Name(c, g), gems.Name(c, made), gems.Value(c, made))
	default:
		return refuse("She can cut a gem or set one.")
	}
	if !gameutil.ConsumeItem(state, itemID, 1) {
		return refuse("You don't have that gem any more.")
	}
	gameutil.DeductGold(state, paid)
	if added, _ := inventory.AddItemToInventory(state, made.ID(), 1); added == 0 {
		session.Ground.Add(world.GroundKey(state), made.ID(), 1)
		msg += " Your pack is full: it's on the ground."
	}
	return &GameActionResponse{Success: true, Color: "green", Message: fmt.Sprintf("%s (Paid %d gold.)", msg, paid),
		Data: map[string]interface{}{"jeweller": jewellerWindow(state, npcID), "jeweller_made": made.ID()}}, nil
}

// cutLine is how a cut turned out.
func cutLine(c *types.GemsConfig, rough, cut gems.Gem) string {
	value := gems.Value(c, cut)
	switch cut.Quality {
	case "flawed":
		return fmt.Sprintf("💎 The wheel finds a flaw running through your %s. Flawed, worth %d gold.", gems.Name(c, rough), value)
	case "fine":
		return fmt.Sprintf("💎 Your %s comes off the wheel bright and clean: Fine, worth %d gold!", gems.Name(c, rough), value)
	case "flawless":
		return fmt.Sprintf("💎 She holds your cut %s up to the light, and goes very quiet. Flawless, worth %d gold!", gems.Name(c, rough), value)
	}
	return fmt.Sprintf("💎 She cuts your %s: a good, standard stone, worth %d gold.", gems.Name(c, rough), value)
}

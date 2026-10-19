package game

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/types"
)

// Labelled dialogue options: an NPC's node can list options as objects
// instead of node keys —
//
//	{"label": "[Orc] Brother, what brought you here?", "next": "orc_brotherhood",
//	 "requirements": [{"type": "race", "values": ["Orc"]}],
//	 "cost": {"items": [{"id": "rations", "quantity": 1}]}}
//
// — gated by the shared requirement evaluator (race, class, stat, skill,
// level, items, quests…) and optionally paid for. The npc package reads only
// plain keys; this layer adds the labelled ones under "opt:<node>:<index>" keys
// with their labels, and resolves a chosen one to its next node.
//
// A node may also carry a one-time "reward" (xp, gold, items): it's granted
// the first time a character reaches that node, and remembered in
// SaveFile.DialogueGifts so asking again doesn't pay out again.

const optKeyPrefix = "opt:"

func optKey(node string, i int) string { return fmt.Sprintf("%s%s:%d", optKeyPrefix, node, i) }

func parseOptKey(key string) (node string, i int, ok bool) {
	if !strings.HasPrefix(key, optKeyPrefix) {
		return "", 0, false
	}
	rest := strings.TrimPrefix(key, optKeyPrefix)
	cut := strings.LastIndex(rest, ":")
	if cut < 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(rest[cut+1:])
	if err != nil {
		return "", 0, false
	}
	return rest[:cut], n, true
}

// dialogueOption is one labelled option on a node.
type dialogueOption struct {
	Label        string
	Next         string
	Requirements []types.POIRequirement
	Cost         types.POICost
}

func parseOption(raw interface{}) (dialogueOption, bool) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return dialogueOption{}, false
	}
	var o dialogueOption
	o.Label, _ = m["label"].(string)
	o.Next, _ = m["next"].(string)
	if o.Label == "" || o.Next == "" {
		return dialogueOption{}, false
	}
	if reqs, ok := m["requirements"].([]interface{}); ok {
		for _, r := range reqs {
			rm, ok := r.(map[string]interface{})
			if !ok {
				continue
			}
			var req types.POIRequirement
			req.Type, _ = rm["type"].(string)
			req.ID, _ = rm["id"].(string)
			if v, ok := rm["min"].(float64); ok {
				req.Min = int(v)
			}
			if vs, ok := rm["values"].([]interface{}); ok {
				for _, v := range vs {
					if s, ok := v.(string); ok {
						req.Values = append(req.Values, s)
					}
				}
			}
			o.Requirements = append(o.Requirements, req)
		}
	}
	if cost, ok := m["cost"].(map[string]interface{}); ok {
		if g, ok := cost["gold"].(float64); ok {
			o.Cost.Gold = int(g)
		}
		for _, it := range npc.ParseItemRequirements(cost["items"]) {
			o.Cost.Items = append(o.Cost.Items, types.POIRewardItem{ID: it.ID, Quantity: it.Quantity})
		}
	}
	return o, true
}

// nodeOptions is the raw option list of an NPC's dialogue node.
func nodeOptions(npcData *serverdb.NPCData, node string) []interface{} {
	n, _ := npcData.Dialogue[node].(map[string]interface{})
	opts, _ := n["options"].([]interface{})
	return opts
}

// labelledOption looks up a chosen "opt:" key.
func labelledOption(npcData *serverdb.NPCData, key string) (dialogueOption, bool) {
	node, i, ok := parseOptKey(key)
	if !ok {
		return dialogueOption{}, false
	}
	opts := nodeOptions(npcData, node)
	if i < 0 || i >= len(opts) {
		return dialogueOption{}, false
	}
	return parseOption(opts[i])
}

// addLabelledOptions rebuilds the shown node's option list in authored order:
// the plain keys the npc package already let through, plus the labelled
// options this player qualifies for.
func addLabelledOptions(resp *types.GameActionResponse, npcID string, state *types.SaveFile) {
	if resp == nil || !resp.Success || resp.Delta == nil {
		return
	}
	dlg, ok := resp.Delta["npc_dialogue"].(map[string]interface{})
	if !ok {
		return
	}
	node, _ := dlg["node"].(string)
	npcData, err := serverdb.GetNPCByID(npcID)
	if node == "" || err != nil || npcData == nil {
		return
	}
	raw := nodeOptions(npcData, node)
	hasLabelled := false
	for _, o := range raw {
		if _, ok := o.(map[string]interface{}); ok {
			hasLabelled = true
			break
		}
	}
	if !hasLabelled {
		return
	}
	plain, _ := dlg["options"].([]string)
	ctx := buildQuestContext(state)
	var out []string
	labels := map[string]string{}
	for i, o := range raw {
		if s, ok := o.(string); ok {
			if slices.Contains(plain, s) {
				out = append(out, s)
			}
			continue
		}
		opt, ok := parseOption(o)
		if !ok || !requirement.Evaluate(opt.Requirements, ctx).OK {
			continue
		}
		// An option that leads to a one-time gift this character already got
		// (a keeper's free Homeward Stone) has nothing left to give: hide it.
		if giftClaimed(npcData, npcID, opt.Next, state) {
			continue
		}
		k := optKey(node, i)
		out = append(out, k)
		labels[k] = opt.Label
	}
	dlg["options"] = out
	dlg["option_labels"] = mergeLabels(dlg["option_labels"], labels)
}

// payOptionCost takes an option's cost, or says what's missing.
func payOptionCost(state *types.SaveFile, cost types.POICost) string {
	if cost.Gold > 0 && gameutil.GetGoldQuantity(state) < cost.Gold {
		return fmt.Sprintf("You need %d gold.", cost.Gold)
	}
	for _, it := range cost.Items {
		qty := max(it.Quantity, 1)
		if gameutil.CountItem(state, it.ID) < qty {
			name := it.ID
			if item, err := serverdb.GetItemByID(it.ID); err == nil && item != nil && item.Name != "" {
				name = item.Name
			}
			return fmt.Sprintf("You need %d %s.", qty, name)
		}
	}
	if cost.Gold > 0 {
		gameutil.DeductGold(state, cost.Gold)
	}
	for _, it := range cost.Items {
		gameutil.ConsumeItem(state, it.ID, max(it.Quantity, 1))
	}
	return ""
}

// giftClaimed reports whether a dialogue node carries a one-time reward this
// character has already been given.
func giftClaimed(npcData *serverdb.NPCData, npcID, node string, state *types.SaveFile) bool {
	n, _ := npcData.Dialogue[node].(map[string]interface{})
	if _, ok := n["reward"].(map[string]interface{}); !ok {
		return false
	}
	return slices.Contains(state.DialogueGifts, npcID+":"+node)
}

// grantDialogueGift pays a node's one-time reward the first time this
// character reaches it.
func grantDialogueGift(state *types.SaveFile, npcID, node string) []string {
	npcData, err := serverdb.GetNPCByID(npcID)
	if err != nil || npcData == nil {
		return nil
	}
	n, _ := npcData.Dialogue[node].(map[string]interface{})
	rawReward, ok := n["reward"].(map[string]interface{})
	if !ok {
		return nil
	}
	key := npcID + ":" + node
	if slices.Contains(state.DialogueGifts, key) {
		return nil
	}
	var reward types.POIReward
	if xp, ok := rawReward["xp"].(float64); ok {
		reward.XP = int(xp)
	}
	if g, ok := rawReward["gold"].(float64); ok {
		reward.Gold = int(g)
	}
	for _, it := range npc.ParseItemRequirements(rawReward["items"]) {
		reward.Items = append(reward.Items, types.POIRewardItem{ID: it.ID, Quantity: it.Quantity})
	}
	advancement, err := loadAdvancement()
	if err != nil {
		return nil
	}
	quest.GrantReward(state, &reward, advancement)
	state.DialogueGifts = append(state.DialogueGifts, key)

	var got []string
	for _, it := range reward.Items {
		name := it.ID
		if item, err := serverdb.GetItemByID(it.ID); err == nil && item != nil && item.Name != "" {
			name = item.Name
		}
		got = append(got, fmt.Sprintf("%d× %s", max(it.Quantity, 1), name))
	}
	if reward.Gold > 0 {
		got = append(got, fmt.Sprintf("%d gold", reward.Gold))
	}
	if reward.XP > 0 {
		got = append(got, fmt.Sprintf("%d XP", reward.XP))
	}
	log.Printf("🎁 %s gave %v (%s)", npcID, got, key)
	return got
}

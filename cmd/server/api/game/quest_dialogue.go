package game

import (
	"fmt"
	"strings"
	"time"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// ─── Quest dialogue ──────────────────────────────────────────────────────────
//
// Quests are taken up in conversation. Each quest's dialogue lives in its own
// JSON (types.QuestDialogue) as up to three trees — offer, active, completed —
// and the game adds the tree matching the quest's state to its giver's menu as
// one ordinary option. Inside a tree every node is the player's line (label),
// the giver's reply (text) and where it can go next; an "accept" node starts the
// quest. A node with no options ends the thread: the player can ask about
// something else or say goodbye.
//
// Choices travel through the normal npc_dialogue_choice action under keys
// shaped "quest:<quest>:<phase>:<node>", so the NPC's own dialogue is untouched.

const (
	questKeyPrefix = "quest:"
	nodeBack       = "@back" // return to the giver's main menu
	nodeEnd        = "@end"  // say goodbye
)

func questKey(questID, phase, node string) string {
	return questKeyPrefix + questID + ":" + phase + ":" + node
}

// parseQuestKey splits a quest dialogue key; ok is false for any other choice.
func parseQuestKey(key string) (questID, phase, node string, ok bool) {
	if !strings.HasPrefix(key, questKeyPrefix) {
		return "", "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(key, questKeyPrefix), ":", 3)
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], parts[2], true
}

// questGiver is the NPC who talks about a quest ("" when nobody in particular).
func questGiver(qd *types.QuestData) string {
	if qd.Dialogue != nil && qd.Dialogue.Giver != "" {
		return qd.Dialogue.Giver
	}
	if qd.StartCondition.Type == "talk" {
		return qd.StartCondition.Target
	}
	return ""
}

// questPhase picks the conversation this NPC has about a quest right now —
// "offer", "active" or "completed" — and its tree, or "" when there's nothing to say.
func questPhase(qd *types.QuestData, npcData *serverdb.NPCData, all []types.QuestData, save *types.SaveFile, ctx requirement.Context, now time.Time) (string, *types.QuestDialogueTree) {
	giver := questGiver(qd)

	if quest.IsRepeatable(qd.Category) {
		// Dailies and weeklies: every vault keeper offers the current pick.
		if qd.StartCondition.Type != "vault_keeper" || len(npcData.StorageConfig) == 0 {
			return "", nil
		}
		if quest.RepeatableAvailable(*qd, save, all, now) && requirement.Evaluate(qd.Requirements, ctx).OK {
			return "offer", offerTree(qd)
		}
		return "", nil
	}

	if giver == "" || giver != npcData.ID {
		return "", nil
	}
	switch {
	case quest.IsActive(save, qd.ID):
		if qd.Dialogue != nil && qd.Dialogue.Active != nil {
			return "active", qd.Dialogue.Active
		}
	case quest.IsCompleted(save, qd.ID):
		if qd.Dialogue != nil && qd.Dialogue.Completed != nil {
			return "completed", qd.Dialogue.Completed
		}
	case quest.CanStart(*qd, save, ctx):
		return "offer", offerTree(qd)
	}
	return "", nil
}

// treeFor returns a quest's tree for a phase (generated for an unwritten offer).
func treeFor(qd *types.QuestData, phase string) *types.QuestDialogueTree {
	switch phase {
	case "offer":
		return offerTree(qd)
	case "active":
		if qd.Dialogue != nil {
			return qd.Dialogue.Active
		}
	case "completed":
		if qd.Dialogue != nil {
			return qd.Dialogue.Completed
		}
	}
	return nil
}

// offerTree is the quest's authored offer, or a plain one built from its
// description so a quest without written dialogue can still be taken up.
func offerTree(qd *types.QuestData) *types.QuestDialogueTree {
	if qd.Dialogue != nil && qd.Dialogue.Offer != nil {
		return qd.Dialogue.Offer
	}
	text := qd.Description
	if text == "" {
		text = "There's something I could use a hand with."
	}
	return &types.QuestDialogueTree{
		Option: "Ask about: " + qd.Name,
		Start:  "offer",
		Nodes: map[string]types.QuestDialogueNode{
			"offer":   {Text: text, Options: []string{"accept", "decline"}},
			"accept":  {Label: "I'll do it.", Text: "Then it's settled. Good luck out there.", Action: "accept"},
			"decline": {Label: "Not right now.", Text: "Come back if you change your mind."},
		},
	}
}

// injectQuestOptions adds a menu option for every quest this NPC has something
// to say about. Runs on the giver's main menu (talking to them, or coming back
// to it from a quest thread). The options go in before a closing "goodbye"
// option so the farewell stays last.
func injectQuestOptions(resp *types.GameActionResponse, npcID string, state *types.SaveFile) {
	if resp == nil || resp.Delta == nil {
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
	all, err := serverdb.GetAllQuests()
	if err != nil {
		return
	}
	ctx := buildQuestContext(state)
	now := time.Now()

	var keys []string
	labels := map[string]string{}
	for i := range all {
		phase, tree := questPhase(&all[i], npcData, all, state, ctx, now)
		if tree == nil {
			continue
		}
		k := questKey(all[i].ID, phase, tree.Start)
		keys = append(keys, k)
		labels[k] = tree.Option
	}
	if len(keys) == 0 {
		return
	}

	options, _ := dlg["options"].([]string)
	insertAt := len(options)
	if insertAt > 0 && closesDialogue(npcData, options[insertAt-1]) {
		insertAt--
	}
	merged := append(append(append([]string{}, options[:insertAt]...), keys...), options[insertAt:]...)
	dlg["options"] = merged
	dlg["option_labels"] = mergeLabels(dlg["option_labels"], labels)
}

// closesDialogue reports whether an NPC dialogue node ends the conversation.
func closesDialogue(npcData *serverdb.NPCData, key string) bool {
	node, _ := npcData.Dialogue[key].(map[string]interface{})
	action, _ := node["action"].(string)
	return action == "end_dialogue"
}

func mergeLabels(existing interface{}, add map[string]string) map[string]string {
	out := map[string]string{}
	if m, ok := existing.(map[string]string); ok {
		for k, v := range m {
			out[k] = v
		}
	}
	for k, v := range add {
		out[k] = v
	}
	return out
}

// handleQuestDialogueChoice resolves a choice inside a quest conversation.
func handleQuestDialogueChoice(sess *session.GameSession, npcID, choice string) (*types.GameActionResponse, error) {
	state := &sess.SaveData
	questID, phase, nodeID, ok := parseQuestKey(choice)
	if !ok {
		return nil, fmt.Errorf("invalid dialogue choice: %s", choice)
	}

	switch nodeID {
	case nodeEnd:
		return &types.GameActionResponse{
			Success: true, Color: "yellow",
			Delta: map[string]interface{}{"npc_dialogue": map[string]interface{}{"action": "close"}},
		}, nil
	case nodeBack:
		return giverMenu(state, npcID)
	}

	qd, err := serverdb.GetQuestByID(questID)
	if err != nil || qd == nil {
		return nil, fmt.Errorf("unknown quest %q", questID)
	}
	npcData, err := serverdb.GetNPCByID(npcID)
	if err != nil || npcData == nil {
		return nil, fmt.Errorf("NPC not found: %s", npcID)
	}

	// The conversation must still be one this NPC is having about this quest —
	// so a stale or forged key can't, say, accept a quest that isn't on offer.
	all, _ := serverdb.GetAllQuests()
	ctx := buildQuestContext(state)
	livePhase, tree := questPhase(qd, npcData, all, state, ctx, time.Now())
	if tree == nil || livePhase != phase {
		return threadEnded(npcID, questID, phase, "They've nothing more to say about that."), nil
	}
	node, ok := tree.Nodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("unknown dialogue node %q in %s", nodeID, questID)
	}

	resp := &types.GameActionResponse{Success: true, Color: "yellow"}
	text := node.Text

	switch node.Action {
	case "accept":
		if phase != "offer" {
			return nil, fmt.Errorf("can't accept %s from the %s conversation", questID, phase)
		}
		if err := quest.Accept(*qd, state, ctx); err != nil {
			return threadEnded(npcID, questID, phase, "Perhaps another time."), nil
		}
		// Accepting is itself a conversation with the giver — record it so a
		// first-stage "talk to them" objective doesn't need a second visit.
		events.Record(state, events.NPCTalked, npcID, 1)
		// The giver's reply is the line; the client announces the start itself.
		resp.Data = map[string]interface{}{"quest_accepted": qd.ID, "quest_name": qd.Name}
	case "close":
		resp.Message = text
		resp.Delta = map[string]interface{}{"npc_dialogue": map[string]interface{}{"action": "close"}}
		return resp, nil
	}

	// Where the thread can go next: the node's options the player qualifies for,
	// or — at the end of a thread — back to the menu or goodbye.
	var keys []string
	labels := map[string]string{}
	for _, next := range node.Options {
		n, ok := tree.Nodes[next]
		if !ok || !requirement.Evaluate(n.Requirements, ctx).OK {
			continue
		}
		k := questKey(questID, phase, next)
		keys = append(keys, k)
		labels[k] = n.Label
		if labels[k] == "" {
			labels[k] = strings.ReplaceAll(next, "-", " ")
		}
	}
	if len(keys) == 0 {
		keys, labels = threadEndOptions(questID, phase)
	}

	resp.Delta = map[string]interface{}{
		"npc_dialogue": map[string]interface{}{
			"npc_id":        npcID,
			"node":          choice,
			"text":          text,
			"options":       keys,
			"option_labels": labels,
		},
	}
	if resp.Message == "" {
		resp.Message = text
	}
	return resp, nil
}

// threadEndOptions are the ways out at the end of a quest thread.
func threadEndOptions(questID, phase string) ([]string, map[string]string) {
	back, end := questKey(questID, phase, nodeBack), questKey(questID, phase, nodeEnd)
	return []string{back, end}, map[string]string{back: "Ask about something else", end: "Goodbye"}
}

// threadEnded answers a choice whose conversation no longer applies.
func threadEnded(npcID, questID, phase, text string) *types.GameActionResponse {
	keys, labels := threadEndOptions(questID, phase)
	return &types.GameActionResponse{
		Success: true, Message: text, Color: "yellow",
		Delta: map[string]interface{}{"npc_dialogue": map[string]interface{}{
			"npc_id": npcID, "text": text, "options": keys, "option_labels": labels,
		}},
	}
}

// giverMenu returns to the NPC's main menu (without their greeting again),
// with its quest options.
func giverMenu(state *types.SaveFile, npcID string) (*types.GameActionResponse, error) {
	resp, err := npc.HandleTalkToNPCAction(state, map[string]interface{}{"npc_id": npcID})
	if err != nil || resp == nil || !resp.Success {
		return resp, err
	}
	injectQuestOptions(resp, npcID, state)
	if dlg, ok := resp.Delta["npc_dialogue"].(map[string]interface{}); ok {
		if text, _ := dlg["text"].(string); text != "" {
			resp.Message = text
		}
	}
	return resp, nil
}

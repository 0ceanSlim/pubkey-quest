package game

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// bazaarSession is a player at the Golden Bazaar at noon, where Nissa works.
func bazaarSession(t *testing.T, slots ...map[string]interface{}) *session.GameSession {
	t.Helper()
	sess := forgeSession(t, slots...)
	state := &sess.SaveData
	state.Location, state.District, state.Building = "goldenhaven", "center", "luxury_market"
	return sess
}

func bench(t *testing.T, sess *session.GameSession, params map[string]any) *GameActionResponse {
	t.Helper()
	params["npc_id"] = "gem-cutter"
	return fishAction(t, sess, "jeweller", params)
}

// commissionTalk talks to Nissa and returns her menu's options.
func commissionTalk(t *testing.T, sess *session.GameSession) []string {
	t.Helper()
	r := fishAction(t, sess, "talk_to_npc", map[string]any{"npc_id": "gem-cutter"})
	dlg, _ := r.Delta["npc_dialogue"].(map[string]interface{})
	opts, _ := dlg["options"].([]string)
	return opts
}

// Cutting reveals a quality and costs the size's fee; setting takes the gold
// and the labour and makes the piece.
func TestCutAndSetAGem(t *testing.T) {
	rough := types.UncutGemID("ruby", "small")
	sess := bazaarSession(t, carrying(rough, 1), carrying("gold-piece", 30000))
	state := &sess.SaveData

	r := commissionTalk(t, sess)
	if !strings.Contains(strings.Join(r, "|"), jewellerChoice) {
		t.Fatalf("her menu has no bench option: %v", r)
	}

	if r := bench(t, sess, map[string]any{"op": "cut", "item_id": rough}); !r.Success {
		t.Fatalf("cut: %+v", r)
	}
	if gameutil.CountItem(state, rough) != 0 || gameutil.GetGoldQuantity(state) != 30000-200 {
		t.Fatalf("after the cut: rough %d, gold %d", gameutil.CountItem(state, rough), gameutil.GetGoldQuantity(state))
	}
	var cut gems.Gem
	for _, q := range []string{"flawed", "standard", "fine", "flawless"} {
		if gameutil.CountItem(state, types.CutGemID("ruby", "small", q)) == 1 {
			cut = gems.Gem{Type: "ruby", Size: "small", Quality: q}
		}
	}
	if !cut.Cut() {
		t.Fatal("no cut ruby in the pack")
	}

	gold, labour := gems.SetFee(gems.Config(), cut, "ring")
	before := gameutil.GetGoldQuantity(state)
	if r := bench(t, sess, map[string]any{"op": "set", "item_id": cut.ID(), "piece": "ring"}); !r.Success {
		t.Fatalf("set: %+v", r)
	}
	ring := cut
	ring.Piece = "ring"
	if gameutil.CountItem(state, ring.ID()) != 1 || gameutil.CountItem(state, cut.ID()) != 0 ||
		gameutil.GetGoldQuantity(state) != before-gold-labour {
		t.Fatalf("after setting: ring %d, gold %d (want %d)", gameutil.CountItem(state, ring.ID()),
			gameutil.GetGoldQuantity(state), before-gold-labour)
	}
}

// She won't work after hours, on what isn't a gem, or for less than her fee.
func TestTheBenchRefuses(t *testing.T) {
	rough := types.UncutGemID("quartz", "huge")
	sess := bazaarSession(t, carrying(rough, 1), carrying("gold-piece", 100))
	if r := bench(t, sess, map[string]any{"op": "cut", "item_id": rough}); r.Success || gameutil.CountItem(&sess.SaveData, rough) != 1 {
		t.Fatalf("cut a huge gem for 100 gold: %+v", r)
	}
	if r := bench(t, sess, map[string]any{"op": "cut", "item_id": "gold-piece"}); r.Success {
		t.Fatalf("cut a coin: %+v", r)
	}
	if r := bench(t, sess, map[string]any{"op": "set", "item_id": rough, "piece": "ring"}); r.Success {
		t.Fatalf("set a rough gem: %+v", r)
	}
	sess.SaveData.TimeOfDay = 1300
	if r := bench(t, sess, map[string]any{}); r.Success {
		t.Fatalf("the bench after hours: %+v", r)
	}
}

// A pick cracks a geode for one rough gem, in game time, and wears a little.
func TestCrackAGeode(t *testing.T) {
	sess := bazaarSession(t, carrying(types.GeodeID, 2), map[string]interface{}{"item": "pick-miners", "quantity": 1})
	state := &sess.SaveData
	r := fishAction(t, sess, "use_targets", map[string]any{"item_id": "pick-miners"})
	targets, _ := r.Data["targets"].([]inventory.UseTarget)
	if len(targets) != 1 || targets[0].ItemID != types.GeodeID {
		t.Fatalf("targets: %+v", r.Data)
	}
	minute := state.TimeOfDay
	r = fishAction(t, sess, "use_on", map[string]any{"item_id": "pick-miners", "slot_type": targets[0].Type, "slot": targets[0].Index})
	if !r.Success || gameutil.CountItem(state, types.GeodeID) != 1 || state.TimeOfDay <= minute {
		t.Fatalf("crack: %+v", r)
	}
	found := 0
	for _, id := range gameutil.CarriedItemIDs(state) {
		if g, ok := gems.Parse(gems.Config(), id); ok && !g.Cut() {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("expected one rough gem, found %d: %s", found, r.Message)
	}
}

// A wielded dagger shucks an oyster; the pearl inside comes with its quality,
// and Nissa sets it without cutting it.
func TestShuckAndSetAPearl(t *testing.T) {
	old := gems.Config()
	sure := *old
	sure.Pearls.Shellfish = map[string]types.GemShellfish{"oyster": {Chance: 1, Types: map[string]int{"black-pearl": 1}, SizeTilt: 1, MaxSize: "tiny"}}
	gems.SetConfig(&sure)
	defer gems.SetConfig(old)

	sess := bazaarSession(t, carrying("oyster", 1), carrying("gold-piece", 10000))
	state := &sess.SaveData
	gear := state.Inventory["gear_slots"].(map[string]interface{})
	gear["mainhand"] = map[string]interface{}{"item": "dagger", "quantity": 1}
	r := fishAction(t, sess, "use_targets", map[string]any{"item_id": "dagger"})
	targets, _ := r.Data["targets"].([]inventory.UseTarget)
	if len(targets) != 1 || targets[0].ItemID != "oyster" {
		t.Fatalf("targets: %+v", r.Data)
	}
	r = fishAction(t, sess, "use_on", map[string]any{"item_id": "dagger", "slot_type": targets[0].Type, "slot": targets[0].Index})
	if !r.Success || gameutil.CountItem(state, "oyster") != 0 || !strings.Contains(r.Message, "black pearl") {
		t.Fatalf("shuck: %+v", r)
	}
	var pearl string
	for _, id := range gameutil.CarriedItemIDs(state) {
		if strings.HasPrefix(id, "gem-black-pearl-tiny-") {
			pearl = id
		}
	}
	if pearl == "" {
		t.Fatalf("no pearl: %v", gameutil.CarriedItemIDs(state))
	}
	if r := bench(t, sess, map[string]any{"op": "cut", "item_id": pearl}); r.Success {
		t.Fatalf("cut a pearl: %s", r.Message)
	}
	if r := bench(t, sess, map[string]any{"op": "set", "item_id": pearl, "piece": "ring"}); !r.Success {
		t.Fatalf("set: %s", r.Message)
	}
	if gameutil.CountItem(state, "ring"+strings.TrimPrefix(pearl, "gem")) != 1 {
		t.Fatalf("no ring: %v", gameutil.CarriedItemIDs(state))
	}
}

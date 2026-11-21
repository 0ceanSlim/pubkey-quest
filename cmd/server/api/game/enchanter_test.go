package game

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// templeSession is a player in the Dawnflame Temple at noon, where Seraphina
// works.
func templeSession(t *testing.T, slots ...map[string]interface{}) *session.GameSession {
	t.Helper()
	sess := forgeSession(t, slots...)
	sess.SaveData.Location, sess.SaveData.Building = "goldenhaven", "dawnflame_temple"
	return sess
}

func enchantAt(t *testing.T, sess *session.GameSession, npcID, itemID string) *GameActionResponse {
	t.Helper()
	return fishAction(t, sess, "enchant", map[string]any{"npc_id": npcID, "item_id": itemID})
}

// rollEnchant fixes the success roll for a test.
func rollEnchant(t *testing.T, r float64) {
	t.Helper()
	old := enchantRoll
	enchantRoll = func() float64 { return r }
	t.Cleanup(func() { enchantRoll = old })
}

// Seraphina enchants a ring: a failure costs the fee and leaves the piece; a
// success makes the enchanted ring, whose effect comes on when it's worn.
func TestEnchantARing(t *testing.T) {
	plain := types.GemPieceID("ring", "quartz", "medium", "standard")
	sess := templeSession(t, carrying(plain, 1), carrying("gold-piece", 20000))
	state := &sess.SaveData

	r := fishAction(t, sess, "talk_to_npc", map[string]any{"npc_id": "oracle-seraphina"})
	dlg, _ := r.Delta["npc_dialogue"].(map[string]interface{})
	opts, _ := dlg["options"].([]string)
	if !strings.Contains(strings.Join(opts, "|"), enchanterChoice) {
		t.Fatalf("her menu has no enchanting option: %v", opts)
	}
	if r := fishAction(t, sess, "npc_dialogue_choice", map[string]any{"npc_id": "oracle-seraphina", "choice": enchanterChoice}); r.Data["open_enchanter"] == nil {
		t.Fatalf("choosing it opens no window: %+v", r)
	}

	fee := 2500 * 2 // a medium stone at Seraphina's ×2
	rollEnchant(t, 0.99)
	if r := enchantAt(t, sess, "oracle-seraphina", plain); !r.Success || r.Data["enchant_failed"] != true {
		t.Fatalf("a failed enchanting: %+v", r)
	}
	if gameutil.CountItem(state, plain) != 1 || gameutil.GetGoldQuantity(state) != 20000-fee {
		t.Fatalf("a failure costs the fee and keeps the piece: ring %d, gold %d", gameutil.CountItem(state, plain), gameutil.GetGoldQuantity(state))
	}

	rollEnchant(t, 0)
	if r := enchantAt(t, sess, "oracle-seraphina", plain); !r.Success || r.Data["enchanter_made"] != plain+types.EnchantedSuffix {
		t.Fatalf("enchanting: %+v", r)
	}
	if gameutil.CountItem(state, plain) != 0 || gameutil.CountItem(state, plain+types.EnchantedSuffix) != 1 ||
		gameutil.GetGoldQuantity(state) != 20000-2*fee {
		t.Fatalf("after: plain %d, enchanted %d, gold %d", gameutil.CountItem(state, plain),
			gameutil.CountItem(state, plain+types.EnchantedSuffix), gameutil.GetGoldQuantity(state))
	}

	state.Stats["intelligence"] = 12
	if r := wear(t, sess, plain+types.EnchantedSuffix, "ring1"); !r.Success {
		t.Fatalf("equip: %+v", r)
	}
	if got := effects.Ability(state, "intelligence"); got != 13 {
		t.Fatalf("INT %d with the ring on, want 13", got)
	}
}

// Enchanters keep to their trade: Brunhild works rings, not necklaces, and
// her own stones; nobody re-enchants an enchanted piece; nobody works off
// shift.
func TestEnchantersKeepToTheirTrade(t *testing.T) {
	necklace := types.GemPieceID("necklace", "ruby", "small", "standard")
	quartz := types.GemPieceID("ring", "quartz", "small", "standard")
	done := types.GemPieceID("ring", "ruby", "small", "standard") + types.EnchantedSuffix
	sess := forgeSession(t, carrying(necklace, 1), carrying(quartz, 1), carrying(done, 1), carrying("gold-piece", 20000))
	state := &sess.SaveData
	rollEnchant(t, 0)
	for _, id := range []string{necklace, quartz, done} {
		if r := enchantAt(t, sess, "forgemistress", id); r.Success {
			t.Fatalf("Brunhild enchanted %s", id)
		}
	}
	if gameutil.GetGoldQuantity(state) != 20000 {
		t.Fatalf("a refusal costs nothing: gold %d", gameutil.GetGoldQuantity(state))
	}
	state.TimeOfDay = 120 // asleep
	if r := enchantAt(t, sess, "forgemistress", quartz); r.Success {
		t.Fatal("enchanted in her sleep")
	}
	if r := enchantAt(t, sess, "gem-cutter", quartz); r.Success {
		t.Fatal("the jeweller doesn't enchant")
	}
}

// wear equips a carried item into a slot.
func wear(t *testing.T, sess *session.GameSession, itemID, slot string) *GameActionResponse {
	t.Helper()
	general := sess.SaveData.Inventory["general_slots"].([]interface{})
	for i, raw := range general {
		if s, _ := raw.(map[string]interface{}); s != nil && s["item"] == itemID {
			return fishAction(t, sess, "equip_item", map[string]any{"item_id": itemID, "from_slot": float64(i), "from_slot_type": "general", "equipment_slot": slot})
		}
	}
	t.Fatalf("not carrying %s", itemID)
	return nil
}

// A worn effect lives exactly as long as its piece is on (beta exploit: a
// ring swapped out for another kept its effect, so swapping back and forth
// stacked a Ring of Protection without limit). Two pieces of the same gem
// give its effect twice.
func TestWornEffectsFollowTheGear(t *testing.T) {
	ruby := types.GemPieceID("ring", "ruby", "large", "standard") + types.EnchantedSuffix
	sess := forgeSession(t, carrying("ring-of-protection", 1), carrying("guild-signet-ring", 1), carrying(ruby, 1), carrying(ruby, 1))
	state := &sess.SaveData

	for i := 0; i < 3; i++ {
		wear(t, sess, "ring-of-protection", "ring1")
		wear(t, sess, "guild-signet-ring", "ring1") // swaps the protection ring out
	}
	if n := effects.Instances(state, "ring-of-protection"); n != 0 {
		t.Fatalf("a swapped-out Ring of Protection still on %d times", n)
	}
	if n := effects.Instances(state, "guild-signet"); n != 1 {
		t.Fatalf("the signet on %d times", n)
	}

	state.Stats["strength"] = 10
	wear(t, sess, ruby, "ring1")
	wear(t, sess, ruby, "ring2")
	if n := effects.Instances(state, "jewel-ruby-large"); n != 2 {
		t.Fatalf("two ruby rings, the effect on %d times", n)
	}
	if got := effects.Ability(state, "strength"); got != 12 {
		t.Fatalf("STR %d with two large rubies, want 12", got)
	}
	wear(t, sess, "ring-of-protection", "ring2") // swaps a ruby ring out
	if n := effects.Instances(state, "jewel-ruby-large"); n != 1 {
		t.Fatalf("one ruby ring left, the effect on %d times", n)
	}
	if n := effects.Instances(state, "ring-of-protection"); n != 1 {
		t.Fatalf("the protection ring on %d times", n)
	}
}

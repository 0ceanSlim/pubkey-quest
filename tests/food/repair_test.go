package food_test

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// Repair (degradation §5): field repair only back up to Worn, a smith's quote.

// armed is a player wielding a worn item, carrying a tool.
func armed(gear, item string, wear int, tool string) *types.SaveFile {
	s := player(2)
	g := s.Inventory["gear_slots"].(map[string]interface{})
	g[gear] = map[string]interface{}{"item": item, "quantity": 1, condition.KeyWear: wear}
	slot(s, 0)["item"], slot(s, 0)["quantity"] = tool, 1
	return s
}

func worn(s *types.SaveFile, gear string) map[string]interface{} {
	return s.Inventory["gear_slots"].(map[string]interface{})[gear].(map[string]interface{})
}

func TestWhetstoneSharpensToWorn(t *testing.T) {
	setup(t)
	// A ruined longsword (300 durability): the whetstone brings it back to 74% left.
	s := armed("mainhand", "longsword", 250, "whetstone")
	targets := inventory.UseTargets(s, "whetstone")
	if len(targets) != 1 || targets[0].Gear != "mainhand" {
		t.Fatalf("whetstone targets: %+v", targets)
	}
	before := s.TimeOfDay
	resp, err := inventory.UseOn(s, "whetstone", targets[0].SlotRef)
	if err != nil || !resp.Success {
		t.Fatalf("sharpen: %v %+v", err, resp)
	}
	if got := condition.WearOf(worn(s, "mainhand")); got != 78 {
		t.Errorf("sharpened longsword wear %d, want 78 (74%% left)", got)
	}
	if tier, _ := condition.WearTier(worn(s, "mainhand")); tier != condition.Worn {
		t.Errorf("sharpened to %s, want Worn", tier)
	}
	if s.TimeOfDay-before != 10 {
		t.Errorf("sharpening took %d minutes, want 10", s.TimeOfDay-before)
	}
	// Once it's Worn there's nothing left the whetstone can do.
	if targets := inventory.UseTargets(s, "whetstone"); len(targets) != 0 {
		t.Errorf("still offered after sharpening: %+v", targets)
	}
	// A mace has no edge: it's the crafting kit's.
	s = armed("mainhand", "mace", 150, "whetstone")
	if targets := inventory.UseTargets(s, "whetstone"); len(targets) != 0 {
		t.Errorf("whetstone offered on a mace: %+v", targets)
	}
}

func TestCraftingKitMends(t *testing.T) {
	setup(t)
	defer func(f func(*types.SaveFile, string) int) { inventory.SkillOf = f }(inventory.SkillOf)

	// Pass (a skill of 60 always beats DC 12): back to Worn.
	inventory.SkillOf = func(*types.SaveFile, string) int { return 60 }
	s := armed("chest", "chainmail-hauberk", 300, "crafting-kit")
	targets := inventory.UseTargets(s, "crafting-kit")
	if len(targets) != 1 || targets[0].Gear != "chest" {
		t.Fatalf("kit targets: %+v", targets)
	}
	if resp, _ := inventory.UseOn(s, "crafting-kit", targets[0].SlotRef); !resp.Success {
		t.Fatalf("mend: %+v", resp)
	}
	if got := condition.WearOf(worn(s, "chest")); got != 104 { // 400 × 26%
		t.Errorf("mended hauberk wear %d, want 104", got)
	}

	// Fail (a skill of -40 never makes it): half the way.
	inventory.SkillOf = func(*types.SaveFile, string) int { return -40 }
	s = armed("chest", "chainmail-hauberk", 300, "crafting-kit")
	resp, _ := inventory.UseOn(s, "crafting-kit", targets[0].SlotRef)
	if got := condition.WearOf(worn(s, "chest")); got != 202 { // 300 − ceil(196/2)
		t.Errorf("rough mend wear %d, want 202 (%s)", got, resp.Message)
	}
	// The kit won't sharpen a sword.
	s = armed("mainhand", "longsword", 250, "crafting-kit")
	if resp, _ := inventory.UseOn(s, "crafting-kit", inventory.SlotRef{Type: "equipment", Gear: "mainhand"}); resp.Success {
		t.Errorf("crafting kit mended a longsword: %s", resp.Message)
	}
}

func TestSmithQuotes(t *testing.T) {
	setup(t)
	smiths := condition.Repair().Smiths
	sword := map[string]interface{}{"item": "longsword", "quantity": 1, condition.KeyWear: 150} // half worn
	bow := map[string]interface{}{"item": "longbow", "quantity": 1, condition.KeyWear: 150}

	// Master smith: gold only, 1.5 × value × the share mended, back to pristine.
	q := condition.SmithQuote(smiths["master-smith"], sword, 1500)
	if q.Gold != 1125 || q.Material != "" || !q.Pristine {
		t.Errorf("master smith: %+v", q)
	}
	// Forgemistress: cheap, plus iron ore per 25% mended.
	q = condition.SmithQuote(smiths["forgemistress"], sword, 1500)
	if q.Gold != 450 || q.Material != "iron-ore" || q.Count != 2 {
		t.Errorf("forgemistress: %+v", q)
	}
	// War-gear trader: only back to Worn (78 wear left of 300).
	q = condition.SmithQuote(smiths["war-gear-trader"], sword, 1500)
	if q.Pristine || q.ToWear != 78 || q.Gold != 180 {
		t.Errorf("war-gear trader: %+v", q)
	}
	// The master smith doesn't work wood; the bowyer does, with a fur discount.
	if q := condition.SmithQuote(smiths["master-smith"], bow, 5000); !q.CantWork {
		t.Errorf("master smith took a longbow: %+v", q)
	}
	if q := condition.SmithQuote(smiths["master-bowyer"], bow, 5000); q.CantWork || q.Discount != "owlbear-fur" || q.Percent != 50 {
		t.Errorf("bowyer: %+v", q)
	}
	if !strings.EqualFold(condition.MaterialOf("hide-cuirass"), "hide") {
		t.Errorf("hide cuirass is %s", condition.MaterialOf("hide-cuirass"))
	}
}

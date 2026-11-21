package gems

import (
	"strings"
	"testing"

	"pubkey-quest/types"
)

// Every enchanted piece's worn effects exist, and the effect ids are unique.
func TestEnchantedPiecesWearRealEffects(t *testing.T) {
	c := testCfg(t)
	defs := map[string]map[string]any{}
	for _, e := range Effects(c) {
		id := e["id"].(string)
		if defs[id] != nil {
			t.Fatalf("duplicate effect %s", id)
		}
		defs[id] = e
		if e["removal"].(map[string]any)["type"] != "equipment" || e["description"] == "While worn: ." {
			t.Fatalf("%s: %v", id, e)
		}
	}
	// 17 plain types × 5 sizes, the pearls × 4, diamond 3 + 4 + 4 by
	// quality, alexandrite 5 sizes × 12 classes.
	if len(defs) != 17*5+2*4+11+5*12 {
		t.Fatalf("%d effects", len(defs))
	}
	pieces := 0
	for _, it := range Items(c) {
		g, _ := Parse(c, it["id"].(string))
		if !g.Enchanted {
			continue
		}
		pieces++
		var worn []string
		if ids, ok := it["effects_when_worn"].([]string); ok {
			worn = ids
		}
		if by, ok := it["effects_when_worn_by_class"].(map[string]any); ok {
			for _, ids := range by {
				worn = append(worn, ids.([]string)...)
			}
		}
		if len(worn) == 0 {
			t.Fatalf("%s wears nothing", it["id"])
		}
		for _, id := range worn {
			if defs[id] == nil {
				t.Fatalf("%s wears %s, which isn't generated", it["id"], id)
			}
		}
		if g.Quality == "flawless" && g.Size != "tiny" && len(worn) < 2 && it["effects_when_worn_by_class"] == nil {
			t.Fatalf("%s: a flawless piece adds the tiny effect: %v", it["id"], worn)
		}
	}
	if pieces != 824 {
		t.Fatalf("%d enchanted pieces", pieces)
	}
}

// Diamonds' resistances follow quality; alexandrite follows the class.
func TestDiamondAndAlexandrite(t *testing.T) {
	c := testCfg(t)
	if id := EffectID(c, "diamond", "large", "flawless", ""); id != "jewel-diamond-large-flawless" {
		t.Fatal(id)
	}
	if id := EffectID(c, "diamond", "medium", "flawless", ""); id != "jewel-diamond-medium" {
		t.Fatal(id)
	}
	if id := EffectID(c, "alexandrite", "large", "fine", "Wizard"); id != "jewel-alexandrite-large-wizard" {
		t.Fatal(id)
	}
	ring := Gem{Type: "diamond", Size: "huge", Quality: "flawless", Piece: "ring"}
	if d := PieceDescription(c, ring, ""); !strings.Contains(d, "necrotic and radiant") {
		t.Fatalf("a flawless huge diamond resists all seven: %s", d)
	}
	flawed := ring
	flawed.Quality = "flawed"
	if d := PieceDescription(c, flawed, ""); strings.Contains(d, "lightning") {
		t.Fatalf("a flawed huge diamond resists fire and cold only: %s", d)
	}
	alex := Gem{Type: "alexandrite", Size: "large", Quality: "standard", Piece: "necklace"}
	if d := PieceDescription(c, alex, "Rogue"); d != "DEX +2" {
		t.Fatalf("rogue: %s", d)
	}
	if d := PieceDescription(c, alex, "cleric"); d != "WIS +2" {
		t.Fatalf("cleric: %s", d)
	}
}

// Fees and chances: the size's base × the enchanter × the quality, the
// chance capped short of flawless, which always takes.
func TestEnchantFeesAndChances(t *testing.T) {
	c := testCfg(t)
	corvin := &types.EnchantConfig{Pieces: []string{"ring", "necklace"}, MaxSize: "large", Success: 0.85, Fee: 1.5}
	g := Gem{Type: "quartz", Size: "medium", Quality: "standard", Piece: "ring"}
	if f := EnchantFee(c, g, corvin); f != 3750 {
		t.Fatalf("fee %d", f)
	}
	if ch := EnchantChance(c, g, corvin); ch != 0.85 {
		t.Fatalf("chance %v", ch)
	}
	g.Quality = "fine"
	if f, ch := EnchantFee(c, g, corvin), EnchantChance(c, g, corvin); f != 3000 || ch != 0.95 {
		t.Fatalf("fine: fee %d, chance %v (capped at 95%%)", f, ch)
	}
	g.Quality = "flawed"
	if ch := EnchantChance(c, g, corvin); ch < 0.649 || ch > 0.651 {
		t.Fatalf("flawed: %v", ch)
	}
	g.Quality = "flawless"
	if ch := EnchantChance(c, g, corvin); ch != 1 {
		t.Fatalf("flawless: %v", ch)
	}

	brunhild := &types.EnchantConfig{Pieces: []string{"ring"}, Gems: []string{"ruby"}, MaxSize: "large", Success: 0.8, Fee: 1}
	for _, tc := range []struct {
		g    Gem
		want string
	}{
		{Gem{Type: "ruby", Size: "large", Quality: "fine", Piece: "ring"}, ""},
		{Gem{Type: "ruby", Size: "large", Quality: "fine", Piece: "necklace"}, "necklace"},
		{Gem{Type: "quartz", Size: "large", Quality: "fine", Piece: "ring"}, "quartz"},
		{Gem{Type: "ruby", Size: "huge", Quality: "fine", Piece: "ring"}, "too big"},
		{Gem{Type: "ruby", Size: "large", Quality: "fine", Piece: "ring", Enchanted: true}, "already"},
	} {
		got := Refusal(c, tc.g, brunhild)
		if (tc.want == "") != (got == "") || !strings.Contains(got, tc.want) {
			t.Fatalf("%+v: %q, want %q", tc.g, got, tc.want)
		}
	}
	seraphina := &types.EnchantConfig{Pieces: []string{"ring", "necklace"}, MaxSize: "huge", Success: 0.75, Fee: 2}
	if why := Refusal(c, Gem{Type: "pearl", Size: "huge", Quality: "fine", Piece: "ring"}, seraphina); why == "" {
		t.Fatal("a huge pearl has nothing to wake")
	}
}

// Enchanting and then selling never pays: a shop's best price (0.95× value at
// Charisma 20) on what the work adds stays under the cheapest fee for it.
func TestEnchantAndSellNeverPays(t *testing.T) {
	c := testCfg(t)
	cheapest := &types.EnchantConfig{Pieces: types.GemPieces, MaxSize: "huge", Success: 1, Fee: 1}
	for _, it := range Items(c) {
		g, _ := Parse(c, it["id"].(string))
		if !g.Enchanted {
			continue
		}
		plain := g
		plain.Enchanted = false
		if gain := 0.95 * float64(Value(c, g)-Value(c, plain)); gain >= float64(EnchantFee(c, plain, cheapest)) {
			t.Fatalf("%s: selling pays %.0f more, the work costs %d", g.ID(), gain, EnchantFee(c, plain, cheapest))
		}
	}
}

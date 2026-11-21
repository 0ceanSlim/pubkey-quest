package gems

import (
	"math"
	"math/rand"
	"testing"

	"pubkey-quest/types"
)

func testCfg(t *testing.T) *types.GemsConfig {
	t.Helper()
	c, err := LoadFile("../../../../game-data/systems/gems.json")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestIDsRoundTrip(t *testing.T) {
	c := testCfg(t)
	seen := map[string]bool{}
	for _, it := range Items(c) {
		id := it["id"].(string)
		if seen[id] {
			t.Fatalf("duplicate id %s", id)
		}
		seen[id] = true
		g, ok := Parse(c, id)
		if !ok || g.ID() != id {
			t.Fatalf("%s parses to %+v", id, g)
		}
		if Value(c, g) != it["value"] {
			t.Fatalf("%s: value %v, item says %v", id, Value(c, g), it["value"])
		}
	}
	// 19 types × 5 sizes × (1 uncut + 4 cut × (1 + 2 pieces)), and the two
	// pearls, which are never uncut; then every piece enchanted at a size its
	// gem has an effect for (no pearl is huge enough): 824 more.
	if len(seen) != 19*5*(1+4*3)+2*5*4*3+(19*5+2*4)*4*2 {
		t.Fatalf("%d items", len(seen))
	}
	for _, bad := range []string{"gem-ruby-small", "gem-rubyx-small-fine", "ring-ruby-small-uncut", "gem-ruby-vast-fine", "hat-ruby-small-fine",
		"gem-pearl-small-uncut", "gem-black-pearl-small-uncut", "gem-black-small-fine",
		"gem-ruby-small-fine-enchanted", "ring-ruby-small-fine-enchanted-enchanted"} {
		if _, ok := Parse(c, bad); ok {
			t.Fatalf("%s parsed", bad)
		}
	}
}

func TestValuesAndRarity(t *testing.T) {
	c := testCfg(t)
	cases := []struct {
		g      Gem
		value  int
		rarity string
	}{
		{Gem{Type: "quartz", Size: "tiny"}, 10, "common"},
		{Gem{Type: "amethyst", Size: "medium", Quality: "flawless"}, 40000, "rare"},
		{Gem{Type: "amethyst", Size: "medium", Quality: "flawed"}, 5000, "uncommon"},
		{Gem{Type: "diamond", Size: "huge", Quality: "flawless"}, 200_000_000, "mythic"},
		{Gem{Type: "ruby", Size: "small", Quality: "fine", Piece: "ring"}, 1000 + 20000, "rare"},
		{Gem{Type: "quartz", Size: "tiny", Quality: "standard", Piece: "necklace"}, 2010, "uncommon"},
	}
	for _, tc := range cases {
		if v := Value(c, tc.g); v != tc.value || Rarity(v) != tc.rarity {
			t.Errorf("%s: %d %s, want %d %s", tc.g.ID(), v, Rarity(v), tc.value, tc.rarity)
		}
	}
	if gold, labour := SetFee(c, Gem{Type: "ruby", Size: "small", Quality: "fine"}, "necklace"); gold != 2000 || labour != 2000 {
		t.Errorf("set fee %d + %d", gold, labour)
	}
}

func TestCurve(t *testing.T) {
	curve := types.Curve{{0, 0.15}, {5, 0.25}, {10, 0.3}}
	for _, tc := range []struct{ x, want float64 }{{-1, 0.15}, {0, 0.15}, {5, 0.25}, {20, 0.3}} {
		if got := At(curve, tc.x); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("At(%v) = %v", tc.x, got)
		}
	}
	if mid := At(curve, 2.5); math.Abs(mid-math.Sqrt(0.15*0.25)) > 1e-9 {
		t.Errorf("log-linear midpoint %v", mid)
	}
}

// The kill curves land near the draft's table (gem-system.md §2): about 34
// gold of gems per CR-0 kill, about 1,070 at CR 5.
func TestKillYield(t *testing.T) {
	c := testCfg(t)
	for _, tc := range []struct {
		cr     float64
		lo, hi float64
	}{{0, 20, 50}, {5, 700, 1500}} {
		m := &types.MonsterData{ChallengeRating: tc.cr, Type: "Humanoid"}
		ev := 0.0
		tw := TypeWeights(c, At(c.Drops.TypeTilt, tc.cr), nil)
		sw := SizeWeights(c, At(c.Drops.SizeTilt, tc.cr), -1)
		tt, st := 0.0, 0.0
		for _, w := range tw {
			tt += w
		}
		for _, w := range sw {
			st += w
		}
		for id, w := range tw {
			for i, s := range c.Sizes {
				ev += w / tt * sw[i] / st * float64(Value(c, Gem{Type: id, Size: s.ID}))
			}
		}
		ev *= KillChance(c, m)
		if ev < tc.lo || ev > tc.hi {
			t.Errorf("CR %v: %.0f gold of gems per kill", tc.cr, ev)
		}
	}
	none := 0.0
	if KillChance(c, &types.MonsterData{Type: "Beast", ChallengeRating: 2}) != 0 ||
		KillChance(c, &types.MonsterData{Type: "Beast", GemChance: ptr(0.5)}) != 0.5 ||
		KillChance(c, &types.MonsterData{Type: "Humanoid", GemChance: &none}) != 0 {
		t.Error("gem_chance overrides and no-gem types")
	}
}

func ptr(f float64) *float64 { return &f }

func TestForFaceLeaningAndCap(t *testing.T) {
	c := testCfg(t)
	rng := rand.New(rand.NewSource(1))
	small := SizeIndex(c, "small")
	for range 2000 {
		g, ok := ForFace(c, 4, map[string]int{"obsidian": 3, "topaz": 1}, small, rng)
		if !ok || (g.Type != "obsidian" && g.Type != "topaz") || SizeIndex(c, g.Size) > small {
			t.Fatalf("rolled %+v", g)
		}
	}
	counts := map[string]int{}
	for range 10000 {
		counts[RollQuality(c, rng)]++
	}
	if counts["standard"] < 4500 || counts["flawless"] > 700 || counts["flawless"] == 0 {
		t.Errorf("qualities %v", counts)
	}
}

func TestPearls(t *testing.T) {
	c := testCfg(t)
	if g, ok := Parse(c, "ring-black-pearl-medium-fine"); !ok || g.Type != "black-pearl" || g.Piece != "ring" || g.Quality != "fine" {
		t.Fatalf("parsed %+v", g)
	}
	for _, tilt := range []float64{0.5, 1, 5} {
		for typ := range TypeWeights(c, tilt, map[string]int{"pearl": 5, "ruby": 1}) {
			if c.Types[typ].Natural {
				t.Fatalf("%s in a rock or kill roll", typ)
			}
		}
	}
	rng := rand.New(rand.NewSource(2))
	found := map[string]int{}
	for range 20000 {
		g, ok := ForShellfish(c, "oyster", rng)
		if !ok {
			continue
		}
		if !c.Types[g.Type].Natural || g.Quality == "" || SizeIndex(c, g.Size) > SizeIndex(c, c.Pearls.Shellfish["oyster"].MaxSize) {
			t.Fatalf("rolled %+v", g)
		}
		found[g.Type]++
	}
	n := found["pearl"] + found["black-pearl"]
	if n < 900 || n > 1500 || found["black-pearl"] == 0 || found["black-pearl"] > n/5 {
		t.Errorf("20,000 oysters gave %v", found)
	}
	if _, ok := ForShellfish(c, "trout", rng); ok {
		t.Error("a trout held a pearl")
	}
	if got := Name(c, Gem{Type: "pearl", Size: "small", Quality: "standard"}); got != "Pearl (small)" {
		t.Errorf("name %q", got)
	}
}

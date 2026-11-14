package mining

import (
	"testing"

	"pubkey-quest/types"
)

func TestSeamDefaults(t *testing.T) {
	cfg := &types.MiningConfig{Tiers: []types.MiningTier{{Tier: 1, DC: 10, FacesPerDay: 3}}}
	s := Effective(cfg, types.Seam{Tier: 1})
	if s.DC != 10 || s.FacesPerDay != 3 {
		t.Fatalf("defaults not applied: %+v", s)
	}
	s = Effective(cfg, types.Seam{Tier: 1, DC: 13, FacesPerDay: 1})
	if s.DC != 13 || s.FacesPerDay != 1 {
		t.Fatalf("authored values overridden: %+v", s)
	}
}

func TestFacesResetEachDay(t *testing.T) {
	m := Memory{}
	if m.Left("a", 4, 3) != 3 {
		t.Fatal("a fresh seam should have all its faces")
	}
	for range 3 {
		m.Work("a", 4)
	}
	if m.Left("a", 4, 3) != 0 {
		t.Fatal("three faces worked, some still left")
	}
	if m.Left("b", 4, 3) != 3 {
		t.Fatal("seams share a count")
	}
	if m.Left("a", 5, 3) != 3 {
		t.Fatal("the seam didn't reset the next day")
	}
}

func TestQuickLootFromOres(t *testing.T) {
	s := types.Seam{Ores: []types.SeamOre{{Item: "iron-ore", Weight: 60, Quantity: [2]int{1, 2}}, {Item: "copper-ore", Weight: 40, Quantity: [2]int{1, 3}}}}
	lt := QuickLoot(s)
	if lt.Rolls != 2 || len(lt.Tiers) != 2 || lt.Tiers[1].Entries[0].Item != "copper-ore" {
		t.Fatalf("derived table: %+v", lt)
	}
	authored := &types.POILootTable{Rolls: 1}
	s.QuickLoot = authored
	if QuickLoot(s) != authored {
		t.Fatal("an authored quick_loot table wasn't used")
	}
}

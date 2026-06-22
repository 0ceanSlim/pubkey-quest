package character_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

func TestProficiencyBonus(t *testing.T) {
	cases := []struct{ level, want int }{
		{1, 2}, {4, 2}, {5, 3}, {8, 3}, {9, 4}, {12, 4}, {13, 5}, {16, 5}, {17, 6}, {20, 6},
	}
	for _, c := range cases {
		if got := character.ProficiencyBonus(c.level); got != c.want {
			t.Errorf("ProficiencyBonus(%d) = %d, want %d", c.level, got, c.want)
		}
	}
}

func TestFeatEligibleLevels(t *testing.T) {
	base := character.FeatEligibleLevels("Wizard")
	for _, lvl := range []int{4, 8, 12, 16, 19} {
		if !base[lvl] {
			t.Errorf("Wizard should be feat-eligible at %d", lvl)
		}
	}
	if base[6] || base[10] || base[14] {
		t.Error("Wizard should not get the Fighter/Rogue bonus feat levels")
	}
	if got := len(base); got != 5 {
		t.Errorf("Wizard feat levels = %d, want 5", got)
	}

	if f := character.FeatEligibleLevels("Fighter"); !f[6] || !f[14] || len(f) != 7 {
		t.Errorf("Fighter feat levels wrong: %v", f)
	}
	if r := character.FeatEligibleLevels("rogue"); !r[10] || len(r) != 6 { // case-insensitive
		t.Errorf("Rogue feat levels wrong: %v", r)
	}
}

func TestBuildLevelGuide_Caster(t *testing.T) {
	adv := testAdvancement()
	save := &types.SaveFile{
		Class:      "Wizard",
		Experience: xpForLevel(3), // current level 3
		Stats:      map[string]interface{}{"Constitution": float64(12), "Intelligence": float64(16)},
	}
	// Wizard spell slots: level_2 first appears at level 3, level_3 at level 5.
	slots := map[int]map[string]int{
		1: {"cantrips": 3, "level_1": 2},
		2: {"cantrips": 3, "level_1": 3},
		3: {"cantrips": 3, "level_1": 4, "level_2": 2},
		4: {"cantrips": 4, "level_1": 4, "level_2": 3},
		5: {"cantrips": 4, "level_1": 4, "level_2": 3, "level_3": 2},
	}
	guide := character.BuildLevelGuide(save, adv, nil, slots)
	if len(guide) != 20 {
		t.Fatalf("guide has %d rows, want 20", len(guide))
	}

	row := func(lvl int) character.GuideLevel { return guide[lvl-1] }

	// Current-level flags.
	if !row(3).IsCurrent || !row(3).Reached || row(4).Reached {
		t.Error("current/reached flags wrong around level 3")
	}
	// Ability-point cadence + feat eligibility.
	if !row(2).AbilityPoint || row(3).AbilityPoint || !row(4).AbilityPoint {
		t.Error("ability-point cadence wrong (want 2 yes, 3 no, 4 yes)")
	}
	if !row(4).FeatEligible || row(6).FeatEligible {
		t.Error("Wizard feat eligibility wrong (4 yes, 6 no)")
	}
	// New spell tiers unlocking.
	if row(3).NewSpellTier != 2 {
		t.Errorf("level 3 new spell tier = %d, want 2", row(3).NewSpellTier)
	}
	if row(5).NewSpellTier != 3 {
		t.Errorf("level 5 new spell tier = %d, want 3", row(5).NewSpellTier)
	}
	if row(2).NewSpellTier != 0 {
		t.Errorf("level 2 opens no new tier, got %d", row(2).NewSpellTier)
	}
	// HP grows; mana present for a caster.
	if row(2).HPGain <= 0 || row(5).MaxMana == 0 {
		t.Errorf("expected HP growth and caster mana: hpGain=%d mana=%d", row(2).HPGain, row(5).MaxMana)
	}
}

func TestDeriveMaxMana_HalfCaster(t *testing.T) {
	stats := map[string]interface{}{"charisma": float64(16), "wisdom": float64(16), "intelligence": float64(16)} // all +3

	// Paladin (CHA) — half-caster: mod + floor(level/2).
	cases := []struct {
		class string
		level int
		want  int
	}{
		{"Paladin", 1, 3},  // 3 + 0
		{"Paladin", 2, 4},  // 3 + 1
		{"Paladin", 20, 13}, // 3 + 10
		{"Ranger", 10, 8},  // 3 + 5 (WIS)
	}
	for _, c := range cases {
		if got := character.DeriveMaxMana(c.class, c.level, stats); got != c.want {
			t.Errorf("DeriveMaxMana(%s, L%d) = %d, want %d", c.class, c.level, got, c.want)
		}
	}
	// A full caster outpaces a half-caster at the same level.
	if character.DeriveMaxMana("Wizard", 20, stats) <= character.DeriveMaxMana("Paladin", 20, stats) {
		t.Error("full caster should have more mana than a half-caster at level 20")
	}
}

func TestBuildLevelGuide_Resource(t *testing.T) {
	adv := testAdvancement()

	// Monk ki = wisdom mod + level → grows.
	monk := &types.SaveFile{Class: "Monk", Experience: xpForLevel(1),
		Stats: map[string]interface{}{"Wisdom": float64(14)}} // +2
	g := character.BuildLevelGuide(monk, adv, nil, nil)
	if g[0].ResourceLabel != "Ki" || g[0].ResourceMax != 3 { // L1: 2 + 1
		t.Errorf("Monk L1 = %q %d, want Ki 3", g[0].ResourceLabel, g[0].ResourceMax)
	}
	if g[4].ResourceMax != 7 { // L5: 2 + 5
		t.Errorf("Monk L5 ki = %d, want 7", g[4].ResourceMax)
	}

	// Fighter stamina is flat.
	fighter := &types.SaveFile{Class: "Fighter", Experience: xpForLevel(1),
		Stats: map[string]interface{}{"Constitution": float64(12)}}
	gf := character.BuildLevelGuide(fighter, adv, nil, nil)
	if gf[0].ResourceLabel != "Stamina" || gf[0].ResourceMax != 10 || gf[10].ResourceMax != 10 {
		t.Errorf("Fighter stamina should be flat 10, got %q %d/%d", gf[0].ResourceLabel, gf[0].ResourceMax, gf[10].ResourceMax)
	}

	// Casters carry no martial resource (mana is surfaced instead).
	wiz := &types.SaveFile{Class: "Wizard", Experience: xpForLevel(1),
		Stats: map[string]interface{}{"Intelligence": float64(16)}}
	if character.BuildLevelGuide(wiz, adv, nil, nil)[0].ResourceLabel != "" {
		t.Error("Wizard should have no martial resource label")
	}
}

func TestBuildLevelGuide_Martial(t *testing.T) {
	adv := testAdvancement()
	save := &types.SaveFile{
		Class:      "Fighter",
		Experience: xpForLevel(1),
		Stats:      map[string]interface{}{"Constitution": float64(14)},
	}
	abilities := []character.GuideAbilityUnlock{
		{Name: "Second Wind", UnlockLevel: 1, Tiers: []character.GuideAbilityTier{
			{Level: 1, Summary: "Heal 25%"},
			{Level: 5, Summary: "Heal 40%"},
		}},
	}
	guide := character.BuildLevelGuide(save, adv, abilities, nil)
	row := func(lvl int) character.GuideLevel { return guide[lvl-1] }

	// Unlock at level 1, upgrade at level 5.
	if len(row(1).NewAbilities) != 1 || row(1).NewAbilities[0].Name != "Second Wind" {
		t.Errorf("level 1 should unlock Second Wind, got %+v", row(1).NewAbilities)
	}
	if len(row(5).AbilityUpgrades) != 1 || row(5).AbilityUpgrades[0].Summary != "Heal 40%" {
		t.Errorf("level 5 should upgrade Second Wind, got %+v", row(5).AbilityUpgrades)
	}
	// Fighter feat bonus levels present; no spell data for a martial.
	if !row(6).FeatEligible || row(20).SpellSlots != nil {
		t.Error("Fighter should be feat-eligible at 6 and have no spell slots")
	}
}

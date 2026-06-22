package character_test

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// testAdvancement mirrors the real cadence: 1 ability point at levels
// 2,4,6,8,10,12,14,16,18,19,20 (11 total). XP thresholds are simplified to
// (level-1)*100 so a level is easy to target in tests.
func testAdvancement() []types.AdvancementEntry {
	pts := map[int]int{2: 1, 4: 1, 6: 1, 8: 1, 10: 1, 12: 1, 14: 1, 16: 1, 18: 1, 19: 1, 20: 1}
	adv := make([]types.AdvancementEntry, 0, 20)
	for lvl := 1; lvl <= 20; lvl++ {
		adv = append(adv, types.AdvancementEntry{
			Level:            lvl,
			ExperiencePoints: (lvl - 1) * 100,
			XPMultiplier:     1.0 + 0.05*float64(lvl-1), // mirrors advancement.json: +5% per level
			AbilityPoints:    pts[lvl],
		})
	}
	return adv
}

// xpForLevel returns an XP total squarely inside the given level's band.
func xpForLevel(level int) int { return (level-1)*100 + 10 }

func TestAbilityPointsEarned(t *testing.T) {
	adv := testAdvancement()
	cases := []struct{ level, want int }{
		{1, 0}, {2, 1}, {3, 1}, {4, 2}, {5, 2},
		{16, 8}, {18, 9}, {19, 10}, {20, 11},
	}
	for _, c := range cases {
		if got := character.AbilityPointsEarned(c.level, adv); got != c.want {
			t.Errorf("AbilityPointsEarned(L%d) = %d, want %d", c.level, got, c.want)
		}
	}
	// The cadence must total exactly 11 (the about-page promise: 89 + 11 = 100).
	if got := character.AbilityPointsEarned(20, adv); got != 11 {
		t.Errorf("total ability points over 20 levels = %d, want 11", got)
	}
}

func TestUnspentAbilityPoints(t *testing.T) {
	adv := testAdvancement()

	// Level 4 → earned 2, nothing spent, no feats → 2 unspent.
	save := &types.SaveFile{Experience: xpForLevel(4)}
	if got := character.UnspentAbilityPoints(save, adv); got != 2 {
		t.Errorf("fresh L4 unspent = %d, want 2", got)
	}

	// One point allocated → 1 left.
	save.AbilityIncreases = map[string]int{"Strength": 1}
	if got := character.UnspentAbilityPoints(save, adv); got != 1 {
		t.Errorf("L4 after 1 spend unspent = %d, want 1", got)
	}

	// A feat consumes a cadence point too → 0 left.
	save.FeatsChosen = []string{"alert"}
	if got := character.UnspentAbilityPoints(save, adv); got != 0 {
		t.Errorf("L4 after 1 spend + 1 feat unspent = %d, want 0", got)
	}

	// Over-allocation never returns negative.
	save2 := &types.SaveFile{Experience: xpForLevel(2), AbilityIncreases: map[string]int{"Strength": 5}}
	if got := character.UnspentAbilityPoints(save2, adv); got != 0 {
		t.Errorf("over-allocated unspent = %d, want 0 (clamped)", got)
	}
}

func TestSpendAbilityPoint(t *testing.T) {
	adv := testAdvancement()
	save := &types.SaveFile{
		Class:      "Fighter", // d10, non-caster
		Experience: xpForLevel(4),
		Stats:      map[string]interface{}{"Constitution": float64(15), "Strength": float64(10)},
	}
	character.Hydrate(save, adv)
	hpBefore := save.MaxHP // CON 15 → +2

	// Spend into CON: 15→16 crosses a modifier boundary (+2 → +3), so MaxHP rises.
	if err := character.SpendAbilityPoint(save, "con", adv); err != nil {
		t.Fatalf("spend con: unexpected error %v", err)
	}
	if got := character.AbilityScores(save)["Constitution"]; got != 16 {
		t.Errorf("Constitution = %d, want 16", got)
	}
	if save.MaxHP <= hpBefore {
		t.Errorf("CON bump should raise MaxHP: %d → %d", hpBefore, save.MaxHP)
	}
	if save.AbilityIncreases["Constitution"] != 1 {
		t.Errorf("AbilityIncreases not recorded: %+v", save.AbilityIncreases)
	}
	if got := character.UnspentAbilityPoints(save, adv); got != 1 {
		t.Errorf("unspent after 1 spend = %d, want 1", got)
	}

	// Spend the second point (abbrev resolves to Strength).
	if err := character.SpendAbilityPoint(save, "STR", adv); err != nil {
		t.Fatalf("spend str: unexpected error %v", err)
	}
	if got := character.AbilityScores(save)["Strength"]; got != 11 {
		t.Errorf("Strength = %d, want 11", got)
	}

	// Out of points now.
	if err := character.SpendAbilityPoint(save, "dexterity", adv); err == nil {
		t.Error("expected error spending with 0 unspent points")
	}
}

func TestSpendAbilityPoint_Errors(t *testing.T) {
	adv := testAdvancement()

	// Invalid ability name.
	save := &types.SaveFile{Experience: xpForLevel(20), Stats: map[string]interface{}{}}
	if err := character.SpendAbilityPoint(save, "luck", adv); err == nil {
		t.Error("expected error for invalid ability")
	}

	// At the cap of 20.
	capped := &types.SaveFile{Experience: xpForLevel(20), Stats: map[string]interface{}{"Strength": float64(20)}}
	err := character.SpendAbilityPoint(capped, "strength", adv)
	if err == nil || !strings.Contains(err.Error(), "cap") {
		t.Errorf("expected cap error, got %v", err)
	}
}

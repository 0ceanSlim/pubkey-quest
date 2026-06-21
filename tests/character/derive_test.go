package character_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

func TestAbilityMod(t *testing.T) {
	cases := []struct{ score, want int }{
		{10, 0}, {11, 0}, {12, 1}, {14, 2}, {20, 5},
		{9, -1}, {8, -1}, {7, -2}, {1, -5},
	}
	for _, c := range cases {
		if got := character.AbilityMod(c.score); got != c.want {
			t.Errorf("AbilityMod(%d) = %d, want %d", c.score, got, c.want)
		}
	}
}

func TestDeriveMaxHP(t *testing.T) {
	stats := map[string]interface{}{"constitution": float64(14)} // +2
	cases := []struct {
		class string
		level int
		want  int
	}{
		{"Wizard", 1, 8},   // d6: 6+2
		{"Wizard", 2, 14},  // + per-level (3+1+2 = 6)
		{"Wizard", 5, 32},  // 8 + 4*6
		{"Fighter", 1, 12}, // d10: 10+2
		{"Fighter", 5, 44}, // 12 + 4*(5+1+2)
	}
	for _, c := range cases {
		if got := character.DeriveMaxHP(c.class, c.level, stats); got != c.want {
			t.Errorf("DeriveMaxHP(%s, L%d) = %d, want %d", c.class, c.level, got, c.want)
		}
	}
}

// Level 1 must equal the legacy creation formula (hitDie + CON mod) so a freshly
// created character keeps the same HP after the first Hydrate.
func TestDeriveMaxHP_Level1MatchesCreation(t *testing.T) {
	stats := map[string]interface{}{"constitution": float64(16)} // +3
	if got := character.DeriveMaxHP("Barbarian", 1, stats); got != 15 {
		t.Errorf("Barbarian L1 HP = %d, want 15 (d12+3)", got)
	}
}

func TestDeriveMaxMana(t *testing.T) {
	stats := map[string]interface{}{
		"intelligence": float64(16), // +3
		"charisma":     float64(10), // +0
		"wisdom":       float64(12), // +1
	}
	if got := character.DeriveMaxMana("Wizard", 1, stats); got != 4 { // +3 +1
		t.Errorf("Wizard L1 mana = %d, want 4", got)
	}
	if got := character.DeriveMaxMana("Wizard", 5, stats); got != 8 { // +3 +5
		t.Errorf("Wizard L5 mana = %d, want 8", got)
	}
	if got := character.DeriveMaxMana("Cleric", 3, stats); got != 4 { // WIS+1 +3
		t.Errorf("Cleric L3 mana = %d, want 4", got)
	}
	if got := character.DeriveMaxMana("Fighter", 5, stats); got != 0 {
		t.Errorf("Fighter mana = %d, want 0 (non-caster)", got)
	}
}

func TestStatsCaseInsensitive(t *testing.T) {
	lower := map[string]interface{}{"constitution": float64(14)}
	upper := map[string]interface{}{"Constitution": 14} // capitalized + int
	if character.DeriveMaxHP("Fighter", 3, lower) != character.DeriveMaxHP("Fighter", 3, upper) {
		t.Errorf("stat key casing/type must not change derived HP")
	}
}

func TestHydrate(t *testing.T) {
	adv := []types.AdvancementEntry{
		{ExperiencePoints: 0, Level: 1, XPMultiplier: 1.0},
		{ExperiencePoints: 250, Level: 2, XPMultiplier: 1.05},
		{ExperiencePoints: 650, Level: 3, XPMultiplier: 1.10},
	}
	save := &types.SaveFile{
		Class:      "Wizard",
		Experience: 300, // level 2
		Stats:      map[string]interface{}{"constitution": float64(14), "intelligence": float64(16)},
		HP:         99, // over max — must clamp down
		Mana:       99,
	}
	character.Hydrate(save, adv)
	if save.MaxHP != 14 {
		t.Errorf("Hydrate MaxHP = %d, want 14 (L2 wizard CON14)", save.MaxHP)
	}
	if save.MaxMana != 5 {
		t.Errorf("Hydrate MaxMana = %d, want 5 (L2 wizard INT16)", save.MaxMana)
	}
	if save.HP != 14 || save.Mana != 5 {
		t.Errorf("Hydrate must clamp HP/Mana to max: HP=%d Mana=%d", save.HP, save.Mana)
	}

	// Level 1 (no XP) derives the lower maximum.
	save.Experience = 0
	character.Hydrate(save, adv)
	if save.MaxHP != 8 {
		t.Errorf("Hydrate L1 MaxHP = %d, want 8", save.MaxHP)
	}
}

func TestGrantXP_LevelsUpAndHeals(t *testing.T) {
	adv := []types.AdvancementEntry{
		{ExperiencePoints: 0, Level: 1, XPMultiplier: 1.0},
		{ExperiencePoints: 250, Level: 2, XPMultiplier: 1.05},
	}
	save := &types.SaveFile{
		Class:      "Fighter", // d10, non-caster
		Experience: 0,
		Stats:      map[string]interface{}{"constitution": float64(14)}, // +2
		HP:         12,
	}
	res := character.GrantXP(save, 300, adv) // crosses 250 → level 2
	if !res.Leveled || res.OldLevel != 1 || res.NewLevel != 2 {
		t.Fatalf("expected level 1→2, got %+v", res)
	}
	if res.OldMaxHP != 12 || res.NewMaxHP != 20 { // L1 10+2; L2 +8
		t.Errorf("MaxHP %d→%d, want 12→20", res.OldMaxHP, res.NewMaxHP)
	}
	if save.HP != 20 {
		t.Errorf("level-up should heal to full: HP=%d, want 20", save.HP)
	}
	if save.Experience != 300 {
		t.Errorf("XP not applied: %d", save.Experience)
	}
}

func TestGrantXP_NoLevelNoHeal(t *testing.T) {
	adv := []types.AdvancementEntry{
		{ExperiencePoints: 0, Level: 1, XPMultiplier: 1.0},
		{ExperiencePoints: 250, Level: 2, XPMultiplier: 1.05},
	}
	save := &types.SaveFile{
		Class:      "Fighter",
		Experience: 0,
		Stats:      map[string]interface{}{"constitution": float64(14)},
		HP:         5, // wounded
	}
	res := character.GrantXP(save, 100, adv) // stays level 1
	if res.Leveled {
		t.Errorf("should not level up at 100 XP, got %+v", res)
	}
	if save.HP != 5 {
		t.Errorf("no level-up must not heal: HP=%d, want 5", save.HP)
	}
	if save.Experience != 100 {
		t.Errorf("XP not applied: %d", save.Experience)
	}
}

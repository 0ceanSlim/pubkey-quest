package character_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// featSave builds a level-targeted character with baseline stats. testAdvancement()
// and xpForLevel() live in ability_points_test.go (same package).
func featSave(class string, level int) *types.SaveFile {
	s := &types.SaveFile{
		Class:      class,
		Experience: xpForLevel(level),
		Stats: map[string]interface{}{
			"strength": 10, "dexterity": 10, "constitution": 12,
			"intelligence": 10, "wisdom": 10, "charisma": 10,
		},
	}
	character.Hydrate(s, testAdvancement())
	return s
}

func fixedFeat(id, stat string) *types.Feat {
	return &types.Feat{ID: id, Name: id, StatGrant: &types.FeatStatGrant{Amount: 1, Choices: []string{stat}}}
}

func TestFeatSlotsAvailable(t *testing.T) {
	adv := testAdvancement()
	// Most classes: eligible at 4,8,12,16,19.
	if got := character.FeatSlotsAvailable(featSave("Wizard", 3), adv); got != 0 {
		t.Errorf("level 3 should have 0 feat slots, got %d", got)
	}
	if got := character.FeatSlotsAvailable(featSave("Wizard", 4), adv); got != 1 {
		t.Errorf("level 4 should have 1 feat slot, got %d", got)
	}
	if got := character.FeatSlotsAvailable(featSave("Wizard", 8), adv); got != 2 {
		t.Errorf("level 8 should have 2 feat slots, got %d", got)
	}
	// Fighter adds level 6.
	if got := character.FeatSlotsAvailable(featSave("Fighter", 6), adv); got != 2 {
		t.Errorf("fighter level 6 should have 2 feat slots, got %d", got)
	}
}

func TestChooseFeatFixedStatConsumesPoint(t *testing.T) {
	adv := testAdvancement()
	s := featSave("Wizard", 4) // 2 earned points, 0 spent
	before := character.UnspentAbilityPoints(s, adv)

	if err := character.ChooseFeat(s, fixedFeat("durable", "constitution"), "", adv); err != nil {
		t.Fatalf("durable: %v", err)
	}
	if character.AbilityScores(s)["Constitution"] != 13 {
		t.Errorf("Durable should raise CON to 13, got %d", character.AbilityScores(s)["Constitution"])
	}
	if !character.HasFeat(s, "durable") {
		t.Error("HasFeat(durable) should be true")
	}
	if got := character.UnspentAbilityPoints(s, adv); got != before-1 {
		t.Errorf("a feat should consume one point: %d → %d", before, got)
	}
}

func TestChooseFeatHalfFeatChoice(t *testing.T) {
	adv := testAdvancement()
	athlete := &types.Feat{ID: "athlete", Name: "Athlete", StatGrant: &types.FeatStatGrant{Amount: 1, Choices: []string{"strength", "dexterity"}}}

	// Missing choice → error.
	if err := character.ChooseFeat(featSave("Rogue", 4), athlete, "", adv); err == nil {
		t.Error("half-feat with no choice should error")
	}
	// Invalid choice → error.
	if err := character.ChooseFeat(featSave("Rogue", 4), athlete, "wisdom", adv); err == nil {
		t.Error("athlete can't grant wisdom")
	}
	// Valid choice → DEX+1, stored with the suffix.
	s := featSave("Rogue", 4)
	if err := character.ChooseFeat(s, athlete, "dexterity", adv); err != nil {
		t.Fatalf("athlete dex: %v", err)
	}
	if character.AbilityScores(s)["Dexterity"] != 11 {
		t.Errorf("athlete(dex) should raise DEX to 11, got %d", character.AbilityScores(s)["Dexterity"])
	}
	if len(s.FeatsChosen) != 1 || s.FeatsChosen[0] != "athlete:dexterity" {
		t.Errorf("FeatsChosen should record the choice, got %v", s.FeatsChosen)
	}
}

func TestChooseFeatGuards(t *testing.T) {
	adv := testAdvancement()

	// No slot at a pre-feat level.
	if err := character.ChooseFeat(featSave("Wizard", 3), fixedFeat("durable", "constitution"), "", adv); err == nil {
		t.Error("taking a feat below the first feat level should error")
	}

	// Can't take the same feat twice.
	s := featSave("Wizard", 8) // 2 slots
	if err := character.ChooseFeat(s, fixedFeat("durable", "constitution"), "", adv); err != nil {
		t.Fatalf("first durable: %v", err)
	}
	if err := character.ChooseFeat(s, fixedFeat("durable", "constitution"), "", adv); err == nil {
		t.Error("taking durable twice should error")
	}
}

func TestToughRaisesMaxHP(t *testing.T) {
	adv := testAdvancement()
	s := featSave("Fighter", 4)
	base := s.MaxHP

	if err := character.ChooseFeat(s, &types.Feat{ID: "tough", Name: "Tough", HPPerLevel: 2}, "", adv); err != nil {
		t.Fatalf("tough: %v", err)
	}
	if s.MaxHP != base+8 { // +2 per level × level 4
		t.Errorf("Tough at level 4 should add 8 max HP: %d → %d", base, s.MaxHP)
	}
}

func TestFeatBaseID(t *testing.T) {
	if id, choice := character.FeatBaseID("resilient:constitution"); id != "resilient" || choice != "constitution" {
		t.Errorf("FeatBaseID split wrong: %q %q", id, choice)
	}
	if id, choice := character.FeatBaseID("tough"); id != "tough" || choice != "" {
		t.Errorf("FeatBaseID no-choice: %q %q", id, choice)
	}
}

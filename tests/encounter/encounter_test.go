package encounter_test

import (
	"math/rand"
	"testing"

	"pubkey-quest/cmd/server/game/encounter"
)

func pool() []encounter.Candidate {
	return []encounter.Candidate{
		{ID: "giant-rat", CR: 0.125},
		{ID: "wolf", CR: 0.25},
		{ID: "owlbear", CR: 3},
		{ID: "ancient-dragon", CR: 20},
	}
}

func TestEligibleScalesWithLevel(t *testing.T) {
	// level 1 cap = 0.75 → rat, wolf
	if got := len(encounter.Eligible(pool(), 1)); got != 2 {
		t.Errorf("level 1: want 2 eligible, got %d", got)
	}
	// level 5 cap = 3.75 → rat, wolf, owlbear
	if got := len(encounter.Eligible(pool(), 5)); got != 3 {
		t.Errorf("level 5: want 3 eligible, got %d", got)
	}
	// high level → everything, including the dragon
	if got := len(encounter.Eligible(pool(), 30)); got != 4 {
		t.Errorf("level 30: want 4 eligible, got %d", got)
	}
}

func TestCRCapHasFloor(t *testing.T) {
	if encounter.CRCap(0) < 0.5 {
		t.Errorf("CR cap should never fall below the floor, got %v", encounter.CRCap(0))
	}
	if encounter.CRCap(10) <= encounter.CRCap(1) {
		t.Error("CR cap should grow with level")
	}
}

func TestTickChanceScalesAndCaps(t *testing.T) {
	if encounter.TickChance(0) != 0 {
		t.Error("no elapsed time should mean zero chance")
	}
	if encounter.TickChance(30) <= 0 || encounter.TickChance(30) >= 1 {
		t.Errorf("30-minute chance should be a sensible probability, got %v", encounter.TickChance(30))
	}
	if encounter.TickChance(100000) > 0.5001 {
		t.Errorf("chance should be capped, got %v", encounter.TickChance(100000))
	}
}

func TestRollNeverFiresWithoutElapsedTime(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	if _, ok := encounter.Roll(pool(), 5, 0, rng); ok {
		t.Error("an encounter fired with zero elapsed time")
	}
}

func TestRollNeverFiresWithNoEligibleMonster(t *testing.T) {
	// Only an over-CR dragon, low-level player, huge time window: still nothing,
	// because no monster fits the band.
	high := []encounter.Candidate{{ID: "ancient-dragon", CR: 20}}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		if _, ok := encounter.Roll(high, 1, 100000, rng); ok {
			t.Fatal("fired despite no CR-eligible monster")
		}
	}
}

func TestRollEventuallyFiresAndIsEligible(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	fired := false
	for i := 0; i < 200; i++ {
		m, ok := encounter.Roll(pool(), 1, 30, rng)
		if ok {
			fired = true
			if m.CR > encounter.CRCap(1) {
				t.Errorf("fired monster %s CR %v exceeds level-1 cap %v", m.ID, m.CR, encounter.CRCap(1))
			}
		}
	}
	if !fired {
		t.Error("expected at least one encounter over 200 rolls")
	}
}

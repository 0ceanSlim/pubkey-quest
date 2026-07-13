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

func TestDifficultyBands(t *testing.T) {
	// level 1 cap = 0.75.
	cases := []struct {
		cr    float64
		level int
		want  string
	}{
		{0.125, 1, "trivial"}, // rat: well under band
		{0.25, 1, "trivial"},  // wolf: 0.33 of cap
		{0.5, 1, "easy"},      // 0.67 of cap
		{0.75, 1, "fair"},     // right at the band
		{1, 1, "tough"},       // 1.33× the band
		{3, 1, "deadly"},      // 4× the band → warn
		{3, 5, "fair"},        // owlbear vs level 5 (cap 3.75) → in band
		{20, 5, "deadly"},     // dragon vs level 5 → deadly
	}
	for _, c := range cases {
		if got := encounter.Difficulty(c.cr, c.level); got != c.want {
			t.Errorf("Difficulty(CR %v, lvl %d) = %q, want %q", c.cr, c.level, got, c.want)
		}
	}
	if !encounter.IsDeadly(3, 1) {
		t.Error("CR 3 at level 1 should be deadly")
	}
	if encounter.IsDeadly(0.25, 1) {
		t.Error("a wolf at level 1 should not be deadly")
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
	if _, ok := encounter.Roll(pool(), 5, 0, rng, ""); ok {
		t.Error("an encounter fired with zero elapsed time")
	}
}

func TestRollNeverFiresWithNoEligibleMonster(t *testing.T) {
	// Only an over-CR dragon, low-level player, huge time window: still nothing,
	// because no monster fits the band.
	high := []encounter.Candidate{{ID: "ancient-dragon", CR: 20}}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 200; i++ {
		if _, ok := encounter.Roll(high, 1, 100000, rng, ""); ok {
			t.Fatal("fired despite no CR-eligible monster")
		}
	}
}

func TestRollEventuallyFiresAndIsEligible(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	fired := false
	for i := 0; i < 200; i++ {
		m, ok := encounter.Roll(pool(), 1, 30, rng, "")
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

func TestRollAvoidsImmediateRepeat(t *testing.T) {
	// pool() at level 1 → rat + wolf eligible (2 options). Avoiding wolf must
	// always yield rat — never the just-fought monster.
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 300; i++ {
		if m, ok := encounter.Roll(pool(), 1, 30, rng, "wolf"); ok && m.ID == "wolf" {
			t.Fatalf("Roll returned the avoided monster despite an alternative")
		}
	}

	// With only one eligible monster, avoid is ignored — there's nothing else.
	single := []encounter.Candidate{{ID: "wolf", CR: 0.25}}
	rng2 := rand.New(rand.NewSource(1))
	fired := false
	for i := 0; i < 300; i++ {
		if m, ok := encounter.Roll(single, 1, 30, rng2, "wolf"); ok {
			fired = true
			if m.ID != "wolf" {
				t.Fatalf("single-monster pool should still return wolf, got %s", m.ID)
			}
		}
	}
	if !fired {
		t.Error("expected the single-monster pool to fire at least once")
	}
}

package encounter_test

import (
	"math"
	"testing"

	"pubkey-quest/cmd/server/game/encounter"
)

func TestXPForCRRoundTrip(t *testing.T) {
	for _, cr := range []float64{0.125, 0.25, 0.5, 1, 3, 8, 20} {
		if got := encounter.CRForXP(encounter.XPForCR(cr)); math.Abs(got-cr) > 1e-9 {
			t.Errorf("CRForXP(XPForCR(%v)) = %v", cr, got)
		}
	}
}

func TestEffectiveCR(t *testing.T) {
	// One monster vs one character is just its own CR.
	if got := encounter.EffectiveCR([]float64{2}, 1); got != 2 {
		t.Errorf("single monster EffectiveCR = %v, want 2", got)
	}
	// A pack outranks any one member: four CR ¼ goblins (200 XP × 2) ≈ CR 1.8.
	pack := encounter.EffectiveCR([]float64{0.25, 0.25, 0.25, 0.25}, 1)
	if pack <= 1 || pack >= 2 {
		t.Errorf("four goblins EffectiveCR = %v, want between 1 and 2", pack)
	}
	// Splitting the same pack across a bigger party lowers the per-member rating.
	if duo := encounter.EffectiveCR([]float64{0.25, 0.25, 0.25, 0.25}, 2); duo >= pack {
		t.Errorf("party of 2 EffectiveCR = %v, want below solo %v", duo, pack)
	}
}

func TestGroupDifficulty(t *testing.T) {
	// A lone goblin is fair for a level-1 hero; four of them are deadly.
	if got := encounter.GroupDifficulty([]float64{0.25}, 1, 1); got != encounter.Difficulty(0.25, 1) {
		t.Errorf("one goblin = %q, want same as Difficulty (%q)", got, encounter.Difficulty(0.25, 1))
	}
	if got := encounter.GroupDifficulty([]float64{0.25, 0.25, 0.25, 0.25}, 1, 1); got != "deadly" {
		t.Errorf("four goblins at level 1 = %q, want deadly", got)
	}
}

package skillcheck_test

import (
	"math/rand"
	"testing"

	"pubkey-quest/cmd/server/game/skillcheck"
)

func TestModifierMatchesDnDScale(t *testing.T) {
	cases := map[int]int{4: -3, 8: -1, 9: -1, 10: 0, 11: 0, 12: 1, 14: 2, 18: 4, 20: 5}
	for skill, want := range cases {
		if got := skillcheck.Modifier(skill); got != want {
			t.Errorf("Modifier(%d) = %d, want %d", skill, got, want)
		}
	}
}

func TestResolveStructure(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	r := skillcheck.Resolve(14, 15, rng) // modifier +2
	if r.Modifier != 2 {
		t.Errorf("modifier = %d, want 2", r.Modifier)
	}
	if r.Roll < 1 || r.Roll > 20 {
		t.Errorf("roll out of range: %d", r.Roll)
	}
	if r.Total != r.Roll+r.Modifier {
		t.Errorf("total %d != roll %d + mod %d", r.Total, r.Roll, r.Modifier)
	}
	if r.Success != (r.Total >= 15) {
		t.Errorf("success rule wrong for total %d vs DC 15", r.Total)
	}
}

func TestResolveBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	// +5 vs DC 5: min total is 1+5=6, always succeeds.
	for i := 0; i < 500; i++ {
		if !skillcheck.Resolve(20, 5, rng).Success {
			t.Fatal("skill 20 vs DC 5 should always succeed")
		}
	}
	// -3 vs DC 25: max total is 20-3=17, always fails.
	for i := 0; i < 500; i++ {
		if skillcheck.Resolve(4, 25, rng).Success {
			t.Fatal("skill 4 vs DC 25 should always fail")
		}
	}
}

func TestPassive(t *testing.T) {
	// 10 + Modifier vs DC; skill 14 → modifier +2 → passive 12.
	if !skillcheck.Passive(14, 12) {
		t.Error("passive (12) should meet DC 12")
	}
	if skillcheck.Passive(14, 13) {
		t.Error("passive (12) should not meet DC 13")
	}
}

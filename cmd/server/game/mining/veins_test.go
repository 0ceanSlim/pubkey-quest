package mining

import (
	"errors"
	"math/rand"
	"testing"

	"pubkey-quest/types"
)

func testCfg() *types.MiningConfig {
	return &types.MiningConfig{
		Tiers: []types.MiningTier{{Tier: 1, DC: 10, FacesPerDay: 3, Grid: [2]int{5, 4}, Veins: [2]int{1, 2}, Faults: [2]int{1, 2}, Hard: 0.1, CaveInDice: 1}},
		Ores:  map[string]types.MiningOre{"iron-ore": {Tier: 1, Pick: 1}, "mithral-ore": {Tier: 4, Pick: 3}},
		Rules: types.MiningRules{SwingMinutes: 5, SoundMinutes: 1, QuickMinutes: 30, ExertionPerFatigue: 6},
	}
}

func testSeam(ore string) types.Seam {
	return Effective(testCfg(), types.Seam{Tier: 1, Ores: []types.SeamOre{{Item: ore, Weight: 1, Quantity: [2]int{1, 2}}}})
}

func name(id string) string { return id }

func newFace(t *testing.T, seed int64, pick Pick, ore string) *Session {
	t.Helper()
	s, err := NewSession(testCfg(), "seam", "face", "Test Seam", testSeam(ore), pick, 12, 12, rand.New(rand.NewSource(seed)))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// find is the first cell matching a test, or -1.
func (s *Session) find(f func(cell) bool) int {
	for i, c := range s.cells {
		if f(c) {
			return i
		}
	}
	return -1
}

func TestLayout(t *testing.T) {
	for seed := range int64(50) {
		s := newFace(t, seed, Pick{Tier: 1}, "iron-ore")
		ore, faults := 0, 0
		for _, c := range s.cells {
			switch c.kind {
			case cellOre:
				ore++
			case cellFault:
				faults++
			}
		}
		if ore < 1 || ore > 8 || faults < 1 || faults > 2 {
			t.Fatalf("seed %d: %d ore cells, %d faults", seed, ore, faults)
		}
		if s.SwingsMax != 7 || s.SoundingsMax != 4 { // Athletics/Perception 12 → +1
			t.Fatalf("swings %d, soundings %d", s.SwingsMax, s.SoundingsMax)
		}
	}
}

func TestViewHidesTheRock(t *testing.T) {
	s := newFace(t, 1, Pick{Tier: 1}, "iron-ore")
	for _, c := range s.View(name)["cells"].([]map[string]any) {
		if c["state"] != "unknown" || c["kind"] != nil || c["ore"] != nil {
			t.Fatalf("an untouched face leaked: %v", c)
		}
	}
}

func TestStrikeOreAndStale(t *testing.T) {
	s := newFace(t, 2, Pick{Tier: 2}, "iron-ore")
	i := s.find(func(c cell) bool { return c.kind == cellOre })
	out, err := s.Act(Action{Kind: "strike", Cell: i}, 0, name)
	if err != nil || out.Items["iron-ore"] < 1 || out.Minutes != 5 {
		t.Fatalf("strike ore: %+v %v", out, err)
	}
	if _, err := s.Act(Action{Kind: "strike", Cell: i}, 0, name); !errors.Is(err, ErrStale) {
		t.Fatalf("a replayed move should be stale, got %v", err)
	}
	if _, err := s.Act(Action{Kind: "strike", Cell: i}, 1, name); err == nil {
		t.Fatal("struck a broken cell")
	}
	if v := s.View(name)["cells"].([]map[string]any)[i]; v["state"] != "broken" || v["ore"] != "iron-ore" {
		t.Fatalf("broken ore view: %v", v)
	}
}

func TestHardRockTakesTwoWithAPlainPick(t *testing.T) {
	s := newFace(t, 3, Pick{Tier: 1}, "iron-ore")
	i := s.find(func(c cell) bool { return c.kind != cellFault })
	s.cells[i].hard = true
	s.Act(Action{Kind: "strike", Cell: i}, 0, name)
	if s.cells[i].broken || !s.cells[i].cracked {
		t.Fatal("hard rock broke on the first strike of a tier-1 pick")
	}
	s.Act(Action{Kind: "strike", Cell: i}, 1, name)
	if !s.cells[i].broken {
		t.Fatal("hard rock didn't break on the second strike")
	}
	d := newFace(t, 3, Pick{Tier: 2}, "iron-ore")
	d.cells[i].hard = true
	d.Act(Action{Kind: "strike", Cell: i}, 0, name)
	if !d.cells[i].broken {
		t.Fatal("a dwarven pick should break hard rock in one")
	}
}

func TestThirdCrackCavesIn(t *testing.T) {
	s := newFace(t, 4, Pick{Tier: 1, Swings: 5}, "iron-ore")
	v := 0
	for k := 0; k < MaxCracks; k++ {
		i := s.find(func(c cell) bool { return c.kind == cellFault && !c.broken })
		if i < 0 {
			i = s.find(func(c cell) bool { return c.kind == cellRock })
			s.cells[i].kind = cellFault
		}
		out, err := s.Act(Action{Kind: "strike", Cell: i}, v, name)
		if err != nil {
			t.Fatal(err)
		}
		v++
		if k < MaxCracks-1 && out.Ended {
			t.Fatalf("crack %d ended the face", k+1)
		}
		if k == MaxCracks-1 && (!out.Ended || s.Status != StatusCollapsed || out.Damage < 0 || out.Damage > 6) {
			t.Fatalf("third crack: %+v status %s", out, s.Status)
		}
	}
}

func TestSoundingAndTooHard(t *testing.T) {
	s := newFace(t, 5, Pick{Tier: 1}, "mithral-ore")
	s.Perception = 40 // always reads it
	i := s.find(func(c cell) bool { return c.kind == cellOre })
	out, err := s.Act(Action{Kind: "sound", Cell: i}, 0, name)
	if err != nil || !s.cells[i].read || !s.cells[i].tooHard || out.Minutes != 1 {
		t.Fatalf("sounding mithral with a pickaxe: %+v %v", out, err)
	}
	out, _ = s.Act(Action{Kind: "strike", Cell: i}, 1, name)
	if s.cells[i].broken || len(out.Items) != 0 {
		t.Fatal("a pickaxe broke mithral")
	}
	if _, err := s.Act(Action{Kind: "sound", Cell: i}, 2, name); err == nil {
		t.Fatal("sounded the same cell twice")
	}
}

func TestOutOfSwings(t *testing.T) {
	s := newFace(t, 6, Pick{Tier: 2}, "iron-ore")
	v := 0
	for s.Status == StatusActive {
		i := s.find(func(c cell) bool { return !c.broken && c.kind != cellFault })
		if _, err := s.Act(Action{Kind: "strike", Cell: i}, v, name); err != nil {
			t.Fatal(err)
		}
		v++
	}
	if s.Status != StatusExhausted || s.Swings != 0 || v != s.SwingsMax {
		t.Fatalf("status %s after %d strikes (max %d)", s.Status, v, s.SwingsMax)
	}
}

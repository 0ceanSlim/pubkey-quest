package mining

import (
	"errors"
	"math/rand"
	"testing"

	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/types"
)

func testCfg() *types.MiningConfig {
	return &types.MiningConfig{
		Tiers: []types.MiningTier{{Tier: 1, DC: 10, FacesPerDay: 3, Grid: [2]int{5, 4}, Veins: [2]int{1, 2}, Faults: [2]int{1, 2}, Hard: 0.1, CaveInDice: 1}},
		Ores:  map[string]types.MiningOre{"iron-ore": {Tier: 1, Pick: 1}, "mithral-ore": {Tier: 4, Pick: 3}},
		Rules: types.MiningRules{SwingMinutes: 5, SoundMinutes: 1, QuickMinutes: 30, ExertionPerFatigue: 6, WearPerStrike: 1, WearPerFault: 3},
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

// A worn pick costs swings: Athletics 12 gives 7 with a pristine one.
func TestPickWearCostsSwings(t *testing.T) {
	for wear, want := range []int{7, 6, 5, 3} {
		if got := swingsFor(12, Pick{Tier: 1, Wear: wear}); got != want {
			t.Errorf("wear tier %d: %d swings, want %d", wear, got, want)
		}
	}
	if got := swingsFor(1, Pick{Tier: 1, Wear: WearRuined}); got != 2 {
		t.Errorf("a weak arm and a ruined pick: %d swings, want the floor of 2", got)
	}
}

// Every strike wears the pick; a fault wears it more.
func TestStrikesWearThePick(t *testing.T) {
	s := newFace(t, 3, Pick{Tier: 2}, "iron-ore")
	rock := s.find(func(c cell) bool { return c.kind == cellRock })
	out, err := s.Act(Action{Kind: "strike", Cell: rock}, 0, name)
	if err != nil || out.Wear != 1 {
		t.Fatalf("a strike on rock: wear %d, %v", out.Wear, err)
	}
	fault := s.find(func(c cell) bool { return c.kind == cellFault })
	if out, err = s.Act(Action{Kind: "strike", Cell: fault}, 1, name); err != nil || out.Wear != 3 {
		t.Fatalf("a strike on a fault: wear %d, %v", out.Wear, err)
	}
	if out, _ = s.Act(Action{Kind: "stop"}, 2, name); out.Wear != 0 {
		t.Errorf("stopping wore the pick %d", out.Wear)
	}
}

// A damaged pick needs two strikes on hard rock, whatever its tier.
func TestADamagedPickCracksHardRock(t *testing.T) {
	for seed := range int64(50) {
		s := newFace(t, seed, Pick{Tier: 2, Wear: WearDamaged}, "iron-ore")
		hard := s.find(func(c cell) bool { return c.hard && c.kind != cellFault })
		if hard < 0 {
			continue
		}
		if _, err := s.Act(Action{Kind: "strike", Cell: hard}, 0, name); err != nil {
			t.Fatal(err)
		}
		if !s.cells[hard].cracked || s.cells[hard].broken {
			t.Fatalf("a damaged dwarven pick broke hard rock in one strike")
		}
		return
	}
	t.Fatal("no hard rock in 50 faces")
}

// No light, no reading the rock: a sounding is refused and changes nothing.
func TestNoSoundingInTheDark(t *testing.T) {
	s := newFace(t, 4, Pick{Tier: 1}, "iron-ore")
	s.Dark = true
	if _, err := s.Act(Action{Kind: "sound", Cell: 0}, 0, name); err == nil {
		t.Fatal("sounded in the dark")
	}
	if s.Soundings != s.SoundingsMax || s.Version != 0 || s.Worked {
		t.Errorf("a refused sounding changed the face: %d soundings, version %d", s.Soundings, s.Version)
	}
	if s.View(name)["dark"] != true {
		t.Error("the view doesn't say it's dark")
	}
	// Swinging blind still works.
	if _, err := s.Act(Action{Kind: "strike", Cell: 0}, 0, name); err != nil {
		t.Fatalf("strike in the dark: %v", err)
	}
}

// gemFace is a face whose seam leans to rubies, with two gem cells.
func gemFace(t *testing.T, seed int64) *Session {
	t.Helper()
	gc, err := gems.LoadFile("../../../../game-data/systems/gems.json")
	if err != nil {
		t.Fatal(err)
	}
	gc.Mining.GeodeShare = 0
	gems.SetConfig(gc)
	t.Cleanup(func() { gems.SetConfig(&types.GemsConfig{}) })
	seam := testSeam("iron-ore")
	seam.Gems = &types.SeamGems{Types: map[string]int{"ruby": 1}, Cells: [2]int{2, 2}}
	s, err := NewSession(testCfg(), "seam", "face", "Test Seam", seam, Pick{Tier: 1}, 12, 12, rand.New(rand.NewSource(seed)))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// A gem glints once a cell beside it is broken, and prying frees it whole; in
// the dark it doesn't show and can't be pried.
func TestPryAGlintingGem(t *testing.T) {
	s := gemFace(t, 3)
	g := s.find(func(c cell) bool { return c.kind == cellGem })
	if g < 0 || s.cells[g].gem.Type != "ruby" {
		t.Fatalf("no ruby cell: %+v", s.cells)
	}
	if _, err := s.Act(Action{Kind: "pry", Cell: g}, s.Version, name); err == nil {
		t.Fatal("pried a gem nobody can see")
	}
	s.cells[s.around(g)[0]].broken = true
	if v := s.View(name)["cells"].([]map[string]any)[g]; v["glint"] != true {
		t.Fatalf("an exposed gem should glint: %v", v)
	}
	s.Dark = true
	if _, err := s.Act(Action{Kind: "pry", Cell: g}, s.Version, name); err == nil {
		t.Fatal("pried in the dark")
	}
	s.Dark = false
	want := s.cells[g].ore
	out, err := s.Act(Action{Kind: "pry", Cell: g}, s.Version, name)
	if err != nil || out.Items[want] != 1 || out.Wear == 0 || out.Minutes == 0 {
		t.Fatalf("pry: %+v %v", out, err)
	}
}

// Struck blind, a gem chips a size down; a tiny one is lost.
func TestStrikingAGemChipsIt(t *testing.T) {
	s := gemFace(t, 5)
	g := s.find(func(c cell) bool { return c.kind == cellGem })
	s.cells[g].gem.Size = "small"
	out, err := s.Act(Action{Kind: "strike", Cell: g}, s.Version, name)
	if err != nil || out.Items[types.UncutGemID("ruby", "tiny")] != 1 {
		t.Fatalf("strike: %+v %v", out, err)
	}
	g2 := s.find(func(c cell) bool { return c.kind == cellGem && !c.broken })
	s.cells[g2].gem.Size = "tiny"
	out, err = s.Act(Action{Kind: "strike", Cell: g2}, s.Version, name)
	if err != nil || len(out.Items) != 0 {
		t.Fatalf("a tiny gem struck blind should be lost: %+v %v", out, err)
	}
	if v := s.View(name)["cells"].([]map[string]any)[g2]; v["ore"] != nil {
		t.Fatalf("a shattered gem shows as %v", v)
	}
}

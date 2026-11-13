package fishing

import (
	"math/rand"
	"testing"

	"pubkey-quest/types"
)

func loadCfg(t *testing.T) *types.FishingConfig {
	t.Helper()
	cfg, err := LoadFile("../../../../game-data/systems/fishing.json")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

var dayHours = [2]int{360, 1200}

func noon() Conditions {
	return Conditions{TimeOfDay: 720, Day: dayHours, Survival: 12, Athletics: 12, Bait: "worms"}
}
func midnight() Conditions { c := noon(); c.TimeOfDay = 0; return c }

func TestGeneratedItems(t *testing.T) {
	cfg := loadCfg(t)
	items := Items(cfg)
	want := len(cfg.Species)*(len(cfg.Sizes)+1) + 2*len(cfg.Sizes)
	if len(items) != want {
		t.Errorf("%d generated items, want %d", len(items), want)
	}
	byID := map[string]map[string]any{}
	for _, it := range items {
		byID[it["id"].(string)] = it
	}
	cod := byID["fish-cod-huge"]
	if cod == nil || cod["weight"].(float64) != 54 || cod["value"].(int) != 360 {
		t.Errorf("huge cod: %v", cod)
	}
	if byID["fish-cod-huge"]["cook"] != "cooked-fish-huge" || byID["cooked-fish-huge"] == nil {
		t.Error("a huge cod should cook into cooked-fish-huge")
	}
	if byID["trophy-goldscale"] == nil {
		t.Error("no goldscale trophy")
	}
}

// Day biters don't bite at night; night biters don't bite at noon.
func TestBitesByHour(t *testing.T) {
	cfg := loadCfg(t)
	spot := cfg.Spots["goldenhaven-wharf"]
	if _, ok := pool(cfg, spot, noon())["sturgeon"]; ok {
		t.Error("sturgeon biting at noon")
	}
	if _, ok := pool(cfg, spot, midnight())["sturgeon"]; !ok {
		t.Error("sturgeon not biting at midnight")
	}
	if _, ok := pool(cfg, spot, midnight())["river-perch"]; ok {
		t.Error("perch biting at midnight")
	}
}

// Any bait beats a bare hook; a fished-out spot doesn't bite.
func TestBiteChance(t *testing.T) {
	cfg := loadCfg(t)
	spot := cfg.Spots["millpond"]
	c := noon()
	worms := BiteChance(cfg, spot, c)
	c.Bait = ""
	bare := BiteChance(cfg, spot, c)
	c.Bait = "spoon" // nothing at the millpond much likes a spoon
	spoon := BiteChance(cfg, spot, c)
	if !(worms > spoon && spoon > bare && bare > 0) {
		t.Errorf("worms %.3f > spoon %.3f > bare hook %.3f > 0", worms, spoon, bare)
	}
	c.Depletion = 0.0001
	if BiteChance(cfg, spot, c) > 0.001 {
		t.Error("a fished-out spot still bites")
	}
}

// fightWith hooks a fish of a species and size on a plain rod.
func fightWith(t *testing.T, cfg *types.FishingConfig, species, size string, seed int64) *Session {
	t.Helper()
	s, err := NewSession(cfg, "goldenhaven-wharf", Gear{Line: 10, Cast: 6}, noon(), rand.New(rand.NewSource(seed)))
	if err != nil {
		t.Fatal(err)
	}
	s.bite = &Fish{Species: species, Size: size, Lb: 10}
	var out Outcome
	s.hook(&out)
	return s
}

// The worked turn in the draft: a medium pike (power 4) runs twice; reeling
// both times snaps a plain rod's line, and loses the lure with it.
func TestRunAndReelSnaps(t *testing.T) {
	cfg := loadCfg(t)
	s := fightWith(t, cfg, "river-pike", "medium", 1)
	s.Tension, s.move = 2, MoveRun
	out, _ := s.Act("reel", s.Version)
	if s.Tension != 7 {
		t.Fatalf("tension %d after the first reel, want 7", s.Tension)
	}
	s.move = MoveRun
	out, _ = s.Act("reel", s.Version)
	if !out.Escaped || !out.LureLost || s.Phase != PhaseDone {
		t.Errorf("the second reel on a run should snap the line: %+v", out)
	}
}

// Giving line on a thrash leaves it slack, and it throws the hook (the lure stays).
func TestThrashOnSlackThrows(t *testing.T) {
	cfg := loadCfg(t)
	s := fightWith(t, cfg, "quipper", "small", 2)
	s.Tension, s.move = 2, MoveThrash
	out, _ := s.Act("give", s.Version)
	if !out.Escaped || out.LureLost {
		t.Errorf("give on a thrash should throw the hook, not snap: %+v", out)
	}
}

// play fights a fish the way the draft teaches: give against runs, hold
// through thrashes, reel on rests and dives.
func play(s *Session) Outcome {
	for range 200 {
		act := "reel"
		switch s.move {
		case MoveRun:
			act = "give"
			if s.Tension <= 4 {
				act = "hold"
			}
		case MoveThrash:
			act = "hold"
		}
		if s.Tension >= s.Gear.Line-3 {
			act = "give"
		}
		out, err := s.Act(act, s.Version)
		if err != nil || s.Phase != PhaseFight {
			return out
		}
	}
	return Outcome{}
}

// Played well, a medium fish usually lands; huge strong ones usually don't on
// a plain rod. So what you land skews smaller than what bites.
func TestLandingSkewsSmall(t *testing.T) {
	cfg := loadCfg(t)
	landed := map[string]int{}
	for seed := int64(0); seed < 300; seed++ {
		for _, size := range []string{"medium", "huge"} {
			if out := play(fightWith(t, cfg, "cod", size, seed)); out.Landed != nil {
				landed[size]++
			}
		}
	}
	if landed["medium"] < 200 {
		t.Errorf("only %d/300 medium cod landed", landed["medium"])
	}
	if landed["huge"] >= landed["medium"] {
		t.Errorf("huge cod landed as often as medium (%d vs %d)", landed["huge"], landed["medium"])
	}
}

// Barehanded, rod species come up tiny or small; barehand-only ones at any size.
func TestBarehandCap(t *testing.T) {
	cfg := loadCfg(t)
	spot := cfg.Spots["cragspire-falls"]
	c := noon()
	c.TimeOfDay, c.Bait, c.Barehand = 360, "", true // dawn: salmon are leaping
	rng := rand.New(rand.NewSource(7))
	small := SizeIndex(cfg, "small")
	bigSalmon := false
	for range 2000 {
		f := RollBite(cfg, spot, c, rng)
		if f == nil {
			continue
		}
		i := SizeIndex(cfg, f.Size)
		if f.Species == "leaping-salmon" {
			bigSalmon = bigSalmon || i > small
		} else if i > small {
			t.Fatalf("a barehand %s %s", f.Size, f.Species)
		}
		if cfg.Species[f.Species].Shy {
			t.Fatalf("grabbed a shy %s", f.Species)
		}
	}
	if !bigSalmon {
		t.Error("barehand-only salmon should come in every size")
	}
}

// A grab is an Athletics check; there's no fight.
func TestGrab(t *testing.T) {
	cfg := loadCfg(t)
	c := noon()
	c.Barehand, c.Bait, c.Athletics = true, "", 50 // can't miss
	s, err := NewSession(cfg, "cragspire-falls", Gear{}, c, rand.New(rand.NewSource(3)))
	if err != nil {
		t.Fatal(err)
	}
	s.bite, s.Float = &Fish{Species: "leaping-salmon", Size: "large", Lb: 20}, FloatDive
	out, _ := s.Act("strike", s.Version)
	if out.Landed == nil || out.Landed.Species != "leaping-salmon" || s.Phase != PhaseDone {
		t.Errorf("the grab should land it: %+v", out)
	}
}

// A move made against an old version changes nothing.
func TestStaleVersion(t *testing.T) {
	cfg := loadCfg(t)
	s, _ := NewSession(cfg, "millpond", Gear{Line: 10, Cast: 6}, noon(), rand.New(rand.NewSource(1)))
	if _, err := s.Act("wait", s.Version); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Act("wait", 0); err != ErrStale {
		t.Errorf("replayed move: %v", err)
	}
}

// The species and size stay hidden until it's landed.
func TestViewHidesTheFish(t *testing.T) {
	cfg := loadCfg(t)
	s := fightWith(t, cfg, "sturgeon", "huge", 4)
	v := s.View()
	for _, k := range []string{"catch", "species", "size"} {
		if _, ok := v[k]; ok {
			t.Errorf("view shows %q mid-fight", k)
		}
	}
	if v["feel"] != "monstrous" {
		t.Errorf("a huge sturgeon should feel monstrous, got %v", v["feel"])
	}
}

func TestDepletion(t *testing.T) {
	m := Memory{}
	for range 12 {
		m.Landed("millpond", 3)
	}
	if f := m.Factor("millpond", 3, 12); f != 0.5 {
		t.Errorf("after daily_catch: %v, want 0.5", f)
	}
	for range 12 {
		m.Landed("millpond", 3)
	}
	if f := m.Factor("millpond", 3, 12); f != 0 {
		t.Errorf("after twice daily_catch: %v, want 0", f)
	}
	if f := m.Factor("millpond", 4, 12); f != 1 {
		t.Errorf("the next day: %v, want 1", f)
	}
}

// Quick fishing never lands anything bigger than medium.
func TestQuickCap(t *testing.T) {
	cfg := loadCfg(t)
	c := noon()
	c.Survival = 30
	rng := rand.New(rand.NewSource(9))
	for range 500 {
		if r := Quick(cfg, cfg.Spots["saltwind-harbor"], c, rng); r.Fish != nil && SizeIndex(cfg, r.Fish.Size) > SizeIndex(cfg, "medium") {
			t.Fatalf("a quick %s", r.Fish.Size)
		}
	}
}

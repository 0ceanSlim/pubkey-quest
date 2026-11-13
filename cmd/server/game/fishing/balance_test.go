package fishing

import "testing"

// A balance check on the fight: an angler who reads every move (as if the tell
// always showed) and keeps a turn's safety margin. Everyday fish should land
// almost every time; a huge goldscale (power 9) mostly beats a plain rod, and
// the Brackwater rod's stronger line is what gives it a real chance.
func TestFightBalance(t *testing.T) {
	cfg := loadCfg(t)
	rate := func(species, size string, line int) float64 {
		landed := 0
		for seed := int64(0); seed < 400; seed++ {
			s := fightWith(t, cfg, species, size, seed)
			s.Gear.Line = line
			for n := 0; s.Phase == PhaseFight && n < 200; n++ {
				readingAngler(s)
			}
			if s.last != nil {
				landed++
			}
		}
		return float64(landed) / 400
	}
	for _, c := range []struct {
		species, size string
		line          int
		min, max      float64
	}{
		{"minnow", "small", 10, 0.95, 1},
		{"river-perch", "medium", 10, 0.95, 1},
		{"cod", "large", 10, 0.95, 1},
		{"cod", "huge", 10, 0.85, 1},
		{"goldscale", "huge", 10, 0, 0.3},
		{"goldscale", "huge", 13, 0.2, 0.7},
	} {
		if r := rate(c.species, c.size, c.line); r < c.min || r > c.max {
			t.Errorf("%s %s on line %d lands %.0f%%, want %.0f–%.0f%%", c.size, c.species, c.line, r*100, c.min*100, c.max*100)
		}
	}
}

// readingAngler picks the move that keeps the line safe through the worst next
// turn (a run while giving line) and brings the fish closest.
func readingAngler(s *Session) {
	f := *s.bite
	p := power(s.cfg, f)
	if s.Spent {
		p /= 2
	}
	best, bestScore := "give", -1e9
	for _, act := range []string{"reel", "hold", "give"} {
		pull, run := 0, 0
		switch s.move {
		case MoveRun:
			pull, run = p+2, 1
		case MoveDive:
			pull = p
		case MoveThrash:
			pull = p + 1
		}
		strain, reel, tire := 0, 0, 0
		switch act {
		case "reel":
			strain, tire = 2, 1
			switch s.move {
			case MoveRun:
			case MoveDive:
				reel = 1
			default:
				reel = 2
			}
		case "hold":
			strain, tire = 1, 2
		case "give":
			strain, reel = -5, -1
		}
		T := max(0, s.Tension-3+pull+strain)
		if act != "give" {
			T = max(1, T)
		}
		D := s.Distance + run - reel
		score := -2*float64(D) + 1.5*float64(tire) - 0.5*float64(T)
		if max(0, T-3+power(s.cfg, f)+2-5) >= s.Gear.Line || D+2 >= s.cfg.Rules.MaxDistance {
			score -= 1000
		}
		if T >= s.Gear.Line || D >= s.cfg.Rules.MaxDistance {
			score -= 1e6
		}
		if T == 0 && (s.move == MoveThrash || s.slack >= 1) {
			score -= 5000
		}
		if score > bestScore {
			best, bestScore = act, score
		}
	}
	s.Act(best, s.Version)
}

// Package encounter rolls random monster encounters by environment biome,
// scaled to the player's level. This is the generic, scheduled travel-combat
// source — distinct from the hand-authored encounter vignettes — and the thing
// that finally gives organic combat entry: while travelling a forest you might
// just get jumped by a wolf. The roller is pure and RNG-injectable; the call
// site (the travel tick) supplies the biome's monster pool and starts combat.
package encounter

import "math/rand"

// Candidate is a monster eligible to be rolled for a biome encounter.
type Candidate struct {
	ID   string
	Name string
	CR   float64
}

// Difficulty tunables — alpha-rough on purpose. The roadmap defers full scaling
// math to beta, so these are meant to be adjusted by feel after playtesting;
// they should season travel, not dominate it.
const (
	// CooldownMinutes is the minimum in-game time between biome encounters, so
	// they can't fire back-to-back after a fight. The roll is skipped within this
	// window of the last encounter (enforced at the call site, which has the
	// session timing).
	CooldownMinutes = 90
	// chancePerMinute is the per-in-game-minute encounter probability. At ~0.006
	// (with the cooldown above) travel averages a couple of fights per crossing.
	chancePerMinute = 0.006
	// maxTickChance caps a single tick so one large time jump can't guarantee a
	// fight (e.g. resuming travel after a long idle).
	maxTickChance = 0.5
	// crBudgetPerLevel sets the upper CR a level-L player is matched against
	// (0.75 → level 1 faces CR ≤ ~0.75, level 5 ≤ ~3.75).
	crBudgetPerLevel = 0.75
	// minCRCap keeps a usable pool at level 1 even if crBudgetPerLevel*level is
	// tiny, so the lowest-CR monsters are always eligible.
	minCRCap = 0.5
)

// CRCap is the highest monster CR a player of the given level is matched
// against. Exposed so callers (and a future "deadly fight" warning) can reason
// about the band.
func CRCap(level int) float64 {
	cap := float64(level) * crBudgetPerLevel
	if cap < minCRCap {
		cap = minCRCap
	}
	return cap
}

// Difficulty classifies a fight for the player relative to the CR band their
// level is matched against (CRCap). Random encounters stay inside the band
// (≤ "fair"); hand-authored / POI monsters can exceed it, which is exactly when
// the "deadly fight" warning earns its keep (M5 §22). Bands are alpha-rough on
// purpose — the roadmap defers full encounter math to beta.
func Difficulty(cr float64, level int) string {
	cap := CRCap(level)
	if cap <= 0 {
		cap = minCRCap
	}
	switch r := cr / cap; {
	case r < 0.35:
		return "trivial"
	case r < 0.75:
		return "easy"
	case r <= 1.05:
		return "fair"
	case r <= 1.75:
		return "tough"
	default:
		return "deadly"
	}
}

// IsDeadly reports whether a monster meaningfully outclasses the player's level.
func IsDeadly(cr float64, level int) bool { return Difficulty(cr, level) == "deadly" }

// Eligible returns the candidates whose CR fits the player's level band.
func Eligible(candidates []Candidate, level int) []Candidate {
	cap := CRCap(level)
	var out []Candidate
	for _, c := range candidates {
		if c.CR <= cap {
			out = append(out, c)
		}
	}
	return out
}

// TickChance is the probability an encounter fires given how much in-game time
// elapsed this tick, capped so a big jump can't guarantee one.
func TickChance(minutesElapsed int) float64 {
	if minutesElapsed <= 0 {
		return 0
	}
	ch := chancePerMinute * float64(minutesElapsed)
	if ch > maxTickChance {
		ch = maxTickChance
	}
	return ch
}

// Roll decides whether a biome encounter fires this tick and, if so, which
// monster the player meets. It returns ok=false when no CR-eligible monster
// exists for the biome or the chance roll misses. rng is injected so callers
// can seed it and tests can make it deterministic.
//
// avoidID is the previous encounter's monster id: when the eligible pool has
// alternatives, it's excluded so you don't fight the exact same monster twice in
// a row (the biggest driver of "same fight over and over"). With only one
// eligible monster it's ignored — there's nothing else to pick.
func Roll(candidates []Candidate, level, minutesElapsed int, rng *rand.Rand, avoidID string) (Candidate, bool) {
	eligible := Eligible(candidates, level)
	if len(eligible) == 0 {
		return Candidate{}, false
	}
	if rng.Float64() >= TickChance(minutesElapsed) {
		return Candidate{}, false
	}
	if avoidID != "" && len(eligible) > 1 {
		var fresh []Candidate
		for _, c := range eligible {
			if c.ID != avoidID {
				fresh = append(fresh, c)
			}
		}
		if len(fresh) > 0 {
			eligible = fresh
		}
	}
	return eligible[rng.Intn(len(eligible))], true
}

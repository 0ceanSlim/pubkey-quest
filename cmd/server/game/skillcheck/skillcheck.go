// Package skillcheck resolves soft (rolled) skill checks — d20 + modifier vs DC.
//
// This is deliberately distinct from hard requirement gates (the requirement
// evaluator, which is deterministic "can you attempt this?"). A skill check is
// the random "do you succeed?" — used by POI check nodes, the quest "check"
// objective, and the passive perception path of POI discovery.
//
// The skill value passed in must be the player's EFFECTIVE skill (base stats +
// active effect modifiers); the modifier is derived from it the way D&D derives
// an ability modifier from an ability score, so authored DCs stay on the
// familiar 5/10/15/20/25 scale.
package skillcheck

import (
	"math"
	"math/rand"
)

// Modifier turns a 0–20 skill score into a check modifier: floor((score-10)/2).
// Score 10 → +0, 14 → +2, 18 → +4, 20 → +5; below 10 goes negative. Keeping the
// modifier small (vs adding the whole score) keeps the d20 roll meaningful.
func Modifier(skillValue int) int {
	return int(math.Floor(float64(skillValue-10) / 2))
}

// Result is the outcome of an active check.
type Result struct {
	Roll     int // the raw d20 (1–20)
	Modifier int // the skill modifier applied
	Total    int // roll + modifier
	DC       int
	Success  bool
}

// Resolve makes an active skill check: d20 + Modifier(skill) vs DC. rng is
// injected so callers can seed it and tests can be deterministic.
func Resolve(skillValue, dc int, rng *rand.Rand) Result {
	roll := rng.Intn(20) + 1
	mod := Modifier(skillValue)
	total := roll + mod
	return Result{Roll: roll, Modifier: mod, Total: total, DC: dc, Success: total >= dc}
}

// Passive makes a passive check (no roll): 10 + Modifier(skill) vs DC. Used for
// background awareness — e.g. noticing a hidden POI while travelling past it.
func Passive(skillValue, dc int) bool {
	return 10+Modifier(skillValue) >= dc
}

package combat

import (
	"fmt"

	"pubkey-quest/types"
)

// ─── Spell range ─────────────────────────────────────────────────────────────
//
// A spell's `range` is how many grid cells (Chebyshev) it reaches the creature
// it's aimed at. "0" covers both self and touch: a range-0 spell cast on
// yourself needs no reach, and one aimed at anyone else is a touch, reaching
// only an adjacent cell. A range-0 spell with a `range_long` is thrown (produce
// flame: the flame is conjured in your hand and hurled), and reaches that far.
// Otherwise `range_long` plays no part: spells, unlike bows, have no long-range
// band, so `range` is the hard limit.
//
// An area centred on the caster (self_radius, cone, line) reaches as far as its
// own size, which aimArea checks; only a sphere is placed at range.

// spellReach is how many cells a spell reaches the creature it's aimed at.
func spellReach(spell map[string]interface{}) int {
	if r := spellRange(spell); r > 0 {
		return r
	}
	if long := parseRangeInt(spell["range_long"]); long > 0 {
		return long
	}
	return 1 // touch
}

// spellNeedsReach reports whether a spell's range limits who it can be aimed at
// (false for an area that spreads from the caster).
func spellNeedsReach(spell map[string]interface{}) bool {
	a := spellAreaOf(spell)
	return a == nil || a.Shape == "sphere"
}

// aimsAtAlly reports whether a spell does something to the one ally it's aimed
// at (heals, cures, a buff on that ally), so their distance matters. A self
// buff ignores who's picked; a party buff picks its own.
func aimsAtAlly(spell map[string]interface{}, buff *types.SpellBuff) bool {
	if buff != nil && buff.Target == "ally" {
		return true
	}
	if h, _ := spell["heal"].(string); h != "" {
		return true
	}
	cures, _ := spell["cures"].([]interface{})
	return len(cures) > 0
}

// checkSpellReach refuses a spell aimed at someone standing at to, by name,
// from a caster at from, when they're beyond its reach.
func checkSpellReach(spell map[string]interface{}, spellName string, from, to types.Position, targetName string) error {
	if !spellNeedsReach(spell) {
		return nil
	}
	reach, dist := spellReach(spell), chebyshev(from, to)
	if dist <= reach {
		return nil
	}
	if spellRange(spell) == 0 && reach == 1 {
		return fmt.Errorf("%s is a touch spell: move next to %s first", spellName, targetName)
	}
	return fmt.Errorf("%s is %d ft away, and %s reaches only %d ft", targetName, dist*5, spellName, reach*5)
}

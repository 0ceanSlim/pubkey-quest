package encounter

import "sort"

// Group difficulty (M5.6 P1) — rating a fight against several monsters.
//
// A pack is more dangerous than its strongest member: five CR ¼ goblins are not a
// CR ¼ fight. This uses the D&D 5e encounter-building method — sum the monsters'
// XP, scale by a multiplier for how many there are (action economy), split across
// the party — then converts that budget back into an equivalent single-monster CR
// so the result rates on the same band as Difficulty.

// crXP is the 5e XP value of each challenge rating.
var crXP = []struct {
	cr float64
	xp float64
}{
	{0, 10}, {0.125, 25}, {0.25, 50}, {0.5, 100},
	{1, 200}, {2, 450}, {3, 700}, {4, 1100}, {5, 1800},
	{6, 2300}, {7, 2900}, {8, 3900}, {9, 5000}, {10, 5900},
	{11, 7200}, {12, 8400}, {13, 10000}, {14, 11500}, {15, 13000},
	{16, 15000}, {17, 18000}, {18, 20000}, {19, 22000}, {20, 25000},
	{21, 33000}, {22, 41000}, {23, 50000}, {24, 62000}, {30, 155000},
}

// XPForCR returns the 5e XP value of a challenge rating, interpolating between
// table rows for off-table values.
func XPForCR(cr float64) float64 {
	if cr <= crXP[0].cr {
		return crXP[0].xp
	}
	for i := 1; i < len(crXP); i++ {
		if cr <= crXP[i].cr {
			lo, hi := crXP[i-1], crXP[i]
			t := (cr - lo.cr) / (hi.cr - lo.cr)
			return lo.xp + t*(hi.xp-lo.xp)
		}
	}
	return crXP[len(crXP)-1].xp
}

// CRForXP is the inverse of XPForCR: the (interpolated) CR worth this much XP.
func CRForXP(xp float64) float64 {
	if xp <= crXP[0].xp {
		return crXP[0].cr
	}
	i := sort.Search(len(crXP), func(i int) bool { return crXP[i].xp >= xp })
	if i >= len(crXP) {
		return crXP[len(crXP)-1].cr
	}
	lo, hi := crXP[i-1], crXP[i]
	t := (xp - lo.xp) / (hi.xp - lo.xp)
	return lo.cr + t*(hi.cr-lo.cr)
}

// groupMultiplier is the 5e encounter multiplier for the number of monsters.
func groupMultiplier(count int) float64 {
	switch {
	case count <= 1:
		return 1
	case count == 2:
		return 1.5
	case count <= 6:
		return 2
	case count <= 10:
		return 2.5
	case count <= 14:
		return 3
	default:
		return 4
	}
}

// EffectiveCR collapses a group of monsters into the single CR whose XP equals
// the group's adjusted XP per party member. One monster against one character
// is just its own CR.
func EffectiveCR(crs []float64, partySize int) float64 {
	if len(crs) == 0 {
		return 0
	}
	if partySize < 1 {
		partySize = 1
	}
	if len(crs) == 1 && partySize == 1 {
		return crs[0]
	}
	total := 0.0
	for _, cr := range crs {
		total += XPForCR(cr)
	}
	adjusted := total * groupMultiplier(len(crs)) / float64(partySize)
	return CRForXP(adjusted)
}

// GroupDifficulty rates a fight against several monsters for a party of
// partySize at the given level, on the same trivial…deadly band as Difficulty.
func GroupDifficulty(crs []float64, level, partySize int) string {
	return Difficulty(EffectiveCR(crs, partySize), level)
}

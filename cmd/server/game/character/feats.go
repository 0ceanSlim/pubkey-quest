package character

import (
	"fmt"
	"sort"
	"strings"

	"pubkey-quest/types"
)

// Feats — the feat-or-ability-point choice (docs/draft/feats-progression.md).
//
// At a class's feat-eligible levels a character may take a feat instead of that
// level's ability point. The choice is stored hydration-first: only FeatsChosen
// (feat ids; half-feats suffix the chosen stat, e.g. "resilient:constitution").
// A feat's stat grant is baked into Stats exactly like an ability point, and the
// accounting (UnspentAbilityPoints = cadence − len(FeatsChosen) − Σincreases)
// already treats each feat as consuming one point.

// FeatLevelsSorted returns a class's feat-eligible levels in ascending order (the
// set itself is FeatEligibleLevels in levelguide.go). Every entry is also an
// ability-point cadence level, so a feat cleanly consumes a point.
func FeatLevelsSorted(class string) []int {
	set := FeatEligibleLevels(class)
	out := make([]int, 0, len(set))
	for l, ok := range set {
		if ok {
			out = append(out, l)
		}
	}
	sort.Ints(out)
	return out
}

// featSlotsReached counts a class's feat-eligible levels at or below level.
func featSlotsReached(class string, level int) int {
	n := 0
	for l, ok := range FeatEligibleLevels(class) {
		if ok && l <= level {
			n++
		}
	}
	return n
}

// FeatSlotsAvailable is how many more feats the character may take right now:
// feat-eligible levels reached minus feats already chosen.
func FeatSlotsAvailable(save *types.SaveFile, advancement []types.AdvancementEntry) int {
	if save == nil {
		return 0
	}
	level := GetLevelFromXP(save.Experience, advancement)
	avail := featSlotsReached(save.Class, level) - len(save.FeatsChosen)
	if avail < 0 {
		return 0
	}
	return avail
}

// FeatBaseID splits a stored feat entry into its base id and the chosen stat.
// Stored as "id" (fixed feats) or "id:constitution" (a half-feat's chosen stat).
func FeatBaseID(stored string) (id, choice string) {
	if i := strings.IndexByte(stored, ':'); i >= 0 {
		return stored[:i], stored[i+1:]
	}
	return stored, ""
}

// HasFeat reports whether the character has taken the feat with the given base id.
func HasFeat(save *types.SaveFile, baseID string) bool {
	if save == nil {
		return false
	}
	for _, f := range save.FeatsChosen {
		if id, _ := FeatBaseID(f); id == baseID {
			return true
		}
	}
	return false
}

// FeatChoice returns the stored choice for a taken feat (the stat for a half-feat,
// or the damage type for Elemental Adept), or "" if the feat isn't taken / has no
// choice.
func FeatChoice(save *types.SaveFile, baseID string) string {
	if save == nil {
		return ""
	}
	for _, f := range save.FeatsChosen {
		if id, choice := FeatBaseID(f); id == baseID {
			return choice
		}
	}
	return ""
}

// featMaxHPBonus is the extra max HP granted by feats (Tough: +2 per level). Kept
// derived from FeatsChosen so MaxHP stays a pure derivation (§4 hydration).
func featMaxHPBonus(save *types.SaveFile, level int) int {
	bonus := 0
	if HasFeat(save, "tough") {
		bonus += 2 * level
	}
	return bonus
}

// ChooseFeat commits a feat at the level-up moment: validates a slot is available,
// the feat isn't already taken, its prerequisite is met, and (for half-feats) the
// stat choice is valid; then bakes the stat grant into Stats (like an ability
// point), records the feat in FeatsChosen (with the chosen stat suffixed), and
// re-hydrates so derived maxima (Max HP for Durable/Tough) update.
func ChooseFeat(save *types.SaveFile, feat *types.Feat, choice string, advancement []types.AdvancementEntry) error {
	if save == nil || feat == nil {
		return fmt.Errorf("nil save or feat")
	}
	if HasFeat(save, feat.ID) {
		return fmt.Errorf("you already have the %s feat", feat.Name)
	}
	if FeatSlotsAvailable(save, advancement) <= 0 {
		return fmt.Errorf("no feat available yet — reach a feat level (%v for a %s)", FeatLevelsSorted(save.Class), save.Class)
	}
	if !featPrereqMet(save, feat) {
		return fmt.Errorf("you don't meet the prerequisite for %s (%s)", feat.Name, feat.Prerequisite)
	}

	stored := feat.ID

	// Apply the stat grant (half-feats need a valid, in-range choice).
	if feat.StatGrant != nil && len(feat.StatGrant.Choices) > 0 {
		chosen := feat.StatGrant.Choices[0] // fixed grant when there's only one option
		if len(feat.StatGrant.Choices) > 1 {
			if choice == "" {
				return fmt.Errorf("%s requires a stat choice (%v)", feat.Name, feat.StatGrant.Choices)
			}
			if !containsFold(feat.StatGrant.Choices, choice) {
				return fmt.Errorf("%s cannot grant %s", feat.Name, choice)
			}
			chosen = choice
		}
		canon := CanonicalAbility(chosen)
		if canon == "" {
			return fmt.Errorf("invalid stat %q", chosen)
		}
		if cur := statScore(save.Stats, canon); cur < AbilityScoreMax {
			next := cur + feat.StatGrant.Amount
			if next > AbilityScoreMax {
				next = AbilityScoreMax
			}
			setStatScore(save, canon, next)
		}
		stored = feat.ID + ":" + strings.ToLower(canon)
	} else if feat.Choice != nil && len(feat.Choice.Options) > 0 {
		// A non-stat pick (e.g. Elemental Adept's damage type) — validated + stored
		// on the id, nothing baked into Stats; the effect hook reads FeatChoice.
		if choice == "" {
			return fmt.Errorf("%s requires a %s choice (%v)", feat.Name, feat.Choice.Kind, feat.Choice.Options)
		}
		if !containsFold(feat.Choice.Options, choice) {
			return fmt.Errorf("%s cannot choose %q", feat.Name, choice)
		}
		stored = feat.ID + ":" + strings.ToLower(choice)
	}

	save.FeatsChosen = append(save.FeatsChosen, stored)
	Hydrate(save, advancement)
	return nil
}

// featPrereqMet checks a feat's prerequisite. Alpha MVP feats have none; the
// "spellcaster" prereq (for the later casting feats) checks the class can cast.
func featPrereqMet(save *types.SaveFile, feat *types.Feat) bool {
	switch strings.ToLower(strings.TrimSpace(feat.Prerequisite)) {
	case "", "none":
		return true
	case "spellcaster":
		return IsCaster(save.Class)
	default:
		return true
	}
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

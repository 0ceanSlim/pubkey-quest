package character

import (
	"fmt"
	"strings"

	"pubkey-quest/types"
)

// Ability-point allocation (§4 hydration rule, M1 progression).
//
// Characters earn ability points on a fixed level cadence (advancement.json's
// per-row AbilityPoints) and spend them to raise ability scores. Only the
// player's *choices* are stored — AbilityIncreases (points spent per ability)
// and FeatsChosen (reserved). The unspent count is always derived, never stored,
// so nothing derivable is frozen into a player-signed save. See
// docs/draft/feats-progression.md for the full cadence and the feat/point trade.

// AbilityScoreMax is the ceiling a single ability can reach by spending earned
// ability points. Generation caps scores at 16 (StatMax); spending lifts a stat
// up to this D&D-standard cap.
const AbilityScoreMax = 20

// abilityCanonical maps any accepted spelling of an ability (full name or the
// three-letter abbreviation, any case) to the canonical capitalized key used in
// a save's Stats map.
var abilityCanonical = map[string]string{
	"strength": "Strength", "str": "Strength",
	"dexterity": "Dexterity", "dex": "Dexterity",
	"constitution": "Constitution", "con": "Constitution",
	"intelligence": "Intelligence", "int": "Intelligence",
	"wisdom": "Wisdom", "wis": "Wisdom",
	"charisma": "Charisma", "cha": "Charisma",
}

// CanonicalAbility returns the canonical ability name for an input, or "" if the
// input names no valid ability.
func CanonicalAbility(name string) string {
	return abilityCanonical[strings.ToLower(strings.TrimSpace(name))]
}

// AbilityPointsEarned returns the total ability points granted by the
// advancement table up to and including the given level.
func AbilityPointsEarned(level int, advancement []types.AdvancementEntry) int {
	total := 0
	for _, e := range advancement {
		if e.Level <= level {
			total += e.AbilityPoints
		}
	}
	return total
}

// UnspentAbilityPoints derives how many ability points a character has banked but
// not yet spent. Per the §4 hydration rule the save stores only the player's
// choices (AbilityIncreases, FeatsChosen); the unspent count is never stored:
//
//	unspent = earned(level) − feats taken − points already allocated
//
// Each feat consumes one cadence point (the feat-or-point choice — see
// docs/draft/feats-progression.md), so taking a feat reduces available points.
func UnspentAbilityPoints(save *types.SaveFile, advancement []types.AdvancementEntry) int {
	if save == nil {
		return 0
	}
	level := GetLevelFromXP(save.Experience, advancement)
	unspent := AbilityPointsEarned(level, advancement) - len(save.FeatsChosen) - sumIncreases(save.AbilityIncreases)
	if unspent < 0 {
		return 0
	}
	return unspent
}

func sumIncreases(m map[string]int) int {
	total := 0
	for _, n := range m {
		total += n
	}
	return total
}

// AbilityScores returns the character's six current ability scores keyed by
// canonical name, reading whatever casing the save's Stats map uses.
func AbilityScores(save *types.SaveFile) map[string]int {
	out := make(map[string]int, len(StatNames))
	for _, name := range StatNames {
		out[name] = statScore(save.Stats, name)
	}
	return out
}

// SpendAbilityPoint allocates one banked ability point into the named ability,
// raising the stored score by 1 and recording the choice in AbilityIncreases. It
// returns an error if the ability name is invalid, the character has no unspent
// points, or the score is already at AbilityScoreMax. Derived maxima are
// re-hydrated afterward, so bumping CON or a casting stat updates Max HP / Mana.
func SpendAbilityPoint(save *types.SaveFile, ability string, advancement []types.AdvancementEntry) error {
	if save == nil {
		return fmt.Errorf("nil save")
	}
	canon := CanonicalAbility(ability)
	if canon == "" {
		return fmt.Errorf("invalid ability %q", ability)
	}
	if UnspentAbilityPoints(save, advancement) <= 0 {
		return fmt.Errorf("no unspent ability points")
	}
	current := statScore(save.Stats, canon)
	if current >= AbilityScoreMax {
		return fmt.Errorf("%s is already at the cap of %d", canon, AbilityScoreMax)
	}

	setStatScore(save, canon, current+1)
	if save.AbilityIncreases == nil {
		save.AbilityIncreases = make(map[string]int)
	}
	save.AbilityIncreases[canon]++

	Hydrate(save, advancement)
	return nil
}

// setStatScore writes an ability score into the save's Stats map, updating the
// existing key whatever its casing, or creating the canonical key if absent.
func setStatScore(save *types.SaveFile, canon string, value int) {
	if save.Stats == nil {
		save.Stats = make(map[string]interface{})
	}
	lower := strings.ToLower(canon)
	for k := range save.Stats {
		if strings.ToLower(k) == lower {
			save.Stats[k] = value
			return
		}
	}
	save.Stats[canon] = value
}

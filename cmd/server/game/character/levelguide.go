package character

import (
	"strings"

	"pubkey-quest/types"
)

// Level-up progression guide (read-only "what do I get at each level" preview).
//
// BuildLevelGuide assembles the full 1→20 path for a character from already
// loaded inputs so it stays a pure, testable function: the handler does the DB
// I/O (advancement, martial abilities, caster spell slots) and passes the data
// in. Everything the guide shows is either game data or derived — nothing here
// touches the save's stored state.

// ProficiencyBonus returns the D&D proficiency bonus for a character level
// (+2 at 1–4, +3 at 5–8, +4 at 9–12, +5 at 13–16, +6 at 17–20). Mirrors the
// combat package's derivation so the guide and combat agree.
func ProficiencyBonus(level int) int {
	switch {
	case level <= 4:
		return 2
	case level <= 8:
		return 3
	case level <= 12:
		return 4
	case level <= 16:
		return 5
	default:
		return 6
	}
}

// FeatEligibleLevels returns the levels at which a class may choose a feat
// instead of that level's ability point. Base cadence is D&D's ASI levels
// (4/8/12/16/19); Fighter adds 6 & 14, Rogue adds 10. See
// docs/draft/feats-progression.md.
func FeatEligibleLevels(class string) map[int]bool {
	levels := map[int]bool{4: true, 8: true, 12: true, 16: true, 19: true}
	switch strings.ToLower(class) {
	case "fighter":
		levels[6] = true
		levels[14] = true
	case "rogue":
		levels[10] = true
	}
	return levels
}

// GuideEntry is a named thing gained at a level (a new ability or an upgrade).
type GuideEntry struct {
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
}

// GuideAbilityUnlock is the slim martial-ability input BuildLevelGuide needs:
// when the ability unlocks and the level/summary of each scaling tier.
type GuideAbilityUnlock struct {
	Name        string
	UnlockLevel int
	Tiers       []GuideAbilityTier
}

// GuideAbilityTier is one scaling step of an ability (its starting level + blurb).
type GuideAbilityTier struct {
	Level   int
	Summary string
}

// GuideLevel is one row of the progression guide.
type GuideLevel struct {
	Level           int            `json:"level"`
	XPRequired      int            `json:"xp_required"`            // cumulative XP to reach this level
	XPFromPrev      int            `json:"xp_from_prev"`           // XP between the previous level and this one
	XPMultiplier    float64        `json:"xp_multiplier,omitempty"` // all XP earned while at this level is ×this (advancement.json)
	Reached         bool           `json:"reached"`                // character is at or past this level
	IsCurrent       bool           `json:"is_current"`             // character's current level
	AbilityPoint    bool           `json:"ability_point"`          // grants an ability point
	FeatEligible    bool           `json:"feat_eligible"`          // may take a feat instead of the point
	Proficiency     int            `json:"proficiency"`            // proficiency bonus at this level
	MaxHP           int            `json:"max_hp"`
	HPGain          int            `json:"hp_gain"`                // MaxHP increase from the previous level
	MaxMana         int            `json:"max_mana,omitempty"`     // casters only
	ResourceLabel   string         `json:"resource_label,omitempty"` // martial class resource name (Stamina/Rage/Ki/Cunning); casters show mana instead
	ResourceMax     int            `json:"resource_max,omitempty"`   // resource pool at this level (Ki grows; others flat)
	NewAbilities    []GuideEntry   `json:"new_abilities,omitempty"`    // abilities unlocking here (martial)
	AbilityUpgrades []GuideEntry   `json:"ability_upgrades,omitempty"` // ability tiers upgrading here (martial)
	NewSpellTier    int            `json:"new_spell_tier,omitempty"`   // highest spell level newly unlocked here (caster)
	SpellSlots      map[string]int `json:"spell_slots,omitempty"`      // slot counts at this level (caster)
}

// BuildLevelGuide produces the 1→20 progression rows for a character. abilities
// is the class's martial abilities (empty for non-martials); spellSlots maps
// level → slot-name → count for casters (nil for non-casters).
func BuildLevelGuide(save *types.SaveFile, advancement []types.AdvancementEntry, abilities []GuideAbilityUnlock, spellSlots map[int]map[string]int) []GuideLevel {
	currentLevel := GetLevelFromXP(save.Experience, advancement)
	feats := FeatEligibleLevels(save.Class)

	xpByLevel := make(map[int]int, len(advancement))
	apByLevel := make(map[int]int, len(advancement))
	multByLevel := make(map[int]float64, len(advancement))
	for _, e := range advancement {
		xpByLevel[e.Level] = e.ExperiencePoints
		apByLevel[e.Level] = e.AbilityPoints
		multByLevel[e.Level] = e.XPMultiplier
	}

	rows := make([]GuideLevel, 0, 20)
	prevHP := 0
	for lvl := 1; lvl <= 20; lvl++ {
		hp := DeriveMaxHP(save.Class, lvl, save.Stats)
		row := GuideLevel{
			Level:        lvl,
			XPRequired:   xpByLevel[lvl],
			XPMultiplier: multByLevel[lvl],
			Reached:      currentLevel >= lvl,
			IsCurrent:    currentLevel == lvl,
			AbilityPoint: apByLevel[lvl] > 0,
			FeatEligible: feats[lvl],
			Proficiency:  ProficiencyBonus(lvl),
			MaxHP:        hp,
		}
		if lvl > 1 {
			row.HPGain = hp - prevHP
			row.XPFromPrev = xpByLevel[lvl] - xpByLevel[lvl-1]
		}
		prevHP = hp

		if mana := DeriveMaxMana(save.Class, lvl, save.Stats); mana > 0 {
			row.MaxMana = mana
		}
		if label, max := resourceForLevel(save.Class, lvl, save.Stats); label != "" {
			row.ResourceLabel = label
			row.ResourceMax = max
		}

		for _, ab := range abilities {
			if ab.UnlockLevel == lvl {
				row.NewAbilities = append(row.NewAbilities, GuideEntry{Name: ab.Name, Summary: tierSummaryAt(ab, lvl)})
				continue
			}
			for _, t := range ab.Tiers {
				if t.Level == lvl && t.Level > ab.UnlockLevel {
					row.AbilityUpgrades = append(row.AbilityUpgrades, GuideEntry{Name: ab.Name, Summary: t.Summary})
				}
			}
		}

		if spellSlots != nil {
			row.SpellSlots = spellSlots[lvl]
			row.NewSpellTier = newSpellTier(spellSlots[lvl], spellSlots[lvl-1])
		}

		rows = append(rows, row)
	}
	return rows
}

// tierSummaryAt returns the summary of the scaling tier covering the given level,
// falling back to the first tier.
func tierSummaryAt(ab GuideAbilityUnlock, level int) string {
	best := ""
	bestLevel := -1
	for _, t := range ab.Tiers {
		if t.Level <= level && t.Level > bestLevel {
			best = t.Summary
			bestLevel = t.Level
		}
	}
	return best
}

// newSpellTier returns the highest spell level (1–9) whose slot first becomes
// available at this level versus the previous one, or 0 if no new tier opens.
func newSpellTier(cur, prev map[string]int) int {
	if cur == nil {
		return 0
	}
	highest := 0
	for tier := 1; tier <= 9; tier++ {
		key := spellTierKey(tier)
		if cur[key] > 0 && prev[key] == 0 {
			if tier > highest {
				highest = tier
			}
		}
	}
	return highest
}

func spellTierKey(tier int) string {
	switch tier {
	case 1:
		return "level_1"
	case 2:
		return "level_2"
	case 3:
		return "level_3"
	case 4:
		return "level_4"
	case 5:
		return "level_5"
	case 6:
		return "level_6"
	case 7:
		return "level_7"
	case 8:
		return "level_8"
	case 9:
		return "level_9"
	}
	return ""
}

// IsCaster reports whether a class uses the spell system (has a casting ability).
func IsCaster(class string) bool {
	_, ok := classCastingStat[class]
	return ok
}

// resourceForLevel returns a martial class's level-up resource (label + pool) for
// the guide. Mirrors game-data/systems/class-resources.json — flat for most, but
// Monk ki scales with wisdom + level. Casters return "" (they surface mana).
// Display-only; combat reads the JSON for the authoritative value.
func resourceForLevel(class string, level int, stats map[string]interface{}) (string, int) {
	switch strings.ToLower(class) {
	case "fighter":
		return "Stamina", 10
	case "barbarian":
		return "Rage", 100
	case "rogue":
		return "Cunning", 10
	case "monk":
		return "Ki", AbilityMod(statScore(stats, "wisdom")) + level
	}
	return "", 0
}

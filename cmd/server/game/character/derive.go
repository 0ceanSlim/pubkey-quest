package character

import (
	"strings"

	"pubkey-quest/types"
)

// Derived character stats (§4 hydration rule).
//
// MaxHP and MaxMana are NOT authoritative state — they are a deterministic
// function of class, level (itself derived from XP), and ability scores. The
// persisted save should not store them; they are recomputed on load via
// Hydrate. Keeping them derived means leveling up raises them automatically
// (because level rises with XP) and nothing derivable is frozen into a save.

// classHitDie maps a class to its hit-die maximum, which is also its level-1
// base HP before the CON modifier. Mirrors
// game-data/systems/new-character/base-hp.json and the legacy calculateHP in
// cmd/server/api/character.
var classHitDie = map[string]int{
	"Barbarian": 12,
	"Fighter":   10,
	"Paladin":   10,
	"Ranger":    10,
	"Monk":      8,
	"Rogue":     8,
	"Bard":      8,
	"Cleric":    8,
	"Druid":     8,
	"Warlock":   8,
	"Sorcerer":  6,
	"Wizard":    6,
}

// classCastingStat maps a caster class to its spellcasting ability (lowercase).
// Non-casters are absent. Mirrors the legacy calculateMana spellcasters map.
var classCastingStat = map[string]string{
	"Wizard":   "intelligence",
	"Sorcerer": "charisma",
	"Warlock":  "charisma",
	"Bard":     "charisma",
	"Cleric":   "wisdom",
	"Druid":    "wisdom",
	"Paladin":  "charisma",
	"Ranger":   "wisdom",
}

// HitDie returns a class's hit-die maximum (default 8 for unknown classes).
func HitDie(class string) int {
	if hd, ok := classHitDie[class]; ok {
		return hd
	}
	return 8
}

// AbilityMod returns the D&D ability-score modifier using floor division
// (9 → -1, 10 → 0, 14 → +2). This is the canonical modifier; some legacy
// creation code truncates toward zero instead — since the derived stats here
// are authoritative, this floor form is the single source of truth.
func AbilityMod(score int) int {
	diff := score - 10
	if diff < 0 && diff%2 != 0 {
		return (diff - 1) / 2
	}
	return diff / 2
}

// statScore reads an ability score from a save's Stats map case-insensitively
// (keys are inconsistently capitalized across the codebase) and tolerates both
// float64 (JSON) and int values. Defaults to 10 when absent.
func statScore(stats map[string]interface{}, name string) int {
	name = strings.ToLower(name)
	for k, v := range stats {
		if strings.ToLower(k) != name {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 10
}

// DeriveMaxHP computes maximum HP for a character of the given class, level, and
// ability scores. HP gain is fixed (no rolling): level 1 is hitDie + CON mod,
// and each level after adds (hitDie/2 + 1) + CON mod. At level 1 this equals the
// legacy creation formula. Never returns less than 1.
func DeriveMaxHP(class string, level int, stats map[string]interface{}) int {
	if level < 1 {
		level = 1
	}
	hd := HitDie(class)
	conMod := AbilityMod(statScore(stats, "constitution"))
	hp := hd + conMod                  // level 1
	perLevel := hd/2 + 1 + conMod      // fixed average gain per level after 1
	hp += (level - 1) * perLevel
	if hp < 1 {
		hp = 1
	}
	return hp
}

// halfCasters cast at half their level. Like D&D half-casters (Paladin, Ranger)
// their magic is secondary to martial prowess: spells start at level 2 and the
// progression tops out around 5th-level spells. Their effective caster level is
// floor(level/2), which mirrors the half-caster slot curve in spell-slots.json.
var halfCasters = map[string]bool{
	"Paladin": true,
	"Ranger":  true,
}

// DeriveMaxMana computes maximum mana. Non-casters get 0. A caster's mana is
// (spellcasting-ability mod + caster level), where full casters use their level
// and half-casters (Paladin/Ranger) use floor(level/2) — so their mana pool
// scales with their half-rate spell progression, not a full caster's. Clamped
// to >= 0.
func DeriveMaxMana(class string, level int, stats map[string]interface{}) int {
	if level < 1 {
		level = 1
	}
	statName, isCaster := classCastingStat[class]
	if !isCaster {
		return 0
	}
	casterLevel := level
	if halfCasters[class] {
		casterLevel = level / 2 // floor — effective caster level for half-casters
	}
	mana := AbilityMod(statScore(stats, statName)) + casterLevel
	if mana < 0 {
		mana = 0
	}
	return mana
}

// Hydrate recomputes derived character fields (MaxHP, MaxMana) from class, the
// level implied by XP, and ability scores, then clamps current HP/Mana to the
// new maxima. Call it whenever a save is loaded and after any XP change. It
// never heals — it only clamps down — so reloading is idempotent; the
// heal-on-level-up reward is applied separately at the level-up moment.
func Hydrate(save *types.SaveFile, advancement []types.AdvancementEntry) {
	if save == nil {
		return
	}
	level := GetLevelFromXP(save.Experience, advancement)
	save.MaxHP = DeriveMaxHP(save.Class, level, save.Stats)
	save.MaxMana = DeriveMaxMana(save.Class, level, save.Stats)
	if save.HP > save.MaxHP {
		save.HP = save.MaxHP
	}
	if save.HP < 0 {
		save.HP = 0
	}
	if save.Mana > save.MaxMana {
		save.Mana = save.MaxMana
	}
	if save.Mana < 0 {
		save.Mana = 0
	}
}

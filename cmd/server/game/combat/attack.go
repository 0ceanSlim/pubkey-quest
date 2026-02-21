package combat

import (
	"strings"
)

// proficiencyBonus returns the D&D proficiency bonus for a given character level.
func proficiencyBonus(level int) int {
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

// classWeaponProficiencies maps lowercased class names to their proficiency categories.
// Categories: "simple", "martial", or a specific item ID for narrow proficiencies.
var classWeaponProficiencies = map[string][]string{
	"barbarian": {"simple", "martial"},
	"fighter":   {"simple", "martial"},
	"paladin":   {"simple", "martial"},
	"ranger":    {"simple", "martial"},
	"rogue":     {"simple", "hand-crossbow", "longsword", "rapier", "shortsword"},
	"bard":      {"simple", "hand-crossbow", "longsword", "rapier", "shortsword"},
	"cleric":    {"simple"},
	"druid":     {"simple"},
	"monk":      {"simple", "shortsword"},
	"sorcerer":  {"dagger", "dart", "sling", "quarterstaff", "light-crossbow"},
	"warlock":   {"dagger", "dart", "sling", "quarterstaff", "light-crossbow"},
	"wizard":    {"dagger", "dart", "sling", "quarterstaff", "light-crossbow"},
}

// IsProficientWith returns true if the class is proficient with the given weapon.
// weaponType is the item's "type" field (e.g. "Martial Melee Weapons").
// weaponID is the item's ID (e.g. "longsword").
func IsProficientWith(class, weaponType, weaponID string) bool {
	profs, ok := classWeaponProficiencies[strings.ToLower(class)]
	if !ok {
		return false
	}

	wtLower := strings.ToLower(weaponType)
	idLower := strings.ToLower(weaponID)

	for _, prof := range profs {
		switch prof {
		case "simple":
			if strings.Contains(wtLower, "simple") {
				return true
			}
		case "martial":
			if strings.Contains(wtLower, "martial") {
				return true
			}
		default:
			if prof == idLower {
				return true
			}
		}
	}
	return false
}

// AttackResult holds the full outcome of an attack roll.
type AttackResult struct {
	Roll      int  // Raw d20 result
	Total     int  // Roll + all modifiers
	Modifier  int  // Sum of modifiers applied
	IsCrit    bool // Natural 20 — always hits, double dice
	IsCritMiss bool // Natural 1 — always misses
	IsHit     bool // Total >= target AC (or crit)
}

// ResolveAttackRoll performs a d20 attack roll.
// advantage: >0 = advantage, <0 = disadvantage, 0 = normal.
func ResolveAttackRoll(attackBonus, targetAC, advantage int) AttackResult {
	var roll int
	switch {
	case advantage > 0:
		roll, _, _ = RollAdvantage()
	case advantage < 0:
		roll, _, _ = RollDisadvantage()
	default:
		roll = RollD20()
	}

	isCrit := roll == 20
	isCritMiss := roll == 1
	total := roll + attackBonus
	isHit := isCrit || (!isCritMiss && total >= targetAC)

	return AttackResult{
		Roll:       roll,
		Total:      total,
		Modifier:   attackBonus,
		IsCrit:     isCrit,
		IsCritMiss: isCritMiss,
		IsHit:      isHit,
	}
}

// hasTag checks whether a list of raw interface tags contains a specific lowercase string.
func hasTag(tags interface{}, tag string) bool {
	list, ok := tags.([]interface{})
	if !ok {
		return false
	}
	for _, t := range list {
		if s, ok := t.(string); ok && strings.EqualFold(s, tag) {
			return true
		}
	}
	return false
}

// WeaponAttackBonus computes the full attack bonus for a player attacking with an item.
// item is the full item map from the database properties column.
// stats is the player's stats map from the save file.
func WeaponAttackBonus(item map[string]interface{}, stats map[string]interface{}, class string, level int) int {
	weaponType, _ := item["type"].(string)
	weaponID, _ := item["id"].(string)
	tags := item["tags"]

	strMod := StatMod(GetStatFromMap(stats, "strength"))
	dexMod := StatMod(GetStatFromMap(stats, "dexterity"))

	var abilityMod int
	isRanged := strings.Contains(strings.ToLower(weaponType), "ranged")

	if hasTag(tags, "finesse") {
		// Use whichever is higher
		if dexMod > strMod {
			abilityMod = dexMod
		} else {
			abilityMod = strMod
		}
	} else if isRanged {
		abilityMod = dexMod
	} else {
		abilityMod = strMod
	}

	prof := 0
	if IsProficientWith(class, weaponType, weaponID) {
		prof = proficiencyBonus(level)
	}

	return abilityMod + prof
}

// WeaponDamageBonus returns the ability modifier added to weapon damage rolls.
func WeaponDamageBonus(item map[string]interface{}, stats map[string]interface{}) int {
	weaponType, _ := item["type"].(string)
	tags := item["tags"]

	strMod := StatMod(GetStatFromMap(stats, "strength"))
	dexMod := StatMod(GetStatFromMap(stats, "dexterity"))

	isRanged := strings.Contains(strings.ToLower(weaponType), "ranged")

	if hasTag(tags, "finesse") {
		if dexMod > strMod {
			return dexMod
		}
		return strMod
	}
	if isRanged {
		return dexMod
	}
	return strMod
}

// WeaponDamageDice returns the damage dice string for an item, choosing 2H for versatile weapons
// when the offhand is empty.
func WeaponDamageDice(item map[string]interface{}, offhandEmpty bool) string {
	raw, _ := item["damage"].(string)
	if raw == "" {
		return "1d4" // Fallback for unusual items
	}

	tags := item["tags"]
	if hasTag(tags, "versatile") && offhandEmpty {
		_, twoH := ParseVersatileDice(raw)
		return twoH
	}

	oneH, _ := ParseVersatileDice(raw)
	return oneH
}

// WeaponDamageType returns the damage type string from an item.
func WeaponDamageType(item map[string]interface{}) string {
	// Items use "damage-type" as the key
	if dt, ok := item["damage-type"].(string); ok && dt != "" {
		return dt
	}
	if dt, ok := item["damage_type"].(string); ok && dt != "" {
		return dt
	}
	return "bludgeoning"
}

// UnarmedAttackBonus returns the attack bonus for an unarmed strike.
func UnarmedAttackBonus(stats map[string]interface{}, class string, level int) int {
	strMod := StatMod(GetStatFromMap(stats, "strength"))
	prof := proficiencyBonus(level) // All classes are proficient with unarmed strikes
	return strMod + prof
}

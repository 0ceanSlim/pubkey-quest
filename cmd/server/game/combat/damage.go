package combat

import (
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"pubkey-quest/cmd/server/game/effects"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// acSlots are the gear slots checked for AC contributions.
// Weapon, ammo, and bag slots are excluded.
var acSlots = []string{
	"chest", "head", "offhand", "legs", "boots", "gloves", "necklace", "ring1", "ring2", "cloak",
}

// CalculatePlayerAC computes the player's total AC from equipped gear.
//
// Item "ac" field formats supported:
//
//	"7 + Dex"           → light armor base: 7 + full DEX mod  (isBase=true)
//	"11 + Dex (max 2)"  → medium armor base: 11 + DEX capped at 2  (isBase=true)
//	"16"                → heavy armor base when gear_slot=="chest": flat 16  (isBase=true)
//	"+2"                → additive bonus (shield, ring, etc.)  (isBase=false)
//	"4"                 → additive flat value for non-chest slots  (isBase=false)
//	7  (number)         → legacy numeric; uses "ac_base" bool if present
func CalculatePlayerAC(db *sql.DB, inventory map[string]interface{}, stats map[string]interface{}) int {
	return CalculatePlayerACFor(db, inventory, stats, "")
}

// CalculatePlayerACFor is CalculatePlayerAC with the class's Unarmored Defense:
// a Barbarian in no body armour gets 10 + DEX + CON (a shield still adds); a
// Monk in no armour and no shield gets 10 + DEX + WIS.
func CalculatePlayerACFor(db *sql.DB, inventory map[string]interface{}, stats map[string]interface{}, class string) int {
	ac, _, _ := gearAC(db, inventory, stats, class)
	return ac
}

// PlayerAC is the player's AC with what lasting spells add to the gear:
// mage armor's base ("unarmored_ac": 13 + DEX while no body armour is worn),
// "armor_class" bonuses, "armored_ac" while armour or a shield is on
// (mending), and barkskin's "ac_floor". Combat adds its own round-to-round spell buffs on top (shield).
func PlayerAC(db *sql.DB, save *types.SaveFile) int {
	stats := effectiveStats(save)
	ac, bodyArmor, anyArmor := gearAC(db, save.Inventory, stats, save.Class)
	if base := effects.ModifierTotal(save, "unarmored_ac"); base > 0 && !bodyArmor {
		if mage := base + StatMod(GetStatFromMap(stats, "dexterity")); mage > ac {
			ac = mage
		}
	}
	ac += effects.ModifierTotal(save, "armor_class")
	if anyArmor {
		ac += effects.ModifierTotal(save, "armored_ac")
	}
	// Barkskin: AC can't be lower than its floor.
	if floor := effects.ModifierTotal(save, "ac_floor"); floor > ac {
		ac = floor
	}
	return ac
}

// gearAC computes AC from equipped gear and the class's unarmored defense, and
// reports whether body armour (bodyArmor) or any armour piece or shield
// (anyArmor) is worn.
func gearAC(db *sql.DB, inventory map[string]interface{}, stats map[string]interface{}, class string) (ac int, bodyArmor, anyArmor bool) {
	dexMod := StatMod(GetStatFromMap(stats, "dexterity"))

	baseAC := 10 + dexMod // Unarmored default
	additiveAC := 0
	baseSet := false

	for _, slot := range acSlots {
		itemID := gaminventory.GetEquippedItemID(inventory, slot)
		if itemID == "" {
			continue
		}

		item, err := loadItemProps(db, itemID)
		if err != nil {
			log.Printf("⚠️ AC calc: could not load item %s: %v", itemID, err)
			continue
		}

		contrib, isBase := parseItemAC(item, dexMod)
		if contrib == 0 {
			continue
		}

		if isBase && !baseSet {
			baseAC = contrib // Already includes DEX contribution
			baseSet = true
		} else {
			additiveAC += contrib
		}
	}

	if !baseSet {
		switch strings.ToLower(class) {
		case "barbarian":
			baseAC = 10 + dexMod + StatMod(GetStatFromMap(stats, "constitution"))
		case "monk":
			if additiveAC == 0 { // no shield (or anything else in the way)
				baseAC = 10 + dexMod + StatMod(GetStatFromMap(stats, "wisdom"))
			}
		}
	}
	return baseAC + additiveAC, baseSet, baseSet || additiveAC > 0
}

// parseItemAC reads the "ac" field from a fully-deserialised item properties map
// and returns (contribution, isBase).
// isBase=true  → this piece replaces the default 10+DEX base.
// isBase=false → this piece is an additive bonus stacked on top.
func parseItemAC(item map[string]interface{}, dexMod int) (contribution int, isBase bool) {
	acRaw, ok := item["ac"]
	if !ok {
		return 0, false
	}
	switch v := acRaw.(type) {
	case float64:
		base, _ := item["ac_base"].(bool)
		return int(v), base
	case int:
		base, _ := item["ac_base"].(bool)
		return v, base
	case string:
		return parseACString(v, item, dexMod)
	}
	return 0, false
}

// parseACString decodes the human-readable AC strings used in item JSON files.
func parseACString(s string, item map[string]interface{}, dexMod int) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	lower := strings.ToLower(s)

	// "+N" → explicit additive (shield, ring, etc.)
	if strings.HasPrefix(s, "+") {
		n, err := strconv.Atoi(strings.TrimSpace(s[1:]))
		if err != nil {
			return 0, false
		}
		return n, false
	}

	// "N + Dex..." → base formula, light or medium armor
	if strings.Contains(lower, "dex") {
		plusIdx := strings.Index(s, "+")
		if plusIdx < 0 {
			return 0, false
		}
		base, err := strconv.Atoi(strings.TrimSpace(s[:plusIdx]))
		if err != nil {
			return 0, false
		}
		// Check for medium armor DEX cap: "max N"
		dexContrib := dexMod
		if strings.Contains(lower, "max") {
			cap := 2
			words := strings.Fields(lower)
			for i, w := range words {
				if w == "max" && i+1 < len(words) {
					if n, err := strconv.Atoi(strings.Trim(words[i+1], "(),")); err == nil {
						cap = n
					}
				}
			}
			if dexContrib > cap {
				dexContrib = cap
			}
		}
		return base + dexContrib, true
	}

	// Plain number: "16", "4", etc.
	// A chest-slot item with a plain number = heavy armor base (no DEX).
	// Everything else (legs, boots, etc.) = additive.
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	gearSlot, _ := item["gear_slot"].(string)
	return n, strings.ToLower(gearSlot) == "chest"
}

// loadItemProps loads a single item's properties JSON from the database.
func loadItemProps(db *sql.DB, itemID string) (map[string]interface{}, error) {
	var propertiesJSON string
	err := db.QueryRow("SELECT properties FROM items WHERE id = ?", itemID).Scan(&propertiesJSON)
	if err != nil {
		return nil, err
	}
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &props); err != nil {
		return nil, err
	}
	return props, nil
}


// ResolveDamageToMonster rolls damage dice and applies the monster's resistances,
// immunities, and vulnerabilities. Returns final damage dealt (minimum 1 on a hit).
func ResolveDamageToMonster(diceExpr string, modifier int, damageType string, isCrit bool, monster *types.MonsterInstance) int {
	raw := RollDice(diceExpr, isCrit) + modifier
	if raw < 1 {
		raw = 1
	}

	dtLower := strings.ToLower(damageType)

	for _, imm := range monster.Data.DamageImmunities {
		if strings.EqualFold(imm, dtLower) {
			return 0
		}
	}
	for _, vuln := range monster.Data.DamageVulnerabilities {
		if strings.EqualFold(vuln, dtLower) {
			return raw * 2
		}
	}
	for _, res := range monster.Data.DamageResistances {
		if strings.EqualFold(res, dtLower) {
			return raw / 2
		}
	}

	return raw
}

// resolveElementalAdeptDamage is ResolveDamageToMonster for a caster with Elemental
// Adept on this damage type: the target's RESISTANCE is ignored (immunity and
// vulnerability still apply), and every damage die that rolls a 1 counts as a 2.
func resolveElementalAdeptDamage(diceExpr string, modifier int, damageType string, isCrit bool, monster *types.MonsterInstance) int {
	raw := rollDiceMinTwo(diceExpr, isCrit) + modifier
	if raw < 1 {
		raw = 1
	}
	dtLower := strings.ToLower(damageType)
	for _, imm := range monster.Data.DamageImmunities {
		if strings.EqualFold(imm, dtLower) {
			return 0
		}
	}
	for _, vuln := range monster.Data.DamageVulnerabilities {
		if strings.EqualFold(vuln, dtLower) {
			return raw * 2
		}
	}
	// Resistance is intentionally NOT applied (that's the whole point of the feat).
	return raw
}

// rollDiceMinTwo rolls a dice expression with every die counting at least 2
// (Elemental Adept). Doubles the dice count on a crit, mirroring RollDice.
func rollDiceMinTwo(diceExpr string, isCrit bool) int {
	count, sides, err := ParseDice(diceExpr)
	if err != nil {
		return 2
	}
	if isCrit {
		count *= 2
	}
	total := 0
	for i := 0; i < count; i++ {
		if r := RollD(sides); r < 2 {
			total += 2
		} else {
			total += r
		}
	}
	return total
}

// ResolveDamageToPlayer rolls damage dealt to the player (minimum 1).
func ResolveDamageToPlayer(diceExpr string, modifier int, isCrit bool) int {
	raw := RollDice(diceExpr, isCrit) + modifier
	if raw < 1 {
		return 1
	}
	return raw
}

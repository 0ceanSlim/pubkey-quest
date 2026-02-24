package combat

import (
	"database/sql"
	"encoding/json"
	"log"
	"strconv"
	"strings"

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

	return baseAC + additiveAC
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

// ResolveDamageToPlayer rolls damage dealt to the player (minimum 1).
func ResolveDamageToPlayer(diceExpr string, modifier int, isCrit bool) int {
	raw := RollDice(diceExpr, isCrit) + modifier
	if raw < 1 {
		return 1
	}
	return raw
}

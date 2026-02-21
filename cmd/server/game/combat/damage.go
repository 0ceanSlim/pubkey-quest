package combat

import (
	"database/sql"
	"encoding/json"
	"log"
	"strings"

	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// acSlots are the gear slots that can contribute to AC.
// mainhand (weapon), ammo, and bag are excluded.
var acSlots = []string{
	"chest", "head", "offhand", "legs", "boots", "gloves", "neck", "ring1", "ring2",
}

// CalculatePlayerAC computes the player's total AC from equipped gear.
// Chest pieces with ac_base:true set the base formula (replacing 10+DEX).
// All other equipped items add their flat ac value on top.
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

		acVal := getIntProp(item, "ac")
		if acVal == 0 {
			continue // Item contributes no AC
		}

		isBase, _ := item["ac_base"].(bool)
		if isBase && !baseSet {
			// This piece sets the base formula — replaces the 10+DEX default.
			armorType, _ := item["armor_type"].(string)
			baseAC = acVal + calcDexContrib(armorType, item, dexMod)
			baseSet = true
		} else {
			// Additive piece (helmet, shield, ring, boots, etc.)
			additiveAC += acVal
		}
	}

	return baseAC + additiveAC
}

// calcDexContrib returns the DEX modifier contribution based on armor type and dex_cap.
func calcDexContrib(armorType string, item map[string]interface{}, dexMod int) int {
	switch strings.ToLower(armorType) {
	case "heavy":
		return 0
	case "medium":
		cap := 2 // D&D 5e default medium cap
		if v, ok := item["dex_cap"].(float64); ok {
			cap = int(v)
		}
		if dexMod > cap {
			return cap
		}
		return dexMod
	default: // "light", "clothing", unset — full DEX
		return dexMod
	}
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

// getIntProp reads an integer property from a map, handling JSON float64.
func getIntProp(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	}
	return 0
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

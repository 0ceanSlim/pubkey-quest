package combat

import (
	"database/sql"
	"fmt"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// WeaponLine is the derived offensive stats for one equipped weapon, for the
// Equipment tab. The client resolves the item's name/icon from its id.
type WeaponLine struct {
	ItemID      string `json:"item_id"`
	AttackBonus int    `json:"attack_bonus"`
	Damage      string `json:"damage"` // e.g. "1d8+3"
	DamageType  string `json:"damage_type"`
}

// EquippedStats are the combat numbers the equipped gear produces, surfaced to
// the Equipment tab. Always computed, never stored (hydration rule). It reuses
// the same helpers the combat engine uses, so the tab and a real attack agree.
type EquippedStats struct {
	ArmorClass int         `json:"armor_class"`
	MainHand   *WeaponLine `json:"main_hand,omitempty"` // melee/versatile weapon in hand
	OffHand    *WeaponLine `json:"off_hand,omitempty"`  // off-hand weapon (not a shield)
	Ranged     *WeaponLine `json:"ranged,omitempty"`    // when the held weapon is ranged
	Ammo       int         `json:"ammo"`                // equipped ammunition count
	Unarmed    *WeaponLine `json:"unarmed,omitempty"`   // shown when no weapon is held
}

// BuildEquippedStats derives AC plus attack/damage lines from the player's
// equipped gear. level (derived from XP by the caller) feeds the proficiency
// bonus, exactly as the combat engine does.
func BuildEquippedStats(db *sql.DB, save *types.SaveFile, level int) EquippedStats {
	inv := save.Inventory
	stats := save.Stats
	es := EquippedStats{
		ArmorClass: CalculatePlayerAC(db, inv, stats),
		Ammo:       equippedAmmoCount(inv),
	}

	offhandEmpty := isOffhandEmpty(inv)

	// Main hand: a melee/versatile weapon is a MainHand line; a ranged weapon
	// (e.g. a bow) is a Ranged line. An empty hand means unarmed strikes.
	if id := gaminventory.GetEquippedItemID(inv, "mainhand"); id != "" {
		if item, err := gamedata.LoadItemByID(db, id); err == nil {
			line := &WeaponLine{
				ItemID:      id,
				AttackBonus: WeaponAttackBonus(item, stats, save.Class, level),
				Damage:      formatDamageExpr(WeaponDamageDice(item, offhandEmpty), WeaponDamageBonus(item, stats)),
				DamageType:  WeaponDamageType(item),
			}
			if isRangedWeapon(item) {
				es.Ranged = line
			} else {
				es.MainHand = line
			}
		}
	} else {
		es.Unarmed = &WeaponLine{
			AttackBonus: UnarmedAttackBonus(stats, save.Class, level),
			Damage:      formatDamageExpr("1d4", StatMod(GetStatFromMap(stats, "strength"))),
			DamageType:  "bludgeoning",
		}
	}

	// Off hand: only a weapon there is an attack line (a shield contributes AC,
	// already counted). Two-weapon fighting adds no ability mod to damage.
	if id := gaminventory.GetEquippedItemID(inv, "offhand"); id != "" {
		if item, err := gamedata.LoadItemByID(db, id); err == nil {
			if dmg, _ := item["damage"].(string); dmg != "" && !isRangedWeapon(item) {
				es.OffHand = &WeaponLine{
					ItemID:      id,
					AttackBonus: WeaponAttackBonus(item, stats, save.Class, level),
					Damage:      WeaponDamageDice(item, false),
					DamageType:  WeaponDamageType(item),
				}
			}
		}
	}

	return es
}

func isRangedWeapon(item map[string]interface{}) bool {
	t, _ := item["type"].(string)
	return strings.Contains(strings.ToLower(t), "ranged")
}

// equippedAmmoCount returns the quantity in the equipped ammunition slot.
func equippedAmmoCount(inv map[string]interface{}) int {
	gs, ok := inv["gear_slots"].(map[string]interface{})
	if !ok {
		return 0
	}
	for _, key := range []string{"ammunition", "ammo"} {
		slot, ok := gs[key].(map[string]interface{})
		if !ok || slot["item"] == nil || slot["item"] == "" {
			continue
		}
		switch q := slot["quantity"].(type) {
		case float64:
			return int(q)
		case int:
			return q
		}
	}
	return 0
}

// formatDamageExpr renders a dice string plus a flat modifier, e.g. "1d8+3",
// "1d6-1", or just "1d8" when the modifier is zero.
func formatDamageExpr(dice string, bonus int) string {
	switch {
	case bonus > 0:
		return fmt.Sprintf("%s+%d", dice, bonus)
	case bonus < 0:
		return fmt.Sprintf("%s%d", dice, bonus)
	default:
		return dice
	}
}

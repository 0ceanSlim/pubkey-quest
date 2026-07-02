package inventory_test

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
)

// Fighter fixture: STR 16 (+3), DEX 14 (+2), level 1 (prof +2).

func TestEquippedStatsUnarmedAndUnarmoredAC(t *testing.T) {
	setup(t)
	s := newSave(4, 20)

	es := combat.BuildEquippedStats(db.GetDB(), s, 1)

	if es.ArmorClass != 12 { // 10 + DEX(+2)
		t.Errorf("AC = %d, want 12 unarmored", es.ArmorClass)
	}
	if es.MainHand != nil {
		t.Errorf("MainHand = %+v, want nil with empty hands", es.MainHand)
	}
	if es.Unarmed == nil {
		t.Fatal("Unarmed line missing with empty hands")
	}
	if es.Unarmed.AttackBonus != 5 { // STR(+3) + prof(+2)
		t.Errorf("unarmed attack = %d, want 5", es.Unarmed.AttackBonus)
	}
}

func TestEquippedStatsMeleeWeaponLine(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	gearSlots(s)["mainhand"] = map[string]interface{}{"item": "longsword", "quantity": float64(1)}

	es := combat.BuildEquippedStats(db.GetDB(), s, 1)

	if es.MainHand == nil {
		t.Fatal("MainHand line missing for an equipped longsword")
	}
	if es.MainHand.AttackBonus != 5 { // STR(+3) + Fighter prof with martial (+2)
		t.Errorf("longsword attack = %d, want 5", es.MainHand.AttackBonus)
	}
	if !strings.HasSuffix(es.MainHand.Damage, "+3") { // STR damage mod
		t.Errorf("longsword damage = %q, want a +3 STR modifier", es.MainHand.Damage)
	}
}

func TestEquippedStatsArmorAC(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	gearSlots(s)["chest"] = map[string]interface{}{"item": "breastplate", "quantity": float64(1)}

	es := combat.BuildEquippedStats(db.GetDB(), s, 1)

	// breastplate is medium armor "14 + Dex(max2)": 14 + min(DEX+2, 2) = 16.
	if es.ArmorClass != 16 {
		t.Errorf("AC = %d, want 16 in a breastplate", es.ArmorClass)
	}
}

func TestEquippedStatsRangedAndAmmo(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	gearSlots(s)["mainhand"] = map[string]interface{}{"item": "longbow", "quantity": float64(1)}
	gearSlots(s)["ammo"] = map[string]interface{}{"item": "arrows", "quantity": float64(12)}

	es := combat.BuildEquippedStats(db.GetDB(), s, 1)

	if es.Ranged == nil {
		t.Fatal("Ranged line missing for an equipped longbow")
	}
	if es.MainHand != nil {
		t.Errorf("MainHand = %+v, want nil (the bow is a ranged line)", es.MainHand)
	}
	if es.Ammo != 12 {
		t.Errorf("Ammo = %d, want 12", es.Ammo)
	}
}

func TestEquippedStatsAmmoSumsQuiverContents(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	gearSlots(s)["mainhand"] = map[string]interface{}{"item": "longbow", "quantity": float64(1)}
	gearSlots(s)["ammo"] = map[string]interface{}{
		"item": "quiver", "quantity": float64(1),
		"contents": []interface{}{
			map[string]interface{}{"item": "arrows", "quantity": float64(10), "slot": float64(0)},
			map[string]interface{}{"item": "arrows", "quantity": float64(5), "slot": float64(1)},
		},
	}

	es := combat.BuildEquippedStats(db.GetDB(), s, 1)
	if es.Ammo != 15 {
		t.Errorf("Ammo = %d, want 15 (summed from an equipped quiver)", es.Ammo)
	}
}

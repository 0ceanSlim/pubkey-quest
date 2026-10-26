package combat_test

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/condition"
)

// A worn weapon hits softer (the log says by how much), and every damaging hit
// wears it further; a hit taken wears the body armour.
func TestWeaponWearInAFight(t *testing.T) {
	combatSetup(t)
	condition.ResetCache()
	adv, err := character.LoadAdvancement(db.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	save := fighterSave()
	sword := map[string]interface{}{"item": "longsword", "quantity": 1, condition.KeyWear: 160} // damaged: −25%
	save.Inventory = map[string]interface{}{
		"general_slots": []interface{}{},
		"gear_slots":    map[string]interface{}{"mainhand": sword},
	}
	cs, err := combat.StartEncounter(db.GetDB(), save, "npub_test", combat.EncounterSpec{MonsterIDs: []string{"giant-rat"}, EnvironmentID: "forest"}, adv)
	if err != nil {
		t.Fatal(err)
	}
	m := &cs.Monsters[0]
	m.ArmorClass, m.CurrentHP, m.MaxHP = 0, 1000, 1000
	me := combat.Actor(cs)
	m.Pos = me.Pos
	m.Pos.X++

	for i := 0; i < 10; i++ {
		cs.Phase = "active"
		me.CombatState.ActionUsed, me.CombatState.BonusActionUsed = false, false
		log, err := combat.ProcessPlayerAttack(db.GetDB(), cs, save, m.InstanceID, "mainhand", "main", false, adv)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(strings.Join(log, "\n"), "damaged:") {
			if condition.WearOf(sword) <= 160 {
				t.Errorf("a hit should wear the sword: %d", condition.WearOf(sword))
			}
			return
		}
	}
	t.Fatal("ten swings at AC 0 and no damaged-weapon line")
}

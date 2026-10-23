package combat_test

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

func fightWithItem(t *testing.T, itemID string) (*types.CombatSession, *types.SaveFile) {
	t.Helper()
	combatSetup(t)
	adv, err := character.LoadAdvancement(db.GetDB())
	if err != nil {
		t.Fatal(err)
	}
	save := fighterSave()
	save.Inventory = map[string]interface{}{
		"general_slots": []interface{}{map[string]interface{}{"slot": 0, "item": itemID, "quantity": 2}},
		"gear_slots":    map[string]interface{}{},
	}
	cs, err := combat.StartEncounter(db.GetDB(), save, "npub_test", combat.EncounterSpec{MonsterIDs: []string{"giant-rat"}, EnvironmentID: "forest"}, adv)
	if err != nil {
		t.Fatal(err)
	}
	cs.Phase = "active"
	combat.Actor(cs).CombatState.ActionUsed = false
	return cs, save
}

func TestSmokeBombEscapesTheFight(t *testing.T) {
	cs, save := fightWithItem(t, "smoke-bomb")
	if _, err := combat.ProcessPlayerUseItem(db.GetDB(), cs, save, "smoke-bomb", ""); err != nil {
		t.Fatal(err)
	}
	if cs.Phase != "loot" {
		t.Errorf("a smoke bomb should end the fight, phase %q", cs.Phase)
	}
}

func TestAntidoteCuresPoison(t *testing.T) {
	cs, save := fightWithItem(t, "antidote")
	st := &combat.Actor(cs).CombatState
	st.Conditions = []types.CombatCondition{{Name: "poisoned", DurationRounds: 3}}
	if _, err := combat.ProcessPlayerUseItem(db.GetDB(), cs, save, "antidote", ""); err != nil {
		t.Fatal(err)
	}
	if len(st.Conditions) != 0 {
		t.Errorf("still %v after an antidote", st.Conditions)
	}

	// Outside a fight it lifts the Poisoned effect.
	if err := effects.ApplyEffect(save, "poison"); err != nil {
		t.Fatal(err)
	}
	inventory.ApplyItemEffects(save, "antidote")
	if effects.HasActiveEffect(save, "poison") {
		t.Error("the antidote should remove the Poisoned effect")
	}
}

// Items at hand cost the action; ones in the backpack can be used too, but
// digging them out takes the bonus action as well.
func TestItemsFromThePackCostTheBonusAction(t *testing.T) {
	cs, save := fightWithItem(t, "antidote")
	save.Inventory["general_slots"] = []interface{}{}
	save.Inventory["gear_slots"] = map[string]interface{}{"bag": map[string]interface{}{
		"item": "backpack", "contents": []interface{}{map[string]interface{}{"item": "antidote", "quantity": 1}},
	}}
	st := &combat.Actor(cs).CombatState

	st.BonusActionUsed = true
	if _, err := combat.ProcessPlayerUseItem(db.GetDB(), cs, save, "antidote", ""); err == nil {
		t.Fatal("with the bonus action spent, the pack is out of reach")
	}

	st.BonusActionUsed, st.ActionUsed = false, false
	if _, err := combat.ProcessPlayerUseItem(db.GetDB(), cs, save, "antidote", ""); err != nil {
		t.Fatal(err)
	}
	if !st.BonusActionUsed || !st.ActionUsed {
		t.Errorf("using it from the pack should spend both: action=%v bonus=%v", st.ActionUsed, st.BonusActionUsed)
	}
}

// Holy water burns undead and fiends (2d6 radiant on a hit) and only wets
// anything else.
func TestHolyWaterBurnsOnlyUndeadAndFiends(t *testing.T) {
	for _, tc := range []struct {
		monster string
		burns   bool
	}{{"ghoul", true}, {"giant-rat", false}} {
		combatSetup(t)
		adv, err := character.LoadAdvancement(db.GetDB())
		if err != nil {
			t.Fatal(err)
		}
		save := fighterSave()
		save.Inventory = map[string]interface{}{
			"general_slots": []interface{}{map[string]interface{}{"slot": 0, "item": "holy-water", "quantity": 10}},
			"gear_slots":    map[string]interface{}{},
		}
		cs, err := combat.StartEncounter(db.GetDB(), save, "npub_test", combat.EncounterSpec{MonsterIDs: []string{tc.monster}, EnvironmentID: "forest"}, adv)
		if err != nil {
			t.Fatal(err)
		}
		m := &cs.Monsters[0]
		m.ArmorClass = 0
		me := combat.Actor(cs)
		m.Pos = types.Position{X: me.Pos.X + 2, Y: me.Pos.Y}
		m.CurrentHP, m.MaxHP = 1000, 1000

		hit := false
		for i := 0; i < 10 && !hit; i++ {
			cs.Phase = "active"
			me.CombatState.ActionUsed, me.CombatState.BonusActionUsed = false, false
			log, err := combat.ProcessPlayerUseItem(db.GetDB(), cs, save, "holy-water", m.InstanceID)
			if err != nil {
				t.Fatal(err)
			}
			for _, l := range log {
				if strings.Contains(l, "radiant damage") || strings.Contains(l, "does nothing") {
					hit = true
				}
			}
		}
		if !hit {
			t.Fatalf("%s: ten throws and no hit against AC 0", tc.monster)
		}
		if burned := m.CurrentHP < 1000; burned != tc.burns {
			t.Errorf("%s: burned=%v, want %v (HP %d)", tc.monster, burned, tc.burns, m.CurrentHP)
		}
	}
}

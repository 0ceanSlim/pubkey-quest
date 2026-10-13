package combat_test

import (
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

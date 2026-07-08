// Package combat_test — DB-backed integration tests for the M5 combat systems
// (class abilities + difficulty). These drive the real engine against the
// migrated www/game.db (abilities table, monster stat blocks), so they exercise
// the DB-loaded ability definitions the unit tests stub out. The suite skips if
// www/game.db is missing — run `go run ./cmd/codex --migrate` first.
package combat_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

func combatSetup(t *testing.T) {
	t.Helper()
	helpers.SetupTestEnvironment(t)
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
}

func fighterSave() *types.SaveFile {
	return &types.SaveFile{
		Race: "Human", Class: "Fighter", Experience: 0, HP: 20, MaxHP: 20,
		Stats: map[string]interface{}{
			"strength": 16, "dexterity": 12, "constitution": 14,
			"intelligence": 10, "wisdom": 10, "charisma": 10,
		},
		Inventory: map[string]interface{}{},
		Location:  "forest",
	}
}

// activeFightWithStamina hand-builds an active combat with a full Stamina pool,
// so the ability path is deterministic (no StartCombat initiative RNG).
func activeFightWithStamina() *types.CombatSession {
	return &types.CombatSession{
		Phase: "active",
		Party: []types.PartyCombatant{{
			Type: "player",
			CombatState: types.PlayerCombatState{
				CurrentHP: 12, MaxHP: 20,
				Resource: &types.ResourcePool{Type: "stamina", Label: "Stamina", Current: 10, Max: 10, PerTurn: 2},
			},
		}},
		Monsters: []types.MonsterInstance{{Name: "Wolf", IsAlive: true, CurrentHP: 5, MaxHP: 5}},
	}
}

// StartCombat should seed the martial resource pool and rate the fight.
func TestStartCombatWiresResourceAndDifficulty(t *testing.T) {
	combatSetup(t)
	adv, err := character.LoadAdvancement(db.GetDB())
	if err != nil {
		t.Fatalf("load advancement: %v", err)
	}
	save := fighterSave()
	cs, err := combat.StartCombat(db.GetDB(), save, "npub_test", "wolf", "forest", adv)
	if err != nil {
		t.Fatalf("StartCombat: %v", err)
	}
	st := cs.Party[0].CombatState
	if st.Resource == nil || st.Resource.Type != "stamina" {
		t.Fatalf("expected a stamina pool, got %+v", st.Resource)
	}
	if st.Resource.Current != st.Resource.Max || st.Resource.Max != 10 {
		t.Errorf("fighter stamina should start full at 10, got %d/%d", st.Resource.Current, st.Resource.Max)
	}
	if cs.Difficulty == "" {
		t.Error("StartCombat should rate the fight's difficulty")
	}
}

// Second Wind, loaded from the DB, heals and spends stamina; it's once per fight.
func TestSecondWindEndToEnd(t *testing.T) {
	combatSetup(t)
	adv, _ := character.LoadAdvancement(db.GetDB())
	save := fighterSave()
	cs := activeFightWithStamina()

	before := cs.Party[0].CombatState.CurrentHP
	if _, err := combat.ProcessPlayerAbility(db.GetDB(), cs, save, "second-wind", adv); err != nil {
		t.Fatalf("second-wind: %v", err)
	}
	st := &cs.Party[0].CombatState
	if st.CurrentHP <= before {
		t.Errorf("expected Second Wind to heal, HP %d → %d", before, st.CurrentHP)
	}
	if st.Resource.Current != 7 { // 10 − 3 cost
		t.Errorf("stamina should drop to 7, got %d", st.Resource.Current)
	}
	if !st.BonusActionUsed {
		t.Error("Second Wind should consume the bonus action")
	}

	// Once-per-combat: even with the bonus action freed, it can't fire twice.
	st.BonusActionUsed = false
	if _, err := combat.ProcessPlayerAbility(db.GetDB(), cs, save, "second-wind", adv); err == nil {
		t.Error("Second Wind should be usable only once per fight")
	}
}

// A fighter can't use a barbarian ability.
func TestAbilityRejectsWrongClass(t *testing.T) {
	combatSetup(t)
	adv, _ := character.LoadAdvancement(db.GetDB())
	save := fighterSave()
	cs := activeFightWithStamina()
	if _, err := combat.ProcessPlayerAbility(db.GetDB(), cs, save, "enter-rage", adv); err == nil {
		t.Error("a fighter should not be able to Enter Rage")
	}
}

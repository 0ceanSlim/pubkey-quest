package combat

import (
	"testing"

	"pubkey-quest/types"
)

func statMap(str, dex, con, intel, wis, cha int) map[string]interface{} {
	return map[string]interface{}{
		"strength": str, "dexterity": dex, "constitution": con,
		"intelligence": intel, "wisdom": wis, "charisma": cha,
	}
}

func TestInitResourcePool(t *testing.T) {
	cases := []struct {
		class    string
		wantType string
		wantCur  int
		wantMax  int
	}{
		{"fighter", "stamina", 10, 10}, // starts at max
		{"barbarian", "rage", 0, 100},  // builds from 0
		{"rogue", "cunning", 10, 10},
		{"monk", "ki", 4, 4}, // wisMod(+3 at 16) + level(1) = 4, starts at max
	}
	for _, c := range cases {
		var st types.PlayerCombatState
		InitResourcePool(&st, c.class, 1, statMap(10, 10, 10, 10, 16, 10))
		if st.Resource == nil {
			t.Fatalf("%s: expected a resource pool", c.class)
		}
		if st.Resource.Type != c.wantType || st.Resource.Current != c.wantCur || st.Resource.Max != c.wantMax {
			t.Errorf("%s: got %s %d/%d, want %s %d/%d", c.class,
				st.Resource.Type, st.Resource.Current, st.Resource.Max,
				c.wantType, c.wantCur, c.wantMax)
		}
	}

	// Casters get no pool.
	var caster types.PlayerCombatState
	InitResourcePool(&caster, "druid", 5, statMap(10, 10, 10, 10, 16, 10))
	if caster.Resource != nil {
		t.Errorf("caster should have no ability resource pool, got %+v", caster.Resource)
	}
}

func TestConsumePlayerAction(t *testing.T) {
	// No extra actions → the action is spent.
	st := types.PlayerCombatState{}
	consumePlayerAction(&st)
	if !st.ActionUsed {
		t.Error("expected ActionUsed after consuming with no extra actions")
	}

	// With an extra action → it's burned first and the action stays available.
	st = types.PlayerCombatState{ExtraActions: 1}
	consumePlayerAction(&st)
	if st.ActionUsed {
		t.Error("action should remain available while extra actions exist")
	}
	if st.ExtraActions != 0 {
		t.Errorf("extra action should be consumed, got %d", st.ExtraActions)
	}
	consumePlayerAction(&st)
	if !st.ActionUsed {
		t.Error("expected ActionUsed once extra actions are exhausted")
	}
}

func TestApplyPlayerDamageRiders(t *testing.T) {
	// Rage adds a percentage of the base hit.
	st := types.PlayerCombatState{RageTurnsLeft: 2, RageDamageBonusPct: 50}
	got, log := applyPlayerDamageRiders(&st, 10, false)
	if got != 15 {
		t.Errorf("rage +50%% of 10: got %d, want 15", got)
	}
	if len(log) == 0 {
		t.Error("expected a rage rider log line")
	}

	// Sneak Attack adds dice and is consumed on the first hit.
	st = types.PlayerCombatState{PendingSneakDice: "2d6"}
	got, _ = applyPlayerDamageRiders(&st, 5, false)
	if got < 5+2 || got > 5+12 {
		t.Errorf("sneak 2d6 on 5: got %d, want 7..17", got)
	}
	if st.PendingSneakDice != "" {
		t.Error("sneak dice should be cleared after the first hit")
	}
	// Second hit same turn gets no sneak.
	got, _ = applyPlayerDamageRiders(&st, 5, false)
	if got != 5 {
		t.Errorf("second hit should have no sneak rider: got %d, want 5", got)
	}

	// No riders → damage passes through unchanged.
	st = types.PlayerCombatState{}
	if got, _ := applyPlayerDamageRiders(&st, 8, true); got != 8 {
		t.Errorf("no riders: got %d, want 8", got)
	}
}

func TestTickPlayerAbilities(t *testing.T) {
	// Per-turn regen, clamped to max.
	st := types.PlayerCombatState{Resource: &types.ResourcePool{Current: 9, Max: 10, PerTurn: 2}}
	tickPlayerAbilities(&st)
	if st.Resource.Current != 10 {
		t.Errorf("regen clamps to max: got %d, want 10", st.Resource.Current)
	}

	// Rage counts down and clears its riders at 0.
	st = types.PlayerCombatState{RageTurnsLeft: 1, RageDamageBonusPct: 50, RageResistPct: 25}
	log := tickPlayerAbilities(&st)
	if st.RageTurnsLeft != 0 || st.RageDamageBonusPct != 0 || st.RageResistPct != 0 {
		t.Errorf("rage should clear at 0: %+v", st)
	}
	if len(log) == 0 {
		t.Error("expected a 'rage subsides' log line")
	}

	// Rage with turns to spare only decrements.
	st = types.PlayerCombatState{RageTurnsLeft: 3, RageDamageBonusPct: 50}
	tickPlayerAbilities(&st)
	if st.RageTurnsLeft != 2 || st.RageDamageBonusPct != 50 {
		t.Errorf("rage should persist: turns=%d bonus=%d", st.RageTurnsLeft, st.RageDamageBonusPct)
	}
}

func TestAbilityMechanics_Buffs(t *testing.T) {
	save := &types.SaveFile{Class: "barbarian", Stats: statMap(16, 10, 14, 10, 10, 10)}

	// enter-rage (tier 0) sets the rage riders.
	st := types.PlayerCombatState{}
	abilityMechanics["enter-rage"].apply(nil, &st, save, 1, 0)
	if st.RageDamageBonusPct != 50 || st.RageResistPct != 25 || st.RageTurnsLeft != 4 {
		t.Errorf("enter-rage t0: %+v", st)
	}

	// second-wind heals a percentage of max HP without overhealing.
	st = types.PlayerCombatState{CurrentHP: 4, MaxHP: 20}
	abilityMechanics["second-wind"].apply(nil, &st, save, 1, 0) // 25% of 20 = 5
	if st.CurrentHP != 9 {
		t.Errorf("second-wind t0: got %d HP, want 9", st.CurrentHP)
	}
	st = types.PlayerCombatState{CurrentHP: 19, MaxHP: 20}
	abilityMechanics["second-wind"].apply(nil, &st, save, 1, 0)
	if st.CurrentHP != 20 {
		t.Errorf("second-wind should not overheal: got %d, want 20", st.CurrentHP)
	}

	// sneak-attack readies dice; patient-defense sets Dodging; surge/flurry grant actions.
	st = types.PlayerCombatState{}
	abilityMechanics["sneak-attack"].apply(nil, &st, save, 1, 0)
	if st.PendingSneakDice != "2d6" {
		t.Errorf("sneak-attack t0: got %q, want 2d6", st.PendingSneakDice)
	}
	st = types.PlayerCombatState{}
	abilityMechanics["patient-defense"].apply(nil, &st, save, 3, 0)
	if !st.Dodging {
		t.Error("patient-defense should set Dodging")
	}
	st = types.PlayerCombatState{}
	abilityMechanics["action-surge"].apply(nil, &st, save, 5, 0)
	if st.ExtraActions != 1 {
		t.Errorf("action-surge t0: got %d extra actions, want 1", st.ExtraActions)
	}
	st = types.PlayerCombatState{}
	abilityMechanics["flurry-of-blows"].apply(nil, &st, save, 1, 0)
	if st.ExtraActions != 1 {
		t.Errorf("flurry t0: got %d extra actions, want 1", st.ExtraActions)
	}
}

func TestIntimidatingRoarFrightens(t *testing.T) {
	// A monster with terrible WIS and no proficiency will (almost) always fail;
	// verify the mechanic either frightens it or reports a resist — never a no-op.
	save := &types.SaveFile{Class: "barbarian", Stats: statMap(18, 10, 14, 10, 10, 10)}
	cs := &types.CombatSession{
		Monsters: []types.MonsterInstance{{
			Name: "Goblin", IsAlive: true,
			Data: types.MonsterData{Stats: types.MonsterStats{Wisdom: 6}},
		}},
	}
	st := types.PlayerCombatState{}
	log := abilityMechanics["intimidating-roar"].apply(cs, &st, save, 3, 0)
	if len(log) == 0 {
		t.Fatal("roar should always produce a log line")
	}
	// If frightened, the condition is on the monster with the disadvantage footprint.
	if HasCondition(cs.Monsters[0].Conditions, "frightened") {
		if specFor("frightened").OwnAttacksDisadvantage != true {
			t.Error("frightened should impose disadvantage on the monster's attacks")
		}
	}
}

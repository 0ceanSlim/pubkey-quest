package spells

import (
	"testing"

	"pubkey-quest/types"
)

// ─── Test fixtures ───────────────────────────────────────────────────────────

// saveWith returns a level-appropriate Druid save that knows and has prepared
// spellID, with the given mana pool and an empty inventory.
func saveWith(spellID string, mana int) *types.SaveFile {
	return &types.SaveFile{
		Class:   "Druid",
		Mana:    mana,
		MaxMana: 10,
		HP:      20,
		MaxHP:   30,
		Stats: map[string]interface{}{
			"wisdom":       float64(16), // +3 modifier → prof(2)+3 attack, DC 13 at L1
			"constitution": float64(14),
		},
		KnownSpells: []string{spellID},
		SpellSlots: map[string]interface{}{
			"cantrips": []interface{}{
				map[string]interface{}{"slot": float64(0), "spell": spellID, "quantity": float64(0)},
			},
		},
		Inventory: map[string]interface{}{
			"general_slots": []interface{}{},
			"gear_slots":    map[string]interface{}{},
		},
	}
}

func testMonster(ac, con int) *types.MonsterInstance {
	return &types.MonsterInstance{
		Name:       "Dummy",
		ArmorClass: ac,
		CurrentHP:  20,
		MaxHP:      20,
		IsAlive:    true,
		Data: types.MonsterData{
			ArmorClass:   ac,
			Stats:        types.MonsterStats{Strength: 10, Dexterity: 10, Constitution: con, Intelligence: 10, Wisdom: 10, Charisma: 10},
			SavingThrows: map[string]int{},
		},
	}
}

// testDeps returns deterministic deps: RollD20 is fixed, RollDice is fixed, and
// monster damage is 6 (12 on a crit) regardless of dice/resistances.
func testDeps(d20, dice int) Deps {
	return Deps{
		RollD20:  func() int { return d20 },
		RollDice: func(_ string, _ bool) int { return dice },
		ResolveMonsterDamage: func(_ string, _ int, _ string, crit bool, _ *types.MonsterInstance) int {
			if crit {
				return 12
			}
			return 6
		},
	}
}

func noFocus(string) bool { return false }

// A scroll casts its spell even when the caster hasn't learned/prepared it and
// lacks its components — but still pays mana and resolves by shape.
func TestCast_FromScrollBypassesGates(t *testing.T) {
	unprepared := func() *types.SaveFile {
		return &types.SaveFile{
			Class: "Druid", Mana: 5, MaxMana: 10, HP: 20, MaxHP: 30,
			Stats:     map[string]interface{}{"wisdom": float64(16)},
			Inventory: map[string]interface{}{"general_slots": []interface{}{}, "gear_slots": map[string]interface{}{}},
		}
	}
	spell := map[string]interface{}{
		"name": "Fire Bolt", "spell_attack": "ranged", "mana_cost": float64(2),
		"damage": "1d10", "damage_type": "fire",
		"material_component": map[string]interface{}{
			"required": []interface{}{map[string]interface{}{"component": "sulfur", "quantity": float64(1)}},
		},
	}

	// A normal cast fails — the spell isn't learned.
	if _, err := resolveCast(testDeps(20, 0), unprepared(), spell, "fire-bolt", 1, testMonster(10, 10), noFocus, false); err == nil {
		t.Error("a normal cast should fail when the spell isn't learned")
	}

	// A scroll cast succeeds despite unknown/unprepared + missing component, and
	// still spends mana.
	s := unprepared()
	res, err := resolveCast(testDeps(20, 0), s, spell, "fire-bolt", 1, testMonster(10, 10), noFocus, true)
	if err != nil {
		t.Fatalf("scroll cast should bypass the gates: %v", err)
	}
	if res.ManaSpent != 2 || s.Mana != 3 {
		t.Errorf("scroll should still spend mana (2): spent %d, left %d", res.ManaSpent, s.Mana)
	}
	if res.Damage <= 0 {
		t.Errorf("scroll Fire Bolt should deal damage, got %d", res.Damage)
	}

	// A scroll still requires mana.
	poor := unprepared()
	poor.Mana = 0
	if _, err := resolveCast(testDeps(20, 0), poor, spell, "fire-bolt", 1, testMonster(10, 10), noFocus, true); err == nil {
		t.Error("scroll cast should still require mana")
	}
}

// ─── Shape resolution ────────────────────────────────────────────────────────

func TestCast_AttackCritHit(t *testing.T) {
	save := saveWith("atk", 5)
	spell := map[string]interface{}{
		"name": "Zap", "spell_attack": "ranged", "damage": "1d10", "damage_type": "fire", "mana_cost": float64(2),
	}
	res, err := resolveCast(testDeps(20, 0), save, spell, "atk", 1, testMonster(15, 10), noFocus, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Hit || !res.Crit {
		t.Errorf("expected crit hit, got hit=%v crit=%v", res.Hit, res.Crit)
	}
	if res.Damage != 12 {
		t.Errorf("expected 12 crit damage, got %d", res.Damage)
	}
	if save.Mana != 3 {
		t.Errorf("expected mana 5-2=3, got %d", save.Mana)
	}
}

func TestCast_AttackMiss(t *testing.T) {
	save := saveWith("atk", 5)
	spell := map[string]interface{}{
		"name": "Zap", "spell_attack": "ranged", "damage": "1d10", "mana_cost": float64(2),
	}
	// d20=2, bonus = prof(2)+wis(+3)=5 → total 7 < AC 20 → miss.
	res, err := resolveCast(testDeps(2, 0), save, spell, "atk", 1, testMonster(20, 10), noFocus, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Hit {
		t.Errorf("expected miss")
	}
	if res.Damage != 0 {
		t.Errorf("expected 0 damage on miss, got %d", res.Damage)
	}
	if save.Mana != 3 {
		t.Errorf("mana should still be spent on a miss: got %d", save.Mana)
	}
}

func TestCast_AutoHit(t *testing.T) {
	save := saveWith("mm", 5)
	spell := map[string]interface{}{
		"name": "Missiles", "spell_attack": "automatic", "damage": "3d4+3", "mana_cost": float64(2),
	}
	res, err := resolveCast(testDeps(1, 0), save, spell, "mm", 1, testMonster(30, 10), noFocus, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Hit || res.Damage != 6 {
		t.Errorf("auto-hit should land for 6 regardless of AC/roll, got hit=%v dmg=%d", res.Hit, res.Damage)
	}
}

func TestCast_SaveFail_FullDamage(t *testing.T) {
	save := saveWith("spray", 5)
	spell := map[string]interface{}{
		"name": "Spray", "save_type": "constitution", "damage": "1d12", "mana_cost": float64(1),
	}
	// DC 13; monster con mod 0; d20=1 → 1 < 13 → fail → full 6.
	res, err := resolveCast(testDeps(1, 0), save, spell, "spray", 1, testMonster(15, 10), noFocus, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SaveMade {
		t.Errorf("expected failed save")
	}
	if res.Damage != 6 {
		t.Errorf("expected full 6 on failed save, got %d", res.Damage)
	}
	if res.SaveDC != 13 {
		t.Errorf("expected DC 13, got %d", res.SaveDC)
	}
}

func TestCast_SaveMade_HalfDamage(t *testing.T) {
	save := saveWith("spray", 5)
	spell := map[string]interface{}{
		"name": "Spray", "save_type": "constitution", "damage": "1d12", "mana_cost": float64(1),
	}
	// d20=20 → 20 >= 13 → save made → half of 6 = 3.
	res, err := resolveCast(testDeps(20, 0), save, spell, "spray", 1, testMonster(15, 10), noFocus, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.SaveMade {
		t.Errorf("expected made save")
	}
	if res.Damage != 3 {
		t.Errorf("expected half (3) on made save, got %d", res.Damage)
	}
}

func TestCast_Heal(t *testing.T) {
	save := saveWith("cure", 5)
	spell := map[string]interface{}{
		"name": "Cure", "heal": "1d8", "mana_cost": float64(2),
	}
	// RollDice=5 + wis mod(+3) = 8 heal. Out of combat: target nil.
	res, err := resolveCast(testDeps(0, 5), save, spell, "cure", 1, nil, noFocus, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Shape != "heal" || res.Heal != 8 {
		t.Errorf("expected heal 8, got shape=%s heal=%d", res.Shape, res.Heal)
	}
}

func TestCast_Buff_UnmappedIsNarrative(t *testing.T) {
	save := saveWith("buffx", 5)
	spell := map[string]interface{}{
		"name": "Bravado", "effect": "You feel bold.", "mana_cost": float64(1), "concentration": true,
	}
	res, err := resolveCast(testDeps(0, 0), save, spell, "buffx", 1, nil, noFocus, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Shape != "buff" {
		t.Errorf("expected buff shape, got %s", res.Shape)
	}
	if res.EffectID != "" {
		t.Errorf("unmapped buff should apply no effect, got %q", res.EffectID)
	}
	if save.Mana != 4 {
		t.Errorf("mana should be spent, got %d", save.Mana)
	}
}

// ─── Validation (no cost spent on failure) ───────────────────────────────────

func TestCast_NotKnown(t *testing.T) {
	save := saveWith("known", 5)
	spell := map[string]interface{}{"name": "X", "mana_cost": float64(1)}
	if _, err := resolveCast(testDeps(10, 0), save, spell, "unknown", 1, nil, noFocus, false); err == nil {
		t.Errorf("expected error for unknown spell")
	}
	if save.Mana != 5 {
		t.Errorf("mana must not be spent on a rejected cast, got %d", save.Mana)
	}
}

func TestCast_NotPrepared(t *testing.T) {
	save := saveWith("prepared", 5)
	save.KnownSpells = append(save.KnownSpells, "unprepped")
	spell := map[string]interface{}{"name": "X", "heal": "1d8", "mana_cost": float64(1)}
	if _, err := resolveCast(testDeps(0, 5), save, spell, "unprepped", 1, nil, noFocus, false); err == nil {
		t.Errorf("expected error for unprepared spell")
	}
	if save.Mana != 5 {
		t.Errorf("mana must not be spent, got %d", save.Mana)
	}
}

func TestCast_InsufficientMana(t *testing.T) {
	save := saveWith("cure", 1)
	spell := map[string]interface{}{"name": "Cure", "heal": "1d8", "mana_cost": float64(2)}
	if _, err := resolveCast(testDeps(0, 5), save, spell, "cure", 1, nil, noFocus, false); err == nil {
		t.Errorf("expected error for insufficient mana")
	}
	if save.Mana != 1 {
		t.Errorf("mana must not be spent, got %d", save.Mana)
	}
}

func TestCast_AttackNeedsTarget(t *testing.T) {
	save := saveWith("atk", 5)
	spell := map[string]interface{}{"name": "Zap", "spell_attack": "ranged", "damage": "1d10", "mana_cost": float64(2)}
	if _, err := resolveCast(testDeps(20, 0), save, spell, "atk", 1, nil, noFocus, false); err == nil {
		t.Errorf("attack spell with no target should error")
	}
	if save.Mana != 5 {
		t.Errorf("mana must not be spent, got %d", save.Mana)
	}
}

// ─── Material components ─────────────────────────────────────────────────────

func invWithComponent(itemID string, qty int) map[string]interface{} {
	return map[string]interface{}{
		"general_slots": []interface{}{
			map[string]interface{}{
				"item": "component-pouch", "quantity": float64(1), "slot": float64(0),
				"contents": []interface{}{
					map[string]interface{}{"item": itemID, "quantity": float64(qty)},
				},
			},
		},
		"gear_slots": map[string]interface{}{},
	}
}

func TestCast_ConsumesComponent(t *testing.T) {
	save := saveWith("burn", 5)
	save.Inventory = invWithComponent("sulfur", 3)
	spell := map[string]interface{}{
		"name": "Burn", "save_type": "dexterity", "damage": "3d6", "mana_cost": float64(2),
		"material_component": map[string]interface{}{
			"required": []interface{}{
				map[string]interface{}{"component": "sulfur", "quantity": float64(2)},
			},
		},
	}
	res, err := resolveCast(testDeps(1, 0), save, spell, "burn", 1, testMonster(15, 10), noFocus, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := countComponent(save.Inventory, "sulfur"); got != 1 {
		t.Errorf("expected 1 sulfur left after consuming 2, got %d", got)
	}
	if res.Damage != 6 {
		t.Errorf("expected damage 6, got %d", res.Damage)
	}
}

func TestCast_MissingComponent(t *testing.T) {
	save := saveWith("burn", 5)
	save.Inventory = invWithComponent("sulfur", 1) // need 2, have 1
	spell := map[string]interface{}{
		"name": "Burn", "save_type": "dexterity", "damage": "3d6", "mana_cost": float64(2),
		"material_component": map[string]interface{}{
			"required": []interface{}{
				map[string]interface{}{"component": "sulfur", "quantity": float64(2)},
			},
		},
	}
	if _, err := resolveCast(testDeps(1, 0), save, spell, "burn", 1, testMonster(15, 10), noFocus, false); err == nil {
		t.Errorf("expected error for missing component")
	}
	if save.Mana != 5 {
		t.Errorf("mana must not be spent when a component is missing, got %d", save.Mana)
	}
	if got := countComponent(save.Inventory, "sulfur"); got != 1 {
		t.Errorf("component must not be consumed on a rejected cast, got %d", got)
	}
}

func TestCast_FocusProvidesComponent(t *testing.T) {
	save := saveWith("burn", 5)
	save.Inventory = map[string]interface{}{ // no sulfur held
		"general_slots": []interface{}{},
		"gear_slots":    map[string]interface{}{},
	}
	spell := map[string]interface{}{
		"name": "Burn", "save_type": "dexterity", "damage": "3d6", "mana_cost": float64(2),
		"material_component": map[string]interface{}{
			"required": []interface{}{
				map[string]interface{}{"component": "sulfur", "quantity": float64(2)},
			},
		},
	}
	focus := func(c string) bool { return c == "sulfur" } // a rod is equipped
	if _, err := resolveCast(testDeps(1, 0), save, spell, "burn", 1, testMonster(15, 10), focus, false); err != nil {
		t.Errorf("focus should satisfy the component, got error: %v", err)
	}
}

// ─── Inventory helpers ───────────────────────────────────────────────────────

func TestCountAndConsumeComponent(t *testing.T) {
	inv := map[string]interface{}{
		"general_slots": []interface{}{
			map[string]interface{}{"item": "salt", "quantity": float64(4), "slot": float64(0)},
			map[string]interface{}{
				"item": "pouch", "quantity": float64(1), "slot": float64(1),
				"contents": []interface{}{
					map[string]interface{}{"item": "salt", "quantity": float64(3)},
				},
			},
		},
		"gear_slots": map[string]interface{}{
			"bag": map[string]interface{}{
				"item": "backpack", "quantity": float64(1),
				"contents": []interface{}{
					map[string]interface{}{"item": "salt", "quantity": float64(2)},
				},
			},
		},
	}
	if got := countComponent(inv, "salt"); got != 9 {
		t.Fatalf("expected 9 salt across all pools, got %d", got)
	}
	if removed := consumeComponent(inv, "salt", 5); removed != 5 {
		t.Fatalf("expected to remove 5, removed %d", removed)
	}
	if got := countComponent(inv, "salt"); got != 4 {
		t.Errorf("expected 4 salt remaining after removing 5, got %d", got)
	}
}

func TestIsSpellPrepared(t *testing.T) {
	save := saveWith("ready", 5)
	if !IsSpellPrepared(save, "ready") {
		t.Errorf("expected 'ready' to be prepared")
	}
	if IsSpellPrepared(save, "other") {
		t.Errorf("did not expect 'other' to be prepared")
	}
}

func TestSpellShape(t *testing.T) {
	cases := []struct {
		spell map[string]interface{}
		want  string
	}{
		{map[string]interface{}{"spell_attack": "ranged"}, "attack"},
		{map[string]interface{}{"spell_attack": "automatic"}, "auto"},
		{map[string]interface{}{"save_type": "dexterity"}, "save"},
		{map[string]interface{}{"heal": "1d8"}, "heal"},
		{map[string]interface{}{"effect": "buffed"}, "buff"},
		{map[string]interface{}{"name": "Light"}, "utility"},
	}
	for _, c := range cases {
		if got := spellShape(c.spell); got != c.want {
			t.Errorf("spellShape(%v) = %s, want %s", c.spell, got, c.want)
		}
	}
}

package combat

import (
	"testing"

	"pubkey-quest/cmd/server/game/spells"
	"pubkey-quest/types"
)

// Unit tests for the spell buff / rider / concentration mechanics that need no
// database. "1d1" dice keep damage deterministic.

func castOf(id string, spell map[string]interface{}) *spells.CastResult {
	return &spells.CastResult{SpellID: id, SpellName: id, Spell: spell}
}

func TestCombatBuffOfReadsSpellJSON(t *testing.T) {
	b := combatBuffOf(map[string]interface{}{"combat_buff": map[string]interface{}{
		"target": "party", "rounds": float64(10), "attack_dice": "1d4", "save_dice": "1d4",
	}})
	if b == nil || b.Target != "party" || b.Rounds != 10 || b.AttackDice != "1d4" || b.SaveDice != "1d4" {
		t.Fatalf("parsed %+v", b)
	}
	if combatBuffOf(map[string]interface{}{}) != nil {
		t.Error("no combat_buff should parse to nil")
	}
}

func TestPartyBuffLandsOnEveryone(t *testing.T) {
	cs := partyBoard(goblins(1))
	res := castOf("bless", map[string]interface{}{"combat_buff": map[string]interface{}{"target": "party", "rounds": float64(10), "attack_dice": "1d4"}})
	applySpellBuff(cs, &cs.Party[0], res, nil, nil, 13, 3)
	for _, p := range cs.Party {
		if len(p.CombatState.SpellBuffs) != 1 || p.CombatState.SpellBuffs[0].SpellID != "bless" {
			t.Errorf("%s: buffs %+v", p.Name, p.CombatState.SpellBuffs)
		}
	}
	// Casting it again refreshes rather than stacks.
	applySpellBuff(cs, &cs.Party[0], res, nil, nil, 13, 3)
	if n := len(cs.Party[1].CombatState.SpellBuffs); n != 1 {
		t.Errorf("recast should refresh, got %d buffs", n)
	}
}

func TestAllyBuffGoesToTheAllyOnly(t *testing.T) {
	cs := partyBoard(goblins(1))
	res := castOf("shield-of-faith", map[string]interface{}{"combat_buff": map[string]interface{}{"target": "ally", "ac_bonus": float64(2)}})
	applySpellBuff(cs, &cs.Party[0], res, &cs.Party[1], nil, 13, 3)
	if len(cs.Party[0].CombatState.SpellBuffs) != 0 || len(cs.Party[1].CombatState.SpellBuffs) != 1 {
		t.Fatalf("buffs: caster %v, ally %v", cs.Party[0].CombatState.SpellBuffs, cs.Party[1].CombatState.SpellBuffs)
	}
	if ac := buffedAC(&cs.Party[1].CombatState, 12); ac != 14 {
		t.Errorf("AC = %d, want 14", ac)
	}
}

func TestACFloor(t *testing.T) {
	st := &types.PlayerCombatState{SpellBuffs: []types.SpellBuff{{ACFloor: 16}}}
	if ac := buffedAC(st, 11); ac != 16 {
		t.Errorf("floor: AC = %d, want 16", ac)
	}
	if ac := buffedAC(st, 18); ac != 18 {
		t.Errorf("floor shouldn't lower AC: %d", ac)
	}
}

func TestTempHPSoaksFirst(t *testing.T) {
	cs := boardWith(goblins(1))
	cs.Party[0].CombatState.TempHP = 5
	applyDamageToPlayer(cs, 7)
	st := cs.Party[0].CombatState
	if st.TempHP != 0 || st.CurrentHP != 8 {
		t.Errorf("temp %d hp %d, want 0 / 8", st.TempHP, st.CurrentHP)
	}
}

func TestDeathWardCatchesTheFall(t *testing.T) {
	cs := boardWith(goblins(1))
	cs.Party[0].CombatState.SpellBuffs = []types.SpellBuff{{Name: "Spare the Dying", DeathWard: true}}
	applyDamageToPlayer(cs, 50)
	st := cs.Party[0].CombatState
	if st.CurrentHP != 1 || st.IsUnconscious || cs.Phase != "active" {
		t.Fatalf("hp %d unconscious %v phase %s", st.CurrentHP, st.IsUnconscious, cs.Phase)
	}
	if len(st.SpellBuffs) != 0 {
		t.Error("the ward should be spent")
	}
	applyDamageToPlayer(cs, 50)
	if cs.Phase != "death_saves" {
		t.Errorf("a second fall should drop you, phase %s", cs.Phase)
	}
}

func TestReviveGetsAFallenAllyUp(t *testing.T) {
	cs := partyBoard(goblins(1))
	cs.Actor = 1
	applyDamageToPlayer(cs, 50)
	res := castOf("spare-the-dying", map[string]interface{}{"combat_buff": map[string]interface{}{"target": "ally", "revive": true, "death_ward": true}})
	applySpellBuff(cs, &cs.Party[0], res, &cs.Party[1], nil, 13, 3)
	if st := cs.Party[1].CombatState; st.IsUnconscious || st.CurrentHP != 1 {
		t.Errorf("ally should be up at 1 HP: %+v", st)
	}
}

func TestTickBuffsExpiresAndTopsUp(t *testing.T) {
	st := &types.PlayerCombatState{SpellBuffs: []types.SpellBuff{
		{Name: "Shield", Rounds: 1, ACBonus: 5},
		{Name: "Bless", Rounds: 3, AttackDice: "1d4"},
		{Name: "Hex", HitDice: "1d6"}, // rest of the fight
		{Name: "Heroism", Rounds: 5, TempHPEachTurn: 3},
	}}
	tickBuffs(st)
	if len(st.SpellBuffs) != 3 || buffedAC(st, 10) != 10 {
		t.Errorf("shield should expire at the start of the next turn: %+v", st.SpellBuffs)
	}
	if st.TempHP != 3 {
		t.Errorf("heroism temp HP = %d, want 3", st.TempHP)
	}
}

func TestHitBuffs(t *testing.T) {
	ms := goblins(2)
	ms[1].InstanceID = "goblin#2"
	cs := boardWith(ms)
	me := &cs.Party[0]
	me.CombatState.SpellBuffs = []types.SpellBuff{
		{Name: "Smite", HitDice: "1d1", HitType: "fire", HitOnce: true, MeleeOnly: true},
		{Name: "Hex", HitDice: "1d1", HitType: "necrotic", HitSpells: true, MarkedID: "goblin#2"},
	}
	// Ranged hit on goblin 1: the melee smite and the mark (on goblin 2) don't apply.
	if extra, _ := buffHitDamage(cs, me, &cs.Monsters[0], false, false, false, "piercing"); extra != 0 {
		t.Errorf("ranged hit on the unmarked goblin: extra %d, want 0", extra)
	}
	// Melee hit on goblin 2: smite (spent) + hex.
	if extra, _ := buffHitDamage(cs, me, &cs.Monsters[1], true, false, false, "slashing"); extra != 2 {
		t.Errorf("melee hit on the hexed goblin: extra %d, want 2", extra)
	}
	if len(me.CombatState.SpellBuffs) != 1 || me.CombatState.SpellBuffs[0].Name != "Hex" {
		t.Errorf("the smite should be spent, hex kept: %+v", me.CombatState.SpellBuffs)
	}
	// A spell hit carries hex (hit_spells).
	if extra, _ := buffHitDamage(cs, me, &cs.Monsters[1], false, true, false, "force"); extra != 1 {
		t.Errorf("spell hit on the hexed goblin: extra %d, want 1", extra)
	}
}

func TestNextAttackAdvantageIsSpent(t *testing.T) {
	st := &types.PlayerCombatState{SpellBuffs: []types.SpellBuff{{Name: "True Strike", Rounds: 2, NextAttackAdvantage: true}}}
	if _, adv, _ := buffAttackRoll(st, true); adv != 1 {
		t.Errorf("adv = %d, want 1", adv)
	}
	if len(st.SpellBuffs) != 0 {
		t.Errorf("true strike should be gone once used: %+v", st.SpellBuffs)
	}
	if _, adv, _ := buffAttackRoll(st, true); adv != 0 {
		t.Error("advantage should only apply once")
	}
}

func TestLandConditionRespectsTypesAndImmunities(t *testing.T) {
	m := goblins(1)[0]
	m.Data.Type = "Humanoid"
	beastOnly := &types.SpellCondition{Condition: "charmed", CreatureTypes: []string{"beast"}}
	if _, ok := landCondition(&m, beastOnly, "animal-friendship", "a", 12); ok || len(m.Conditions) != 0 {
		t.Error("a beast-only condition shouldn't land on a humanoid")
	}
	m.Data.ConditionImmunities = []string{"frightened"}
	if _, ok := landCondition(&m, &types.SpellCondition{Condition: "frightened"}, "cause-fear", "a", 12); ok {
		t.Error("immune monsters shouldn't be frightened")
	}
	if _, ok := landCondition(&m, &types.SpellCondition{Condition: "restrained", SaveEachTurn: "strength"}, "entangle", "a", 13); !ok {
		t.Fatal("restrained should land")
	}
	c := m.Conditions[0]
	if c.DurationRounds != -1 || c.SaveDC != 13 || c.SaveStat != "strength" || c.Source != "entangle" || c.CasterID != "a" {
		t.Errorf("condition %+v", c)
	}
}

func TestDropConcentrationEndsWhatItHeld(t *testing.T) {
	cs := partyBoard(goblins(1))
	me := &cs.Party[0]
	me.Concentration = &types.ConcentrationState{SpellID: "bless", SpellName: "Bless"}
	for i := range cs.Party {
		cs.Party[i].CombatState.SpellBuffs = []types.SpellBuff{
			{SpellID: "bless", CasterID: me.ID, AttackDice: "1d4"},
			{SpellID: "bless", CasterID: "someone-else", AttackDice: "1d4"},
		}
	}
	cs.Monsters[0].Conditions = []types.CombatCondition{
		{Name: "restrained", Source: "bless", CasterID: me.ID},
		{Name: "prone"},
	}
	dropConcentration(cs, me, &types.SaveFile{}, "")
	if me.Concentration != nil {
		t.Error("concentration should clear")
	}
	for _, p := range cs.Party {
		if len(p.CombatState.SpellBuffs) != 1 || p.CombatState.SpellBuffs[0].CasterID != "someone-else" {
			t.Errorf("%s keeps %+v", p.Name, p.CombatState.SpellBuffs)
		}
	}
	if len(cs.Monsters[0].Conditions) != 1 || cs.Monsters[0].Conditions[0].Name != "prone" {
		t.Errorf("monster keeps %+v", cs.Monsters[0].Conditions)
	}
}

func TestAttackDefense(t *testing.T) {
	cs := partyBoard(goblins(1))
	m := &cs.Monsters[0]
	m.Data.Type = "Undead"
	ann := &cs.Party[0]
	ann.CombatState.SpellBuffs = []types.SpellBuff{
		{AttackersDisadvantage: true},
		{WardTypes: []string{"undead"}},
		{RangedDefense: true},
	}
	if adv, _, lost, _ := attackDefense(cs, ann, m, false); adv != -2 || lost {
		t.Errorf("melee: adv %d lost %v, want -2", adv, lost)
	}
	if adv, _, _, _ := attackDefense(cs, ann, m, true); adv != -3 {
		t.Errorf("ranged: adv %d, want -3", adv)
	}
	// A dueled monster attacking someone other than its duellist: disadvantage.
	m.Conditions = []types.CombatCondition{{Name: "dueled", CasterID: "a"}}
	if adv, _, _, _ := attackDefense(cs, &cs.Party[1], m, false); adv != -1 {
		t.Errorf("duel: adv %d, want -1", adv)
	}
}

func TestDuelHoldsTheMonster(t *testing.T) {
	cs := partyBoard(goblins(1))
	m := &cs.Monsters[0]
	m.Threat = map[string]int{"b": 10}
	m.Conditions = []types.CombatCondition{{Name: "dueled", CasterID: "a"}}
	if got := aiTarget(cs, m); got.ID != "a" {
		t.Errorf("dueled monster targets %s, want a", got.ID)
	}
	// Badly hurt, it still won't run.
	m.Data.Behavior.FleeThreshold = 0.9
	m.CurrentHP = 1
	m.Pos = types.Position{X: 8, Y: 3}
	if d := DecideMonsterAction(cs, m); d.Action == "retreat" || d.Action == "escape" {
		t.Errorf("a dueled monster shouldn't flee: %+v", d)
	}
}

func TestResistAndDecoys(t *testing.T) {
	cs := boardWith(goblins(1))
	me := &cs.Party[0]
	me.CombatState.SpellBuffs = []types.SpellBuff{{Name: "Blade Ward", Resist: []string{"slashing"}}, {Name: "Mirror Image", Decoys: 1}}
	if d, _ := resistDamage(&me.CombatState, "slashing", 9); d != 4 {
		t.Errorf("resisted 9 → %d, want 4", d)
	}
	if d, _ := resistDamage(&me.CombatState, "fire", 9); d != 9 {
		t.Errorf("unresisted fire → %d, want 9", d)
	}
	if ok, _ := deflectHit(cs, me, nil, &cs.Monsters[0], AttackResult{IsHit: true, Total: 30}, 12); !ok {
		t.Error("a double should take the hit")
	}
	for _, b := range me.CombatState.SpellBuffs {
		if b.Decoys > 0 {
			t.Error("the double should be used up")
		}
	}
}

func TestHeldFastMonsterDoesntMove(t *testing.T) {
	cs := boardWith(goblins(1))
	m := &cs.Monsters[0]
	m.Pos = types.Position{X: 7, Y: 3}
	m.Conditions = []types.CombatCondition{{Name: "restrained"}}
	ApplyMonsterMove(cs, m, MonsterDecision{Move: -1, TargetRange: 1, Action: "none"}, 1, nil)
	if m.Pos != (types.Position{X: 7, Y: 3}) {
		t.Errorf("restrained monster moved to %+v", m.Pos)
	}
}

func TestMeleeOverrideOnlyForListedWeapons(t *testing.T) {
	st := &types.PlayerCombatState{SpellBuffs: []types.SpellBuff{{Name: "Shillelagh", MeleeStat: "spellcasting", WeaponDice: "1d8", Weapons: []string{"club"}}}}
	save := &types.SaveFile{Class: "Druid"}
	stats := map[string]interface{}{"Strength": 8, "Wisdom": 16}
	item, s, _ := meleeOverride(st, save, map[string]interface{}{"damage": "1d4"}, stats, "club")
	if item["damage"] != "1d8" || GetStatFromMap(s, "strength") != 16 {
		t.Errorf("club: damage %v strength %d", item["damage"], GetStatFromMap(s, "strength"))
	}
	item, s, _ = meleeOverride(st, save, map[string]interface{}{"damage": "1d6"}, stats, "scimitar")
	if item["damage"] != "1d6" || GetStatFromMap(s, "strength") != 8 {
		t.Error("a weapon not on the list shouldn't change")
	}
}

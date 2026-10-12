package combat

import (
	"testing"
	"time"

	"pubkey-quest/types"
)

// partyBoard is a 9×7 board with two players (a, b) and the given monsters.
func partyBoard(monsters []types.MonsterInstance) *types.CombatSession {
	cs := boardWith(monsters)
	cs.Party[0].ID, cs.Party[0].Name = "a", "Ann"
	cs.Party = append(cs.Party, types.PartyCombatant{
		Type: "player", ID: "b", Name: "Bo",
		Pos:         types.Position{X: 1, Y: 4},
		CombatState: types.PlayerCombatState{CurrentHP: 10, MaxHP: 10},
	})
	return cs
}

func TestMonstersFollowThreat(t *testing.T) {
	ms := goblins(1)
	cs := partyBoard(ms)
	cs.Monsters[0].Pos = types.Position{X: 2, Y: 3} // next to Ann
	cs.Monsters[0].Threat = map[string]int{"b": 6}
	if got := aiTarget(cs, &cs.Monsters[0]); got.ID != "b" {
		t.Errorf("target = %s, want b (most threat)", got.ID)
	}
	cs.Monsters[0].Threat = nil
	if got := aiTarget(cs, &cs.Monsters[0]); got.ID != "a" {
		t.Errorf("no threat: target = %s, want the nearest (a)", got.ID)
	}
}

func TestPartyMemberDownedThenWipe(t *testing.T) {
	cs := partyBoard(goblins(1))
	cs.Actor = 0
	log := applyDamageToPlayer(cs, 50)
	if cs.Phase != "active" || !cs.Party[0].CombatState.IsUnconscious {
		t.Fatalf("one down: phase %q, unconscious %v (%v)", cs.Phase, cs.Party[0].CombatState.IsUnconscious, log)
	}
	if got := aiTarget(cs, &cs.Monsters[0]); got.ID != "b" {
		t.Errorf("monsters should leave the downed alone, targeted %s", got.ID)
	}
	cs.Actor = 1
	applyDamageToPlayer(cs, 50)
	if cs.Phase != "defeat" {
		t.Errorf("everyone down should be a wipe, phase %q", cs.Phase)
	}
}

func TestHelpRevivesAndHealingPaysXP(t *testing.T) {
	ms := goblins(1)
	ms[0].Data.XP = 50
	ms[0].MaxHP = 10
	cs := partyBoard(ms)
	cs.Actor = 1
	applyDamageToPlayer(cs, 50) // Bo goes down
	cs.Actor = 0
	if _, err := ProcessHelp(cs, "b"); err != nil {
		t.Fatal(err)
	}
	if cs.Party[1].CombatState.IsUnconscious || cs.Party[1].CombatState.CurrentHP != 1 {
		t.Fatalf("help should revive at 1 HP: %+v", cs.Party[1].CombatState)
	}
	// Healing Bo's lost HP pays Ann XP; overheal doesn't.
	healed, _ := HealMember(cs, &cs.Party[0], &cs.Party[1], 100, 600, 1, nil)
	if healed != 9 {
		t.Errorf("healed %d, want 9 (to max)", healed)
	}
	if cs.Party[0].XPEarned <= 0 {
		t.Error("healer earned no XP")
	}
}

func TestFlankingNeedsAnAllyOpposite(t *testing.T) {
	cs := partyBoard(goblins(1))
	g := &cs.Monsters[0]
	g.Pos = types.Position{X: 3, Y: 3}
	cs.Party[0].Pos = types.Position{X: 2, Y: 3}
	cs.Party[1].Pos = types.Position{X: 3, Y: 2} // beside, not opposite
	if flanking(cs, &cs.Party[0], g) {
		t.Error("an ally at a right angle isn't flanking")
	}
	cs.Party[1].Pos = types.Position{X: 4, Y: 3}
	if !flanking(cs, &cs.Party[0], g) {
		t.Error("an ally directly opposite should flank")
	}
}

func TestPartyLootSplitsGold(t *testing.T) {
	ms := goblins(1)
	ms[0].Data.LootTable = types.LootTable{
		Guaranteed: []types.LootGuaranteed{{Item: "gold-piece", Quantity: [2]int{10, 10}}, {Item: "dagger", Quantity: [2]int{1, 1}}},
	}
	ms[0].Threat = map[string]int{"b": 5}
	cs := partyBoard(ms)
	members := inFightMembers(cs)
	distributePartyLoot(cs, &cs.Monsters[0], members)
	gold := func(p *types.PartyCombatant) int {
		n := 0
		for _, d := range p.Loot {
			if d.Item == "gold-piece" {
				n += d.Quantity
			}
		}
		return n
	}
	if gold(&cs.Party[0]) != 5 || gold(&cs.Party[1]) != 5 {
		t.Errorf("gold split %d/%d, want 5/5", gold(&cs.Party[0]), gold(&cs.Party[1]))
	}
	hasDagger := false
	for _, d := range cs.Party[1].Loot {
		hasDagger = hasDagger || d.Item == "dagger"
	}
	if !hasDagger {
		t.Error("the guaranteed dagger should go to the top contributor (b)")
	}
}

func TestTurnPassesToTheNextStandingPlayer(t *testing.T) {
	cs := partyBoard(goblins(1))
	cs.Monsters[0].Pos = types.Position{X: 7, Y: 0}
	cs.Initiative = []types.InitiativeEntry{{ID: "a", Type: "player"}, {ID: "b", Type: "player"}, {ID: cs.Monsters[0].InstanceID, Type: "monster"}}
	cs.CurrentTurnIndex = 0
	cs.Actor = 0
	save := &types.SaveFile{Race: "Human"}
	runUntilPlayerTurn(nil, cs, save, false)
	if !IsMemberTurn(cs, "b") || cs.Actor != 1 {
		t.Fatalf("after Ann's turn it should be Bo's; turn %+v actor %d", CurrentMember(cs), cs.Actor)
	}
	if cs.TurnDeadline == 0 {
		t.Error("a party turn should start the timer")
	}
	// Bo's down: his turn passes, the goblin goes, and it's Ann's again.
	cs.Party[1].CombatState.IsUnconscious = true
	runUntilPlayerTurn(nil, cs, save, false)
	if !IsMemberTurn(cs, "a") {
		t.Errorf("a downed member's turn should pass; now %+v", CurrentMember(cs))
	}
}

func TestExpiredTurnDodges(t *testing.T) {
	cs := partyBoard(goblins(1))
	cs.Monsters[0].Pos = types.Position{X: 7, Y: 0}
	cs.Initiative = []types.InitiativeEntry{{ID: "a", Type: "player"}, {ID: "b", Type: "player"}, {ID: cs.Monsters[0].InstanceID, Type: "monster"}}
	cs.Phase = "active"
	cs.CurrentTurnIndex = 0
	cs.TurnDeadline = time.Now().Add(-time.Second).UnixMilli()
	prev := MemberSave
	MemberSave = func(*types.PartyCombatant) *types.SaveFile { return &types.SaveFile{Race: "Human"} }
	defer func() { MemberSave = prev }()
	log := ExpireTurn(nil, cs, time.Now())
	if len(log) == 0 || !cs.Party[0].CombatState.Dodging || !IsMemberTurn(cs, "b") {
		t.Errorf("Ann should Dodge and play pass to Bo: log %v, turn %+v", log, CurrentMember(cs))
	}
}

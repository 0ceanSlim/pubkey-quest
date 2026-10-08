package combat

import (
	"testing"

	"pubkey-quest/types"
)

// Unit tests for the M5.6 P1 multi-entity mechanics that need no database:
// instance ids, placement, initiative order, targeting, occupancy-aware AI
// movement and the victory rule.

func goblinData() types.MonsterData {
	return types.MonsterData{
		ID: "goblin", Name: "Goblin", HitPoints: 7,
		Speed:   types.MonsterSpeed{Walk: 30},
		Stats:   types.MonsterStats{Dexterity: 14},
		Actions: []types.MonsterAction{{Name: "Scimitar", Type: "melee_attack", AttackBonus: 4}},
	}
}

func goblins(n int) []types.MonsterInstance {
	out := make([]types.MonsterInstance, n)
	for i := range out {
		d := goblinData()
		out[i] = types.MonsterInstance{TemplateID: d.ID, InstanceID: d.ID, Name: d.Name, IsAlive: true, CurrentHP: 7, MaxHP: 7, Data: d}
	}
	return out
}

func boardWith(monsters []types.MonsterInstance) *types.CombatSession {
	return &types.CombatSession{
		Phase:      "active",
		GridWidth:  combatGridWidth,
		GridHeight: combatGridHeight,
		Party: []types.PartyCombatant{{
			Type: "player", ID: "npub_me",
			Pos:         types.Position{X: 1, Y: combatGridHeight / 2},
			CombatState: types.PlayerCombatState{CurrentHP: 10, MaxHP: 10},
		}},
		Monsters: monsters,
	}
}

func TestAssignInstanceIDs(t *testing.T) {
	// A lone monster keeps its template id and name.
	one := goblins(1)
	assignInstanceIDs(one)
	if one[0].InstanceID != "goblin" || one[0].Name != "Goblin" {
		t.Errorf("single monster: got %q/%q, want goblin/Goblin", one[0].InstanceID, one[0].Name)
	}

	// Repeats are numbered; a different template in the mix is left alone.
	mixed := append(goblins(2), types.MonsterInstance{TemplateID: "wolf", Name: "Wolf", Data: types.MonsterData{ID: "wolf", Name: "Wolf"}})
	assignInstanceIDs(mixed)
	want := []struct{ id, name string }{{"goblin#1", "Goblin 1"}, {"goblin#2", "Goblin 2"}, {"wolf", "Wolf"}}
	for i, w := range want {
		if mixed[i].InstanceID != w.id || mixed[i].Name != w.name {
			t.Errorf("monster %d: got %q/%q, want %q/%q", i, mixed[i].InstanceID, mixed[i].Name, w.id, w.name)
		}
	}
}

func TestPlaceMonstersNoOverlap(t *testing.T) {
	for _, n := range []int{1, 3, maxEncounterMonsters} {
		ms := goblins(n)
		assignInstanceIDs(ms)
		cs := boardWith(ms)
		placeMonsters(cs, 0) // range 0 must still spawn adjacent, never on the player
		seen := map[types.Position]bool{playerPos(cs): true}
		for _, m := range cs.Monsters {
			if !inGrid(cs, m.Pos) {
				t.Errorf("n=%d: %s placed off the grid at %+v", n, m.InstanceID, m.Pos)
			}
			if seen[m.Pos] {
				t.Errorf("n=%d: %s shares cell %+v", n, m.InstanceID, m.Pos)
			}
			seen[m.Pos] = true
			if r := chebyshev(m.Pos, playerPos(cs)); r < 1 {
				t.Errorf("n=%d: %s spawned at range %d", n, m.InstanceID, r)
			}
		}
	}
}

func TestSortInitiative(t *testing.T) {
	entries := func() []types.InitiativeEntry {
		return []types.InitiativeEntry{
			{ID: "me", Type: "player", Initiative: 18, DEXScore: 12},
			{ID: "a", Type: "monster", Initiative: 10, DEXScore: 14},
			{ID: "b", Type: "monster", Initiative: 18, DEXScore: 16}, // ties the player, higher DEX
		}
	}
	got := sortInitiative(entries(), false)
	if got[0].ID != "b" || got[1].ID != "me" || got[2].ID != "a" {
		t.Errorf("normal order = %v, want b, me, a", ids(got))
	}

	// Surprised: every monster goes before the party, keeping their own order.
	got = sortInitiative(entries(), true)
	if got[0].ID != "b" || got[1].ID != "a" || got[2].ID != "me" {
		t.Errorf("surprise order = %v, want b, a, me", ids(got))
	}
}

func ids(es []types.InitiativeEntry) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}

func TestResolveTarget(t *testing.T) {
	ms := goblins(3)
	assignInstanceIDs(ms)
	cs := boardWith(ms)
	cs.Monsters[0].Pos = types.Position{X: 5, Y: 3}
	cs.Monsters[1].Pos = types.Position{X: 3, Y: 3} // nearest
	cs.Monsters[2].Pos = types.Position{X: 4, Y: 1}

	if m, err := ResolveTarget(cs, ""); err != nil || m.InstanceID != "goblin#2" {
		t.Errorf("default target = %v (%v), want the nearest goblin#2", m, err)
	}
	if m, err := ResolveTarget(cs, "goblin#3"); err != nil || m.InstanceID != "goblin#3" {
		t.Errorf("explicit target = %v (%v), want goblin#3", m, err)
	}

	cs.Monsters[1].IsAlive = false
	if _, err := ResolveTarget(cs, "goblin#2"); err == nil {
		t.Error("targeting a dead monster should fail")
	}
	if m, _ := ResolveTarget(cs, ""); m == nil || m.InstanceID == "goblin#2" {
		t.Errorf("default target should skip the dead one, got %v", m)
	}
	if _, err := ResolveTarget(cs, "dragon"); err == nil {
		t.Error("targeting an unknown id should fail")
	}
}

func TestStepMonsterRoutesAroundAllies(t *testing.T) {
	ms := goblins(2)
	assignInstanceIDs(ms)
	cs := boardWith(ms)
	// goblin#1 already adjacent to the player; goblin#2 directly behind it.
	cs.Monsters[0].Pos = types.Position{X: 2, Y: 3}
	cs.Monsters[1].Pos = types.Position{X: 3, Y: 3}

	next, ok := stepMonster(cs, &cs.Monsters[1], playerPos(cs), -1)
	if !ok {
		t.Fatal("expected a step around the blocking ally")
	}
	if next == cs.Monsters[0].Pos || next == playerPos(cs) {
		t.Errorf("stepped into an occupied cell %+v", next)
	}
	if chebyshev(next, playerPos(cs)) != 1 {
		t.Errorf("expected to reach adjacency around the ally, landed at %+v", next)
	}

	// Full move: both end up adjacent, on distinct cells.
	d := DecideMonsterAction(cs, &cs.Monsters[1])
	ApplyMonsterMove(cs, &cs.Monsters[1], d, 0, nil)
	if cs.Monsters[1].Pos == cs.Monsters[0].Pos {
		t.Error("two monsters ended on the same cell")
	}
}

func TestCheckVictoryNeedsEveryEnemyDown(t *testing.T) {
	ms := goblins(2)
	assignInstanceIDs(ms)
	cs := boardWith(ms)

	cs.Monsters[0].IsAlive = false
	if checkVictory(cs) != nil || cs.Phase != "active" {
		t.Fatalf("one of two dead should not end the fight (phase %q)", cs.Phase)
	}

	cs.Monsters[1].Fled = true
	if checkVictory(cs) == nil || cs.Phase != "loot" {
		t.Errorf("last enemy fled → fight should end in loot, phase %q", cs.Phase)
	}
}

func TestAdvanceCursorCountsRounds(t *testing.T) {
	cs := boardWith(nil)
	cs.Round = 1
	cs.Initiative = []types.InitiativeEntry{{Type: "monster"}, {Type: "player"}, {Type: "monster"}}
	cs.CurrentTurnIndex = 1
	advanceCursor(cs) // → 2
	advanceCursor(cs) // wrap → 0, round 2
	if cs.CurrentTurnIndex != 0 || cs.Round != 2 {
		t.Errorf("after wrap: index %d round %d, want 0 / 2", cs.CurrentTurnIndex, cs.Round)
	}
	if playerTurnIndex(cs) != 1 {
		t.Errorf("playerTurnIndex = %d, want 1", playerTurnIndex(cs))
	}
}

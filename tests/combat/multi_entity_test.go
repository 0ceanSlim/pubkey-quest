package combat_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/types"
)

// DB-backed tests for multi-monster encounters (M5.6 P1): a real StartEncounter
// against migrated goblin stat blocks, the initiative loop, and the
// fight-ends-only-when-everyone-is-down rule.

func startGoblinPack(t *testing.T, n int, surprise bool) (*types.CombatSession, *types.SaveFile) {
	t.Helper()
	combatSetup(t)
	adv, err := character.LoadAdvancement(db.GetDB())
	if err != nil {
		t.Fatalf("load advancement: %v", err)
	}
	save := fighterSave()
	save.HP, save.MaxHP = 500, 500 // survive the pack while we inspect the loop
	spec := combat.EncounterSpec{MonsterIDs: combat.MonsterIDsForCount("goblin", n), EnvironmentID: "forest", Surprise: surprise}
	cs, err := combat.StartEncounter(db.GetDB(), save, "npub_test", spec, adv)
	if err != nil {
		t.Fatalf("StartEncounter: %v", err)
	}
	return cs, save
}

func assertNoSharedCells(t *testing.T, cs *types.CombatSession) {
	t.Helper()
	seen := map[types.Position]string{combat.PlayerPos(cs): "player"}
	for _, m := range cs.Monsters {
		if !m.Active() {
			continue
		}
		if who, ok := seen[m.Pos]; ok {
			t.Errorf("%s and %s share cell %+v", m.InstanceID, who, m.Pos)
		}
		seen[m.Pos] = m.InstanceID
	}
}

func TestStartEncounterBuildsAPack(t *testing.T) {
	cs, _ := startGoblinPack(t, 3, false)

	if len(cs.Monsters) != 3 {
		t.Fatalf("got %d monsters, want 3", len(cs.Monsters))
	}
	if len(cs.Initiative) != 4 {
		t.Errorf("initiative has %d entries, want 4 (player + 3)", len(cs.Initiative))
	}
	ids := map[string]bool{}
	for _, m := range cs.Monsters {
		if ids[m.InstanceID] {
			t.Errorf("duplicate instance id %q", m.InstanceID)
		}
		ids[m.InstanceID] = true
	}
	assertNoSharedCells(t, cs)
	if cs.Phase == "active" && !combat.IsPlayerTurn(cs) {
		t.Error("a fresh encounter should be parked on the player's turn")
	}
	if cs.Difficulty == "" {
		t.Error("expected the group to be rated")
	}
}

func TestSurprisedPartyActsLast(t *testing.T) {
	cs, _ := startGoblinPack(t, 2, true)
	if !cs.IsSurprised {
		t.Fatal("expected IsSurprised")
	}
	if last := cs.Initiative[len(cs.Initiative)-1]; last.Type != "player" {
		t.Errorf("surprised player should be last in the order, got %v", cs.Initiative)
	}
}

func TestEndTurnRunsEveryMonsterAndAdvancesARound(t *testing.T) {
	cs, save := startGoblinPack(t, 3, false)
	if cs.Phase != "active" {
		t.Skipf("opening turns ended the fight (phase %q)", cs.Phase)
	}
	round := cs.Round
	if _, err := combat.ProcessEndTurn(db.GetDB(), cs, save); err != nil {
		t.Fatalf("ProcessEndTurn: %v", err)
	}
	if cs.Phase != "active" {
		return // the pack dropped us — the loop still ran; nothing more to assert
	}
	if !combat.IsPlayerTurn(cs) {
		t.Error("after end-turn the cursor should be back on the player")
	}
	if cs.Round != round+1 {
		t.Errorf("round = %d, want %d (one full pass of the order)", cs.Round, round+1)
	}
	assertNoSharedCells(t, cs)
}

func TestFightEndsOnlyWhenEveryEnemyIsDown(t *testing.T) {
	cs, save := startGoblinPack(t, 2, false)
	if cs.Phase != "active" {
		t.Skipf("opening turns ended the fight (phase %q)", cs.Phase)
	}
	adv, _ := character.LoadAdvancement(db.GetDB())

	// Pin the board: both goblins adjacent, one hit from death, trivially hittable.
	me := combat.PlayerPos(cs)
	for i := range cs.Monsters {
		m := &cs.Monsters[i]
		m.Pos = types.Position{X: me.X + 1, Y: me.Y - 1 + 2*i}
		m.CurrentHP, m.ArmorClass = 1, 1
	}

	kill := func(target string) {
		t.Helper()
		for tries := 0; tries < 50; tries++ {
			if m := combat.FindMonster(cs, target); m == nil || !m.IsAlive {
				return
			}
			cs.Party[0].CombatState.ActionUsed = false
			if _, err := combat.ProcessPlayerAttack(db.GetDB(), cs, save, target, "unarmed", "main", false, adv); err != nil {
				t.Fatalf("attack %s: %v", target, err)
			}
		}
		t.Fatalf("could not kill %s in 50 swings", target)
	}

	first, second := cs.Monsters[0].InstanceID, cs.Monsters[1].InstanceID
	kill(first)
	if cs.Phase != "active" {
		t.Fatalf("killing one of two ended the fight (phase %q)", cs.Phase)
	}
	if _, err := combat.ProcessPlayerAttack(db.GetDB(), cs, save, first, "unarmed", "main", false, adv); err == nil {
		t.Error("attacking a dead goblin should be rejected")
	}
	kill(second)
	if cs.Phase != "loot" {
		t.Errorf("both goblins dead → phase %q, want loot", cs.Phase)
	}
}

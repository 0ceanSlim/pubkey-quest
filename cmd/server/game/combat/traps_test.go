package combat

import (
	"testing"

	"pubkey-quest/types"
)

func TestScatterCaltropsTowardFoe(t *testing.T) {
	loadTestTerrain(t)
	ms := goblinsAt(types.Position{X: 6, Y: 3})
	cs := boardWith(ms) // player at (1,3)
	slot := map[string]interface{}{"item": "caltrops", "quantity": 2}
	scatter := map[string]interface{}{"terrain": "caltrops", "cells": float64(3)}
	if _, err := scatterItem(cs, nil, scatter, "Caltrops", slot, false, ""); err != nil {
		t.Fatal(err)
	}
	n := 0
	for i, id := range cs.Terrain {
		if id != "caltrops" {
			continue
		}
		n++
		if x := i % cs.GridWidth; x != 3 {
			t.Errorf("caltrops should land on the far edge toward the goblin (x=3), got x=%d", x)
		}
	}
	if n != 3 {
		t.Errorf("want 3 caltrop squares, got %d", n)
	}
	if slot["quantity"] != 1 || !cs.Party[0].CombatState.ActionUsed {
		t.Errorf("scattering uses one item and the action: quantity %v, action %v", slot["quantity"], cs.Party[0].CombatState.ActionUsed)
	}
}

func TestCaltropsStopThePlayer(t *testing.T) {
	cfg := loadTestTerrain(t)
	cfg.Types["caltrops"].Trap.DC = 100 // always fail
	cs := boardWith(goblinsAt(types.Position{X: 8, Y: 0}))
	cs.Terrain = make([]string, cs.GridWidth*cs.GridHeight)
	// A wall of caltrops at x=2 (no way round), the player at (1,3).
	for y := 0; y < cs.GridHeight; y++ {
		cs.Terrain[y*cs.GridWidth+2] = "caltrops"
	}
	cs.Party[0].CombatState.MovementBudget = 6
	if _, err := ProcessPlayerMove(nil, cs, &types.SaveFile{}, 4, 3); err != nil {
		t.Fatal(err)
	}
	me := cs.Party[0]
	if me.Pos.X != 2 {
		t.Errorf("a failed save should stop you on the caltrops at x=2; got %v", me.Pos)
	}
	if me.CombatState.MovementSpent != me.CombatState.MovementBudget {
		t.Errorf("the rest of your movement is lost: spent %d of %d", me.CombatState.MovementSpent, me.CombatState.MovementBudget)
	}
	if me.CombatState.CurrentHP != 9 {
		t.Errorf("caltrops deal 1 piercing: HP %d, want 9", me.CombatState.CurrentHP)
	}
}

func TestBallBearingsTripAMonster(t *testing.T) {
	cfg := loadTestTerrain(t)
	cfg.Types["ball_bearings"].Trap.DC = 100
	ms := goblinsAt(types.Position{X: 7, Y: 3})
	cs := boardWith(ms)
	cs.Terrain = make([]string, cs.GridWidth*cs.GridHeight)
	for y := 0; y < cs.GridHeight; y++ {
		cs.Terrain[y*cs.GridWidth+5] = "ball_bearings"
	}
	g := &cs.Monsters[0]
	ApplyMonsterMove(cs, g, MonsterDecision{Move: -1, TargetRange: 1, Action: "none"}, 0, nil)
	if g.Pos.X != 5 {
		t.Errorf("the goblin should stop on the bearings at x=5, got %v", g.Pos)
	}
	if !HasCondition(g.Conditions, "prone") {
		t.Error("the goblin should be prone")
	}
}

func TestTrapsDontHurtWhoStandsThere(t *testing.T) {
	loadTestTerrain(t)
	cs := boardWith(goblins(1))
	cs.Terrain = make([]string, cs.GridWidth*cs.GridHeight)
	cs.Terrain[3*cs.GridWidth+1] = "caltrops" // under the player
	if log := endPlayerTurn(cs, &types.SaveFile{}); len(log) != 0 {
		t.Errorf("standing on caltrops does nothing: %v", log)
	}
}

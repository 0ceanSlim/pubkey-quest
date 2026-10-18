package combat

import (
	"strings"
	"testing"

	"pubkey-quest/cmd/server/game/spells"
	"pubkey-quest/types"
)

// Area-of-effect spells: who an area catches, and what one cast does to
// everyone it caught. The caster stands at (1,3) on the 9×7 test board.

// goblinsAt numbers n goblins and stands them on the given cells.
func goblinsAt(cells ...types.Position) []types.MonsterInstance {
	ms := goblins(len(cells))
	assignInstanceIDs(ms)
	for i := range ms {
		ms[i].Pos = cells[i]
	}
	return ms
}

func caughtNames(caught []*types.MonsterInstance) string {
	var names []string
	for _, m := range caught {
		names = append(names, m.Name)
	}
	return strings.Join(names, ",")
}

func TestSpellAreaOfReadsSpellJSON(t *testing.T) {
	a := spellAreaOf(map[string]interface{}{"aoe": map[string]interface{}{"shape": "sphere", "radius": float64(4)}})
	if a == nil || a.Shape != "sphere" || a.Radius != 4 {
		t.Fatalf("parsed %+v", a)
	}
	if spellAreaOf(map[string]interface{}{}) != nil {
		t.Error("no aoe should parse to nil")
	}
}

func TestSphereCatchesEveryoneInRadius(t *testing.T) {
	cs := boardWith(goblinsAt(
		types.Position{X: 6, Y: 4}, // in: diagonal to the aim point
		types.Position{X: 5, Y: 3}, // the aim point
		types.Position{X: 4, Y: 2}, // in
		types.Position{X: 7, Y: 3}, // out: 2 cells from the aim point
	))
	caught := areaTargets(cs, &cs.Party[0], &cs.Monsters[1], &types.SpellArea{Shape: "sphere", Radius: 1})
	if got := caughtNames(caught); got != "Goblin 2,Goblin 1,Goblin 3" {
		t.Errorf("caught %s, want the aimed goblin first then the two beside it", got)
	}
}

func TestSelfRadiusHitsWhoeverIsNear(t *testing.T) {
	cs := partyBoard(goblinsAt(
		types.Position{X: 4, Y: 3}, // out of reach
		types.Position{X: 2, Y: 3},
		types.Position{X: 2, Y: 2},
	))
	spell := map[string]interface{}{"aoe": map[string]interface{}{"shape": "self_radius", "radius": float64(1)}}
	// Aimed at the far goblin, it still bursts around the caster.
	primary, caught, err := aimArea(cs, &cs.Party[0], spell, "Word of Radiance", &cs.Monsters[0])
	if err != nil {
		t.Fatal(err)
	}
	if got := caughtNames(caught); got != "Goblin 2,Goblin 3" || primary != caught[0] {
		t.Errorf("caught %s (primary %s), want Goblin 2,Goblin 3", got, primary.Name)
	}
	// Nobody in reach: refused before it costs anything.
	cs.Monsters[1].Pos, cs.Monsters[2].Pos = types.Position{X: 5, Y: 1}, types.Position{X: 5, Y: 5}
	if _, _, err := aimArea(cs, &cs.Party[0], spell, "Word of Radiance", &cs.Monsters[0]); err == nil {
		t.Error("expected a refusal with no enemy in reach")
	}
}

func TestConeWidensAwayFromTheCaster(t *testing.T) {
	cs := boardWith(goblinsAt(
		types.Position{X: 2, Y: 3}, // aimed at: in
		types.Position{X: 3, Y: 4}, // in: 2 out, 1 off the axis
		types.Position{X: 4, Y: 2}, // in: 3 out, 1 off
		types.Position{X: 2, Y: 4}, // out: 1 out is only the axis cell
		types.Position{X: 0, Y: 3}, // out: behind the caster
		types.Position{X: 5, Y: 3}, // out: past the cone's 3 cells
	))
	area := &types.SpellArea{Shape: "cone", Radius: 3}
	if got := caughtNames(areaTargets(cs, &cs.Party[0], &cs.Monsters[0], area)); got != "Goblin 1,Goblin 2,Goblin 3" {
		t.Errorf("cone caught %s", got)
	}
	// Aimed at a goblin past its reach: refused.
	spell := map[string]interface{}{"aoe": map[string]interface{}{"shape": "cone", "radius": float64(3)}}
	if _, _, err := aimArea(cs, &cs.Party[0], spell, "Burning Hands", &cs.Monsters[5]); err == nil {
		t.Error("expected a refusal for a target beyond the cone")
	}
}

func TestLineRunsThroughTheTarget(t *testing.T) {
	cs := boardWith(goblinsAt(
		types.Position{X: 3, Y: 3}, // aimed at
		types.Position{X: 7, Y: 3}, // in: further down the line
		types.Position{X: 4, Y: 4}, // out: beside it
	))
	caught := areaTargets(cs, &cs.Party[0], &cs.Monsters[0], &types.SpellArea{Shape: "line", Radius: 20})
	if got := caughtNames(caught); got != "Goblin 1,Goblin 2" {
		t.Errorf("line caught %s", got)
	}
}

func TestWallSheltersFromTheBlast(t *testing.T) {
	loadTestTerrain(t)
	cs := arenaBoard(t,
		".........",
		".........",
		".........",
		"......#..", // a wall between the aim point and (7,3)
		".........",
		".........",
		".........",
	)
	cs.Monsters = goblinsAt(types.Position{X: 5, Y: 3}, types.Position{X: 7, Y: 3}, types.Position{X: 5, Y: 4})
	caught := areaTargets(cs, &cs.Party[0], &cs.Monsters[0], &types.SpellArea{Shape: "sphere", Radius: 2})
	if got := caughtNames(caught); got != "Goblin 1,Goblin 3" {
		t.Errorf("caught %s, want the goblin behind the wall spared", got)
	}
}

// areaCast is a resolved area save spell (damage plus a blinding rider) with
// the given outcome for each monster on the board, in order.
func areaCast(cs *types.CombatSession, outcomes ...spells.TargetOutcome) *spells.CastResult {
	for i := range outcomes {
		outcomes[i].Monster = &cs.Monsters[i]
	}
	return &spells.CastResult{
		SpellID: "flash", SpellName: "Flash", Shape: "save", SaveDC: 13, DamageType: "radiant",
		Damage: outcomes[0].Damage, SaveMade: outcomes[0].SaveMade, Targets: outcomes,
		Spell: map[string]interface{}{
			"save_type":         "constitution",
			"aoe":               map[string]interface{}{"shape": "self_radius", "radius": float64(2)},
			"applies_condition": map[string]interface{}{"condition": "blinded", "rounds": float64(1)},
		},
	}
}

func TestAreaCastHitsEveryoneCaught(t *testing.T) {
	ms := goblinsAt(types.Position{X: 2, Y: 3}, types.Position{X: 2, Y: 2}, types.Position{X: 3, Y: 4}, types.Position{X: 8, Y: 0})
	for i := range ms {
		ms[i].Data.XP = 70 // 10 XP per HP
	}
	cs := boardWith(ms)
	res := areaCast(cs,
		spells.TargetOutcome{Damage: 9},                 // failed: dies
		spells.TargetOutcome{Damage: 4},                 // failed: hurt and blinded
		spells.TargetOutcome{Damage: 2, SaveMade: true}, // saved: half, no rider
	)
	log := castConsequences(cs, &cs.Party[0], &types.SaveFile{TimeOfDay: 720}, res, nil, &cs.Monsters[0], 1, nil)
	text := strings.Join(log, "\n")

	g1, g2, g3, g4 := &cs.Monsters[0], &cs.Monsters[1], &cs.Monsters[2], &cs.Monsters[3]
	if g1.IsAlive || !strings.Contains(text, "Goblin 1 is defeated!") {
		t.Errorf("Goblin 1 should die: alive %v\n%s", g1.IsAlive, text)
	}
	if g2.CurrentHP != 3 || !HasCondition(g2.Conditions, "blinded") {
		t.Errorf("Goblin 2: HP %d, conditions %v, want 3 and blinded", g2.CurrentHP, g2.Conditions)
	}
	if g3.CurrentHP != 5 || HasCondition(g3.Conditions, "blinded") {
		t.Errorf("Goblin 3: HP %d, conditions %v, want 5 and no rider (it saved)", g3.CurrentHP, g3.Conditions)
	}
	if g4.CurrentHP != 7 {
		t.Errorf("Goblin 4 wasn't caught but took damage: HP %d", g4.CurrentHP)
	}
	if HasCondition(g1.Conditions, "blinded") {
		t.Error("the dead goblin shouldn't be blinded")
	}
	// Damage XP for all 15 HP dealt, reported in one line.
	if n := strings.Count(text, "XP"); n != 1 || cs.Party[0].XPEarned != 150 {
		t.Errorf("want one XP line and 150 XP, got %d lines, %d XP\n%s", n, cs.Party[0].XPEarned, text)
	}
	if cs.Phase != "active" {
		t.Errorf("phase %q: a goblin is still up", cs.Phase)
	}
}

func TestAreaCastKillingTheLastFoesWinsOnce(t *testing.T) {
	cs := boardWith(goblinsAt(types.Position{X: 2, Y: 3}, types.Position{X: 2, Y: 2}))
	res := areaCast(cs, spells.TargetOutcome{Damage: 10}, spells.TargetOutcome{Damage: 3, SaveMade: true})
	res.Targets[1].Damage = 8
	log := castConsequences(cs, &cs.Party[0], &types.SaveFile{}, res, nil, &cs.Monsters[0], 1, nil)
	text := strings.Join(log, "\n")
	if cs.Phase != "loot" || strings.Count(text, "Victory") != 1 {
		t.Fatalf("phase %q, want loot with one victory line:\n%s", cs.Phase, text)
	}
	if v, d2 := strings.Index(text, "Victory"), strings.Index(text, "Goblin 2 is defeated!"); d2 < 0 || v < d2 {
		t.Errorf("victory should follow both kills:\n%s", text)
	}
}

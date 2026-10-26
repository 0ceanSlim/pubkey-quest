package combat

import (
	"strings"
	"testing"

	"pubkey-quest/types"
)

// A push moves the target straight away from the caster, square by square,
// and stops early at the edge of the field or another creature.
func TestPushMovesAwayAndStops(t *testing.T) {
	cs := &types.CombatSession{GridWidth: 10, GridHeight: 10}
	cs.Monsters = []types.MonsterInstance{
		{InstanceID: "a", Name: "Goblin", IsAlive: true, Pos: types.Position{X: 5, Y: 5}},
		{InstanceID: "b", Name: "Wolf", IsAlive: true, Pos: types.Position{X: 8, Y: 8}},
	}
	caster := types.Position{X: 4, Y: 5}

	line := pushMonster(cs, caster, &cs.Monsters[0], 2)
	if got := cs.Monsters[0].Pos; got != (types.Position{X: 7, Y: 5}) || !strings.Contains(line, "10 ft") {
		t.Errorf("pushed to %+v (%q), want 7,5 and 10 ft", got, line)
	}

	// Diagonally into the board's corner: only one square of room.
	cs.Monsters[0].Pos = types.Position{X: 8, Y: 8}
	cs.Monsters[1].Pos = types.Position{X: 0, Y: 0}
	pushMonster(cs, types.Position{X: 7, Y: 7}, &cs.Monsters[0], 2)
	if got := cs.Monsters[0].Pos; got != (types.Position{X: 9, Y: 9}) {
		t.Errorf("into the corner: %+v, want 9,9", got)
	}

	// Another creature right behind: nowhere to go.
	cs.Monsters[0].Pos = types.Position{X: 5, Y: 5}
	cs.Monsters[1].Pos = types.Position{X: 6, Y: 5}
	if line := pushMonster(cs, caster, &cs.Monsters[0], 2); cs.Monsters[0].Pos.X != 5 || !strings.Contains(line, "nowhere") {
		t.Errorf("blocked push moved it to %+v (%q)", cs.Monsters[0].Pos, line)
	}
}

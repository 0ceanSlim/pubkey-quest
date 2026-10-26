package combat

import (
	"fmt"

	"pubkey-quest/types"
)

// Pushes: a spell with "push": N (squares, 5 ft each) shoves every creature
// that failed its save straight away from the caster, as Thunderwave's 10 feet
// (2 squares). It moves square by square and stops early at a wall, the edge of
// the field or another creature.

// pushOf is how many squares a spell pushes (0 for none).
func pushOf(spell map[string]interface{}) int {
	if n, ok := spell["push"].(float64); ok && n > 0 {
		return int(n)
	}
	return 0
}

// sign is -1, 0 or 1.
func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

// pushMonster moves m up to squares away from the origin (the caster) and
// returns the log line.
func pushMonster(cs *types.CombatSession, origin types.Position, m *types.MonsterInstance, squares int) string {
	step := types.Position{X: sign(m.Pos.X - origin.X), Y: sign(m.Pos.Y - origin.Y)}
	if step.X == 0 && step.Y == 0 {
		return ""
	}
	moved := 0
	for moved < squares {
		next := types.Position{X: m.Pos.X + step.X, Y: m.Pos.Y + step.Y}
		if !endable(cs, next, m.InstanceID) {
			break
		}
		m.Pos = next
		moved++
	}
	if moved == 0 {
		return fmt.Sprintf("  💨 %s is slammed back but has nowhere to go.", m.Name)
	}
	return fmt.Sprintf("  💨 %s is pushed back %d ft.", m.Name, moved*5)
}

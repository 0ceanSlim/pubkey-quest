package combat

import (
	"fmt"
	"math/rand"

	"pubkey-quest/types"
)

// ─── Grid, occupancy and targeting (M5.6 P1) ─────────────────────────────────
//
// Every combatant carries its own grid cell. Range is always measured between
// two specific combatants (Chebyshev: 1 = adjacent, 2+ = ranged); no two living
// combatants may share a cell. Dead or fled monsters drop off the grid.

// maxEncounterMonsters caps one encounter's monster count so a bad authored
// count can't flood the 9×7 grid.
const maxEncounterMonsters = 8

// noTargetRange is the range reported when there is nothing left to measure to.
const noTargetRange = 99

// playerPos returns the (single, P1) player's grid cell.
func playerPos(cs *types.CombatSession) types.Position {
	if len(cs.Party) == 0 {
		return types.Position{}
	}
	return cs.Party[0].Pos
}

// PlayerPos is the exported form of playerPos for the API layer.
func PlayerPos(cs *types.CombatSession) types.Position {
	return playerPos(cs)
}

// rangeTo is the range between the player and a monster.
func rangeTo(cs *types.CombatSession, m *types.MonsterInstance) int {
	return chebyshev(playerPos(cs), m.Pos)
}

// RangeTo is the exported form of rangeTo for the API layer.
func RangeTo(cs *types.CombatSession, m *types.MonsterInstance) int {
	return rangeTo(cs, m)
}

// activeMonsters returns the monsters still in the fight (alive, not fled).
func activeMonsters(cs *types.CombatSession) []*types.MonsterInstance {
	out := make([]*types.MonsterInstance, 0, len(cs.Monsters))
	for i := range cs.Monsters {
		if cs.Monsters[i].Active() {
			out = append(out, &cs.Monsters[i])
		}
	}
	return out
}

// FindMonster returns the monster with the given instance id, or nil.
func FindMonster(cs *types.CombatSession, instanceID string) *types.MonsterInstance {
	for i := range cs.Monsters {
		if cs.Monsters[i].InstanceID == instanceID {
			return &cs.Monsters[i]
		}
	}
	return nil
}

// nearestMonster returns the active monster closest to the player (ties go to
// the earlier one in the roster), or nil when none remain.
func nearestMonster(cs *types.CombatSession) *types.MonsterInstance {
	var best *types.MonsterInstance
	bestR := noTargetRange
	for _, m := range activeMonsters(cs) {
		if r := rangeTo(cs, m); r < bestR {
			best, bestR = m, r
		}
	}
	return best
}

// NearestMonster is the exported form of nearestMonster for the API layer.
func NearestMonster(cs *types.CombatSession) *types.MonsterInstance {
	return nearestMonster(cs)
}

// nearestRange is the player's range to the closest active monster.
func nearestRange(cs *types.CombatSession) int {
	if m := nearestMonster(cs); m != nil {
		return rangeTo(cs, m)
	}
	return noTargetRange
}

// ResolveTarget picks the monster an action is aimed at: the named instance when
// targetID is set (it must still be in the fight), otherwise the nearest one.
func ResolveTarget(cs *types.CombatSession, targetID string) (*types.MonsterInstance, error) {
	if targetID != "" {
		m := FindMonster(cs, targetID)
		if m == nil {
			return nil, fmt.Errorf("no such target %q", targetID)
		}
		if !m.Active() {
			return nil, fmt.Errorf("%s is no longer in the fight", m.Name)
		}
		return m, nil
	}
	if m := nearestMonster(cs); m != nil {
		return m, nil
	}
	return nil, fmt.Errorf("no living enemy to target")
}

// occupied reports whether a living combatant other than self stands on pos.
// self is the instance id / npub of the mover (its own cell never blocks it).
func occupied(cs *types.CombatSession, pos types.Position, self string) bool {
	for i := range cs.Party {
		p := &cs.Party[i]
		if p.ID != self && p.Pos == pos {
			return true
		}
	}
	for i := range cs.Monsters {
		m := &cs.Monsters[i]
		if m.InstanceID != self && m.Active() && m.Pos == pos {
			return true
		}
	}
	return false
}

// inGrid reports whether pos lies on the board.
func inGrid(cs *types.CombatSession, pos types.Position) bool {
	return pos.X >= 0 && pos.X < cs.GridWidth && pos.Y >= 0 && pos.Y < cs.GridHeight
}

// ClearSpawnPositions drops the opening-move markers once the encounter's first
// response has been built, so later state reads don't replay the animation.
func ClearSpawnPositions(cs *types.CombatSession) {
	if cs == nil {
		return
	}
	for i := range cs.Monsters {
		cs.Monsters[i].SpawnPos = nil
	}
}

// placeMonsters lays the monsters out in a column startRange cells from the
// player, fanning out from the centre row (0, -1, +1, -2, +2, …) and spilling
// into the next column back once a column fills. Nobody starts stacked on the
// player — the closest any monster spawns is adjacent.
func placeMonsters(cs *types.CombatSession, startRange int) {
	if startRange < 1 {
		startRange = 1
	}
	col := playerPos(cs).X + startRange
	if col > cs.GridWidth-2 {
		col = cs.GridWidth - 2
	}
	mid := cs.GridHeight / 2
	offsets := []int{0}
	for d := 1; d <= cs.GridHeight; d++ {
		offsets = append(offsets, -d, d)
	}

	slot := 0
	for i := range cs.Monsters {
		for {
			x := col + slot/cs.GridHeight
			y := mid + offsets[slot%cs.GridHeight]
			slot++
			if x >= cs.GridWidth {
				x = cs.GridWidth - 1
			}
			pos := types.Position{X: x, Y: y}
			if inGrid(cs, pos) && !occupied(cs, pos, cs.Monsters[i].InstanceID) {
				cs.Monsters[i].Pos = pos
				break
			}
			if slot > cs.GridWidth*cs.GridHeight {
				cs.Monsters[i].Pos = pos // board full — accept the overlap rather than loop forever
				break
			}
		}
	}
}

// assignInstanceIDs gives every monster a unique instance id and, when a template
// appears more than once, a numbered display name ("Goblin 1", "Goblin 2"). A lone
// monster keeps its template id as its instance id.
func assignInstanceIDs(monsters []types.MonsterInstance) {
	total := map[string]int{}
	for _, m := range monsters {
		total[m.TemplateID]++
	}
	seen := map[string]int{}
	for i := range monsters {
		m := &monsters[i]
		if total[m.TemplateID] < 2 {
			m.InstanceID = m.TemplateID
			continue
		}
		seen[m.TemplateID]++
		n := seen[m.TemplateID]
		m.InstanceID = fmt.Sprintf("%s#%d", m.TemplateID, n)
		m.Name = fmt.Sprintf("%s %d", m.Data.Name, n)
	}
}

// aiTarget chooses which party member a monster goes after, honouring its
// authored target_priority ("lowest_hp", "random"; default/"highest_threat" →
// nearest). Unconscious members are skipped while anyone is still standing.
func aiTarget(cs *types.CombatSession, m *types.MonsterInstance) *types.PartyCombatant {
	var standing []*types.PartyCombatant
	for i := range cs.Party {
		if !cs.Party[i].CombatState.IsUnconscious {
			standing = append(standing, &cs.Party[i])
		}
	}
	if len(standing) == 0 {
		if len(cs.Party) == 0 {
			return nil
		}
		return &cs.Party[0]
	}
	switch m.Data.Behavior.TargetPriority {
	case "lowest_hp":
		best := standing[0]
		for _, p := range standing[1:] {
			if p.CombatState.CurrentHP < best.CombatState.CurrentHP {
				best = p
			}
		}
		return best
	case "random":
		return standing[rand.Intn(len(standing))]
	default:
		best := standing[0]
		for _, p := range standing[1:] {
			if chebyshev(m.Pos, p.Pos) < chebyshev(m.Pos, best.Pos) {
				best = p
			}
		}
		return best
	}
}

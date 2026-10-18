package combat

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"pubkey-quest/types"
)

// ─── Area-of-effect spells ───────────────────────────────────────────────────
//
// A save spell with an `aoe` block (types.SpellArea) catches every enemy in its
// area, and each makes its own save (the engine rolls them; castConsequences
// applies each result). The area is measured on the grid, in 5-ft cells:
//
//   sphere       every cell within radius (Chebyshev) of the target's cell — fireball
//   self_radius  every cell within radius of the caster — arms of hadar
//   cone         from the caster toward the target, as wide at each step as
//                half its distance either side of the axis — burning hands
//   line         one cell wide, from the caster through the target — lightning bolt
//
// A wall that blocks sight from the area's origin (the target's cell for a
// sphere, the caster otherwise) shelters whoever stands behind it.
//
// Only enemies are caught: the caster and their allies are spared. Spells are
// aimed by picking a monster, with no template shown on the board first, so
// friendly fire would land blind.

// spellAreaOf reads a spell's aoe block, or nil if it has none.
func spellAreaOf(spell map[string]interface{}) *types.SpellArea {
	raw, ok := spell["aoe"].(map[string]interface{})
	if !ok {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var a types.SpellArea
	if json.Unmarshal(b, &a) != nil || a.Shape == "" || a.Radius < 1 {
		return nil
	}
	return &a
}

// areaOrigin is the cell an area is measured from.
func areaOrigin(area *types.SpellArea, caster, aim types.Position) types.Position {
	if area.Shape == "sphere" {
		return aim
	}
	return caster
}

// inArea reports whether cell lies in an area cast by a caster at caster,
// aimed at the cell aim.
func inArea(area *types.SpellArea, caster, aim, cell types.Position) bool {
	switch area.Shape {
	case "sphere":
		return chebyshev(aim, cell) <= area.Radius
	case "self_radius":
		d := chebyshev(caster, cell)
		return d >= 1 && d <= area.Radius
	case "cone", "line":
		d := chebyshev(caster, cell)
		if d < 1 || d > area.Radius || aim == caster {
			return false
		}
		// Project the cell onto the caster→aim axis.
		vx, vy := float64(aim.X-caster.X), float64(aim.Y-caster.Y)
		ux, uy := float64(cell.X-caster.X), float64(cell.Y-caster.Y)
		along := (ux*vx + uy*vy) / math.Hypot(vx, vy)
		if along <= 0 {
			return false
		}
		perp := math.Sqrt(math.Max(0, ux*ux+uy*uy-along*along))
		const eps = 1e-9
		if area.Shape == "line" {
			return perp <= 0.5+eps
		}
		return perp <= along/2+eps
	}
	return false
}

// areaTargets returns the enemies an area spell aimed at aim catches: the
// aimed monster first when it's caught, the rest nearest the origin first.
func areaTargets(cs *types.CombatSession, caster *types.PartyCombatant, aim *types.MonsterInstance, area *types.SpellArea) []*types.MonsterInstance {
	origin := areaOrigin(area, caster.Pos, aim.Pos)
	var caught []*types.MonsterInstance
	for _, m := range activeMonsters(cs) {
		if !inArea(area, caster.Pos, aim.Pos, m.Pos) {
			continue
		}
		if visible, _ := Sight(cs, origin, m.Pos); !visible {
			continue
		}
		caught = append(caught, m)
	}
	sort.SliceStable(caught, func(i, j int) bool {
		if (caught[i] == aim) != (caught[j] == aim) {
			return caught[i] == aim
		}
		return chebyshev(origin, caught[i].Pos) < chebyshev(origin, caught[j].Pos)
	})
	return caught
}

// aimArea works out who an area spell catches when aimed at monster. It
// returns the cast's main target and everyone caught (nil for a spell with no
// area). A spell around the caster hits whoever is near, so its main target
// is the first enemy caught; a cone or line must reach the monster it's aimed at.
func aimArea(cs *types.CombatSession, caster *types.PartyCombatant, spell map[string]interface{}, name string, monster *types.MonsterInstance) (*types.MonsterInstance, []*types.MonsterInstance, error) {
	area := spellAreaOf(spell)
	if area == nil || monster == nil {
		return monster, nil, nil
	}
	caught := areaTargets(cs, caster, monster, area)
	if area.Shape == "self_radius" {
		if len(caught) == 0 {
			return nil, nil, fmt.Errorf("no enemy is within %d ft of you for %s", area.Radius*5, name)
		}
		return caught[0], caught, nil
	}
	if len(caught) == 0 || caught[0] != monster {
		return nil, nil, fmt.Errorf("%s is beyond the reach of %s (%d ft)", monster.Name, name, area.Radius*5)
	}
	return monster, caught, nil
}

// selfCentred reports whether a spell's area is centred on its caster, so it
// needs no line to the monster it was aimed at.
func selfCentred(spell map[string]interface{}) bool {
	a := spellAreaOf(spell)
	return a != nil && a.Shape == "self_radius"
}

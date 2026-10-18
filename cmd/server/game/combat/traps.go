package combat

import (
	"fmt"
	"sort"
	"strings"

	"pubkey-quest/types"
)

// ─── Traps: caltrops, ball bearings ──────────────────────────────────────────
//
// A trap cell is terrain with a "trap" block (combat-terrain.json): a creature
// stepping onto it makes a save, and on a failure takes the trap's damage, gets
// its condition and (if stop) ends its move there. Unlike a hazard it does
// nothing to a creature that just stands on it. The player scatters them with
// an item's "scatter" block; flying creatures pass over.

// TerrainTrap is what a trap cell does to a creature that steps onto it.
type TerrainTrap struct {
	Save       string `json:"save"` // ability for the save ("dexterity")
	DC         int    `json:"dc"`
	Damage     string `json:"damage,omitempty"` // dice or a flat number, on a failed save
	DamageType string `json:"damage_type,omitempty"`
	Condition  string `json:"condition,omitempty"` // landed on a failed save ("prone")
	Stop       bool   `json:"stop,omitempty"`      // a failed save ends the move here
}

// trapAt returns the trap at pos, or nil.
func trapAt(cs *types.CombatSession, pos types.Position) *TerrainTrap {
	return terrainTypeAt(cs, pos).Trap
}

// TrapNote describes a trap for the legend: "DC 15 Dexterity save or 1 piercing
// damage and stop moving".
func TrapNote(t *TerrainTrap) string {
	if t == nil {
		return ""
	}
	var fail []string
	if t.Damage != "" {
		fail = append(fail, fmt.Sprintf("%s %s damage", t.Damage, t.DamageType))
	}
	if t.Condition != "" {
		fail = append(fail, "fall "+t.Condition)
	}
	if t.Stop {
		fail = append(fail, "stop moving")
	}
	return fmt.Sprintf("step on it: DC %d %s save or %s", t.DC, capitalize(t.Save), strings.Join(fail, " and "))
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// trapPlanCost is the extra route cost of stepping on a trap: movers go round
// when they easily can, and cross when they can't.
const trapPlanCost = 3

// playerPlanCost keeps the player's routes off trap cells they didn't ask to
// stand on (their own caltrops included).
func playerPlanCost(cs *types.CombatSession) func(types.Position) int {
	return func(p types.Position) int {
		if trapAt(cs, p) != nil {
			return trapPlanCost
		}
		return 0
	}
}

// playerTrap resolves a trap for the acting party member stepping onto pos:
// the log, the damage to deal (the caller deals it once the move is placed),
// and whether the move stops here.
func playerTrap(cs *types.CombatSession, save *types.SaveFile, pos types.Position) ([]string, int, bool) {
	t := trapAt(cs, pos)
	if t == nil {
		return nil, 0, false
	}
	name := strings.ToLower(terrainName(cs, terrainAt(cs, pos)))
	me := actor(cs)
	who := whoName(cs, me)
	total := playerSaveTotal(&me.CombatState, save, t.Save)
	if total >= t.DC {
		return []string{fmt.Sprintf("  %s %s through the %s (%s save %d vs DC %d).", who, verbFor(who, "pick", "picks"), name, t.Save, total, t.DC)}, 0, false
	}
	log := []string{fmt.Sprintf("  ✘ %s %s on the %s! (%s save %d vs DC %d)", who, verbFor(who, "step", "steps"), name, t.Save, total, t.DC)}
	dmg := 0
	if t.Damage != "" {
		dmg = RollDice(t.Damage, false)
		log = append(log, fmt.Sprintf("  %s %s %d %s damage.", who, verbFor(who, "take", "takes"), dmg, t.DamageType))
	}
	if t.Condition != "" {
		ApplyCondition(&me.CombatState.Conditions, types.CombatCondition{Name: t.Condition, DurationRounds: 1, Source: "trap"})
		log = append(log, fmt.Sprintf("  %s %s %s.", who, verbFor(who, "are", "is"), conditionWord(t.Condition)))
	}
	return log, dmg, t.Stop
}

// monsterTrap resolves a trap for a monster stepping onto pos: the log, the
// damage to deal, and whether its move stops here. Flyers pass over.
func monsterTrap(cs *types.CombatSession, m *types.MonsterInstance, pos types.Position) ([]string, int, bool) {
	t := trapAt(cs, pos)
	if t == nil || m.Data.Speed.Fly > 0 {
		return nil, 0, false
	}
	name := strings.ToLower(terrainName(cs, terrainAt(cs, pos)))
	total := monsterSaveTotal(m, t.Save)
	if total >= t.DC {
		return []string{fmt.Sprintf("  %s picks its way through the %s (%s save %d vs DC %d).", m.Name, name, t.Save, total, t.DC)}, 0, false
	}
	log := []string{fmt.Sprintf("  ✔ %s steps on the %s! (%s save %d vs DC %d)", m.Name, name, t.Save, total, t.DC)}
	dmg := 0
	if t.Damage != "" {
		dmg = RollDice(t.Damage, false)
		log = append(log, fmt.Sprintf("  %s takes %d %s damage.", m.Name, dmg, t.DamageType))
	}
	if t.Condition != "" && !containsFold(m.Data.ConditionImmunities, t.Condition) {
		ApplyCondition(&m.Conditions, types.CombatCondition{Name: t.Condition, DurationRounds: 1, Source: "trap"})
		log = append(log, fmt.Sprintf("  %s is %s.", m.Name, conditionWord(t.Condition)))
	}
	return log, dmg, t.Stop
}

func verbFor(who, you, other string) string {
	if who == "You" {
		return you
	}
	return other
}

func conditionWord(c string) string {
	if c == "prone" {
		return "knocked prone"
	}
	if p := conditionPhrase[c]; p != "" {
		return p
	}
	return c
}

// scatterItem spreads a trap item (an item with a "scatter" block: caltrops,
// ball bearings) over the open ground between the player and the chosen foe —
// the cells within two squares of the player, nearest the foe first. It costs
// the action, and the item is used up.
//
//	"scatter": {"terrain": "caltrops", "cells": 3}
func scatterItem(cs *types.CombatSession, item, scatter map[string]interface{}, name string, slot map[string]interface{}, fromPack bool, targetID string) ([]string, error) {
	terrain, _ := scatter["terrain"].(string)
	if _, ok := activeTerrain().Types[terrain]; !ok || terrain == "" {
		return nil, fmt.Errorf("the %s can't be scattered here", name)
	}
	n := 3
	if v, ok := scatter["cells"].(float64); ok && v > 0 {
		n = int(v)
	}
	me := actor(cs)
	foe, err := ResolveTarget(cs, targetID)
	if err != nil {
		return nil, err
	}

	var cells []types.Position
	for y := me.Pos.Y - 2; y <= me.Pos.Y+2; y++ {
		for x := me.Pos.X - 2; x <= me.Pos.X+2; x++ {
			p := types.Position{X: x, Y: y}
			if p == me.Pos || !inGrid(cs, p) || terrainAt(cs, p) != "open" || occupied(cs, p, "") {
				continue
			}
			cells = append(cells, p)
		}
	}
	if len(cells) == 0 {
		return nil, fmt.Errorf("there's no open ground around you to scatter the %s on", name)
	}
	sort.SliceStable(cells, func(i, j int) bool {
		di, dj := chebyshev(cells[i], foe.Pos), chebyshev(cells[j], foe.Pos)
		if di != dj {
			return di < dj
		}
		return chebyshev(cells[i], me.Pos) < chebyshev(cells[j], me.Pos)
	})
	if len(cells) > n {
		cells = cells[:n]
	}
	if len(cs.Terrain) != cs.GridWidth*cs.GridHeight {
		cs.Terrain = make([]string, cs.GridWidth*cs.GridHeight)
	}
	for _, p := range cells {
		cs.Terrain[p.Y*cs.GridWidth+p.X] = terrain
	}

	var log []string
	if fromPack {
		log = append(log, fmt.Sprintf("  You dig the %s out of your pack (bonus action).", name))
	}
	log = append(log, fmt.Sprintf("  You scatter the %s across %d squares toward %s.", name, len(cells), foe.Name))
	decrementSlotStack(slot)
	me.CombatState.ActionUsed = true
	if fromPack {
		me.CombatState.BonusActionUsed = true
	}
	return log, nil
}

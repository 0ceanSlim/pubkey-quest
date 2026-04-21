package combat

import (
	"fmt"
	"strings"

	"pubkey-quest/types"
)

// MonsterDecision describes what a monster will do on its turn.
type MonsterDecision struct {
	Move        int    // Intent: -1 = move closer, 0 = stay, +1 = move farther
	Action      string // "attack", "flee", "none"
	ActionIndex int    // Index into monster.Data.Actions to use
}

// DecideMonsterAction runs the monster AI decision tree and returns its chosen turn.
func DecideMonsterAction(cs *types.CombatSession, monster *types.MonsterInstance) MonsterDecision {
	// 1. Check flee threshold
	hpFraction := float64(monster.CurrentHP) / float64(monster.MaxHP)
	if hpFraction <= monster.Data.Behavior.FleeThreshold && monster.Data.Behavior.Aggression != "berserker" {
		return MonsterDecision{Move: 1, Action: "flee"}
	}

	preferred := monster.Data.Behavior.PreferredRange
	if preferred == 0 {
		preferred = monster.Data.PreferredRange
	}
	r := currentRange(cs)

	// 2. Decide movement direction toward preferred range
	var move int
	switch {
	case r > preferred:
		move = -1 // Move closer
	case r < preferred:
		move = 1 // Move farther
	default:
		move = 0 // Already at preferred range
	}

	// 3. Select best available attack for projected (post-move) range
	projectedRange := r + move
	if projectedRange < 0 {
		projectedRange = 0
	}

	actionIdx := selectBestAction(monster.Data.Actions, projectedRange)
	if actionIdx < 0 {
		// No attack available at projected range — try staying put
		actionIdx = selectBestAction(monster.Data.Actions, r)
		if actionIdx < 0 {
			return MonsterDecision{Move: 0, Action: "none"}
		}
		move = 0
	}

	return MonsterDecision{Move: move, Action: "attack", ActionIndex: actionIdx}
}

// selectBestAction returns the index of the first action usable at the given range.
// Returns -1 if no action is usable.
func selectBestAction(actions []types.MonsterAction, currentRange int) int {
	for i, action := range actions {
		switch action.Type {
		case "melee_attack":
			reach := 1 // Default melee = adjacent (Range 0–1)
			if action.Reach != nil && *action.Reach > 0 {
				reach = *action.Reach
			}
			if currentRange <= reach {
				return i
			}
		case "ranged_attack":
			maxRange := 3 // Fallback
			if action.RangeLong != nil {
				maxRange = *action.RangeLong
			} else if action.Range != nil {
				maxRange = *action.Range
			}
			if currentRange <= maxRange {
				return i
			}
		}
	}
	return -1
}

// ApplyMonsterMove moves the monster on the grid toward or away from the player.
// Uses greedy pathfinding (one Chebyshev step per movement point).
// Call this before ApplyMonsterAction when you need to interleave other logic.
func ApplyMonsterMove(cs *types.CombatSession, monster *types.MonsterInstance, decision MonsterDecision) []string {
	if decision.Move == 0 {
		return nil
	}

	speed := monster.Data.Speed.Walk
	if speed <= 0 {
		speed = 30
	}
	maxSteps := speed / 5

	preferred := monster.Data.Behavior.PreferredRange
	if preferred == 0 {
		preferred = monster.Data.PreferredRange
	}

	moved := 0
	for i := 0; i < maxSteps; i++ {
		r := chebyshev(cs.PlayerPos, cs.MonsterPos)
		if decision.Move == -1 && r <= preferred {
			break // Already at or closer than preferred range
		}
		if decision.Move == 1 && r >= preferred {
			break // Already at or farther than preferred range
		}

		newPos := stepMonster(cs.MonsterPos, cs.PlayerPos, decision.Move, cs.GridWidth, cs.GridHeight)
		if newPos == cs.MonsterPos {
			break // Stuck (edge of grid)
		}
		if newPos == cs.PlayerPos {
			break // Can't enter player's cell
		}
		cs.MonsterPos = newPos
		moved++
	}

	if moved == 0 {
		return nil
	}
	dir := "toward you"
	if decision.Move > 0 {
		dir = "away from you"
	}
	return []string{fmt.Sprintf("  %s moves %s. (range: %d)", monster.Name, dir, currentRange(cs))}
}

// stepMonster returns the next grid position one Chebyshev step in the given direction.
// moveDir: -1 = toward player, +1 = away from player.
func stepMonster(from, player types.Position, moveDir, gridW, gridH int) types.Position {
	var dx, dy int
	if moveDir == -1 {
		dx = clampInt(player.X-from.X, -1, 1)
		dy = clampInt(player.Y-from.Y, -1, 1)
	} else {
		dx = clampInt(from.X-player.X, -1, 1)
		dy = clampInt(from.Y-player.Y, -1, 1)
	}
	next := types.Position{X: from.X + dx, Y: from.Y + dy}
	// Clamp to grid bounds
	if next.X < 0 {
		next.X = 0
	}
	if next.X >= gridW {
		next.X = gridW - 1
	}
	if next.Y < 0 {
		next.Y = 0
	}
	if next.Y >= gridH {
		next.Y = gridH - 1
	}
	return next
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ApplyMonsterAction executes the monster's chosen action (attack/flee/none).
// Movement must already have been applied. Returns damage dealt and log entries.
//
// useReflex: when true, the player makes a reflex save (d20+reflexDEXMod vs DC 12)
// before damage resolves — on success the attack misses entirely. Pass false normally.
func ApplyMonsterAction(cs *types.CombatSession, monster *types.MonsterInstance, decision MonsterDecision, playerAC int, useReflex bool, reflexDEXMod int) (damageDealt int, logEntries []string) {
	switch decision.Action {
	case "flee":
		logEntries = append(logEntries, fmt.Sprintf("  %s turns and flees!", monster.Name))
		cs.Phase = "victory"

	case "none":
		logEntries = append(logEntries, fmt.Sprintf("  %s has no action available.", monster.Name))

	case "attack":
		action := monster.Data.Actions[decision.ActionIndex]

		// If the player is dodging this turn, the monster attacks at disadvantage.
		monsterAdvantage := 0
		if len(cs.Party) > 0 && cs.Party[0].CombatState.Dodging {
			monsterAdvantage = -1
		}
		result := ResolveAttackRoll(action.AttackBonus, playerAC, monsterAdvantage)

		logEntries = append(logEntries, fmt.Sprintf(
			"  %s attacks with %s: rolled %d%s — %s",
			monster.Name, action.Name,
			result.Roll, formatModifier(action.AttackBonus), attackQualifier(result),
		))

		if result.IsHit {
			// Reflex save: player hasn't chosen their stance, so they may dodge on instinct.
			if useReflex {
				reflexRoll := RollD20() + reflexDEXMod
				logEntries = append(logEntries, fmt.Sprintf(
					"  You react on instinct — reflex save: rolled %d (DC 12).", reflexRoll,
				))
				if reflexRoll >= 12 {
					logEntries = append(logEntries, "  You twist away just in time — the attack misses!")
					break // attack negated
				}
				logEntries = append(logEntries, "  Not quick enough to fully evade!")
			}

			dmg := ResolveDamageToPlayer(action.Hit.Dice, action.Hit.Mod, result.IsCrit)
			damageDealt = dmg
			critStr := ""
			if result.IsCrit {
				critStr = " CRITICAL HIT!"
			}
			logEntries = append(logEntries, fmt.Sprintf(
				"  %s deals %d %s damage.%s",
				monster.Name, dmg, action.Hit.Type, critStr,
			))
		}
	}
	return damageDealt, logEntries
}

// ExecuteMonsterTurn runs the monster's full turn (move + action).
// Returns damage dealt and all log entries.
func ExecuteMonsterTurn(cs *types.CombatSession, monster *types.MonsterInstance, playerAC int, useReflex bool, reflexDEXMod int) (damageDealt int, logEntries []string) {
	decision := DecideMonsterAction(cs, monster)
	logEntries = append(logEntries, ApplyMonsterMove(cs, monster, decision)...)
	dmg, actionLog := ApplyMonsterAction(cs, monster, decision, playerAC, useReflex, reflexDEXMod)
	return dmg, append(logEntries, actionLog...)
}

// attackQualifier returns a short string describing the attack roll outcome.
func attackQualifier(r AttackResult) string {
	switch {
	case r.IsCrit:
		return " — CRITICAL HIT!"
	case r.IsCritMiss:
		return " — critical miss!"
	case r.IsHit:
		return fmt.Sprintf(" vs AC %d — HIT!", r.Total)
	default:
		return " vs AC — MISS"
	}
}

// formatModifier turns an integer into "+N" or "-N" string.
func formatModifier(mod int) string {
	if mod >= 0 {
		return fmt.Sprintf("+%d", mod)
	}
	return fmt.Sprintf("%d", mod)
}

// IsRangedAction returns true if the action type indicates a ranged attack.
func IsRangedAction(actionType string) bool {
	return strings.Contains(strings.ToLower(actionType), "ranged")
}

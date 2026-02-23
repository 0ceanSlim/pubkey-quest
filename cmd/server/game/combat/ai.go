package combat

import (
	"fmt"
	"strings"

	"pubkey-quest/types"
)

// MonsterDecision describes what a monster will do on its turn.
type MonsterDecision struct {
	Move   int    // Range change: -1 = closer, 0 = stay, +1 = farther
	Action string // "attack", "flee", "none"
	ActionIndex int // Index into monster.Data.Actions to use
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
	currentRange := cs.Range

	// 2. Decide movement toward preferred range
	var move int
	switch {
	case currentRange > preferred:
		move = -1 // Move closer
	case currentRange < preferred:
		move = 1 // Move farther
	default:
		move = 0 // Already at preferred range
	}

	// 3. Select best available attack for current (post-move) range
	projectedRange := currentRange + move
	if projectedRange < 0 {
		projectedRange = 0
	}

	actionIdx := selectBestAction(monster.Data.Actions, projectedRange)
	if actionIdx < 0 {
		// No attack available at projected range — try staying put
		actionIdx = selectBestAction(monster.Data.Actions, currentRange)
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

// ApplyMonsterMove applies the monster's movement decision and returns log entries.
// Call this before ApplyMonsterAction when you need to interleave other logic between them.
func ApplyMonsterMove(cs *types.CombatSession, monster *types.MonsterInstance, decision MonsterDecision) []string {
	if decision.Move == 0 {
		return nil
	}
	cs.Range += decision.Move
	if cs.Range < 0 {
		cs.Range = 0
	}
	dir := "closer"
	if decision.Move > 0 {
		dir = "farther away"
	}
	return []string{fmt.Sprintf("  %s moves %s (range: %d).", monster.Name, dir, cs.Range)}
}

// ApplyMonsterAction executes the monster's chosen action (attack/flee/none).
// Movement must already have been applied. Returns damage dealt and log entries.
func ApplyMonsterAction(cs *types.CombatSession, monster *types.MonsterInstance, decision MonsterDecision, playerAC int) (damageDealt int, logEntries []string) {
	switch decision.Action {
	case "flee":
		logEntries = append(logEntries, fmt.Sprintf("  %s turns and flees!", monster.Name))
		cs.Phase = "victory"

	case "none":
		logEntries = append(logEntries, fmt.Sprintf("  %s has no action available.", monster.Name))

	case "attack":
		action := monster.Data.Actions[decision.ActionIndex]
		result := ResolveAttackRoll(action.AttackBonus, playerAC, 0)

		logEntries = append(logEntries, fmt.Sprintf(
			"  %s attacks with %s: rolled %d%s — %s",
			monster.Name, action.Name,
			result.Roll, formatModifier(action.AttackBonus), attackQualifier(result),
		))

		if result.IsHit {
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
func ExecuteMonsterTurn(cs *types.CombatSession, monster *types.MonsterInstance, playerAC int) (damageDealt int, logEntries []string) {
	decision := DecideMonsterAction(cs, monster)
	logEntries = append(logEntries, ApplyMonsterMove(cs, monster, decision)...)
	dmg, actionLog := ApplyMonsterAction(cs, monster, decision, playerAC)
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
		return fmt.Sprintf(" vs AC %d — HIT!", r.Total) // simplified
	default:
		return fmt.Sprintf(" vs AC — MISS")
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

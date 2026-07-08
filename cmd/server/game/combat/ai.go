package combat

import (
	"fmt"
	"strings"

	"pubkey-quest/types"
)

// MonsterDecision describes what a monster will do on its turn.
type MonsterDecision struct {
	Move        int    // Intent: -1 = move closer, 0 = stay, +1 = move farther (away)
	TargetRange int    // Range the monster is trying to reach during movement
	Action      string // "attack", "retreat", "escape", "none"
	ActionIndex int    // Index into monster.Data.Actions to use
}

// fleeMinPlayerRange is the minimum Chebyshev distance to the player required
// before a wounded monster will try to flee. If the player is adjacent/nearby,
// the monster is cornered and fights instead.
const fleeMinPlayerRange = 3

// atGridEdge reports whether the position lies on any outer edge of the grid.
// Wounded monsters escape combat once they reach an edge with the player still far.
func atGridEdge(pos types.Position, gridW, gridH int) bool {
	return pos.X == 0 || pos.Y == 0 || pos.X == gridW-1 || pos.Y == gridH-1
}

// adjustPreferredRange shifts a monster's preferred range based on aggression + HP.
// Aggressive monsters press in at full health; cautious ones back off when wounded.
func adjustPreferredRange(preferred int, aggression string, hpFraction float64) int {
	switch aggression {
	case "berserker":
		// Always crash into melee, regardless of JSON preference.
		return 0
	case "aggressive":
		// At healthy HP, push one step closer than configured preference.
		if hpFraction >= 0.75 && preferred > 0 {
			return preferred - 1
		}
	case "cautious":
		// When wounded (but not fleeing), open up more distance.
		if hpFraction < 0.75 && preferred < 6 {
			return preferred + 1
		}
	}
	return preferred
}

// effectivePreferredRange returns the aggression/HP-adjusted range the monster wants to hold.
func effectivePreferredRange(monster *types.MonsterInstance) int {
	preferred := monster.Data.Behavior.PreferredRange
	if preferred == 0 && monster.Data.PreferredRange != 0 {
		preferred = monster.Data.PreferredRange
	}
	aggression := monster.Data.Behavior.Aggression
	if aggression == "" {
		aggression = "aggressive"
	}
	hpFraction := float64(monster.CurrentHP) / float64(monster.MaxHP)
	return adjustPreferredRange(preferred, aggression, hpFraction)
}

// DecideMonsterAction runs the monster AI decision tree and returns its chosen turn.
// Movement is chosen here; the final attack is resolved *after* ApplyMonsterMove runs
// via RefreshAttackDecision, so monsters that can cover multiple cells this turn still
// pick a valid attack at their actual post-move range.
func DecideMonsterAction(cs *types.CombatSession, monster *types.MonsterInstance) MonsterDecision {
	// Flee when badly wounded (berserkers never flee, cornered monsters can't).
	hpFraction := float64(monster.CurrentHP) / float64(monster.MaxHP)
	aggression := monster.Data.Behavior.Aggression
	if aggression == "" {
		aggression = "aggressive"
	}
	r := currentRange(cs)
	wantsToFlee := hpFraction <= monster.Data.Behavior.FleeThreshold && aggression != "berserker"
	if wantsToFlee && r >= fleeMinPlayerRange {
		// Already at an edge → escape this turn.
		if atGridEdge(cs.MonsterPos, cs.GridWidth, cs.GridHeight) {
			return MonsterDecision{Move: 0, Action: "escape"}
		}
		// Otherwise break toward the edge (away from the player). TargetRange=6 lets
		// ApplyMonsterMove run the monster the full length of its speed until it
		// bumps the grid boundary.
		return MonsterDecision{Move: 1, TargetRange: 6, Action: "retreat"}
	}

	preferred := effectivePreferredRange(monster)

	move := 0
	switch {
	case r > preferred:
		move = -1
	case r < preferred:
		move = 1
	}

	// If already at preferred range and an attack is usable, take it without moving.
	if move == 0 {
		if idx := selectBestAction(monster.Data.Actions, r); idx >= 0 {
			return MonsterDecision{Move: 0, TargetRange: preferred, Action: "attack", ActionIndex: idx}
		}
	}

	// Commit to movement; the attack is resolved post-move once the actual range is known.
	return MonsterDecision{Move: move, TargetRange: preferred, Action: "none"}
}

// RefreshAttackDecision picks the best attack for the actual range after movement.
// Upgrades a "none" decision to "attack" when an action is now usable at the post-move range.
// Upgrades a "retreat" decision to "escape" when the monster reached an edge with the
// player still at least fleeMinPlayerRange cells away.
func RefreshAttackDecision(cs *types.CombatSession, monster *types.MonsterInstance, decision MonsterDecision) MonsterDecision {
	switch decision.Action {
	case "escape":
		return decision
	case "retreat":
		if currentRange(cs) >= fleeMinPlayerRange &&
			atGridEdge(cs.MonsterPos, cs.GridWidth, cs.GridHeight) {
			decision.Action = "escape"
		}
		return decision
	}
	idx := selectBestAction(monster.Data.Actions, currentRange(cs))
	if idx >= 0 {
		decision.Action = "attack"
		decision.ActionIndex = idx
	} else {
		decision.Action = "none"
	}
	return decision
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

// MonsterMeleeReach returns the max reach among the monster's melee actions.
// Returns 0 if the monster has no melee actions.
func MonsterMeleeReach(monster *types.MonsterInstance) int {
	max := 0
	for _, a := range monster.Data.Actions {
		if a.Type != "melee_attack" {
			continue
		}
		reach := 1
		if a.Reach != nil && *a.Reach > 0 {
			reach = *a.Reach
		}
		if reach > max {
			max = reach
		}
	}
	return max
}

// ApplyMonsterMove moves the monster on the grid toward or away from the player.
// Uses greedy pathfinding (one Chebyshev step per movement point).
// Movement budget is monster.Data.Speed.Walk / 5 (falls back to 6 if unset).
//
// oaTrigger, if non-nil, is called at most once — on the first step that takes
// the monster from within the player's melee reach to outside it. The callback
// receives the step's distance (always 1) and must itself enforce reaction/
// disengage rules; its returned log entries are appended to the move log.
func ApplyMonsterMove(cs *types.CombatSession, monster *types.MonsterInstance, decision MonsterDecision, playerMeleeReach int, oaTrigger func() []string) []string {
	if decision.Move == 0 {
		return nil
	}

	speed := monster.Data.Speed.Walk
	if speed <= 0 {
		speed = 30
	}
	maxSteps := speed / 5

	preferred := decision.TargetRange
	var oaLog []string
	oaFired := false

	moved := 0
	for i := 0; i < maxSteps; i++ {
		r := chebyshev(cs.PlayerPos, cs.MonsterPos)
		if decision.Move == -1 && r <= preferred {
			break
		}
		if decision.Move == 1 && r >= preferred {
			break
		}

		newPos := stepMonster(cs.MonsterPos, cs.PlayerPos, decision.Move, cs.GridWidth, cs.GridHeight)
		if newPos == cs.MonsterPos {
			break
		}
		if newPos == cs.PlayerPos {
			break
		}
		prevR := r
		cs.MonsterPos = newPos
		moved++

		newR := chebyshev(cs.PlayerPos, cs.MonsterPos)

		// Opportunity attack check: monster left the player's melee reach.
		if !oaFired && oaTrigger != nil && playerMeleeReach > 0 &&
			prevR <= playerMeleeReach && newR > playerMeleeReach {
			if entries := oaTrigger(); entries != nil {
				oaLog = append(oaLog, entries...)
			}
			oaFired = true
			// If the OA killed the monster, stop moving.
			if !monster.IsAlive {
				break
			}
		}
	}

	if moved == 0 {
		return oaLog
	}
	dir := "toward you"
	if decision.Move > 0 {
		dir = "away from you"
	}
	out := []string{fmt.Sprintf("  %s moves %s. (range: %d)", monster.Name, dir, currentRange(cs))}
	return append(out, oaLog...)
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
func ApplyMonsterAction(cs *types.CombatSession, monster *types.MonsterInstance, decision MonsterDecision, playerAC int, useReflex bool, reflexDEXMod int, save *types.SaveFile) (damageDealt int, logEntries []string) {
	// Stunned / paralyzed / unconscious monsters lose their action entirely.
	if IsIncapacitated(monster.Conditions) {
		return 0, []string{fmt.Sprintf("  %s is %s and can't act.", monster.Name, incapacitatingConditionName(monster.Conditions))}
	}
	// A charmed monster can't bring itself to attack the one who charmed it.
	if decision.Action == "attack" && HasCondition(monster.Conditions, "charmed") {
		return 0, []string{fmt.Sprintf("  %s is charmed and won't attack you.", monster.Name)}
	}
	switch decision.Action {
	case "retreat":
		// Monster is wounded and scrambling for the grid edge; attack skipped this turn.
		logEntries = append(logEntries, fmt.Sprintf("  %s is wounded and tries to flee!", monster.Name))

	case "escape":
		// Monster reached the edge with the player too far to stop it — combat ends with no kill.
		logEntries = append(logEntries, fmt.Sprintf("  %s escapes off the edge of the battlefield!", monster.Name))
		cs.Phase = "loot"
		cs.LootRolled = nil

	case "none":
		logEntries = append(logEntries, fmt.Sprintf("  %s has no action available.", monster.Name))

	case "attack":
		action := monster.Data.Actions[decision.ActionIndex]

		// If the player is dodging this turn, the monster attacks at disadvantage.
		monsterAdvantage := 0
		var playerConds []types.CombatCondition
		if len(cs.Party) > 0 {
			if cs.Party[0].CombatState.Dodging {
				monsterAdvantage = -1
			}
			playerConds = cs.Party[0].CombatState.Conditions
		}
		// Conditions: the monster's own (poisoned/frightened/…) impose disadvantage;
		// the player's (prone/restrained/…) grant the monster advantage.
		monsterAdvantage += ConditionAttackAdvantage(monster.Conditions, playerConds)
		result := ResolveAttackRoll(action.AttackBonus, playerAC, monsterAdvantage)

		logEntries = append(logEntries,
			fmt.Sprintf(
				"  %s attacks with %s: rolled %d%s",
				monster.Name, action.Name,
				result.Roll, formatModifier(action.AttackBonus),
			),
			outcomeLine(result),
		)

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
			// On-hit condition rider: the player saves or gains the condition.
			logEntries = append(logEntries, applyMonsterConditionRider(cs, save, action)...)
		}
	}
	return damageDealt, logEntries
}

// ExecuteMonsterTurn runs the monster's full turn (move + action).
// Returns damage dealt and all log entries. No opportunity attacks are resolved
// here (opening/death-save turns — player either hasn't started or is down).
func ExecuteMonsterTurn(cs *types.CombatSession, monster *types.MonsterInstance, playerAC int, useReflex bool, reflexDEXMod int, save *types.SaveFile) (damageDealt int, logEntries []string) {
	decision := DecideMonsterAction(cs, monster)
	logEntries = append(logEntries, ApplyMonsterMove(cs, monster, decision, 0, nil)...)
	decision = RefreshAttackDecision(cs, monster, decision)
	dmg, actionLog := ApplyMonsterAction(cs, monster, decision, playerAC, useReflex, reflexDEXMod, save)
	return dmg, append(logEntries, actionLog...)
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

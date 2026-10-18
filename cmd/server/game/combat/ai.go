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

// monsterRange is the range from a monster to the party member it's going after.
func monsterRange(cs *types.CombatSession, monster *types.MonsterInstance) int {
	t := aiTarget(cs, monster)
	if t == nil {
		return noTargetRange
	}
	return chebyshev(monster.Pos, t.Pos)
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
	r := monsterRange(cs, monster)
	wantsToFlee := hpFraction <= monster.Data.Behavior.FleeThreshold && aggression != "berserker" &&
		!HasCondition(monster.Conditions, "dueled") // a compelled duel holds it to the fight
	if wantsToFlee && r >= fleeMinPlayerRange {
		// Already at an edge → escape this turn.
		if atGridEdge(monster.Pos, cs.GridWidth, cs.GridHeight) {
			return MonsterDecision{Move: 0, Action: "escape"}
		}
		// Otherwise break toward the edge (away from the player). TargetRange=6 lets
		// ApplyMonsterMove run the monster the full length of its speed until it
		// bumps the grid boundary.
		return MonsterDecision{Move: 1, TargetRange: 6, Action: "retreat"}
	}

	preferred := effectivePreferredRange(monster)
	if preferred < 1 {
		preferred = 1 // adjacent is as close as two combatants get
	}

	move := 0
	switch {
	case r > preferred:
		move = -1
	case r < preferred:
		move = 1
	}

	// Already where it wants to be with an attack it can make (in range and in
	// sight)? Attack without moving. In range but cut off by cover? Close in to
	// find a line.
	if move == 0 {
		if idx := selectActionFor(cs, monster); idx >= 0 {
			return MonsterDecision{Move: 0, TargetRange: preferred, Action: "attack", ActionIndex: idx}
		}
		move = -1
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
		if monsterRange(cs, monster) >= fleeMinPlayerRange &&
			atGridEdge(monster.Pos, cs.GridWidth, cs.GridHeight) {
			decision.Action = "escape"
		}
		return decision
	}
	idx := selectActionFor(cs, monster)
	if idx >= 0 {
		decision.Action = "attack"
		decision.ActionIndex = idx
	} else {
		decision.Action = "none"
	}
	return decision
}

// actionReaches reports whether an attack action can reach currentRange.
func actionReaches(action types.MonsterAction, currentRange int) bool {
	switch action.Type {
	case "melee_attack":
		reach := 1 // Default melee = adjacent (Range 0–1)
		if action.Reach != nil && *action.Reach > 0 {
			reach = *action.Reach
		}
		return currentRange <= reach
	case "ranged_attack":
		maxRange := 3 // Fallback
		if action.RangeLong != nil {
			maxRange = *action.RangeLong
		} else if action.Range != nil {
			maxRange = *action.Range
		}
		return currentRange <= maxRange
	}
	return false
}

// selectActionFor returns the index of the monster's first attack it can make
// against its target from where it stands — in range and with a line of sight.
// Returns -1 if none.
func selectActionFor(cs *types.CombatSession, monster *types.MonsterInstance) int {
	t := aiTarget(cs, monster)
	if t == nil {
		return -1
	}
	if visible, _ := Sight(cs, monster.Pos, t.Pos); !visible {
		return -1
	}
	r := chebyshev(monster.Pos, t.Pos)
	for i, action := range monster.Data.Actions {
		if actionReaches(action, r) {
			return i
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

// ApplyMonsterMove moves the monster on the grid toward or away from its target.
// It routes by terrain cost (around cover, through rough ground only when it's
// worth it) to the best cell for what it wants this turn — see pickMonsterGoal —
// and walks as far along that route as its speed allows. It can pass through
// other monsters but never through a player, and never ends on an occupied cell.
// Movement budget is monster.Data.Speed.Walk / 5 (falls back to 6 if unset).
//
// oaTrigger, if non-nil, is called at most once — on the first step that takes
// the monster from within the player's melee reach to outside it. The callback
// must itself enforce reaction/disengage rules; its returned log entries are
// appended to the move log.
func ApplyMonsterMove(cs *types.CombatSession, monster *types.MonsterInstance, decision MonsterDecision, playerMeleeReach int, oaTrigger func() []string) []string {
	if decision.Move == 0 {
		return nil
	}
	if IsSpeedZero(monster.Conditions) {
		return []string{fmt.Sprintf("  %s is held fast and can't move.", monster.Name)}
	}
	target := aiTarget(cs, monster)
	if target == nil {
		return nil
	}

	speed := monster.Data.Speed.Walk
	if speed <= 0 {
		speed = 30
	}
	budget := speed / 5

	pm := shortestPaths(cs, monster.Pos, moveRules{moverID: monster.InstanceID, planCost: monsterPlanCost(cs, monster)})
	goal, ok := pickMonsterGoal(cs, monster, target, decision, pm)
	if !ok {
		return nil
	}
	path := pm.pathTo(goal)

	// How far along the route this turn's movement reaches, backing off to the
	// last cell it may actually stop on (it can't end inside another monster).
	stop := -1
	spent := 0
	for i, p := range path {
		prev := monster.Pos
		if i > 0 {
			prev = path[i-1]
		}
		c, ok := stepCost(cs, prev, p)
		if !ok || spent+c > budget {
			break
		}
		spent += c
		stop = i
	}
	for stop >= 0 && !endable(cs, path[stop], monster.InstanceID) {
		stop--
	}
	if stop < 0 {
		return nil
	}

	var oaLog []string
	oaFired := false
	player := playerPos(cs)
	for _, p := range path[:stop+1] {
		prevPR := chebyshev(player, monster.Pos)
		monster.Pos = p

		// Opportunity attack check: monster left the player's melee reach.
		newPR := chebyshev(player, monster.Pos)
		if !oaFired && oaTrigger != nil && playerMeleeReach > 0 &&
			prevPR <= playerMeleeReach && newPR > playerMeleeReach {
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

	dir := "toward you"
	if decision.Move > 0 {
		dir = "away from you"
	}
	out := []string{fmt.Sprintf("  %s moves %s. (range: %d)", monster.Name, dir, chebyshev(player, monster.Pos))}
	if t := terrainAt(cs, monster.Pos); t == "high_ground" || t == "deep_water" {
		out[0] = fmt.Sprintf("  %s moves %s, onto the %s. (range: %d)", monster.Name, dir,
			strings.ToLower(terrainName(cs, t)), chebyshev(player, monster.Pos))
	}
	return append(out, oaLog...)
}

// monsterPlanCost is the extra route cost a monster puts on cells it would
// rather avoid: hazards, and deep water when it can't swim.
func monsterPlanCost(cs *types.CombatSession, m *types.MonsterInstance) func(types.Position) int {
	swims := m.Data.Speed.Swim > 0 || m.Data.Speed.Fly > 0
	return func(p types.Position) int {
		t := terrainTypeAt(cs, p)
		extra := 0
		if t.Damage != "" {
			extra += 6
		}
		if t.Swim && !swims {
			extra += 3
		}
		return extra
	}
}

// pickMonsterGoal chooses where the monster heads this turn, from every cell it
// can route to:
//   - retreating: the nearest board edge, preferring ones far from its target;
//   - closing in: a cell within its preferred range that has a line of sight
//     to the target. Ranged monsters favour cells where the target would see
//     them through cover, and high ground; cautious ones care about cover more;
//   - backing off: a cell at least its preferred range away, again favouring cover.
//
// When no such cell can be reached, it settles for the reachable cell nearest
// (or farthest, when backing off) the target. Scores are route cost × 10 plus
// preferences; lowest wins, ties to the top-left for determinism.
func pickMonsterGoal(cs *types.CombatSession, m *types.MonsterInstance, target *types.PartyCombatant, decision MonsterDecision, pm pathMap) (types.Position, bool) {
	preferred := decision.TargetRange
	if preferred < 1 {
		preferred = 1
	}
	ranged := preferred > 1
	cautious := m.Data.Behavior.Aggression == "cautious"

	var best types.Position
	bestScore, found := 0, false
	consider := func(p types.Position, score int) {
		if !found || score < bestScore || (score == bestScore && (p.Y < best.Y || (p.Y == best.Y && p.X < best.X))) {
			best, bestScore, found = p, score, true
		}
	}
	preference := func(p types.Position) int {
		bonus := 0
		if ranged || cautious {
			if visible, cover := Sight(cs, target.Pos, p); visible && cover > 0 {
				w := 3
				if cautious {
					w = 5
				}
				bonus -= w * cover / 2
			}
		}
		if ranged && terrainTypeAt(cs, p).RangedBonus > 0 {
			bonus -= 8
		}
		return bonus
	}

	for p, plan := range pm.plan {
		if p != m.Pos && !endable(cs, p, m.InstanceID) {
			continue
		}
		r := chebyshev(p, target.Pos)
		switch {
		case decision.Action == "retreat" || decision.Action == "escape":
			if atGridEdge(p, cs.GridWidth, cs.GridHeight) {
				consider(p, plan*10-3*r)
			}
		case decision.Move < 0:
			if r <= preferred && r >= 1 {
				if visible, _ := Sight(cs, p, target.Pos); visible {
					consider(p, plan*10+preference(p)-r)
				}
			}
		default:
			if r >= preferred {
				consider(p, plan*10+preference(p))
			}
		}
	}
	if found {
		return best, best != m.Pos
	}

	// Nothing ideal is reachable: just make progress.
	for p, plan := range pm.plan {
		if p != m.Pos && !endable(cs, p, m.InstanceID) {
			continue
		}
		r := chebyshev(p, target.Pos)
		if decision.Move < 0 {
			if r >= 1 {
				consider(p, r*100+plan)
			}
		} else {
			consider(p, -r*100+plan)
		}
	}
	return best, found && best != m.Pos
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
		// Monster reached the edge with the player too far to stop it — it leaves
		// the fight with no kill. The fight ends once nobody hostile is left.
		logEntries = append(logEntries, fmt.Sprintf("  %s escapes off the edge of the battlefield!", monster.Name))
		monster.Fled = true
		logEntries = append(logEntries, checkVictory(cs)...)

	case "none":
		logEntries = append(logEntries, fmt.Sprintf("  %s has no action available.", monster.Name))

	case "attack":
		action := monster.Data.Actions[decision.ActionIndex]

		// If the target is dodging this turn, the monster attacks at disadvantage.
		monsterAdvantage := 0
		attackBonus := action.AttackBonus
		var playerConds []types.CombatCondition
		target := aiTarget(cs, monster) // picked once: "random" targeting must not re-roll mid-attack
		if t := target; t != nil {
			if t.CombatState.Dodging {
				monsterAdvantage = -1
			}
			playerConds = t.CombatState.Conditions
			// Terrain: cover raises your AC, high ground helps its arrows, and a
			// non-swimmer fights badly in deep water.
			_, cover := Sight(cs, monster.Pos, t.Pos)
			if cover > 0 {
				playerAC += cover
				logEntries = append(logEntries, fmt.Sprintf("  🛡 You have %s (+%d AC).", coverLabel(cover), cover))
			}
			if IsRangedAction(action.Type) {
				attackBonus += highGroundBonus(cs, monster.Pos, t.Pos)
			} else if inDeepWater(cs, monster.Pos) && monster.Data.Speed.Swim == 0 {
				monsterAdvantage--
			}
		}
		// Conditions: the monster's own (poisoned/frightened/…) impose disadvantage;
		// the player's (prone/restrained/…) grant the monster advantage.
		monsterAdvantage += ConditionAttackAdvantage(monster.Conditions, playerConds)
		// Sunlight Sensitivity: the creatures of the dark fight badly by day.
		if sunlightHampers(cs, monster, save) {
			monsterAdvantage--
			logEntries = append(logEntries, fmt.Sprintf("  ☀️ %s squints in the sunlight — disadvantage.", monster.Data.Name))
		}
		// Spells: the target's wards (blur, protection from evil, sanctuary) and
		// what's been cast on the monster (bane, a compelled duel).
		if target != nil {
			adv, penalty, lost, defLog := attackDefense(cs, target, monster, IsRangedAction(action.Type))
			logEntries = append(logEntries, defLog...)
			if lost {
				return 0, logEntries
			}
			monsterAdvantage += adv
			attackBonus -= penalty
		}
		result := ResolveAttackRoll(attackBonus, playerAC, monsterAdvantage)

		logEntries = append(logEntries,
			fmt.Sprintf(
				"  %s attacks with %s: rolled %d%s",
				monster.Name, action.Name,
				result.Roll, formatModifier(attackBonus),
			),
			outcomeLine(result),
		)

		// A hit can still be turned aside: an illusory double takes it, or a
		// prepared shield spell lifts AC past the roll.
		if result.IsHit && target != nil {
			if deflected, defLog := deflectHit(cs, target, save, monster, result, playerAC); deflected {
				logEntries = append(logEntries, defLog...)
				break
			}
		}

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
			critStr := ""
			if result.IsCrit {
				critStr = " CRITICAL HIT!"
			}
			logEntries = append(logEntries, fmt.Sprintf(
				"  %s deals %d %s damage.%s",
				monster.Name, dmg, action.Hit.Type, critStr,
			))
			if target != nil {
				var resistLog []string
				dmg, resistLog = resistDamage(&target.CombatState, action.Hit.Type, dmg)
				logEntries = append(logEntries, resistLog...)
			}
			damageDealt = dmg
			// On-hit condition rider: the player saves or gains the condition.
			logEntries = append(logEntries, applyMonsterConditionRider(cs, save, action)...)
			// Armor of agathys: a melee attacker that hits you pays for it.
			if target != nil && !IsRangedAction(action.Type) {
				logEntries = append(logEntries, retaliate(cs, target, save, monster)...)
			}
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

package combat

import (
	"database/sql"
	"fmt"
	"strings"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
)

// ─── Turn order (M5.6 P1) ────────────────────────────────────────────────────
//
// cs.Initiative is the full turn order; cs.CurrentTurnIndex points at whoever is
// acting. The player acts through the HTTP endpoints; ending the turn walks the
// cursor forward, running each monster's turn in order, and parks it back on the
// player. Round counts full passes through the order (it increments when the
// cursor wraps), not individual actions.
//
// Reactions refresh at the start of the reacting creature's own turn, so the
// player gets one opportunity attack per round no matter how many monsters
// stream past, and each monster gets one.

// playerTurnIndex returns the player's position in the initiative order.
func playerTurnIndex(cs *types.CombatSession) int {
	for i, e := range cs.Initiative {
		if e.Type == "player" {
			return i
		}
	}
	return 0
}

// advanceCursor moves to the next entry in the order, starting a new round on wrap.
func advanceCursor(cs *types.CombatSession) {
	cs.CurrentTurnIndex++
	if cs.CurrentTurnIndex >= len(cs.Initiative) {
		cs.CurrentTurnIndex = 0
		cs.Round++
	}
}

// IsPlayerTurn reports whether the cursor is on the player's entry.
func IsPlayerTurn(cs *types.CombatSession) bool {
	return len(cs.Initiative) > 0 && cs.Initiative[cs.CurrentTurnIndex].Type == "player"
}

// startPlayerTurn resets the player's per-turn economy as their turn begins.
func startPlayerTurn(cs *types.CombatSession, save *types.SaveFile) {
	if len(cs.Party) == 0 {
		return
	}
	state := &actor(cs).CombatState
	state.ActionUsed = false
	state.BonusActionUsed = false
	state.MovementSpent = 0
	// Spells add movement (longstrider, expeditious retreat); being held fast
	// (restrained, grappled) takes it all away.
	tickBuffs(state)
	state.MovementBudget = playerMovementBudget(save.Race) + effects.ModifierTotal(save, "combat_move") + buffMovement(state)
	if IsSpeedZero(state.Conditions) {
		state.MovementBudget = 0
	}
	state.Dodging = false
	state.HeldPosition = false
	state.ReactionUsed = false
	state.Disengaged = false
	// Extra actions and a readied-but-unused sneak attack don't carry over.
	// (Rage persists — it has its own duration countdown in tickPlayerAbilities.)
	state.ExtraActions = 0
	state.PendingSneakDice = ""
	startTurnTimer(cs)
}

// endPlayerTurn resolves the end of the player's turn: their conditions
// save-to-end / count down, the class resource regenerates and rage ticks.
// Dodging and a readied stance stay up through the monsters' turns and drop
// when the player's next turn starts.
func endPlayerTurn(cs *types.CombatSession, save *types.SaveFile) []string {
	if len(cs.Party) == 0 {
		return nil
	}
	state := &actor(cs).CombatState
	state.Ambushing = false
	log := TickCreatureConditions(whoName(cs, actor(cs)), &state.Conditions,
		func(stat string) int { return playerSaveTotal(state, save, stat) })
	log = append(log, tickPlayerAbilities(state)...)
	// Ending a turn in a hazard burns again.
	return append(log, playerHazard(cs, save, actor(cs).Pos, "are caught in")...)
}

// playerHazard applies a hazard cell's damage to the player (verb: "step into",
// "are caught in"). No-op on safe ground.
func playerHazard(cs *types.CombatSession, save *types.SaveFile, pos types.Position, verb string) []string {
	dice, dtype, name := hazardAt(cs, pos)
	if dice == "" {
		return nil
	}
	dmg := RollDice(dice, false)
	if dmg < 1 {
		dmg = 1
	}
	who := whoName(cs, actor(cs))
	if who != "You" {
		verb = strings.Replace(strings.Replace(verb, "are caught", "is caught", 1), "step into", "steps into", 1)
	}
	log := []string{fmt.Sprintf("  🔥 %s %s the %s: %d %s damage.", who, verb, strings.ToLower(name), dmg, dtype)}
	return append(log, damagePlayer(cs, save, dmg)...)
}

// monsterHazard applies a hazard cell's damage to a monster. A hazard kill
// still counts as the player's kill (loot, quest progress, kill bonus).
func monsterHazard(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, m *types.MonsterInstance, pos types.Position) []string {
	dice, dtype, name := hazardAt(cs, pos)
	if dice == "" || !m.Active() {
		return nil
	}
	dmg := RollDice(dice, false)
	if dmg < 1 {
		dmg = 1
	}
	applyDamageToMonster(m, dmg)
	log := []string{fmt.Sprintf("  🔥 %s is burned by the %s: %d %s damage.", m.Name, strings.ToLower(name), dmg, dtype)}
	if !m.IsAlive {
		advancement, _ := character.LoadAdvancement(db)
		log = append(log, handleMonsterKill(cs, m, save, advancement)...)
	}
	return log
}

// ProcessEndTurn finalises the player's turn and runs every monster's turn, in
// initiative order, until it is the player's turn again (or the fight ends).
func ProcessEndTurn(db *sql.DB, cs *types.CombatSession, save *types.SaveFile) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot end turn: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	log := endPlayerTurn(cs, save)
	return append(log, runUntilPlayerTurn(db, cs, save, false)...), nil
}

// runOpeningTurns runs the monsters that won initiative before the player has
// had a turn. Each records where it spawned so the client can animate its
// opening step.
func runOpeningTurns(db *sql.DB, cs *types.CombatSession, save *types.SaveFile) []string {
	cs.CurrentTurnIndex = -1
	return runUntilPlayerTurn(db, cs, save, true)
}

// runUntilPlayerTurn advances the cursor, running each active monster's turn,
// until it reaches the player (whose turn then starts). If the player drops or
// the fight ends partway, the remaining monsters forfeit and the cursor parks
// on the player — death saves and the end screen take it from there.
func runUntilPlayerTurn(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, opening bool) []string {
	var log []string
	for i := 0; i < len(cs.Initiative); i++ {
		advanceCursor(cs)
		e := cs.Initiative[cs.CurrentTurnIndex]
		if e.Type == "player" {
			idx := MemberIndex(cs, e.ID)
			if idx < 0 || cs.Party[idx].Fled {
				continue
			}
			p := &cs.Party[idx]
			// In a party fight a downed member's turn passes: they lie there
			// until someone heals or helps them up (or the fight ends).
			if PartyFight(cs) && p.CombatState.IsUnconscious {
				continue
			}
			cs.Actor = idx
			startPlayerTurn(cs, saveOf(p, save))
			return log
		}
		m := FindMonster(cs, e.ID)
		if m == nil || !m.Active() {
			continue
		}
		if opening {
			spawn := m.Pos
			m.SpawnPos = &spawn
		}
		log = append(log, runMonsterTurn(db, cs, save, m, opening)...)
		if cs.Phase != "active" {
			cs.CurrentTurnIndex = playerTurnIndex(cs)
			return log
		}
	}
	cs.CurrentTurnIndex = playerTurnIndex(cs)
	return log
}

// runMonsterTurn runs one monster's full turn: move, then act.
//
// opening: the monster won initiative at combat start, before the player chose a
// stance — the player gets a reflex save against its hit (unless surprised), and
// no opportunity attacks or readied counters apply.
//
// Otherwise, if the player braced (Hold) and this monster steps into melee
// reach, the readied counter-attack fires before it can swing.
func runMonsterTurn(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, m *types.MonsterInstance, opening bool) []string {
	// A creature's reaction comes back at the start of its own turn.
	m.ReactionUsed = false
	m.Disengaged = false

	// Everything below acts against the member this monster goes after: their
	// AC, their reactions, their HP.
	if t := aiTarget(cs, m); t != nil {
		cs.Actor = MemberIndex(cs, t.ID)
		save = saveOf(t, save)
	}

	var log []string
	// Burning (searing smite, acid arrow) hurts at the start of its turn.
	if log = conditionDamage(cs, save, m); !m.IsAlive {
		return log
	}
	// Caught by a sneak attack: it spends its first turn getting its bearings.
	if m.Surprised {
		m.Surprised = false
		return append(log, fmt.Sprintf("  %s is caught off guard and can't act this turn.", m.Name))
	}
	if IsIncapacitated(m.Conditions) {
		log = append(log, fmt.Sprintf("  %s is %s and can't act.", m.Name, incapacitatingConditionName(m.Conditions)))
		return append(log, tickMonsterConditions(m)...)
	}

	playerAC := computePlayerAC(db, cs, save)

	if opening {
		dexMod := StatMod(GetStatFromMap(effectiveStats(save), "dexterity"))
		dmg, turnLog := ExecuteMonsterTurn(cs, m, playerAC, !cs.IsSurprised, dexMod, save)
		log = append(log, turnLog...)
		log = append(log, damagePlayer(cs, save, dmg)...)
		log = append(log, monsterHazard(db, cs, save, m, m.Pos)...)
		if !m.Active() {
			return log
		}
		return append(log, tickMonsterConditions(m)...)
	}

	state := &actor(cs).CombatState
	decision := DecideMonsterAction(cs, m)
	playerReach := getPlayerMeleeReach(db, save)

	// Starting adjacent and trying to retreat? Use Disengage (consumes its
	// action, but avoids the player's OA).
	if decision.Action == "retreat" && rangeTo(cs, m) <= playerReach {
		m.Disengaged = true
		log = append(log, fmt.Sprintf("  %s disengages and breaks off.", m.Name))
	}

	oaTrigger := func() []string {
		return executePlayerOA(cs, save, db, m)
	}
	// No OA if the monster disengaged, the player has no reach, is down, or
	// already reacted this round.
	if m.Disengaged || playerReach <= 0 || state.ReactionUsed || state.IsUnconscious {
		oaTrigger = nil
	}

	log = append(log, ApplyMonsterMove(cs, m, decision, playerReach, oaTrigger)...)
	if !m.IsAlive {
		return log // the opportunity attack finished it
	}

	// Readied stance: the first monster to step into reach eats the counter.
	if state.HeldPosition && decision.Move == -1 && rangeTo(cs, m) <= playerReach {
		log = append(log, fmt.Sprintf("  Your readied stance pays off — you strike as %s steps in!", m.Name))
		counterLog, killed := executeReadiedAttack(db, cs, save, m)
		log = append(log, counterLog...)
		state.HeldPosition = false
		if killed {
			return log
		}
	}

	// Re-pick the attack based on the actual post-move range (the monster may have
	// closed enough to make a reach check succeed, or moved out of its original band).
	decision = RefreshAttackDecision(cs, m, decision)

	dmg, actionLog := ApplyMonsterAction(cs, m, decision, playerAC, false, 0, save)
	log = append(log, actionLog...)
	log = append(log, damagePlayer(cs, save, dmg)...)
	if dmg > 0 && cs.Phase == "active" {
		log = append(log, tryDamagedReaction(cs, actor(cs), save, m)...)
		if !m.IsAlive {
			return log
		}
	}
	// Ending its turn in a hazard burns it.
	log = append(log, monsterHazard(db, cs, save, m, m.Pos)...)

	if m.Active() {
		log = append(log, tickMonsterConditions(m)...)
	}
	return log
}

// damagePlayer applies a monster's damage to the player and checks concentration.
func damagePlayer(cs *types.CombatSession, save *types.SaveFile, dmg int) []string {
	if dmg <= 0 {
		return nil
	}
	log := applyDamageToPlayer(cs, dmg)
	return append(log, checkConcentrationOnDamage(cs, save, dmg)...)
}

// tickMonsterConditions resolves the end of a monster's turn: it rolls saves to
// shake off conditions (restrained/stunned/…) and timed conditions count down
// and expire — D&D resolves both at the end of the afflicted creature's turn.
func tickMonsterConditions(m *types.MonsterInstance) []string {
	return TickCreatureConditions(m.Name, &m.Conditions,
		func(stat string) int { return monsterSaveTotal(m, stat) })
}

// ─── Death saves ─────────────────────────────────────────────────────────────

// ProcessDeathSave rolls the downed player's death saving throw (their turn),
// then each monster takes its turn in initiative order. Only relentless
// monsters (behavior.relentless — a dragon, a wolf pack's hunger) attack a
// downed player; the rest stop, and a predator that's been badly hurt walks
// off (runMonsterDeathSaveTurn). Three successes, or a natural 20, puts the
// player back on their feet at 1 HP.
// Returns log entries for this round. Caller appends them to cs.Log.
func ProcessDeathSave(cs *types.CombatSession, save *types.SaveFile) []string {
	if len(cs.Party) == 0 || cs.Phase != "death_saves" {
		return nil
	}
	cs.CurrentTurnIndex = playerTurnIndex(cs)

	roll := RollD20()
	log := []string{fmt.Sprintf("  Death saving throw: rolled %d.", roll)}
	log = append(log, resolveDeathSaveRoll(&actor(cs).CombatState, cs, roll))

	switch cs.Phase {
	case "active":
		startPlayerTurn(cs, save) // natural 20 — back on your feet, your turn
	case "death_saves":
		for i := 0; i < len(cs.Initiative); i++ {
			advanceCursor(cs)
			e := cs.Initiative[cs.CurrentTurnIndex]
			if e.Type == "player" {
				break
			}
			m := FindMonster(cs, e.ID)
			if m == nil || !m.Active() {
				continue
			}
			log = append(log, runMonsterDeathSaveTurn(cs, save, m)...)
			if cs.Phase != "death_saves" {
				break
			}
		}
		cs.CurrentTurnIndex = playerTurnIndex(cs)
	}
	return log
}

// runMonsterDeathSaveTurn runs one monster's attack against the unconscious
// player. Hits apply death save failures rather than HP damage. A monster that
// wants out slips away instead; once they all have, the player is safe.
func runMonsterDeathSaveTurn(cs *types.CombatSession, save *types.SaveFile, m *types.MonsterInstance) []string {
	m.ReactionUsed = false
	if IsIncapacitated(m.Conditions) {
		return tickMonsterConditions(m)
	}

	// Not a killer: it stops attacking someone who's down. A hurt predator
	// (or anything past its flee threshold) takes the chance to leave; the
	// rest stand over you and wait to see if you get up.
	if !m.Data.Behavior.Relentless {
		if leavesTheDowned(m) {
			return leaveDownedPlayer(cs, m, fmt.Sprintf("  %s backs off and limps away, leaving you where you fell.", m.Name))
		}
		if !m.WaitingOnDowned {
			m.WaitingOnDowned = true
			return append([]string{fmt.Sprintf("  %s stands over you, watching.", m.Name)}, tickMonsterConditions(m)...)
		}
		return tickMonsterConditions(m)
	}

	decision := DecideMonsterAction(cs, m)

	if decision.Action == "retreat" || decision.Action == "escape" {
		return leaveDownedPlayer(cs, m, fmt.Sprintf("  %s disengages and slips away.", m.Name))
	}
	if decision.Action != "attack" {
		return nil
	}

	action := m.Data.Actions[decision.ActionIndex]
	playerAC := 10 + StatMod(GetStatFromMap(effectiveStats(save), "dexterity"))
	result := ResolveAttackRoll(action.AttackBonus, playerAC, 1) // advantage: the target is helpless

	log := []string{
		fmt.Sprintf("  %s attacks: rolled %d%s",
			m.Name, result.Roll, formatModifier(action.AttackBonus)),
		outcomeLine(result),
	}
	if result.IsHit {
		log = append(log, applyDeathSaveHit(cs, result.IsCrit))
	}
	return append(log, tickMonsterConditions(m)...)
}

// leavesTheDowned reports whether a monster walks away from a downed player
// instead of waiting: past its flee threshold, or a beast/monstrosity hurt to
// half its HP or worse — a predator that's had enough.
func leavesTheDowned(m *types.MonsterInstance) bool {
	frac := float64(m.CurrentHP) / float64(max(m.MaxHP, 1))
	if m.Data.Behavior.Aggression == "berserker" {
		return false
	}
	if frac <= m.Data.Behavior.FleeThreshold {
		return true
	}
	switch strings.ToLower(m.Data.Type) {
	case "beast", "monstrosity":
		return frac <= 0.5
	}
	return false
}

// leaveDownedPlayer takes a monster out of the fight while the player is down.
// When the last one goes, the player comes to on their own with 1 HP.
func leaveDownedPlayer(cs *types.CombatSession, m *types.MonsterInstance, line string) []string {
	m.Fled = true
	log := []string{line}
	if v := checkVictory(cs); v != nil {
		reviveAtOneHP(&actor(cs).CombatState)
		log = append(log, "  You are alone. After a while you come to, hurt but alive (1 HP).")
	}
	return log
}

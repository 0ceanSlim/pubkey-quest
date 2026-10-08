package combat

import (
	"database/sql"
	"fmt"

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
	state := &cs.Party[0].CombatState
	state.ActionUsed = false
	state.BonusActionUsed = false
	state.MovementSpent = 0
	state.MovementBudget = playerMovementBudget(save.Race)
	state.Dodging = false
	state.HeldPosition = false
	state.ReactionUsed = false
	state.Disengaged = false
	// Extra actions and a readied-but-unused sneak attack don't carry over.
	// (Rage persists — it has its own duration countdown in tickPlayerAbilities.)
	state.ExtraActions = 0
	state.PendingSneakDice = ""
}

// endPlayerTurn resolves the end of the player's turn: their conditions
// save-to-end / count down, the class resource regenerates and rage ticks.
// Dodging and a readied stance stay up through the monsters' turns and drop
// when the player's next turn starts.
func endPlayerTurn(cs *types.CombatSession, save *types.SaveFile) []string {
	if len(cs.Party) == 0 {
		return nil
	}
	state := &cs.Party[0].CombatState
	log := TickCreatureConditions("You", &state.Conditions,
		func(stat string) int { return playerSaveTotal(save, stat) })
	return append(log, tickPlayerAbilities(state)...)
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
			startPlayerTurn(cs, save)
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

	var log []string
	if IsIncapacitated(m.Conditions) {
		log = append(log, fmt.Sprintf("  %s is %s and can't act.", m.Name, incapacitatingConditionName(m.Conditions)))
		return append(log, tickMonsterConditions(m)...)
	}

	playerAC := computePlayerAC(db, save)

	if opening {
		dexMod := StatMod(GetStatFromMap(effectiveStats(save), "dexterity"))
		dmg, turnLog := ExecuteMonsterTurn(cs, m, playerAC, !cs.IsSurprised, dexMod, save)
		log = append(log, turnLog...)
		log = append(log, damagePlayer(cs, save, dmg)...)
		return append(log, tickMonsterConditions(m)...)
	}

	state := &cs.Party[0].CombatState
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
// then every monster in range takes a swing at them in initiative order.
// Returns log entries for this round. Caller appends them to cs.Log.
func ProcessDeathSave(cs *types.CombatSession, save *types.SaveFile) []string {
	if len(cs.Party) == 0 || cs.Phase != "death_saves" {
		return nil
	}
	cs.CurrentTurnIndex = playerTurnIndex(cs)

	roll := RollD20()
	log := []string{fmt.Sprintf("  Death saving throw: rolled %d.", roll)}
	log = append(log, resolveDeathSaveRoll(&cs.Party[0].CombatState, cs, roll))

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
	decision := DecideMonsterAction(cs, m)

	if decision.Action == "retreat" || decision.Action == "escape" {
		m.Fled = true
		log := []string{fmt.Sprintf("  %s disengages and slips away.", m.Name)}
		if v := checkVictory(cs); v != nil {
			log = append(log, "  You are safe.")
		}
		return log
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

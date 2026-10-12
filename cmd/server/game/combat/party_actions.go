package combat

import (
	"fmt"

	"pubkey-quest/types"
)

// ─── Things allies do for each other (party-combat-plan §1) ──────────────────

// partyAfterDeparture settles the fight after a member leaves it (fled): if
// nobody is left standing, the downed are overrun; if nobody is left at all,
// the fight just ends.
func partyAfterDeparture(cs *types.CombatSession) []string {
	if len(standingMembers(cs)) > 0 {
		return nil
	}
	if len(inFightMembers(cs)) > 0 {
		cs.Phase = "defeat"
		return []string{"  With no one left standing, the fallen are overrun…"}
	}
	cs.Phase = "loot"
	return []string{"  The party has scattered."}
}

// ProcessPartyRetreat is the leader calling a retreat: one roll for the whole
// party, made at the slowest standing member's pace and from the closest
// anyone is to an enemy. Success ends the fight for everyone (the downed are
// carried off, nothing is looted). Failure spends the leader's action.
func ProcessPartyRetreat(cs *types.CombatSession, save *types.SaveFile) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot retreat: combat phase is %q", cs.Phase)
	}
	foes := activeMonsters(cs)
	if len(foes) == 0 {
		return nil, fmt.Errorf("no living enemy to retreat from")
	}
	standing := standingMembers(cs)
	closest := noTargetRange
	for _, p := range standing {
		for _, m := range foes {
			if r := chebyshev(p.Pos, m.Pos); r < closest {
				closest = r
			}
		}
	}
	if closest < 3 {
		return nil, fmt.Errorf("too close to retreat — everyone needs every enemy at range 3 or more")
	}

	monsterAth, relentless := 0.0, false
	var pursuer *types.MonsterInstance
	for _, m := range foes {
		if a := athleticsScore(m.Data.Stats.Strength, m.Data.Stats.Constitution, m.Data.Stats.Dexterity); a > monsterAth || pursuer == nil {
			pursuer, monsterAth = m, a
		}
		relentless = relentless || m.Data.Behavior.Relentless
	}
	slowest := -1.0
	for _, p := range standing {
		st := effectiveStats(saveOf(p, save))
		a := athleticsScore(GetStatFromMap(st, "strength"), GetStatFromMap(st, "constitution"), GetStatFromMap(st, "dexterity"))
		if slowest < 0 || a < slowest {
			slowest = a
		}
	}

	chance := float64(closest-2)*0.25 + (slowest-monsterAth)*0.03
	if chance > 0.90 {
		chance = 0.90
	}
	if relentless {
		chance -= 0.20
	}
	if chance < 0.05 {
		chance = 0.05
	}
	pct := int(chance * 100)
	roll := RollRange(1, 100)
	log := []string{fmt.Sprintf("  📯 %s calls a retreat! Escape chance: %d%% (rolled %d).", actor(cs).Name, pct, roll)}
	if roll <= pct {
		cs.Phase = "loot"
		cs.Retreated = true
		for i := range cs.Party {
			cs.Party[i].Loot = nil
		}
		return append(log, "  The party falls back together, carrying the wounded — you got away!"), nil
	}
	actor(cs).CombatState.ActionUsed = true
	return append(log, fmt.Sprintf("  %s cuts off the retreat! You're still in the fight.", pursuer.Name)), nil
}

// ProcessHelp is the Help action on an adjacent ally: a downed ally is hauled
// back to their feet at 1 HP; a standing one gets advantage on their next
// attack. Uses your action.
func ProcessHelp(cs *types.CombatSession, allyID string) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot help: combat phase is %q", cs.Phase)
	}
	me := actor(cs)
	if me.CombatState.ActionUsed {
		return nil, fmt.Errorf("action already used this turn")
	}
	ally := FindMember(cs, allyID)
	if ally == nil || ally == me || ally.Fled {
		return nil, fmt.Errorf("no such ally here")
	}
	if chebyshev(me.Pos, ally.Pos) > 1 {
		return nil, fmt.Errorf("move next to %s first", ally.Name)
	}
	me.CombatState.ActionUsed = true
	if ally.CombatState.IsUnconscious {
		ally.CombatState.IsUnconscious = false
		ally.CombatState.CurrentHP = 1
		ally.CombatState.DeathSaveFailures, ally.CombatState.DeathSaveSuccesses = 0, 0
		return []string{fmt.Sprintf("  🤝 %s hauls %s back to their feet (1 HP).", me.Name, ally.Name)}, nil
	}
	ally.CombatState.Helped = true
	return []string{fmt.Sprintf("  🤝 %s helps %s — their next attack has advantage.", me.Name, ally.Name)}, nil
}

// flanking reports whether a standing ally is next to the target on the far
// side from the attacker (both adjacent to it, on opposite sides) — melee
// advantage.
func flanking(cs *types.CombatSession, attacker *types.PartyCombatant, target *types.MonsterInstance) bool {
	if chebyshev(attacker.Pos, target.Pos) != 1 {
		return false
	}
	ax, ay := attacker.Pos.X-target.Pos.X, attacker.Pos.Y-target.Pos.Y
	for _, p := range standingMembers(cs) {
		if p == attacker || chebyshev(p.Pos, target.Pos) != 1 {
			continue
		}
		bx, by := p.Pos.X-target.Pos.X, p.Pos.Y-target.Pos.Y
		if ax*bx+ay*by < 0 { // pointing away from each other across the target
			return true
		}
	}
	return false
}

// HealMember restores HP to a party member in combat, waking them if they were
// down, and pays the healer XP for it. Returns the HP actually restored.
func HealMember(cs *types.CombatSession, healer, target *types.PartyCombatant, amount, timeOfDay, level int, advancement []types.AdvancementEntry) (int, []string) {
	st := &target.CombatState
	before := st.CurrentHP
	st.CurrentHP += amount
	if st.CurrentHP > st.MaxHP {
		st.CurrentHP = st.MaxHP
	}
	healed := st.CurrentHP - before
	var log []string
	if st.IsUnconscious && st.CurrentHP > 0 {
		st.IsUnconscious = false
		st.DeathSaveFailures, st.DeathSaveSuccesses = 0, 0
		log = append(log, fmt.Sprintf("  ✨ %s is back on their feet!", target.Name))
	}
	if xp := awardHealXP(cs, healer, target, healed, timeOfDay, level, advancement); xp > 0 {
		log = append(log, fmt.Sprintf("  +%d XP (healing)", xp))
	}
	return healed, log
}

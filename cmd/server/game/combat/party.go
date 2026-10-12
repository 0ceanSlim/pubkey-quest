package combat

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"pubkey-quest/types"
)

// ─── Party fights (party-combat-plan, P5) ────────────────────────────────────
//
// A fight can hold several players. Every request acts as one of them — the
// actor (cs.Actor). Engine code that used to mean "the player" means "the
// acting member"; during a monster's turn the actor is that monster's target,
// so damage, AC, reactions and death all land on whoever it attacked.

// TurnSeconds is how long a party member has to act before they Dodge.
const TurnSeconds = 45

// actor is the party member the current request (or monster attack) acts as.
func actor(cs *types.CombatSession) *types.PartyCombatant {
	if cs.Actor < 0 || cs.Actor >= len(cs.Party) {
		cs.Actor = 0
	}
	return &cs.Party[cs.Actor]
}

// Actor is the exported form of actor.
func Actor(cs *types.CombatSession) *types.PartyCombatant { return actor(cs) }

// PartyFight reports whether more than one player is in the fight.
func PartyFight(cs *types.CombatSession) bool { return len(cs.Party) > 1 }

// MemberIndex returns a member's index by id, or -1.
func MemberIndex(cs *types.CombatSession, id string) int {
	for i := range cs.Party {
		if cs.Party[i].ID == id {
			return i
		}
	}
	return -1
}

// FindMember returns the member with that id, or nil.
func FindMember(cs *types.CombatSession, id string) *types.PartyCombatant {
	if i := MemberIndex(cs, id); i >= 0 {
		return &cs.Party[i]
	}
	return nil
}

// SetActor makes the member with that id the actor. False if they're not in it.
func SetActor(cs *types.CombatSession, id string) bool {
	i := MemberIndex(cs, id)
	if i < 0 {
		return false
	}
	cs.Actor = i
	return true
}

// MemberSave resolves a party member's live save (registered by the API layer,
// which owns the sessions). Nil in tests / solo-only use.
var MemberSave func(p *types.PartyCombatant) *types.SaveFile

// saveOf returns a member's save, or fallback when it can't be resolved.
func saveOf(p *types.PartyCombatant, fallback *types.SaveFile) *types.SaveFile {
	if MemberSave != nil && p != nil {
		if s := MemberSave(p); s != nil {
			return s
		}
	}
	return fallback
}

// standingMembers lists members still up and in the fight.
func standingMembers(cs *types.CombatSession) []*types.PartyCombatant {
	var out []*types.PartyCombatant
	for i := range cs.Party {
		if cs.Party[i].Standing() {
			out = append(out, &cs.Party[i])
		}
	}
	return out
}

// inFightMembers lists members who haven't fled (standing or down).
func inFightMembers(cs *types.CombatSession) []*types.PartyCombatant {
	var out []*types.PartyCombatant
	for i := range cs.Party {
		if !cs.Party[i].Fled {
			out = append(out, &cs.Party[i])
		}
	}
	return out
}

// whoName is how the log names a member: "You" in a solo fight, the
// character's name in a party fight.
func whoName(cs *types.CombatSession, p *types.PartyCombatant) string {
	if !PartyFight(cs) || p.Name == "" {
		return "You"
	}
	return p.Name
}

// CurrentMember is whoever's turn it is, or nil on a monster's turn.
func CurrentMember(cs *types.CombatSession) *types.PartyCombatant {
	if len(cs.Initiative) == 0 || cs.CurrentTurnIndex < 0 || cs.CurrentTurnIndex >= len(cs.Initiative) {
		return nil
	}
	e := cs.Initiative[cs.CurrentTurnIndex]
	if e.Type != "player" {
		return nil
	}
	return FindMember(cs, e.ID)
}

// IsMemberTurn reports whether it's that member's turn.
func IsMemberTurn(cs *types.CombatSession, id string) bool {
	m := CurrentMember(cs)
	return m != nil && m.ID == id
}

// ─── One lock per fight ──────────────────────────────────────────────────────

var fightLocks sync.Map // *types.CombatSession → *sync.Mutex

// LockFight serializes every request against one fight (several players'
// requests share it). Returns the unlock func.
func LockFight(cs *types.CombatSession) func() {
	v, _ := fightLocks.LoadOrStore(cs, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// ForgetFight drops a finished fight's lock.
func ForgetFight(cs *types.CombatSession) { fightLocks.Delete(cs) }

// ─── Turn timer ──────────────────────────────────────────────────────────────

// startTurnTimer arms the timer for the member whose turn just began.
func startTurnTimer(cs *types.CombatSession) {
	if !PartyFight(cs) {
		cs.TurnDeadline = 0
		return
	}
	cs.TurnDeadline = time.Now().Add(TurnSeconds * time.Second).UnixMilli()
}

// MemberAway reports whether a member has stopped answering (closed the game).
// Registered by the API layer from presence; nil means never.
var MemberAway func(p *types.PartyCombatant) bool

// ExpireTurn ends the current member's turn if their time ran out (or they're
// gone): they take the Dodge action and play moves on. Returns the log. Safe
// to call on every request.
func ExpireTurn(db *sql.DB, cs *types.CombatSession, now time.Time) []string {
	var log []string
	for guard := 0; guard < len(cs.Party)+1; guard++ {
		if cs.Phase != "active" || !PartyFight(cs) || cs.TurnDeadline == 0 {
			return log
		}
		m := CurrentMember(cs)
		if m == nil {
			return log
		}
		away := MemberAway != nil && MemberAway(m)
		if !away && now.UnixMilli() < cs.TurnDeadline {
			return log
		}
		cs.Actor = MemberIndex(cs, m.ID)
		m.CombatState.Dodging = true
		if away {
			log = append(log, fmt.Sprintf("  💤 %s isn't here — they take the Dodge action.", m.Name))
		} else {
			log = append(log, fmt.Sprintf("  ⏱ %s hesitates and takes the Dodge action.", m.Name))
		}
		save := saveOf(m, nil)
		if save == nil {
			return log
		}
		log = append(log, endPlayerTurn(cs, save)...)
		log = append(log, runUntilPlayerTurn(db, cs, save, false)...)
	}
	return log
}

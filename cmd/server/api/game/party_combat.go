package game

import (
	"math/rand"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/social"
	"pubkey-quest/types"
)

// ─── Starting a fight for the whole party (party-combat-plan P5) ─────────────
//
// When the leader runs into a fight, everyone in the party standing with them
// drops into it: one shared CombatSession, pointed at by every member's
// session. Followers' clients notice through the social poll (in_combat).

// fightMembers is the leader plus every party member standing with them who
// isn't busy (in another fight or waiting on an unaware choice).
func fightMembers(leader *session.GameSession) []*session.GameSession {
	out := []*session.GameSession{leader}
	p := social.PartyOf(leader.Npub)
	if p == nil || p.Leader().Npub != leader.Npub {
		return out
	}
	for _, m := range p.Members[1:] {
		s, err := session.GetSessionManager().GetSession(m.Npub, m.SaveID)
		if err != nil || s.ActiveCombat != nil || s.PendingEncounter != nil {
			continue
		}
		if !sameSpot(leader.Npub, m.Npub) {
			continue
		}
		out = append(out, s)
	}
	return out
}

// scaleForParty grows an encounter for a bigger party (party-combat-plan §1):
// a lone monster may become an elite (+HP, +1 to hit per extra player);
// otherwise one more of the group's monsters joins per extra player.
func scaleForParty(spec *combat.EncounterSpec, players int) {
	extra := players - 1
	if extra <= 0 || len(spec.MonsterIDs) == 0 {
		return
	}
	if len(spec.MonsterIDs) == 1 && rand.Intn(2) == 0 {
		spec.Elite = extra
		return
	}
	for i := 0; i < extra; i++ {
		spec.MonsterIDs = append(spec.MonsterIDs, spec.MonsterIDs[rand.Intn(len(spec.MonsterIDs))])
	}
}

// startFight starts an encounter for the leader and whoever stands with them,
// scaling it to the party when scale is set, and hands every member the fight.
func startFight(leader *session.GameSession, spec combat.EncounterSpec, advancement []types.AdvancementEntry, scale bool) (*types.CombatSession, error) {
	members := fightMembers(leader)
	if scale {
		scaleForParty(&spec, len(members))
	}
	fms := make([]combat.FightMember, 0, len(members))
	for _, s := range members {
		fms = append(fms, combat.FightMember{Npub: s.Npub, SaveID: s.SaveID, Save: &s.SaveData})
	}
	cs, err := combat.StartPartyEncounter(serverdb.GetDB(), fms, spec, advancement)
	if err != nil {
		return nil, err
	}
	for _, s := range members {
		s.ActiveCombat = cs
		s.PendingEncounter = nil
	}
	if len(members) > 1 {
		names := ""
		for i, s := range members {
			if i > 0 {
				names += ", "
			}
			names += s.SaveData.D
		}
		cs.Log = append([]string{"⚔️ The party fights together: " + names + "."}, cs.Log...)
	}
	return cs, nil
}

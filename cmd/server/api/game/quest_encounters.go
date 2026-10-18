package game

import (
	"log"
	"math/rand"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/social"
	"pubkey-quest/types"
)

// ─── Quest-stage encounters ──────────────────────────────────────────────────
//
// A quest stage can put its monsters somewhere in the world (types.QuestStage
// Encounter): "the tavern cellar is overrun with rats". While the stage is
// active and the player stands in that place, a fight starts with as many of
// the monster as the stage's slay objective still needs, up to the encounter's
// per-fight Count. Kills feed the objective through the normal event recorder,
// so a stage needing more than one fight comes in waves.
//
// Places in town (a cellar, a district) start their fight the moment you're
// there. Roads are different: a stage's monsters "waiting on the Cragspire
// road" find you while you travel it, rolled each travel tick like any road
// encounter (only likelier), never the instant you load in or while you're
// stopped.

// roadQuestChancePerMinute is the per-minute odds that a road quest fight finds
// a moving traveller: about one every 50 in-game minutes, so it arrives a short
// way into the trip. Capped per tick like road encounters.
const (
	roadQuestChancePerMinute = 0.02
	roadQuestMaxTickChance   = 0.5
)

// questRoll is the road quest fight's dice (swapped in tests).
var questRoll = rand.Float64

// questEncounter is a stage encounter the player is standing in, with what's owed.
type questEncounter struct {
	quest *types.QuestData
	enc   *types.QuestStageEncounter
	owed  int // kills of enc.MonsterID the stage still needs
}

// maybeStartQuestEncounter starts the fight for an active quest stage whose
// encounter is at the player's current place. Runs after every action (next to
// the authored-encounter triggers); a no-op while a fight or POI walk is open.
func maybeStartQuestEncounter(sess *session.GameSession, response *types.GameActionResponse) {
	// Followers don't trigger fights; a leader's quest fight brings the party.
	if sess.ActiveCombat != nil || sess.ActivePOI != nil || sess.PendingEncounter != nil || social.IsFollower(sess.Npub) {
		return
	}
	// Arriving somewhere new resets the "already fired here" memory, so leaving
	// and coming back is how you retry after fleeing.
	if sess.QuestEncounterPlace != sess.PlaceKey {
		sess.QuestEncounterPlace = sess.PlaceKey
		sess.QuestEncounterOwed = -1
	}

	qe := findQuestEncounter(&sess.SaveData, false)
	if qe == nil || qe.owed == sess.QuestEncounterOwed {
		return // nothing here, or this wave already fired and nothing changed since
	}
	sess.QuestEncounterOwed = qe.owed
	startQuestEncounter(sess, qe, response)
}

// maybeStartRoadQuestEncounter rolls for a quest stage's road fight while the
// player travels that road (minutesElapsed of movement this tick). Called from
// the travel tick, after travel progress advanced and only while not stopped.
func maybeStartRoadQuestEncounter(sess *session.GameSession, minutesElapsed int, response *types.GameActionResponse) {
	if minutesElapsed <= 0 || sess.SaveData.TravelStopped ||
		sess.ActiveCombat != nil || sess.ActivePOI != nil || sess.PendingEncounter != nil || social.IsFollower(sess.Npub) {
		return
	}
	qe := findQuestEncounter(&sess.SaveData, true)
	if qe == nil {
		return
	}
	if questRoll() >= min(roadQuestChancePerMinute*float64(minutesElapsed), roadQuestMaxTickChance) {
		return
	}
	startQuestEncounter(sess, qe, response)
}

// startQuestEncounter starts a quest stage's fight: as many of the monster as
// still owed, up to the encounter's per-fight Count.
func startQuestEncounter(sess *session.GameSession, qe *questEncounter, response *types.GameActionResponse) {
	count := qe.owed
	if qe.enc.Count > 0 && count > qe.enc.Count {
		count = qe.enc.Count
	}
	advancement, err := loadAdvancement()
	if err != nil {
		return
	}
	state := &sess.SaveData
	spec := combat.EncounterSpec{
		MonsterIDs:    combat.MonsterIDsForCount(qe.enc.MonsterID, count),
		EnvironmentID: state.Location,
		Surprise:      qe.enc.Surprise,
		Arena:         qe.enc.Arena,
	}
	cs, err := startFight(sess, spec, advancement, true)
	if err != nil {
		log.Printf("⚠️ quest encounter %s: %v", qe.quest.ID, err)
		return
	}
	if qe.enc.Text != "" {
		cs.Log = append([]string{"📜 " + qe.enc.Text}, cs.Log...)
	}
	sess.ActiveCombat = cs

	if response.Data == nil {
		response.Data = make(map[string]interface{})
	}
	payload := buildStateResponse(cs, state, cs.Log)
	combat.ClearSpawnPositions(cs)
	response.Data["combat_started"] = true
	response.Data["combat"] = payload
	log.Printf("⚔️  Quest encounter: %s — %d× %s (%d owed)", qe.quest.ID, count, qe.enc.MonsterID, qe.owed)
}

// findQuestEncounter returns the first active quest whose current stage has an
// encounter at the player's place and still owes kills, or nil. road picks
// which kind: encounters out on a road (an environment) or in a town.
func findQuestEncounter(save *types.SaveFile, road bool) *questEncounter {
	for _, qp := range save.QuestsActive {
		qd, err := serverdb.GetQuestByID(qp.QuestID)
		if err != nil || qd == nil || qp.Stage >= len(qd.Stages) {
			continue
		}
		stage := qd.Stages[qp.Stage]
		enc := stage.Encounter
		if enc == nil || !encounterHere(enc, save) || isRoad(enc.Location) != road {
			continue
		}
		if owed := killsOwed(stage, qp.ObjectiveCounts, enc.MonsterID); owed > 0 {
			return &questEncounter{quest: qd, enc: enc, owed: owed}
		}
	}
	return nil
}

// isRoad reports whether a location is out in the world (an environment).
func isRoad(location string) bool {
	biome, err := serverdb.GetEnvironmentType(location)
	return err == nil && biome != ""
}

// encounterHere reports whether the player stands where the encounter is.
// Empty district/building/room in the encounter match anything.
func encounterHere(enc *types.QuestStageEncounter, save *types.SaveFile) bool {
	if enc.Location != save.Location {
		return false
	}
	if enc.District != "" && enc.District != save.District {
		return false
	}
	if enc.Building != "" && enc.Building != save.Building {
		return false
	}
	if enc.Room != "" && enc.Room != save.Room {
		return false
	}
	return true
}

// killsOwed is how many more of monsterID the stage's slay objectives need.
func killsOwed(stage types.QuestStage, counts []int, monsterID string) int {
	owed := 0
	for i, obj := range stage.Objectives {
		if obj.Type != types.ObjectiveSlay || obj.Target != monsterID {
			continue
		}
		target := obj.Count
		if target <= 0 {
			target = 1
		}
		done := 0
		if i < len(counts) {
			done = counts[i]
		}
		if done < target {
			owed += target - done
		}
	}
	return owed
}

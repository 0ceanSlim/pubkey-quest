package game

import (
	"log"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/session"
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
	if sess.ActiveCombat != nil || sess.ActivePOI != nil {
		return
	}
	// Arriving somewhere new resets the "already fired here" memory, so leaving
	// and coming back is how you retry after fleeing.
	if sess.QuestEncounterPlace != sess.PlaceKey {
		sess.QuestEncounterPlace = sess.PlaceKey
		sess.QuestEncounterOwed = -1
	}

	qe := findQuestEncounter(&sess.SaveData)
	if qe == nil || qe.owed == sess.QuestEncounterOwed {
		return // nothing here, or this wave already fired and nothing changed since
	}
	sess.QuestEncounterOwed = qe.owed

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
	}
	cs, err := combat.StartEncounter(serverdb.GetDB(), state, sess.Npub, spec, advancement)
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
// encounter at the player's place and still owes kills, or nil.
func findQuestEncounter(save *types.SaveFile) *questEncounter {
	for _, qp := range save.QuestsActive {
		qd, err := serverdb.GetQuestByID(qp.QuestID)
		if err != nil || qd == nil || qp.Stage >= len(qd.Stages) {
			continue
		}
		stage := qd.Stages[qp.Stage]
		enc := stage.Encounter
		if enc == nil || !encounterHere(enc, save) {
			continue
		}
		if owed := killsOwed(stage, qp.ObjectiveCounts, enc.MonsterID); owed > 0 {
			return &questEncounter{quest: qd, enc: enc, owed: owed}
		}
	}
	return nil
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

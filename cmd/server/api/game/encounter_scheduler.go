package game

import (
	"log"
	"math/rand"
	"slices"
	"time"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/encounter"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// Authored-encounter scheduler. The biome roller (game/encounter) handles random
// monster fights; this fires the hand-authored vignettes — they share the POI
// node schema, so each runs through the same node walker (game/poi) as a POI.
// Triggered by context on the relevant tick/action; honors per-encounter
// cooldown + non-repeatable one-shots, and the shared encounter cooldown so
// vignettes and biome fights don't bunch.

// noteLocation records where the player is standing, resetting the dwell timer
// and the once-per-arrival roll whenever they move somewhere new. Called on
// every action that can change place, so the work is skipped unless it did.
func noteLocation(sess *session.GameSession) {
	state := &sess.SaveData
	key := state.Location + "|" + state.District + "|" + state.Building + "|" + state.Room
	if key == sess.PlaceKey {
		return
	}
	sess.PlaceKey = key
	sess.PlaceSince = state.CurrentDay*1440 + state.TimeOfDay
	sess.PlaceRolled = false
	sess.PlaceInCity = isCityLocation(state.Location)
	sess.PlaceBuildingType = ""
	if state.Building != "" {
		if btype, err := building.GetBuildingType(serverdb.GetDB(), state.Location, state.Building); err == nil {
			sess.PlaceBuildingType = btype
		}
	}
}

// isCityLocation reports whether a location is a city rather than a travel
// environment. Guards the place-based triggers: an authored encounter with no
// valid_locations matches everywhere, and "everywhere" must not include the open
// road, where the travel trigger belongs. Resolved once per move, not per action.
func isCityLocation(locationID string) bool {
	database := serverdb.GetDB()
	if database == nil {
		return false
	}
	var locationType string
	if err := database.QueryRow("SELECT location_type FROM locations WHERE id = ?", locationID).Scan(&locationType); err != nil {
		return false
	}
	return locationType != "environment"
}

// maybeFireEncounter rolls the authored encounters whose trigger + context match
// the player's situation and fires the first success. No-op when a walk or fight
// is already active, or while the shared encounter cooldown is in effect.
//
// minutesElapsed is the in-game time this check covers; pass 0 for an
// event-driven check (arriving somewhere), which is gated by dwell time instead.
func maybeFireEncounter(sess *session.GameSession, trigger string, contexts []string, minutesElapsed int, response *types.GameActionResponse) {
	if sess.ActivePOI != nil || sess.ActiveCombat != nil {
		return
	}
	state := &sess.SaveData
	nowAbs := state.CurrentDay*1440 + state.TimeOfDay
	if sess.LastEncounterTime > 0 && nowAbs-sess.LastEncounterTime < encounter.CooldownMinutes {
		return
	}

	// Event-driven triggers roll once per arrival, and only after the player has
	// been here a little while — no doorway ambushes, and no farming them by
	// pacing between two districts.
	eventDriven := minutesElapsed <= 0
	if eventDriven {
		if sess.PlaceRolled || nowAbs-sess.PlaceSince < encounter.PlaceDwellMinutes {
			return
		}
	}

	encs, err := serverdb.GetEncountersByTrigger(trigger)
	if err != nil || len(encs) == 0 {
		return
	}
	ctx := buildQuestContext(state)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	if eventDriven {
		sess.PlaceRolled = true // this arrival has had its one chance
	}

	for _, enc := range encs {
		if !encounterContextMatches(enc, contexts) {
			continue
		}
		if !requirement.Evaluate(enc.Requirements, ctx).OK {
			continue
		}
		if last, fired := sess.EncountersFired[enc.ID]; fired {
			if !enc.Repeatable {
				continue // one-shot, already seen
			}
			if enc.CooldownMinutes > 0 && nowAbs-last < enc.CooldownMinutes {
				continue // still cooling down
			}
		}
		chance := enc.Chance
		if !eventDriven {
			chance = encounter.VignetteChance(chance, minutesElapsed)
		}
		if rng.Float64() >= chance {
			continue
		}
		fireEncounter(sess, enc, response)
		return // one per check
	}
}

// encounterContextMatches reports whether an encounter's scope (ValidLocations
// for travel/location triggers, BuildingTypes for building_type) includes any of
// the current context values. An unscoped travel/location encounter matches all.
func encounterContextMatches(enc types.EncounterData, contexts []string) bool {
	if enc.Trigger == types.EncounterTriggerBuildingType {
		for _, bt := range enc.BuildingTypes {
			if slices.Contains(contexts, bt) {
				return true
			}
		}
		return false
	}
	if len(enc.ValidLocations) == 0 {
		return true
	}
	for _, loc := range enc.ValidLocations {
		if slices.Contains(contexts, loc) {
			return true
		}
	}
	return false
}

// fireEncounter starts an encounter walk through the shared node walker and flags
// it on the response so the client opens the exploration overlay — or drops into
// combat when the start node is a monster (the walk resumes on victory).
func fireEncounter(sess *session.GameSession, enc types.EncounterData, response *types.GameActionResponse) {
	state := &sess.SaveData
	nowAbs := state.CurrentDay*1440 + state.TimeOfDay
	if sess.EncountersFired == nil {
		sess.EncountersFired = map[string]int{}
	}
	sess.EncountersFired[enc.ID] = nowAbs
	sess.LastEncounterTime = nowAbs // share the encounter cooldown with biome fights

	sess.ActivePOI = &poi.Session{POIID: enc.ID, Title: enc.Name, Nodes: enc.Nodes, CurrentNode: enc.StartNode}
	data := map[string]any{}
	res, err := stepPOI(sess, enc.StartNode, data)
	if err != nil {
		log.Printf("⚠️ encounter %q failed to start: %v", enc.ID, err)
		sess.ActivePOI = nil
		return
	}

	if response.Data == nil {
		response.Data = make(map[string]interface{})
	}
	if data["combat_started"] == true {
		response.Data["combat_started"] = true
		response.Data["combat"] = data["combat"]
	} else {
		response.Data["encounter_started"] = true
		response.Data["encounter_name"] = enc.Name
		response.Data["poi_step"] = res
	}
	log.Printf("✨ Encounter fired: %s (%s)", enc.ID, enc.Trigger)
}

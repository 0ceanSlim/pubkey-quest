package game

import (
	"pubkey-quest/cmd/server/game/encounter"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/survival"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/cmd/server/social"
)

// handleBlowWhistle blows a signal whistle (docs/draft/survival-gear.md §3):
// out in the wild it calls a road encounter to where you stand, from the same
// biome, level band and day-or-night monsters as the road. It keeps the road's
// encounter cooldown, so it brings a fight sooner, never more of them. In a
// party only the leader blows it, as only the leader rolls the road.
func handleBlowWhistle(sess *GameSession) (*GameActionResponse, error) {
	state := &sess.SaveData
	id := survival.Config().Whistle
	if id == "" || !gameutil.PlayerHasItem(state, id) {
		return fishRefusal("You don't have a whistle."), nil
	}
	if !travel.IsInEnvironment(state) || state.Building != "" {
		return fishRefusal("There's no one out here to hear it but townsfolk. Blow it in the wild."), nil
	}
	if social.IsFollower(sess.Npub) {
		return fishRefusal("Your party's leader decides when to call trouble."), nil
	}
	if sess.ActiveCombat != nil || sess.PendingEncounter != nil || sess.ActivePOI != nil {
		return fishRefusal("Not now."), nil
	}
	resp := &GameActionResponse{Success: true, Color: "yellow", Data: map[string]interface{}{},
		Message: "📯 You blow a long, shrill note across the wild."}
	now := state.CurrentDay*1440 + state.TimeOfDay
	if last := state.Lim().LastEncounter; last > 0 && now-last < encounter.CooldownMinutes {
		resp.Message += " Nothing answers: whatever's out here is still keeping its distance."
		return resp, nil
	}
	if !rollTravelEncounter(sess, state, 0, true, resp) {
		resp.Message += " Nothing answers."
		return resp, nil
	}
	resp.Message += " Something answers."
	return resp, nil
}

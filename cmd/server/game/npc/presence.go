package npc

import (
	"encoding/json"
	"fmt"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
)

// OnDuty reports whether the NPC is here and working a slot that offers the
// action ("open_shop", "rent_room"): a smith repairs and an innkeeper serves
// only on shift, not over a drink or asleep.
func OnDuty(state *types.SaveFile, npcID, action string) bool {
	info, with := scheduleWith(state, npcID)
	if !with || info == nil || info.State == "sleeping" {
		return false // a keeper woken by the night bell rents a room, nothing more
	}
	for _, a := range info.AvailableActions {
		if a == action {
			return true
		}
	}
	return false
}

// scheduleWith resolves the NPC's schedule now, and whether they're where the
// player is.
func scheduleWith(state *types.SaveFile, npcID string) (*types.NPCScheduleInfo, bool) {
	database := db.GetDB()
	if database == nil {
		return nil, false
	}
	var props string
	if database.QueryRow("SELECT properties FROM npcs WHERE id = ?", npcID).Scan(&props) != nil {
		return nil, false
	}
	var npcData types.NPCData
	if json.Unmarshal([]byte(props), &npcData) != nil {
		return nil, false
	}
	info := ResolveNPCSchedule(&npcData, state.CurrentDay, state.TimeOfDay)
	switch DetermineLocationType(info.Location) {
	case "building":
		return info, state.Building == info.Location
	case "district":
		return info, state.Building == "" && fmt.Sprintf("%s-%s", state.Location, state.District) == info.Location
	}
	return info, false
}

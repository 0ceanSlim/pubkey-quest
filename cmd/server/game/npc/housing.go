package npc

import (
	"fmt"
	"log"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

// HandleRentRoomAction rents a room at an inn/tavern
func HandleRentRoomAction(state *types.SaveFile, session SessionProvider, params map[string]interface{}) (*types.GameActionResponse, error) {
	buildingID := state.Building
	if buildingID == "" {
		return nil, fmt.Errorf("not in a building")
	}

	// Get cost from params (from NPC dialogue config)
	cost := 50 // Default cost
	if c, ok := params["cost"].(float64); ok {
		cost = int(c)
	}

	// Check if player has enough gold
	goldAmount := gameutil.GetGoldQuantity(state)
	log.Printf("🪙 Player has %d gold, room costs %d gold", goldAmount, cost)
	if goldAmount < cost {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("You need %d gold to rent a room. You have %d gold.", cost, goldAmount),
			Color:   "red",
		}, nil
	}

	// Deduct gold
	log.Printf("💰 Attempting to deduct %d gold...", cost)
	if !gameutil.DeductGold(state, cost) {
		log.Printf("❌ Failed to deduct gold!")
		return &types.GameActionResponse{
			Success: false,
			Message: "Failed to deduct gold for room rental",
			Color:   "red",
		}, nil
	}

	// Verify gold was deducted
	newGoldAmount := gameutil.GetGoldQuantity(state)
	log.Printf("✅ Gold deducted successfully. Old: %d, New: %d", goldAmount, newGoldAmount)

	// Initialize rented rooms if needed (session-only data)
	rentedRooms := session.GetRentedRooms()
	if rentedRooms == nil {
		rentedRooms = []map[string]interface{}{}
	}

	// Check if already rented at this building
	for _, room := range rentedRooms {
		if building, ok := room["building"].(string); ok && building == buildingID {
			return &types.GameActionResponse{
				Success: false,
				Message: "You already have a room rented here",
				Color:   "yellow",
			}, nil
		}
	}

	// Add rented room (expires at end of next day - 23:59)
	expirationDay := state.CurrentDay + 1
	expirationTime := 1439 // 23:59

	room := map[string]interface{}{
		"building":        buildingID,
		"expiration_day":  expirationDay,
		"expiration_time": expirationTime,
	}

	rentedRooms = append(rentedRooms, room)
	session.SetRentedRooms(rentedRooms)

	log.Printf("🏠 Rented room at %s for %d gold (expires day %d at %d)", buildingID, cost, expirationDay, expirationTime)

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Rented a room for %d gold. You can sleep here until tomorrow night.", cost),
		Color:   "green",
		Data: map[string]interface{}{
			"rented_rooms": rentedRooms,
		},
	}, nil
}

// SleepSessionProvider extends SessionProvider with methods needed for sleep
type SleepSessionProvider interface {
	SessionProvider
	UpdateBuildingStates(buildingStates map[string]bool, time int)
	UpdateNPCsAtLocation(npcIDs []string, hour int)
	UpdateSnapshotAndCalculateDeltaProvider() types.DeltaProvider
}

// HandleSleepAction sleeps in a rented room
func HandleSleepAction(state *types.SaveFile, session SleepSessionProvider, npcIdsFunc func(string, string, string, int) []string) (*types.GameActionResponse, error) {
	buildingID := state.Building
	if buildingID == "" {
		return nil, fmt.Errorf("not in a building")
	}

	// Check if player has a rented room here and find the index (session-only data)
	hasRoom := false
	roomIndex := -1
	rentedRooms := session.GetRentedRooms()
	if rentedRooms != nil {
		for i, room := range rentedRooms {
			if building, ok := room["building"].(string); ok && building == buildingID {
				// Check if expired
				expDay, _ := room["expiration_day"].(int)
				expTime, _ := room["expiration_time"].(int)

				if state.CurrentDay > expDay || (state.CurrentDay == expDay && state.TimeOfDay > expTime) {
					// Room expired, remove it
					rentedRooms = append(rentedRooms[:i], rentedRooms[i+1:]...)
					session.SetRentedRooms(rentedRooms)
					return &types.GameActionResponse{
						Success: false,
						Message: "Your room rental has expired. Please rent another room.",
						Color:   "yellow",
					}, nil
				}

				hasRoom = true
				roomIndex = i
				break
			}
		}
	}

	if !hasRoom {
		return &types.GameActionResponse{
			Success: false,
			Message: "You don't have a room rented here",
			Color:   "red",
		}, nil
	}

	// Calculate sleep quality based on current time (late bedtime = poor sleep)
	// Ideal bedtime: before midnight (0-359 minutes or 1320-1439 minutes)
	// Late bedtime: after midnight (360-720 minutes) - fatigue penalty
	poorSleep := false
	if state.TimeOfDay >= 360 && state.TimeOfDay <= 720 {
		poorSleep = true
	}

	// Calculate how many minutes will be slept
	oldTime := state.TimeOfDay
	targetTime := 360 // 6 AM
	var minutesSlept int
	if oldTime >= targetTime {
		// Already past 6 AM, sleep until 6 AM next day (e.g., 10 PM = 1320 mins, sleep 8h40m)
		minutesSlept = (1440 - oldTime) + targetTime
		state.CurrentDay++
	} else {
		// Before 6 AM, sleep until 6 AM same day
		minutesSlept = targetTime - oldTime
	}
	state.TimeOfDay = targetTime

	// Tick down duration-based effects for the time slept
	// This handles buffs/debuffs like performance-high that expire over time
	effects.TickDownEffectDurations(state, minutesSlept)

	// Reset fatigue based on sleep quality
	if poorSleep {
		state.Fatigue = 1 // Poor sleep - still a bit tired
		log.Printf("😴 Poor sleep due to late bedtime (fatigue level 1)")
	} else {
		state.Fatigue = 0 // Good sleep - fully rested
		log.Printf("😴 Good sleep (fully rested)")
	}
	status.ResetFatigueAccumulator(state)
	status.UpdateFatiguePenaltyEffects(state)

	// Reset hunger (well fed after waking up)
	state.Hunger = 2
	status.ResetHungerAccumulator(state)
	status.UpdateHungerPenaltyEffects(state)
	status.EnsureHungerAccumulation(state)

	// Restore HP and Mana fully
	state.HP = state.MaxHP
	state.Mana = state.MaxMana

	// Remove the rented room after sleeping (room is used up)
	if roomIndex >= 0 && roomIndex < len(rentedRooms) {
		rentedRooms = append(rentedRooms[:roomIndex], rentedRooms[roomIndex+1:]...)
		session.SetRentedRooms(rentedRooms)
		log.Printf("🚪 Room rental at %s has been used and removed", buildingID)
	}

	sleepMessage := "You wake up refreshed at 6 AM."
	if poorSleep {
		sleepMessage = "You wake up at 6 AM, but didn't sleep well due to going to bed late."
	}

	// Update building states and NPCs after sleep (time jump)
	database := db.GetDB()
	if database != nil {
		newTime := state.TimeOfDay
		currentHour := newTime / 60

		// Refresh building states
		buildingStates, err := building.GetAllBuildingStatesForDistrict(
			database,
			state.Location,
			state.District,
			newTime,
		)
		if err == nil && len(buildingStates) > 0 {
			session.UpdateBuildingStates(buildingStates, newTime)
		}

		// Refresh NPCs
		npcIDs := npcIdsFunc(
			state.Location,
			state.District,
			state.Building,
			newTime,
		)
		session.UpdateNPCsAtLocation(npcIDs, currentHour)
	}

	// Calculate delta for frontend updates
	delta := session.UpdateSnapshotAndCalculateDeltaProvider()

	return &types.GameActionResponse{
		Success: true,
		Message: sleepMessage,
		Color:   "green",
		Delta:   delta.ToMap(),
		Data: map[string]interface{}{
			"time_of_day":  state.TimeOfDay,
			"current_day":  state.CurrentDay,
			"fatigue":      state.Fatigue,
			"hunger":       state.Hunger,
			"hp":           state.HP,
			"max_hp":       state.MaxHP,
			"mana":         state.Mana,
			"max_mana":     state.MaxMana,
			"rented_rooms": session.GetRentedRooms(), // Send updated rooms so frontend knows room was used
		},
	}, nil
}

// HandleRestAction rests to restore HP/Mana (sleep in rented room)
func HandleRestAction(state *types.SaveFile, _ map[string]interface{}) (*types.GameActionResponse, error) {
	// Restore HP and Mana
	state.HP = state.MaxHP
	state.Mana = state.MaxMana

	// Calculate sleep duration
	// If it's before 6 AM, sleep until 6 AM
	// If it's after 6 AM, sleep until 6 AM the next day
	sleepMinutes := 0
	targetTime := 6 * 60 // 6:00 AM

	if state.TimeOfDay < targetTime {
		// Sleep until 6 AM today
		sleepMinutes = targetTime - state.TimeOfDay
	} else {
		// Sleep until 6 AM tomorrow
		sleepMinutes = (1440 - state.TimeOfDay) + targetTime
	}

	// Clear fatigue and re-evaluate effects
	state.Fatigue = 0
	status.HandleFatigueChange(state)

	// Advance time manually (can't call gametime.AdvanceTime due to circular dependency)
	oldTime := state.TimeOfDay
	state.TimeOfDay += sleepMinutes
	if state.TimeOfDay >= 1440 {
		daysAdvanced := state.TimeOfDay / 1440
		state.CurrentDay += daysAdvanced
		state.TimeOfDay = state.TimeOfDay % 1440
	}

	// Tick active effects (includes fatigue/hunger accumulation effects)
	timeMessages := effects.TickEffects(state, sleepMinutes)

	// Update penalty effects based on current fatigue/hunger levels
	fatigueMsg, _ := status.UpdateFatiguePenaltyEffects(state)
	hungerMsg, _ := status.UpdateHungerPenaltyEffects(state)

	// Build response with time messages
	response := &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("You rest and sleep for %.1f hours. You wake up at 6:00 AM feeling refreshed.", float64(sleepMinutes)/60.0),
		State:   state,
	}

	// Add effect messages to response data
	effectMessages := []types.EffectMessage{}

	// Append effect messages from time advancement
	for _, msg := range timeMessages {
		effectMessages = append(effectMessages, msg)
	}

	// Add any new penalty effect messages
	if fatigueMsg != nil && !fatigueMsg.Silent {
		effectMessages = append(effectMessages, *fatigueMsg)
	}
	if hungerMsg != nil && !hungerMsg.Silent {
		effectMessages = append(effectMessages, *hungerMsg)
	}

	// Add to response data if we have messages
	if len(effectMessages) > 0 {
		response.Data = map[string]interface{}{
			"effect_messages": effectMessages,
		}
	}

	log.Printf("🛌 Player rested from %d to %d minutes (%.1f hours)", oldTime, state.TimeOfDay, float64(sleepMinutes)/60.0)

	return response, nil
}

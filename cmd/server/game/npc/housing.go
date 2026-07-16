package npc

import (
	"fmt"
	"log"
	"math"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

// HandleRentRoomAction rents a room at an inn/tavern. The rental is recorded on
// the save (survives reload) and unlocks the building's rented-state room; the
// player then walks into that room to sleep. The session param is unused now
// that rentals live on the save.
func HandleRentRoomAction(state *types.SaveFile, _ SessionProvider, params map[string]interface{}) (*types.GameActionResponse, error) {
	buildingID := state.Building
	if buildingID == "" {
		return nil, fmt.Errorf("not in a building")
	}

	// Get cost from params (from NPC dialogue config)
	cost := 50 // Default cost
	if c, ok := params["cost"].(float64); ok {
		cost = int(c)
	}

	if gameutil.HasActiveRental(state, buildingID) {
		return &types.GameActionResponse{Success: false, Message: "You already have a room rented here.", Color: "yellow"}, nil
	}

	goldAmount := gameutil.GetGoldQuantity(state)
	if goldAmount < cost {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("You need %d gold to rent a room. You have %d gold.", cost, goldAmount),
			Color:   "red",
		}, nil
	}
	if !gameutil.DeductGold(state, cost) {
		return &types.GameActionResponse{Success: false, Message: "Failed to deduct gold for room rental", Color: "red"}, nil
	}

	// Rental holds through the end of the next day.
	expiresDay := state.CurrentDay + 1
	expiresMin := 1439 // 23:59
	gameutil.AddRental(state, buildingID, expiresDay, expiresMin)

	log.Printf("🏠 Rented room at %s for %d gold (expires day %d)", buildingID, cost, expiresDay)

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Rented a room for %d gold. Head to your room — you can sleep there after 9 PM.", cost),
		Color:   "green",
		Data: map[string]interface{}{
			"rentals": state.Rentals,
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

// Sleep window: a night activity, no daytime naps.
const (
	sleepEarliest = 1260 // 21:00 — sleeping is barred before 9 PM
	sleepWakeTime = 360  // 06:00 — wake time
)

// CanSleepNow reports whether the time of day is in the night sleep window
// (after 9 PM, or before the 6 AM wake time).
func CanSleepNow(timeOfDay int) bool {
	return timeOfDay >= sleepEarliest || timeOfDay < sleepWakeTime
}

// HandleSleepAction sleeps in the player's rented room. Requires an active rental
// for the building, standing in the rented-state room (when the building has
// rooms), and a time after 9 PM. The rental persists until it expires — sleeping
// doesn't consume it.
func HandleSleepAction(state *types.SaveFile, session SleepSessionProvider, npcIdsFunc func(string, string, string, string, int) []string) (*types.GameActionResponse, error) {
	buildingID := state.Building
	if buildingID == "" {
		log.Printf("😴 sleep rejected: not in a building (location=%s room=%q)", state.Location, state.Room)
		return &types.GameActionResponse{Success: false, Message: "You can't sleep out here — rent a room at an inn.", Color: "red"}, nil
	}

	// Must hold an active rental for this building.
	if !gameutil.HasActiveRental(state, buildingID) {
		log.Printf("😴 sleep rejected: no active rental for %s (rentals=%+v)", buildingID, state.Rentals)
		return &types.GameActionResponse{Success: false, Message: "You don't have a room rented here. Rent one from the innkeeper.", Color: "red"}, nil
	}

	// If the building has rooms, you must be standing in your rented room.
	if database := db.GetDB(); database != nil {
		if rooms, _, roomErr := building.GetBuildingRooms(database, state.Location, buildingID); roomErr == nil && len(rooms) > 0 {
			room, found := building.FindRoom(rooms, state.Room)
			if !found || room.Access == nil || room.Access.State != "rented" {
				accessState := "<nil>"
				if found && room.Access != nil {
					accessState = room.Access.State
				}
				log.Printf("😴 sleep rejected: not in rented room (building=%s state.Room=%q found=%v access=%s)",
					buildingID, state.Room, found, accessState)
				return &types.GameActionResponse{Success: false, Message: "Head to your rented room to sleep.", Color: "yellow"}, nil
			}
		}
	}

	// Time gate: only after 9 PM (through the 6 AM wake time).
	if !CanSleepNow(state.TimeOfDay) {
		log.Printf("😴 sleep rejected: too early (time_of_day=%d)", state.TimeOfDay)
		return &types.GameActionResponse{Success: false, Message: "It's too early to sleep — come back after 9 PM.", Color: "yellow"}, nil
	}

	// Inn/tavern: a real bed and breakfast in the fee — best comfort, hunger restored.
	return applySleep(state, comfortInn, true, session, npcIdsFunc)
}

// comfort tiers for sleeping — bed quality drives how much fatigue a night restores.
const (
	comfortInn     = 1.0  // a real bed at an inn (breakfast is in the fee)
	comfortBedroll = 0.75 // camping with a bedroll
	comfortRough   = 0.5  // no gear — a rough night on hard ground
)

// HandleWildernessSleepAction lets a player bed down for the night out in a travel
// environment (not a town). A bedroll makes for a decent camp; without one it's a
// rough sleep that recovers less fatigue. No breakfast out here, so hunger isn't
// restored — you wake a little hungrier. Same night-time gate as an inn.
func HandleWildernessSleepAction(state *types.SaveFile, session SleepSessionProvider, npcIdsFunc func(string, string, string, string, int) []string) (*types.GameActionResponse, error) {
	if state.Building != "" {
		return &types.GameActionResponse{Success: false, Message: "You're indoors — rent a room to sleep here.", Color: "yellow"}, nil
	}
	if !isEnvironmentLocation(state.Location) {
		return &types.GameActionResponse{Success: false, Message: "You can't bed down in the middle of town — find an inn, or make camp out in the wild.", Color: "yellow"}, nil
	}
	if !CanSleepNow(state.TimeOfDay) {
		return &types.GameActionResponse{Success: false, Message: "It's not time to sleep yet — make camp after 9 PM.", Color: "yellow"}, nil
	}

	comfort := comfortRough
	if gameutil.PlayerHasItem(state, "bedroll") {
		comfort = comfortBedroll
	}
	return applySleep(state, comfort, false, session, npcIdsFunc)
}

// applySleep is the shared sleep resolution: sleep until the 6 AM wake time,
// restore HP/mana in proportion to how long was slept (so it scales with level),
// restore fatigue scaled by bed comfort × bedtime × hours, optionally restore
// hunger (paid lodging only), then advance time and refresh the world.
func applySleep(state *types.SaveFile, comfort float64, restoreHunger bool, session SleepSessionProvider, npcIdsFunc func(string, string, string, string, int) []string) (*types.GameActionResponse, error) {
	oldTime := state.TimeOfDay

	// Sleep until 6 AM (next day if already past it).
	var minutesSlept int
	if oldTime >= sleepWakeTime {
		minutesSlept = (1440 - oldTime) + sleepWakeTime
		state.CurrentDay++
	} else {
		minutesSlept = sleepWakeTime - oldTime
	}
	state.TimeOfDay = sleepWakeTime

	// Fraction of a full night actually slept (0..1) — drives restore amounts.
	frac := float64(minutesSlept) / float64(status.FullRestMinutes)
	if frac > 1 {
		frac = 1
	}

	// Bedtime modifier: bedding down in daylight (roughly 6 AM–8 PM) rests poorly.
	bedtime := 1.0
	if oldTime >= sleepWakeTime && oldTime < 1200 {
		bedtime = 0.7
	}

	// Duration-based buffs/debuffs expire over the time slept.
	effects.TickDownEffectDurations(state, minutesSlept)

	// HP/mana — proportional to time slept, scaled to Max (so it scales with level).
	hpGain, manaGain := status.RestoreVitalsForRest(state, minutesSlept)

	// Fatigue — restored by comfort × bedtime × hours (a comfy full night clears it).
	fatigueRestored := int(math.Round(10.0 * comfort * bedtime * frac))
	state.Fatigue -= fatigueRestored
	if state.Fatigue < 0 {
		state.Fatigue = 0
	}
	status.ResetFatigueAccumulator(state)
	status.UpdateFatiguePenaltyEffects(state)

	// Hunger — only paid lodging includes breakfast. Out in the wild the night's
	// calorie burn leaves you a step hungrier.
	if restoreHunger {
		state.Hunger = 2 // Satisfied
		status.ResetHungerAccumulator(state)
		status.UpdateHungerPenaltyEffects(state)
		status.EnsureHungerAccumulation(state)
	} else if minutesSlept >= 240 && state.Hunger > 0 {
		state.Hunger--
		status.UpdateHungerPenaltyEffects(state)
		status.EnsureHungerAccumulation(state)
	}
	log.Printf("😴 slept %dm (comfort=%.2f bedtime=%.2f) → +%d HP +%d mana, fatigue=%d hunger=%d",
		minutesSlept, comfort, bedtime, hpGain, manaGain, state.Fatigue, state.Hunger)

	// Refresh building states + NPCs after the time jump.
	if database := db.GetDB(); database != nil {
		newTime := state.TimeOfDay
		if buildingStates, err := building.GetAllBuildingStatesForDistrict(database, state.Location, state.District, newTime); err == nil && len(buildingStates) > 0 {
			session.UpdateBuildingStates(buildingStates, newTime)
		}
		npcIDs := npcIdsFunc(state.Location, state.District, state.Building, state.Room, newTime)
		session.UpdateNPCsAtLocation(npcIDs, newTime/60)
	}

	delta := session.UpdateSnapshotAndCalculateDeltaProvider()

	msg := fmt.Sprintf("You sleep %dh and wake at 6 AM", minutesSlept/60)
	switch {
	case comfort >= comfortInn:
		msg += ", warm and rested."
	case comfort >= comfortBedroll:
		msg += ", stiff but recovered from your camp."
	default:
		msg += " on the hard ground — better than nothing."
	}
	if hpGain > 0 || manaGain > 0 {
		msg += fmt.Sprintf(" (+%d HP, +%d mana)", hpGain, manaGain)
	}

	return &types.GameActionResponse{
		Success: true,
		Message: msg,
		Color:   "green",
		Delta:   delta.ToMap(),
		Data: map[string]interface{}{
			"time_of_day": state.TimeOfDay,
			"current_day": state.CurrentDay,
			"fatigue":     state.Fatigue,
			"hunger":      state.Hunger,
			"hp":          state.HP,
			"max_hp":      state.MaxHP,
			"mana":        state.Mana,
			"max_mana":    state.MaxMana,
			"rentals":     state.Rentals,
		},
	}, nil
}

// isEnvironmentLocation reports whether the location id is a travel environment
// (the wilderness) rather than a town — used to gate camping.
func isEnvironmentLocation(location string) bool {
	database := db.GetDB()
	if database == nil {
		return false
	}
	var lt string
	if err := database.QueryRow("SELECT COALESCE(location_type,'') FROM locations WHERE id = ?", location).Scan(&lt); err != nil {
		return false
	}
	return lt == "environment"
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

	// Tick active effects (includes fatigue/hunger accumulation effects). Sleeping
	// is rest — no fatigue accrual (it's zeroed on wake anyway); hunger still ticks.
	timeMessages := effects.TickEffects(state, sleepMinutes, false)

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

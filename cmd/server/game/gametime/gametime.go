package gametime

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/types"
)

// TimeSessionProvider defines the session interface needed for time operations
type TimeSessionProvider interface {
	GetLastActionGameTime() int
	GetLastActionTime() int64
	ShouldRefreshBuildings(newTime int) bool
	UpdateBuildingStates(buildingStates map[string]bool, time int)
	ShouldRefreshNPCs(currentHour int) bool
	GetNPCsAtLocation() []string
	UpdateNPCsAtLocation(npcIDs []string, hour int)
	GetBookedShows() []map[string]interface{}
	UpdateSnapshotAndCalculateDeltaProvider() types.DeltaProvider
}

// IdleResetSessionProvider defines session interface for idle timer reset
type IdleResetSessionProvider interface {
	SetLastActionTime(unixTime int64)
	SetLastActionGameTime(gameTime int)
	GetSaveDataAbsMinutes() int
}

// TimeConfig is game-data/systems/time-config.json: how the clock flows.
type TimeConfig struct {
	// IdleAutoPauseSeconds is the backstop for a player who has walked away:
	// real seconds with no input before the game pauses itself. Day to day
	// the clock is stopped by events instead (see PauseOnEffects, discoveries).
	IdleAutoPauseSeconds int `json:"idle_auto_pause_seconds"`
	// TownTimeScale slows (or speeds) the clock in a city, as a fraction of the
	// road's pace: 0.5 is half speed. The client applies it; the road is 1.
	TownTimeScale float64 `json:"town_time_scale"`
	// PauseOnEffects stops the clock the moment one of these effects starts
	// (hungry, tired, …) so a penalty never lands unnoticed.
	PauseOnEffects []string `json:"pause_on_effects"`

	// Monsters marked "active": "night" or "day" join the road pool only inside
	// their window ([from, to) in minutes since midnight; may wrap midnight).
	// The windows overlap at dusk and dawn. Unmarked monsters roam any time.
	NightMonsterHours [2]int `json:"night_monster_hours"`
	DayMonsterHours   [2]int `json:"day_monster_hours"`
	// NightEncounterMultiplier scales the road-fight chance during the night
	// window (types.NightStart/NightEnd).
	NightEncounterMultiplier float64 `json:"night_encounter_multiplier"`
	// DarkvisionRaces see in the dark: no attack disadvantage at night.
	DarkvisionRaces []string `json:"darkvision_races"`
	// SunlightHours is when monsters with Sunlight Sensitivity are at
	// disadvantage outdoors.
	SunlightHours [2]int `json:"sunlight_hours"`
}

var (
	timeConfigOnce sync.Once
	timeConfig     TimeConfig
)

// Config returns the time config, read once from the systems table, with
// defaults for anything missing.
func Config() TimeConfig {
	timeConfigOnce.Do(func() {
		timeConfig = TimeConfig{
			IdleAutoPauseSeconds:     300,
			TownTimeScale:            1,
			NightMonsterHours:        [2]int{1260, 540},
			DayMonsterHours:          [2]int{420, 1380},
			NightEncounterMultiplier: 1,
			SunlightHours:            [2]int{360, 1200},
		}
		database := db.GetDB()
		if database == nil {
			return
		}
		var props string
		if database.QueryRow("SELECT properties FROM systems WHERE id = 'time-config'").Scan(&props) != nil {
			return
		}
		var cfg TimeConfig
		if err := json.Unmarshal([]byte(props), &cfg); err != nil {
			log.Printf("⚠️ time-config: %v", err)
			return
		}
		if cfg.IdleAutoPauseSeconds > 0 {
			timeConfig.IdleAutoPauseSeconds = cfg.IdleAutoPauseSeconds
		}
		if cfg.TownTimeScale > 0 {
			timeConfig.TownTimeScale = cfg.TownTimeScale
		}
		timeConfig.PauseOnEffects = cfg.PauseOnEffects
		if cfg.NightMonsterHours != [2]int{} {
			timeConfig.NightMonsterHours = cfg.NightMonsterHours
		}
		if cfg.DayMonsterHours != [2]int{} {
			timeConfig.DayMonsterHours = cfg.DayMonsterHours
		}
		if cfg.NightEncounterMultiplier > 0 {
			timeConfig.NightEncounterMultiplier = cfg.NightEncounterMultiplier
		}
		timeConfig.DarkvisionRaces = cfg.DarkvisionRaces
		if cfg.SunlightHours != [2]int{} {
			timeConfig.SunlightHours = cfg.SunlightHours
		}
	})
	return timeConfig
}

// MonsterActiveAt reports whether a monster with this "active" value ("night",
// "day", or "" for any time) roams at a time of day.
func MonsterActiveAt(active string, timeOfDay int) bool {
	cfg := Config()
	switch active {
	case "night":
		return types.InWindow(timeOfDay, cfg.NightMonsterHours[0], cfg.NightMonsterHours[1])
	case "day":
		return types.InWindow(timeOfDay, cfg.DayMonsterHours[0], cfg.DayMonsterHours[1])
	}
	return true
}

// HasDarkvision reports whether a race sees in the dark.
func HasDarkvision(race string) bool {
	for _, r := range Config().DarkvisionRaces {
		if strings.EqualFold(r, race) {
			return true
		}
	}
	return false
}

// IsSunlit reports whether it's bright day (for Sunlight Sensitivity).
func IsSunlit(timeOfDay int) bool {
	cfg := Config()
	return types.InWindow(timeOfDay, cfg.SunlightHours[0], cfg.SunlightHours[1])
}

// HandleAdvanceTimeAction advances game time by segments (hours)
func HandleAdvanceTimeAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	segments, ok := params["segments"].(float64)
	if !ok {
		segments = 1 // Default to 1 time segment
	}

	// Convert hours to minutes (segments is in hours, time_of_day is now in minutes)
	minutesToAdvance := int(segments) * 60

	// Advance time in minutes (0-1439 range)
	state.TimeOfDay += minutesToAdvance
	daysAdvanced := state.TimeOfDay / 1440
	state.CurrentDay += daysAdvanced
	state.TimeOfDay = state.TimeOfDay % 1440

	// Fatigue and hunger are now handled by accumulation effects (fatigue-accumulation, hunger-accumulation-*)
	// No need to manually tick them here

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Advanced %d time segment(s)", int(segments)),
	}, nil
}

// HandleUpdateTimeAction syncs time from frontend clock to backend state
// This is the main tick handler that updates buildings, NPCs, and effects
func HandleUpdateTimeAction(state *types.SaveFile, params map[string]interface{}, session TimeSessionProvider, npcIdsFunc func(string, string, string, string, int) []string) (*types.GameActionResponse, error) {
	// Get the new time from frontend
	newTimeOfDay, timeOk := params["time_of_day"].(float64)
	newCurrentDay, dayOk := params["current_day"].(float64)

	if !timeOk || !dayOk {
		return &types.GameActionResponse{
			Success: false,
			Message: "Missing time parameters",
		}, nil
	}

	// Auto-pause: nobody has touched the game for IdleAutoPauseSeconds of
	// real time — they've likely walked away. (Events pause it otherwise.)
	autoPause := false
	if session != nil {
		idle := time.Now().Unix() - session.GetLastActionTime()
		if idle >= int64(Config().IdleAutoPauseSeconds) {
			autoPause = true
			log.Printf("⏸️ Auto-pause triggered: %ds with no player input", idle)
		}
	}

	// Calculate time delta
	oldTime := state.TimeOfDay
	oldDay := state.CurrentDay
	newTime := int(newTimeOfDay)
	newDay := int(newCurrentDay)

	// Calculate total minutes elapsed
	var minutesElapsed int
	if newDay == oldDay {
		// Same day
		minutesElapsed = newTime - oldTime
	} else {
		// Day(s) advanced
		minutesElapsed = (1440 - oldTime) + newTime + ((newDay - oldDay - 1) * 1440)
	}

	// Only process if time actually advanced
	if minutesElapsed > 0 {
		// Use AdvanceTime to properly process effects. Ambient time accrues fatigue
		// normally, EXCEPT while stopped to rest mid-travel — then the clock keeps
		// moving (hunger, arrival timers) but fatigue is frozen.
		AdvanceTime(state, minutesElapsed, !state.TravelStopped)
	}

	// Check for missed shows (session-only data) and apply penalty
	if session != nil {
		bookedShows := session.GetBookedShows()
		if len(bookedShows) > 0 {
			// Use npc package's CheckMissedShows if session implements the required interface
			if showSession, ok := session.(npc.SessionProvider); ok {
				npc.CheckMissedShows(state, showSession, newTime, newDay)
			}
		}
	}

	// Update buildings and NPCs if we have a session
	if session != nil {
		database := db.GetDB()

		// Update building states if needed (every 5 in-game minutes or first call)
		if session.ShouldRefreshBuildings(newTime) {
			buildingStates, err := building.GetAllBuildingStatesForDistrict(
				database,
				state.Location,
				state.District,
				newTime,
			)
			if err == nil && len(buildingStates) > 0 {
				session.UpdateBuildingStates(buildingStates, newTime)
			}
		}

		// Update NPCs if hour changed
		currentHour := newTime / 60
		if session.ShouldRefreshNPCs(currentHour) {
			npcIDs := npcIdsFunc(
				state.Location,
				state.District,
				state.Building,
				state.Room,
				newTime,
			)
			// Only log when NPCs actually change (reduces spam)
			if len(npcIDs) > 0 || len(session.GetNPCsAtLocation()) > 0 {
				log.Printf("🧑 NPCs updated: hour=%d, %s/%s/%s, was=%v, now=%v",
					currentHour, state.Location, state.District, state.Building, session.GetNPCsAtLocation(), npcIDs)
			}
			session.UpdateNPCsAtLocation(npcIDs, currentHour)
		}

		// Calculate delta
		delta := session.UpdateSnapshotAndCalculateDeltaProvider()
		if delta != nil && delta.GetNPCs() != nil {
			log.Printf("🧑 NPC delta: added=%v, removed=%v", delta.GetNPCs().Added, delta.GetNPCs().Removed)
		}
		if delta != nil && !delta.IsEmpty() {
			return &types.GameActionResponse{
				Success: true,
				Message: "Time updated",
				Delta:   delta.ToMap(),
				Data: map[string]interface{}{
					"time_of_day":    state.TimeOfDay,
					"current_day":    state.CurrentDay,
					"fatigue":        state.Fatigue,
					"hunger":         state.Hunger,
					"hp":             state.HP,
					"active_effects": effects.EnrichActiveEffects(state.ActiveEffects, state),
					"auto_pause":     autoPause,
				},
			}, nil
		}
	}

	// Return updated state so frontend can sync (fallback if no session/delta)
	return &types.GameActionResponse{
		Success: true,
		Message: "Time updated",
		Data: map[string]interface{}{
			"time_of_day":    state.TimeOfDay,
			"current_day":    state.CurrentDay,
			"fatigue":        state.Fatigue,
			"hunger":         state.Hunger,
			"hp":             state.HP,
			"active_effects": effects.EnrichActiveEffects(state.ActiveEffects, state),
			"auto_pause":     autoPause,
		},
	}, nil
}

// HandleWaitAction waits for a specified amount of time
// Accepts either "minutes" (15-360) or "hours" (1-6) for backwards compatibility
func HandleWaitAction(state *types.SaveFile, params map[string]interface{}, session TimeSessionProvider, npcIdsFunc func(string, string, string, string, int) []string) (*types.GameActionResponse, error) {
	var minutesToAdvance int

	// Check for minutes first (more granular), fall back to hours
	if minutesFloat, ok := params["minutes"].(float64); ok {
		minutesToAdvance = int(minutesFloat)
		// Validate minutes (15-360 in 15 minute increments, 6 hours max)
		if minutesToAdvance < 15 || minutesToAdvance > 360 {
			return &types.GameActionResponse{
				Success: false,
				Message: "You can only wait between 15 minutes and 6 hours",
				Color:   "red",
			}, nil
		}
	} else if hoursFloat, ok := params["hours"].(float64); ok {
		hours := int(hoursFloat)
		// Validate hours (1-6 hours max)
		if hours < 1 || hours > 6 {
			return &types.GameActionResponse{
				Success: false,
				Message: "You can only wait between 1 and 6 hours",
				Color:   "red",
			}, nil
		}
		minutesToAdvance = hours * 60
	} else {
		return nil, fmt.Errorf("hours or minutes parameter is required")
	}

	// Track hunger before wait (fatigue is intentionally frozen while waiting)
	oldHunger := state.Hunger

	// Advance time and process all effects. Waiting does NOT accrue fatigue — you're
	// deliberately resting — but hunger, durations, and starvation still progress.
	timeMessages := AdvanceTime(state, minutesToAdvance, false)

	// Waiting is light rest: recover HP/mana proportional to the time waited (a
	// full ~8h restores all; percentage-of-Max so it scales with level).
	hpGain, manaGain := status.RestoreVitalsForRest(state, minutesToAdvance)

	// Update building states and NPCs after time jump
	if session != nil {
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
				state.Room,
				newTime,
			)
			session.UpdateNPCsAtLocation(npcIDs, currentHour)
		}
	}

	// Format message based on wait duration
	var message string
	hours := minutesToAdvance / 60
	mins := minutesToAdvance % 60
	if hours > 0 && mins > 0 {
		message = fmt.Sprintf("You waited %d hour%s and %d minute%s.",
			hours, gameutil.Pluralize(hours), mins, gameutil.Pluralize(mins))
	} else if hours > 0 {
		message = fmt.Sprintf("You waited %d hour%s.", hours, gameutil.Pluralize(hours))
	} else {
		message = fmt.Sprintf("You waited %d minute%s.", mins, gameutil.Pluralize(mins))
	}

	// Waiting doesn't change fatigue anymore (it's frozen), so only surface hunger.
	if state.Hunger != oldHunger {
		hungerChange := state.Hunger - oldHunger
		hungerNames := map[int]string{0: "Starving", 1: "Hungry", 2: "Well Fed", 3: "Stuffed"}
		if hungerChange < 0 {
			message += fmt.Sprintf("\n\n🍽️ You're feeling hungrier (now %s)", hungerNames[state.Hunger])
		}
	}

	if hpGain > 0 || manaGain > 0 {
		message += fmt.Sprintf("\n\n💤 You recover %d HP and %d mana.", hpGain, manaGain)
	}

	// Append any time-based messages (like starvation damage)
	if len(timeMessages) > 0 {
		for _, msg := range timeMessages {
			if !msg.Silent {
				message += "\n\n" + msg.Message
			}
		}
	}

	log.Printf("⏱️ Waited %d minutes - Time: %d, Fatigue: %d (frozen), Hunger: %d→%d", minutesToAdvance, state.TimeOfDay, state.Fatigue, oldHunger, state.Hunger)

	// Calculate delta for UI updates
	var deltaMap map[string]interface{}
	if session != nil {
		delta := session.UpdateSnapshotAndCalculateDeltaProvider()
		if delta != nil {
			deltaMap = delta.ToMap()
		}
	}

	return &types.GameActionResponse{
		Success: true,
		Message: message,
		Color:   "yellow",
		Delta:   deltaMap,
		Data: map[string]interface{}{
			"time_of_day": state.TimeOfDay,
			"current_day": state.CurrentDay,
			"fatigue":     state.Fatigue,
			"hunger":      state.Hunger,
			"hp":          state.HP,
			"max_hp":      state.MaxHP,
			"mana":        state.Mana,
			"max_mana":    state.MaxMana,
		},
	}, nil
}

// HandleResetIdleTimerAction resets the auto-pause idle timer
// Called when the play button is pressed to prevent immediate re-triggering of auto-pause
func HandleResetIdleTimerAction(session IdleResetSessionProvider, currentUnixTime int64) (*types.GameActionResponse, error) {
	// Reset the idle tracking to current time
	session.SetLastActionTime(currentUnixTime)
	session.SetLastActionGameTime(session.GetSaveDataAbsMinutes())

	return &types.GameActionResponse{
		Success: true,
		Message: "Idle timer reset",
	}, nil
}

// AdvanceTime advances the game time by the specified minutes and ticks all time-based systems
// Returns messages from any effects that triggered (like starvation damage).
//
// accrueFatigue is passed through to the effect tick: pass false when the player is
// deliberately waiting or resting (stopped mid-travel) so time advances without
// building fatigue; pass true for normal time passing (ambient city clock, travel).
func AdvanceTime(state *types.SaveFile, minutes int, accrueFatigue bool) []types.EffectMessage {
	oldTime := state.TimeOfDay
	oldDay := state.CurrentDay

	// Advance time
	state.TimeOfDay += minutes

	// Handle day wrap-around
	if state.TimeOfDay >= 1440 {
		daysAdvanced := state.TimeOfDay / 1440
		state.CurrentDay += daysAdvanced
		state.TimeOfDay = state.TimeOfDay % 1440
	}

	// Tick active effects (includes fatigue/hunger accumulation effects)
	messages := effects.TickEffects(state, minutes, accrueFatigue)

	// Update penalty effects based on current fatigue/hunger levels
	fatigueMsg, _ := status.UpdateFatiguePenaltyEffects(state)
	hungerMsg, _ := status.UpdateHungerPenaltyEffects(state)

	// Add any new penalty effect messages
	if fatigueMsg != nil && !fatigueMsg.Silent {
		messages = append(messages, *fatigueMsg)
	}
	if hungerMsg != nil && !hungerMsg.Silent {
		messages = append(messages, *hungerMsg)
	}

	if state.CurrentDay != oldDay {
		log.Printf("📅 Day advanced from %d to %d", oldDay, state.CurrentDay)
	}

	// Only log time changes when hour changes (reduces log spam)
	oldHour := oldTime / 60
	newHour := state.TimeOfDay / 60
	if newHour != oldHour || state.CurrentDay != oldDay {
		log.Printf("⏰ Hour changed: %02d:00 -> %02d:00 (Day %d)", oldHour, newHour, state.CurrentDay)
	}

	return messages
}

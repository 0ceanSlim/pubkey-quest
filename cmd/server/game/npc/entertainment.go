package npc

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/types"
)

// HandleBookShowAction books a performance at a tavern
func HandleBookShowAction(state *types.SaveFile, session SessionProvider, params map[string]interface{}) (*types.GameActionResponse, error) {
	showID, ok := params["show_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing show_id parameter")
	}

	npcID, ok := params["npc_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing npc_id parameter")
	}

	venueID := state.Building
	if venueID == "" {
		return nil, fmt.Errorf("not in a building")
	}

	// Get NPC data from database
	database := db.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}

	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM npcs WHERE id = ?", npcID).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("NPC not found: %s", npcID)
	}

	// Parse NPC properties
	var npcData map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &npcData); err != nil {
		return nil, fmt.Errorf("failed to parse NPC data: %v", err)
	}

	// Load show configuration from NPC data
	showConfig, ok := npcData["show_config"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("NPC %s does not have show_config", npcID)
	}

	// Get day of week
	dayOfWeek := GetDayOfWeek(state.CurrentDay)
	dayStr := fmt.Sprintf("%d", dayOfWeek)

	// Get available shows for this day
	showsByDay, _ := showConfig["shows_by_day"].(map[string]interface{})
	availableShows, ok := showsByDay[dayStr].([]interface{})
	if !ok || len(availableShows) == 0 {
		return &types.GameActionResponse{
			Success: false,
			Message: "No shows available today",
			Color:   "yellow",
		}, nil
	}

	// Find the requested show
	var selectedShow map[string]interface{}
	for _, show := range availableShows {
		if showMap, ok := show.(map[string]interface{}); ok {
			if id, ok := showMap["id"].(string); ok && id == showID {
				selectedShow = showMap
				break
			}
		}
	}

	if selectedShow == nil {
		return &types.GameActionResponse{
			Success: false,
			Message: "Show not available today",
			Color:   "yellow",
		}, nil
	}

	// Check booking deadline (must book before specified time, e.g., 8 PM = 1200 minutes)
	showTime := int(showConfig["show_time"].(float64))
	bookingDeadline := int(showConfig["booking_deadline"].(float64))

	// Check if current time is past the booking deadline
	if state.TimeOfDay >= bookingDeadline && state.TimeOfDay < showTime {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("Too late to book! Booking closes at %d:%02d.", bookingDeadline/60, bookingDeadline%60),
			Color:   "red",
		}, nil
	}

	// No instrument needed to book: you need it at show time, and a booking you
	// don't play costs you (CheckMissedShows), which the booking says.
	// One show a day. The day it was played is kept with the save
	// (types.Limits): a booking alone is session memory, and a reload once
	// let a player book and play again the same day.
	if state.Lim().ShowDay == state.CurrentDay {
		return &types.GameActionResponse{
			Success: false,
			Message: "You've already played a show today",
			Color:   "yellow",
		}, nil
	}

	// Check if already booked a show for today (session-only data)
	bookedShows := session.GetBookedShows()
	if bookedShows == nil {
		bookedShows = []map[string]interface{}{}
	}

	for _, booking := range bookedShows {
		if day, ok := booking["day"].(int); ok && day == state.CurrentDay {
			return &types.GameActionResponse{
				Success: false,
				Message: "You've already booked a show for today",
				Color:   "yellow",
			}, nil
		}
	}

	// Book the show
	booking := map[string]interface{}{
		"show_id":   showID,
		"venue_id":  venueID,
		"day":       state.CurrentDay,
		"show_time": showTime,
		"performed": false,
		"show_data": selectedShow, // Store show data for later
	}

	bookedShows = append(bookedShows, booking)
	session.SetBookedShows(bookedShows)

	showName, _ := selectedShow["name"].(string)
	log.Printf("🎭 Booked show '%s' (ID: %s) at %s for day %d", showName, showID, venueID, state.CurrentDay)

	msg := fmt.Sprintf("Booked '%s' for tonight at %s. Bring %s.", showName, ShowClock(showTime), InstrumentList(ShowInstruments(selectedShow)))
	if missing := MissingInstruments(state, selectedShow); len(missing) > 0 {
		msg += fmt.Sprintf(" You still need %s.", InstrumentList(missing))
	}
	msg += " If you don't play, word gets around: -2 Charisma for a day."
	return &types.GameActionResponse{
		Success: true,
		Message: msg,
		Color:   "green",
		Data: map[string]interface{}{
			"booked_shows": bookedShows,
		},
	}, nil
}

// PerformShowSessionProvider extends SessionProvider with show tracking
type PerformShowSessionProvider interface {
	SessionProvider
	GetPerformedShows() []string
	SetPerformedShows(shows []string)
}

// HandlePlayShowAction performs a booked show
func HandlePlayShowAction(state *types.SaveFile, session PerformShowSessionProvider, advanceTimeFunc func(*types.SaveFile, int, bool) []types.EffectMessage) (*types.GameActionResponse, error) {
	log.Printf("🎭 handlePlayShowAction called - building: %s, day: %d, time: %d", state.Building, state.CurrentDay, state.TimeOfDay)

	venueID := state.Building
	if venueID == "" {
		return nil, fmt.Errorf("not in a building")
	}

	// Find booked show for today at this venue (session-only data)
	var booking map[string]interface{}
	var bookingIndex int
	bookedShows := session.GetBookedShows()
	if bookedShows != nil {
		for i, b := range bookedShows {
			day, _ := b["day"].(int)
			venue, _ := b["venue_id"].(string)
			performed, _ := b["performed"].(bool)

			if day == state.CurrentDay && venue == venueID && !performed {
				booking = b
				bookingIndex = i
				break
			}
		}
	}

	if booking == nil {
		return &types.GameActionResponse{
			Success: false,
			Message: "You don't have a show booked for tonight",
			Color:   "yellow",
		}, nil
	}

	// Check if it's show time (must be within 30 minutes of show time)
	showTime := gameutil.GetIntValue(booking, "show_time", 0)
	timeDiff := state.TimeOfDay - showTime
	if timeDiff < 0 || timeDiff > 30 {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("It's not show time yet (the show starts at %s)", ShowClock(showTime)),
			Color:   "yellow",
		}, nil
	}

	// Get show data
	showData, _ := booking["show_data"].(map[string]interface{})
	if showData == nil {
		return nil, fmt.Errorf("show data not found in booking")
	}

	// Every instrument the show names (a duo needs both); the first sets how hard
	// it is to play.
	if missing := MissingInstruments(state, showData); len(missing) > 0 {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("You need %s to play this show.", InstrumentList(missing)),
			Color:   "red",
		}, nil
	}
	instruments := ShowInstruments(showData)
	if len(instruments) == 0 {
		return nil, fmt.Errorf("show names no instrument")
	}
	instrumentID := instruments[0]

	// Load instrument difficulty data
	instrumentData, err := loadInstrumentData(instrumentID)
	if err != nil {
		log.Printf("⚠️ Failed to load instrument data for %s, using defaults: %v", instrumentID, err)
		// Use safe defaults if instrument not found
		instrumentData = map[string]interface{}{
			"base_success":      70.0,
			"charisma_modifier": 5.0,
		}
	}

	// Get charisma stat (with active effect modifiers)
	charisma := effects.Ability(state, "charisma")

	// Calculate performance success chance
	baseSuccess := gameutil.GetFloatValue(instrumentData, "base_success", 50.0)
	charismaMod := gameutil.GetFloatValue(instrumentData, "charisma_modifier", 5.0)
	successChance := baseSuccess + (float64(charisma-10) * charismaMod)

	// Clamp success chance between 5% and 95%
	if successChance < 5 {
		successChance = 5
	}
	if successChance > 95 {
		successChance = 95
	}

	// Perform charisma check (roll 1-100)
	roll := rand.Intn(100) + 1
	performanceSuccess := float64(roll) <= successChance

	log.Printf("🎲 Performance check: roll=%d, success_threshold=%.0f%% - %v", roll, successChance, performanceSuccess)

	// Calculate rewards
	baseGold := gameutil.GetIntValue(showData, "base_gold", 0)
	baseXP := gameutil.GetIntValue(showData, "base_xp", 0)
	charismaBonus := gameutil.GetIntValue(showData, "charisma_gold_bonus", 0)

	// Calculate total gold (charisma bonus applies to points above 10)
	charismaMod2 := charisma - 10
	if charismaMod2 < 0 {
		charismaMod2 = 0
	}
	totalGold := baseGold + (charismaMod2 * charismaBonus)

	// Add gold to inventory (always get paid)
	if err := gameutil.AddGoldToInventory(state.Inventory, totalGold); err != nil {
		log.Printf("⚠️ Failed to add gold to inventory: %v", err)
	}

	// Only award XP on successful performance
	var resultMessage string
	var resultColor string
	var levelUp types.LevelUpResult
	if performanceSuccess {
		if adv, advErr := character.LoadAdvancement(db.GetDB()); advErr == nil {
			// Apply the per-level XP bonus so performances scale like combat XP.
			levelUp = character.GrantXP(state, character.BonusedXP(state, baseXP, adv), adv)
		} else {
			log.Printf("⚠️ advancement load failed; XP applied without level-up check: %v", advErr)
			state.Experience += baseXP
		}
		// Apply performance-high effect (+2 charisma for 12 hours)
		if err := effects.ApplyEffect(state, "performance-high"); err != nil {
			log.Printf("⚠️ Failed to apply performance-high effect: %v", err)
			resultMessage = fmt.Sprintf("🎵 Excellent performance! The crowd loved it! Earned %d gold and %d XP!", totalGold, baseXP)
		} else {
			resultMessage = fmt.Sprintf("🎵 Excellent performance! The crowd loved it! Earned %d gold and %d XP. You feel confident! (+2 Charisma for 12 hours)", totalGold, baseXP)
		}
		resultColor = "green"
		log.Printf("✅ Performance success! Earned %d gold, %d XP", totalGold, baseXP)
	} else {
		// Apply stage-fright effect (-1 charisma for 12 hours)
		if err := effects.ApplyEffect(state, "stage-fright"); err != nil {
			log.Printf("⚠️ Failed to apply stage-fright effect: %v", err)
			resultMessage = fmt.Sprintf("😰 The performance was lackluster. Earned %d gold but no experience.", totalGold)
		} else {
			resultMessage = fmt.Sprintf("😰 The performance was lackluster. Earned %d gold but no experience. You feel shaken. (-1 Charisma for 12 hours)", totalGold)
		}
		resultColor = "yellow"
		log.Printf("❌ Performance failure! Earned %d gold (no XP)", totalGold)
	}

	// Advance time by 60 minutes (1 hour performance). Watching a show is normal
	// city time passing, so fatigue accrues as usual (true).
	oldTime := state.TimeOfDay
	timeMessages := advanceTimeFunc(state, 60, true)
	log.Printf("⏰ Time advanced from %d to %d (60 minutes)", oldTime, state.TimeOfDay)

	// Append any time-based messages (like starvation damage) to result
	if len(timeMessages) > 0 {
		for _, msg := range timeMessages {
			if !msg.Silent {
				resultMessage += "\n\n" + msg.Message
			}
		}
	}

	// Mark show as performed: the booking in the session, the day with the save.
	state.Lim().ShowDay = state.CurrentDay
	bookedShows[bookingIndex]["performed"] = true
	session.SetBookedShows(bookedShows)

	// Add to performed shows list for daily tracking (session-only data)
	performedShows := session.GetPerformedShows()
	if performedShows == nil {
		performedShows = []string{}
	}
	showID, _ := booking["show_id"].(string)
	performedKey := fmt.Sprintf("%s_%d", showID, state.CurrentDay)
	performedShows = append(performedShows, performedKey)
	session.SetPerformedShows(performedShows)

	resp := &types.GameActionResponse{
		Success: true,
		Message: resultMessage,
		Color:   resultColor,
	}
	if levelUp.Leveled {
		resp.Data = map[string]interface{}{"level_up": levelUp}
		resp.Message += fmt.Sprintf("\n\nYou reached level %d!", levelUp.NewLevel)
	}
	return resp, nil
}

// CheckMissedShows checks for any booked shows that the player missed and applies a penalty
func CheckMissedShows(state *types.SaveFile, session SessionProvider, currentTime int, currentDay int) {
	bookedShows := session.GetBookedShows()
	if bookedShows == nil {
		return
	}

	for i, booking := range bookedShows {
		// Skip already performed or already penalized shows
		performed, _ := booking["performed"].(bool)
		penalized, _ := booking["penalized"].(bool)
		if performed || penalized {
			continue
		}

		bookingDay := gameutil.GetIntValue(booking, "day", -1)
		showTime := gameutil.GetIntValue(booking, "show_time", 0)
		showEndTime := showTime + 60 // 1-hour show window (9-10pm)

		// Check if we've passed the show window
		// Case 1: Same day, past the show end time
		// Case 2: Day has advanced (definitely missed)
		showMissed := false
		if bookingDay == currentDay && currentTime > showEndTime {
			showMissed = true
		} else if currentDay > bookingDay {
			showMissed = true
		}

		if showMissed {
			log.Printf("🎭 Player missed booked show! Day %d, show was at %d, current time is day %d at %d",
				bookingDay, showTime, currentDay, currentTime)

			// Apply no-show penalty effect (-2 charisma for 24 hours)
			if err := effects.ApplyEffect(state, "no-show"); err != nil {
				log.Printf("⚠️ Failed to apply no-show effect: %v", err)
			} else {
				log.Printf("😔 Applied no-show penalty: -2 Charisma for 24 hours")
			}

			// Mark as penalized so we don't keep applying the penalty
			bookedShows[i]["penalized"] = true
		}
	}
	session.SetBookedShows(bookedShows)
}

// loadInstrumentData loads instrument difficulty data from database
func loadInstrumentData(instrumentID string) (map[string]interface{}, error) {
	database := db.GetDB()
	if database == nil {
		return nil, fmt.Errorf("database not available")
	}

	// Query item properties from database
	var propertiesJSON string
	err := database.QueryRow("SELECT properties FROM items WHERE id = ?", instrumentID).Scan(&propertiesJSON)
	if err != nil {
		return nil, fmt.Errorf("instrument not found in database: %s", instrumentID)
	}

	// Parse properties JSON
	var properties map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &properties); err != nil {
		return nil, fmt.Errorf("failed to parse instrument properties: %v", err)
	}

	// Extract performance data
	performance, ok := properties["performance"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("instrument %s has no performance data", instrumentID)
	}

	return performance, nil
}

// ShowInstruments lists the instruments a show is played on.
func ShowInstruments(show map[string]interface{}) []string {
	raw, _ := show["required_instruments"].([]interface{})
	var out []string
	for _, r := range raw {
		if id, _ := r.(string); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// MissingInstruments lists the show's instruments the player isn't carrying.
func MissingInstruments(state *types.SaveFile, show map[string]interface{}) []string {
	var missing []string
	for _, id := range ShowInstruments(show) {
		if !gameutil.PlayerHasItem(state, id) {
			missing = append(missing, id)
		}
	}
	return missing
}

// InstrumentList says instruments in a sentence: "a lute", "a lute and a flute".
func InstrumentList(ids []string) string {
	names := make([]string, len(ids))
	for i, id := range ids {
		names[i] = "a " + strings.ReplaceAll(id, "-", " ")
	}
	switch len(names) {
	case 0:
		return "your instrument"
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// ShowClock says a time of day the way innkeepers do: "9 PM", "8:30 PM".
func ShowClock(minutes int) string {
	h, m := (minutes/60)%24, minutes%60
	half := "AM"
	if h >= 12 {
		half = "PM"
	}
	if h = h % 12; h == 0 {
		h = 12
	}
	if m == 0 {
		return fmt.Sprintf("%d %s", h, half)
	}
	return fmt.Sprintf("%d:%02d %s", h, m, half)
}

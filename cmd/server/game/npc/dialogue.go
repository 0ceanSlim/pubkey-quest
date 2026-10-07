package npc

import (
	"encoding/json"
	"fmt"
	"log"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/types"
)

// HandleTalkToNPCAction initiates dialogue with an NPC
func HandleTalkToNPCAction(state *types.SaveFile, params map[string]interface{}) (*types.GameActionResponse, error) {
	npcID, ok := params["npc_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid npc_id parameter")
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

	// Parse NPC properties into NPCData struct
	var npcData types.NPCData
	if err := json.Unmarshal([]byte(propertiesJSON), &npcData); err != nil {
		return nil, fmt.Errorf("failed to parse NPC data: %v", err)
	}

	// Resolve NPC schedule based on current time
	scheduleInfo := ResolveNPCSchedule(&npcData, state.TimeOfDay)

	// Determine location type from location ID
	locationType := DetermineLocationType(scheduleInfo.Location)

	// Check if player is at NPC's current location
	playerAtLocation := false
	if locationType == "building" && state.Building == scheduleInfo.Location {
		playerAtLocation = true
	} else if locationType == "district" && state.Building == "" {
		// Construct full district ID from location + district (e.g., "kingdom" + "center" = "kingdom-center")
		playerDistrictID := fmt.Sprintf("%s-%s", state.Location, state.District)
		if playerDistrictID == scheduleInfo.Location {
			playerAtLocation = true
		}
	}

	if !playerAtLocation {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("%s is not here at this time.", npcData.Name),
			Color:   "yellow",
		}, nil
	}

	// Check if NPC is available for interaction
	if !scheduleInfo.IsAvailable {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("%s is busy right now.", npcData.Name),
			Color:   "yellow",
		}, nil
	}

	// Determine greeting based on state
	greetingText := ""
	isRegistered := vault.IsVaultRegistered(state, scheduleInfo.Location)
	isNativeRace := IsNativeRaceForLocation(state.Race, state.Location)

	if isNativeRace {
		greetingText, _ = npcData.Greeting["native_race"]
	} else if isRegistered {
		greetingText, _ = npcData.Greeting["returning"]
	} else {
		greetingText, _ = npcData.Greeting["first_time"]
	}

	// Get initial dialogue node (first from available options)
	var dialogueNode string
	var dialogueText string
	var options []string

	if len(scheduleInfo.AvailableDialogue) > 0 {
		dialogueNode = scheduleInfo.AvailableDialogue[0]

		// Get dialogue content
		if nodeData, ok := npcData.Dialogue[dialogueNode].(map[string]interface{}); ok {
			dialogueText, _ = nodeData["text"].(string)
			if opts, ok := nodeData["options"].([]interface{}); ok {
				for _, opt := range opts {
					if optStr, ok := opt.(string); ok {
						// Filter options based on requirements
						optionNode, _ := npcData.Dialogue[optStr].(map[string]interface{})
						if optionNode != nil {
							requirements, _ := optionNode["requirements"].(map[string]interface{})
							if CheckDialogueRequirements(state, requirements) {
								options = append(options, optStr)
							}
						} else {
							options = append(options, optStr)
						}
					}
				}
			}
		}
	} else {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("%s is busy right now.", npcData.Name),
			Color:   "yellow",
		}, nil
	}

	log.Printf("💬 %s: %s (schedule state: %s, showing %d options)", npcID, greetingText, scheduleInfo.State, len(options))

	return &types.GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("%s\n\n%s", greetingText, dialogueText),
		Color:   "yellow",
		Delta: map[string]interface{}{
			"npc_dialogue": map[string]interface{}{
				"npc_id":         npcID,
				"node":           dialogueNode,
				"text":           dialogueText,
				"options":        options,
				"schedule_state": scheduleInfo.State,
			},
		},
	}, nil
}

// HandleNPCDialogueChoiceAction processes player's dialogue choice
// Note: This function needs access to GameSession for certain actions, so it uses an interface
type SessionProvider interface {
	GetSaveData() *types.SaveFile
	GetBookedShows() []map[string]interface{}
	SetBookedShows(shows []map[string]interface{})
	GetRentedRooms() []map[string]interface{}
	SetRentedRooms(rooms []map[string]interface{})
}

// HandleNPCDialogueChoiceActionWithSession processes dialogue with session access
func HandleNPCDialogueChoiceActionWithSession(state *types.SaveFile, params map[string]interface{}, session SessionProvider) (*types.GameActionResponse, error) {
	npcID, _ := params["npc_id"].(string)
	choice, _ := params["choice"].(string)

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

	dialogue, _ := npcData["dialogue"].(map[string]interface{})
	choiceNode, _ := dialogue[choice].(map[string]interface{})

	if choiceNode == nil {
		return nil, fmt.Errorf("invalid dialogue choice: %s", choice)
	}

	// Check requirements
	requirements, _ := choiceNode["requirements"].(map[string]interface{})
	if !CheckDialogueRequirements(state, requirements) {
		return &types.GameActionResponse{
			Success: false,
			Message: "Requirements not met for this dialogue option",
			Color:   "red",
		}, nil
	}

	// Get action if any
	action, _ := choiceNode["action"].(string)
	responseText, _ := choiceNode["text"].(string)

	// Process action
	var actionResult string
	switch action {
	case "register_storage":
		// What a keeper wants to let you in varies by city: coin, goods, or a
		// favour already done (gated by requirements on the node). Gold and
		// goods are both taken here, and only once everything checks out — so a
		// short player never loses half the price.
		cost, _ := choiceNode["cost"].(float64)
		goods := ParseItemRequirements(choiceNode["consume_items"])

		affordable := gameutil.GetGoldQuantity(state) >= int(cost)
		for _, g := range goods {
			if gameutil.CountItem(state, g.ID) < g.Quantity {
				affordable = false
				break
			}
		}

		if !affordable {
			actionResult, _ = choiceNode["failure"].(string)
			break
		}
		if !gameutil.DeductGold(state, int(cost)) {
			log.Printf("⚠️ Failed to deduct gold even though we had enough")
			actionResult, _ = choiceNode["failure"].(string)
			break
		}
		taken := true
		for _, g := range goods {
			if !gameutil.ConsumeItem(state, g.ID, g.Quantity) {
				log.Printf("⚠️ Failed to take %dx %s even though the player had them", g.Quantity, g.ID)
				taken = false
				break
			}
		}
		if !taken {
			actionResult, _ = choiceNode["failure"].(string)
			break
		}
		vault.RegisterVault(state, state.Building)
		actionResult, _ = choiceNode["success"].(string)

	case "rent_room":
		cost, _ := choiceNode["cost"].(float64)
		result, err := HandleRentRoomAction(state, session, map[string]interface{}{"cost": cost})
		if err != nil {
			log.Printf("❌ Failed to rent room: %v", err)
			actionResult, _ = choiceNode["failure"].(string)
		} else if result.Success {
			actionResult, _ = choiceNode["success"].(string)
		} else {
			// Failed but not an error (e.g., not enough gold, already rented)
			actionResult = result.Message
		}

	case "open_storage":
		// One shared vault, many doors — this keeper just has to have accepted
		// the player (see package vault).
		if !vault.IsVaultRegistered(state, state.Building) {
			log.Printf("❌ Keeper at %s has not accepted this player (registered: %v)", state.Building, state.VaultKeepers)
			return &types.GameActionResponse{
				Success: false,
				Message: "This keeper won't open the vault for you yet",
				Color:   "error",
			}, nil
		}
		log.Printf("✅ Opening the shared vault via %s", state.Building)
		return &types.GameActionResponse{
			Success: true,
			Message: responseText,
			Color:   "yellow",
			Delta: map[string]interface{}{
				"open_vault": vault.Response(state),
				"npc_dialogue": map[string]interface{}{
					"action": "close",
				},
			},
		}, nil

	case "open_shop":
		// Return signal to open shop UI
		log.Printf("✅ Opening shop for merchant: %s", npcID)
		return &types.GameActionResponse{
			Success: true,
			Message: responseText,
			Color:   "yellow",
			Delta: map[string]interface{}{
				"open_shop": npcID,
				"npc_dialogue": map[string]interface{}{
					"action": "close",
				},
			},
		}, nil

	case "open_sell":
		// Return signal to open shop UI in sell mode
		log.Printf("✅ Opening shop (sell mode) for merchant: %s", npcID)
		return &types.GameActionResponse{
			Success: true,
			Message: responseText,
			Color:   "yellow",
			Delta: map[string]interface{}{
				"open_shop": npcID,
				"shop_tab":  "sell",
				"npc_dialogue": map[string]interface{}{
					"action": "close",
				},
			},
		}, nil

	case "book_show":
		return handleBookShowDialogue(state, session, npcID, npcData, choiceNode, responseText)

	case "end_dialogue":
		return &types.GameActionResponse{
			Success: true,
			Message: responseText,
			Color:   "yellow",
			Delta: map[string]interface{}{
				"npc_dialogue": map[string]interface{}{
					"action": "close",
				},
			},
		}, nil
	}

	// Get next options and filter based on requirements
	options, _ := choiceNode["options"].([]interface{})
	var optionsList []string
	for _, opt := range options {
		if optStr, ok := opt.(string); ok {
			// Get the option node to check requirements
			optionNode, _ := dialogue[optStr].(map[string]interface{})
			if optionNode != nil {
				requirements, _ := optionNode["requirements"].(map[string]interface{})
				// Only include option if requirements are met
				if CheckDialogueRequirements(state, requirements) {
					optionsList = append(optionsList, optStr)
				} else {
					log.Printf("🚫 Filtered out option '%s' (requirements not met)", optStr)
				}
			} else {
				// No requirements, include it
				optionsList = append(optionsList, optStr)
			}
		}
	}

	displayText := responseText
	if actionResult != "" {
		displayText = actionResult
	}

	return &types.GameActionResponse{
		Success: true,
		Message: displayText,
		Color:   "yellow",
		Delta: map[string]interface{}{
			"npc_dialogue": map[string]interface{}{
				"npc_id":  npcID,
				"node":    choice,
				"text":    displayText,
				"options": optionsList,
			},
		},
	}, nil
}

// handleBookShowDialogue handles the book_show dialogue action
func handleBookShowDialogue(state *types.SaveFile, session SessionProvider, npcID string, npcData map[string]interface{}, _ map[string]interface{}, responseText string) (*types.GameActionResponse, error) {
	// Load show configuration from NPC data
	log.Printf("✅ Getting available shows for NPC: %s", npcID)

	// Get current building/venue for tracking
	venueID := state.Building
	if venueID == "" {
		return &types.GameActionResponse{
			Success: false,
			Message: "Not in a building",
			Color:   "red",
		}, nil
	}

	// Get show configuration from NPC
	showConfig, ok := npcData["show_config"].(map[string]interface{})
	if !ok {
		log.Printf("❌ NPC %s does not have show_config", npcID)
		return &types.GameActionResponse{
			Success: false,
			Message: "This NPC doesn't offer show bookings",
			Color:   "red",
		}, nil
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
			Delta: map[string]interface{}{
				"npc_dialogue": map[string]interface{}{
					"action": "close",
				},
			},
		}, nil
	}

	// Check booking deadline
	bookingDeadline := int(showConfig["booking_deadline"].(float64))
	showTime := int(showConfig["show_time"].(float64))

	if state.TimeOfDay >= bookingDeadline && state.TimeOfDay < showTime {
		return &types.GameActionResponse{
			Success: false,
			Message: fmt.Sprintf("Too late to book! Booking closes at %d:%02d.", bookingDeadline/60, bookingDeadline%60),
			Color:   "red",
			Delta: map[string]interface{}{
				"npc_dialogue": map[string]interface{}{
					"action": "close",
				},
			},
		}, nil
	}

	// Check if already booked a show for today (session-only data)
	if session != nil {
		bookedShows := session.GetBookedShows()
		if bookedShows != nil {
			for _, booking := range bookedShows {
				if day, ok := booking["day"].(int); ok && day == state.CurrentDay {
					return &types.GameActionResponse{
						Success: false,
						Message: "You've already booked a show for today",
						Color:   "yellow",
						Delta: map[string]interface{}{
							"npc_dialogue": map[string]interface{}{
								"action": "close",
							},
						},
					}, nil
				}
			}
		}
	}

	// Check instrument availability for each show (but include all shows)
	var allShows []map[string]interface{}
	hasAnyBookableShow := false
	for _, show := range availableShows {
		if showMap, ok := show.(map[string]interface{}); ok {
			requiredInstruments, _ := showMap["required_instruments"].([]interface{})
			hasAllInstruments := true
			var missingInstruments []string
			for _, instrRaw := range requiredInstruments {
				instrument, _ := instrRaw.(string)
				if !gameutil.PlayerHasItem(state, instrument) {
					hasAllInstruments = false
					missingInstruments = append(missingInstruments, instrument)
				}
			}

			// Add instrument availability info to the show
			showWithAvailability := make(map[string]interface{})
			for k, v := range showMap {
				showWithAvailability[k] = v
			}
			showWithAvailability["can_book"] = hasAllInstruments
			showWithAvailability["missing_instruments"] = missingInstruments

			allShows = append(allShows, showWithAvailability)

			if hasAllInstruments {
				hasAnyBookableShow = true
			}
		}
	}

	// Modify message if player can't book any shows
	displayMessage := responseText
	if !hasAnyBookableShow {
		displayMessage = "Here are tonight's available shows. You'll need the right instruments to book a performance."
	}

	// Return show selection UI with all shows
	return &types.GameActionResponse{
		Success: true,
		Message: displayMessage,
		Color:   "yellow",
		Delta: map[string]interface{}{
			"show_booking": map[string]interface{}{
				"npc_id":          npcID,
				"venue_id":        venueID,
				"available_shows": allShows,
				"show_time":       showTime,
			},
			"npc_dialogue": map[string]interface{}{
				"action": "close",
			},
		},
	}, nil
}

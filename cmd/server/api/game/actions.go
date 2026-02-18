package game

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/movement"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/game/travel"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// Type aliases for backward compatibility
type GameAction = types.GameAction
type GameActionResponse = types.GameActionResponse
type EffectMessage = types.EffectMessage
type SaveFile = types.SaveFile
type GameSession = session.GameSession

// GameActionRequest represents a game action request
// swagger:model GameActionRequest
type GameActionRequest struct {
	Npub   string     `json:"npub" example:"npub1..."`
	SaveID string     `json:"save_id" example:"save_1234567890"`
	Action GameAction `json:"action"`
}

// GameActionHandler godoc
// @Summary      Execute game action
// @Description  Process a game action (move, use_item, equip, cast_spell, rest, etc.)
// @Tags         Game
// @Accept       json
// @Produce      json
// @Param        request  body      GameActionRequest  true  "Action request"
// @Success      200      {object}  types.GameActionResponse
// @Failure      400      {string}  string  "Invalid request"
// @Failure      404      {string}  string  "Session not found"
// @Failure      405      {string}  string  "Method not allowed"
// @Router       /game/action [post]
func GameActionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Npub   string     `json:"npub"`
		SaveID string     `json:"save_id"`
		Action GameAction `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		log.Printf("❌ Failed to decode action request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.Npub == "" || request.SaveID == "" {
		http.Error(w, "Missing npub or save_id", http.StatusBadRequest)
		return
	}

	sessionMgr := session.GetSessionManager()

	// Get session from memory
	session, err := sessionMgr.GetSession(request.Npub, request.SaveID)
	if err != nil {
		// Try to load it if not in memory
		session, err = sessionMgr.LoadSession(request.Npub, request.SaveID)
		if err != nil {
			log.Printf("❌ Session not found: %v", err)
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
	}

	// Track last action time for player actions (not tick-type actions)
	if request.Action.Type != "update_time" {
		session.LastActionTime = time.Now().Unix()
		session.LastActionGameTime = session.SaveData.TimeOfDay
	}

	// Process the action based on type
	response, err := processGameAction(session, request.Action)
	if err != nil {
		log.Printf("❌ Action failed: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(GameActionResponse{
			Success: false,
			Error:   err.Error(),
			Message: fmt.Sprintf("Failed to process action: %v", err),
		})
		return
	}

	// Update encumbrance effects if this action modified inventory
	updateEncumbranceIfNeeded(&session.SaveData, request.Action.Type, response)

	// Update session in memory
	if err := sessionMgr.UpdateSession(request.Npub, request.SaveID, session.SaveData); err != nil {
		log.Printf("❌ Failed to update session: %v", err)
		http.Error(w, "Failed to update session", http.StatusInternalServerError)
		return
	}

	// Calculate delta from previous snapshot
	delta := session.UpdateSnapshotAndCalculateDelta()
	if delta != nil && !delta.IsEmpty() {
		calculatedDelta := delta.ToMap()

		// Merge calculated delta with any handler-specific delta (like vault_data)
		// This preserves special data added by handlers while also including state changes
		if response.Delta != nil {
			// Handler already set some delta data - merge with calculated
			for key, value := range calculatedDelta {
				response.Delta[key] = value
			}
		} else {
			response.Delta = calculatedDelta
		}
	}

	// Return updated state (and delta if available)
	response.State = &session.SaveData

	// Always include enriched effects and calculated values in Data for frontend display
	if response.Data == nil {
		response.Data = make(map[string]interface{})
	}
	response.Data["enriched_effects"] = effects.EnrichActiveEffects(session.SaveData.ActiveEffects, &session.SaveData)
	response.Data["total_weight"] = status.CalculateTotalWeight(&session.SaveData)
	response.Data["weight_capacity"] = status.CalculateWeightCapacity(&session.SaveData)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// processGameAction routes to specific action handlers
func processGameAction(session *GameSession, action GameAction) (*GameActionResponse, error) {
	state := &session.SaveData

	// Snapshot time before action (for travel progress calculation)
	oldTimeOfDay := state.TimeOfDay
	oldCurrentDay := state.CurrentDay

	response, err := processActionSwitch(session, state, action)

	// After any action, advance travel progress if time changed and player is in environment
	if err == nil && response != nil && response.Success {
		var minutesElapsed int
		if state.CurrentDay == oldCurrentDay {
			minutesElapsed = state.TimeOfDay - oldTimeOfDay
		} else if state.CurrentDay > oldCurrentDay {
			minutesElapsed = (1440 - oldTimeOfDay) + state.TimeOfDay + ((state.CurrentDay - oldCurrentDay - 1) * 1440)
		}

		if minutesElapsed > 0 {
			travelUpdate := travel.MaybeAdvanceTravelProgress(state, minutesElapsed)
			if travelUpdate != nil {
				if response.Data == nil {
					response.Data = make(map[string]interface{})
				}
				response.Data["travel_progress"] = travelUpdate.TravelProgress
				if travelUpdate.Arrived {
					response.Data["arrived"] = true
					response.Data["dest_city"] = travelUpdate.DestCity
					response.Data["dest_city_name"] = travelUpdate.DestCityName
					response.Data["dest_district"] = travelUpdate.DestDistrict
					response.Data["newly_discovered"] = travelUpdate.NewlyDiscovered
					response.Data["music_unlocked"] = travelUpdate.MusicUnlocked
				}
			}
		}
	}

	return response, err
}

// processActionSwitch dispatches to specific action handlers
func processActionSwitch(session *GameSession, state *SaveFile, action GameAction) (*GameActionResponse, error) {
	switch action.Type {
	case "move":
		return handleMoveAction(state, action.Params)
	case "use_item":
		return handleUseItemAction(state, action.Params)
	case "equip_item":
		return inventory.HandleEquipItemAction(state, action.Params)
	case "unequip_item":
		return inventory.HandleUnequipItemAction(state, action.Params)
	case "drop_item":
		return handleDropItemAction(state, action.Params)
	case "remove_from_inventory":
		return handleRemoveFromInventoryAction(state, action.Params)
	case "pickup_item":
		return handlePickupItemAction(state, action.Params)
	case "cast_spell":
		return handleCastSpellAction(state, action.Params)
	case "rest":
		return handleRestAction(state, action.Params)
	case "advance_time":
		return handleAdvanceTimeAction(state, action.Params)
	case "update_time":
		return handleUpdateTimeAction(state, action.Params)
	case "vault_deposit":
		return handleVaultDepositAction(state, action.Params)
	case "vault_withdraw":
		return handleVaultWithdrawAction(state, action.Params)
	case "move_item":
		return handleMoveItemAction(state, action.Params)
	case "stack_item":
		return handleStackItemAction(state, action.Params)
	case "split_item":
		return handleSplitItemAction(state, action.Params)
	case "add_item":
		return handleAddItemAction(state, action.Params)
	case "add_to_container":
		return handleAddToContainerAction(state, action.Params)
	case "remove_from_container":
		return handleRemoveFromContainerAction(state, action.Params)
	case "enter_building":
		return handleEnterBuildingAction(state, action.Params)
	case "exit_building":
		return handleExitBuildingAction(state, action.Params)
	case "talk_to_npc":
		return handleTalkToNPCAction(state, action.Params)
	case "npc_dialogue_choice":
		return handleNPCDialogueChoiceAction(session, action.Params)
	case "register_vault":
		return handleRegisterVaultAction(state, action.Params)
	case "open_vault":
		return handleOpenVaultAction(state, action.Params)
	case "rent_room":
		return handleRentRoomAction(session, action.Params)
	case "sleep":
		return handleSleepAction(session, action.Params)
	case "wait":
		return handleWaitAction(session, action.Params)
	case "book_show":
		return handleBookShowAction(session, action.Params)
	case "play_show":
		return handlePlayShowAction(session, action.Params)
	case "start_travel":
		return handleStartTravelAction(state, action.Params)
	case "stop_travel":
		return handleStopTravelAction(state, action.Params)
	case "resume_travel":
		return handleResumeTravelAction(state, action.Params)
	case "turn_back":
		return handleTurnBackAction(state, action.Params)
	case "reset_idle_timer":
		return handleResetIdleTimerAction(session)
	default:
		return nil, fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// updateEncumbranceIfNeeded updates encumbrance effects if the action modifies inventory
func updateEncumbranceIfNeeded(state *SaveFile, actionType string, response *GameActionResponse) {
	// Only update encumbrance for inventory-modifying actions that succeeded
	if response == nil || !response.Success {
		return
	}

	inventoryActions := map[string]bool{
		"equip_item":            true,
		"unequip_item":          true,
		"drop_item":             true,
		"remove_from_inventory": true,
		"pickup_item":           true,
		"vault_deposit":         true,
		"vault_withdraw":        true,
		"move_item":             true,
		"stack_item":            true,
		"split_item":            true,
		"add_item":              true,
		"add_to_container":      true,
		"remove_from_container": true,
		"use_item":              true, // Consumables affect weight too
	}

	if inventoryActions[actionType] {
		if encMsg, err := status.UpdateEncumbrancePenaltyEffects(state); err != nil {
			log.Printf("⚠️ Failed to update encumbrance effects: %v", err)
		} else if encMsg != nil && !encMsg.Silent {
			// Append encumbrance message to response if there was a change
			if response.Message != "" {
				response.Message += " " + encMsg.Message
			} else {
				response.Message = encMsg.Message
			}
		}
	}
}

// ============================================================================
// ACTION HANDLERS
// ============================================================================

// pluralize is a helper - uses gameutil.Pluralize
func pluralize(count int) string {
	return gameutil.Pluralize(count)
}

// handleMoveAction moves the player to a new location
func handleMoveAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := movement.HandleMoveAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// handleUseItemAction uses a consumable item
func handleUseItemAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleUseItemAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// Equipment handlers are now in equipment.go

// handleDropItemAction drops an item from inventory
func handleDropItemAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleDropItemAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// handleRemoveFromInventoryAction removes an item from inventory (for sell staging)
func handleRemoveFromInventoryAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleRemoveFromInventoryAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// handlePickupItemAction picks up an item from the ground
func handlePickupItemAction(_ *SaveFile, params map[string]any) (*GameActionResponse, error) {
	itemID, ok := params["item_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid item_id parameter")
	}

	// TODO: Validate item is on the ground at current location
	// TODO: Find empty inventory slot
	// TODO: Add item to inventory

	return &GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Picked up %s", itemID),
	}, nil
}

// handleCastSpellAction casts a spell
func handleCastSpellAction(_ *SaveFile, params map[string]any) (*GameActionResponse, error) {
	spellID, ok := params["spell_id"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid spell_id parameter")
	}

	// TODO: Validate spell is known
	// TODO: Check mana cost
	// TODO: Apply spell effects
	// TODO: Reduce mana

	return &GameActionResponse{
		Success: true,
		Message: fmt.Sprintf("Cast %s", spellID),
	}, nil
}

// handleRestAction rests to restore HP/Mana
func handleRestAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := npc.HandleRestAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
		}, err
	}
	return nil, err
}

// handleAdvanceTimeAction advances game time
func handleAdvanceTimeAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := gametime.HandleAdvanceTimeAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// handleUpdateTimeAction syncs time from frontend clock to backend state
// This is the main tick handler that updates buildings, NPCs, and effects
func handleUpdateTimeAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}

	// Get session for delta tracking
	npub := state.InternalNpub
	saveID := state.InternalID
	session, err := session.GetSessionManager().GetSession(npub, saveID)
	if err != nil {
		log.Printf("⚠️ Session not found for delta: %s:%s", npub, saveID)
	}

	resp, err := gametime.HandleUpdateTimeAction(state, paramsIface, session, data.GetNPCIDsAtLocation)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
			Delta:   resp.Delta,
			Data:    resp.Data,
		}, err
	}
	return nil, err
}

// handleVaultDepositAction deposits items into vault (uses existing move_item action for vault transfers)
func handleVaultDepositAction(_ *SaveFile, _ map[string]any) (*GameActionResponse, error) {
	// Vaults work like containers - use the container system
	// This is handled by frontend calling move_item or add_to_container with vault as destination
	return &GameActionResponse{
		Success: true,
		Message: "Item deposited to vault",
	}, nil
}

// handleVaultWithdrawAction withdraws items from vault (uses existing move_item action for vault transfers)
func handleVaultWithdrawAction(_ *SaveFile, _ map[string]any) (*GameActionResponse, error) {
	// Vaults work like containers - use the container system
	// This is handled by frontend calling move_item or remove_from_container with vault as source
	return &GameActionResponse{
		Success: true,
		Message: "Item withdrawn from vault",
	}, nil
}

// handleMoveItemAction moves/swaps items between inventory slots
func handleMoveItemAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleMoveItemAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Error: resp.Error, Color: resp.Color, Delta: resp.Delta}, err
	}
	return nil, err
}

// handleStackItemAction stacks items together
func handleStackItemAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleStackItemAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// handleSplitItemAction splits a stack into two stacks
func handleSplitItemAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleSplitItemAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// handleAddItemAction adds an item to inventory
func handleAddItemAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleAddItemAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// GetGameStateHandler godoc
// @Summary      Get game state
// @Description  Returns current game state for a session including character data, inventory, and session-specific data
// @Tags         Game
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save file ID"
// @Success      200      {object}  map[string]interface{}
// @Failure      400      {string}  string  "Missing npub or save_id"
// @Failure      404      {string}  string  "Session not found"
// @Failure      405      {string}  string  "Method not allowed"
// @Router       /game/state [get]
func GetGameStateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")

	if npub == "" || saveID == "" {
		http.Error(w, "Missing npub or save_id", http.StatusBadRequest)
		return
	}

	sessionMgr := session.GetSessionManager()

	// Get session from memory
	session, err := sessionMgr.GetSession(npub, saveID)
	if err != nil {
		// Try to load it if not in memory
		session, err = sessionMgr.LoadSession(npub, saveID)
		if err != nil {
			log.Printf("❌ Failed to get session: %v", err)
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
	}

	// Calculate weight and capacity (NOT stored in save file - calculated on-the-fly)
	totalWeight := status.CalculateTotalWeight(&session.SaveData)
	weightCapacity := status.CalculateWeightCapacity(&session.SaveData)

	// Include session-specific data in the response
	stateWithSession := map[string]any{
		"success": true,
		"state": map[string]any{
			// Include all SaveData fields
			"d":                     session.SaveData.D,
			"created_at":            session.SaveData.CreatedAt,
			"race":                  session.SaveData.Race,
			"class":                 session.SaveData.Class,
			"background":            session.SaveData.Background,
			"alignment":             session.SaveData.Alignment,
			"experience":            session.SaveData.Experience,
			"hp":                    session.SaveData.HP,
			"max_hp":                session.SaveData.MaxHP,
			"mana":                  session.SaveData.Mana,
			"max_mana":              session.SaveData.MaxMana,
			"fatigue":               session.SaveData.Fatigue,
			"hunger":                session.SaveData.Hunger,
			"stats":                 session.SaveData.Stats,
			"location":              session.SaveData.Location,
			"district":              session.SaveData.District,
			"building":              session.SaveData.Building,
			"travel_progress":       session.SaveData.TravelProgress,
			"travel_stopped":        session.SaveData.TravelStopped,
			"current_day":           session.SaveData.CurrentDay,
			"time_of_day":           session.SaveData.TimeOfDay,
			"inventory":             session.SaveData.Inventory,
			"vaults":                session.SaveData.Vaults,
			"known_spells":          session.SaveData.KnownSpells,
			"spell_slots":           session.SaveData.SpellSlots,
			"locations_discovered":  session.SaveData.LocationsDiscovered,
			"music_tracks_unlocked": session.SaveData.MusicTracksUnlocked,
			"active_effects":        effects.EnrichActiveEffects(session.SaveData.ActiveEffects, &session.SaveData),

			// Add calculated values (NOT persisted - calculated at runtime)
			"total_weight":    totalWeight,
			"weight_capacity": weightCapacity,

			// Add session-specific data
			"rented_rooms":    session.RentedRooms,
			"booked_shows":    session.BookedShows,
			"performed_shows": session.PerformedShows,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stateWithSession)
}

// handleEnterBuildingAction enters a building
func handleEnterBuildingAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := movement.HandleEnterBuildingAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Color: resp.Color}, err
	}
	return nil, err
}

// handleExitBuildingAction exits a building
func handleExitBuildingAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := movement.HandleExitBuildingAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Color: resp.Color}, err
	}
	return nil, err
}

// handleTalkToNPCAction initiates dialogue with an NPC
func handleTalkToNPCAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	// Convert params to map[string]interface{} for package compatibility
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := npc.HandleTalkToNPCAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
			Color:   resp.Color,
			Delta:   resp.Delta,
		}, err
	}
	return nil, err
}

// handleNPCDialogueChoiceAction processes player's dialogue choice
func handleNPCDialogueChoiceAction(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	// Convert params to map[string]interface{} for package compatibility
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := npc.HandleNPCDialogueChoiceActionWithSession(&session.SaveData, paramsIface, session)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
			Color:   resp.Color,
			Delta:   resp.Delta,
			Data:    resp.Data,
		}, err
	}
	return nil, err
}

// handleRegisterVaultAction registers a vault (called after payment)
func handleRegisterVaultAction(state *SaveFile, _ map[string]any) (*GameActionResponse, error) {
	buildingID := state.Building
	if buildingID == "" {
		return nil, fmt.Errorf("not in a building")
	}

	vault.RegisterVault(state, buildingID)

	return &GameActionResponse{
		Success: true,
		Message: "Vault registered successfully",
		Color:   "green",
	}, nil
}

// handleOpenVaultAction returns vault data for UI
func handleOpenVaultAction(state *SaveFile, _ map[string]any) (*GameActionResponse, error) {
	buildingID := state.Building
	if buildingID == "" {
		return nil, fmt.Errorf("not in a building")
	}

	vaultData := vault.GetVaultForLocation(state, buildingID)
	if vaultData == nil {
		return nil, fmt.Errorf("no vault registered at this location")
	}

	return &GameActionResponse{
		Success: true,
		Message: "Vault opened",
		Delta: map[string]any{
			"vault": vaultData,
		},
	}, nil
}

// handleAddToContainerAction adds an item to a container
func handleAddToContainerAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleAddToContainerAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Color: resp.Color}, err
	}
	return nil, err
}

// handleRemoveFromContainerAction removes an item from a container
func handleRemoveFromContainerAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := inventory.HandleRemoveFromContainerAction(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Color: resp.Color}, err
	}
	return nil, err
}

// ============================================================================
// TAVERN ACTIONS
// ============================================================================

// handleRentRoomAction rents a room at an inn/tavern
func handleRentRoomAction(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	// Convert params to map[string]interface{} for package compatibility
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := npc.HandleRentRoomAction(&session.SaveData, session, paramsIface)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
			Color:   resp.Color,
			Data:    resp.Data,
		}, err
	}
	return nil, err
}

// handleSleepAction sleeps in a rented room
func handleSleepAction(session *GameSession, _ map[string]any) (*GameActionResponse, error) {
	resp, err := npc.HandleSleepAction(&session.SaveData, session, data.GetNPCIDsAtLocation)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
			Color:   resp.Color,
			Delta:   resp.Delta,
			Data:    resp.Data,
		}, err
	}
	return nil, err
}

// handleWaitAction waits for a specified amount of time
// Accepts either "minutes" (15-360) or "hours" (1-6) for backwards compatibility
func handleWaitAction(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := gametime.HandleWaitAction(&session.SaveData, paramsIface, session, data.GetNPCIDsAtLocation)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
			Color:   resp.Color,
			Delta:   resp.Delta,
			Data:    resp.Data,
		}, err
	}
	return nil, err
}

// ============================================================================
// TRAVEL ACTIONS
// ============================================================================

// handleStartTravelAction begins travel through an environment
func handleStartTravelAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := travel.HandleStartTravel(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Color: resp.Color, Data: resp.Data}, err
	}
	return nil, err
}

// handleStopTravelAction stops movement in environment (time keeps flowing)
func handleStopTravelAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := travel.HandleStopTravel(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Color: resp.Color, Data: resp.Data}, err
	}
	return nil, err
}

// handleResumeTravelAction resumes movement in environment
func handleResumeTravelAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := travel.HandleResumeTravel(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Color: resp.Color, Data: resp.Data}, err
	}
	return nil, err
}

// handleTurnBackAction reverses travel direction
func handleTurnBackAction(state *SaveFile, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := travel.HandleTurnBack(state, paramsIface)
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message, Color: resp.Color, Data: resp.Data}, err
	}
	return nil, err
}

// handleResetIdleTimerAction resets the auto-pause idle timer
// Called when the play button is pressed to prevent immediate re-triggering of auto-pause
func handleResetIdleTimerAction(session *GameSession) (*GameActionResponse, error) {
	resp, err := gametime.HandleResetIdleTimerAction(session, time.Now().Unix())
	if resp != nil {
		return &GameActionResponse{Success: resp.Success, Message: resp.Message}, err
	}
	return nil, err
}

// handleBookShowAction books a performance at a tavern
func handleBookShowAction(session *GameSession, params map[string]any) (*GameActionResponse, error) {
	paramsIface := make(map[string]interface{}, len(params))
	for k, v := range params {
		paramsIface[k] = v
	}
	resp, err := npc.HandleBookShowAction(&session.SaveData, session, paramsIface)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
			Color:   resp.Color,
			Data:    resp.Data,
		}, err
	}
	return nil, err
}

// handlePlayShowAction performs a booked show
func handlePlayShowAction(session *GameSession, _ map[string]any) (*GameActionResponse, error) {
	resp, err := npc.HandlePlayShowAction(&session.SaveData, session, gametime.AdvanceTime)
	if resp != nil {
		return &GameActionResponse{
			Success: resp.Success,
			Message: resp.Message,
			Color:   resp.Color,
		}, err
	}
	return nil, err
}

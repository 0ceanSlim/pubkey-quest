package data

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/types"
)

// NPCRoomMatches reports whether an NPC whose schedule slot is in slotRoom is
// visible to a player standing in playerRoom. An empty slot room means the
// building's default room; an empty player room means the building has no rooms.
func NPCRoomMatches(slotRoom, playerRoom, defaultRoom string) bool {
	if slotRoom == "" {
		return playerRoom == "" || playerRoom == defaultRoom
	}
	return slotRoom == playerRoom
}

// buildingDefaultRoom returns a building's default room id (empty if it has none).
func buildingDefaultRoom(database *sql.DB, locationID, buildingID string) string {
	if buildingID == "" {
		return ""
	}
	if _, defaultRoom, err := building.GetBuildingRooms(database, locationID, buildingID); err == nil {
		return defaultRoom
	}
	return ""
}

// NPCLocationResponse represents NPC visibility at a location
// swagger:model NPCLocationResponse
type NPCLocationResponse struct {
	NPCID          string `json:"npc_id" example:"blacksmith-john"`
	Name           string `json:"name" example:"John"`
	Title          string `json:"title" example:"Blacksmith"`
	LocationType   string `json:"location_type" example:"building"` // "building" or "district"
	LocationID     string `json:"location_id" example:"forge"`
	State          string `json:"state" example:"working"`
	IsInteractable bool   `json:"is_interactable" example:"true"`
}

// GetNPCsAtLocationHandler godoc
// @Summary      Get NPCs at location
// @Description  Returns NPCs visible at player's current location and time of day
// @Tags         GameData
// @Produce      json
// @Param        location  query     string  true   "Location ID (e.g., kingdom)"
// @Param        district  query     string  false  "District ID (e.g., kingdom-center)"
// @Param        building  query     string  false  "Building ID (e.g., forge)"
// @Param        time      query     int     false  "Time of day in minutes (0-1439, default 720 = noon)"
// @Success      200       {array}   NPCLocationResponse
// @Failure      405       {string}  string  "Method not allowed"
// @Failure      500       {string}  string  "Database error"
// @Router       /npcs/at-location [get]
func GetNPCsAtLocationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	locationID := r.URL.Query().Get("location")
	districtID := r.URL.Query().Get("district")
	buildingID := r.URL.Query().Get("building")
	roomID := r.URL.Query().Get("room")
	timeOfDay := 720 // Default noon

	if t := r.URL.Query().Get("time"); t != "" {
		fmt.Sscanf(t, "%d", &timeOfDay)
	}

	database := db.GetDB()
	if database == nil {
		http.Error(w, "Database not available", http.StatusInternalServerError)
		return
	}

	// Get all NPCs for this location
	rows, err := database.Query("SELECT id, properties FROM npcs WHERE location = ?", locationID)
	if err != nil {
		http.Error(w, "Failed to query NPCs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	visibleNPCs := []NPCLocationResponse{}
	defaultRoom := buildingDefaultRoom(database, locationID, buildingID)

	// Note: districtID is already the full district ID (e.g., "kingdom-center") from frontend
	for rows.Next() {
		var npcID, propertiesJSON string
		if err := rows.Scan(&npcID, &propertiesJSON); err != nil {
			continue
		}

		var npcData types.NPCData
		if err := json.Unmarshal([]byte(propertiesJSON), &npcData); err != nil {
			continue
		}

		// Resolve schedule
		scheduleInfo := npc.ResolveNPCSchedule(&npcData, timeOfDay)

		// Determine location type from location ID
		locationType := npc.DetermineLocationType(scheduleInfo.Location)

		// Check if NPC is at player's current location (and room, when inside)
		isVisible := false
		if buildingID != "" && locationType == "building" && scheduleInfo.Location == buildingID {
			isVisible = NPCRoomMatches(scheduleInfo.Room, roomID, defaultRoom)
		} else if buildingID == "" && locationType == "district" && scheduleInfo.Location == districtID {
			isVisible = true
		}

		if isVisible {
			visibleNPCs = append(visibleNPCs, NPCLocationResponse{
				NPCID:          npcID,
				Name:           npcData.Name,
				Title:          npcData.Title,
				LocationType:   locationType,
				LocationID:     scheduleInfo.Location,
				State:          scheduleInfo.State,
				IsInteractable: scheduleInfo.IsAvailable,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(visibleNPCs)
}

// GetNPCIDsAtLocation returns a list of NPC IDs visible at the given location
// Used by the delta system to track NPC changes
func GetNPCIDsAtLocation(locationID, districtID, buildingID, roomID string, timeOfDay int) []string {
	database := db.GetDB()
	if database == nil {
		return []string{}
	}
	defaultRoom := buildingDefaultRoom(database, locationID, buildingID)

	// Get all NPCs for this location
	rows, err := database.Query("SELECT id, properties FROM npcs WHERE location = ?", locationID)
	if err != nil {
		return []string{}
	}
	defer rows.Close()

	visibleNPCs := []string{}

	// Construct full district ID (e.g., "kingdom" + "center" = "kingdom-center")
	fullDistrictID := locationID + "-" + districtID

	for rows.Next() {
		var npcID, propertiesJSON string
		if err := rows.Scan(&npcID, &propertiesJSON); err != nil {
			continue
		}

		var npcData types.NPCData
		if err := json.Unmarshal([]byte(propertiesJSON), &npcData); err != nil {
			continue
		}

		// Resolve schedule
		scheduleInfo := npc.ResolveNPCSchedule(&npcData, timeOfDay)

		// Determine location type from location ID
		locationType := npc.DetermineLocationType(scheduleInfo.Location)

		// Check if NPC is at player's current location (and room, when inside)
		isVisible := false
		if buildingID != "" && locationType == "building" && scheduleInfo.Location == buildingID {
			isVisible = NPCRoomMatches(scheduleInfo.Room, roomID, defaultRoom)
		} else if buildingID == "" && locationType == "district" && scheduleInfo.Location == fullDistrictID {
			isVisible = true
		}

		if isVisible {
			visibleNPCs = append(visibleNPCs, npcID)
		}
	}

	return visibleNPCs
}

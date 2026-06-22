package building

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// Rooms (M2): navigable spaces inside a building, in the city JSON as
// building.rooms[] + building.default_room. A building with no rooms behaves
// exactly as before — one implicit default room — so most buildings need no
// change. Rooms may be gated by an optional access (hours / key / state).

// Room is one space inside a building.
type Room struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Access      *RoomAccess `json:"access,omitempty"` // nil = always accessible
}

// RoomAccess gates a room. Every set check must pass; unset checks are open.
type RoomAccess struct {
	Hours *RoomHours `json:"hours,omitempty"` // time window (minutes), like building hours
	Key   string     `json:"key,omitempty"`   // required item ID in the player's inventory
	State string     `json:"state,omitempty"` // flag, e.g. "rented" — an active rental for this building
}

// RoomHours is an open/close window in minutes (0-1439), wrapping past midnight.
type RoomHours struct {
	Open  int `json:"open"`
	Close int `json:"close"`
}

// GetBuildingRooms returns a building's rooms and its default-room id. A building
// with no "rooms" returns (nil, "", nil); callers treat that as one implicit room.
func GetBuildingRooms(db *sql.DB, locationID, buildingID string) ([]Room, string, error) {
	b, err := findBuilding(db, locationID, buildingID)
	if err != nil {
		return nil, "", err
	}
	var rooms []Room
	if raw, ok := b["rooms"]; ok {
		data, _ := json.Marshal(raw)
		_ = json.Unmarshal(data, &rooms)
	}
	defaultRoom, _ := b["default_room"].(string)
	if defaultRoom == "" && len(rooms) > 0 {
		defaultRoom = rooms[0].ID
	}
	return rooms, defaultRoom, nil
}

// FindRoom returns the room with the given id.
func FindRoom(rooms []Room, roomID string) (Room, bool) {
	for _, r := range rooms {
		if r.ID == roomID {
			return r, true
		}
	}
	return Room{}, false
}

// RoomAccessible reports whether a room can be entered, with a player-facing
// reason when it can't. hasKey/isRented are supplied by the caller since the
// inventory and rentals live outside this package.
func RoomAccessible(room Room, timeOfDay int, hasKey, isRented bool) (bool, string) {
	a := room.Access
	if a == nil {
		return true, ""
	}
	if a.Hours != nil && !withinHours(a.Hours.Open, a.Hours.Close, timeOfDay) {
		return false, fmt.Sprintf("%s is closed right now.", roomLabel(room))
	}
	if a.Key != "" && !hasKey {
		return false, fmt.Sprintf("%s is locked — you need the right key.", roomLabel(room))
	}
	if a.State == "rented" && !isRented {
		return false, fmt.Sprintf("%s isn't yours — rent it first.", roomLabel(room))
	}
	return true, ""
}

func roomLabel(r Room) string {
	if r.Name != "" {
		return r.Name
	}
	return "That room"
}

// withinHours reports whether t is in [open, close), wrapping past midnight.
func withinHours(open, close, t int) bool {
	if close < open {
		return t >= open || t < close
	}
	return t >= open && t < close
}

// findBuilding locates a building's raw JSON object within a location's districts.
func findBuilding(db *sql.DB, locationID, buildingID string) (map[string]interface{}, error) {
	var propertiesJSON string
	if err := db.QueryRow("SELECT properties FROM locations WHERE id = ?", locationID).Scan(&propertiesJSON); err != nil {
		return nil, fmt.Errorf("location not found: %s", locationID)
	}
	var locationData map[string]interface{}
	if err := json.Unmarshal([]byte(propertiesJSON), &locationData); err != nil {
		return nil, fmt.Errorf("failed to parse location data: %v", err)
	}
	districts, ok := locationData["districts"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("location has no districts")
	}
	for _, dd := range districts {
		district, ok := dd.(map[string]interface{})
		if !ok {
			continue
		}
		buildings, ok := district["buildings"].([]interface{})
		if !ok {
			continue
		}
		for _, bd := range buildings {
			b, ok := bd.(map[string]interface{})
			if ok && b["id"] == buildingID {
				return b, nil
			}
		}
	}
	return nil, fmt.Errorf("building not found: %s", buildingID)
}

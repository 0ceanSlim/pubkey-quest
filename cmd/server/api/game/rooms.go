package game

import (
	"encoding/json"
	"net/http"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/gameutil"
)

// RoomView is one room of the current building as the UI needs it: name +
// whether the player can currently enter it (and why not, when locked).
type RoomView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Accessible   bool   `json:"accessible"`
	LockedReason string `json:"locked_reason,omitempty"`
	IsCurrent    bool   `json:"is_current"`
}

// RoomsResponse lists the rooms in the player's current building.
type RoomsResponse struct {
	Success     bool       `json:"success"`
	Building    string     `json:"building"`
	CurrentRoom string     `json:"current_room"`
	Rooms       []RoomView `json:"rooms"`
}

// GetRoomsHandler godoc
// @Summary      Get rooms in the current building
// @Description  Returns the rooms of the building the player is in, each marked with
//               whether it's currently accessible (hours / key / rental) and which one
//               the player occupies. Empty when the player isn't in a building or the
//               building has no rooms.
// @Tags         Rooms
// @Produce      json
// @Param        npub     query     string  true  "Nostr public key"
// @Param        save_id  query     string  true  "Save ID"
// @Success      200      {object}  RoomsResponse
// @Failure      404      {string}  string  "Session not found"
// @Router       /api/rooms [get]
func GetRoomsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	npub := r.URL.Query().Get("npub")
	saveID := r.URL.Query().Get("save_id")
	if npub == "" || saveID == "" {
		http.Error(w, "Missing query params: npub, save_id", http.StatusBadRequest)
		return
	}

	sess := getSessionAndValidate(w, npub, saveID)
	if sess == nil {
		return
	}
	save := sess.GetSaveData()

	resp := RoomsResponse{Success: true, Building: save.Building, CurrentRoom: save.Room, Rooms: []RoomView{}}
	if save.Building != "" {
		if rooms, _, err := building.GetBuildingRooms(serverdb.GetDB(), save.Location, save.Building); err == nil {
			for _, room := range rooms {
				hasKey := room.Access != nil && room.Access.Key != "" && gameutil.PlayerHasItem(save, room.Access.Key)
				isRented := gameutil.HasActiveRental(save, save.Building)
				accessible, reason := building.RoomAccessible(room, save.TimeOfDay, hasKey, isRented)
				resp.Rooms = append(resp.Rooms, RoomView{
					ID:           room.ID,
					Name:         room.Name,
					Description:  room.Description,
					Accessible:   accessible,
					LockedReason: reason,
					IsCurrent:    room.ID == save.Room,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

package game

import (
	"encoding/json"
	"net/http"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/types"
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
	// Sleepable marks the rented room itself — the only place Sleep works
	// (game/npc/housing.go), so the UI offers Sleep only there.
	Sleepable bool `json:"sleepable,omitempty"`
	// Pickable: the door is locked to you, its lock can be picked, and it isn't
	// jammed today. Jammed: a pick jammed it today (only the key works).
	Pickable bool `json:"pickable,omitempty"`
	Jammed   bool `json:"jammed,omitempty"`
}

// RoomsResponse lists the rooms in the player's current building.
type RoomsResponse struct {
	Success     bool       `json:"success"`
	Building    string     `json:"building"`
	CurrentRoom string     `json:"current_room"`
	DefaultRoom string     `json:"default_room"` // the common/entrance room — where "Exit Room" returns you
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
		if rooms, defaultRoom, err := building.GetBuildingRooms(serverdb.GetDB(), save.Location, save.Building); err == nil {
			resp.DefaultRoom = defaultRoom
			for _, room := range rooms {
				key := building.RoomKey(room)
				lockID, day := roomLockID(save, room.ID), save.CurrentDay
				unlocked := sess.Locks.IsOpen(lockID, day) || (key != "" && gameutil.PlayerHasItem(save, key))
				isRented := gameutil.HasActiveRental(save, save.Building)
				accessible, reason := building.RoomAccessible(room, save.TimeOfDay, unlocked, isRented)
				lock := (*types.Lock)(nil)
				if room.Access != nil {
					lock = room.Access.Lock
				}
				jammed := lock != nil && sess.Locks.IsJammed(lockID, day)
				resp.Rooms = append(resp.Rooms, RoomView{
					ID:           room.ID,
					Name:         room.Name,
					Description:  room.Description,
					Accessible:   accessible,
					LockedReason: reason,
					IsCurrent:    room.ID == save.Room,
					Sleepable:    room.Sleep || (isRented && room.Access != nil && room.Access.State == "rented"),
					Pickable:     !unlocked && lock != nil && lock.DC > 0 && !jammed,
					Jammed:       !unlocked && jammed,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

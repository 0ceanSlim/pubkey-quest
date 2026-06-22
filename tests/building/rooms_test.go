package building_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/building"
)

func TestRoomAccessible_Open(t *testing.T) {
	// No access block → always open.
	if ok, _ := building.RoomAccessible(building.Room{ID: "common", Name: "Common Room"}, 720, false, false); !ok {
		t.Error("a room with no access should be open")
	}
}

func TestRoomAccessible_Hours(t *testing.T) {
	night := building.Room{ID: "cellar", Name: "Cellar", Access: &building.RoomAccess{
		Hours: &building.RoomHours{Open: 1320, Close: 360}, // 22:00–06:00, wraps midnight
	}}
	if ok, _ := building.RoomAccessible(night, 1380, false, false); !ok { // 23:00
		t.Error("should be open at 23:00 within an overnight window")
	}
	if ok, _ := building.RoomAccessible(night, 720, false, false); ok { // noon
		t.Error("should be closed at noon outside the window")
	}
}

func TestRoomAccessible_Key(t *testing.T) {
	vault := building.Room{ID: "strongroom", Name: "Strongroom", Access: &building.RoomAccess{Key: "iron-key"}}
	if ok, reason := building.RoomAccessible(vault, 720, false, false); ok || reason == "" {
		t.Errorf("locked without key should be denied with a reason, got ok=%v reason=%q", ok, reason)
	}
	if ok, _ := building.RoomAccessible(vault, 720, true, false); !ok {
		t.Error("holding the key should open the room")
	}
}

func TestRoomAccessible_RentedState(t *testing.T) {
	guest := building.Room{ID: "guest_1", Name: "Guest Room", Access: &building.RoomAccess{State: "rented"}}
	if ok, _ := building.RoomAccessible(guest, 720, false, false); ok {
		t.Error("a rented-state room should be locked until rented")
	}
	if ok, _ := building.RoomAccessible(guest, 720, false, true); !ok {
		t.Error("an active rental should unlock the room")
	}
}

func TestFindRoom(t *testing.T) {
	rooms := []building.Room{{ID: "common"}, {ID: "kitchen"}}
	if _, ok := building.FindRoom(rooms, "kitchen"); !ok {
		t.Error("should find an existing room")
	}
	if _, ok := building.FindRoom(rooms, "attic"); ok {
		t.Error("should not find a missing room")
	}
}

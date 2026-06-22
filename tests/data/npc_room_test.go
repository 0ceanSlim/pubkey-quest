package data_test

import (
	"testing"

	"pubkey-quest/cmd/server/api/data"
)

// NPC room visibility: a slot with no room means the building's default room;
// an empty player room means the building has no rooms (today's behavior).
func TestNPCRoomMatches(t *testing.T) {
	cases := []struct {
		name                          string
		slotRoom, playerRoom, defRoom string
		want                          bool
	}{
		{"no rooms in building", "", "", "", true},
		{"no-room NPC in default room", "", "common_room", "common_room", true},
		{"no-room NPC not in side room", "", "kitchen", "common_room", false},
		{"roomed NPC, player in that room", "kitchen", "kitchen", "common_room", true},
		{"roomed NPC, player elsewhere", "kitchen", "common_room", "common_room", false},
	}
	for _, c := range cases {
		if got := data.NPCRoomMatches(c.slotRoom, c.playerRoom, c.defRoom); got != c.want {
			t.Errorf("%s: NPCRoomMatches(%q,%q,%q) = %v, want %v", c.name, c.slotRoom, c.playerRoom, c.defRoom, got, c.want)
		}
	}
}

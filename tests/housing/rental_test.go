package housing_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/npc"
	"pubkey-quest/types"
)

func TestRentals(t *testing.T) {
	state := &types.SaveFile{CurrentDay: 5, TimeOfDay: 1300}

	if gameutil.HasActiveRental(state, "sailors_tavern") {
		t.Error("no rental yet")
	}

	gameutil.AddRental(state, "sailors_tavern", 6, 1439) // through day 6
	if !gameutil.HasActiveRental(state, "sailors_tavern") {
		t.Error("rental should be active")
	}
	if gameutil.HasActiveRental(state, "miners_inn") {
		t.Error("rental is building-specific")
	}

	// Renting again at the same building replaces, not duplicates.
	gameutil.AddRental(state, "sailors_tavern", 7, 1439)
	if len(state.Rentals) != 1 {
		t.Errorf("expected 1 rental after re-rent, got %d", len(state.Rentals))
	}

	// Expiry: a rental from a past day is inactive.
	past := &types.SaveFile{CurrentDay: 10, TimeOfDay: 600, Rentals: []types.Rental{{Building: "sailors_tavern", ExpiresDay: 6, ExpiresMin: 1439}}}
	if gameutil.HasActiveRental(past, "sailors_tavern") {
		t.Error("expired rental should be inactive")
	}
}

func TestCanSleepNow(t *testing.T) {
	cases := []struct {
		time int
		want bool
	}{
		{1260, true},  // 9:00 PM — earliest
		{1380, true},  // 11:00 PM
		{60, true},    // 1:00 AM
		{359, true},   // 5:59 AM — last minute before wake
		{360, false},  // 6:00 AM — wake time, too late to start
		{720, false},  // noon
		{1259, false}, // 8:59 PM — one minute too early
	}
	for _, c := range cases {
		if got := npc.CanSleepNow(c.time); got != c.want {
			t.Errorf("CanSleepNow(%d) = %v, want %v", c.time, got, c.want)
		}
	}
}

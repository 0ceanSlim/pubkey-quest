package game

import (
	"testing"
	"time"

	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// A tick moves the clock only as far as real time allows (beta exploit audit D4).
func TestClockKeepsItsOwnPace(t *testing.T) {
	sess := &session.GameSession{}
	state := &types.SaveFile{CurrentDay: 5, TimeOfDay: 600}
	at := time.Unix(1_000_000, 0)
	tick := func(day, tod int, after time.Duration) int {
		at = at.Add(after)
		p := map[string]interface{}{"current_day": float64(day), "time_of_day": float64(tod)}
		clampTick(sess, state, p, at)
		got := int(p["current_day"].(float64))*1440 + int(p["time_of_day"].(float64))
		moved := got - (state.CurrentDay*1440 + state.TimeOfDay)
		state.CurrentDay, state.TimeOfDay = got/1440, got%1440
		return moved
	}
	if moved := tick(8, 600, 0); moved != int(clockBank) {
		t.Fatalf("three days asked on the first tick: moved %d minutes", moved)
	}
	// One real second later: about 2.4 game minutes (with slack), not 10.
	if moved := tick(state.CurrentDay, state.TimeOfDay+10, time.Second); moved < 2 || moved > 4 {
		t.Fatalf("10 minutes asked after a second: moved %d", moved)
	}
	// A normal tick (417 ms, one game minute) always goes through.
	for range 50 {
		if moved := tick(state.CurrentDay, state.TimeOfDay+1, 417*time.Millisecond); moved != 1 {
			t.Fatalf("a normal tick moved %d", moved)
		}
	}
	// A long pause banks no more than clockBank.
	if moved := tick(state.CurrentDay+1, state.TimeOfDay, time.Hour); moved != int(clockBank) {
		t.Fatalf("a day asked after an hour away: moved %d", moved)
	}
}

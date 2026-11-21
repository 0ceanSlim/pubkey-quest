package game

import (
	"strings"
	"testing"
)

// A signal whistle calls a road fight where you stand, in the wild only, and
// never inside the road's cooldown.
func TestBlowTheWhistle(t *testing.T) {
	sess := forgeSession(t, carrying("signal-whistle", 1))
	state := &sess.SaveData
	if r := fishAction(t, sess, "blow_whistle", nil); r.Success {
		t.Fatalf("blown in the forge: %+v", r)
	}

	state.Location, state.District, state.Building, state.TravelProgress = "greywind-plains", "", "", 0.4
	now := state.CurrentDay*1440 + state.TimeOfDay
	state.Lim().LastEncounter = now - 60
	r := fishAction(t, sess, "blow_whistle", nil)
	if !r.Success || !strings.Contains(r.Message, "Nothing answers") || r.Data["combat_started"] != nil || r.Data["encounter_unaware"] != nil {
		t.Fatalf("an hour after a fight: %+v", r)
	}

	state.Lim().LastEncounter = 0
	r = fishAction(t, sess, "blow_whistle", nil)
	if !r.Success || (r.Data["combat_started"] == nil && r.Data["encounter_unaware"] == nil) {
		t.Fatalf("no fight came: %+v", r)
	}
	if state.Lim().LastEncounter != now {
		t.Fatalf("the cooldown wasn't started: last encounter %d, now %d", state.Lim().LastEncounter, now)
	}
	sess.ActiveCombat, sess.PendingEncounter = nil, nil

	general := state.Inventory["general_slots"].([]interface{})
	general[0] = nil
	state.Lim().LastEncounter = 0
	if r := fishAction(t, sess, "blow_whistle", nil); r.Success {
		t.Fatalf("blown without a whistle: %+v", r)
	}
}

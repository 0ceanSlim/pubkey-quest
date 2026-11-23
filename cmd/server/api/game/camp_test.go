package game

import "testing"

// Making camp on the road stops the trip: you wake where you lay down, not
// further along (or arrived) for the hours you slept.
func TestCampingStopsTheTrip(t *testing.T) {
	sess := bobSession(t)
	sess.SaveData.Location, sess.SaveData.District, sess.SaveData.Building = "merchants-highway", "kingdom-east", ""
	sess.SaveData.TimeOfDay, sess.SaveData.TravelProgress, sess.SaveData.TravelStopped = 1300, 0.4, false
	sess.PlaceKey = "road"

	r := lockAction(t, sess, "sleep", nil)
	if !r.Success {
		t.Fatalf("can't camp on the road: %s", r.Message)
	}
	if s := sess.SaveData; s.Location != "merchants-highway" || s.TravelProgress != 0.4 || !s.TravelStopped {
		t.Errorf("woke at %s, %.0f%% along, stopped=%v; want where you lay down, stopped",
			s.Location, s.TravelProgress*100, s.TravelStopped)
	}
}

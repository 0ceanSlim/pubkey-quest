package types

// Lock is a lock on a door, gate or chest: a room's door (RoomAccess.Lock) or a
// lock node inside a POI / quest site (POIStep.Lock).
//
//	{"key":"silver-key"}                         only the key opens it
//	{"key":"anchor-spare-key","dc":18}           the key, or a pick at DC 18
//	{"dc":12,"jam_on_miss":5}                    no key exists; a pick that misses
//	                                             by 5 or more jams it for the day
//
// Every failed pick breaks a lockpick. A jammed lock can't be picked again until
// the next in-game day; its key still works.
type Lock struct {
	Key       string `json:"key,omitempty"`         // item that opens it
	DC        int    `json:"dc,omitempty"`          // Thieving DC to pick; 0 = can't be picked
	JamOnMiss int    `json:"jam_on_miss,omitempty"` // miss by this much or more and it jams (0 = never)
}

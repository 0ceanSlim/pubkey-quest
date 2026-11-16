package types

// Limits is what paces a character's repeatable things: the rock faces worked
// and fish landed today, the locks picked or jammed, a check's run of
// failures, when authored encounters last fired, a POI's spent rewards, the
// work toward the next point of fatigue, the day's show. It lived only in the
// server's memory once, so saving, reloading and coming back reset it all
// (beta exploit audit D2/D3). It's kept with the save: whatever state a save
// is in, its limits match it. None of it can be derived from anything else.
//
// Times are a day number (CurrentDay) or an absolute game minute
// (day*1440 + time of day). Prune drops what no longer limits anything.
type Limits struct {
	Seams  map[string]SeamDay  `json:"seams,omitempty"`  // mine node → faces worked that day
	Fish   map[string]FishDay  `json:"fish,omitempty"`   // fishing spot → fish landed that day
	Locks  map[string]LockDay  `json:"locks,omitempty"`  // lock id → picked open / jammed that day
	Checks map[string]CheckTry `json:"checks,omitempty"` // "<walk>/<node>" → failures and cooldown

	Encounters    map[string]int `json:"encounters,omitempty"`     // encounter id → minute it last fired
	OneShots      []string       `json:"one_shots,omitempty"`      // non-repeatable encounters that have fired
	LastEncounter int            `json:"last_encounter,omitempty"` // minute of the last road or authored encounter

	Spent map[string]int `json:"spent,omitempty"` // "<poi>/<node>" → minute a POI's reward, loot or fight was had

	Exertion int `json:"exertion,omitempty"` // hard work toward the next point of fatigue
	ShowDay  int `json:"show_day,omitempty"` // the day a show was last played
}

// SeamDay is a mine node's faces worked on a day.
type SeamDay struct {
	Day    int `json:"d"`
	Worked int `json:"n"`
}

// FishDay is a fishing spot's fish landed on a day.
type FishDay struct {
	Day    int `json:"d"`
	Landed int `json:"n"`
}

// LockDay is a lock picked open or jammed on a day.
type LockDay struct {
	Day    int  `json:"d"`
	Open   bool `json:"open,omitempty"`
	Jammed bool `json:"jammed,omitempty"`
}

// CheckTry is a check node's run of failures, and when it opens again.
type CheckTry struct {
	Fails       int `json:"fails,omitempty"`
	LockedUntil int `json:"until,omitempty"` // absolute game minute; 0 = open
}

// How long spent things are remembered. A POI is fresh again after
// SpentMinutes (docs/poi-quest-design.md §3); encounter cooldowns are at most
// EncounterMemoryMinutes (the ethereal caravan's week).
const (
	SpentMinutes           = 3 * 1440
	EncounterMemoryMinutes = 7 * 1440
)

// Lim returns the save's limits, making them on first use.
func (s *SaveFile) Lim() *Limits {
	if s.Limits == nil {
		s.Limits = &Limits{}
	}
	return s.Limits
}

// Prune drops what no longer limits anything at day / minute now: other
// days' seams, fish and locks, opened checks with no failures, cooled-down
// encounters and spent POI nodes. One-shot encounters are kept for good.
func (l *Limits) Prune(day, now int) {
	if l == nil {
		return
	}
	for id, d := range l.Seams {
		if d.Day != day {
			delete(l.Seams, id)
		}
	}
	for id, d := range l.Fish {
		if d.Day != day {
			delete(l.Fish, id)
		}
	}
	for id, d := range l.Locks {
		if d.Day != day {
			delete(l.Locks, id)
		}
	}
	for id, c := range l.Checks {
		if c.LockedUntil <= now && c.Fails == 0 {
			delete(l.Checks, id)
		}
	}
	for id, at := range l.Encounters {
		if now-at >= EncounterMemoryMinutes {
			delete(l.Encounters, id)
		}
	}
	for id, at := range l.Spent {
		if now-at >= SpentMinutes {
			delete(l.Spent, id)
		}
	}
}

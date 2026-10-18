// Package presence tracks which players are online and where, so players who
// share a spot can see each other (M5.6 P3).
//
// Every game session publishes a small snapshot of itself here after each
// action. Other players' lookups read only these snapshots — never another
// session's live save data — so cross-player reads can't race the owner's
// own requests.
package presence

import (
	"sort"
	"sync"
	"time"
)

const (
	// OnlineWindow is how recently a player must have been seen to count as
	// online. The client heartbeats every few seconds even while its clock is
	// paused (combat, menus), so this is a generous margin, not a timeout.
	OnlineWindow = 20 * time.Second
	// TimeWindowMinutes is how far apart two players' in-game clocks may be and
	// still see each other. Clocks are per save, so two people at the same inn at
	// 10:00 and 10:45 meet; at 10:00 and 13:00 they're in different hours of the day.
	TimeWindowMinutes = 60
)

// Entry is one player's latest presence snapshot.
type Entry struct {
	Npub   string
	SaveID string
	Name   string // the character's name
	Race   string
	Class  string
	Level  int
	// Spot is the normalized place key — location|district|building|room, with the
	// building's default room filled in. Empty while travelling: players on the
	// road don't appear anywhere.
	Spot       string
	AbsMinutes int // CurrentDay*1440 + TimeOfDay
	InCombat   bool
	// ClockRunning is the player's play/pause choice; a party follower's clock
	// follows their leader's. ClockHeld: something stops it for now (a POI walk
	// or an unaware choice; a fight is InCombat).
	ClockRunning bool
	ClockHeld    bool
	LastSeen     time.Time

	// Location is the city or road (environment) id — the "area" for the
	// headcount and the Area list. AreaName is its display name.
	Location string
	AreaName string
	// Where describes the spot for other players: "Salty Anchor · Taproom",
	// "East District", "on the road".
	Where string
	// Route / RoutePos place a traveller on a road: the environment id and a
	// position 0–1 measured from the road's first end (so two travellers heading
	// opposite ways compare correctly). Route is "" when not travelling.
	Route    string
	RoutePos float64
	// Portrait is the character's chosen portrait URL ("" for none).
	Portrait string
}

// Online reports whether the entry was seen within OnlineWindow of now.
func (e Entry) Online(now time.Time) bool {
	return now.Sub(e.LastSeen) <= OnlineWindow
}

var registry = struct {
	sync.RWMutex
	byNpub map[string]Entry // one entry per player: their most recently active save
}{byNpub: map[string]Entry{}}

// Publish records a player's snapshot, replacing any earlier one. A player with
// several saves loaded shows as whichever was active last.
func Publish(e Entry) {
	registry.Lock()
	registry.byNpub[e.Npub] = e
	registry.Unlock()
}

// Remove forgets a player (logout / session cleanup).
func Remove(npub string) {
	registry.Lock()
	delete(registry.byNpub, npub)
	registry.Unlock()
}

// Get returns a player's snapshot.
func Get(npub string) (Entry, bool) {
	registry.RLock()
	defer registry.RUnlock()
	e, ok := registry.byNpub[npub]
	return e, ok
}

// Here returns the other online players at spot whose in-game clock is within
// TimeWindowMinutes of absMinutes, sorted by character name. self is excluded.
func Here(self, spot string, absMinutes int, now time.Time) []Entry {
	if spot == "" {
		return nil
	}
	registry.RLock()
	defer registry.RUnlock()
	var out []Entry
	for npub, e := range registry.byNpub {
		if npub == self || e.Spot != spot || !e.Online(now) {
			continue
		}
		diff := e.AbsMinutes - absMinutes
		if diff < 0 {
			diff = -diff
		}
		if diff > TimeWindowMinutes {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// InArea returns every other online player in the same location (city or
// road), any spot, any clock — the Area list. Sorted by name.
func InArea(self, location string, now time.Time) []Entry {
	if location == "" {
		return nil
	}
	registry.RLock()
	defer registry.RUnlock()
	var out []Entry
	for npub, e := range registry.byNpub {
		if npub == self || e.Location != location || !e.Online(now) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RoadNearby is how close (as a fraction of the road) two travellers must be
// to see each other.
const RoadNearby = 0.05

// NearbyOnRoad returns other online travellers on the same road within
// RoadNearby of pos whose clocks are within TimeWindowMinutes.
func NearbyOnRoad(self, route string, pos float64, absMinutes int, now time.Time) []Entry {
	if route == "" {
		return nil
	}
	registry.RLock()
	defer registry.RUnlock()
	var out []Entry
	for npub, e := range registry.byNpub {
		if npub == self || e.Route != route || !e.Online(now) {
			continue
		}
		d := e.RoutePos - pos
		if d < 0 {
			d = -d
		}
		diff := e.AbsMinutes - absMinutes
		if diff < 0 {
			diff = -diff
		}
		if d > RoadNearby || diff > TimeWindowMinutes {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// StartPruner drops players unseen for 10 minutes, once a minute.
func StartPruner() {
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for now := range t.C {
			Prune(10*time.Minute, now)
		}
	}()
}

// OnlineCount is how many players are online right now.
func OnlineCount(now time.Time) int {
	registry.RLock()
	defer registry.RUnlock()
	n := 0
	for _, e := range registry.byNpub {
		if e.Online(now) {
			n++
		}
	}
	return n
}

// OnlineSet reports which of npubs are online right now — for the friends list.
func OnlineSet(npubs []string, now time.Time) map[string]Entry {
	registry.RLock()
	defer registry.RUnlock()
	out := map[string]Entry{}
	for _, n := range npubs {
		if e, ok := registry.byNpub[n]; ok && e.Online(now) {
			out[n] = e
		}
	}
	return out
}

// Prune drops entries not seen for a long while, so the registry doesn't grow
// with every player who ever logged in.
func Prune(olderThan time.Duration, now time.Time) {
	registry.Lock()
	defer registry.Unlock()
	for npub, e := range registry.byNpub {
		if now.Sub(e.LastSeen) > olderThan {
			delete(registry.byNpub, npub)
		}
	}
}

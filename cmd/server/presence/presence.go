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
	LastSeen   time.Time
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

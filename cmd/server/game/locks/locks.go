// Package locks resolves picking a lock (types.Lock) and remembers, per session,
// which locks are picked open or jammed today. Lock state is memory only and
// lasts the in-game day: a lock picked open relocks overnight, and a jammed one
// frees up the next day. Nothing about it is saved.
package locks

import (
	"fmt"
	"math/rand"

	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// Lockpick is the item a pick attempt uses up when it fails.
const Lockpick = "lockpick"

// PickMinutes is the in-game time one attempt takes.
const PickMinutes = 10

// DayState is what's happened to one lock today.
type DayState struct {
	Day    int
	Open   bool
	Jammed bool
}

// Memory is a session's lock state, by lock id ("room:goldenhaven/sailors_tavern/
// spare_room", "poi:<walk>/<node>").
type Memory map[string]DayState

func (m Memory) today(id string, day int) DayState {
	if s, ok := m[id]; ok && s.Day == day {
		return s
	}
	return DayState{Day: day}
}

// IsOpen reports whether the lock was picked open today.
func (m Memory) IsOpen(id string, day int) bool { return m.today(id, day).Open }

// IsJammed reports whether the lock was jammed today.
func (m Memory) IsJammed(id string, day int) bool { return m.today(id, day).Jammed }

// Set records today's state of a lock.
func (m Memory) Set(id string, day int, open, jammed bool) {
	m[id] = DayState{Day: day, Open: open, Jammed: jammed}
}

// Result is the outcome of one pick attempt.
type Result struct {
	Opened bool
	Jammed bool // the attempt failed badly enough to jam the lock
	Check  skillcheck.Result
}

// Pick rolls one attempt at a lock with the given Thieving value.
func Pick(lock types.Lock, thieving int, rng *rand.Rand) Result {
	r := skillcheck.Resolve(thieving, lock.DC, rng)
	res := Result{Opened: r.Success, Check: r}
	// The failed pick breaks; whether it jams is the same check a minigame's
	// broken pick makes (JamCheck), so neither way of picking jams more.
	if !r.Success {
		res.Jammed = JamCheck(lock, thieving, rng)
	}
	return res
}

// Describe is the player-facing line for an attempt.
func (r Result) Describe() string {
	roll := fmt.Sprintf("(Thieving %d vs DC %d)", r.Check.Total, r.Check.DC)
	switch {
	case r.Opened:
		return "The lock clicks open. " + roll
	case r.Jammed:
		return "Your pick snaps off inside the lock, jamming it. Only the key will open it today. " + roll
	default:
		return "Your pick snaps. The lock holds. " + roll
	}
}

// CanPick reports why a lock can't be picked right now ("" when it can).
func CanPick(lock types.Lock, jammed bool, picks int) string {
	switch {
	case lock.DC <= 0:
		return "This lock can't be picked."
	case jammed:
		return "The lock is jammed. Only the key will open it today."
	case picks <= 0:
		return "You have no lockpicks."
	}
	return ""
}

package locks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"

	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// ─── Lock sessions: picking a lock as a minigame ─────────────────────────────
//
// A Session is one sitting at a lock, played turn by turn. The server generates
// the hidden puzzle when the session starts, resolves every action against it,
// and only ever hands the client the public View — never the solution. Each
// accepted action bumps Version; an action carrying an old version is refused,
// so a replayed or duplicated request can't apply a cost or a success twice.
//
// The session decides what happened (a pin set, the pick broke, the lock
// jammed, it opened); the caller applies the consequences to the game — using
// up the lockpick, passing time, remembering the lock open or jammed today,
// moving a POI walk on. Rules shared with the quick roll (Pick): a failure
// breaks one pick, and a broken pick runs the same jam check.
//
// Styles plug in through the puzzle interface. Tumblers is built; Sweet Spot
// and Wards would each be another puzzle implementation.

// Style is a lock minigame.
type Style string

const (
	StyleTumblers Style = "tumblers"
)

// Status is where a session stands.
type Status string

const (
	StatusActive    Status = "active"
	StatusOpened    Status = "opened"
	StatusJammed    Status = "jammed"
	StatusNoPicks   Status = "no_picks"
	StatusAbandoned Status = "abandoned"
)

// Target is what's being picked: a room's door, or a lock node in the player's
// current POI / quest-site walk.
type Target struct {
	Kind   string `json:"kind"` // "room" | "poi"
	RoomID string `json:"room_id,omitempty"`
	Node   string `json:"node,omitempty"`
}

// Action is one player move.
type Action struct {
	Kind string `json:"act"`           // "lift" | "inspect" | "give_up"
	Pin  int    `json:"pin,omitempty"` // lift: which pin (0-based)
}

// Outcome is what an action did, for the caller to apply.
type Outcome struct {
	Opened    bool
	PickBroke bool
	Jammed    bool
	GaveUp    bool
	// PickWorked: the pick in hand was used at all (a lift was tried), so
	// giving up still costs the time spent.
	PickWorked bool
	Lines      []string
}

// Session is one sitting at a lock.
type Session struct {
	ID       string
	LockID   string
	Name     string
	Target   Target
	Style    Style
	Lock     types.Lock
	Thieving int

	Version   int
	Status    Status
	Strain    int
	StrainMax int
	Broken    int  // picks broken this session
	Inspected bool // inspect used with the current pick
	Worked    bool // the current pick has been tried

	puzzle puzzle
	rng    *mrand.Rand
}

// puzzle is one minigame style's rules over its hidden state.
type puzzle interface {
	// reseat starts a fresh pick: the puzzle is laid out anew.
	reseat(s *Session)
	// lift tries a move; it reports whether it was a mistake (strain) and
	// whether the lock is now open.
	lift(s *Session, pin int) (mistake, opened bool, line string, err error)
	// inspect reveals what a successful inspection earns.
	inspect(s *Session) string
	// view is the public state — never the solution.
	view(s *Session) map[string]any
}

// NewSession lays out a lock for picking. thieving is the player's effective
// Thieving (kit bonus included); rng is injectable for tests.
func NewSession(lockID, name string, target Target, style Style, lock types.Lock, thieving int, rng *mrand.Rand) (*Session, error) {
	if lock.DC <= 0 {
		return nil, fmt.Errorf("this lock can't be picked")
	}
	s := &Session{
		ID: newID(), LockID: lockID, Name: name, Target: target, Style: style,
		Lock: lock, Thieving: thieving, Status: StatusActive, rng: rng,
	}
	switch style {
	case StyleTumblers, "":
		s.Style = StyleTumblers
		s.puzzle = newTumblers(lock.DC)
	default:
		return nil, fmt.Errorf("unknown lock style %q", style)
	}
	s.StrainMax = s.puzzle.(*tumblers).strainBudget(thieving)
	s.puzzle.reseat(s)
	return s, nil
}

// Act resolves one action. version must be the session's current Version.
// Errors leave the session untouched (an invalid pin, a stale version, a closed
// session); a nil error means the action happened and Version moved on.
func (s *Session) Act(a Action, version int) (Outcome, error) {
	var out Outcome
	if s.Status != StatusActive {
		return out, fmt.Errorf("this lock session is over")
	}
	if version != s.Version {
		return out, ErrStale
	}

	switch a.Kind {
	case "give_up":
		s.Status = StatusAbandoned
		out.GaveUp, out.PickWorked = true, s.Worked
		out.Lines = append(out.Lines, "You ease the pick out and step back from the lock.")

	case "inspect":
		if s.Inspected {
			return out, fmt.Errorf("you've already studied the lock with this pick")
		}
		s.Inspected = true
		r := skillcheck.Resolve(s.Thieving, s.Lock.DC, s.rng)
		if r.Success {
			out.Lines = append(out.Lines, s.puzzle.inspect(s))
		} else {
			out.Lines = append(out.Lines, "You peer into the keyway, but the pins give nothing away.")
		}

	case "lift":
		mistake, opened, line, err := s.puzzle.lift(s, a.Pin)
		if err != nil {
			return out, err
		}
		s.Worked = true
		out.Lines = append(out.Lines, line)
		if opened {
			s.Status = StatusOpened
			out.Opened = true
			break
		}
		if mistake {
			s.Strain++
			if s.Strain >= s.StrainMax {
				out = s.breakPick(out)
			} else if s.Strain == s.StrainMax-1 {
				out.Lines = append(out.Lines, "The pick bends. One more slip and it will snap.")
			}
		}

	default:
		return out, fmt.Errorf("unknown lock action %q", a.Kind)
	}
	s.Version++
	return out, nil
}

// breakPick snaps the pick in hand: the same jam check as a failed quick roll,
// then the pins reseat for the next pick.
func (s *Session) breakPick(out Outcome) Outcome {
	out.PickBroke = true
	s.Broken++
	if JamCheck(s.Lock, s.Thieving, s.rng) {
		out.Jammed = true
		s.Status = StatusJammed
		out.Lines = append(out.Lines, "Your pick snaps off inside the lock, jamming it. Only the key will open it today.")
		return out
	}
	out.Lines = append(out.Lines, "Your pick snaps. The pins drop and reseat.")
	s.Strain, s.Inspected, s.Worked = 0, false, false
	s.puzzle.reseat(s)
	return out
}

// OutOfPicks ends the session when the player has no lockpick left to try.
func (s *Session) OutOfPicks() {
	if s.Status == StatusActive {
		s.Status = StatusNoPicks
	}
}

// View is the public state of the session for the client.
func (s *Session) View() map[string]any {
	v := map[string]any{
		"session_id": s.ID,
		"name":       s.Name,
		"style":      s.Style,
		"version":    s.Version,
		"status":     s.Status,
		"strain":     s.Strain,
		"strain_max": s.StrainMax,
		"broken":     s.Broken,
		"inspected":  s.Inspected,
		"target":     s.Target,
		"tier":       tierName(s.Lock.DC),
	}
	for k, val := range s.puzzle.view(s) {
		v[k] = val
	}
	return v
}

// ErrStale is returned for an action made against an old version of the session.
var ErrStale = fmt.Errorf("that move was already made")

// JamCheck is the jam rule shared by both ways of picking: a pick that breaks
// jams the lock when a Thieving check misses its DC by JamOnMiss or more.
func JamCheck(lock types.Lock, thieving int, rng *mrand.Rand) bool {
	if lock.JamOnMiss <= 0 {
		return false
	}
	r := skillcheck.Resolve(thieving, lock.DC, rng)
	return !r.Success && lock.DC-r.Total >= lock.JamOnMiss
}

func tierName(dc int) string {
	switch {
	case dc <= 10:
		return "Easy"
	case dc <= 13:
		return "Ordinary"
	case dc <= 16:
		return "Difficult"
	}
	return "Expert"
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

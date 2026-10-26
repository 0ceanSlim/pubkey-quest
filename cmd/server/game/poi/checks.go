package poi

import (
	"fmt"
	"strings"

	"pubkey-quest/cmd/server/game/skillcheck"
	"pubkey-quest/types"
)

// A check node can't be retried forever: after a run of failures it closes for
// a while (types.POIStep Attempts / CooldownMinutes, these when unset).
const (
	DefaultCheckAttempts = 3
	DefaultCheckCooldown = 240 // game minutes
)

// CheckAttempts is how many failures in a row a check node allows.
func CheckAttempts(node types.POIStep) int {
	if node.Attempts > 0 {
		return node.Attempts
	}
	return DefaultCheckAttempts
}

// CheckCooldown is how long (game minutes) a check stays closed once its
// attempts are spent.
func CheckCooldown(node types.POIStep) int {
	if node.CooldownMinutes > 0 {
		return node.CooldownMinutes
	}
	return DefaultCheckCooldown
}

// TryState is one check node's run of failures, and when it opens again.
type TryState struct {
	Fails       int
	LockedUntil int // absolute game minute (day*1440 + time); 0 = open
}

// CheckTries is a session's check retry state, by check id
// ("<walk id>/<node id>"). Memory only, like lock state.
type CheckTries map[string]TryState

// Closed reports whether the check is still cooling down at game minute now,
// and how many minutes are left.
func (t CheckTries) Closed(id string, now int) (bool, int) {
	s := t[id]
	if s.LockedUntil > now {
		return true, s.LockedUntil - now
	}
	return false, 0
}

// Record notes a roll of the check and returns the failures still allowed
// before it closes (0 once it just has). A success clears the run.
func (t CheckTries) Record(id string, node types.POIStep, success bool, now int) int {
	if success {
		delete(t, id)
		return 0
	}
	s := t[id]
	if s.LockedUntil != 0 && s.LockedUntil <= now {
		s = TryState{} // the last cooldown ran out: a fresh run
	}
	s.Fails++
	left := CheckAttempts(node) - s.Fails
	if left <= 0 {
		s = TryState{LockedUntil: now + CheckCooldown(node)}
		left = 0
	}
	t[id] = s
	return left
}

// RollLine is the dice line a check shows: what was rolled, against what.
func RollLine(skill string, r skillcheck.Result) string {
	verdict := "failure"
	if r.Success {
		verdict = "success"
	}
	return fmt.Sprintf("🎲 %s check: rolled %d %s = %d vs DC %d (%s)",
		titleWord(skill), r.Roll, signed(r.Modifier), r.Total, r.DC, verdict)
}

// WaitText is how long a closed check has left, roughly ("about 3 hours").
func WaitText(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", max(minutes, 1))
	}
	h := (minutes + 30) / 60
	if h == 1 {
		return "about an hour"
	}
	return fmt.Sprintf("about %d hours", h)
}

func signed(n int) string {
	if n < 0 {
		return fmt.Sprintf("- %d", -n)
	}
	return fmt.Sprintf("+ %d", n)
}

func titleWord(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Package boards keeps a player's highscores (docs/draft/server-stats.md §7):
// their score on each board, in their lifetime record (cmd/server/stats).
//
// Two kinds of board. A standing board is where a character stands now (XP,
// net worth), read from each kept save; a player has up to five characters,
// so their best one stands for them. A record board is the best ever (fish
// landed, toughest kill), from the lifetime record, and never goes down.
//
// A board's name is its Gamestr level tag.
package boards

import (
	"fmt"
	"sort"
	"strings"

	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
)

// The boards.
const (
	XP          = "xp"
	NetWorth    = "net-worth"
	QuestPoints = "quest-points"
	Places      = "places-discovered"
	Badges      = "badges"
	Slain       = "monsters-slain"
	Toughest    = "toughest-kill"
	FishLanded  = "fish-landed"
	Biggest     = "biggest-" // + a fish species, in ounces
)

// StandingBoards are the boards a character stands on.
var StandingBoards = []string{XP, NetWorth, QuestPoints, Places}

// Standing is a character's numbers on the standing boards at a kept save.
func Standing(save *types.SaveFile, level int, netWorth int64, questPoints, places int) map[string]stats.Entry {
	who, detail := save.D, fmt.Sprintf("level %d %s", level, save.Class)
	return map[string]stats.Entry{
		XP:          {Value: int64(save.Experience), Who: who, Detail: detail},
		NetWorth:    {Value: netWorth, Who: who, Detail: detail},
		QuestPoints: {Value: int64(questPoints), Who: who, Detail: detail},
		Places:      {Value: int64(places), Who: who, Detail: detail},
	}
}

// Stand records a character's standing at its kept save and refreshes the
// player's scores; it reports whether anything changed.
func Stand(r *stats.Record, slot string, standing map[string]stats.Entry, now int64) bool {
	if r.Standing == nil {
		r.Standing = map[string]map[string]stats.Entry{}
	}
	r.Standing[slot] = standing
	return Rescore(r, now)
}

// Drop forgets a deleted character's standing: the player's next best
// character stands for them. It reports whether anything changed.
func Drop(r *stats.Record, slot string, now int64) bool {
	if _, ok := r.Standing[slot]; !ok {
		return false
	}
	delete(r.Standing, slot)
	Rescore(r, now)
	return true
}

// Current is the player's value on every board, from the record.
func Current(r *stats.Record) map[string]stats.Entry {
	out := map[string]stats.Entry{}
	slots := make([]string, 0, len(r.Standing))
	for slot := range r.Standing {
		slots = append(slots, slot)
	}
	sort.Strings(slots) // a tie goes to the same character every time
	for _, b := range StandingBoards {
		for _, slot := range slots {
			if e, ok := r.Standing[slot][b]; ok && e.Value > out[b].Value {
				out[b] = e
			}
		}
	}
	out[Slain] = stats.Entry{Value: r.Counters[stats.Slain]}
	out[FishLanded] = stats.Entry{Value: r.Counters[stats.FishLanded]}
	out[Badges] = stats.Entry{Value: int64(len(r.Badges))}
	if b, ok := r.Bests[stats.ToughestKill]; ok {
		out[Toughest] = stats.Entry{Value: b.Value, Who: b.Who, Detail: b.Label}
	}
	for key, b := range r.Bests {
		if species, ok := strings.CutPrefix(key, stats.FishOunces); ok {
			out[Biggest+species] = stats.Entry{Value: b.Value, Who: b.Who, Detail: species}
		}
	}
	return out
}

// Rescore sets the player's score on every board from the record: a score
// that changed gets a new time (and waits to be signed again, on official).
// It reports whether any changed.
func Rescore(r *stats.Record, now int64) bool {
	current := Current(r)
	if r.Scores == nil {
		r.Scores = map[string]stats.Score{}
	}
	changed := false
	for b, e := range current {
		if e.Value <= 0 {
			continue
		}
		if old, ok := r.Scores[b]; !ok || old.Entry != e {
			r.Scores[b] = stats.Score{Entry: e, At: now}
			changed = true
		}
	}
	for b := range r.Scores {
		if current[b].Value <= 0 {
			delete(r.Scores, b) // the only character on it was deleted
			changed = true
		}
	}
	return changed
}

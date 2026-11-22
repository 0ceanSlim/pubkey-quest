package stats

import (
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ─── The highscores ─────────────────────────────────────────────────────────
//
// Every player's scores, kept in memory for the highscores page: read from
// the records on first use, then kept current as records are written.

var (
	boardMu     sync.Mutex
	boardLoaded bool
	boardScores = map[string]map[string]Score{} // npub → board → score
)

// noteScores keeps the in-memory board current with a record just written.
func noteScores(npub string, scores map[string]Score) {
	boardMu.Lock()
	defer boardMu.Unlock()
	if boardLoaded {
		boardScores[npub] = scores
	}
}

// loadBoards reads every record's scores (boardMu held).
func loadBoards() {
	if boardLoaded {
		return
	}
	boardLoaded = true
	paths, _ := filepath.Glob(filepath.Join(Dir, "*.json"))
	for _, p := range paths {
		npub := strings.TrimSuffix(filepath.Base(p), ".json")
		recordMu.Lock()
		r, err := load(npub)
		recordMu.Unlock()
		if err == nil && len(r.Scores) > 0 {
			boardScores[npub] = r.Scores
		}
	}
}

// Ranked is one player's place on a board.
type Ranked struct {
	Npub string `json:"npub"`
	Score
}

// Top is a board's best n scores, highest first; a tie goes to whoever got
// there first. n <= 0 is all of them.
func Top(board string, n int) []Ranked {
	boardMu.Lock()
	loadBoards()
	var out []Ranked
	for npub, scores := range boardScores {
		if s, ok := scores[board]; ok && s.Value > 0 {
			out = append(out, Ranked{Npub: npub, Score: s})
		}
	}
	boardMu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		if out[i].At != out[j].At {
			return out[i].At < out[j].At
		}
		return out[i].Npub < out[j].Npub
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// resetBoards forgets the in-memory board (tests moving Dir).
func resetBoards() {
	boardMu.Lock()
	boardLoaded, boardScores = false, map[string]map[string]Score{}
	boardMu.Unlock()
}

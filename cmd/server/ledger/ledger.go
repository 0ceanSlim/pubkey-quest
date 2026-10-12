// Package ledger is the official server's memory of which signed save it last
// accepted for each character (docs/draft/relay-saves-plan.md §3).
//
// One small JSON file per npub under data/ledger/, separate from game.db (a
// rebuildable cache that must never hold player records). Each entry is keyed
// by the save event's d tag and holds the accepted event id and the relays
// that confirmed storing it — no copy of the event: the relays are where it
// is fetched back from.
package ledger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Dir is where ledger files live.
var Dir = filepath.Join("data", "ledger")

// Entry is the last accepted save for one character slot.
type Entry struct {
	EventID    string   `json:"id"`
	Relays     []string `json:"relays"`     // relays that answered OK
	CreatedAt  int64    `json:"created_at"` // the event's created_at
	Version    int      `json:"v"`          // save format version
	AcceptedAt int64    `json:"accepted_at"`
}

type file struct {
	Slots map[string]Entry `json:"slots"` // d tag → entry
}

var mu sync.Mutex

func pathFor(npub string) (string, error) {
	if npub == "" || filepath.Base(npub) != npub {
		return "", fmt.Errorf("ledger: bad npub %q", npub)
	}
	return filepath.Join(Dir, npub+".json"), nil
}

func read(npub string) (file, error) {
	f := file{Slots: map[string]Entry{}}
	p, err := pathFor(npub)
	if err != nil {
		return f, err
	}
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		return f, err
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("ledger %s: %w", npub, err)
	}
	if f.Slots == nil {
		f.Slots = map[string]Entry{}
	}
	return f, nil
}

// Get returns the accepted save for a slot.
func Get(npub, d string) (Entry, bool) {
	mu.Lock()
	defer mu.Unlock()
	f, err := read(npub)
	if err != nil {
		return Entry{}, false
	}
	e, ok := f.Slots[d]
	return e, ok
}

// List returns every slot the player has an accepted save for.
func List(npub string) map[string]Entry {
	mu.Lock()
	defer mu.Unlock()
	f, _ := read(npub)
	return f.Slots
}

// Record stores a newly accepted save. The write is atomic (temp file +
// rename), so a crash never leaves a half-written ledger.
func Record(npub, d string, e Entry) error {
	mu.Lock()
	defer mu.Unlock()
	f, err := read(npub)
	if err != nil {
		return err
	}
	if e.AcceptedAt == 0 {
		e.AcceptedAt = time.Now().Unix()
	}
	f.Slots[d] = e
	p, _ := pathFor(npub)
	if err := os.MkdirAll(Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// Remove forgets a slot (the player deleted that character).
func Remove(npub, d string) error {
	mu.Lock()
	defer mu.Unlock()
	f, err := read(npub)
	if err != nil {
		return err
	}
	if _, ok := f.Slots[d]; !ok {
		return nil
	}
	delete(f.Slots, d)
	p, _ := pathFor(npub)
	data, _ := json.MarshalIndent(f, "", "  ")
	return os.WriteFile(p, data, 0o644)
}

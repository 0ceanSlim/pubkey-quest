// Package stats is the server's memory of each player's lifetime play: the
// counters, sets and bests that badges and highscores are judged on, which
// the save can't hold (docs/draft/competition-plan.md, server-stats.md).
//
// It counts only what a kept save covers. Gameplay adds to a session's
// Pending stats; a kept save commits them to the player's Record (Commit);
// a reload, or quitting without saving, drops them, as the play they came
// from is gone. Pending stats are journaled with their session, so a crash
// doesn't lose them either.
//
// A Record is per player (npub), not per character: every character adds to
// the same lifetime record. It's kept as data/stats/<npub>.json. On the
// official server the Dungeon Master also publishes it to relays, which are
// its store of record; the file is the server's copy.
//
// Play never reads a Record: badges and highscores are flair, and the game
// plays the same with stats off.
package stats

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"pubkey-quest/types"
)

// Enabled switches the store on (config stats.enabled). Off, nothing is
// counted and every call here does nothing.
var Enabled bool

// Dir is where each player's Record is kept.
var Dir = filepath.Join("data", "stats")

// ─── What play adds ─────────────────────────────────────────────────────────

// Op is one thing play added, waiting for a kept save.
type Op struct {
	Kind   string `json:"k"`           // opCount | opOnce | opMember | opBest
	Key    string `json:"key"`         // the counter, set or best
	N      int64  `json:"n,omitempty"` // opCount: how many; opBest: the value
	Member string `json:"m,omitempty"` // opMember: the member; opOnce: what's counted once a period
	Period string `json:"p,omitempty"` // opOnce: the period (a day)
	Label  string `json:"l,omitempty"` // opBest: what it was ("Mara, river pike")
	seq    int
}

const (
	opCount  = "count"
	opOnce   = "once"
	opMember = "member"
	opBest   = "best"
)

// Pending is a session's stats since its last kept save.
type Pending struct {
	mu  sync.Mutex
	seq int
	ops []Op
}

func (p *Pending) add(op Op) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seq++
	op.seq = p.seq
	p.ops = append(p.ops, op)
}

// Mark is where the session's stats stand now: a save made now covers
// everything up to it. A relay save keeps its mark while it's being signed,
// so play after it waits for the next save.
func (p *Pending) Mark() int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.seq
}

// take removes and returns the ops up to mark.
func (p *Pending) take(mark int) []Op {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []Op
	kept := p.ops[:0]
	for _, op := range p.ops {
		if op.seq <= mark {
			out = append(out, op)
		} else {
			kept = append(kept, op)
		}
	}
	p.ops = kept
	return out
}

// Snapshot copies the pending ops (the crash journal keeps them).
func (p *Pending) Snapshot() []Op {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Op(nil), p.ops...)
}

var (
	openMu sync.Mutex
	open   = map[string]*Pending{}
)

func sessionKey(npub, saveID string) string { return npub + "|" + saveID }

// Open starts a session's pending stats: empty, or the ops its crash journal
// kept. It replaces whatever that session had (a reload drops it). Nil when
// the store is off.
func Open(npub, saveID string, restored []Op) *Pending {
	if !Enabled {
		return nil
	}
	p := &Pending{}
	for _, op := range restored {
		p.add(op)
	}
	openMu.Lock()
	open[sessionKey(npub, saveID)] = p
	openMu.Unlock()
	return p
}

// Close forgets a session's pending stats (it's gone from memory; its
// journal keeps them if it has one).
func Close(npub, saveID string) {
	openMu.Lock()
	delete(open, sessionKey(npub, saveID))
	openMu.Unlock()
}

// pendingFor is the pending stats of the session playing save.
func pendingFor(save *types.SaveFile) *Pending {
	if save == nil || !Enabled {
		return nil
	}
	openMu.Lock()
	defer openMu.Unlock()
	return open[sessionKey(save.InternalNpub, save.InternalID)]
}

// Count adds n to a lifetime counter (fish landed, monsters slain).
func Count(save *types.SaveFile, key string, n int64) {
	if p := pendingFor(save); p != nil && n > 0 {
		p.add(Op{Kind: opCount, Key: key, N: n})
	}
}

// CountOnce adds 1 to a counter for what, at most once per period for the
// player, however many of their characters do it (a daily quest: once a day).
func CountOnce(save *types.SaveFile, key, what, period string) {
	if p := pendingFor(save); p != nil {
		p.add(Op{Kind: opOnce, Key: key, Member: what, Period: period})
	}
}

// AddMember adds member to a set (the fish species caught).
func AddMember(save *types.SaveFile, key, member string) {
	if p := pendingFor(save); p != nil && member != "" {
		p.add(Op{Kind: opMember, Key: key, Member: member})
	}
}

// Best records value for a best-ever (the biggest pike, in ounces); only a
// new high counts.
func Best(save *types.SaveFile, key string, value int64, label string) {
	if p := pendingFor(save); p != nil {
		p.add(Op{Kind: opBest, Key: key, N: value, Label: label})
	}
}

// ─── A player's record ──────────────────────────────────────────────────────

// Record is a player's lifetime stats.
type Record struct {
	Counters map[string]int64            `json:"counters,omitempty"`
	Once     map[string]string           `json:"once,omitempty"` // key|what → the last period counted
	Sets     map[string]map[string]int64 `json:"sets,omitempty"` // set → member → first seen (unix)
	Bests    map[string]BestEntry        `json:"bests,omitempty"`
	Updated  int64                       `json:"updated_at"`
}

// BestEntry is a best-ever: the value, what it was, and when.
type BestEntry struct {
	Value int64  `json:"v"`
	Label string `json:"l,omitempty"`
	At    int64  `json:"at"`
}

// Apply adds ops to the record as of now.
func (r *Record) Apply(ops []Op, now int64) {
	if r.Counters == nil {
		r.Counters = map[string]int64{}
	}
	if r.Once == nil {
		r.Once = map[string]string{}
	}
	if r.Sets == nil {
		r.Sets = map[string]map[string]int64{}
	}
	if r.Bests == nil {
		r.Bests = map[string]BestEntry{}
	}
	for _, op := range ops {
		switch op.Kind {
		case opCount:
			r.Counters[op.Key] += op.N
		case opOnce:
			k := op.Key + "|" + op.Member
			if r.Once[k] != op.Period {
				r.Once[k] = op.Period
				r.Counters[op.Key]++
			}
		case opMember:
			if r.Sets[op.Key] == nil {
				r.Sets[op.Key] = map[string]int64{}
			}
			if _, ok := r.Sets[op.Key][op.Member]; !ok {
				r.Sets[op.Key][op.Member] = now
			}
		case opBest:
			if b, ok := r.Bests[op.Key]; !ok || op.N > b.Value {
				r.Bests[op.Key] = BestEntry{Value: op.N, Label: op.Label, At: now}
			}
		}
	}
	r.Updated = now
}

var recordMu sync.Mutex

func recordPath(npub string) (string, error) {
	if npub == "" || strings.ContainsAny(npub, `/\.:`) {
		return "", fmt.Errorf("bad npub %q", npub)
	}
	return filepath.Join(Dir, npub+".json"), nil
}

// Load reads a player's record (empty if they have none).
func Load(npub string) (Record, error) {
	recordMu.Lock()
	defer recordMu.Unlock()
	return load(npub)
}

func load(npub string) (Record, error) {
	var r Record
	path, err := recordPath(npub)
	if err != nil {
		return r, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(data, &r)
}

func write(npub string, r Record) error {
	path, err := recordPath(npub)
	if err != nil {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(Dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Commit adds a session's stats up to mark to the player's record: the save
// that covers them was just kept. It returns the record as it now stands.
func Commit(npub, saveID string, mark int) (Record, error) {
	if !Enabled {
		return Record{}, nil
	}
	recordMu.Lock()
	defer recordMu.Unlock()
	r, err := load(npub)
	if err != nil {
		return r, err // the stats stay pending
	}

	openMu.Lock()
	p := open[sessionKey(npub, saveID)]
	openMu.Unlock()
	var ops []Op
	if p != nil {
		ops = p.take(mark)
	}
	if len(ops) == 0 {
		return r, nil
	}
	r.Apply(ops, time.Now().Unix())
	if err := write(npub, r); err != nil {
		p.putBack(ops) // they wait for the next save
		return r, err
	}
	return r, nil
}

// putBack returns ops a failed commit took, ahead of anything newer.
func (p *Pending) putBack(ops []Op) {
	if p == nil || len(ops) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ops = append(append([]Op(nil), ops...), p.ops...)
}

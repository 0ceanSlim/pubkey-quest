// Package events is the central gameplay-event recorder: a single Record entry
// point that fans every notable action out to subscribed consumers. It is the
// shared spine the roadmap defers from M1 to here — the same stream feeds quest
// objective tracking (its first consumer), daily/weekly progress, and, on the
// official server, the badge/stat tracker and action audit trail. Recording
// happens in one place so a "monster killed" or "item acquired" is reported
// once and every interested system sees it, rather than each call site poking
// quests, dailies, and stats separately.
package events

import "pubkey-quest/types"

// Kind enumerates the recordable gameplay events. The first five map onto the
// quest objective types (slay/fetch/explore/talk/check) so the quest consumer
// can match an event to an objective directly; the rest feed dailies and the
// future stat/audit consumers.
type Kind string

const (
	MonsterKilled     Kind = "monster_killed"     // Target = monster id   → quest "slay"
	ItemAcquired      Kind = "item_acquired"      // Target = item id      → quest "fetch"
	LocationDiscovered Kind = "location_discovered" // Target = location/POI id → quest "explore"
	NPCTalked         Kind = "npc_talked"         // Target = npc id       → quest "talk"
	SkillCheckPassed  Kind = "skill_check_passed" // Target = skill id     → quest "check"
	ShopTransaction   Kind = "shop_transaction"   // Target = item id
	Slept             Kind = "slept"              // Target = building id
)

// Event is one recorded occurrence. Count is the magnitude (monsters slain,
// items acquired); it is always >= 1 by the time a consumer sees it.
type Event struct {
	Kind   Kind
	Target string
	Count  int
}

// Consumer reacts to a recorded event. Consumers run in registration order and
// may mutate the save (e.g. advancing a quest objective's count). Each consumer
// sees every event and decides for itself whether the event is relevant.
type Consumer func(save *types.SaveFile, ev Event)

// Recorder fans recorded events out to its subscribed consumers. The zero value
// is usable; tests construct their own, while the server wires one up at init
// and exposes it through the package-level default below.
type Recorder struct {
	consumers []Consumer
}

// Subscribe registers a consumer to receive every subsequently recorded event.
func (r *Recorder) Subscribe(c Consumer) {
	if c != nil {
		r.consumers = append(r.consumers, c)
	}
}

// Record builds an event and delivers it to every consumer. A count <= 0 is
// normalised to 1 so call sites can omit it for single occurrences.
func (r *Recorder) Record(save *types.SaveFile, kind Kind, target string, count int) {
	if count <= 0 {
		count = 1
	}
	ev := Event{Kind: kind, Target: target, Count: count}
	for _, c := range r.consumers {
		c(save, ev)
	}
}

// defaultRecorder is the process-wide recorder the server's spread-out call
// sites (combat, inventory, travel, dialogue) record against without threading
// a Recorder through every handler. Consumers subscribe once at startup.
var defaultRecorder = &Recorder{}

// Subscribe registers a consumer on the default recorder (call at startup).
func Subscribe(c Consumer) { defaultRecorder.Subscribe(c) }

// Record reports an event to the default recorder. This is the call sites' entry
// point: events.Record(save, events.MonsterKilled, monsterID, 1).
func Record(save *types.SaveFile, kind Kind, target string, count int) {
	defaultRecorder.Record(save, kind, target, count)
}

// Reset clears the default recorder's consumers. Intended for tests that want a
// clean slate; the server subscribes exactly once at startup.
func Reset() { defaultRecorder = &Recorder{} }

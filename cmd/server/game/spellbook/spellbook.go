// Package spellbook works out what a caster has at their level: how many spell
// slots of each level (spell-slots.json), and which spells they know.
//
// Both derive from class + level (+ the creation snapshot in KnownSpells), so
// neither is stored beyond what already is (hydration rule, roadmap §4):
//   - Slots: the save keeps the slot arrays because they hold what's prepared
//     in them; SyncSlots grows them to the counts for the current level.
//   - Known spells: KnownSpells is the creation snapshot (the 5e starting list).
//     A whole-list caster (spell-progression.json spells_known ≥ 99: cleric,
//     druid) also knows every spell of their class at each spell level they have
//     slots for — Known adds those.
//   - Picks: every other caster (bard, sorcerer, warlock, paladin, ranger,
//     wizard) learns a new spell for each step up their spells_known count since
//     level 1. A pick is a choice, so it's stored (appended to KnownSpells); the
//     number still open derives (Picks). See docs/draft/spell-learning.md.
package spellbook

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// wholeListMarker is the spells_known value that means "the whole class list".
const wholeListMarker = 99

type tables struct {
	slots     map[string]map[int]map[string]int // class → level → slot type → count
	known     map[string][]int                  // class → spells_known per level (index 0 = level 1)
	byClass   map[string]map[int][]string       // class → spell level → spell ids
	levelOf   map[string]int                    // spell id → spell level
	schoolOf  map[string]string                 // spell id → school (lower case)
	lootW     map[int]int                       // spell level → loot weight
	maxScroll int                               // highest spell level with scroll items
	starting  map[string]map[string]bool        // class → its creation spell list
	advanced  []types.AdvancementEntry
}

var (
	mu  sync.Mutex
	tbl *tables
)

// load reads the tables once the database is open, and caches them. Before
// that (an early call in tests) it returns empty tables without caching, so a
// later call still loads.
func load() *tables {
	mu.Lock()
	defer mu.Unlock()
	if tbl != nil {
		return tbl
	}
	t := &tables{slots: map[string]map[int]map[string]int{}, known: map[string][]int{}, byClass: map[string]map[int][]string{},
		levelOf: map[string]int{}, starting: map[string]map[string]bool{}, schoolOf: map[string]string{}, lootW: map[int]int{}}
	database := db.GetDB()
	if database == nil {
		return t
	}
	loadSlots(database, t)
	loadProgression(database, t)
	loadStarting(database, t)
	if spells, err := gamedata.LoadAllSpells(database); err == nil {
		for _, s := range spells {
			t.levelOf[s.ID] = s.Level
			t.schoolOf[s.ID] = strings.ToLower(s.School)
			for _, c := range s.Classes {
				c = strings.ToLower(strings.TrimSpace(c))
				if t.byClass[c] == nil {
					t.byClass[c] = map[int][]string{}
				}
				t.byClass[c][s.Level] = append(t.byClass[c][s.Level], s.ID)
			}
		}
	}
	t.advanced, _ = character.LoadAdvancement(database)
	loadScrollLoot(database, t)
	tbl = t
	return tbl
}

func loadSlots(database *sql.DB, t *tables) {
	var blob string
	if database.QueryRow("SELECT data FROM spell_slots_progression WHERE id = 'spell-slots'").Scan(&blob) != nil {
		return
	}
	var top map[string]json.RawMessage
	if json.Unmarshal([]byte(blob), &top) != nil {
		return
	}
	for class, raw := range top {
		var byLevel map[string]map[string]int
		if json.Unmarshal(raw, &byLevel) != nil {
			continue // "description"
		}
		m := map[int]map[string]int{}
		for lvl, slots := range byLevel {
			if n, err := strconv.Atoi(lvl); err == nil {
				m[n] = slots
			}
		}
		t.slots[strings.ToLower(class)] = m
	}
}

func loadProgression(database *sql.DB, t *tables) {
	var blob string
	if database.QueryRow("SELECT data FROM spell_progression WHERE id = 'spell-progression'").Scan(&blob) != nil {
		return
	}
	var top map[string]struct {
		SpellsKnown []int `json:"spells_known"`
	}
	if json.Unmarshal([]byte(blob), &top) != nil {
		return
	}
	for class, p := range top {
		t.known[strings.ToLower(class)] = p.SpellsKnown
	}
}

func loadStarting(database *sql.DB, t *tables) {
	var blob string
	if database.QueryRow("SELECT data FROM starting_spells WHERE id = 'starting-spells'").Scan(&blob) != nil {
		return
	}
	var all map[string]map[string][]string
	if json.Unmarshal([]byte(blob), &all) != nil {
		return
	}
	for class, lists := range all {
		set := map[string]bool{}
		for _, ids := range lists {
			for _, id := range ids {
				set[id] = true
			}
		}
		t.starting[strings.ToLower(class)] = set
	}
}

func loadScrollLoot(database *sql.DB, t *tables) {
	var blob string
	if database.QueryRow("SELECT data FROM scroll_scribing WHERE id = 'scroll-scribing'").Scan(&blob) != nil {
		return
	}
	var cfg struct {
		MaxSpellLevel int            `json:"max_spell_level"`
		Weights       map[string]int `json:"loot_level_weights"`
	}
	if json.Unmarshal([]byte(blob), &cfg) != nil {
		return
	}
	t.maxScroll = cfg.MaxSpellLevel
	for k, w := range cfg.Weights {
		if l, err := strconv.Atoi(k); err == nil {
			t.lootW[l] = w
		}
	}
}

// Level is the save's character level.
func Level(save *types.SaveFile) int {
	return character.GetLevelFromXP(save.Experience, load().advanced)
}

// SlotCounts is the slot table row for a class at a level ("cantrips",
// "level_1", …), nil for non-casters.
func SlotCounts(class string, level int) map[string]int {
	return load().slots[strings.ToLower(class)][level]
}

// MaxSpellLevel is the highest spell level the class has slots for at a level.
func MaxSpellLevel(class string, level int) int {
	best := 0
	for k, n := range SlotCounts(class, level) {
		if n <= 0 || !strings.HasPrefix(k, "level_") {
			continue
		}
		if l, err := strconv.Atoi(strings.TrimPrefix(k, "level_")); err == nil && l > best {
			best = l
		}
	}
	return best
}

// WholeListCaster reports whether the class knows its whole class list (cleric,
// druid) at that level.
func WholeListCaster(class string, level int) bool {
	sk := load().known[strings.ToLower(class)]
	return level >= 1 && level <= len(sk) && sk[level-1] >= wholeListMarker
}

// Known is every spell the character knows: their creation list, plus — for a
// whole-list caster — each class spell of a level they have slots for.
func Known(save *types.SaveFile) []string {
	if save == nil {
		return nil
	}
	out := append([]string{}, save.KnownSpells...)
	out = append(out, save.SpellsMemorized...)
	level := Level(save)
	if !WholeListCaster(save.Class, level) {
		return out
	}
	have := map[string]bool{}
	for _, id := range out {
		have[id] = true
	}
	byLevel := load().byClass[strings.ToLower(save.Class)]
	for l := 1; l <= MaxSpellLevel(save.Class, level); l++ {
		for _, id := range byLevel[l] {
			if !have[id] {
				have[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}

// Knows reports whether the character knows a spell.
func Knows(save *types.SaveFile, spellID string) bool {
	for _, id := range Known(save) {
		if id == spellID {
			return true
		}
	}
	return false
}

// SyncSlots grows the save's slot arrays to the counts for its level, keeping
// whatever is prepared. Slots are never taken away (a level is never lost).
// Returns true when anything was added.
func SyncSlots(save *types.SaveFile) bool {
	if save == nil {
		return false
	}
	counts := SlotCounts(save.Class, Level(save))
	if len(counts) == 0 {
		return false
	}
	if save.SpellSlots == nil {
		save.SpellSlots = map[string]interface{}{}
	}
	changed := false
	for slotType, want := range counts {
		var list []interface{}
		switch v := save.SpellSlots[slotType].(type) {
		case []interface{}:
			list = v
		case []map[string]interface{}: // fresh from character creation
			for _, e := range v {
				list = append(list, e)
			}
		}
		for i := len(list); i < want; i++ {
			list = append(list, map[string]interface{}{"slot": float64(i), "spell": nil, "quantity": float64(0)})
			changed = true
		}
		if list == nil {
			list = []interface{}{}
		}
		save.SpellSlots[slotType] = list
	}
	return changed
}

// ─── Picks (known casters + wizards) ────────────────────────────────────────

// Picks is how many spells the character may still learn: the rise in their
// class's spells_known since level 1, less the spells already picked (known
// spells off their creation list). 0 for whole-list casters and non-casters.
func Picks(save *types.SaveFile) int {
	if save == nil {
		return 0
	}
	class := strings.ToLower(save.Class)
	level := Level(save)
	sk := load().known[class]
	if len(sk) == 0 || WholeListCaster(save.Class, level) {
		return 0
	}
	if level > len(sk) {
		level = len(sk)
	}
	gained := sk[level-1] - sk[0]
	n := gained - pickedCount(save)
	if n < 0 {
		return 0
	}
	return n
}

// pickedCount counts known spells (not cantrips) that aren't on the class's
// creation list: the ones learned by picking.
func pickedCount(save *types.SaveFile) int {
	t := load()
	start := t.starting[strings.ToLower(save.Class)]
	n := 0
	for _, id := range save.KnownSpells {
		if !start[id] && t.levelOf[id] >= 1 {
			n++
		}
	}
	return n
}

// PickOptions lists the spells the character could learn now: class spells of
// a level they have slots for (cantrips excluded), not already known, lowest
// level first.
func PickOptions(save *types.SaveFile) []string {
	if save == nil {
		return nil
	}
	known := map[string]bool{}
	for _, id := range Known(save) {
		known[id] = true
	}
	byLevel := load().byClass[strings.ToLower(save.Class)]
	var out []string
	for l := 1; l <= MaxSpellLevel(save.Class, Level(save)); l++ {
		for _, id := range byLevel[l] {
			if !known[id] {
				out = append(out, id)
			}
		}
	}
	return out
}

// Learn spends a pick on a spell. The caller saves nothing extra: the spell is
// appended to KnownSpells, which the save already carries.
func Learn(save *types.SaveFile, spellID string) error {
	if Picks(save) <= 0 {
		return fmt.Errorf("no spells to learn right now")
	}
	for _, id := range PickOptions(save) {
		if id == spellID {
			save.KnownSpells = append(save.KnownSpells, spellID)
			return nil
		}
	}
	if Knows(save, spellID) {
		return fmt.Errorf("you already know that spell")
	}
	return fmt.Errorf("you can't learn that spell yet")
}

// ─── Random scrolls (loot) ───────────────────────────────────────────────────

// RandomScroll picks a scroll item for a "random-scroll" loot entry: a spell
// matching the filter, weighted by its level (loot_level_weights: cantrips and
// level 1 common, higher levels rare). rnd(n) returns 0..n-1. "" when nothing
// matches.
func RandomScroll(f *types.ScrollFilter, rnd func(int) int) string {
	t := load()
	if f == nil {
		f = &types.ScrollFilter{}
	}
	maxL := f.MaxLevel
	if maxL <= 0 || (t.maxScroll > 0 && maxL > t.maxScroll) {
		maxL = t.maxScroll
	}
	classOK := func(id string) bool {
		if len(f.Classes) == 0 {
			return true
		}
		for _, c := range f.Classes {
			for _, sid := range t.byClass[strings.ToLower(c)][t.levelOf[id]] {
				if sid == id {
					return true
				}
			}
		}
		return false
	}
	schoolOK := func(id string) bool {
		if len(f.Schools) == 0 {
			return true
		}
		for _, s := range f.Schools {
			if strings.EqualFold(s, t.schoolOf[id]) {
				return true
			}
		}
		return false
	}
	ids := make([]string, 0, len(t.levelOf))
	for id := range t.levelOf {
		ids = append(ids, id)
	}
	sort.Strings(ids) // stable order, so a seeded roll repeats
	var pool []string
	var weights []int
	total := 0
	for _, id := range ids {
		l := t.levelOf[id]
		w := t.lootW[l]
		if l < f.MinLevel || l > maxL || w <= 0 || !classOK(id) || !schoolOK(id) {
			continue
		}
		pool, weights = append(pool, id), append(weights, w)
		total += w
	}
	if total == 0 {
		return ""
	}
	r := rnd(total)
	for i, w := range weights {
		if r < w {
			return "scroll-of-" + pool[i]
		}
		r -= w
	}
	return ""
}

func init() {
	// A level-up mid-session grows the slots at once.
	character.OnLevelUp = func(save *types.SaveFile) { SyncSlots(save) }
}

// SpellLevel is a spell's level (-1 when unknown).
func SpellLevel(spellID string) int {
	if l, ok := load().levelOf[spellID]; ok {
		return l
	}
	return -1
}

// OnClassList reports whether a spell is on a class's spell list.
func OnClassList(class, spellID string) bool {
	t := load()
	for _, id := range t.byClass[strings.ToLower(class)][t.levelOf[spellID]] {
		if id == spellID {
			return true
		}
	}
	return false
}

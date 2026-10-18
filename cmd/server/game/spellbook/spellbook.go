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
//     slots for — Known adds those. Picked spells for known casters (bard,
//     sorcerer, warlock, paladin, ranger) and the wizard's spellbook are
//     docs/draft/spell-learning.md.
package spellbook

import (
	"database/sql"
	"encoding/json"
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
	slots    map[string]map[int]map[string]int // class → level → slot type → count
	known    map[string][]int                  // class → spells_known per level (index 0 = level 1)
	byClass  map[string]map[int][]string        // class → spell level → spell ids
	advanced []types.AdvancementEntry
}

var (
	once sync.Once
	tbl  *tables
)

func load() *tables {
	once.Do(func() {
		t := &tables{slots: map[string]map[int]map[string]int{}, known: map[string][]int{}, byClass: map[string]map[int][]string{}}
		database := db.GetDB()
		if database == nil {
			tbl = t
			return
		}
		loadSlots(database, t)
		loadProgression(database, t)
		if spells, err := gamedata.LoadAllSpells(database); err == nil {
			for _, s := range spells {
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
		tbl = t
	})
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

func init() {
	// A level-up mid-session grows the slots at once.
	character.OnLevelUp = func(save *types.SaveFile) { SyncSlots(save) }
}

package validation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Scroll items are generated at migrate (cmd/codex/migration/scrolls.go) for
// every spell up to scroll-scribing.json's max_spell_level, so they have no
// file in game-data/items. Loot tables and shops may name them, and loot may
// use a "random-scroll" entry; the reference checks accept both.

var (
	genScrollOnce sync.Once
	genScrollIDs  map[string]bool
)

// generatedScrollIDs is "scroll-of-<spell>" for every spell a scroll item is
// generated for (loaded once).
func generatedScrollIDs() map[string]bool {
	genScrollOnce.Do(func() {
		genScrollIDs = map[string]bool{}
		maxLevel := 3
		if raw, err := os.ReadFile("game-data/magic/scroll-scribing.json"); err == nil {
			var cfg struct {
				MaxSpellLevel int `json:"max_spell_level"`
			}
			if json.Unmarshal(raw, &cfg) == nil && cfg.MaxSpellLevel > 0 {
				maxLevel = cfg.MaxSpellLevel
			}
		}
		paths, _ := filepath.Glob("game-data/magic/spells/*.json")
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			var sp struct {
				Level int `json:"level"`
			}
			if json.Unmarshal(raw, &sp) == nil && sp.Level <= maxLevel {
				genScrollIDs["scroll-of-"+strings.TrimSuffix(filepath.Base(p), ".json")] = true
			}
		}
	})
	return genScrollIDs
}

// withGeneratedScrolls adds the generated scroll ids to an item-id set.
func withGeneratedScrolls(ids map[string]bool) map[string]bool {
	for id := range generatedScrollIDs() {
		ids[id] = true
	}
	return ids
}

// scrollSchools are the schools a random-scroll filter may name.
var scrollSchools = map[string]bool{
	"abjuration": true, "conjuration": true, "divination": true, "enchantment": true,
	"evocation": true, "illusion": true, "necromancy": true, "transmutation": true,
}

// scrollFilterProblem checks a random-scroll entry's "scroll" filter; "" if fine.
func scrollFilterProblem(raw interface{}) string {
	if raw == nil {
		return ""
	}
	f, ok := raw.(map[string]interface{})
	if !ok {
		return "random-scroll \"scroll\" must be an object {max_level, classes, schools}"
	}
	if s, ok := f["schools"].([]interface{}); ok {
		for _, v := range s {
			if name, _ := v.(string); !scrollSchools[strings.ToLower(name)] {
				return "random-scroll school '" + name + "' isn't a school of magic"
			}
		}
	}
	return ""
}

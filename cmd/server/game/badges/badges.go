// Package badges judges the badges a player earns (docs/draft/badges.md):
// achievements the Dungeon Master awards. They're judged at each kept save,
// against the player's lifetime record (cmd/server/stats) and the save just
// kept, so a quest done or a level reached before badges existed is awarded
// at the next save.
package badges

import (
	"encoding/json"
	"slices"
	"sort"
	"sync"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/stats"
	"pubkey-quest/types"
)

var (
	cfgOnce sync.Once
	cfg     *types.BadgesConfig

	mediaOnce sync.Once
	media     *types.BadgeMediaConfig

	catOnce sync.Once
	cat     map[string][]string
)

// Config is badges.json, read once from the systems table. It's never nil.
func Config() *types.BadgesConfig {
	cfgOnce.Do(func() {
		cfg = &types.BadgesConfig{}
		readSystem("badges", cfg)
	})
	return cfg
}

// Media is badge-media.json: each badge's art on Blossom. It's never nil.
func Media() *types.BadgeMediaConfig {
	mediaOnce.Do(func() {
		media = &types.BadgeMediaConfig{}
		readSystem("badge-media", media)
	})
	return media
}

// SetConfig replaces the loaded badges and catalog (tests).
func SetConfig(c *types.BadgesConfig, catalog map[string][]string) {
	cfgOnce.Do(func() {})
	catOnce.Do(func() {})
	cfg, cat = c, catalog
}

func readSystem(id string, into any) {
	database := serverdb.GetDB()
	if database == nil {
		return
	}
	var props string
	if database.QueryRow("SELECT properties FROM systems WHERE id = ?", id).Scan(&props) == nil {
		json.Unmarshal([]byte(props), into)
	}
}

// Catalog is every member of each collection a badge can ask for: the fish
// species, gem types, ores, songs, towns, wild regions and every place.
func Catalog() map[string][]string {
	catOnce.Do(func() { cat = loadCatalog() })
	return cat
}

func loadCatalog() map[string][]string {
	out := map[string][]string{}
	database := serverdb.GetDB()
	if database == nil {
		return out
	}
	var fishing struct {
		Species []struct {
			ID string `json:"id"`
		} `json:"species"`
	}
	readSystem("fishing", &fishing)
	for _, s := range fishing.Species {
		out[types.CollectionFish] = append(out[types.CollectionFish], s.ID)
	}
	var gems struct {
		Types map[string]json.RawMessage `json:"types"`
	}
	readSystem("gems", &gems)
	for id := range gems.Types {
		out[types.CollectionGems] = append(out[types.CollectionGems], id)
	}
	var mining struct {
		Ores map[string]json.RawMessage `json:"ores"`
	}
	readSystem("mining", &mining)
	for id := range mining.Ores {
		out[types.CollectionOres] = append(out[types.CollectionOres], id)
	}
	var music struct {
		Tracks []struct {
			Title string `json:"title"`
		} `json:"tracks"`
	}
	var data string
	if database.QueryRow("SELECT data FROM music_tracks WHERE id = 'music'").Scan(&data) == nil {
		json.Unmarshal([]byte(data), &music)
	}
	for _, t := range music.Tracks {
		out[types.CollectionSongs] = append(out[types.CollectionSongs], t.Title)
	}
	if rows, err := database.Query("SELECT id, location_type FROM locations"); err == nil {
		for rows.Next() {
			var id, kind string
			if rows.Scan(&id, &kind) == nil {
				if kind == "environment" {
					out[types.CollectionWilds] = append(out[types.CollectionWilds], id)
				} else {
					out[types.CollectionTowns] = append(out[types.CollectionTowns], id)
				}
				out[types.CollectionPlaces] = append(out[types.CollectionPlaces], id)
			}
		}
		rows.Close()
	}
	if rows, err := database.Query("SELECT id FROM pois"); err == nil {
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				out[types.CollectionPlaces] = append(out[types.CollectionPlaces], id)
			}
		}
		rows.Close()
	}
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// Judge awards the badges a kept save has earned that the player doesn't
// hold yet, into the record. level is the save's character level. It returns
// the new badges' ids, in badges.json's order.
func Judge(defs []types.BadgeDef, catalog map[string][]string, r *stats.Record, save *types.SaveFile, level int, now int64) []string {
	var earned []string
	for _, b := range defs {
		if _, has := r.Badges[b.ID]; has || !Earned(b.Rule, catalog, r, save, level) {
			continue
		}
		if r.Badges == nil {
			r.Badges = map[string]int64{}
		}
		r.Badges[b.ID] = now
		earned = append(earned, b.ID)
	}
	return earned
}

// Earned reports whether a rule is met.
func Earned(rule types.BadgeRule, catalog map[string][]string, r *stats.Record, save *types.SaveFile, level int) bool {
	switch {
	case rule.Quest != "":
		return slices.Contains(save.QuestsCompleted, rule.Quest)
	case len(rule.AnyQuest) > 0:
		return slices.ContainsFunc(rule.AnyQuest, func(q string) bool { return slices.Contains(save.QuestsCompleted, q) })
	case rule.Collection != "":
		return complete(catalog[rule.Collection], has(rule.Collection, r, save))
	case rule.Level > 0:
		return level >= rule.Level
	case rule.Counter != "":
		return rule.At > 0 && r.Counters[rule.Counter] >= rule.At
	}
	return false
}

// has reports whether the player has one member of a collection: lifetime
// collections from the record, the rest from the character's save.
func has(collection string, r *stats.Record, save *types.SaveFile) func(string) bool {
	inSet := func(key string) func(string) bool {
		return func(m string) bool { _, ok := r.Sets[key][m]; return ok }
	}
	switch collection {
	case types.CollectionFish:
		return inSet(stats.FishSpecies)
	case types.CollectionGems:
		return inSet(stats.GemTypes)
	case types.CollectionOres:
		return inSet(stats.OresMined)
	case types.CollectionSongs:
		return func(m string) bool { return slices.Contains(save.MusicTracksUnlocked, m) }
	case types.CollectionTowns, types.CollectionWilds, types.CollectionPlaces:
		return func(m string) bool { return slices.Contains(save.LocationsDiscovered, m) }
	}
	return func(string) bool { return false }
}

// complete reports whether every member is had (never for an empty set).
func complete(members []string, had func(string) bool) bool {
	if len(members) == 0 {
		return false
	}
	for _, m := range members {
		if !had(m) {
			return false
		}
	}
	return true
}

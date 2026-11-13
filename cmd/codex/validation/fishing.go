package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"pubkey-quest/cmd/server/game/fishing"
	"pubkey-quest/types"
)

// Fish items are generated at migrate from game-data/systems/fishing.json
// (cmd/codex/migration/fish.go), so like scrolls they have no item file. The
// reference checks accept them, and ValidateFishing checks fishing.json itself.

const fishingFile = "game-data/systems/fishing.json"

var (
	genFishOnce sync.Once
	genFishIDs  map[string]bool
)

// generatedFishIDs is every item id fishing.json generates.
func generatedFishIDs() map[string]bool {
	genFishOnce.Do(func() {
		genFishIDs = map[string]bool{}
		cfg, err := fishing.LoadFile(fishingFile)
		if err != nil {
			return
		}
		for _, it := range fishing.Items(cfg) {
			id, _ := it["id"].(string)
			genFishIDs[id] = true
		}
	})
	return genFishIDs
}

// withGeneratedFish adds the generated fish ids to an item-id set.
func withGeneratedFish(ids map[string]bool) map[string]bool {
	for id := range generatedFishIDs() {
		ids[id] = true
	}
	return ids
}

// ValidateFishing checks fishing.json: sizes, profiles and baits make sense,
// and every spot names real species, items, monsters and districts. Barehand-only
// species never sit in a spot's rod pool.
func ValidateFishing() ([]Issue, error) {
	var issues []Issue
	file := filepath.Base(fishingFile)
	add := func(kind, field, msg string, args ...any) {
		issues = append(issues, Issue{Type: kind, Category: "fishing", File: file, Field: field, Message: fmt.Sprintf(msg, args...)})
	}
	cfg, err := fishing.LoadFile(fishingFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		add("error", "", "%v", err)
		return issues, nil
	}

	if fishing.SizeIndex(cfg, "medium") < 0 || fishing.SizeIndex(cfg, "huge") < 0 {
		add("error", "sizes", "sizes must include medium and huge")
	}
	prev := 0.0
	for _, s := range cfg.Sizes {
		if s.MaxPct <= prev || s.MaxPct > 1 {
			add("error", "sizes."+s.ID, "max_pct must grow, up to 1")
		}
		prev = s.MaxPct
	}
	for id, p := range cfg.Profiles {
		if p.Run+p.Dive+p.Thrash+p.Rest <= 0 {
			add("error", "profiles."+id, "a profile's weights must add up to more than 0")
		}
	}
	for id, sp := range cfg.Species {
		f := "species." + id
		if _, ok := cfg.Profiles[sp.Profile]; !ok {
			add("error", f, "unknown profile %q", sp.Profile)
		}
		if sp.MaxLb <= 0 || sp.Value <= 0 {
			add("error", f, "max_lb and value must be positive")
		}
		switch sp.Bites {
		case "any", "day", "night", "dawn_dusk", "dawn":
		default:
			add("error", f, "bites must be any, day, night, dawn_dusk or dawn, not %q", sp.Bites)
		}
		for _, b := range sp.Likes {
			if _, ok := cfg.Baits[b]; !ok && b != "any" {
				add("error", f, "likes unknown bait %q", b)
			}
		}
	}

	items := knownItemIDs()
	monsters := map[string]bool{}
	if paths, err := filepath.Glob("game-data/monsters/*.json"); err == nil {
		for _, p := range paths {
			monsters[strings.TrimSuffix(filepath.Base(p), ".json")] = true
		}
	}
	districts := knownDistricts()
	for id, spot := range cfg.Spots {
		f := "spots." + id
		if spot.District != "" && !districts[spot.District] {
			add("error", f, "district %q isn't a town district", spot.District)
		}
		if spot.DC <= 0 || spot.BiteRate <= 0 {
			add("error", f, "dc and bite_rate must be positive")
		}
		if len(spot.Species) == 0 {
			add("error", f, "a spot needs species")
		}
		for sid := range spot.Species {
			sp, ok := cfg.Species[sid]
			switch {
			case !ok:
				add("error", f, "unknown species %q", sid)
			case sp.Barehand:
				add("error", f, "%s is barehand-only: list it under barehand, not species", sid)
			case !hasWater(sp, spot.Water):
				add("warning", f, "%s doesn't live in %s water", sid, spot.Water)
			}
		}
		for sid := range spot.Barehand {
			if sp, ok := cfg.Species[sid]; !ok || !sp.Barehand {
				add("error", f, "barehand %q isn't a barehand-only species", sid)
			}
		}
		for _, m := range []map[string]float64{spot.Finds, spot.Shellfish} {
			for item := range m {
				if !items[item] {
					add("error", f, "unknown item %q", item)
				}
			}
		}
		if len(spot.Shellfish) > 0 && !spot.Shallows {
			add("warning", f, "shellfish only come up in a net haul, which needs shallows")
		}
		for _, m := range spot.Monsters {
			if !monsters[m.Monster] {
				add("error", f, "unknown monster %q", m.Monster)
			}
		}
	}
	return issues, nil
}

func hasWater(sp types.FishSpecies, water string) bool { return slices.Contains(sp.Water, water) }

// SpotExists reports whether fishing.json has a spot (for POI fishing nodes).
func SpotExists(id string) bool {
	cfg, err := fishing.LoadFile(fishingFile)
	if err != nil {
		return false
	}
	_, ok := cfg.Spots[id]
	return ok
}

// knownDistricts is every town district id ("saltwind-center").
func knownDistricts() map[string]bool {
	out := map[string]bool{}
	towns, err := loadTowns()
	if err != nil {
		return out
	}
	for _, t := range towns {
		for d := range t.districts {
			out[d] = true
		}
	}
	return out
}

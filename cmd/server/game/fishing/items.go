// Package fishing is fishing (docs/draft/fishing-system.md): the fish items
// generated from fishing.json, the bite/species/size rolls, the Cast & Reel
// fight session, quick fishing, the barehand grab and per-day spot depletion.
package fishing

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"

	"pubkey-quest/types"
)

// LoadFile reads fishing.json from disk (Codex: migration, validation, wiki).
func LoadFile(path string) (*types.FishingConfig, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg types.FishingConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("fishing.json: %v", err)
	}
	return &cfg, nil
}

// SizeIndex is a size's place in the bands (0 = tiny), or -1.
func SizeIndex(cfg *types.FishingConfig, size string) int {
	for i, s := range cfg.Sizes {
		if s.ID == size {
			return i
		}
	}
	return -1
}

// mediumIndex is the band a size's step counts from.
func mediumIndex(cfg *types.FishingConfig) int {
	if i := SizeIndex(cfg, "medium"); i >= 0 {
		return i
	}
	return len(cfg.Sizes) / 2
}

// FishValue is what a species is worth at a size (at least 1 gold).
func FishValue(cfg *types.FishingConfig, sp types.FishSpecies, size types.FishSize) int {
	return max(1, int(math.Round(float64(sp.Value)*size.Value)))
}

// band is a size's weight range as fractions of the species' max.
func band(cfg *types.FishingConfig, i int) (lo, hi float64) {
	if i > 0 {
		lo = cfg.Sizes[i-1].MaxPct
	}
	return lo, cfg.Sizes[i].MaxPct
}

// FishWeight is a fish's carry weight: the middle of its band, in lb.
func FishWeight(cfg *types.FishingConfig, sp types.FishSpecies, i int) float64 {
	lo, hi := band(cfg, i)
	return max(0.1, math.Round(sp.MaxLb*(lo+hi)/2*10)/10)
}

var rarities = []string{"common", "uncommon", "rare", "very rare", "legendary"}

func fishRarity(sp types.FishSpecies, size string) string {
	r := 0
	for i, name := range rarities {
		if name == sp.Rarity {
			r = i
		}
	}
	if size == "huge" {
		r++
	}
	return rarities[min(r, len(rarities)-1)]
}

// SortedSpecies is the species ids in a stable order.
func SortedSpecies(cfg *types.FishingConfig) []string {
	ids := make([]string, 0, len(cfg.Species))
	for id := range cfg.Species {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Items expands fishing.json into its generated items: a fish per species and
// size, cooked and salted fish per size, and a trophy per species. Each is an
// item JSON object (as a game-data/items file would be), with "generated": true.
func Items(cfg *types.FishingConfig) []map[string]any {
	var out []map[string]any
	for _, sid := range SortedSpecies(cfg) {
		sp := cfg.Species[sid]
		for i, size := range cfg.Sizes {
			out = append(out, fishItem(cfg, sid, sp, i, size))
		}
		if huge := SizeIndex(cfg, "huge"); huge >= 0 {
			out = append(out, trophyItem(cfg, sid, sp, cfg.Sizes[huge]))
		}
	}
	for _, kind := range []string{"cooked", "salted"} {
		p, ok := cfg.Prepared[kind]
		if !ok {
			continue
		}
		for _, size := range cfg.Sizes {
			out = append(out, preparedItem(kind, p, size))
		}
	}
	return out
}

func fishItem(cfg *types.FishingConfig, sid string, sp types.FishSpecies, i int, size types.FishSize) map[string]any {
	tags := []string{"consumable", "material", "fish", "fish-" + sid, "size-" + size.ID}
	for _, w := range sp.Water {
		tags = append(tags, "water-"+w)
	}
	notes := []string{
		fmt.Sprintf("Keeps you fed %d hour%s. Eaten raw, it may give you food poisoning (Constitution save, DC 10).", size.RawHours, plural(size.RawHours)),
		"Spoils in 12 hours: cook it at a fire (or have an innkeeper cook it), or salt it.",
	}
	if i <= SizeIndex(cfg, "small") {
		notes = append(notes, "Small enough to cut into bait (2 cut bait).")
	}
	if size.ID == "huge" {
		notes = append(notes, "Finn Brackwater in Saltwind will mount a huge fish as a trophy, if you get it to him fresh.")
	}
	if sp.Barehand {
		notes = append(notes, "No hook holds it: it can only be caught barehanded.")
	}
	return map[string]any{
		"id":          types.FishItemID(sid, size.ID),
		"name":        size.Name + " " + sp.Name,
		"description": sp.Desc,
		"type":        "Food",
		"value":       FishValue(cfg, sp, size),
		"rarity":      fishRarity(sp, size.ID),
		"stack":       1,
		"weight":      FishWeight(cfg, sp, i),
		"image":       "/res/img/items/fish-" + sid + ".png",
		"tags":        tags,
		"notes":       notes,
		"food":        map[string]any{"hours": size.RawHours, "risky_dc": 10},
		"spoils":      map[string]any{"shelf_life": 720},
		"cook":        types.PreparedFishID("cooked", size.ID),
		"salt":        types.PreparedFishID("salted", size.ID),
		"fish":        map[string]any{"species": sid, "size": size.ID},
		"generated":   true,
	}
}

func trophyItem(cfg *types.FishingConfig, sid string, sp types.FishSpecies, huge types.FishSize) map[string]any {
	value := int(math.Round(float64(FishValue(cfg, sp, huge)) * cfg.Trophy.Value))
	return map[string]any{
		"id":          types.TrophyItemID(sid),
		"name":        "Mounted " + sp.Name,
		"description": fmt.Sprintf("A huge %s, mounted on a varnished plaque by Finn Brackwater. It will never spoil, and never stop staring.", strings.ToLower(sp.Name)),
		"type":        "Treasure",
		"value":       value,
		"rarity":      fishRarity(types.FishSpecies{Rarity: sp.Rarity}, "huge"),
		"stack":       1,
		"weight":      cfg.Trophy.Weight,
		"image":       "/res/img/items/trophy-" + sid + ".png",
		"tags":        []string{"trophy", "fish-trophy", "fish-" + sid},
		"notes":       []string{"A trophy keeps forever and sells for more than the fish did."},
		"generated":   true,
	}
}

func preparedItem(kind string, p types.PreparedFish, size types.FishSize) map[string]any {
	hours := p.Hours[size.ID]
	image := p.Image
	if p.ImageLarge != "" && (size.ID == "large" || size.ID == "huge") {
		image = p.ImageLarge
	}
	what := "grilled whole over the coals, the skin crisp and the flesh flaking"
	keeps := "Spoils in 2 days."
	if kind == "salted" {
		what = "split, packed in salt and dried. Not much flavour, but it keeps"
		keeps = "Spoils in 30 days."
	}
	return map[string]any{
		"id":          types.PreparedFishID(kind, size.ID),
		"name":        fmt.Sprintf("%s Fish (%s)", p.Name, strings.ToLower(size.Name)),
		"description": fmt.Sprintf("A %s fish, %s.", strings.ToLower(size.Name), what),
		"type":        "Food",
		"value":       p.Value[size.ID],
		"rarity":      "common",
		"stack":       1,
		"weight":      map[bool]float64{true: 2, false: 0.5}[size.ID == "large" || size.ID == "huge"],
		"image":       image,
		"tags":        []string{"consumable", "food", "fish"},
		"notes":       []string{fmt.Sprintf("Keeps you fed %d hour%s.", hours, plural(hours)), keeps},
		"food":        map[string]any{"hours": hours},
		"spoils":      map[string]any{"shelf_life": p.Shelf},
		"generated":   true,
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

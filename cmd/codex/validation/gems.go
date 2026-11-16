package validation

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"pubkey-quest/cmd/server/game/gems"
	"pubkey-quest/types"
)

// Gem items are generated at migrate from game-data/systems/gems.json
// (cmd/codex/migration/gems.go), like fish. The reference checks accept them,
// and ValidateGems checks gems.json itself.

const gemsFile = "game-data/systems/gems.json"

var (
	genGemOnce sync.Once
	genGemIDs  map[string]bool
)

// generatedGemIDs is every item id gems.json generates.
func generatedGemIDs() map[string]bool {
	genGemOnce.Do(func() {
		genGemIDs = map[string]bool{}
		cfg, err := gems.LoadFile(gemsFile)
		if err != nil {
			return
		}
		for _, it := range gems.Items(cfg) {
			id, _ := it["id"].(string)
			genGemIDs[id] = true
		}
	})
	return genGemIDs
}

// withGeneratedGems adds the generated gem ids to an item-id set.
func withGeneratedGems(ids map[string]bool) map[string]bool {
	for id := range generatedGemIDs() {
		ids[id] = true
	}
	return ids
}

// ValidateGems checks gems.json: types, sizes and qualities make sense, the
// curves are in order, the jeweller is a real NPC, and every type has its
// sprites (a missing one is a warning: the item still works).
func ValidateGems() ([]Issue, error) {
	var issues []Issue
	file := filepath.Base(gemsFile)
	add := func(kind, field, msg string, args ...any) {
		issues = append(issues, Issue{Type: kind, Category: "gems", File: file, Field: field, Message: fmt.Sprintf(msg, args...)})
	}
	cfg, err := gems.LoadFile(gemsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		add("error", "", "%v", err)
		return issues, nil
	}
	for id, t := range cfg.Types {
		if strings.Contains(id, "-") {
			add("error", "types."+id, "a gem type id can't contain \"-\" (it's part of the item id)")
		}
		if t.Name == "" || t.BaseValue <= 0 || t.DropRate <= 0 {
			add("error", "types."+id, "needs a name, a base_value and a drop_rate")
		}
		for _, img := range []string{"gem-" + id, "gem-" + id + "-uncut", "ring-" + id, "necklace-" + id} {
			if _, err := os.Stat(filepath.Join("www", "res", "img", "items", img+".png")); err != nil {
				add("warning", "types."+id, "no sprite %s.png", img)
			}
		}
	}
	if gems.SizeIndex(cfg, "tiny") != 0 || len(cfg.Sizes) == 0 {
		add("error", "sizes", "sizes must start at tiny")
	}
	prev := 0.0
	for _, s := range cfg.Sizes {
		if s.Multiplier <= prev || s.DropRate <= 0 || s.CutFee <= 0 {
			add("error", "sizes."+s.ID, "multipliers must grow, and every size needs a drop_rate and a cut_fee")
		}
		prev = s.Multiplier
	}
	total := 0.0
	for _, q := range cfg.Qualities {
		if q.Multiplier <= 0 || q.DropRate < 0 || q.Name == "" {
			add("error", "qualities."+q.ID, "needs a name, a multiplier and a drop_rate")
		}
		total += q.DropRate
	}
	if math.Abs(total-1) > 0.001 {
		add("error", "qualities", "drop_rates add up to %.3f, want 1", total)
	}
	if gems.QualityIndex(cfg, "standard") < 0 {
		add("error", "qualities", "must include standard")
	}
	for name, c := range map[string]types.Curve{
		"drops.gem_chance": cfg.Drops.Chance, "drops.type_tilt": cfg.Drops.TypeTilt, "drops.size_tilt": cfg.Drops.SizeTilt,
		"mining.type_tilt": cfg.Mining.TypeTilt, "mining.size_tilt": cfg.Mining.SizeTilt,
	} {
		for i, p := range c {
			if p[1] <= 0 || (i > 0 && p[0] <= c[i-1][0]) {
				add("error", name, "points must be in order with positive values")
				break
			}
		}
	}
	for _, p := range cfg.Drops.Chance {
		if p[1] > 1 {
			add("error", "drops.gem_chance", "a chance can't be over 1")
		}
	}
	m := cfg.Mining
	if gems.SizeIndex(cfg, m.QuickMaxSize) < 0 {
		add("error", "mining.quick_max_size", "%q isn't a size", m.QuickMaxSize)
	}
	if m.GeodeShare < 0 || m.GeodeShare > 1 || m.QuickChance < 0 || m.QuickChance > 1 {
		add("error", "mining", "geode_share and quick_chance must be 0–1")
	}
	if m.GeodeTier < 1 || m.CrackMinutes <= 0 {
		add("error", "mining", "geode_tier and crack_minutes must be positive")
	}
	j := cfg.Jeweller
	if _, err := os.Stat(npcFile(j.NPC)); err != nil {
		add("error", "jeweller.npc", "no NPC %q", j.NPC)
	}
	for _, p := range types.GemPieces {
		if pc := j.Pieces[p]; pc.Gold <= 0 || pc.GearSlot == "" || pc.Name == "" {
			add("error", "jeweller.pieces."+p, "needs a name, gold and a gear_slot")
		}
	}
	return issues, nil
}

// npcFile finds an NPC's file under game-data/npcs/<home>/<id>.json.
func npcFile(id string) string {
	matches, _ := filepath.Glob(filepath.Join("game-data", "npcs", "*", id+".json"))
	if len(matches) == 0 {
		return filepath.Join("game-data", "npcs", id+".json")
	}
	return matches[0]
}

// SeamGemProblems is what's wrong with a seam's gem leaning: every type must
// be in gems.json with a positive weight.
func SeamGemProblems(g *types.SeamGems) []string {
	if g == nil {
		return nil
	}
	cfg, err := gems.LoadFile(gemsFile)
	if err != nil {
		return nil
	}
	var out []string
	for t, w := range g.Types {
		if _, ok := cfg.Types[t]; !ok {
			out = append(out, fmt.Sprintf("seam gem type %q isn't in gems.json", t))
		}
		if w <= 0 {
			out = append(out, fmt.Sprintf("seam gem type %q needs a positive weight", t))
		}
	}
	if g.Cells[1] < g.Cells[0] || g.Cells[0] < 0 {
		out = append(out, "seam gems.cells must be [min, max]")
	}
	return out
}

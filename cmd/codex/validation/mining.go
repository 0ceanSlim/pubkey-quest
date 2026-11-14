package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"pubkey-quest/types"
)

const miningFile = "game-data/systems/mining.json"

func loadMining() (*types.MiningConfig, error) {
	data, err := os.ReadFile(miningFile)
	if err != nil {
		return nil, err
	}
	cfg := &types.MiningConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ValidateMining checks mining.json: tiers 1–4, each with a DC, faces and costs.
func ValidateMining() ([]Issue, error) {
	cfg, err := loadMining()
	if err != nil {
		return nil, fmt.Errorf("mining.json: %w", err)
	}
	var issues []Issue
	add := func(field, msg string, args ...any) {
		issues = append(issues, Issue{Type: "error", Category: "mining", File: filepath.Base(miningFile), Field: field, Message: fmt.Sprintf(msg, args...)})
	}
	for i, t := range cfg.Tiers {
		f := fmt.Sprintf("tiers[%d]", i)
		if t.Tier != i+1 {
			add(f, "tier %d is listed where tier %d belongs", t.Tier, i+1)
		}
		if t.DC <= 0 || t.FacesPerDay <= 0 {
			add(f, "needs a dc and faces_per_day")
		}
		if t.XP < 0 || t.Exertion < 0 {
			add(f, "xp and exertion can't be negative")
		}
	}
	if len(cfg.Tiers) != 4 {
		add("tiers", "has %d tiers, want 4", len(cfg.Tiers))
	}
	r := cfg.Rules
	if r.QuickMinutes <= 0 || r.SwingMinutes <= 0 || r.SoundMinutes <= 0 || r.ExertionPerFatigue <= 0 {
		add("rules", "quick_minutes, swing_minutes, sound_minutes and exertion_per_fatigue must be positive")
	}
	return issues, nil
}

// SeamProblems is what's wrong with a mine node's seam: its tier must be in
// mining.json, and each ore a real item tagged "ore" with a sane weight and
// quantity. A seam with no ores needs its own quick_loot.
func SeamProblems(s *types.Seam) []string {
	if s == nil {
		return []string{"mine node has no seam"}
	}
	var out []string
	if cfg, err := loadMining(); err == nil && !slices.ContainsFunc(cfg.Tiers, func(t types.MiningTier) bool { return t.Tier == s.Tier }) {
		out = append(out, fmt.Sprintf("seam tier %d isn't a tier in mining.json", s.Tier))
	}
	if len(s.Ores) == 0 && s.QuickLoot == nil {
		out = append(out, "seam has no ores and no quick_loot")
	}
	for _, o := range s.Ores {
		if !itemHasTag(o.Item, "ore") {
			out = append(out, fmt.Sprintf("seam ore %q isn't an item tagged \"ore\"", o.Item))
		}
		if o.Weight <= 0 || o.Quantity[0] < 1 || o.Quantity[1] < o.Quantity[0] {
			out = append(out, fmt.Sprintf("seam ore %q needs a positive weight and a [min, max] quantity", o.Item))
		}
	}
	return out
}

// itemHasTag reads game-data/items/<id>.json for a tag.
func itemHasTag(id, tag string) bool {
	data, err := os.ReadFile(filepath.Join("game-data", "items", id+".json"))
	if err != nil {
		return false
	}
	var item struct {
		Tags []string `json:"tags"`
	}
	return json.Unmarshal(data, &item) == nil && slices.Contains(item.Tags, tag)
}

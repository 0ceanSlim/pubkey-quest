package validation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"pubkey-quest/types"
)

const survivalFile = "game-data/systems/survival.json"

// ValidateSurvival checks survival.json (docs/draft/survival-gear.md): each
// weather bites in real environments, names real items and effects, and costs
// fatigue on a failed save; the whistle is an item.
func ValidateSurvival() ([]Issue, error) {
	data, err := os.ReadFile(survivalFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("survival.json: %w", err)
	}
	cfg := &types.SurvivalConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("survival.json: %w", err)
	}
	var issues []Issue
	add := func(field, msg string, args ...any) {
		issues = append(issues, Issue{Type: "error", Category: "survival", File: filepath.Base(survivalFile), Field: field, Message: fmt.Sprintf(msg, args...)})
	}
	items := knownItemIDs()
	for name, e := range map[string]types.Exposure{"cold": cfg.Cold, "heat": cfg.Heat} {
		for _, p := range SurvivalProblems(e, items) {
			add(name, "%s", p)
		}
	}
	if cfg.Whistle != "" && !items[cfg.Whistle] {
		add("whistle", "%q isn't an item", cfg.Whistle)
	}
	return issues, nil
}

// SurvivalProblems is what's wrong with one weather.
func SurvivalProblems(e types.Exposure, items map[string]bool) []string {
	var out []string
	if e.DC <= 0 {
		return append(out, "needs a dc")
	}
	if e.Fatigue <= 0 {
		out = append(out, "fatigue (per failed save) must be positive")
	}
	if e.From < 0 || e.From > 23 || e.To < 0 || e.To > 24 {
		out = append(out, "from and to are hours, 0–24")
	}
	if e.DCPerDryHour > 0 && !e.Water {
		out = append(out, "dc_per_dry_hour needs water: true")
	}
	for _, env := range e.Environments {
		if _, err := os.Stat(filepath.Join("game-data/locations/environments", env+".json")); err != nil {
			out = append(out, fmt.Sprintf("%q isn't an environment", env))
		}
	}
	for _, list := range [][]string{e.Protect, e.Advantage, e.Fire} {
		for _, id := range list {
			if !items[id] {
				out = append(out, fmt.Sprintf("%q isn't an item", id))
			}
		}
	}
	for _, t := range e.Disadvantage {
		if !slices.Contains([]string{"Light Armor", "Medium Armor", "Heavy Armor"}, t) {
			out = append(out, fmt.Sprintf("disadvantage %q isn't an armour type", t))
		}
	}
	if e.Effect != "" {
		if _, err := os.Stat(filepath.Join("game-data/effects", e.Effect+".json")); err != nil {
			out = append(out, fmt.Sprintf("effect %q doesn't exist", e.Effect))
		}
	}
	if e.Water {
		if _, err := os.Stat("game-data/effects/watered.json"); err != nil {
			out = append(out, "water needs the watered effect")
		}
	}
	return out
}

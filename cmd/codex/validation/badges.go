package validation

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"pubkey-quest/types"
)

const (
	badgesFile     = "game-data/systems/badges.json"
	badgeMediaFile = "game-data/systems/badge-media.json"
	badgeImgDir    = "www/res/img/badges"
)

// badgeRims are the rims badge art is drawn with (docs/draft/badges.md §7).
var badgeRims = map[string]bool{"gold": true, "silver": true, "bronze": true, "gems": true, "iron": true, "legend": true, "level": true}

// badgeCounters are the lifetime counters a badge can count to (stats/keys.go).
var badgeCounters = map[string]bool{"dailies_completed": true, "weeklies_completed": true, "monsters_slain": true, "fish_landed": true}

// ValidateBadges checks badges.json (docs/draft/badges.md): each badge has a
// name, a rim and exactly one rule naming real quests, collections or
// counters; each has its art on Blossom (badge-media.json) and in the game
// (www/res/img/badges); and every main, side, class and race quest has one.
func ValidateBadges() ([]Issue, error) {
	data, err := os.ReadFile(badgesFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("badges.json: %w", err)
	}
	cfg := &types.BadgesConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("badges.json: %w", err)
	}
	media := &types.BadgeMediaConfig{}
	if raw, err := os.ReadFile(badgeMediaFile); err == nil {
		if err := json.Unmarshal(raw, media); err != nil {
			return nil, fmt.Errorf("badge-media.json: %w", err)
		}
	}

	var issues []Issue
	add := func(kind, id, msg string, args ...any) {
		issues = append(issues, Issue{Type: kind, Category: "badges", File: filepath.Base(badgesFile), Field: id, Message: fmt.Sprintf(msg, args...)})
	}
	quests := questCategories()
	seen := map[string]bool{}
	covered := map[string]bool{}
	for _, b := range cfg.Badges {
		if b.ID == "" {
			add("error", "", "a badge has no id")
			continue
		}
		if seen[b.ID] {
			add("error", b.ID, "the id is used twice")
		}
		seen[b.ID] = true
		for _, p := range BadgeProblems(b, quests) {
			add("error", b.ID, "%s", p)
		}
		covered[b.Rule.Quest] = true
		for _, q := range b.Rule.AnyQuest {
			covered[q] = true
		}
		m, ok := media.Badges[b.ID]
		if !ok || m.Image == "" || m.Thumb256 == "" || m.Thumb64 == "" {
			add("error", b.ID, "no art on Blossom: add its image and thumbs to badge-media.json")
		}
		for _, f := range []string{b.ID + ".png", b.ID + "-256.png", b.ID + "-64.png"} {
			if _, err := os.Stat(filepath.Join(badgeImgDir, f)); err != nil {
				add("error", b.ID, "%s is missing from %s", f, badgeImgDir)
			}
		}
	}
	for id, cat := range quests {
		switch cat {
		case types.QuestMain, types.QuestSide, types.QuestClass, types.QuestRace:
			if !covered[id] {
				add("warning", id, "the %s quest %q has no badge", cat, id)
			}
		}
	}
	return issues, nil
}

// BadgeProblems is what's wrong with one badge, given every quest's category.
func BadgeProblems(b types.BadgeDef, quests map[string]types.QuestCategory) []string {
	var out []string
	if b.Name == "" || b.Description == "" {
		out = append(out, "needs a name and a description")
	}
	if !badgeRims[b.Rim] {
		out = append(out, fmt.Sprintf("rim %q isn't one of gold, silver, bronze, gems, iron, legend, level", b.Rim))
	}
	r := b.Rule
	n := 0
	for _, set := range []bool{r.Quest != "", len(r.AnyQuest) > 0, r.Collection != "", r.Level > 0, r.Counter != ""} {
		if set {
			n++
		}
	}
	if n != 1 {
		return append(out, "the rule must set exactly one of quest, any_quest, collection, level, counter")
	}
	for _, q := range append([]string{r.Quest}, r.AnyQuest...) {
		if _, ok := quests[q]; q != "" && !ok {
			out = append(out, fmt.Sprintf("%q isn't a quest", q))
		}
	}
	switch r.Collection {
	case "", types.CollectionFish, types.CollectionGems, types.CollectionOres, types.CollectionSongs,
		types.CollectionTowns, types.CollectionWilds, types.CollectionPlaces:
	default:
		out = append(out, fmt.Sprintf("%q isn't a collection", r.Collection))
	}
	if r.Level > 20 {
		out = append(out, "the highest level is 20")
	}
	if r.Counter != "" && (!badgeCounters[r.Counter] || r.At <= 0) {
		out = append(out, fmt.Sprintf("counter %q (at %d) isn't a lifetime counter with a positive at", r.Counter, r.At))
	}
	return out
}

// questCategories is every quest draft's id and category.
func questCategories() map[string]types.QuestCategory {
	out := map[string]types.QuestCategory{}
	filepath.WalkDir("game-data/quests-drafts", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") || d.Name() == "template.json" {
			return nil
		}
		var q struct {
			ID       string              `json:"id"`
			Category types.QuestCategory `json:"category"`
		}
		if raw, err := os.ReadFile(path); err == nil && json.Unmarshal(raw, &q) == nil && q.ID != "" {
			out[q.ID] = q.Category
		}
		return nil
	})
	return out
}

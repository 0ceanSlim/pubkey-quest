package validation

import (
	"encoding/json"
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
		if t.Name == "" || t.BaseValue <= 0 || (t.DropRate <= 0 && !t.Natural) {
			add("error", "types."+id, "needs a name, a base_value and a drop_rate (a natural gem needs none)")
		}
		imgs := []string{"gem-" + id, "ring-" + id, "necklace-" + id}
		if !t.Natural {
			imgs = append(imgs, "gem-"+id+"-uncut")
		}
		for _, img := range imgs {
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
		if strings.Contains(s.ID, "-") {
			add("error", "sizes."+s.ID, "a size id can't contain \"-\" (it's part of the item id)")
		}
		if s.Multiplier <= prev || s.DropRate <= 0 || s.CutFee <= 0 {
			add("error", "sizes."+s.ID, "multipliers must grow, and every size needs a drop_rate and a cut_fee")
		}
		prev = s.Multiplier
	}
	total := 0.0
	for _, q := range cfg.Qualities {
		if strings.Contains(q.ID, "-") || q.ID == "uncut" {
			add("error", "qualities."+q.ID, "a quality id can't contain \"-\" or be \"uncut\" (it's part of the item id)")
		}
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
	for item, sf := range cfg.Pearls.Shellfish {
		if strings.HasPrefix(item, "_") {
			continue
		}
		if _, err := os.Stat(filepath.Join("game-data", "items", item+".json")); err != nil {
			add("error", "pearls.shellfish."+item, "no item %q", item)
		}
		if sf.Chance <= 0 || sf.Chance > 1 || sf.SizeTilt <= 0 {
			add("error", "pearls.shellfish."+item, "chance must be 0–1 and size_tilt positive")
		}
		if sf.MaxSize != "" && gems.SizeIndex(cfg, sf.MaxSize) < 0 {
			add("error", "pearls.shellfish."+item+".max_size", "%q isn't a size", sf.MaxSize)
		}
		for t, w := range sf.Types {
			if gt, ok := cfg.Types[t]; !ok || !gt.Natural || w <= 0 {
				add("error", "pearls.shellfish."+item+".types."+t, "must be a natural gem type with a positive weight")
			}
		}
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
	for _, p := range EnchantingProblems(cfg) {
		add("error", "enchanting", "%s", p)
	}
	return issues, nil
}

// gemModStats are the constant modifiers an enchanted piece may carry, beside
// the abilities and skill_<id> (types.GemEnchant).
var gemModStats = map[string]bool{
	"armor_class": true, "darkvision": true, "travel_speed": true, "ambush_ward": true, "temp_hp": true,
	"combat_move": true, "weight_capacity": true, "fight_mana": true, "fatigue_slow": true, "light": true,
	"heal_d6": true, "resource_pct": true,
}

// gemSkills is skills.json's skill ids.
func gemSkills() map[string]bool {
	out := map[string]bool{}
	var raw map[string]any
	if b, err := os.ReadFile(filepath.Join("game-data", "systems", "skills.json")); err == nil && json.Unmarshal(b, &raw) == nil {
		for id := range raw {
			out[id] = true
		}
	}
	return out
}

// EnchantingProblems checks gems.json's enchanting table: a fee for every
// size, a quality entry for every quality, an effect for every gem from tiny
// to large, and only stats, ticks, conditions and classes the engine knows.
func EnchantingProblems(cfg *types.GemsConfig) []string {
	var out []string
	bad := func(format string, a ...any) { out = append(out, fmt.Sprintf(format, a...)) }
	en := cfg.Enchanting
	prev := 0
	for _, s := range cfg.Sizes {
		if en.Fees[s.ID] <= prev {
			bad("fees.%s must be above the size below it", s.ID)
		}
		prev = en.Fees[s.ID]
	}
	for _, q := range cfg.Qualities {
		if eq, ok := en.Quality[q.ID]; !ok || (eq.Fee <= 0) {
			bad("quality.%s needs a fee factor", q.ID)
		}
	}
	if en.MaxSuccess <= 0 || en.MaxSuccess > 1 {
		bad("max_success must be 0–1")
	}
	// Selling an enchanted piece (a shop pays at most 0.95× value) must never
	// pay back the cheapest fee, the best quality's factor at a ×1 enchanter
	// (none charges less): no enchant-and-sell profit.
	cheapest := math.MaxFloat64
	for _, q := range en.Quality {
		cheapest = math.Min(cheapest, q.Fee)
	}
	if en.ValueShare < 0 || en.ValueShare*0.95 >= cheapest {
		bad("value_share %.2f lets an enchanted piece sell for more than the cheapest fee", en.ValueShare)
	}
	skills := gemSkills()
	for id, cl := range en.Classes {
		if !abilityNames[cl.Ability] || !skills[cl.Skill] {
			bad("classes.%s needs a real ability and skill", id)
		}
	}
	for t := range cfg.Types {
		for _, size := range []string{"tiny", "small", "medium", "large"} {
			if _, ok := en.Effects[t][size]; !ok {
				bad("effects.%s.%s is missing", t, size)
			}
		}
	}
	for t, bySize := range en.Effects {
		if _, ok := cfg.Types[t]; !ok {
			bad("effects.%s isn't a gem type", t)
		}
		for size, e := range bySize {
			at := "effects." + t + "." + size
			if gems.SizeIndex(cfg, size) < 0 {
				bad("%s: %q isn't a size", at, size)
			}
			for stat, v := range e.Mods {
				known := abilityNames[stat] || gemModStats[stat] || skills[strings.TrimPrefix(stat, "skill_")] && strings.HasPrefix(stat, "skill_")
				if !known || v == 0 {
					bad("%s: mod %q isn't something a worn effect does", at, stat)
				}
			}
			for _, tk := range e.Ticks {
				if (tk.Stat != "hp" && tk.Stat != "mana") || tk.Minutes <= 0 || (tk.When != "" && tk.When != "night") {
					bad("%s: a tick restores hp or mana every so many minutes, at any time or \"night\"", at)
				}
			}
			if b := e.CombatBuff; b != nil {
				for _, c := range b.Immune {
					if !spellConditions[c] {
						bad("%s: immune to %q, which isn't a condition", at, c)
					}
				}
				if b.Sanctuary && b.SaveDC <= 0 {
					bad("%s: a sanctuary needs its save_dc", at)
				}
			}
			for q := range e.ResistByQuality {
				if gems.QualityIndex(cfg, q) < 0 {
					bad("%s: resist_by_quality.%s isn't a quality", at, q)
				}
			}
			if (e.ClassAbility > 0 || e.ClassSkill > 0 || e.ClassResource) && len(en.Classes) == 0 {
				bad("%s follows the class, but there's no classes table", at)
			}
		}
	}
	return out
}

// EnchanterProblems checks an NPC's enchant_config: real pieces, gems and
// size, a chance and a fee, and the "enchant" action on a shift, so they
// can be found at work.
func EnchanterProblems(n *types.NPCData) []string {
	ec := n.EnchantConfig
	if ec == nil {
		return nil
	}
	cfg, err := gems.LoadFile(gemsFile)
	if err != nil {
		return nil
	}
	var out []string
	for _, p := range ec.Pieces {
		if cfg.Jeweller.Pieces[p].Gold <= 0 {
			out = append(out, fmt.Sprintf("enchant_config: %q isn't a piece", p))
		}
	}
	if len(ec.Pieces) == 0 {
		out = append(out, "enchant_config: needs pieces")
	}
	for _, g := range ec.Gems {
		if _, ok := cfg.Types[g]; !ok {
			out = append(out, fmt.Sprintf("enchant_config: %q isn't a gem type", g))
		}
	}
	if gems.SizeIndex(cfg, ec.MaxSize) < 0 {
		out = append(out, fmt.Sprintf("enchant_config: max_size %q isn't a size", ec.MaxSize))
	}
	if ec.Success <= 0 || ec.Success > 1 || ec.Fee < 1 {
		out = append(out, "enchant_config: success must be 0–1 and fee at least 1 (a cheaper enchanter would let enchant-and-sell pay)")
	}
	shift := false
	for _, s := range n.Schedule {
		for _, a := range s.AvailableActions {
			shift = shift || a == "enchant"
		}
	}
	if !shift {
		out = append(out, "enchant_config: no schedule entry offers the \"enchant\" action")
	}
	return out
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
		if gt, ok := cfg.Types[t]; !ok {
			out = append(out, fmt.Sprintf("seam gem type %q isn't in gems.json", t))
		} else if gt.Natural {
			out = append(out, fmt.Sprintf("seam gem type %q is natural: it isn't found in rock", t))
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

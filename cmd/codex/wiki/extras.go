package wiki

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ── Scrolls, spell learning, NPC services, rewards and hand-ins ─────────────
//
// What the game derives at migrate and run time, mirrored here so the wiki
// shows it: a scroll item for every spell (cmd/codex/migration/scrolls.go),
// where each is scribed, who learns which spell when, what an item is a
// reward for or handed in for, and what each NPC offers.

// extras is the config the extra sections read.
type extras struct {
	scribe struct {
		Tiers map[string]struct {
			Gold int `json:"gold"`
		} `json:"tiers"`
		MaxSpellLevel int            `json:"max_spell_level"`
		LootWeights   map[string]int `json:"loot_level_weights"`
		Consumes      []struct {
			Item string `json:"item"`
		} `json:"consumes"`
		Carried    []string `json:"carried"`
		Traditions map[string]struct {
			Name    string   `json:"name"`
			Station string   `json:"station"`
			Classes []string `json:"classes"`
		} `json:"traditions"`
	}
	teleport struct {
		Chain  []string `json:"chain"`
		Cities map[string]struct {
			Name          string `json:"name"`
			Stone         string `json:"stone"`
			Keeper        string `json:"keeper"`
			StoneQuest    string `json:"stone_quest"`
			WaystoneQuest string `json:"waystone_quest"`
		} `json:"cities"`
	}
	slots    map[string]map[string]map[string]int // class → level → slot type → count
	progress map[string]struct {
		SpellsKnown []int `json:"spells_known"`
	}
}

// loadExtras reads the configs and adds the generated scroll items.
func (g *gen) loadExtras() {
	d := g.opt.DataDir
	g.x = &extras{}
	readJSON(filepath.Join(d, "magic", "scroll-scribing.json"), &g.x.scribe)
	readJSON(filepath.Join(d, "systems", "teleportation.json"), &g.x.teleport)
	var rawSlots map[string]json.RawMessage
	readJSON(filepath.Join(d, "magic", "spell-slots.json"), &rawSlots)
	g.x.slots = map[string]map[string]map[string]int{}
	for class, raw := range rawSlots {
		var byLevel map[string]map[string]int
		if json.Unmarshal(raw, &byLevel) == nil {
			g.x.slots[strings.ToLower(class)] = byLevel
		}
	}
	readJSON(filepath.Join(d, "magic", "spell-progression.json"), &g.x.progress)

	// A scroll for every spell the game generates one for.
	maxLvl := g.x.scribe.MaxSpellLevel
	for _, sid := range sortedKeys(g.spells) {
		s := g.spells[sid]
		id := "scroll-of-" + sid
		if s.Level > maxLvl || g.items[id] != nil {
			continue
		}
		kind := "cantrip"
		if s.Level > 0 {
			kind = fmt.Sprintf("level %d spell", s.Level)
		}
		g.items[id] = &item{
			ID: id, Name: "Scroll of " + s.Name, Type: "Spell Scroll", Rarity: "common",
			Description: fmt.Sprintf("A single-use scroll inscribed with the %s %s. Reading it casts the spell; it still costs mana and is consumed.", s.Name, kind),
			Value:       g.x.scribe.Tiers[strconv.Itoa(s.Level)].Gold, Weight: 0.1, Stack: 5,
			Image:   "/res/img/items/scroll-" + strings.ToLower(s.School) + ".png",
			SpellID: sid, Tags: []string{"consumable", "spell-scroll"},
			Notes: []string{fmt.Sprintf("Read it to cast %s (%d mana); consumed on use. Any caster can read it.", s.Name, s.ManaCost),
				"Casters who know the spell can scribe more at a station of their tradition."},
		}
	}
}

// ── who learns a spell, and where it's scribed ──────────────────────────────

// firstLevelFor is the character level at which a class first has slots for a
// spell level (0 when never).
func (g *gen) firstLevelFor(class string, spellLevel int) int {
	byLevel := g.x.slots[strings.ToLower(class)]
	best := 0
	key := "level_" + strconv.Itoa(spellLevel)
	if spellLevel == 0 {
		key = "cantrips"
	}
	for lvl, slots := range byLevel {
		n, err := strconv.Atoi(lvl)
		if err != nil || slots[key] <= 0 {
			continue
		}
		if best == 0 || n < best {
			best = n
		}
	}
	return best
}

// wholeList reports whether a class knows its whole class list (spells_known 99).
func (g *gen) wholeList(class string) bool {
	for k, p := range g.x.progress {
		if strings.EqualFold(k, class) && len(p.SpellsKnown) > 0 && p.SpellsKnown[0] >= 99 {
			return true
		}
	}
	return false
}

// learnedBy says, per class on the spell's list, how and from what level the
// spell is learned.
func (g *gen) learnedBy(s *spell) []string {
	var out []string
	for _, c := range s.Classes {
		lvl := g.firstLevelFor(c, s.Level)
		if lvl == 0 {
			continue
		}
		how := "pick it when you level up"
		switch {
		case s.Level == 0:
			how = "a cantrip: on the class's starting list, or learned later"
		case g.wholeList(c):
			how = "known automatically (the whole class list)"
		case strings.EqualFold(c, "wizard"):
			how = "pick it when you level up, or commit a scroll of it to memory"
		}
		out = append(out, fmt.Sprintf("%s — from level %d: %s", link(titleCase(c), "Characters/"+strings.ToLower(c)), lvl, how))
	}
	return out
}

// scribedAt lists the stations where the spell's classes can scribe it.
func (g *gen) scribedAt(s *spell) []string {
	if s.Level > g.x.scribe.MaxSpellLevel {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, trID := range sortedKeys(g.x.scribe.Traditions) {
		tr := g.x.scribe.Traditions[trID]
		match := false
		for _, c := range s.Classes {
			for _, tc := range tr.Classes {
				if strings.EqualFold(c, tc) {
					match = true
				}
			}
		}
		if !match || seen[trID] {
			continue
		}
		seen[trID] = true
		var where []string
		for _, st := range g.stations(trID) {
			where = append(where, st)
		}
		out = append(out, fmt.Sprintf("%s scroll (%s): %s", titleCase(tr.Name), strings.Join(tr.Classes, ", "), strings.Join(where, " · ")))
	}
	return out
}

// stations lists the buildings with a crafting station, as "Building (City)" links.
func (g *gen) stations(stationID string) []string {
	var out []string
	for _, cid := range sortedKeys(g.cities) {
		c := g.cities[cid]
		for _, dk := range sortedKeys(c.Districts) {
			for _, b := range c.Districts[dk].Buildings {
				for _, st := range b.Stations {
					if st == stationID {
						out = append(out, fmt.Sprintf("%s, %s", b.Name, link(c.Name, "Locations/"+cid)))
					}
				}
			}
		}
	}
	return out
}

// stationName names a crafting station for a building table.
func (g *gen) stationName(id string) string {
	if tr, ok := g.x.scribe.Traditions[id]; ok {
		return "📜 " + titleCase(tr.Name) + " scribing"
	}
	return titleCase(id)
}

// ── random scrolls: which loot tables can drop a given scroll ───────────────

func (g *gen) filterMatches(f *scrollFilter, s *spell) bool {
	maxL := g.x.scribe.MaxSpellLevel
	if f != nil && f.MaxLevel > 0 && f.MaxLevel < maxL {
		maxL = f.MaxLevel
	}
	if s.Level > maxL || (f != nil && s.Level < f.MinLevel) || g.x.scribe.LootWeights[strconv.Itoa(s.Level)] <= 0 {
		return false
	}
	if f == nil {
		return true
	}
	if len(f.Classes) > 0 {
		ok := false
		for _, c := range f.Classes {
			for _, sc := range s.Classes {
				if strings.EqualFold(c, sc) {
					ok = true
				}
			}
		}
		if !ok {
			return false
		}
	}
	if len(f.Schools) > 0 {
		ok := false
		for _, sc := range f.Schools {
			if strings.EqualFold(sc, s.School) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// randomScrollFilters is every random-scroll loot entry, with where it is.
func (g *gen) randomScrollFilters() []struct {
	where string
	f     *scrollFilter
} {
	var out []struct {
		where string
		f     *scrollFilter
	}
	add := func(where string, lt *lootTable) {
		if lt == nil {
			return
		}
		for _, t := range lt.Tiers {
			for _, e := range t.Entries {
				if e.Item == "random-scroll" {
					out = append(out, struct {
						where string
						f     *scrollFilter
					}{where, e.Scroll})
				}
			}
		}
	}
	for _, id := range sortedKeys(g.monsters) {
		add(link(g.monsters[id].Name, "Monsters/"+id), g.monsters[id].Loot)
	}
	for _, id := range sortedKeys(g.pois) {
		for _, nid := range sortedKeys(g.pois[id].Nodes) {
			add(g.poiLink(id), g.pois[id].Nodes[nid].Loot)
		}
	}
	return out
}

// randomScrollSources lists the loot tables a scroll can turn up in.
func (g *gen) randomScrollSources(s *spell) []string {
	var out []string
	seen := map[string]bool{}
	for _, rs := range g.randomScrollFilters() {
		if g.filterMatches(rs.f, s) && !seen[rs.where] {
			seen[rs.where] = true
			out = append(out, rs.where)
		}
	}
	return out
}

// ── rewards, hand-ins, gifts ────────────────────────────────────────────────

// indexExtras builds the reverse indexes the richer pages use.
func (g *gen) indexExtras() {
	g.rewardOf, g.neededFor, g.giftOf = map[string][]string{}, map[string][]string{}, map[string][]string{}
	g.questsBy = map[string][]string{}
	for _, id := range sortedKeys(g.quests) {
		q := g.quests[id]
		ql := g.questLink(id)
		if g.npcs[q.Start.Target] != nil {
			addOnce(g.questsBy, q.Start.Target, id)
		}
		for i, st := range q.Stages {
			if st.Rewards != nil {
				for _, it := range st.Rewards.Items {
					if !q.HandedBack(i, it.ID) {
						addOnce(g.rewardOf, it.ID, ql)
					}
				}
			}
			for _, o := range st.Objectives {
				if o.Type == "fetch" && g.items[o.Target] != nil {
					what := "Handed in for "
					if !o.Consume {
						what = "Shown for "
					}
					addOnce(g.neededFor, o.Target, what+ql)
				}
			}
		}
	}
	for _, c := range g.x.scribe.Consumes {
		addOnce(g.neededFor, c.Item, link("Scribing scrolls", "Rules/Scribing")+" (used up)")
	}
	for _, it := range g.x.scribe.Carried {
		addOnce(g.neededFor, it, link("Scribing scrolls", "Rules/Scribing")+" (carried)")
	}
	for _, nid := range sortedKeys(g.npcs) {
		for _, node := range sortedKeys(g.npcs[nid].Dialogue) {
			var n struct {
				Reward *reward `json:"reward"`
			}
			if json.Unmarshal(g.npcs[nid].Dialogue[node], &n) == nil && n.Reward != nil {
				for _, it := range n.Reward.Items {
					addOnce(g.giftOf, it.ID, g.npcLink(nid))
				}
			}
		}
	}
	// Quest rewards are listed as rewards now, not as "found in".
	for id, wheres := range g.foundIn {
		var keep []string
		for _, w := range wheres {
			if !strings.Contains(w, "/Quests/") {
				keep = append(keep, w)
			}
		}
		g.foundIn[id] = keep
	}
}

// npcServices lists what an NPC offers beyond talk.
func (g *gen) npcServices(n *npc) []string {
	var out []string
	if len(n.Storage) > 0 && string(n.Storage) != "null" {
		out = append(out, "🏦 Vault keeper: stores your belongings in the shared vault")
	}
	for _, cid := range g.x.teleport.Chain {
		c := g.x.teleport.Cities[cid]
		if c.Keeper == n.ID {
			out = append(out, fmt.Sprintf("🌀 Sells %s (see %s)", g.itemLink(c.Stone), link("Homeward Stones", "Rules/Teleportation")))
		}
	}
	works := map[string]bool{}
	for _, s := range n.Schedule {
		if s.State == "working" {
			works[s.Location] = true
		}
	}
	for _, cid := range sortedKeys(g.cities) {
		for _, dk := range sortedKeys(g.cities[cid].Districts) {
			for _, b := range g.cities[cid].Districts[dk].Buildings {
				if works[b.ID] {
					for _, st := range b.Stations {
						out = append(out, fmt.Sprintf("Works where casters scribe scrolls: %s (%s)", b.Name, link("Scribing", "Rules/Scribing")))
						_ = st
						break
					}
				}
			}
		}
	}
	return out
}

// keep sort imported for future index ordering helpers
var _ = sort.Strings

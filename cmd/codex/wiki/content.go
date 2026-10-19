package wiki

import (
	"fmt"
	"sort"
	"strings"
)

// Authored content: places of interest, travel/town encounters and quests. Pages
// summarize what a place holds (people, creatures, checks, loot) rather than
// walking its node graph step by step.

// nodeSummary is what a POI or encounter's nodes add up to.
type nodeSummary struct {
	monsters []string // "2 × [[Bandit]]"
	checks   []string // "Perception DC 10"
	keys     []string // item links
	loot     []string // item ids
	npcs     []string
	xp, gold int
}

func (g *gen) summarize(nodes map[string]node) nodeSummary {
	var s nodeSummary
	seenCheck, seenLoot := map[string]bool{}, map[string]bool{}
	for _, id := range sortedKeys(nodes) {
		n := nodes[id]
		switch {
		case n.MonsterID != "":
			c := n.Count
			if c < 1 {
				c = 1
			}
			s.monsters = append(s.monsters, fmt.Sprintf("%d × %s", c, g.monsterLink(n.MonsterID)))
		case n.Skill != "" && n.DC > 0:
			c := fmt.Sprintf("%s DC %d", titleCase(n.Skill), n.DC)
			if n.Type == "passive_check" {
				c += " (passive)"
			}
			if !seenCheck[c] {
				seenCheck[c] = true
				s.checks = append(s.checks, c)
			}
		}
		for _, ch := range n.Choices {
			for _, r := range ch.Requirements {
				if r.Type == "skill" && r.Min > 0 {
					c := fmt.Sprintf("%s %d", titleCase(r.ID), r.Min)
					if !seenCheck[c] {
						seenCheck[c] = true
						s.checks = append(s.checks, c)
					}
				}
			}
		}
		if n.Lock != nil && n.Lock.Key != "" {
			s.keys = append(s.keys, g.itemLink(n.Lock.Key))
		}
		addLoot := func(id string) {
			if id != "" && id != "nothing" && !seenLoot[id] {
				seenLoot[id] = true
				s.loot = append(s.loot, id)
			}
		}
		if n.Loot != nil {
			for _, e := range n.Loot.Guaranteed {
				addLoot(e.Item)
			}
			for _, t := range n.Loot.Tiers {
				for _, e := range t.Entries {
					addLoot(e.Item)
				}
			}
		}
		if n.Reward != nil {
			s.xp += n.Reward.XP
			s.gold += n.Reward.Gold
			for _, it := range n.Reward.Items {
				addLoot(it.ID)
			}
		}
		s.npcs = append(s.npcs, n.NPCIDs...)
	}
	return s
}

func (g *gen) writeSummary(b *strings.Builder, s nodeSummary) {
	bullets(b, "Creatures", s.monsters)
	bullets(b, "Checks", s.checks)
	bullets(b, "Keys", s.keys)
	var loot []string
	for _, id := range s.loot {
		loot = append(loot, g.itemLink(id))
	}
	if s.xp > 0 {
		loot = append(loot, fmt.Sprintf("up to %d XP", s.xp))
	}
	if s.gold > 0 {
		loot = append(loot, fmt.Sprintf("up to %s", gold(s.gold)))
	}
	bullets(b, "Treasure", loot)
}

func requirementText(r requirement) string {
	if r.Description != "" {
		return r.Description
	}
	switch r.Type {
	case "skill":
		return fmt.Sprintf("%s %d", titleCase(r.ID), r.Min)
	case "quest_points":
		return fmt.Sprintf("%d Quest Points", r.Min)
	case "item":
		return titleCase(r.ID)
	}
	if len(r.Values) > 0 {
		return titleCase(r.Type) + ": " + strings.Join(r.Values, ", ")
	}
	return titleCase(r.Type)
}

func (g *gen) monsterLink(id string) string {
	if m, ok := g.monsters[id]; ok {
		return link(m.Name, "Monsters/"+id)
	}
	return titleCase(id)
}

// ── places of interest ───────────────────────────────────────────────────────

func (g *gen) poiLink(id string) string {
	if p, ok := g.pois[id]; ok {
		return link(p.Name, "Locations/"+id)
	}
	return titleCase(id)
}

func (g *gen) poiPage(p *poi) string {
	var b strings.Builder
	b.WriteString("_" + p.Description + "_\n\n")
	find := ""
	if p.Discovery.Chance > 0 {
		find = fmt.Sprintf("%.0f%% chance each time you explore", p.Discovery.Chance*100)
	}
	var reqs []string
	for _, r := range p.Requirements {
		reqs = append(reqs, requirementText(r))
	}
	var people []string
	for _, n := range p.NPCIDs {
		people = append(people, g.npcLink(n))
	}
	facts(&b,
		"Kind", titleCase(p.Category),
		"Region", g.homeLink(p.Environment),
		"Finding it", find,
		"To enter", strings.Join(reqs, "; "),
		"People", strings.Join(people, ", "),
	)
	s := g.summarize(p.Nodes)
	g.writeSummary(&b, s)

	var quests []string
	for _, q := range g.questsAt[p.ID] {
		quests = append(quests, g.questLink(q))
	}
	bullets(&b, "Quests", quests)
	return b.String()
}

// ── encounters ───────────────────────────────────────────────────────────────

func (g *gen) writeEncounters() error {
	var b strings.Builder
	b.WriteString("Events that can happen to you on the road, in town, or in a tavern.\n\n")
	ids := sortedKeys(g.encounters)
	sort.SliceStable(ids, func(i, j int) bool { return g.encounters[ids[i]].Name < g.encounters[ids[j]].Name })
	for _, id := range ids {
		e := g.encounters[id]
		b.WriteString("## " + e.Name + "\n\n_" + e.Description + "_\n\n")
		var where []string
		for _, l := range e.Locations {
			where = append(where, g.homeLink(l))
		}
		for _, t := range e.BuildingTypes {
			where = append(where, "any "+t)
		}
		chance, repeat := "", "Once"
		if e.Chance > 0 {
			chance = fmt.Sprintf("%.1f%%", e.Chance*100)
		}
		if e.Repeatable {
			repeat = "Yes"
			if e.Cooldown > 0 {
				repeat += ", every " + duration(e.Cooldown) + " at most"
			}
		}
		var people []string
		for _, n := range e.NPCs {
			people = append(people, strings.TrimSpace(n.Name+", "+n.Title))
		}
		facts(&b,
			"Where", strings.Join(where, ", "),
			"When", titleCase(e.Trigger),
			"Chance", chance,
			"Repeats", repeat,
			"People", strings.Join(people, "; "),
		)
		s := g.summarize(e.Nodes)
		g.writeSummary(&b, s)
	}
	return g.page("Encounters", "Encounters", b.String())
}

// ── quests ───────────────────────────────────────────────────────────────────

func (g *gen) questLink(id string) string {
	if q, ok := g.quests[id]; ok {
		return link(q.Name, "Quests/"+id)
	}
	return titleCase(id)
}

// targetLink links a quest target to whatever it names: an NPC, monster, place
// or item.
func (g *gen) targetLink(id string) string {
	switch {
	case g.npcs[id] != nil:
		return g.npcLink(id)
	case g.monsters[id] != nil:
		return g.monsterLink(id)
	case g.pois[id] != nil:
		return g.poiLink(id)
	case g.items[id] != nil:
		return g.itemLink(id)
	case g.cities[id] != nil, g.envs[id] != nil, g.district[id] != "":
		return g.homeLink(id)
	}
	return ""
}

var questOrder = map[string]int{"main": 0, "side": 1, "class": 2, "race": 3, "daily": 4, "weekly": 5}

func (g *gen) writeQuests() error {
	byCat := map[string][]string{}
	for _, id := range sortedKeys(g.quests) {
		q := g.quests[id]
		byCat[q.Category] = append(byCat[q.Category], id)
		if err := g.page("Quests/"+id, q.Name, g.questPage(q)); err != nil {
			return err
		}
	}
	cats := sortedKeys(byCat)
	sort.SliceStable(cats, func(i, j int) bool { return questOrder[cats[i]] < questOrder[cats[j]] })

	var b strings.Builder
	for _, c := range cats {
		b.WriteString("## " + titleCase(c) + " quests\n\n")
		var rows [][]string
		for _, id := range byCat[c] {
			q := g.quests[id]
			rows = append(rows, []string{g.questLink(id), q.Difficulty, fmt.Sprint(q.QuestPoints), cell(q.Description)})
		}
		table(&b, []string{"Quest", "Difficulty", "Quest Points", "About"}, rows)
	}
	return g.page("Quests", "Quests", b.String())
}

func (g *gen) questPage(q *quest) string {
	var b strings.Builder
	b.WriteString("_" + q.Description + "_\n\n")
	var reqs, pre []string
	for _, r := range q.Requirements {
		reqs = append(reqs, requirementText(r))
	}
	for _, p := range q.Prereqs {
		pre = append(pre, g.questLink(p))
	}
	qp := ""
	if q.QuestPoints > 0 {
		qp = fmt.Sprint(q.QuestPoints)
	}
	repeat := ""
	if q.Category == "daily" || q.Category == "weekly" {
		repeat = titleCase(q.Category) + ", with a new target each time"
	}
	facts(&b,
		"Type", titleCase(q.Category),
		"Difficulty", q.Difficulty,
		"Quest Points", qp,
		"Repeats", repeat,
		"Starts", q.Start.Hint,
		"Requires", strings.Join(reqs, "; "),
		"After", strings.Join(pre, ", "),
		"Recommended", strings.Join(q.Recommended.Stats, ", "),
		"Danger", q.Recommended.Danger,
	)

	if len(q.Stages) > 0 {
		b.WriteString("## Steps\n\n")
		for i, st := range q.Stages {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, st.Description))
			for _, o := range st.Objectives {
				line := o.Description
				if t := g.targetLink(o.Target); t != "" {
					line += " (" + t + ")"
				}
				if line != "" {
					b.WriteString("    - " + line + "\n")
				}
			}
		}
		b.WriteString("\n")
	}

	var rewards []string
	xp, gp := 0, 0
	for i, st := range q.Stages {
		if st.Rewards == nil {
			continue
		}
		xp += st.Rewards.XP
		gp += st.Rewards.Gold
		for _, it := range st.Rewards.Items {
			if q.HandedBack(i, it.ID) {
				continue // a hand-in for a later step (the ritual's incense), not a reward
			}
			n := it.Quantity
			if n < 1 {
				n = 1
			}
			rewards = append(rewards, fmt.Sprintf("%d × %s", n, g.itemLink(it.ID)))
		}
	}
	if xp > 0 {
		rewards = append([]string{fmt.Sprintf("%d XP", xp)}, rewards...)
	}
	if gp > 0 {
		rewards = append([]string{gold(gp)}, rewards...)
	}
	bullets(&b, "Rewards", rewards)
	return b.String()
}

package wiki

import (
	"fmt"
	"sort"
	"strings"
)

// ── items ────────────────────────────────────────────────────────────────────

func (g *gen) itemLink(id string) string {
	if id == "random-scroll" { // a scroll picked at roll time, common spells likeliest
		return "a random spell scroll"
	}
	if it, ok := g.items[id]; ok {
		return link(it.Name, "Items/"+id)
	}
	return titleCase(id)
}

func (g *gen) writeItems() error {
	byType := map[string][]string{}
	for _, id := range sortedKeys(g.items) {
		it := g.items[id]
		byType[it.Type] = append(byType[it.Type], id)
		if err := g.page("Items/"+id, it.Name, g.itemPage(it)); err != nil {
			return err
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Every item in the game (%d), grouped by type. Values are base prices — shops charge more, and pay less, depending on your Charisma.\n\n", len(g.items)))
	for _, t := range sortedKeys(byType) {
		ids := byType[t]
		sort.Slice(ids, func(i, j int) bool { return g.items[ids[i]].Name < g.items[ids[j]].Name })
		b.WriteString("## " + titleCase(t) + "\n\n")
		var rows [][]string
		for _, id := range ids {
			it := g.items[id]
			rows = append(rows, []string{g.image(it.Image, it.Name, 32), g.itemLink(id),
				titleCase(it.Rarity), gold(it.Value), num(it.Weight)})
		}
		table(&b, []string{"", "Item", "Rarity", "Value", "Weight (lb)"}, rows)
	}
	return g.page("Items", "Items", b.String())
}

func (g *gen) itemPage(it *item) string {
	var b strings.Builder
	if img := g.image(it.Image, it.Name, 96); img != "" {
		b.WriteString(img + "\n\n")
	}
	if it.Description != "" {
		b.WriteString("_" + it.Description + "_\n\n")
	}

	dmg := strings.ReplaceAll(it.Damage, ",", " / ") // versatile: one-handed / two-handed
	if dmg != "" && it.DamageType != "" {
		dmg += " " + it.DamageType
	}
	rng := num(it.Range)
	if rng != "" {
		if l := num(it.RangeLong); l != "" {
			rng += "/" + l
		}
		rng += " squares"
	}
	stack := ""
	if it.Stack > 1 && it.Stack < 1_000_000 {
		stack = fmt.Sprint(it.Stack)
	}
	var cures []string
	for _, c := range it.Cures {
		cures = append(cures, titleCase(c))
	}
	var skills []string
	for _, s := range sortedKeys(it.SkillBonus) {
		skills = append(skills, fmt.Sprintf("%s +%d", titleCase(s), it.SkillBonus[s]))
	}
	container := ""
	if it.ContainerSlots > 0 {
		container = plural(it.ContainerSlots, "slot")
		if len(it.AllowedTypes) > 0 {
			container += " (" + strings.Join(it.AllowedTypes, ", ") + ")"
		}
	}
	set, ammo, provides, scroll := "", "", "", ""
	if it.Set != "" {
		set = g.itemLink(it.Set)
	}
	if it.Ammunition != "" {
		ammo = g.itemLink(it.Ammunition)
	}
	if it.Provides != "" {
		provides = "Unlimited " + g.itemLink(it.Provides) + " for spellcasting"
	}
	if it.SpellID != "" {
		scroll = g.spellLink(it.SpellID)
	}

	facts(&b,
		"Type", titleCase(it.Type),
		"Rarity", titleCase(it.Rarity),
		"Value", gold(it.Value),
		"Weight", num(it.Weight)+" lb",
		"Stacks to", stack,
		"Slot", titleCase(it.GearSlot),
		"Damage", dmg,
		"Range", rng,
		"Armor Class", num(it.AC),
		"Healing", it.Heal,
		"Cures", strings.Join(cures, ", "),
		"Skill bonus", strings.Join(skills, ", "),
		"Holds", container,
		"Ammunition", ammo,
		"Casts", scroll,
		"Focus", provides,
		"Part of set", set,
	)

	bullets(&b, "Notes", publicNotes(it.Notes))

	var contents []string
	for _, c := range it.contents() {
		contents = append(contents, fmt.Sprintf("%d × %s", c.qty, g.itemLink(c.id)))
	}
	bullets(&b, "Contents", contents)
	bullets(&b, "Set pieces", g.itemLinks(g.setPieces[it.ID]))

	var spells []string
	for _, s := range g.usedBy[it.ID] {
		spells = append(spells, g.spellLink(s))
	}
	bullets(&b, "Spell component for", spells)

	var sold []string
	for _, n := range g.soldBy[it.ID] {
		sold = append(sold, g.npcLink(n)+" — "+g.homeLink(g.npcs[n].PrimaryHome))
	}
	bullets(&b, "Sold by", sold)

	var drops []string
	for _, m := range g.droppedBy[it.ID] {
		drops = append(drops, link(g.monsters[m].Name, "Monsters/"+m))
	}
	if it.SpellID != "" && g.spells[it.SpellID] != nil {
		drops = append(drops, g.randomScrollSources(g.spells[it.SpellID])...)
	}
	bullets(&b, "Dropped by", drops)
	bullets(&b, "Found in packs", g.itemLinks(g.inPacks[it.ID]))
	bullets(&b, "Found in", g.foundIn[it.ID])
	bullets(&b, "Reward from", g.rewardOf[it.ID])
	bullets(&b, "Given by", g.giftOf[it.ID])
	bullets(&b, "Needed for", g.neededFor[it.ID])
	if it.SpellID != "" && g.spells[it.SpellID] != nil {
		bullets(&b, "Scribe it at", g.scribedAt(g.spells[it.SpellID]))
	}

	if len(it.Tags) > 0 {
		b.WriteString("**Tags:** " + strings.Join(it.Tags, ", ") + "\n")
	}
	return b.String()
}

func (g *gen) itemLinks(ids []string) []string {
	var out []string
	for _, id := range ids {
		out = append(out, g.itemLink(id))
	}
	return out
}

// ── monsters ─────────────────────────────────────────────────────────────────

func cr(f float64) string {
	switch f {
	case 0.125:
		return "1/8"
	case 0.25:
		return "1/4"
	case 0.5:
		return "1/2"
	}
	return fmt.Sprintf("%g", f)
}

func mod(score int) string {
	m := score/2 - 5
	if m >= 0 {
		return fmt.Sprintf("%d (+%d)", score, m)
	}
	return fmt.Sprintf("%d (%d)", score, m)
}

func signed(n int) string {
	if n >= 0 {
		return fmt.Sprintf("+%d", n)
	}
	return fmt.Sprint(n)
}

var abilityOrder = []string{"strength", "dexterity", "constitution", "intelligence", "wisdom", "charisma"}

func (g *gen) writeMonsters() error {
	ids := sortedKeys(g.monsters)
	sort.SliceStable(ids, func(i, j int) bool {
		a, b := g.monsters[ids[i]], g.monsters[ids[j]]
		if a.CR != b.CR {
			return a.CR < b.CR
		}
		return a.Name < b.Name
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Every creature in the game (%d), weakest first. Out in the wilds you meet the ones whose habitat matches the land you're crossing — see each [[Locations|/Locations]] page.\n\n", len(ids)))
	var rows [][]string
	for _, id := range ids {
		m := g.monsters[id]
		rows = append(rows, []string{g.monsterImage(m, 96), link(m.Name, "Monsters/"+id), cr(m.CR),
			fmt.Sprint(m.XP), titleCase(m.Size + " " + m.Type), strings.Join(m.Environment, ", ")})
		if err := g.page("Monsters/"+id, m.Name, g.monsterPage(m)); err != nil {
			return err
		}
	}
	table(&b, []string{"", "Monster", "CR", "XP", "Kind", "Habitat"}, rows)
	return g.page("Monsters", "Monsters", b.String())
}

func (g *gen) monsterImage(m *monster, width int) string {
	return g.image("/res/img/monsters/"+m.ID+".png", m.Name, width)
}

func (g *gen) monsterPage(m *monster) string {
	var b strings.Builder
	if img := g.monsterImage(m, 192); img != "" { // sprites are 96×72: whole multiples stay crisp
		b.WriteString(img + "\n\n")
	}
	b.WriteString(fmt.Sprintf("_%s %s, %s_\n\n", titleCase(m.Size), strings.ToLower(m.Type), m.Alignment))

	var speed []string
	for _, k := range sortedKeys(m.Speed) {
		if m.Speed[k] > 0 {
			speed = append(speed, fmt.Sprintf("%s %d ft", k, m.Speed[k]))
		}
	}
	var senses []string
	for _, k := range sortedKeys(m.Senses) {
		if m.Senses[k] > 0 {
			senses = append(senses, fmt.Sprintf("%s %d", strings.ReplaceAll(k, "_", " "), m.Senses[k]))
		}
	}
	facts(&b,
		"Challenge", fmt.Sprintf("%s (%d XP)", cr(m.CR), m.XP),
		"Armor Class", fmt.Sprint(m.AC),
		"Hit Points", fmt.Sprintf("%d (%s)", m.HP, m.HPDice),
		"Speed", strings.Join(speed, ", "),
		"Saving throws", signedMap(m.Saves),
		"Skills", signedMap(m.Skills),
		"Senses", strings.Join(senses, ", "),
		"Resistances", strings.Join(m.DamageResistances, ", "),
		"Immunities", strings.Join(m.DamageImmunities, ", "),
		"Vulnerabilities", strings.Join(m.DamageVulnerabilities, ", "),
		"Condition immunities", strings.Join(m.ConditionImmunities, ", "),
		"Habitat", g.habitatLinks(m),
	)

	var head, row []string
	for _, a := range abilityOrder {
		head = append(head, strings.ToUpper(a[:3]))
		row = append(row, mod(m.Stats[a]))
	}
	table(&b, head, [][]string{row})

	g.actionSection(&b, "Traits", m.Special)
	g.actionSection(&b, "Actions", m.Actions)
	g.actionSection(&b, "Bonus actions", m.BonusActions)
	g.actionSection(&b, "Reactions", m.Reactions)
	g.actionSection(&b, "Legendary actions", m.LegendaryActions)
	g.lootSection(&b, m)
	bullets(&b, "Met in", g.metIn[m.ID])
	return b.String()
}

func signedMap(m map[string]int) string {
	var out []string
	for _, k := range sortedKeys(m) {
		out = append(out, titleCase(k)+" "+signed(m[k]))
	}
	return strings.Join(out, ", ")
}

// habitatLinks names the environments a monster can turn up in.
func (g *gen) habitatLinks(m *monster) string {
	var out []string
	for _, id := range sortedKeys(g.envs) {
		e := g.envs[id]
		for _, h := range m.Environment {
			if h == e.EnvType {
				out = append(out, link(e.Name, "Locations/"+id))
				break
			}
		}
	}
	return strings.Join(out, ", ")
}

func (g *gen) actionSection(b *strings.Builder, heading string, acts []action) {
	if len(acts) == 0 {
		return
	}
	b.WriteString("## " + heading + "\n\n")
	for _, a := range acts {
		name := a.Name
		if a.Recharge != "" {
			name += " (Recharge " + a.Recharge + ")"
		}
		var parts []string
		if a.Attacks > 1 {
			parts = append(parts, fmt.Sprintf("%d attacks", a.Attacks))
		}
		if a.AttackBonus != nil {
			kind := "Melee"
			if strings.HasPrefix(a.Type, "ranged") {
				kind = "Ranged"
			}
			parts = append(parts, fmt.Sprintf("%s attack %s to hit", kind, signed(*a.AttackBonus)))
		}
		if a.Range != nil {
			r := fmt.Sprintf("range %d", *a.Range)
			if a.RangeLong != nil {
				r += fmt.Sprintf("/%d", *a.RangeLong)
			}
			parts = append(parts, r)
		}
		if a.Hit != nil && a.Hit.Dice != "" {
			hit := "hit " + a.Hit.Dice
			if a.Hit.Mod != 0 {
				hit += " " + strings.Replace(signed(a.Hit.Mod), "+", "+ ", 1)
			}
			hit += " " + a.Hit.Type
			if sp := special(a.Hit.Special); sp != "" {
				hit += ", " + sp
			}
			parts = append(parts, hit)
		}
		line := "- **" + name + ".** "
		if len(parts) > 0 {
			line += strings.Join(parts, "; ") + ". "
		}
		b.WriteString(line + a.Description + "\n")
	}
	b.WriteString("\n")
}

// special renders a hit rider: plain prose, or a save like "DC 12 Constitution save".
func special(v any) string {
	switch s := v.(type) {
	case string:
		return s
	case map[string]any:
		out := ""
		if dc, ok := s["dc"].(float64); ok {
			out = fmt.Sprintf("DC %g %s save", dc, titleCase(num(s["ability"])))
		}
		if e := num(s["effect"]); e != "" {
			out += " or " + e
		} else if t := num(s["type"]); strings.HasPrefix(t, "poison") {
			out += " or poisoned"
		}
		return out
	}
	return ""
}

func (g *gen) lootSection(b *strings.Builder, m *monster) {
	if m.Loot == nil {
		return
	}
	qty := func(q []int) string {
		switch {
		case len(q) == 2 && q[0] != q[1]:
			return fmt.Sprintf("%d–%d", q[0], q[1])
		case len(q) > 0:
			return fmt.Sprint(q[0])
		}
		return "1"
	}
	var rows [][]string
	for _, e := range m.Loot.Guaranteed {
		rows = append(rows, []string{g.itemLink(e.Item), qty(e.Quantity), "always"})
	}
	tierTotal := 0
	for _, t := range m.Loot.Tiers {
		tierTotal += t.Weight
	}
	for _, t := range m.Loot.Tiers {
		entryTotal := 0
		for _, e := range t.Entries {
			entryTotal += e.Weight
		}
		if tierTotal == 0 || entryTotal == 0 {
			continue
		}
		for _, e := range t.Entries {
			if e.Item == "nothing" {
				continue
			}
			p := 100 * float64(t.Weight) / float64(tierTotal) * float64(e.Weight) / float64(entryTotal)
			rows = append(rows, []string{g.itemLink(e.Item), qty(e.Quantity), fmt.Sprintf("%.1f%%", p)})
		}
	}
	if len(rows) == 0 {
		return
	}
	b.WriteString("## Loot\n\n")
	if m.Loot.Rolls > 0 {
		b.WriteString(fmt.Sprintf("Chances are per roll; this creature gets %s on the loot table.\n\n", plural(m.Loot.Rolls, "roll")))
	}
	table(b, []string{"Item", "Quantity", "Chance"}, rows)
}

// ── spells ───────────────────────────────────────────────────────────────────

func (g *gen) spellLink(id string) string {
	if s, ok := g.spells[id]; ok {
		return link(s.Name, "Spells/"+id)
	}
	return titleCase(id)
}

func levelName(l int) string {
	if l == 0 {
		return "Cantrips"
	}
	return fmt.Sprintf("Level %d", l)
}

func (g *gen) writeSpells() error {
	byLevel := map[int][]string{}
	byClass := map[string][]string{}
	ids := sortedKeys(g.spells)
	sort.SliceStable(ids, func(i, j int) bool { return g.spells[ids[i]].Name < g.spells[ids[j]].Name })
	for _, id := range ids {
		s := g.spells[id]
		byLevel[s.Level] = append(byLevel[s.Level], id)
		for _, c := range s.Classes {
			byClass[c] = append(byClass[c], id)
		}
		if err := g.page("Spells/"+id, s.Name, g.spellPage(s)); err != nil {
			return err
		}
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Every spell in the game (%d). Spells cost mana to cast and some need material components; prepare them ahead of time.\n\n", len(ids)))
	var levels []int
	for l := range byLevel {
		levels = append(levels, l)
	}
	sort.Ints(levels)
	for _, l := range levels {
		b.WriteString("## " + levelName(l) + "\n\n")
		var rows [][]string
		for _, id := range byLevel[l] {
			s := g.spells[id]
			rows = append(rows, []string{g.spellLink(id), titleCase(s.School), fmt.Sprint(s.ManaCost), classList(s.Classes)})
		}
		table(&b, []string{"Spell", "School", "Mana", "Classes"}, rows)
	}
	b.WriteString("## By class\n\n")
	for _, c := range sortedKeys(byClass) {
		var links []string
		for _, id := range byClass[c] {
			links = append(links, g.spellLink(id))
		}
		b.WriteString("**" + titleCase(c) + ":** " + strings.Join(links, ", ") + "\n\n")
	}
	return g.page("Spells", "Spells", b.String())
}

func classList(cs []string) string {
	var out []string
	for _, c := range cs {
		out = append(out, titleCase(c))
	}
	return strings.Join(out, ", ")
}

func (g *gen) spellPage(s *spell) string {
	var b strings.Builder
	if img := g.image("/res/img/spells/"+s.School+".png", s.School, 70); img != "" {
		b.WriteString(img + "\n\n")
	}
	kind := fmt.Sprintf("Level %d %s", s.Level, s.School)
	if s.Level == 0 {
		kind = titleCase(s.School) + " cantrip"
	}
	b.WriteString("_" + kind + "_\n\n")
	b.WriteString(s.Description + "\n\n")

	rng := num(s.Range)
	if l := num(s.RangeLong); l != "" && l != rng {
		rng += "/" + l
	}
	conc := ""
	if s.Concentration {
		conc = "Yes"
	}
	dmg := num(s.Damage)
	if dmg != "" {
		dmg += " " + num(s.DamageType)
	}
	attack := ""
	if s.SpellAttack != nil && s.SpellAttack != false {
		attack = titleCase(num(s.SpellAttack))
	}
	var comps []string
	if s.Material != nil {
		for _, r := range s.Material.Required {
			comps = append(comps, fmt.Sprintf("%d × %s", r.Quantity, g.itemLink(r.Component)))
		}
	}
	var scrolls []string
	for _, id := range g.scrollsOf[s.ID] {
		scrolls = append(scrolls, g.itemLink(id))
	}
	facts(&b,
		"Mana", fmt.Sprint(s.ManaCost),
		"Casting time", titleCase(s.CastingTime),
		"Range", rng,
		"Duration", titleCase(s.Duration),
		"Concentration", conc,
		"Damage", strings.TrimSpace(dmg),
		"Healing", num(s.Heal),
		"Spell attack", attack,
		"Saving throw", titleCase(num(s.SaveType)),
		"Components", strings.Join(comps, ", "),
		"Preparation", duration(s.PrepTime),
		"Classes", classList(s.Classes),
		"Scrolls", strings.Join(scrolls, ", "),
	)
	bullets(&b, "Notes", publicNotes(s.Notes))
	bullets(&b, "Learned by", g.learnedBy(s))
	bullets(&b, "Scribed at", g.scribedAt(s))
	bullets(&b, "Scrolls drop from", g.randomScrollSources(s))
	return b.String()
}

// ── locations ────────────────────────────────────────────────────────────────

// homeLink links a city, district, environment or place of interest.
func (g *gen) homeLink(id string) string {
	if _, ok := g.pois[id]; ok {
		return g.poiLink(id)
	}
	if c, ok := g.cities[id]; ok {
		return link(c.Name, "Locations/"+id)
	}
	if e, ok := g.envs[id]; ok {
		return link(e.Name, "Locations/"+id)
	}
	if cid, ok := g.district[id]; ok {
		c := g.cities[cid]
		for _, d := range c.Districts {
			if d.ID == id {
				return link(c.Name+" — "+d.Name, "Locations/"+cid)
			}
		}
	}
	return titleCase(id)
}

func (g *gen) writeLocations() error {
	var b strings.Builder
	b.WriteString("## Towns\n\n")
	var rows [][]string
	for _, id := range sortedKeys(g.cities) {
		c := g.cities[id]
		rows = append(rows, []string{link(c.Name, "Locations/"+id), titleCase(c.Type), strings.Join(c.Races, ", ")})
		if err := g.page("Locations/"+id, c.Name, g.cityPage(c)); err != nil {
			return err
		}
	}
	table(&b, []string{"Town", "Kind", "Home of"}, rows)

	b.WriteString("## The wilds\n\n")
	rows = nil
	for _, id := range sortedKeys(g.envs) {
		e := g.envs[id]
		rows = append(rows, []string{link(e.Name, "Locations/"+id), titleCase(e.EnvType), titleCase(e.Difficulty), duration(e.TravelTime)})
		if err := g.page("Locations/"+id, e.Name, g.envPage(e)); err != nil {
			return err
		}
	}
	table(&b, []string{"Region", "Terrain", "Difficulty", "Crossing time"}, rows)

	b.WriteString("## Places of interest\n\nFound by exploring the wilds.\n\n")
	rows = nil
	for _, id := range sortedKeys(g.pois) {
		p := g.pois[id]
		rows = append(rows, []string{g.poiLink(id), titleCase(p.Category), g.homeLink(p.Environment)})
		if err := g.page("Locations/"+id, p.Name, g.poiPage(p)); err != nil {
			return err
		}
	}
	table(&b, []string{"Place", "Kind", "Region"}, rows)
	return g.page("Locations", "Locations", b.String())
}

func (g *gen) cityPage(c *city) string {
	var b strings.Builder
	if c.Image != "" {
		if img := g.image(c.Image, c.Name, 480); img != "" {
			b.WriteString(img + "\n\n")
		}
	}
	b.WriteString("_" + titleCase(c.Type) + "_ — " + c.Description + "\n\n")
	if len(c.Races) > 0 {
		b.WriteString("**Home of:** " + strings.Join(c.Races, ", ") + "\n\n")
	}

	// who works where, for the building tables
	workers := map[string][]string{}
	for _, n := range g.npcsAt[c.ID] {
		seen := map[string]bool{}
		for _, s := range g.npcs[n].Schedule {
			if s.State == "working" && !seen[s.Location] {
				seen[s.Location] = true
				workers[s.Location] = append(workers[s.Location], g.npcLink(n))
			}
		}
	}

	keys := sortedKeys(c.Districts)
	sort.SliceStable(keys, func(i, j int) bool { return keys[i] == "center" && keys[j] != "center" })
	for _, k := range keys {
		d := c.Districts[k]
		b.WriteString("## " + d.Name + "\n\n" + d.Description + "\n\n")
		if c.Waystone != nil && c.Waystone.District == k {
			b.WriteString("🌀 **The Waystone.** " + c.Waystone.Description + " (" + link("Homeward Stones & Waystones", "Rules/Teleportation") + ")\n\n")
		}
		var exits []string
		for _, dir := range sortedKeys(d.Connections) {
			exits = append(exits, titleCase(dir)+": "+g.homeLink(d.Connections[dir]))
		}
		if len(exits) > 0 {
			b.WriteString("**Exits:** " + strings.Join(exits, " · ") + "\n\n")
		}
		var rows [][]string
		for _, bl := range d.Buildings {
			if bl.private() {
				continue
			}
			about := cell(bl.Description)
			for _, st := range bl.Stations {
				about += " **" + g.stationName(st) + "**"
			}
			rows = append(rows, []string{"**" + bl.Name + "**", bl.hours(), strings.Join(workers[bl.ID], ", "), about})
		}
		table(&b, []string{"Building", "Hours", "People", "About"}, rows)
	}

	if len(c.EntryFees) > 0 {
		b.WriteString("## Vault fees\n\nWhat each race pays to register storage here:\n\n")
		var rows [][]string
		for _, r := range sortedKeys(c.EntryFees) {
			rows = append(rows, []string{r, gold(c.EntryFees[r])})
		}
		table(&b, []string{"Race", "Fee"}, rows)
	}

	var people []string
	for _, n := range g.npcsAt[c.ID] {
		people = append(people, g.npcLink(n))
	}
	bullets(&b, "People", people)
	return b.String()
}

func (g *gen) envPage(e *environment) string {
	var b strings.Builder
	if img := g.image("/res/img/environments/"+e.ID+".png", e.Name, 480); img != "" {
		b.WriteString(img + "\n\n")
	}
	b.WriteString(e.Description + "\n\n")
	var conn []string
	for _, c := range e.Connects {
		conn = append(conn, g.homeLink(c))
	}
	facts(&b,
		"Terrain", titleCase(e.EnvType),
		"Difficulty", titleCase(e.Difficulty),
		"Crossing time", duration(e.TravelTime),
		"Connects", strings.Join(conn, " · "),
	)

	var rows [][]string
	ids := sortedKeys(g.monsters)
	sort.SliceStable(ids, func(i, j int) bool { return g.monsters[ids[i]].CR < g.monsters[ids[j]].CR })
	for _, id := range ids {
		m := g.monsters[id]
		for _, h := range m.Environment {
			if h == e.EnvType {
				rows = append(rows, []string{link(m.Name, "Monsters/"+id), cr(m.CR)})
				break
			}
		}
	}
	if len(rows) > 0 {
		b.WriteString("## Creatures you may meet\n\n")
		table(&b, []string{"Monster", "CR"}, rows)
	}
	rows = nil
	for _, id := range g.poisIn[e.ID] {
		p := g.pois[id]
		rows = append(rows, []string{g.poiLink(id), titleCase(p.Category)})
	}
	if len(rows) > 0 {
		b.WriteString("## Places of interest\n\n")
		table(&b, []string{"Place", "Kind"}, rows)
	}
	var people []string
	for _, n := range g.npcsAt[e.ID] {
		people = append(people, g.npcLink(n))
	}
	bullets(&b, "People", people)
	return b.String()
}

// ── NPCs ─────────────────────────────────────────────────────────────────────

func (g *gen) npcLink(id string) string {
	if n, ok := g.npcs[id]; ok {
		return link(n.Name, "NPCs/"+id)
	}
	return titleCase(id)
}

// placeName resolves a schedule location (a building or district id) in a home.
func (g *gen) placeName(home, loc string) string {
	if c, ok := g.cities[home]; ok {
		for _, d := range c.Districts {
			if d.ID == loc {
				return d.Name
			}
			for _, bl := range d.Buildings {
				if bl.ID == loc {
					return bl.Name
				}
			}
		}
	}
	if loc == "" {
		return "Home"
	}
	return titleCase(loc)
}

func (g *gen) writeNPCs() error {
	byHome := map[string][]string{}
	for _, id := range sortedKeys(g.npcs) {
		n := g.npcs[id]
		byHome[n.PrimaryHome] = append(byHome[n.PrimaryHome], id)
		if err := g.page("NPCs/"+id, n.Name, g.npcPage(n)); err != nil {
			return err
		}
	}
	var b strings.Builder
	for _, h := range sortedKeys(byHome) {
		b.WriteString("## " + g.homeLink(h) + "\n\n")
		var rows [][]string
		for _, id := range byHome[h] {
			n := g.npcs[id]
			rows = append(rows, []string{g.npcLink(id), n.Title, n.Race})
		}
		table(&b, []string{"Name", "Role", "Race"}, rows)
	}
	return g.page("NPCs", "NPCs", b.String())
}

func (g *gen) npcPage(n *npc) string {
	var b strings.Builder
	if n.Description != "" {
		b.WriteString("_" + n.Description + "_\n\n")
	}
	facts(&b,
		"Role", n.Title,
		"Race", n.Race,
		"Home", g.homeLink(n.PrimaryHome),
	)
	bullets(&b, "Services", g.npcServices(n))
	var given []string
	for _, q := range g.questsBy[n.ID] {
		given = append(given, g.questLink(q))
	}
	bullets(&b, "Quests", given)

	var rows [][]string
	for _, s := range n.Schedule {
		rows = append(rows, []string{clock(s.Start) + "–" + clock(s.End), g.placeName(n.PrimaryHome, s.Location), titleCase(s.State)})
	}
	if len(rows) > 0 {
		b.WriteString("## Daily schedule\n\n")
		table(&b, []string{"Time", "Where", "Doing"}, rows)
	}

	if n.Shop != nil && len(n.Shop.Inventory) > 0 {
		b.WriteString("## Shop\n\n")
		if n.Shop.BuysItems {
			b.WriteString("Buys and sells. ")
		}
		b.WriteString("Prices depend on your Charisma; stock restocks over time.\n\n")
		var rows [][]string
		for _, s := range n.Shop.Inventory {
			v := ""
			if it, ok := g.items[s.ItemID]; ok {
				v = gold(it.Value)
			}
			rows = append(rows, []string{g.itemLink(s.ItemID), v, fmt.Sprint(s.MaxStock)})
		}
		table(&b, []string{"Item", "Base value", "Stock"}, rows)
	}
	return b.String()
}

// ── rules ────────────────────────────────────────────────────────────────────

func (g *gen) writeRules() error {
	// Effects
	var b strings.Builder
	b.WriteString("Conditions and effects that can be on your character. Timers run on in-game time.\n\n")
	sort.Slice(g.effects, func(i, j int) bool { return g.effects[i].Name < g.effects[j].Name })
	for _, e := range g.effects {
		b.WriteString("## " + e.Name + "\n\n")
		if e.Category != "" {
			b.WriteString("_" + titleCase(e.Category) + "_ — ")
		}
		b.WriteString(e.Description + "\n\n")
		var mods []string
		for _, m := range e.Modifiers {
			l := titleCase(m.Stat) + " " + signed(m.Value)
			if m.Type == "periodic" && m.TickInterval > 0 {
				l += " every " + duration(m.TickInterval)
			}
			mods = append(mods, l)
		}
		if len(mods) > 0 {
			b.WriteString("**Effect:** " + strings.Join(mods, ", ") + "\n\n")
		}
		if e.Removal.Type == "timed" && e.Removal.Timer > 0 {
			b.WriteString("**Lasts:** " + duration(e.Removal.Timer) + "\n\n")
		}
	}
	if err := g.page("Rules/Effects", "Conditions & Effects", b.String()); err != nil {
		return err
	}

	// Feats
	b.Reset()
	b.WriteString("At certain levels you can take a feat instead of an ability point.\n\n")
	sort.Slice(g.feats, func(i, j int) bool { return g.feats[i].Name < g.feats[j].Name })
	for _, f := range g.feats {
		b.WriteString("## " + f.Name + "\n\n" + f.Description + "\n\n")
		if f.Prerequisite != "" {
			b.WriteString("**Prerequisite:** " + titleCase(f.Prerequisite) + "\n\n")
		}
		bullets(&b, "", f.Effects)
	}
	if err := g.page("Rules/Feats", "Feats", b.String()); err != nil {
		return err
	}

	// Class abilities
	b.Reset()
	byClass := map[string][]*ability{}
	for _, a := range g.abilities {
		byClass[a.Class] = append(byClass[a.Class], a)
	}
	for _, c := range sortedKeys(byClass) {
		as := byClass[c]
		sort.Slice(as, func(i, j int) bool {
			if as[i].UnlockLevel != as[j].UnlockLevel {
				return as[i].UnlockLevel < as[j].UnlockLevel
			}
			return as[i].Name < as[j].Name
		})
		b.WriteString("## " + titleCase(c) + "\n\n")
		for _, a := range as {
			meta := fmt.Sprintf("Level %d · %d %s", a.UnlockLevel, a.ResourceCost, a.ResourceType)
			if a.Cooldown != "" && a.Cooldown != "none" {
				meta += " · " + strings.ReplaceAll(a.Cooldown, "_", " ")
			}
			b.WriteString(fmt.Sprintf("### %s\n\n_%s_\n\n%s\n\n", a.Name, meta, a.Description))
			var rows [][]string
			for _, t := range a.Tiers {
				lv := fmt.Sprintf("%d–%d", t.MinLevel, t.MaxLevel)
				if t.MaxLevel >= 99 {
					lv = fmt.Sprintf("%d+", t.MinLevel)
				}
				rows = append(rows, []string{lv, cell(t.Summary)})
			}
			table(&b, []string{"Levels", "Effect"}, rows)
		}
	}
	if err := g.page("Rules/Class-Abilities", "Class Abilities", b.String()); err != nil {
		return err
	}

	return g.page("Rules", "Rules", "## How the game works\n\n"+
		"- [[Combat|/Rules/Combat]]\n- [[Hunger, Fatigue & Carrying|/Rules/Survival]]\n- [[Day & Night|/Rules/Day-and-Night]]\n"+
		"- [[Learning Spells|/Rules/Learning-Spells]]\n- [[Scribing Scrolls|/Rules/Scribing]]\n- [[Homeward Stones & Waystones|/Rules/Teleportation]]\n\n"+
		"## Reference\n\n- [[Conditions & Effects|/Rules/Effects]]\n- [[Feats|/Rules/Feats]]\n- [[Class Abilities|/Rules/Class-Abilities]]\n")
}

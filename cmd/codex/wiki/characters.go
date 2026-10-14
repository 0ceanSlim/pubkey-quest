package wiki

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Character pages: races, classes, skills and leveling, from systems/new-character,
// systems/{skills,advancement,class-resources}.json and magic/.

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func loadCharacters(dataDir string) (*characters, error) {
	sys := filepath.Join(dataDir, "systems")
	nc := filepath.Join(sys, "new-character")
	c := &characters{}
	var hp struct {
		BaseHP map[string]string `json:"base-hp"`
	}
	var weights struct {
		Races        []string                  `json:"Races"`
		RaceWeights  []int                     `json:"RaceWeights"`
		ClassWeights map[string]map[string]int `json:"classWeightsByRace"`
	}
	var locs struct {
		Info map[string]struct {
			City        string `json:"starting_city"`
			Description string `json:"description"`
		} `json:"racial_descriptions"`
	}
	var startGold struct {
		Gold [][]any `json:"starting-gold"`
	}
	reads := []struct {
		path string
		v    any
	}{
		{filepath.Join(sys, "skills.json"), &c.Skills},
		{filepath.Join(sys, "advancement.json"), &c.Levels},
		{filepath.Join(sys, "class-resources.json"), &c.Resources},
		{filepath.Join(nc, "base-hp.json"), &hp},
		{filepath.Join(nc, "starting-gear.json"), &c.Gear},
		{filepath.Join(nc, "starting-spells.json"), &c.Spells},
		{filepath.Join(nc, "generation-weights.json"), &weights},
		{filepath.Join(nc, "starting-locations.json"), &locs},
		{filepath.Join(nc, "starting-gold.json"), &startGold},
		{filepath.Join(dataDir, "magic", "spell-slots.json"), &c.Slots},
	}
	for _, r := range reads {
		if err := readJSON(r.path, r.v); err != nil {
			return nil, err
		}
	}
	c.BaseHP, c.Races, c.RaceWeights = hp.BaseHP, weights.Races, weights.RaceWeights
	c.ClassWeights, c.RaceInfo, c.StartingGold = weights.ClassWeights, locs.Info, startGold.Gold
	return c, nil
}

func classLink(class string) string {
	return link(titleCase(class), "Characters/"+strings.ToLower(class))
}

func (g *gen) writeCharacters() error {
	c := g.chars
	classes := sortedKeys(c.BaseHP)
	for _, cl := range classes {
		if err := g.page("Characters/"+strings.ToLower(cl), cl, g.classPage(cl)); err != nil {
			return err
		}
	}

	// Index
	var b strings.Builder
	b.WriteString("Your character is generated from your Nostr key: race, class, background, " +
		"stats and starting gear are the same every time for the same key.\n\n")
	b.WriteString("- [[Races|/Characters/Races]]\n- [[Skills|/Characters/Skills]]\n- [[Leveling|/Characters/Leveling]]\n\n")
	b.WriteString("## Classes\n\n")
	var rows [][]string
	for _, cl := range classes {
		res := ""
		if r, ok := c.Resources[strings.ToLower(cl)]; ok {
			res = r.Label
			if res == "" {
				res = titleCase(r.Type)
			}
		}
		rows = append(rows, []string{classLink(cl), "d" + c.BaseHP[cl], res})
	}
	table(&b, []string{"Class", "Hit die", "Resource"}, rows)
	if len(c.StartingGold) > 0 {
		b.WriteString("## Backgrounds\n\nYour background sets your starting gold.\n\n")
		rows = nil
		for _, bg := range c.StartingGold {
			if len(bg) == 2 {
				rows = append(rows, []string{num(bg[0]), gold(int(toFloat(bg[1])))})
			}
		}
		table(&b, []string{"Background", "Starting gold"}, rows)
	}
	if err := g.page("Characters", "Characters", b.String()); err != nil {
		return err
	}

	// Races
	b.Reset()
	total := 0
	for _, w := range c.RaceWeights {
		total += w
	}
	for i, r := range c.Races {
		b.WriteString("## " + r + "\n\n")
		info := c.RaceInfo[r]
		if info.Description != "" {
			b.WriteString(info.Description + "\n\n")
		}
		share := ""
		if i < len(c.RaceWeights) && total > 0 {
			share = fmt.Sprintf("%.0f%% of characters", 100*float64(c.RaceWeights[i])/float64(total))
		}
		facts(&b,
			"Starts in", g.homeLink(info.City),
			"How common", share,
			"Usual classes", topClasses(c.ClassWeights[r], 3),
		)
	}
	if err := g.page("Characters/Races", "Races", b.String()); err != nil {
		return err
	}

	// Skills
	b.Reset()
	b.WriteString("Skills grow from your ability scores in the proportions below.\n\n")
	rows = nil
	for _, id := range sortedKeys(c.Skills) {
		s := c.Skills[id]
		rows = append(rows, []string{"**" + s.Name + "**", cell(s.Description), ratioText(s.Ratio)})
	}
	table(&b, []string{"Skill", "Used for", "Built from"}, rows)
	if err := g.page("Characters/Skills", "Skills", b.String()); err != nil {
		return err
	}

	// Leveling
	b.Reset()
	b.WriteString("Experience needed for each level, and the ability points a level grants.\n\n")
	rows = nil
	for _, l := range c.Levels {
		ap := ""
		if l.AbilityPoints > 0 {
			ap = fmt.Sprint(l.AbilityPoints)
		}
		rows = append(rows, []string{fmt.Sprint(l.Level), fmt.Sprint(l.XP), ap})
	}
	table(&b, []string{"Level", "XP", "Ability points"}, rows)
	return g.page("Characters/Leveling", "Leveling", b.String())
}

func toFloat(v any) float64 {
	f, _ := v.(float64)
	return f
}

func topClasses(w map[string]int, n int) string {
	names := sortedKeys(w)
	sort.SliceStable(names, func(i, j int) bool { return w[names[i]] > w[names[j]] })
	if len(names) > n {
		names = names[:n]
	}
	var out []string
	for _, c := range names {
		out = append(out, classLink(c))
	}
	return strings.Join(out, ", ")
}

func ratioText(r map[string]float64) string {
	keys := sortedKeys(r)
	sort.SliceStable(keys, func(i, j int) bool { return r[keys[i]] > r[keys[j]] })
	var out []string
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%s %.0f%%", titleCase(k), r[k]*100))
	}
	return strings.Join(out, ", ")
}

func (g *gen) classPage(class string) string {
	c := g.chars
	key := strings.ToLower(class)
	var b strings.Builder

	res := ""
	if r, ok := c.Resources[key]; ok {
		res = r.Label
		if res == "" {
			res = titleCase(r.Type)
		}
		if r.Max > 0 {
			res += fmt.Sprintf(" (max %d", r.Max)
			for _, k := range sortedKeys(r.Regen) {
				res += fmt.Sprintf(", +%d %s", r.Regen[k], strings.ReplaceAll(k, "_", " "))
			}
			res += ")"
		}
	}
	var races []string
	for _, r := range c.Races {
		if w := c.ClassWeights[r]; w != nil && topContains(w, class, 3) {
			races = append(races, r)
		}
	}
	facts(&b,
		"Hit die", "d"+c.BaseHP[class],
		"Resource", res,
		"Common among", strings.Join(races, ", "),
	)

	// Abilities
	var abil []*ability
	for _, a := range g.abilities {
		if a.Class == key {
			abil = append(abil, a)
		}
	}
	if len(abil) > 0 {
		sort.Slice(abil, func(i, j int) bool { return abil[i].UnlockLevel < abil[j].UnlockLevel })
		b.WriteString("## Abilities\n\n")
		var rows [][]string
		for _, a := range abil {
			rows = append(rows, []string{"**" + a.Name + "**", fmt.Sprint(a.UnlockLevel), fmt.Sprintf("%d %s", a.ResourceCost, a.ResourceType), cell(a.Description)})
		}
		table(&b, []string{"Ability", "Level", "Cost", "Effect"}, rows)
		b.WriteString("Full scaling: [[Class Abilities|/Rules/Class-Abilities]]\n\n")
	}

	// Starting gear
	for _, sg := range c.Gear {
		if !strings.EqualFold(sg.Class, class) {
			continue
		}
		b.WriteString("## Starting gear\n\n")
		if len(sg.Gear.Given) > 0 {
			b.WriteString("Every " + class + " starts with:\n\n")
			for _, it := range sg.Gear.Given {
				b.WriteString(fmt.Sprintf("- %d × %s\n", max(it.Quantity, 1), g.itemLink(it.Item)))
			}
			b.WriteString("\n…plus one pick from each list, decided by your key:\n\n")
		} else {
			b.WriteString("One pick from each list, decided by your key:\n\n")
		}
		for _, ch := range sg.Gear.Choices {
			var opts []string
			for _, o := range ch.Options {
				if s := g.gearOption(o); s != "" {
					opts = append(opts, s)
				}
			}
			b.WriteString("- " + strings.Join(dedupe(opts), " · ") + "\n")
		}
		b.WriteString("\n")
	}

	// Spells
	if sp, ok := c.Spells[key]; ok {
		b.WriteString("## Starting spells\n\n")
		for _, tier := range sortedKeys(sp) {
			var links []string
			for _, id := range sp[tier] {
				links = append(links, g.spellLink(id))
			}
			b.WriteString("**" + titleCase(strings.Replace(tier, "level1", "level 1", 1)) + ":** " + strings.Join(links, ", ") + "\n\n")
		}
	}
	g.slotTable(&b, key)

	var list []string
	for _, id := range sortedKeys(g.spells) {
		for _, cl := range g.spells[id].Classes {
			if cl == key {
				list = append(list, g.spellLink(id))
			}
		}
	}
	if len(list) > 0 {
		b.WriteString("## Spell list\n\n" + strings.Join(list, ", ") + "\n")
	}
	return b.String()
}

func topContains(w map[string]int, class string, n int) bool {
	names := sortedKeys(w)
	sort.SliceStable(names, func(i, j int) bool { return w[names[i]] > w[names[j]] })
	for i, c := range names {
		if i >= n {
			break
		}
		if strings.EqualFold(c, class) {
			return true
		}
	}
	return false
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func (g *gen) gearOption(o gearOption) string {
	qty := func(id string, n int) string {
		if n > 1 {
			return fmt.Sprintf("%d × %s", n, g.itemLink(id))
		}
		return g.itemLink(id)
	}
	switch o.Type {
	case "single":
		return qty(o.Item, o.Quantity)
	case "bundle":
		var parts []string
		for _, it := range o.Items {
			parts = append(parts, qty(it.Item, it.Quantity))
		}
		return strings.Join(parts, " + ")
	case "multi_slot":
		var parts []string
		for _, s := range o.Slots {
			if s.Item != "" {
				parts = append(parts, qty(s.Item, s.Quantity))
			} else if len(s.Options) > 0 {
				parts = append(parts, "a "+strings.ReplaceAll(strings.TrimSuffix(s.Type, "_choice"), "_", " ")+" of your key's choosing")
			}
		}
		return strings.Join(parts, " + ")
	}
	return ""
}

// slotTable renders the class's spell slots per character level.
func (g *gen) slotTable(b *strings.Builder, class string) {
	raw, ok := g.chars.Slots[class]
	if !ok {
		return
	}
	var byLevel map[string]map[string]int
	if json.Unmarshal(raw, &byLevel) != nil || len(byLevel) == 0 {
		return
	}
	cols := map[string]bool{}
	var levels []int
	for l, slots := range byLevel {
		var n int
		if _, err := fmt.Sscan(l, &n); err == nil {
			levels = append(levels, n)
		}
		for k := range slots {
			cols[k] = true
		}
	}
	sort.Ints(levels)
	colKeys := sortedKeys(cols)
	sort.SliceStable(colKeys, func(i, j int) bool { return colKeys[i] == "cantrips" && colKeys[j] != "cantrips" })
	head := []string{"Level"}
	for _, k := range colKeys {
		head = append(head, strings.TrimPrefix(titleCase(k), "Level "))
	}
	var rows [][]string
	hasSlots := false
	for _, l := range levels {
		row := []string{fmt.Sprint(l)}
		for _, k := range colKeys {
			v := byLevel[fmt.Sprint(l)][k]
			if v == 0 {
				row = append(row, "")
			} else {
				hasSlots = true
				row = append(row, fmt.Sprint(v))
			}
		}
		rows = append(rows, row)
	}
	if !hasSlots {
		return
	}
	b.WriteString("## Spell slots\n\nSlots by character level; numbered columns are spell levels.\n\n")
	table(b, head, rows)
}

package wiki

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"pubkey-quest/types"
)

// Rules guides: "how the game works" pages built from the game's own config
// files (teleportation, scribing, spell progression, survival, combat terrain,
// the clock). Numbers come from the data; the few rules that live only in the
// engine are stated in prose next to the code they mirror.

// guideRoadClockSpeed is how many in-game minutes pass per real minute on the
// road (src/systems/smoothClock.js TIME_MULTIPLIER); towns scale it by
// time-config's town_time_scale.
const guideRoadClockSpeed = 144

// writeGuides writes the six Rules guide pages.
func (g *gen) writeGuides() error {
	c, err := g.guideLoad()
	if err != nil {
		return err
	}
	pages := []struct {
		rel, title string
		body       func() (string, error)
	}{
		{"Rules/Teleportation", "Homeward Stones & Waystones", c.guideTeleportPage},
		{"Rules/Scribing", "Scribing Scrolls", c.guideScribingPage},
		{"Rules/Learning-Spells", "Learning Spells", c.guideLearningPage},
		{"Rules/Survival", "Hunger, Fatigue & Carrying", c.guideSurvivalPage},
		{"Rules/Combat", "Combat", c.guideCombatPage},
		{"Rules/Day-and-Night", "Day & Night", c.guideDayNightPage},
	}
	for _, p := range pages {
		body, err := p.body()
		if err != nil {
			return fmt.Errorf("%s: %w", p.rel, err)
		}
		if err := g.page(p.rel, p.title, body); err != nil {
			return err
		}
	}
	return nil
}

// ── data the guides read beyond what the reference pages load ────────────────

type guideEffect struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Removal     struct {
		Type  string `json:"type"`
		Timer int    `json:"timer"`
	} `json:"removal"`
	Modifiers []struct {
		Stat         string `json:"stat"`
		Value        int    `json:"value"`
		Type         string `json:"type"`
		TickInterval int    `json:"tick_interval"`
	} `json:"modifiers"`
	SystemCheck *struct {
		Stat     string `json:"stat"`
		Operator string `json:"operator"`
		Value    int    `json:"value"`
	} `json:"system_check"`
	SkillScaling *struct {
		Skill          string  `json:"skill"`
		BaseLevel      int     `json:"base_level"`
		BonusPerLevel  float64 `json:"bonus_per_level"`
		MaxBonusLevels int     `json:"max_bonus_levels"`
	} `json:"skill_scaling"`
	CombatBuff *struct {
		HitDice  string `json:"hit_dice"`
		HitType  string `json:"hit_type"`
		Hits     int    `json:"hits"`
		HitBonus int    `json:"hit_bonus"`
	} `json:"combat_buff"`
	Visible bool `json:"visible"`
}

type guideItem struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	GearSlot string   `json:"gear_slot"`
	Tags     []string `json:"tags"`
	Heal     string   `json:"heal"`
	Reusable bool     `json:"reusable"`
	Range    any      `json:"range"`
	RangeL   any      `json:"range_long"`
	Effects  []struct {
		Type        string `json:"type"`
		Value       int    `json:"value"`
		ApplyEffect string `json:"apply_effect"`
		Chance      int    `json:"chance"`
	} `json:"effects"`
	EffectsWhenWorn []string `json:"effects_when_worn"`
	Throw           *struct {
		Damage     string `json:"damage"`
		DamageType string `json:"damage_type"`
		DC         int    `json:"dc"`
		Condition  *struct {
			Condition    string `json:"condition"`
			SaveEachTurn string `json:"save_each_turn"`
			DamageDice   string `json:"damage_dice"`
			DamageType   string `json:"damage_type"`
		} `json:"condition"`
	} `json:"throw"`
	Scatter *struct {
		Terrain string `json:"terrain"`
		Cells   int    `json:"cells"`
	} `json:"scatter"`
}

func (it *guideItem) hasTag(tag string) bool {
	for _, t := range it.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

type guideMonster struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Active  string `json:"active"`
	Special []struct {
		Name string `json:"name"`
	} `json:"special_abilities"`
}

type guideSpell struct {
	ID            string `json:"-"`
	AppliesEffect string `json:"applies_effect"`
}

func (s *guideSpell) setFile(path string) {
	s.ID = strings.TrimSuffix(filepath.Base(path), ".json")
}

type guideCity struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Districts map[string]struct {
		Name      string `json:"name"`
		Buildings []struct {
			ID       string   `json:"id"`
			Name     string   `json:"name"`
			Stations []string `json:"crafting_stations"`
		} `json:"buildings"`
	} `json:"districts"`
}

type guideNPC struct {
	ID          string                     `json:"id"`
	PrimaryHome string                     `json:"primary_home"`
	Dialogue    map[string]json.RawMessage `json:"dialogue"`
}

// guideCtx is the generator plus the extra data the guides read.
type guideCtx struct {
	*gen
	effects  map[string]*guideEffect // every effect, visible or not
	items    map[string]*guideItem
	monsters []*guideMonster
	spells   []*guideSpell
	cities   map[string]*guideCity
	npcs     []*guideNPC
}

func (g *gen) guideLoad() (*guideCtx, error) {
	d := g.opt.DataDir
	c := &guideCtx{gen: g, effects: map[string]*guideEffect{}, items: map[string]*guideItem{}, cities: map[string]*guideCity{}}
	effs, err := loadDir[guideEffect](filepath.Join(d, "effects"), false, nil)
	if err != nil {
		return nil, err
	}
	for _, e := range effs {
		c.effects[e.ID] = e
	}
	items, err := loadDir[guideItem](filepath.Join(d, "items"), false, nil)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		c.items[it.ID] = it
	}
	if c.monsters, err = loadDir[guideMonster](filepath.Join(d, "monsters"), false, nil); err != nil {
		return nil, err
	}
	if c.spells, err = loadDir[guideSpell](filepath.Join(d, "magic", "spells"), false, nil); err != nil {
		return nil, err
	}
	cities, err := loadDir[guideCity](filepath.Join(d, "locations", "cities"), false, nil)
	if err != nil {
		return nil, err
	}
	for _, ct := range cities {
		c.cities[ct.ID] = ct
	}
	if c.npcs, err = loadDir[guideNPC](filepath.Join(d, "npcs"), true, nil); err != nil {
		return nil, err
	}
	return c, nil
}

// ── shared helpers ───────────────────────────────────────────────────────────

func (c *guideCtx) guideEffectName(id string) string {
	if e, ok := c.effects[id]; ok && e.Name != "" {
		if e.Visible {
			return link(e.Name, "Rules/Effects")
		}
		return e.Name
	}
	return titleCase(id)
}

// guideStatName names a stat a modifier touches.
func guideStatName(stat string) string {
	switch strings.ToLower(stat) {
	case "hp":
		return "HP"
	case "ac":
		return "AC"
	}
	return titleCase(stat)
}

// guideMods renders an effect's constant ability modifiers ("Dexterity -1, Strength -1").
func guideMods(e *guideEffect) string {
	if e == nil {
		return ""
	}
	var out []string
	for _, m := range e.Modifiers {
		switch m.Type {
		case "constant":
			out = append(out, guideStatName(m.Stat)+" "+signed(m.Value))
		case "periodic":
			if m.TickInterval > 0 {
				out = append(out, fmt.Sprintf("%s %s %s", guideStatName(m.Stat), signed(m.Value), guideEvery(m.TickInterval)))
			}
		}
	}
	return strings.Join(out, ", ")
}

// guideCompare evaluates a system check's operator.
func guideCompare(v int, op string, want int) bool {
	switch op {
	case "==":
		return v == want
	case "!=":
		return v != want
	case "<":
		return v < want
	case "<=":
		return v <= want
	case ">":
		return v > want
	case ">=":
		return v >= want
	}
	return false
}

// guideStatusAt is the status effect the game shows at a value of a tracked
// stat (hunger, fatigue, weight_percent), picked the way the game picks it:
// exact checks first, then ranges, higher thresholds first; one effect at a time.
func (c *guideCtx) guideStatusAt(stat string, v int) *guideEffect {
	var cands []*guideEffect
	for _, id := range sortedKeys(c.effects) {
		e := c.effects[id]
		if e.SystemCheck != nil && e.SystemCheck.Stat == stat {
			cands = append(cands, e)
		}
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i].SystemCheck, cands[j].SystemCheck
		if (a.Operator == "==") != (b.Operator == "==") {
			return a.Operator == "=="
		}
		return a.Value > b.Value
	})
	for _, e := range cands {
		if guideCompare(v, e.SystemCheck.Operator, e.SystemCheck.Value) {
			return e
		}
	}
	return nil
}

type guideBand struct {
	from, to int
	effect   *guideEffect
}

// guideBands groups lo..hi into runs that show the same status effect.
func (c *guideCtx) guideBands(stat string, lo, hi int) []guideBand {
	var out []guideBand
	for v := lo; v <= hi; v++ {
		e := c.guideStatusAt(stat, v)
		if n := len(out); n > 0 && out[n-1].effect == e {
			out[n-1].to = v
			continue
		}
		out = append(out, guideBand{v, v, e})
	}
	return out
}

func guideRange(from, to int) string {
	if from == to {
		return fmt.Sprint(from)
	}
	return fmt.Sprintf("%d–%d", from, to)
}

func guidePercent(f float64) string { return fmt.Sprintf("%g%%", f*100) }

// guideQuestLevel is the level a quest asks for (0 when none).
func (c *guideCtx) guideQuestLevel(id string) int {
	q, ok := c.quests[id]
	if !ok {
		return 0
	}
	for _, r := range q.Requirements {
		if r.Type == "level" {
			return r.Min
		}
	}
	return 0
}

func guideLevelText(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprint(n)
}

func (c *guideCtx) guideRead(rel string, v any) error {
	return readJSON(filepath.Join(c.opt.DataDir, filepath.FromSlash(rel)), v)
}

// guideEvery is "every hour", "every 4 hours".
func guideEvery(min int) string {
	return "every " + strings.TrimPrefix(duration(min), "1 ")
}

// guideOnce is "once a day", "once an hour", "once every 3 days".
func guideOnce(min int) string {
	d := duration(min)
	switch {
	case d == "1 hour":
		return "once an hour"
	case strings.HasPrefix(d, "1 "):
		return "once a " + strings.TrimPrefix(d, "1 ")
	}
	return "once every " + d
}

func guideJoinOr(parts []string) string {
	if len(parts) < 2 {
		return guideJoinAnd(parts)
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " or " + parts[len(parts)-1]
}

func guideJoinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// ── 1. Homeward Stones & waystones ───────────────────────────────────────────

type guideTeleportConfig struct {
	StoneCooldown    string   `json:"stone_cooldown_effect"`
	WaystoneCooldown string   `json:"waystone_cooldown_effect"`
	Chain            []string `json:"chain"`
	Cities           map[string]struct {
		Name          string `json:"name"`
		District      string `json:"district"`
		Stone         string `json:"stone"`
		Keeper        string `json:"keeper"`
		StoneQuest    string `json:"stone_quest"`
		WaystoneQuest string `json:"waystone_quest"`
	} `json:"cities"`
}

func (c *guideCtx) guideTeleportPage() (string, error) {
	var cfg guideTeleportConfig
	if err := c.guideRead("systems/teleportation.json", &cfg); err != nil {
		return "", err
	}
	cooldown := func(id string) string {
		if e, ok := c.effects[id]; ok {
			return duration(e.Removal.Timer)
		}
		return ""
	}
	square := func(city, district string) string {
		if ct, ok := c.cities[city]; ok {
			if d, ok := ct.Districts[district]; ok && d.Name != "" {
				return d.Name
			}
		}
		return "the town square"
	}
	cityLink := func(id string) string { return c.homeLink(id) }

	// Price: what the keepers' shops charge is the stone item's value.
	price := 0
	for _, id := range cfg.Chain {
		if it, ok := c.gen.items[cfg.Cities[id].Stone]; ok && it.Value > 0 {
			price = it.Value
			break
		}
	}

	var b strings.Builder
	b.WriteString("Homeward Stones carry you back to a city in an instant, and the waystones in the town squares " +
		"carry you between cities you have woken. Neither one makes time pass.\n\n")

	b.WriteString("## Homeward Stones\n\n")
	bullets(&b, "", []string{
		"Every new character starts with their home city's stone. It is the only free one.",
		"A character made before stones existed can claim their home stone once, free, from their home city's vault keeper.",
		fmt.Sprintf("Vault keepers sell their city's stone for %s (base price; your Charisma moves it a little). "+
			"They sell to people born in that city, or to anyone who has finished the city's stone quest.", gold(price)),
		"No keeper sells you a stone you already have, whether it's in your packs, your gear or your vault.",
		"Using a stone takes you to that city's waystone in its town square. No time passes. If you were on the road, that trip is abandoned.",
		"The stone has to be in your packs to use it, not in your vault. It isn't used up.",
		fmt.Sprintf("All stones share one charge: after any stone carries you, none of them works for %s.", cooldown(cfg.StoneCooldown)),
		"You can't use a stone in a fight, inside a dungeon or other place of interest, during an encounter, while picking a lock, or while you're in a party.",
	})

	var rows [][]string
	for _, id := range cfg.Chain {
		ct := cfg.Cities[id]
		natives := ""
		if gc, ok := c.gen.cities[id]; ok {
			natives = strings.Join(gc.Races, ", ")
		}
		rows = append(rows, []string{cityLink(id), c.itemLink(ct.Stone), c.npcLink(ct.Keeper), natives,
			c.questLink(ct.StoneQuest), guideLevelText(c.guideQuestLevel(ct.StoneQuest))})
	}
	b.WriteString("### Stones and their keepers\n\nNatives start with their city's stone and can always buy a replacement. Everyone else does the stone quest first.\n\n")
	table(&b, []string{"City", "Stone", "Keeper", "Natives", "Stone quest", "Level"}, rows)

	b.WriteString("## Waystones\n\n")
	bullets(&b, "", []string{
		"Each of these cities has a waystone in its town square. Stand at it (outside any building) to use it.",
		"From a waystone you can hop to any other waystone you have woken.",
		fmt.Sprintf("Hops have their own charge, separate from the stones: after a hop, the waystones won't carry you again for %s.", cooldown(cfg.WaystoneCooldown)),
		"The same limits apply as for stones: not in a fight, an encounter, a dungeon, while picking a lock, or in a party.",
		"You wake a waystone by finishing its quest. The quests form one chain, each opening after the one before.",
	})
	rows = nil
	for i, id := range cfg.Chain {
		ct := cfg.Cities[id]
		rows = append(rows, []string{fmt.Sprint(i + 1), cityLink(id), square(id, ct.District),
			c.questLink(ct.WaystoneQuest), guideLevelText(c.guideQuestLevel(ct.WaystoneQuest))})
	}
	table(&b, []string{"Order", "City", "Waystone stands in", "Quest", "Level"}, rows)

	ferry, err := c.guideFerryman()
	if err != nil {
		return "", err
	}
	b.WriteString(ferry)
	return b.String(), nil
}

// guideFerryman summarizes the ferry encounter: the other paid shortcut.
func (c *guideCtx) guideFerryman() (string, error) {
	var e struct {
		Name      string   `json:"name"`
		Chance    float64  `json:"chance"`
		Cooldown  int      `json:"cooldown_minutes"`
		Locations []string `json:"valid_locations"`
		Nodes     map[string]struct {
			Type  string `json:"type"`
			Skill string `json:"skill"`
			DC    int    `json:"dc"`
			Cost  *struct {
				Gold int `json:"gold"`
			} `json:"cost"`
			TravelAhead float64 `json:"travel_ahead"`
		} `json:"nodes"`
	}
	if err := c.guideRead("systems/encounters-draft/ferryman.json", &e); err != nil {
		return "", err
	}
	var fares []int
	ahead := 0.0
	haggle := ""
	for _, id := range sortedKeys(e.Nodes) {
		n := e.Nodes[id]
		if n.Type == "transaction" && n.Cost != nil && n.Cost.Gold > 0 {
			fares = append(fares, n.Cost.Gold)
			if n.TravelAhead > ahead {
				ahead = n.TravelAhead
			}
		}
		if n.Type == "check" && n.Skill != "" && n.DC > 0 && haggle == "" {
			haggle = fmt.Sprintf("%s DC %d check", titleCase(n.Skill), n.DC)
		}
	}
	if len(fares) == 0 {
		return "", nil
	}
	sort.Sort(sort.Reverse(sort.IntSlice(fares)))
	var where []string
	for _, id := range e.Locations {
		if env, ok := c.envs[id]; ok {
			where = append(where, link(env.Name, "Locations/"+id))
		} else {
			where = append(where, c.homeLink(id))
		}
	}
	fare := gold(fares[0])
	if len(fares) > 1 && haggle != "" {
		fare += fmt.Sprintf(" (%s if you haggle and win an %s)", gold(fares[len(fares)-1]), haggle)
	}
	var b strings.Builder
	b.WriteString("## The ferryman\n\n")
	b.WriteString(fmt.Sprintf("Crossing %s on foot, you may meet [[%s|/Encounters]]: a %.0f%% chance, at most %s. ",
		guideJoinAnd(where), e.Name, e.Chance*100, guideOnce(e.Cooldown)))
	b.WriteString(fmt.Sprintf("For %s he poles you %.0f%% of the way across the region you're crossing.\n\n", fare, ahead*100))
	return b.String(), nil
}

// ── 2. Scribing scrolls ──────────────────────────────────────────────────────

type guideScribing struct {
	Tiers map[string]struct {
		Gold       int `json:"gold"`
		Minutes    int `json:"minutes"`
		BaseChance int `json:"base_chance"`
	} `json:"tiers"`
	MaxSpellLevel int `json:"max_spell_level"`
	PerMod        int `json:"per_mod"`
	MinChance     int `json:"min_chance"`
	MaxChance     int `json:"max_chance"`
	Consumes      []struct {
		Item     string `json:"item"`
		Quantity int    `json:"quantity"`
	} `json:"consumes"`
	Carried  []string `json:"carried"`
	Memorize struct {
		Classes         []string `json:"classes"`
		GoldPerLevel    int      `json:"gold_per_level"`
		MinutesPerLevel int      `json:"minutes_per_level"`
	} `json:"memorize"`
	Traditions map[string]struct {
		Name    string   `json:"name"`
		Station string   `json:"station"`
		Classes []string `json:"classes"`
	} `json:"traditions"`
}

func (c *guideCtx) guideScribingPage() (string, error) {
	var cfg guideScribing
	if err := c.guideRead("magic/scroll-scribing.json", &cfg); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Spellcasters can write their spells onto scrolls. Anyone can read a scroll later to cast the spell, " +
		"without knowing it or having it prepared.\n\n")

	// What it takes
	var consumed []string
	for _, it := range cfg.Consumes {
		consumed = append(consumed, fmt.Sprintf("%d × %s", max(it.Quantity, 1), c.itemLink(it.Item)))
	}
	var carried []string
	for _, id := range cfg.Carried {
		carried = append(carried, c.itemLink(id))
	}
	b.WriteString("## What you need\n\n")
	bullets(&b, "", []string{
		fmt.Sprintf("A spell you know, of a spell level you have slots for (up to level %d). See [[Learning Spells|/Rules/Learning-Spells]].", cfg.MaxSpellLevel),
		"A scribing station of your class's tradition (below). Stand inside the building.",
		guideJoinAnd(consumed) + ", used up.",
		guideJoinAnd(carried) + " in your packs. These aren't used up.",
		"The spell's own material components, if it has any. They are used up.",
		"Gold and time, by spell level:",
	})
	var rows [][]string
	levels := sortedKeys(cfg.Tiers)
	sort.SliceStable(levels, func(i, j int) bool { a, _ := strconv.Atoi(levels[i]); b, _ := strconv.Atoi(levels[j]); return a < b })
	for _, l := range levels {
		t := cfg.Tiers[l]
		n, _ := strconv.Atoi(l)
		rows = append(rows, []string{strings.TrimSuffix(levelName(n), "s"), gold(t.Gold), duration(t.Minutes), fmt.Sprintf("%d%%", t.BaseChance)})
	}
	table(&b, []string{"Spell level", "Gold", "Time", "Base chance"}, rows)

	b.WriteString("## Your chance\n\n")
	b.WriteString(fmt.Sprintf("Your chance is the base chance, plus %d%% for each point of your spellcasting ability modifier "+
		"(minus %d%% for each point below zero). It never goes below %d%% or above %d%%.\n\n",
		cfg.PerMod, cfg.PerMod, cfg.MinChance, cfg.MaxChance))
	b.WriteString("If the attempt fails, the parchment, components and gold are still spent and the time still passes. You just don't get a scroll.\n\n")

	// Traditions and where their stations are
	b.WriteString("## Where to scribe\n\n")
	rows = nil
	for _, id := range sortedKeys(cfg.Traditions) {
		t := cfg.Traditions[id]
		var classes []string
		for _, cl := range t.Classes {
			classes = append(classes, classLink(cl))
		}
		var where []string
		for _, cid := range sortedKeys(c.cities) {
			ct := c.cities[cid]
			for _, dk := range sortedKeys(ct.Districts) {
				for _, bl := range ct.Districts[dk].Buildings {
					for _, s := range bl.Stations {
						if s == id {
							where = append(where, bl.Name+" in "+link(ct.Name, "Locations/"+cid))
						}
					}
				}
			}
		}
		rows = append(rows, []string{titleCase(t.Name), strings.Join(classes, ", "), t.Station, strings.Join(where, "<br>")})
	}
	table(&b, []string{"Tradition", "Classes", "Station", "Found at"}, rows)

	b.WriteString("## Reading a scroll\n\n")
	bullets(&b, "", []string{
		"Any character can read any scroll, as long as they have the mana: it costs the spell's usual mana.",
		"You don't need to know the spell, have it prepared, or carry its components.",
		"The scroll is used up. In a fight, reading one takes your action.",
	})

	if len(cfg.Memorize.Classes) > 0 {
		var who []string
		for _, cl := range cfg.Memorize.Classes {
			who = append(who, classLink(cl))
		}
		b.WriteString("## Commit to memory\n\n")
		if len(who) == 1 {
			b.WriteString("A " + who[0] + " can learn a spell for good from a scroll.\n\n")
		} else {
			b.WriteString(guideJoinAnd(who) + " characters can learn a spell for good from a scroll.\n\n")
		}
		bullets(&b, "", []string{
			"Put the scroll inside your " + c.itemLink("spellbook") + ", then right-click it and choose **Commit to memory**.",
			"The spell must be on your class's list, of a spell level you have slots for, and not one you already know.",
			"Cantrips can't be committed to memory.",
			"There's no roll. The scroll is used up.",
			fmt.Sprintf("It costs %s and %s per spell level.", gold(cfg.Memorize.GoldPerLevel), duration(cfg.Memorize.MinutesPerLevel)),
		})
		rows = nil
		for l := 1; l <= cfg.MaxSpellLevel; l++ {
			rows = append(rows, []string{fmt.Sprint(l), gold(cfg.Memorize.GoldPerLevel * l), duration(cfg.Memorize.MinutesPerLevel * l)})
		}
		table(&b, []string{"Spell level", "Gold", "Time"}, rows)
	}
	return b.String(), nil
}

// ── 3. Learning spells ───────────────────────────────────────────────────────

// guideWholeList is the spells_known value meaning "the whole class list".
const guideWholeList = 99

func (c *guideCtx) guideLearningPage() (string, error) {
	var prog map[string]struct {
		SpellsKnown []int `json:"spells_known"`
	}
	if err := c.guideRead("magic/spell-progression.json", &prog); err != nil {
		return "", err
	}
	slots := map[string]map[int]map[string]int{} // class → level → slot type → count
	for class, raw := range c.chars.Slots {
		var byLevel map[string]map[string]int
		if json.Unmarshal(raw, &byLevel) != nil {
			continue // the description
		}
		m := map[int]map[string]int{}
		for l, s := range byLevel {
			if n, err := strconv.Atoi(l); err == nil {
				m[n] = s
			}
		}
		slots[strings.ToLower(class)] = m
	}
	maxSpell := func(class string, level int) int {
		best := 0
		for k, n := range slots[class][level] {
			if l, err := strconv.Atoi(strings.TrimPrefix(k, "level_")); err == nil && n > 0 && l > best {
				best = l
			}
		}
		return best
	}

	var b strings.Builder
	b.WriteString("What spells you know depends on your class. Spells of a level you have no slots for yet " +
		"wait until you level up.\n\n")

	// Starting spells
	b.WriteString("## Starting spells\n\nEvery caster starts knowing their class's whole starting list. " +
		"You can prepare a spell once you have slots of its level.\n\n")
	var rows [][]string
	for _, class := range sortedKeys(c.chars.Spells) {
		sp := c.chars.Spells[class]
		var cantrips, spells []string
		for _, tier := range sortedKeys(sp) {
			for _, id := range sp[tier] {
				if tier == "cantrips" {
					cantrips = append(cantrips, c.spellLink(id))
				} else {
					spells = append(spells, c.spellLink(id))
				}
			}
		}
		rows = append(rows, []string{classLink(class), strings.Join(cantrips, ", "), strings.Join(spells, ", ")})
	}
	table(&b, []string{"Class", "Cantrips", "Spells"}, rows)

	// Whole-list casters vs pickers
	var whole, pickers []string
	for _, class := range sortedKeys(prog) {
		sk := prog[class].SpellsKnown
		if len(sk) == 0 {
			continue
		}
		if sk[0] >= guideWholeList {
			whole = append(whole, class)
		} else if sk[len(sk)-1] > sk[0] {
			pickers = append(pickers, class)
		}
	}
	if len(whole) > 0 {
		var links []string
		for _, cl := range whole {
			links = append(links, classLink(cl))
		}
		b.WriteString("## Casters who know their whole list\n\n")
		b.WriteString(guideJoinAnd(links) + " know every spell on their class list of each spell level they have slots for. " +
			"When you gain slots of a new spell level, you know all of that level's class spells at once. There is nothing to pick.\n\n")
	}
	if len(pickers) > 0 {
		var links []string
		for _, cl := range pickers {
			links = append(links, classLink(cl))
		}
		b.WriteString("## Casters who pick their spells\n\n")
		b.WriteString(guideJoinAnd(links) + " learn new spells by picking them as they level up. " +
			"Each pick is a spell from your class list of a level you have slots for (not a cantrip). " +
			"The table shows how many new spells each level up brings.\n\n")
		head := []string{"Level"}
		for _, cl := range pickers {
			head = append(head, classLink(cl))
		}
		rows = nil
		maxLen := 0
		for _, cl := range pickers {
			maxLen = max(maxLen, len(prog[cl].SpellsKnown))
		}
		for l := 2; l <= maxLen; l++ {
			row := []string{fmt.Sprint(l)}
			gained := false
			for _, cl := range pickers {
				sk := prog[cl].SpellsKnown
				v := ""
				if l <= len(sk) {
					if d := sk[l-1] - sk[l-2]; d > 0 {
						v = "+" + fmt.Sprint(d)
						gained = true
					}
				}
				row = append(row, v)
			}
			if gained {
				rows = append(rows, row)
			}
		}
		table(&b, head, rows)
		if c.guideHasMemorize() {
			b.WriteString("Wizards can also learn spells from scrolls: see [[Commit to memory|/Rules/Scribing]].\n\n")
		}
	}

	// Slots by level
	var casters []string
	for _, class := range sortedKeys(slots) {
		for l := range slots[class] {
			if maxSpell(class, l) > 0 {
				casters = append(casters, class)
				break
			}
		}
	}
	if len(casters) > 0 {
		b.WriteString("## Highest spell level by character level\n\n" +
			"Your spell slots grow as you level. The table shows the highest spell level you have slots for; " +
			"each class page has the full slot counts.\n\n")
		head := []string{"Level"}
		maxLevel := 0
		for _, cl := range casters {
			head = append(head, classLink(cl))
			for l := range slots[cl] {
				maxLevel = max(maxLevel, l)
			}
		}
		rows = nil
		for l := 1; l <= maxLevel; l++ {
			row := []string{fmt.Sprint(l)}
			for _, cl := range casters {
				v := ""
				if n := maxSpell(cl, l); n > 0 {
					v = fmt.Sprint(n)
				}
				row = append(row, v)
			}
			rows = append(rows, row)
		}
		table(&b, head, rows)
	}
	return b.String(), nil
}

func (c *guideCtx) guideHasMemorize() bool {
	var cfg guideScribing
	return c.guideRead("magic/scroll-scribing.json", &cfg) == nil && len(cfg.Memorize.Classes) > 0
}

// ── 4. Hunger, fatigue & carrying ────────────────────────────────────────────

// guideHungerTickers are the effects that lower hunger at each level, by level
// (cmd/server/game/status/hunger.go). Level 0 has none: it doesn't drop further.
var guideHungerTickers = map[int]string{
	3: "hunger-accumulation-stuffed",
	2: "hunger-accumulation-wellfed",
	1: "hunger-accumulation-hungry",
}

func (c *guideCtx) guideSurvivalPage() (string, error) {
	var b strings.Builder
	b.WriteString("Your character gets hungry and tired as time passes, and slows down when they carry too much.\n\n")

	// Hunger
	b.WriteString("## Hunger\n\n")
	hungerName := func(l int) string {
		if e := c.guideStatusAt("hunger", l); e != nil {
			return e.Name
		}
		if e, ok := c.effects[guideHungerTickers[l]]; ok {
			n := e.Name
			if i := strings.Index(n, "("); i >= 0 {
				n = strings.TrimSuffix(n[i+1:], ")")
			}
			return n
		}
		return fmt.Sprint(l)
	}
	var rows [][]string
	for l := 3; l >= 0; l-- {
		drop := "Doesn't drop further"
		if e, ok := c.effects[guideHungerTickers[l]]; ok {
			for _, m := range e.Modifiers {
				if m.Stat == "hunger" && m.TickInterval > 0 {
					drop = "E" + strings.TrimPrefix(guideEvery(m.TickInterval), "e")
				}
			}
		}
		effect := ""
		if e := c.guideStatusAt("hunger", l); e != nil {
			effect = guideMods(e)
		}
		if effect == "" {
			effect = "None"
		}
		rows = append(rows, []string{fmt.Sprint(l), "**" + hungerName(l) + "**", effect, drop})
	}
	b.WriteString("Hunger runs from 3 (full) down to 0. It drops by one step at a time as time passes.\n\n")
	table(&b, []string{"Level", "Status", "Effect", "Drops a step"}, rows)
	if st := c.guideStatusAt("hunger", 0); st != nil {
		b.WriteString(fmt.Sprintf("While %s you keep losing HP until you eat. If it kills you, you wake in your home city as if you'd lost a fight (see [[Combat|/Rules/Combat]]).\n\n", strings.ToLower(st.Name)))
	}

	// Food
	b.WriteString("### Food\n\n")
	rows = nil
	ids := sortedKeys(c.items)
	sort.SliceStable(ids, func(i, j int) bool { return c.items[ids[i]].Name < c.items[ids[j]].Name })
	for _, id := range ids {
		it := c.items[id]
		hunger := 0
		var also []string
		for _, e := range it.Effects {
			switch {
			case e.Type == "hunger":
				hunger += e.Value
			case e.Type == "fatigue" && e.Value < 0:
				also = append(also, fmt.Sprintf("eases fatigue by %d", -e.Value))
			case e.Type == "fatigue" && e.Value > 0:
				also = append(also, fmt.Sprintf("adds %d fatigue", e.Value))
			case e.ApplyEffect != "":
				ch := ""
				if e.Chance > 0 && e.Chance < 100 {
					ch = fmt.Sprintf("%d%% chance: ", e.Chance)
				}
				also = append(also, ch+c.guideEffectName(e.ApplyEffect))
			}
		}
		if hunger == 0 && it.Type != "Food" {
			continue
		}
		if it.Heal != "" {
			also = append([]string{"heals " + it.Heal + " HP"}, also...)
		}
		h := ""
		if hunger > 0 {
			h = "+" + plural(hunger, "step")
		}
		rows = append(rows, []string{c.itemLink(id), h, strings.Join(also, "; ")})
	}
	table(&b, []string{"Food", "Hunger", "Also"}, rows)
	b.WriteString("A night at an inn includes breakfast: you wake " + hungerName(2) + ". A night in the wild leaves you one step hungrier.\n\n")

	// Fatigue
	b.WriteString("## Fatigue\n\n")
	acc := c.effects["fatigue-accumulation"]
	if acc != nil {
		for _, m := range acc.Modifiers {
			if m.Stat == "fatigue" && m.TickInterval > 0 {
				b.WriteString(fmt.Sprintf("Fatigue runs from 0 (fresh) to 10. It rises by %d %s as time passes, in town and on the road. ",
					m.Value, guideEvery(m.TickInterval)))
			}
		}
		if s := acc.SkillScaling; s != nil && s.BonusPerLevel > 0 {
			b.WriteString(fmt.Sprintf("Every point of %s above %d makes it build %s slower (up to %d points).",
				titleCase(s.Skill), s.BaseLevel, guidePercent(s.BonusPerLevel), s.MaxBonusLevels))
		}
		b.WriteString("\n\n")
	}
	rows = nil
	for _, band := range c.guideBands("fatigue", 0, 10) {
		name, eff := "Rested", "None"
		if band.effect != nil {
			name, eff = band.effect.Name, guideMods(band.effect)
		}
		rows = append(rows, []string{guideRange(band.from, band.to), "**" + name + "**", eff})
	}
	table(&b, []string{"Fatigue", "Status", "Effect"}, rows)

	var easers []string
	for _, id := range ids {
		for _, e := range c.items[id].Effects {
			if e.Type == "fatigue" && e.Value < 0 {
				easers = append(easers, fmt.Sprintf("%s (−%d)", c.itemLink(id), -e.Value))
			}
		}
	}
	for _, s := range c.spells {
		if e, ok := c.effects[s.AppliesEffect]; ok {
			for _, m := range e.Modifiers {
				if m.Stat == "fatigue" && m.Type == "instant" && m.Value < 0 {
					easers = append(easers, fmt.Sprintf("the %s spell (−%d)", c.spellLink(s.ID), -m.Value))
				}
			}
		}
	}
	lines := []string{
		"Fatigue stops building while you wait, while you sleep, and while you're stopped on the road.",
		"Sleeping lowers it. You can sleep from 9 PM and you wake at 6 AM.",
		"A bed at an inn is best: a full night clears all fatigue. Camping with a " + c.itemLink("bedroll") + " clears most of it, and sleeping on the bare ground about half.",
		"Waiting and sleeping also bring back HP and mana: a full 8 hours restores all of it.",
	}
	if len(easers) > 0 {
		lines = append(lines, "Also lowers fatigue: "+strings.Join(easers, ", ")+".")
	}
	bullets(&b, "", lines)

	if inns := c.guideInns(); len(inns) > 0 {
		b.WriteString("### Inns\n\nRent a room from the innkeeper, go to it, and sleep after 9 PM.\n\n")
		table(&b, []string{"Innkeeper", "Town", "Room"}, inns)
	}

	// Carrying
	b.WriteString("## Carrying\n\n")
	var enc struct {
		System struct {
			Calc struct {
				Multiplier float64 `json:"base_weight_multiplier"`
			} `json:"weight_calculation"`
		} `json:"encumbrance_system"`
	}
	if err := c.guideRead("systems/encumbrance.json", &enc); err != nil {
		return "", err
	}
	b.WriteString(fmt.Sprintf("You can carry %g lb for every point of Strength", enc.System.Calc.Multiplier))
	var packs []string
	for _, id := range ids {
		it := c.items[id]
		bonus := 0
		for _, eid := range it.EffectsWhenWorn {
			if e, ok := c.effects[eid]; ok {
				for _, m := range e.Modifiers {
					if m.Stat == "weight_capacity" && m.Type == "constant" {
						bonus += m.Value
					}
				}
			}
		}
		if bonus > 0 {
			packs = append(packs, fmt.Sprintf("%s +%d lb", c.itemLink(id), bonus))
		}
	}
	if len(packs) > 0 {
		b.WriteString(", plus what your gear adds while worn (" + strings.Join(packs, ", ") + ")")
	}
	b.WriteString(". Everything you carry counts: your packs, your gear and what's in your bags.\n\n")
	rows = nil
	bands := c.guideBands("weight_percent", 0, 250)
	for i, band := range bands {
		load := guideRange(band.from, band.to) + "%"
		if i == len(bands)-1 {
			load = fmt.Sprintf("Over %d%%", band.from-1)
		} else if band.from == 0 {
			load = fmt.Sprintf("Up to %d%%", band.to)
		}
		name, eff := "Normal", "None"
		if band.effect != nil {
			name, eff = band.effect.Name, guideMods(band.effect)
		}
		rows = append(rows, []string{load, "**" + name + "**", eff})
	}
	table(&b, []string{"Load (of capacity)", "Status", "Effect"}, rows)
	return b.String(), nil
}

// guideInns lists innkeepers who rent rooms, with the price their dialogue asks.
func (c *guideCtx) guideInns() [][]string {
	var rows [][]string
	sort.Slice(c.npcs, func(i, j int) bool { return c.npcs[i].ID < c.npcs[j].ID })
	for _, n := range c.npcs {
		price := 0
		for _, k := range sortedKeys(n.Dialogue) {
			var node struct {
				Action string          `json:"action"`
				Cost   json.RawMessage `json:"cost"`
			}
			if json.Unmarshal(n.Dialogue[k], &node) != nil || node.Action != "rent_room" {
				continue
			}
			var v int
			if json.Unmarshal(node.Cost, &v) == nil && v > 0 {
				price = v
			}
		}
		if price > 0 {
			rows = append(rows, []string{c.npcLink(n.ID), c.homeLink(n.PrimaryHome), gold(price) + " a night"})
		}
	}
	return rows
}

// ── 5. Combat ────────────────────────────────────────────────────────────────

type guideTerrain struct {
	Types map[string]struct {
		Name           string `json:"name"`
		MoveCost       int    `json:"move_cost"`
		ClimbCost      int    `json:"climb_cost"`
		BlocksMovement bool   `json:"blocks_movement"`
		BlocksSight    bool   `json:"blocks_sight"`
		CoverAC        int    `json:"cover_ac"`
		RangedBonus    int    `json:"ranged_bonus"`
		Swim           bool   `json:"swim"`
		Damage         string `json:"damage"`
		DamageType     string `json:"damage_type"`
		Trap           *struct {
			Save       string `json:"save"`
			DC         int    `json:"dc"`
			Damage     string `json:"damage"`
			DamageType string `json:"damage_type"`
			Condition  string `json:"condition"`
			Stop       bool   `json:"stop"`
		} `json:"trap"`
	} `json:"types"`
	Battlefields map[string]struct {
		Name          string `json:"name"`
		StartingRange [2]int `json:"starting_range"`
		Look          map[string]struct {
			Name string `json:"name"`
		} `json:"look"`
	} `json:"battlefields"`
}

func (c *guideCtx) guideCombatPage() (string, error) {
	var ter guideTerrain
	if err := c.guideRead("systems/combat-terrain.json", &ter); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Fights are turn by turn on a grid. Each square is 5 feet. On your turn you can move and take one action, " +
		"and sometimes a bonus action.\n\n")

	b.WriteString("## Your turn\n\n")
	bullets(&b, "", []string{
		"**Move** up to your speed. Rough ground and water cost extra. Walking away from a foe next to you lets it make a free attack.",
		"**Attack** with the weapon in your main hand, or **throw** a thrown weapon. With a light weapon in each hand you can make an extra off-hand attack as your bonus action.",
		"**Cast** a prepared spell (it costs mana and any components).",
		"**Use an item**: drink a potion, throw a flask, read a scroll, coat your blade. Items in your general slots or a pouch are at hand; digging one out of your backpack also takes your bonus action.",
		"**Class abilities** use your class's resource. See [[Class Abilities|/Rules/Class-Abilities]].",
		"**Hold**: brace instead of moving. If a foe steps into your reach, you strike first, with advantage.",
		"**Disengage**: move away this turn without giving anyone a free attack.",
		"**Flee**: try to escape. Every foe must be at least 3 squares away. The farther they are the better your chance; fast pursuers make it harder.",
		"**Help** (in a party): get a fallen ally back up at 1 HP, or give a standing ally advantage on their next attack.",
	})
	b.WriteString("If you come across monsters that haven't noticed you, you can sneak away, sneak attack (you go first, they lose their first turn, and your first attacks have advantage), or walk up to them.\n\n")

	b.WriteString("## Hitting\n\n")
	bullets(&b, "", []string{
		"Roll a d20 and add your attack bonus (your ability modifier, plus your proficiency bonus if your class knows the weapon). Equal or beat the target's Armor Class to hit.",
		"Melee weapons use Strength, ranged weapons Dexterity, and finesse weapons whichever is higher.",
		"A natural 20 always hits and is a critical hit: the damage dice are doubled. A natural 1 always misses.",
		"With **advantage** you roll two d20s and keep the higher; with **disadvantage**, the lower. Several sources don't stack: if you have both, they cancel out.",
	})
	b.WriteString("### Advantage and disadvantage\n\n")
	table(&b, []string{"You get", "When"}, [][]string{
		{"Advantage", "The target is restrained, blinded, stunned, paralyzed or unconscious"},
		{"Advantage", "An ally stands on the other side of the target (flanking, melee only)"},
		{"Advantage", "An ally used Help on you, or it's the first turn of your sneak attack"},
		{"Disadvantage", "You shoot or throw at a foe right next to you"},
		{"Disadvantage", "The target is beyond your weapon's normal range (but within its long range)"},
		{"Disadvantage", "It's night, you're outdoors, and you have no darkvision and no light in hand (see [[Day & Night|/Rules/Day-and-Night]])"},
		{"Disadvantage", "You're poisoned, frightened, prone, restrained or blinded"},
		{"Disadvantage", "You swing a heavy weapon as a Halfling or Gnome"},
		{"Disadvantage", "You fight in melee from deep water"},
	})
	b.WriteString("Monsters follow the same rules, and some, like those with Sunlight Sensitivity, are hampered by daylight.\n\n")

	// Terrain
	b.WriteString("## Terrain\n\nEvery battlefield is scattered with terrain that depends on where the fight breaks out.\n\n")
	var rows [][]string
	typeIDs := sortedKeys(ter.Types)
	sort.SliceStable(typeIDs, func(i, j int) bool { return ter.Types[typeIDs[i]].Name < ter.Types[typeIDs[j]].Name })
	for _, id := range typeIDs {
		t := ter.Types[id]
		move := ""
		switch {
		case t.BlocksMovement:
			move = "Can't enter"
		case t.MoveCost > 0:
			move = plural(t.MoveCost, "square")
		}
		var notes []string
		if t.CoverAC > 0 {
			notes = append(notes, fmt.Sprintf("+%d AC to anyone behind it", t.CoverAC))
		}
		if t.BlocksSight {
			notes = append(notes, "blocks line of sight")
		}
		if t.ClimbCost > 0 {
			notes = append(notes, fmt.Sprintf("climbing up costs %s more", plural(t.ClimbCost, "square")))
		}
		if t.RangedBonus > 0 {
			notes = append(notes, fmt.Sprintf("+%d to hit when shooting down from it", t.RangedBonus))
		}
		if t.Swim {
			notes = append(notes, "you fight at disadvantage in melee while in it")
		}
		if t.Damage != "" {
			notes = append(notes, fmt.Sprintf("%s %s damage when you enter or end your turn in it", t.Damage, t.DamageType))
		}
		if t.Trap != nil {
			notes = append(notes, "a trap, see below")
		}
		rows = append(rows, []string{"**" + t.Name + "**", move, strings.Join(notes, "; ")})
	}
	table(&b, []string{"Terrain", "Move cost", "Notes"}, rows)

	rows = nil
	bfIDs := sortedKeys(ter.Battlefields)
	sort.SliceStable(bfIDs, func(i, j int) bool { return ter.Battlefields[bfIDs[i]].Name < ter.Battlefields[bfIDs[j]].Name })
	for _, id := range bfIDs {
		bf := ter.Battlefields[id]
		var looks []string
		for _, k := range sortedKeys(bf.Look) {
			if t, ok := ter.Types[k]; ok {
				looks = append(looks, bf.Look[k].Name+" ("+strings.ToLower(t.Name)+")")
			}
		}
		rows = append(rows, []string{bf.Name, guideRange(bf.StartingRange[0], bf.StartingRange[1]), strings.Join(looks, ", ")})
	}
	b.WriteString("### Battlefields\n\nHow far apart the two sides start, in squares, and what the terrain looks like there.\n\n")
	table(&b, []string{"Battlefield", "Starting distance", "Terrain"}, rows)

	// Traps
	ids := sortedKeys(c.items)
	sort.SliceStable(ids, func(i, j int) bool { return c.items[ids[i]].Name < c.items[ids[j]].Name })
	rows = nil
	for _, id := range ids {
		it := c.items[id]
		if it.Scatter == nil {
			continue
		}
		t, ok := ter.Types[it.Scatter.Terrain]
		if !ok || t.Trap == nil {
			continue
		}
		var eff []string
		if t.Trap.Damage != "" {
			eff = append(eff, t.Trap.Damage+" "+t.Trap.DamageType+" damage")
		}
		if t.Trap.Condition != "" {
			eff = append(eff, "knocked "+t.Trap.Condition)
		}
		if t.Trap.Stop {
			eff = append(eff, "stops moving")
		}
		rows = append(rows, []string{c.itemLink(id), plural(it.Scatter.Cells, "square"),
			fmt.Sprintf("DC %d %s", t.Trap.DC, titleCase(t.Trap.Save)), strings.Join(eff, ", ")})
	}
	if len(rows) > 0 {
		b.WriteString("## Traps\n\nScatter them on the ground between you and a foe (your action). Anything that steps on them, you included, " +
			"makes a saving throw or suffers the effect. Flying creatures pass over.\n\n")
		table(&b, []string{"Item", "Covers", "Save", "On a failed save"}, rows)
	}

	// Throwables
	rows = nil
	for _, id := range ids {
		it := c.items[id]
		if it.Throw == nil {
			continue
		}
		rng := num(it.Range)
		if l := num(it.RangeL); l != "" && l != rng {
			rng += "/" + l
		}
		dmg := ""
		if it.Throw.Damage != "" && it.Throw.Damage != "0" {
			dmg = it.Throw.Damage + " " + it.Throw.DamageType
		}
		cond := ""
		if k := it.Throw.Condition; k != nil && k.Condition != "" {
			dc := it.Throw.DC
			if dc <= 0 {
				dc = 10
			}
			cond = titleCase(k.Condition)
			if k.DamageDice != "" {
				cond += fmt.Sprintf(" (%s %s each turn)", k.DamageDice, k.DamageType)
			}
			if k.SaveEachTurn != "" {
				cond += fmt.Sprintf(" until it makes a DC %d %s save", dc, titleCase(k.SaveEachTurn))
			}
		}
		rows = append(rows, []string{c.itemLink(id), rng, dmg, cond})
	}
	if len(rows) > 0 {
		b.WriteString("## Throwing flasks and nets\n\nA ranged attack with your Dexterity modifier (no proficiency bonus). " +
			"It takes your action, and the item is used up whether it hits or not.\n\n")
		table(&b, []string{"Item", "Range (squares)", "Damage", "On a hit"}, rows)
	}

	// Coatings
	rows = nil
	for _, id := range ids {
		it := c.items[id]
		if !it.hasTag("coating") {
			continue
		}
		for _, e := range it.Effects {
			ef, ok := c.effects[e.ApplyEffect]
			if !ok || ef.CombatBuff == nil {
				continue
			}
			cb := ef.CombatBuff
			var what []string
			if cb.HitDice != "" {
				w := fmt.Sprintf("+%s %s on", cb.HitDice, cb.HitType)
				if cb.Hits > 0 {
					w += fmt.Sprintf(" your first %d weapon hits", cb.Hits)
				} else {
					w += " every weapon hit"
				}
				what = append(what, w)
			}
			if cb.HitBonus > 0 {
				what = append(what, fmt.Sprintf("+%d damage on every weapon hit", cb.HitBonus))
			}
			used := "Yes"
			if it.Reusable {
				used = "No"
			}
			rows = append(rows, []string{c.itemLink(id), strings.Join(what, ", ") + " in your next fight", duration(ef.Removal.Timer), used})
		}
	}
	if len(rows) > 0 {
		b.WriteString("## Weapon coatings\n\nUse one before a fight, or during one (that takes your action). " +
			"It lasts until your next fight ends, or wears off if no fight comes in time.\n\n")
		table(&b, []string{"Item", "Effect", "Wears off after", "Used up"}, rows)
	}

	// Conditions
	b.WriteString("## Conditions\n\nSome attacks and spells leave a condition that lasts a few turns, or until the creature makes a saving throw at the end of its turn.\n\n")
	table(&b, []string{"Condition", "Effect"}, [][]string{
		{"Blinded", "Its attacks have disadvantage; attacks against it have advantage"},
		{"Poisoned", "Its attacks have disadvantage"},
		{"Frightened", "Its attacks have disadvantage"},
		{"Prone", "Its attacks have disadvantage"},
		{"Restrained", "Can't move; its attacks have disadvantage; attacks against it have advantage"},
		{"Grappled", "Can't move"},
		{"Stunned, Paralyzed", "Can't act; attacks against it have advantage"},
		{"Unconscious", "Can't act or move; attacks against it have advantage"},
	})

	// Defeat
	b.WriteString("## Falling and defeat\n\n")
	bullets(&b, "", []string{
		"At 0 HP you fall unconscious and roll a death saving throw each turn: 10 or higher is a success, lower is a failure.",
		"A natural 20 gets you back up at 1 HP. A natural 1 counts as two failures. Being hit while down counts as a failure (two for a critical hit).",
		"Three successes and you get back up at 1 HP. Three failures and you are defeated.",
		"In a party, a fallen member waits for an ally to help or heal them. The party only loses when everyone is down.",
		"When you're defeated you keep the XP from the fight and wake in your home city with full HP and mana.",
		"You keep only the **3 most valuable items** you carried; everything else is lost. Each gold counts as one item, so carried gold is nearly always lost. What's in your vault is safe.",
	})
	b.WriteString("Fighting at night pays more XP, but it's dark: see [[Day & Night|/Rules/Day-and-Night]].\n\n")
	return b.String(), nil
}

// ── 6. Day & night ───────────────────────────────────────────────────────────

type guideTimeConfig struct {
	IdleAutoPause    int      `json:"idle_auto_pause_seconds"`
	TownTimeScale    float64  `json:"town_time_scale"`
	PauseOnEffects   []string `json:"pause_on_effects"`
	NightEncounters  float64  `json:"night_encounter_multiplier"`
	NightMonsterTime [2]int   `json:"night_monster_hours"`
	DayMonsterTime   [2]int   `json:"day_monster_hours"`
	DarkvisionRaces  []string `json:"darkvision_races"`
	SunlightHours    [2]int   `json:"sunlight_hours"`
}

func guideWindow(w [2]int) string { return clock(w[0]) + " to " + clock(w[1]) }

func (c *guideCtx) guideDayNightPage() (string, error) {
	var cfg guideTimeConfig
	if err := c.guideRead("systems/time-config.json", &cfg); err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("The world runs on a clock. Shops open and close, people go home to sleep, and the night brings out different monsters.\n\n")

	b.WriteString("## The clock\n\n")
	roadDay := 1440 / guideRoadClockSpeed
	lines := []string{fmt.Sprintf("On the road a whole in-game day passes in about %s of real time.", plural(roadDay, "minute"))}
	if cfg.TownTimeScale > 0 && cfg.TownTimeScale != 1 {
		lines = append(lines, fmt.Sprintf("In town the clock runs at %s of that pace: a day takes about %s.",
			guidePercent(cfg.TownTimeScale), plural(int(float64(roadDay)/cfg.TownTimeScale+0.5), "minute")))
	}
	lines = append(lines, "The clock stops while you fight, deal with an encounter, shop, use your vault, talk to someone or explore a place of interest.")
	if len(cfg.PauseOnEffects) > 0 {
		var names []string
		for _, id := range cfg.PauseOnEffects {
			names = append(names, c.guideEffectName(id))
		}
		lines = append(lines, "It also pauses the moment you become "+guideJoinOr(names)+", so you notice.")
	}
	if cfg.IdleAutoPause > 0 {
		lines = append(lines, fmt.Sprintf("If you leave the game alone for %s it pauses itself.", duration(cfg.IdleAutoPause/60)))
	}
	bullets(&b, "", lines)

	b.WriteString("## Night\n\n")
	b.WriteString(fmt.Sprintf("Night is from %s to %s. At night:\n\n", clock(types.NightStart), clock(types.NightEnd)))
	night := []string{"Fights pay a quarter more XP."}
	if cfg.NightEncounters > 1 {
		night = append(night, fmt.Sprintf("Fights on the road are %g times as likely.", cfg.NightEncounters))
	}
	night = append(night, "Outdoors it's dark: your attacks have disadvantage unless you have darkvision or hold a light. Towns are lit.")
	bullets(&b, "", night)

	if len(cfg.DarkvisionRaces) > 0 {
		b.WriteString("**Darkvision:** " + strings.Join(cfg.DarkvisionRaces, ", ") + ".\n\n")
	}
	var rows [][]string
	ids := sortedKeys(c.items)
	sort.SliceStable(ids, func(i, j int) bool { return c.items[ids[i]].Name < c.items[ids[j]].Name })
	for _, id := range ids {
		it := c.items[id]
		if !it.hasTag("light-source") {
			continue
		}
		where := ""
		switch it.GearSlot {
		case "mainhand", "offhand", "hands":
			where = "Held in a hand"
		case "neck":
			where = "Worn at the neck"
		default:
			continue // only lights in hand or at the neck count
		}
		rows = append(rows, []string{c.itemLink(id), where})
	}
	if len(rows) > 0 {
		b.WriteString("### Lights\n\nAny of these, equipped, lets you see in the dark:\n\n")
		table(&b, []string{"Light", "Where"}, rows)
	}

	// Monsters by time
	var nightMs, dayMs, sun []string
	anyTime := 0
	sort.Slice(c.monsters, func(i, j int) bool { return c.monsters[i].Name < c.monsters[j].Name })
	for _, m := range c.monsters {
		l := c.monsterLink(m.ID)
		switch m.Active {
		case "night":
			nightMs = append(nightMs, l)
		case "day":
			dayMs = append(dayMs, l)
		default:
			anyTime++
		}
		for _, s := range m.Special {
			if strings.EqualFold(s.Name, "Sunlight Sensitivity") {
				sun = append(sun, l)
				break
			}
		}
	}
	b.WriteString("## Monsters by the hour\n\n")
	b.WriteString(fmt.Sprintf("Most creatures (%d) roam at any hour. The rest keep to the night or the day; the two windows overlap at dusk and dawn.\n\n", anyTime))
	rows = nil
	if len(nightMs) > 0 {
		rows = append(rows, []string{"Night", guideWindow(cfg.NightMonsterTime), strings.Join(nightMs, ", ")})
	}
	if len(dayMs) > 0 {
		rows = append(rows, []string{"Day", guideWindow(cfg.DayMonsterTime), strings.Join(dayMs, ", ")})
	}
	table(&b, []string{"Active", "Hours", "Monsters"}, rows)
	if len(sun) > 0 {
		b.WriteString(fmt.Sprintf("### Sunlight Sensitivity\n\nOutdoors from %s, these creatures attack at disadvantage: %s.\n\n",
			guideWindow(cfg.SunlightHours), strings.Join(sun, ", ")))
	}

	b.WriteString("## Opening hours\n\n")
	var towns []string
	for _, id := range sortedKeys(c.gen.cities) {
		towns = append(towns, c.homeLink(id))
	}
	bullets(&b, "", []string{
		"Shops, temples and other buildings keep opening hours, and the people in them follow daily schedules. Shopkeepers go home at night.",
		"Each town's page lists its buildings' hours: " + strings.Join(towns, ", ") + ".",
		"You can sleep from 9 PM; see [[Hunger, Fatigue & Carrying|/Rules/Survival]].",
	})
	return b.String(), nil
}

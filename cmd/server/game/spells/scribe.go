package spells

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"sync"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/gameutil"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/spellbook"
	"pubkey-quest/types"
)

// ─── Scroll scribing (docs/draft/spell-scroll-system.md) ────────────────────
//
// A caster scribes a scroll of a spell they know (spellbook.Knows) of a level
// they have slots for, at a station of their class's tradition. The recipe is
// the spell itself: parchment + carried ink and pen + its material component +
// gold and time by spell level (scroll-scribing.json). The roll uses the
// class's casting ability. Nothing is stored beyond the scroll it makes.

// ScribingTable mirrors game-data/magic/scroll-scribing.json.
type ScribingTable struct {
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
	Carried    []string `json:"carried"`
	Memorize   struct {
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

var (
	scribeMu  sync.Mutex
	scribeTbl *ScribingTable
	// scribeRoll is the d100 for a scribing attempt (1–100); tests replace it.
	scribeRoll = func() int { return rand.Intn(100) + 1 }
)

// scribingTable reads scroll-scribing.json from the database once it's open
// (not cached until then).
func scribingTable(db *sql.DB) *ScribingTable {
	scribeMu.Lock()
	defer scribeMu.Unlock()
	if scribeTbl != nil {
		return scribeTbl
	}
	t := &ScribingTable{}
	var blob string
	if db == nil || db.QueryRow("SELECT data FROM scroll_scribing WHERE id = 'scroll-scribing'").Scan(&blob) != nil {
		return t
	}
	json.Unmarshal([]byte(blob), t)
	scribeTbl = t
	return t
}

// ScrollItemID is the item id of a spell's scroll (generated at migrate when
// no hand-written one exists).
func ScrollItemID(spellID string) string { return "scroll-of-" + spellID }

// Tradition is the scribing station kind a class uses ("scribe_divine"), or "".
func Tradition(db *sql.DB, class string) string {
	for id, t := range scribingTable(db).Traditions {
		for _, c := range t.Classes {
			if strings.EqualFold(c, class) {
				return id
			}
		}
	}
	return ""
}

// ScribeOption is one spell a caster could scribe here, with what it costs.
type ScribeOption struct {
	SpellID   string   `json:"spell_id"`
	Name      string   `json:"name"`
	Level     int      `json:"level"`
	Gold      int      `json:"gold"`
	Minutes   int      `json:"minutes"`
	Chance    int      `json:"chance"`            // percent
	Needs     []string `json:"needs,omitempty"`   // items consumed ("1 × parchment", "1 × quartz-dust")
	Missing   []string `json:"missing,omitempty"` // what stops it right now
	CanScribe bool     `json:"can_scribe"`
}

// ScribeStation describes the station the caster is standing at.
type ScribeStation struct {
	Here      bool           `json:"here"`      // a station of the caster's tradition is here
	Tradition string         `json:"tradition"` // the caster's tradition name ("divine")
	Where     string         `json:"where"`     // where the caster's tradition scribes ("a temple or shrine")
	Ability   string         `json:"ability"`   // casting ability the roll uses
	Options   []ScribeOption `json:"options"`
}

// StationHere reports whether the caster stands in a building with a station
// of their tradition.
func StationHere(db *sql.DB, save *types.SaveFile) bool {
	tr := Tradition(db, save.Class)
	if tr == "" || save.Building == "" {
		return false
	}
	for _, s := range building.Stations(db, save.Location, save.Building) {
		if s == tr {
			return true
		}
	}
	return false
}

// Scribable lists every spell the caster could scribe (known, of a level they
// have slots for, up to the table's max), cheapest first, each with its cost,
// chance and what's missing. Here is false away from a station of their
// tradition (options still list, all blocked), so the panel can explain.
func Scribable(db *sql.DB, save *types.SaveFile) (*ScribeStation, error) {
	tbl := scribingTable(db)
	trID := Tradition(db, save.Class)
	if trID == "" {
		return nil, fmt.Errorf("only spellcasters can scribe scrolls")
	}
	tr := tbl.Traditions[trID]
	ability, _ := character.SpellcastingAbility(save.Class)
	out := &ScribeStation{Here: StationHere(db, save), Tradition: tr.Name, Where: tr.Station, Ability: ability}

	maxLvl := spellbook.MaxSpellLevel(save.Class, spellbook.Level(save))
	if tbl.MaxSpellLevel < maxLvl {
		maxLvl = tbl.MaxSpellLevel
	}
	for _, id := range spellbook.Known(save) {
		spell, err := gamedata.LoadSpellByID(db, id)
		if err != nil {
			continue
		}
		lvl := intField(spell, "level")
		if lvl > maxLvl {
			continue
		}
		out.Options = append(out.Options, scribeOption(db, save, tbl, id, spell, out.Here))
	}
	sort.SliceStable(out.Options, func(i, j int) bool {
		if out.Options[i].Level != out.Options[j].Level {
			return out.Options[i].Level < out.Options[j].Level
		}
		return out.Options[i].Name < out.Options[j].Name
	})
	return out, nil
}

func scribeOption(db *sql.DB, save *types.SaveFile, tbl *ScribingTable, id string, spell map[string]interface{}, here bool) ScribeOption {
	lvl := intField(spell, "level")
	tier := tbl.Tiers[strconv.Itoa(lvl)]
	name := stringField(spell, "name")
	if name == "" {
		name = id
	}
	o := ScribeOption{SpellID: id, Name: name, Level: lvl, Gold: tier.Gold, Minutes: tier.Minutes, Chance: scribeChance(save, tbl, lvl)}
	if !here {
		o.Missing = append(o.Missing, "a "+tbl.Traditions[Tradition(db, save.Class)].Name+" station")
	}
	for _, c := range tbl.Consumes {
		o.Needs = append(o.Needs, fmt.Sprintf("%d × %s", c.Quantity, c.Item))
		if gameutil.CountItem(save, c.Item) < c.Quantity {
			o.Missing = append(o.Missing, fmt.Sprintf("%d × %s", c.Quantity, c.Item))
		}
	}
	for _, it := range tbl.Carried {
		if gameutil.CountItem(save, it) < 1 {
			o.Missing = append(o.Missing, it)
		}
	}
	focus := func(component string) bool { return equippedFocusProvides(db, save, component) }
	if plan, err := resolveComponents(save, spell, focus, false); err != nil {
		o.Missing = append(o.Missing, strings.TrimPrefix(err.Error(), "need "))
	} else {
		for _, u := range plan {
			o.Needs = append(o.Needs, fmt.Sprintf("%d × %s", u.quantity, u.component))
		}
	}
	if gameutil.GetGoldQuantity(save) < tier.Gold {
		o.Missing = append(o.Missing, fmt.Sprintf("%d gold", tier.Gold))
	}
	o.CanScribe = len(o.Missing) == 0
	return o
}

// scribeChance is the percent chance to scribe a spell of a level.
func scribeChance(save *types.SaveFile, tbl *ScribingTable, level int) int {
	ability, _ := character.SpellcastingAbility(save.Class)
	mod := character.AbilityMod(character.AbilityScore(effects.EffectiveStats(save), ability))
	c := tbl.Tiers[strconv.Itoa(level)].BaseChance + tbl.PerMod*mod
	if c < tbl.MinChance {
		c = tbl.MinChance
	}
	if tbl.MaxChance > 0 && c > tbl.MaxChance {
		c = tbl.MaxChance
	}
	return c
}

// ScribeResult is the outcome of an attempt.
type ScribeResult struct {
	Scribed bool   `json:"scribed"`
	ItemID  string `json:"item_id,omitempty"`
	Name    string `json:"name"`
	Chance  int    `json:"chance"`
	Roll    int    `json:"roll"`
	Minutes int    `json:"minutes"`
	Gold    int    `json:"gold"`
}

// Scribe attempts a scroll. Nothing is spent unless every check passes; then
// the parchment, components and gold are spent and the time passes whether or
// not the roll succeeds.
func Scribe(db *sql.DB, save *types.SaveFile, spellID string) (*ScribeResult, error) {
	st, err := Scribable(db, save)
	if err != nil {
		return nil, err
	}
	var opt *ScribeOption
	for i := range st.Options {
		if st.Options[i].SpellID == spellID {
			opt = &st.Options[i]
		}
	}
	if opt == nil {
		return nil, fmt.Errorf("you can't scribe that spell — you don't know it, or it's beyond your spell slots")
	}
	if !opt.CanScribe {
		return nil, fmt.Errorf("you need %s", strings.Join(opt.Missing, ", "))
	}
	itemID := ScrollItemID(spellID)
	if !roomFor(save, itemID) {
		return nil, fmt.Errorf("no room in your pack for the scroll")
	}

	// Spend: parchment, the spell's components, the gold.
	tbl := scribingTable(db)
	spell, _ := gamedata.LoadSpellByID(db, spellID)
	focus := func(component string) bool { return equippedFocusProvides(db, save, component) }
	plan, err := resolveComponents(save, spell, focus, false)
	if err != nil {
		return nil, err
	}
	for _, c := range tbl.Consumes {
		gameutil.ConsumeItem(save, c.Item, c.Quantity)
	}
	for _, u := range plan {
		consumeComponent(save.Inventory, u.component, u.quantity)
	}
	gameutil.DeductGold(save, opt.Gold)
	gametime.AdvanceTime(save, opt.Minutes, true)

	res := &ScribeResult{Name: opt.Name, Chance: opt.Chance, Roll: scribeRoll(), Minutes: opt.Minutes, Gold: opt.Gold}
	if res.Roll <= opt.Chance {
		if n, _ := gaminventory.AddItemToInventory(save, itemID, 1); n > 0 {
			res.Scribed, res.ItemID = true, itemID
		}
	}
	return res, nil
}

// roomFor reports whether one of the item would fit, by trying it on a copy of
// the inventory.
func roomFor(save *types.SaveFile, itemID string) bool {
	raw, err := json.Marshal(save.Inventory)
	if err != nil {
		return false
	}
	trial := &types.SaveFile{}
	if json.Unmarshal(raw, &trial.Inventory) != nil {
		return false
	}
	n, _ := gaminventory.AddItemToInventory(trial, itemID, 1)
	return n > 0
}

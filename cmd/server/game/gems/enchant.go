package gems

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"pubkey-quest/types"
)

// ─── Enchanting (docs/draft/enchanting.md) ───────────────────────────────────
//
// An enchanter turns a plain ring or necklace into its enchanted twin
// (ring-<type>-<size>-<quality>-enchanted), whose worn effect gems.json's
// enchanting table describes by gem and size. Each effect is generated here as
// an effect definition (jewel-<type>-<size>) and inserted by --migrate, like
// the items: abilities and skills are plain modifiers, ticks are periodic
// ones, and riders are a combat_buff that rides every fight (combat.wornBuffs).
// Diamonds' resistances grow with quality, so their effects are per quality;
// alexandrite follows the wearer's class, so its effects are per class.

// Enchant is a gem's enchant table entry at a size, and whether there is one.
func Enchant(c *types.GemsConfig, gemType, size string) (types.GemEnchant, bool) {
	e, ok := c.Enchanting.Effects[gemType][size]
	return e, ok
}

// Enchantable reports whether a piece can be enchanted at all: a plain piece
// whose gem has an effect at its size (no pearl is huge enough).
func Enchantable(c *types.GemsConfig, g Gem) bool {
	_, ok := Enchant(c, g.Type, g.Size)
	return g.Piece != "" && g.Cut() && !g.Enchanted && ok
}

// byQuality reports whether a gem's effect at a size differs by quality
// (diamonds' resistances).
func byQuality(c *types.GemsConfig, gemType, size string) bool {
	e, _ := Enchant(c, gemType, size)
	return len(e.ResistByQuality) > 0
}

// byClass reports whether a gem's effect follows the wearer's class.
func byClass(e types.GemEnchant) bool {
	return e.ClassAbility > 0 || e.ClassSkill > 0 || e.ClassResource
}

// EffectID is the worn effect of a gem at a size: jewel-<type>-<size>, with
// the quality for a diamond's and the class for alexandrite's.
func EffectID(c *types.GemsConfig, gemType, size, quality, class string) string {
	id := "jewel-" + gemType + "-" + size
	e, _ := Enchant(c, gemType, size)
	if byQuality(c, gemType, size) {
		id += "-" + quality
	}
	if byClass(e) {
		id += "-" + strings.ToLower(class)
	}
	return id
}

// pieceSizes is the sizes whose effects an enchanted piece wears: its own,
// and tiny too when its quality adds it (flawless).
func pieceSizes(c *types.GemsConfig, g Gem) []string {
	sizes := []string{g.Size}
	if c.Enchanting.Quality[g.Quality].AddsTiny && g.Size != "tiny" {
		if _, ok := Enchant(c, g.Type, "tiny"); ok {
			sizes = append(sizes, "tiny")
		}
	}
	return sizes
}

// pieceEffects is the effects an enchanted piece wears.
func pieceEffects(c *types.GemsConfig, g Gem, class string) []string {
	var ids []string
	for _, size := range pieceSizes(c, g) {
		ids = append(ids, EffectID(c, g.Type, size, g.Quality, class))
	}
	return ids
}

// classIDs is the classes, sorted.
func classIDs(c *types.GemsConfig) []string {
	ids := make([]string, 0, len(c.Enchanting.Classes))
	for id := range c.Enchanting.Classes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// wornFields is an enchanted piece's effects_when_worn, or for alexandrite its
// effects_when_worn_by_class.
func wornFields(c *types.GemsConfig, g Gem) map[string]any {
	e, _ := Enchant(c, g.Type, g.Size)
	tiny, _ := Enchant(c, g.Type, "tiny")
	if !byClass(e) && !byClass(tiny) {
		return map[string]any{"effects_when_worn": pieceEffects(c, g, "")}
	}
	by := map[string]any{}
	for _, class := range classIDs(c) {
		by[class] = pieceEffects(c, g, class)
	}
	return map[string]any{"effects_when_worn_by_class": by}
}

// ─── The effect definitions ──────────────────────────────────────────────────

// Effects generates every enchanted piece's worn effect definition, each an
// effect JSON object (as a game-data/effects file would be) with
// "generated": true.
func Effects(c *types.GemsConfig) []map[string]any {
	var out []map[string]any
	for _, t := range SortedTypes(c) {
		for _, s := range c.Sizes {
			e, ok := Enchant(c, t, s.ID)
			if !ok {
				continue
			}
			switch {
			case byClass(e):
				for _, class := range classIDs(c) {
					out = append(out, effectDef(c, t, s.ID, "", class, e))
				}
			case len(e.ResistByQuality) > 0:
				for _, q := range c.Qualities {
					out = append(out, effectDef(c, t, s.ID, q.ID, "", e))
				}
			default:
				out = append(out, effectDef(c, t, s.ID, "", "", e))
			}
		}
	}
	return out
}

// EffectName is a worn effect's name: a huge gem's own ("Emberheart"), else
// "Enchanted Ruby (large)".
func EffectName(c *types.GemsConfig, gemType, size, quality string) string {
	e, _ := Enchant(c, gemType, size)
	if e.Name != "" {
		return e.Name
	}
	inner := strings.ToLower(sizeOf(c, Gem{Size: size}).Name)
	if byQuality(c, gemType, size) {
		inner += ", " + strings.ToLower(qualityOf(c, Gem{Quality: quality}).Name)
	}
	return fmt.Sprintf("Enchanted %s (%s)", c.Types[gemType].Name, inner)
}

// resolved is a gem's effect at a size for one quality and class: the class's
// ability and skill filled in, and a diamond's resistances in its buff.
func resolved(c *types.GemsConfig, e types.GemEnchant, quality, class string) (mods map[string]int, buff *types.SpellBuff) {
	mods = map[string]int{}
	for k, v := range e.Mods {
		mods[k] = v
	}
	if byClass(e) {
		cl := c.Enchanting.Classes[strings.ToLower(class)]
		if e.ClassAbility > 0 && cl.Ability != "" {
			mods[cl.Ability] += e.ClassAbility
		}
		if e.ClassSkill > 0 && cl.Skill != "" {
			mods["skill_"+cl.Skill] += e.ClassSkill
		}
		if e.ClassResource {
			if cl.Resource {
				mods["resource_pct"] += 10
			} else {
				mods["fight_mana"] += 2
			}
		}
	}
	if e.CombatBuff != nil {
		b := *e.CombatBuff
		buff = &b
	}
	if r := e.ResistByQuality[quality]; len(r) > 0 {
		if buff == nil {
			buff = &types.SpellBuff{}
		}
		buff.Resist = append(append([]string{}, buff.Resist...), r...)
	}
	return mods, buff
}

func effectDef(c *types.GemsConfig, gemType, size, quality, class string, e types.GemEnchant) map[string]any {
	mods, buff := resolved(c, e, quality, class)
	keys := make([]string, 0, len(mods))
	for k := range mods {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return modOrder(keys[i]) < modOrder(keys[j]) || (modOrder(keys[i]) == modOrder(keys[j]) && keys[i] < keys[j]) })
	var modifiers []map[string]any
	for _, k := range keys {
		modifiers = append(modifiers, map[string]any{"stat": k, "type": "constant", "value": mods[k]})
	}
	for _, t := range e.Ticks {
		m := map[string]any{"stat": t.Stat, "type": "periodic", "value": 1, "tick_interval": t.Minutes}
		if t.When != "" {
			m["when"] = t.When
		}
		modifiers = append(modifiers, m)
	}
	name := EffectName(c, gemType, size, quality)
	def := map[string]any{
		"id":          EffectID(c, gemType, size, quality, class),
		"name":        name,
		"description": "While worn: " + Describe(mods, e.Ticks, buff) + ".",
		"source_type": "applied",
		"category":    "buff",
		"removal":     map[string]any{"type": "equipment"},
		"modifiers":   modifiers,
		"visible":     true,
		"generated":   true,
	}
	if buff != nil {
		// One rider per gem however many pieces carry it: the strongest
		// (the bigger gem; for diamonds, the more resistances) wins.
		buff.SpellID, buff.Name = "jewel-"+gemType, name
		def["combat_buff"] = buff
		def["rank"] = SizeIndex(c, size) + 10*len(buff.Resist)
	}
	return def
}

// ─── Words ───────────────────────────────────────────────────────────────────

var abilityShort = map[string]string{
	"strength": "STR", "dexterity": "DEX", "constitution": "CON",
	"intelligence": "INT", "wisdom": "WIS", "charisma": "CHA",
}

// modOrder puts abilities first, then skills, then the rest.
func modOrder(stat string) int {
	switch {
	case abilityShort[stat] != "":
		return 0
	case strings.HasPrefix(stat, "skill_"):
		return 1
	}
	return 2
}

func titleWord(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// every says how often a tick comes: "every 2 hours", "every 30 minutes".
func every(minutes int) string {
	switch {
	case minutes == 60:
		return "every hour"
	case minutes%60 == 0:
		return fmt.Sprintf("every %d hours", minutes/60)
	}
	return fmt.Sprintf("every %d minutes", minutes)
}

// Describe says what a worn effect does, in a list for players: "STR +1,
// +1d4 fire damage on every hit".
func Describe(mods map[string]int, ticks []types.GemTick, buff *types.SpellBuff) string {
	keys := make([]string, 0, len(mods))
	for k := range mods {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return modOrder(keys[i]) < modOrder(keys[j]) || (modOrder(keys[i]) == modOrder(keys[j]) && keys[i] < keys[j]) })
	var parts []string
	add := func(format string, a ...any) { parts = append(parts, fmt.Sprintf(format, a...)) }
	for _, k := range keys {
		v := mods[k]
		switch {
		case abilityShort[k] != "":
			add("%s %+d", abilityShort[k], v)
		case strings.HasPrefix(k, "skill_"):
			add("%s %+d", titleWord(strings.TrimPrefix(k, "skill_")), v)
		case k == "armor_class":
			add("AC %+d", v)
		case k == "darkvision":
			add("darkvision")
		case k == "travel_speed":
			add("travel %+d%%", v)
		case k == "ambush_ward":
			add("never ambushed on the road")
		case k == "weight_capacity":
			add("carry %+d lb", v)
		case k == "temp_hp":
			add("%d temporary HP at the start of every fight", v)
		case k == "combat_move":
			add("%+d move in a fight", v)
		case k == "fight_mana":
			add("%+d mana at the start of every fight", v)
		case k == "fatigue_slow":
			add("fatigue builds half as fast")
		case k == "light":
			add("sheds light like a torch that never burns down")
		case k == "heal_d6":
			add("healing spells heal %dd6 more", v)
		case k == "resource_pct":
			add("%+d%% to your class resource (Rage, Stamina, Ki, Cunning)", v)
		default:
			add("%s %+d", k, v)
		}
	}
	for _, t := range ticks {
		line := fmt.Sprintf("1 %s %s", strings.ToUpper(t.Stat), every(t.Minutes))
		if t.Stat == "mana" {
			line = fmt.Sprintf("1 mana %s", every(t.Minutes))
		}
		if t.When == "night" {
			line += " at night"
		}
		parts = append(parts, line)
	}
	if b := buff; b != nil {
		if b.AttackBonus != 0 {
			add("%+d to attack rolls", b.AttackBonus)
		}
		if b.HitDice != "" {
			line := fmt.Sprintf("+%s %s damage on every hit", b.HitDice, b.HitType)
			if b.HitSpells {
				line += ", spells too"
			}
			parts = append(parts, line)
		}
		if b.Retaliate != "" {
			add("a melee attacker that hits you takes %s %s damage", b.Retaliate, b.RetaliateType)
		}
		if len(b.Resist) > 0 {
			add("resist %s damage", listWords(b.Resist))
		}
		if len(b.Immune) > 0 {
			add("can't be %s", strings.Join(b.Immune, " or "))
		}
		if len(b.WardTypes) > 0 {
			add("%s attack you at disadvantage", listWords(b.WardTypes))
		}
		if b.SaveDice != "" {
			add("+%s to saving throws", b.SaveDice)
		}
		if b.DeathWard {
			add("once a fight, a blow that would drop you leaves you at 1 HP")
		}
		if b.Sanctuary && b.SanctuaryOnce {
			add("the first foe to attack you each fight must pass a Wisdom save (DC %d) or lose the attack", b.SaveDC)
		}
	}
	return strings.Join(parts, ", ")
}

// listWords joins words with commas and "and".
func listWords(w []string) string {
	switch len(w) {
	case 0:
		return ""
	case 1:
		return w[0]
	}
	return strings.Join(w[:len(w)-1], ", ") + " and " + w[len(w)-1]
}

// PieceDescription is what an enchanted piece does, for its notes and the
// enchanter's window: alexandrite's follows the class it's described for.
func PieceDescription(c *types.GemsConfig, g Gem, class string) string {
	var parts []string
	for _, size := range pieceSizes(c, g) {
		e, _ := Enchant(c, g.Type, size)
		mods, buff := resolved(c, e, g.Quality, class)
		parts = append(parts, Describe(mods, e.Ticks, buff))
	}
	return strings.Join(parts, "; ")
}

// ─── The work ────────────────────────────────────────────────────────────────

// EnchantFee is what an enchanter charges for a piece: the size's base fee ×
// their factor × the quality's.
func EnchantFee(c *types.GemsConfig, g Gem, cfg *types.EnchantConfig) int {
	q := c.Enchanting.Quality[g.Quality]
	fee := float64(c.Enchanting.Fees[g.Size]) * cfg.Fee
	if q.Fee > 0 {
		fee *= q.Fee
	}
	return max(1, int(math.Round(fee)))
}

// EnchantChance is an enchanter's chance on a piece: their base plus the
// quality's, held between 5% and max_success; a flawless gem always takes.
func EnchantChance(c *types.GemsConfig, g Gem, cfg *types.EnchantConfig) float64 {
	q := c.Enchanting.Quality[g.Quality]
	if q.Always {
		return 1
	}
	top := c.Enchanting.MaxSuccess
	if top <= 0 {
		top = 0.95
	}
	return math.Max(0.05, math.Min(top, cfg.Success+q.Success))
}

// Refusal says why an enchanter won't take a piece, or "" if they will.
func Refusal(c *types.GemsConfig, g Gem, cfg *types.EnchantConfig) string {
	switch {
	case cfg == nil:
		return "They don't enchant."
	case g.Enchanted:
		return "It's already enchanted."
	case !Enchantable(c, g):
		return "There's nothing in a stone that size to wake."
	case !contains(cfg.Pieces, g.Piece):
		return fmt.Sprintf("They don't work %ss.", strings.ToLower(c.Jeweller.Pieces[g.Piece].Name))
	case len(cfg.Gems) > 0 && !contains(cfg.Gems, g.Type):
		return fmt.Sprintf("They don't know %s.", strings.ToLower(c.Types[g.Type].Name))
	case SizeIndex(c, g.Size) > SizeIndex(c, cfg.MaxSize):
		return "That stone is too big for them."
	}
	return ""
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

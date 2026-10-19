package spells

import (
	"errors"
	"database/sql"
	"fmt"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
	"pubkey-quest/cmd/server/game/spellbook"
)

// ─── Casting engine (M4 Phase A) ─────────────────────────────────────────────
//
// Cast is the single, shared, data-driven spell resolver used by both combat
// (game/combat) and out-of-combat (api/game). It is pure with respect to dice:
// randomness and monster-damage resolution are injected via Deps so the same
// engine is deterministic under test and reuses combat's one implementation of
// dice/resistances at runtime (avoiding an import cycle — combat imports spells,
// never the reverse).
//
// Resolution is shape-driven, read straight from the spell JSON:
//   spell_attack set  → attack roll vs target AC          (fire-bolt)
//   spell_attack=auto → auto-hit damage                   (magic-missile)
//   save_type set     → target save vs DC, full/half dmg  (poison-spray);
//                       with an `aoe`, every creature in Deps.Area saves (fireball)
//   heal set          → healing to the caster/self        (cure-wounds)
//   effect set        → buff/utility, mapped to an ActiveEffect where one exists
//
// The engine mutates `save` only for costs that are identical in and out of
// combat: it spends mana, consumes material components, and applies buff effects
// to the caster. Damage and healing to a *target* are returned in CastResult so
// the caller can apply them to the correct HP pool (a combat monster, the combat
// player-HP, or out-of-combat save.HP) and own the XP/kill/death flow.

// Deps injects the non-deterministic pieces so the engine stays testable.
type Deps struct {
	// RollD20 returns a raw d20 (1–20).
	RollD20 func() int
	// RollDice rolls a dice expression like "1d10" (crit doubles the dice count).
	RollDice func(expr string, crit bool) int
	// ResolveMonsterDamage rolls damage and applies the monster's
	// resistances/immunities/vulnerabilities, returning final damage (min 1 on a
	// hit). It does NOT mutate monster HP — the caller applies it.
	ResolveMonsterDamage func(diceExpr string, mod int, damageType string, crit bool, m *types.MonsterInstance) int
	// ComponentsAtHand limits material components to what's at hand — the
	// general slots and containers in them (a component pouch). Set in combat:
	// components left in the backpack give ErrComponentsInPack, and digging them
	// out costs the caster's bonus action (the combat layer retries without it).
	ComponentsAtHand bool
	// InCombat: the cast is a combat turn. Out of combat, a spell whose only
	// effect is in a fight (a combat buff, a condition) is refused before it
	// costs anything; in combat, a spell with both a combat buff and an
	// out-of-combat effect uses the buff.
	InCombat bool
	// AddItem puts a spell-created item in the caster's inventory (goodberry),
	// returning how many fit. Nil leaves created items out.
	AddItem func(save *types.SaveFile, itemID string, qty int) int
	// AttackBonus and AttackAdvantage feed a spell attack roll what the fight
	// adds: buff dice (bless) and the sum of advantage sources (>0 advantage,
	// <0 disadvantage), e.g. a faerie-fired or restrained target.
	AttackBonus     int
	AttackAdvantage int
	// Area lists the creatures a save spell's area (its `aoe`) caught, the
	// target first. Each makes its own save; the results are CastResult.Targets.
	// Empty means the target alone.
	Area []*types.MonsterInstance
}

// ErrComponentsInPack: a combat cast's components are only in the backpack.
var ErrComponentsInPack = errors.New("components are in your pack")

// CastResult describes the outcome of a resolved cast. Costs (mana/components/
// buff-effect) are already applied to the save; damage/heal are returned for the
// caller to apply to the right HP pool.
type CastResult struct {
	SpellID       string
	SpellName     string
	Shape         string   // "attack" | "auto" | "save" | "heal" | "buff" | "utility"
	Log           []string // player-facing lines (combat-log style)
	ManaSpent     int
	Hit           bool   // attack shapes: did the attack land
	Crit          bool   // natural 20
	Damage        int    // final damage to apply to the target (0 if none/miss)
	DamageType    string
	SaveDC        int    // save shapes: the DC the target rolled against
	SaveMade      bool   // save shapes: did the target succeed
	Heal          int    // healing to apply (caller decides the pool)
	EffectID      string // ActiveEffect applied to the caster ("" if none)
	Concentration bool   // caster is now concentrating on this spell
	// Spell is the spell's data, so the combat layer can read its combat_buff
	// and applies_condition blocks without loading it again.
	Spell map[string]interface{}
	// Cures lists the conditions the spell ends (its `cures` field); out of
	// combat the matching status effects are already lifted, in combat the
	// caller ends the conditions on the target.
	Cures []string
	// Targets is each creature an area spell caught (Deps.Area) and how it
	// fared, the target first; Damage and SaveMade above mirror that first
	// one. Nil for a single-target cast.
	Targets []TargetOutcome
}

// TargetOutcome is one creature's result against an area save spell.
type TargetOutcome struct {
	Monster  *types.MonsterInstance
	Damage   int  // final damage for the caller to apply (0 if none)
	SaveMade bool // the creature saved: half damage, and no rider
}

// Cast validates and resolves a spell. `level` is the caster level (drives the
// spell attack bonus and save DC). `target` is the combat monster, or nil for an
// out-of-combat cast. Returns an error (before spending any cost) if the cast is
// illegal: not known, not prepared, insufficient mana, missing components, or a
// combat-only shape cast with no target.
//
// Cast is the DB-backed entry point: it loads the spell and wires the equipped-
// focus lookup, then delegates the pure resolution to resolveCast (which is
// DB-free and unit-testable with a hand-built spell + stub focus lookup).
func Cast(db *sql.DB, deps Deps, save *types.SaveFile, spellID string, level int, target *types.MonsterInstance) (*CastResult, error) {
	spell, err := gamedata.LoadSpellByID(db, spellID)
	if err != nil {
		return nil, err
	}
	focus := func(component string) bool { return equippedFocusProvides(db, save, component) }
	return resolveCast(withDefaults(deps), save, spell, spellID, level, target, focus, false)
}

// withDefaults wires the DB-backed pieces a caller didn't inject.
func withDefaults(deps Deps) Deps {
	if deps.AddItem == nil {
		deps.AddItem = func(save *types.SaveFile, itemID string, qty int) int {
			n, _ := gaminventory.AddItemToInventory(save, itemID, qty)
			return n
		}
	}
	return deps
}

// HasOutOfCombatUse reports whether a spell does anything outside a fight:
// healing, a lasting effect, curing a condition, or creating something. Spells
// without one (a smite, a condition) can only be cast in combat.
func HasOutOfCombatUse(spell map[string]interface{}) bool {
	if stringField(spell, "heal") != "" || stringField(spell, "applies_effect") != "" {
		return true
	}
	if c, _ := spell["cures"].([]interface{}); len(c) > 0 {
		return true
	}
	_, creates := spell["creates_item"].(map[string]interface{})
	return creates
}

// IsReaction reports whether a spell is cast as a reaction (shield, hellish
// rebuke). Reactions aren't a turn's action: kept prepared, they fire on their
// own when their trigger happens (see the combat layer).
func IsReaction(spell map[string]interface{}) bool {
	return stringField(spell, "action_cost") == "reaction"
}

// TargetsEnemy reports whether a spell is aimed at an enemy (attack, auto-hit
// or save shapes), as opposed to healing, buffs and utility.
func TargetsEnemy(db *sql.DB, spellID string) bool {
	spell, err := gamedata.LoadSpellByID(db, spellID)
	if err != nil {
		return false
	}
	shape := spellShape(spell)
	return shape == "attack" || shape == "auto" || shape == "save"
}

// CastFromScroll casts a spell off a scroll. The scroll IS the spell, so it
// bypasses the known + prepared + material-component gates, but still spends mana
// and resolves by the same shape as a normal cast. The caller consumes the scroll.
func CastFromScroll(db *sql.DB, deps Deps, save *types.SaveFile, spellID string, level int, target *types.MonsterInstance) (*CastResult, error) {
	spell, err := gamedata.LoadSpellByID(db, spellID)
	if err != nil {
		return nil, err
	}
	return resolveCast(withDefaults(deps), save, spell, spellID, level, target, func(string) bool { return false }, true)
}

// resolveCast is the pure casting resolver: no database access. Components that
// need a focus are checked via focusProvides. fromScroll skips the known/prepared/
// component gates (the scroll carries the spell). See Cast for the DB-backed wrapper.
func resolveCast(deps Deps, save *types.SaveFile, spell map[string]interface{}, spellID string, level int, target *types.MonsterInstance, focusProvides func(string) bool, fromScroll bool) (*CastResult, error) {
	name := stringField(spell, "name")
	if name == "" {
		name = spellID
	}

	// ── Validation (no state is mutated until every check passes) ──
	// A scroll bypasses the known + prepared gates — the scroll carries the spell.
	if !fromScroll {
		if !knowsSpell(save, spellID) {
			return nil, fmt.Errorf("you have not learned %s", name)
		}
		if !IsSpellPrepared(save, spellID) {
			return nil, fmt.Errorf("%s is not prepared", name)
		}
	}

	shape := spellShape(spell)
	needsTarget := shape == "attack" || shape == "auto" || shape == "save"
	if needsTarget && target == nil {
		return nil, fmt.Errorf("%s can only be cast on an enemy in combat", name)
	}
	if !deps.InCombat && target == nil && !HasOutOfCombatUse(spell) {
		return nil, fmt.Errorf("%s only has an effect in a fight", name)
	}

	manaCost := intField(spell, "mana_cost")
	if save.Mana < manaCost {
		return nil, fmt.Errorf("not enough mana to cast %s (need %d, have %d)", name, manaCost, save.Mana)
	}

	// Scrolls skip material components (paid at craft time); a normal cast resolves
	// them (consuming from the pouch, or free if a focus provides them).
	var plan []componentUse
	if !fromScroll {
		p, err := resolveComponents(save, spell, focusProvides, deps.ComponentsAtHand)
		if err != nil {
			return nil, err
		}
		plan = p
	}

	// ── Commit costs ──
	save.Mana -= manaCost
	for _, c := range plan {
		consumeComponent(save.Inventory, c.component, c.quantity)
	}

	res := &CastResult{
		SpellID:       spellID,
		SpellName:     name,
		Shape:         shape,
		ManaSpent:     manaCost,
		Spell:         spell,
		Concentration: boolField(spell, "concentration"),
	}

	statScore, ability, isCaster := castingProfile(save)
	_ = ability

	// ── Resolve by shape ──
	switch shape {
	case "attack":
		atkBonus := spellAttackBonus(level, statScore) + deps.AttackBonus
		roll := deps.RollD20()
		advNote := ""
		if deps.AttackAdvantage != 0 {
			first, second := roll, deps.RollD20()
			if (deps.AttackAdvantage > 0) == (second > roll) {
				roll = second
			}
			kind := "advantage"
			if deps.AttackAdvantage < 0 {
				kind = "disadvantage"
			}
			advNote = fmt.Sprintf(" (%s: %d, %d)", kind, first, second)
		}
		crit := roll == 20
		critMiss := roll == 1
		total := roll + atkBonus
		hit := crit || (!critMiss && total >= target.ArmorClass)
		res.Hit = hit
		res.Crit = crit
		res.Log = append(res.Log, fmt.Sprintf("  You cast %s: rolled %d%s vs AC %d%s", name, roll, mod(atkBonus), target.ArmorClass, advNote))
		if !hit {
			res.Log = append(res.Log, "  ✘ The spell misses.")
			break
		}
		dtype := stringField(spell, "damage_type")
		res.Damage = deps.ResolveMonsterDamage(stringField(spell, "damage"), 0, dtype, crit, target)
		res.DamageType = dtype
		critTxt := ""
		if crit {
			critTxt = " Critical hit!"
		}
		res.Log = append(res.Log, fmt.Sprintf("  💥 %s strikes for %d %s damage.%s", name, res.Damage, dtype, critTxt))

	case "auto":
		dtype := stringField(spell, "damage_type")
		res.Hit = true
		res.Damage = deps.ResolveMonsterDamage(stringField(spell, "damage"), 0, dtype, false, target)
		res.DamageType = dtype
		res.Log = append(res.Log, fmt.Sprintf("  You cast %s — it strikes automatically for %d %s damage.", name, res.Damage, dtype))

	case "save":
		saveType := strings.ToLower(stringField(spell, "save_type"))
		dc := spellSaveDC(level, statScore)
		res.SaveDC = dc
		dtype := stringField(spell, "damage_type")
		hasDamage := stringField(spell, "damage") != ""
		if hasDamage {
			res.DamageType = dtype
		}
		// An area spell: everyone caught saves on their own.
		if len(deps.Area) > 0 {
			res.Log = append(res.Log, fmt.Sprintf("  You cast %s (DC %d %s save) — it catches %s.", name, dc, saveType, enemyCount(len(deps.Area))))
			for _, m := range deps.Area {
				out, roll, bonus := saveAgainst(deps, spell, saveType, dc, m)
				res.Targets = append(res.Targets, out)
				res.Log = append(res.Log, areaSaveLine(out, roll, bonus, dtype, hasDamage))
			}
			res.Damage, res.SaveMade = res.Targets[0].Damage, res.Targets[0].SaveMade
			break
		}
		out, roll, saveBonus := saveAgainst(deps, spell, saveType, dc, target)
		res.Damage, res.SaveMade = out.Damage, out.SaveMade
		res.Log = append(res.Log, fmt.Sprintf("  You cast %s (DC %d). %s rolls %d%s %s save.",
			name, dc, target.Name, roll, mod(saveBonus), saveType))
		switch {
		case hasDamage && out.SaveMade:
			res.Log = append(res.Log, fmt.Sprintf("  ✔ %s resists — %d %s damage (half).", target.Name, res.Damage, dtype))
		case hasDamage:
			res.Log = append(res.Log, fmt.Sprintf("  ✘ %s fails — %d %s damage.", target.Name, res.Damage, dtype))
		case out.SaveMade:
			// A pure control save (entangle, command): the combat layer lands
			// the spell's applies_condition on a failure.
			res.Log = append(res.Log, fmt.Sprintf("  ✔ %s resists the spell.", target.Name))
		}

	case "heal":
		castMod := character.AbilityMod(statScore)
		if !isCaster {
			castMod = 0
		}
		heal := deps.RollDice(stringField(spell, "heal"), false) + castMod
		if heal < 1 {
			heal = 1
		}
		res.Heal = heal
		res.Log = append(res.Log, fmt.Sprintf("  You cast %s, mending %d HP.", name, heal))

	default: // "buff" | "utility"
		res.Log = append(res.Log, castNarrative(name, spell))
	}

	resolveLasting(deps, save, spell, res)
	return res, nil
}

// resolveLasting applies a spell's out-of-combat parts, whatever its shape:
// the ActiveEffect it grants (applies_effect), the conditions it cures, and
// the items it creates. In combat a spell that also has a combat_buff uses
// the buff instead of its effect (the combat layer applies it).
func resolveLasting(deps Deps, save *types.SaveFile, spell map[string]interface{}, res *CastResult) {
	_, hasBuff := spell["combat_buff"].(map[string]interface{})
	if effectID := stringField(spell, "applies_effect"); effectID != "" && !(deps.InCombat && hasBuff) {
		effects.RemoveEffect(save, effectID) // casting again refreshes it rather than stacking
		if msg, err := effects.ApplyEffectWithMessage(save, effectID); err == nil {
			res.EffectID = effectID
			if msg != nil && msg.Message != "" {
				res.Log = append(res.Log, "  "+msg.Message)
			}
		}
	}

	if list, _ := spell["cures"].([]interface{}); len(list) > 0 {
		for _, c := range list {
			if s, ok := c.(string); ok && s != "" {
				res.Cures = append(res.Cures, s)
			}
		}
		if !deps.InCombat {
			cured := gaminventory.CureEffects(save, res.Cures)
			for _, c := range cured {
				res.Log = append(res.Log, fmt.Sprintf("  You are no longer %s.", c))
			}
			if len(cured) == 0 {
				res.Log = append(res.Log, "  Nothing here needed it.")
			}
		}
	}

	if mk, ok := spell["creates_item"].(map[string]interface{}); ok && deps.AddItem != nil {
		itemID := stringField(mk, "item")
		qty := intField(mk, "quantity")
		if qty < 1 {
			qty = 1
		}
		if itemID != "" {
			got := deps.AddItem(save, itemID, qty)
			switch {
			case got <= 0:
				res.Log = append(res.Log, "  You have no room to hold what the spell made, and it fades.")
			case got < qty:
				res.Log = append(res.Log, fmt.Sprintf("  Only %d of %d fit in your pack; the rest fade.", got, qty))
			}
		}
	}
}

// saveAgainst rolls one creature's save against a save spell, and the spell's
// damage to it: full on a failure, half on a success. Bane takes a d4 off the
// save. Returns the outcome, the d20 and the creature's save bonus.
func saveAgainst(deps Deps, spell map[string]interface{}, saveType string, dc int, m *types.MonsterInstance) (TargetOutcome, int, int) {
	roll := deps.RollD20()
	bonus := monsterSaveBonus(m, saveType)
	if hasCondition(m.Conditions, "baned") {
		bonus -= deps.RollDice("1d4", false)
	}
	out := TargetOutcome{Monster: m, SaveMade: roll+bonus >= dc}
	if dice := stringField(spell, "damage"); dice != "" {
		full := deps.ResolveMonsterDamage(dice, 0, stringField(spell, "damage_type"), false, m)
		out.Damage = full
		if out.SaveMade {
			out.Damage = full / 2
		}
	}
	return out, roll, bonus
}

// areaSaveLine is one creature's line in an area spell's log.
func areaSaveLine(out TargetOutcome, roll, bonus int, dtype string, hasDamage bool) string {
	name, rolled := out.Monster.Name, fmt.Sprintf("%d%s", roll, mod(bonus))
	switch {
	case hasDamage && out.SaveMade:
		return fmt.Sprintf("  ✔ %s saves (%s) — %d %s damage (half).", name, rolled, out.Damage, dtype)
	case hasDamage:
		return fmt.Sprintf("  ✘ %s fails (%s) — %d %s damage.", name, rolled, out.Damage, dtype)
	case out.SaveMade:
		return fmt.Sprintf("  ✔ %s saves (%s).", name, rolled)
	}
	return fmt.Sprintf("  ✘ %s fails (%s).", name, rolled)
}

// enemyCount words how many enemies an area caught.
func enemyCount(n int) string {
	if n == 1 {
		return "1 enemy"
	}
	return fmt.Sprintf("%d enemies", n)
}

// hasCondition reports whether a monster's condition list holds a condition.
func hasCondition(conds []types.CombatCondition, name string) bool {
	for _, c := range conds {
		if strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

// castNarrative returns a flavour line for buff/utility spells, preferring the
// spell's `effect` prose and falling back to a plain "You cast X."
func castNarrative(name string, spell map[string]interface{}) string {
	if eff := stringField(spell, "effect"); eff != "" {
		return fmt.Sprintf("  You cast %s. %s", name, eff)
	}
	return fmt.Sprintf("  You cast %s.", name)
}

// ShapeOf is a spell's resolution shape: "attack" | "auto" | "save" | "heal" |
// "buff" | "utility".
func ShapeOf(spell map[string]interface{}) string { return spellShape(spell) }

// spellShape classifies a spell by its primary resolution shape (see file header).
func spellShape(spell map[string]interface{}) string {
	atk := strings.ToLower(stringField(spell, "spell_attack"))
	switch atk {
	case "ranged", "melee":
		return "attack"
	case "automatic":
		return "auto"
	}
	if stringField(spell, "save_type") != "" {
		return "save"
	}
	if stringField(spell, "heal") != "" {
		return "heal"
	}
	if stringField(spell, "effect") != "" || spell["combat_buff"] != nil || stringField(spell, "applies_effect") != "" {
		return "buff"
	}
	return "utility"
}

// ─── Casting math ────────────────────────────────────────────────────────────

// castingProfile returns the caster's spellcasting ability score, ability name,
// and whether the class casts at all.
func castingProfile(save *types.SaveFile) (score int, ability string, isCaster bool) {
	ability, isCaster = character.SpellcastingAbility(save.Class)
	if !isCaster {
		return 10, "", false
	}
	return character.AbilityScore(save.Stats, ability), ability, true
}

// spellAttackBonus = proficiency bonus + spellcasting ability modifier.
func spellAttackBonus(level, statScore int) int {
	return character.ProficiencyBonus(level) + character.AbilityMod(statScore)
}

// spellSaveDC = 8 + proficiency bonus + spellcasting ability modifier.
func spellSaveDC(level, statScore int) int {
	return 8 + character.ProficiencyBonus(level) + character.AbilityMod(statScore)
}

// monsterSaveBonus returns a monster's total save modifier for an ability. If the
// monster is proficient (the ability appears in saving_throws, which lists the
// full 5e save bonus) that value is used; otherwise it is the ability modifier.
func monsterSaveBonus(m *types.MonsterInstance, ability string) int {
	if m == nil {
		return 0
	}
	if v, ok := m.Data.SavingThrows[ability]; ok {
		return v
	}
	return character.AbilityMod(monsterAbilityScore(m.Data.Stats, ability))
}

// monsterAbilityScore reads a named ability score off a monster's stat block.
func monsterAbilityScore(s types.MonsterStats, ability string) int {
	switch strings.ToLower(ability) {
	case "strength", "str":
		return s.Strength
	case "dexterity", "dex":
		return s.Dexterity
	case "constitution", "con":
		return s.Constitution
	case "intelligence", "int":
		return s.Intelligence
	case "wisdom", "wis":
		return s.Wisdom
	case "charisma", "cha":
		return s.Charisma
	}
	return 10
}

// ─── Known / prepared checks ────────────────────────────────────────────────

func knowsSpell(save *types.SaveFile, spellID string) bool {
	return spellbook.Knows(save, spellID)
}

// IsSpellPrepared reports whether spellID currently occupies any prepared slot in
// the save's SpellSlots (across every slot level).
func IsSpellPrepared(save *types.SaveFile, spellID string) bool {
	for _, raw := range save.SpellSlots {
		list, ok := raw.([]interface{})
		if !ok {
			continue
		}
		for _, entry := range list {
			m, ok := entry.(map[string]interface{})
			if !ok {
				continue
			}
			if s, ok := m["spell"].(string); ok && s == spellID {
				return true
			}
		}
	}
	return false
}

// ─── Material components ─────────────────────────────────────────────────────

type componentUse struct {
	component string
	quantity  int
}

// resolveComponents builds the consume-plan for a spell's material components.
// A required component is free when focusProvides reports an equipped focus
// supplies it; otherwise the player must hold enough of it (component pouch /
// general slots / backpack). Returns an error naming the first shortfall.
func resolveComponents(save *types.SaveFile, spell map[string]interface{}, focusProvides func(string) bool, atHand bool) ([]componentUse, error) {
	mc, ok := spell["material_component"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	reqRaw, ok := mc["required"].([]interface{})
	if !ok || len(reqRaw) == 0 {
		return nil, nil
	}

	var plan []componentUse
	for _, r := range reqRaw {
		rm, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		comp := stringField(rm, "component")
		if comp == "" {
			continue
		}
		qty := intField(rm, "quantity")
		if qty < 1 {
			qty = 1
		}
		if focusProvides != nil && focusProvides(comp) {
			continue // focus supplies it — no consumption
		}
		if countComponent(save.Inventory, comp) < qty {
			return nil, fmt.Errorf("need %d × %s (or a focus that provides it)", qty, comp)
		}
		if atHand && countAtHand(save.Inventory, comp) < qty {
			return nil, fmt.Errorf("%w: %s", ErrComponentsInPack, comp)
		}
		plan = append(plan, componentUse{component: comp, quantity: qty})
	}
	return plan, nil
}

// equippedFocusProvides reports whether any item equipped in a gear slot is a
// focus (tag "focus") whose `provides` equals the component id.
func equippedFocusProvides(db *sql.DB, save *types.SaveFile, component string) bool {
	gear, ok := save.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return false
	}
	for _, raw := range gear {
		slot, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		itemID, _ := slot["item"].(string)
		if itemID == "" {
			continue
		}
		item, err := gamedata.LoadItemByID(db, itemID)
		if err != nil {
			continue
		}
		if !hasStringTag(item["tags"], "focus") {
			continue
		}
		if provides, _ := item["provides"].(string); provides == component {
			return true
		}
	}
	return false
}

// countAtHand totals how many of itemID sit in the general slots, including
// containers in them (a component pouch) — what can be reached mid-fight.
func countAtHand(inv map[string]interface{}, itemID string) int {
	gen, _ := inv["general_slots"].([]interface{})
	return countInSlotList(gen, itemID)
}

// countComponent totals how many of itemID the player holds across general slots
// (including container contents like the component pouch) and the equipped bag.
func countComponent(inv map[string]interface{}, itemID string) int {
	total := 0
	if gen, ok := inv["general_slots"].([]interface{}); ok {
		total += countInSlotList(gen, itemID)
	}
	if gear, ok := inv["gear_slots"].(map[string]interface{}); ok {
		for _, raw := range gear {
			slot, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			if contents, ok := slot["contents"].([]interface{}); ok {
				total += countInSlotList(contents, itemID)
			}
		}
	}
	return total
}

// countInSlotList sums matching quantities in a slot list, recursing one level
// into any container `contents` it encounters.
func countInSlotList(list []interface{}, itemID string) int {
	total := 0
	for _, raw := range list {
		slot, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := slot["item"].(string); id == itemID {
			total += slotQuantity(slot)
		}
		if contents, ok := slot["contents"].([]interface{}); ok {
			total += countInSlotList(contents, itemID)
		}
	}
	return total
}

// consumeComponent removes up to qty of itemID from the inventory, draining
// stacks (general slots, container contents, the bag) until satisfied. Emptied
// stacks are cleared to {item:null, quantity:0} to match the rest of the
// inventory code. Returns the amount actually removed.
func consumeComponent(inv map[string]interface{}, itemID string, qty int) int {
	remaining := qty
	if gen, ok := inv["general_slots"].([]interface{}); ok {
		remaining = consumeFromSlotList(gen, itemID, remaining)
	}
	if remaining > 0 {
		if gear, ok := inv["gear_slots"].(map[string]interface{}); ok {
			for _, raw := range gear {
				slot, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if contents, ok := slot["contents"].([]interface{}); ok {
					remaining = consumeFromSlotList(contents, itemID, remaining)
					if remaining == 0 {
						break
					}
				}
			}
		}
	}
	return qty - remaining
}

// consumeFromSlotList drains matching stacks in a slot list (recursing into
// container contents), returning how many still need to be removed.
func consumeFromSlotList(list []interface{}, itemID string, need int) int {
	for _, raw := range list {
		if need == 0 {
			return 0
		}
		slot, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := slot["item"].(string); id == itemID {
			have := slotQuantity(slot)
			take := have
			if take > need {
				take = need
			}
			left := have - take
			if left <= 0 {
				slot["item"] = nil
				slot["quantity"] = 0
			} else {
				slot["quantity"] = left
			}
			need -= take
		}
		if contents, ok := slot["contents"].([]interface{}); ok {
			need = consumeFromSlotList(contents, itemID, need)
		}
	}
	return need
}

// ─── Small field readers (tolerant of JSON float64 / int) ────────────────────

func slotQuantity(slot map[string]interface{}) int {
	return intField(slot, "quantity")
}

func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func intField(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		// spell fields like "range" are stored as strings; tolerate a numeric one.
		n := 0
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return 0
}

func boolField(m map[string]interface{}, key string) bool {
	b, _ := m[key].(bool)
	return b
}

// mod formats a signed modifier for a log line (+3 / -1 / +0).
func mod(n int) string {
	if n >= 0 {
		return fmt.Sprintf("+%d", n)
	}
	return fmt.Sprintf("%d", n)
}

// hasStringTag reports whether a raw JSON tags value contains tag (case-insensitive).
func hasStringTag(tags interface{}, tag string) bool {
	list, ok := tags.([]interface{})
	if !ok {
		return false
	}
	for _, t := range list {
		if s, ok := t.(string); ok && strings.EqualFold(s, tag) {
			return true
		}
	}
	return false
}

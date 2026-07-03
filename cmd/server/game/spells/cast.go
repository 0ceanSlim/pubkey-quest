package spells

import (
	"database/sql"
	"fmt"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/types"
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
//   save_type set     → target save vs DC, full/half dmg  (poison-spray)
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
}

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
	return resolveCast(deps, save, spell, spellID, level, target, focus)
}

// resolveCast is the pure casting resolver: no database access. Components that
// need a focus are checked via focusProvides. See Cast for the DB-backed wrapper.
func resolveCast(deps Deps, save *types.SaveFile, spell map[string]interface{}, spellID string, level int, target *types.MonsterInstance, focusProvides func(string) bool) (*CastResult, error) {
	name := stringField(spell, "name")
	if name == "" {
		name = spellID
	}

	// ── Validation (no state is mutated until every check passes) ──
	if !knowsSpell(save, spellID) {
		return nil, fmt.Errorf("you have not learned %s", name)
	}
	if !IsSpellPrepared(save, spellID) {
		return nil, fmt.Errorf("%s is not prepared", name)
	}

	shape := spellShape(spell)
	needsTarget := shape == "attack" || shape == "auto" || shape == "save"
	if needsTarget && target == nil {
		return nil, fmt.Errorf("%s can only be cast on an enemy in combat", name)
	}

	manaCost := intField(spell, "mana_cost")
	if save.Mana < manaCost {
		return nil, fmt.Errorf("not enough mana to cast %s (need %d, have %d)", name, manaCost, save.Mana)
	}

	plan, err := resolveComponents(save, spell, focusProvides)
	if err != nil {
		return nil, err
	}

	// ── Commit costs ──
	save.Mana -= manaCost
	for _, c := range plan {
		consumeComponent(save.Inventory, c.component, c.quantity)
	}

	res := &CastResult{
		SpellID:   spellID,
		SpellName: name,
		Shape:     shape,
		ManaSpent: manaCost,
	}

	statScore, ability, isCaster := castingProfile(save)
	_ = ability

	// ── Resolve by shape ──
	switch shape {
	case "attack":
		atkBonus := spellAttackBonus(level, statScore)
		roll := deps.RollD20()
		crit := roll == 20
		critMiss := roll == 1
		total := roll + atkBonus
		hit := crit || (!critMiss && total >= target.ArmorClass)
		res.Hit = hit
		res.Crit = crit
		res.Log = append(res.Log, fmt.Sprintf("  You cast %s: rolled %d%s vs AC %d", name, roll, mod(atkBonus), target.ArmorClass))
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
		roll := deps.RollD20()
		saveBonus := monsterSaveBonus(target, saveType)
		made := roll+saveBonus >= dc
		res.SaveMade = made
		res.Log = append(res.Log, fmt.Sprintf("  You cast %s (DC %d). %s rolls %d%s %s save.",
			name, dc, target.Name, roll, mod(saveBonus), saveType))
		dice := stringField(spell, "damage")
		if dice != "" {
			dtype := stringField(spell, "damage_type")
			full := deps.ResolveMonsterDamage(dice, 0, dtype, false, target)
			if made {
				res.Damage = full / 2
				res.Log = append(res.Log, fmt.Sprintf("  ✔ %s resists — %d %s damage (half).", target.Name, res.Damage, dtype))
			} else {
				res.Damage = full
				res.Log = append(res.Log, fmt.Sprintf("  ✘ %s fails — %d %s damage.", target.Name, res.Damage, dtype))
			}
			res.DamageType = dtype
		} else {
			// Pure control/condition save (entangle, faerie-fire): the save
			// resolves, but the condition itself lands in the M5 conditions pass.
			res.Log = append(res.Log, "  (The spell's condition takes hold — full effect arrives with the conditions system.)")
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

	case "buff":
		if effectID, ok := spellEffect(spellID); ok {
			if err := effects.ApplyEffect(save, effectID); err == nil {
				res.EffectID = effectID
				res.Concentration = boolField(spell, "concentration")
			}
		}
		res.Log = append(res.Log, castNarrative(name, spell))

	default: // "utility"
		res.Log = append(res.Log, castNarrative(name, spell))
	}

	return res, nil
}

// castNarrative returns a flavour line for buff/utility spells, preferring the
// spell's `effect` prose and falling back to a plain "You cast X."
func castNarrative(name string, spell map[string]interface{}) string {
	if eff := stringField(spell, "effect"); eff != "" {
		return fmt.Sprintf("  You cast %s. %s", name, eff)
	}
	return fmt.Sprintf("  You cast %s.", name)
}

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
	if stringField(spell, "effect") != "" {
		return "buff"
	}
	return "utility"
}

// spellEffect maps a spell id to an ActiveEffect def id, for buffs that have a
// matching effect in game-data/effects. Unmapped buffs resolve as narrative
// (no mechanical effect yet) — new effect defs (bless, mage-armor, …) extend
// this map without touching the engine.
var spellEffectByID = map[string]string{
	"bless":       "blessed",
	"haste":       "haste",
	"regenerate":  "regeneration",
}

func spellEffect(spellID string) (string, bool) {
	e, ok := spellEffectByID[spellID]
	return e, ok
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
	for _, id := range save.KnownSpells {
		if id == spellID {
			return true
		}
	}
	return false
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
func resolveComponents(save *types.SaveFile, spell map[string]interface{}, focusProvides func(string) bool) ([]componentUse, error) {
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

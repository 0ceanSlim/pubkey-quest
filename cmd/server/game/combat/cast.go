package combat

import (
	"errors"
	"database/sql"
	"fmt"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/character"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/spells"
	"pubkey-quest/types"
)

// ─── Combat casting (M4 Phase B) ─────────────────────────────────────────────
//
// ProcessPlayerCast is the combat-side wrapper around the shared casting engine
// (spells.Cast). It mirrors ProcessPlayerAttack: validate the action economy,
// run the data-driven engine against the target monster, then own the combat
// consequences the engine deliberately leaves to the caller — applying damage to
// the monster, awarding damage XP, resolving a kill, healing the combat HP pool,
// and setting concentration. The engine already spent mana + components and
// applied any buff effect to the save.

// ProcessPlayerCast resolves the player casting spellID at a monster (targetID,
// "" = nearest). Does NOT run the monsters' turns — the caller ends the turn like
// any action.
func ProcessPlayerCast(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, spellID, targetID string, advancement []types.AdvancementEntry) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot cast: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	spell, err := gamedata.LoadSpellByID(db, spellID)
	if err != nil {
		return nil, err
	}
	name, _ := spell["name"].(string)
	if name == "" {
		name = spellID
	}
	buff := combatBuffOf(spell)
	hostile := hostileSpell(db, spellID, spell)

	// A spell can be aimed at an ally (heals, buffs) or an enemy.
	ally := FindMember(cs, targetID)
	var monster *types.MonsterInstance
	if ally != nil {
		if ally.Fled {
			return nil, fmt.Errorf("%s has left the fight", ally.Name)
		}
		if hostile {
			return nil, fmt.Errorf("aim that spell at an enemy")
		}
	} else {
		m, err := ResolveTarget(cs, targetID)
		if err != nil {
			return nil, err
		}
		monster = m
	}
	me := actor(cs)
	state := &me.CombatState
	if IsIncapacitated(state.Conditions) {
		return nil, fmt.Errorf("you are incapacitated and can't cast")
	}
	// The one it's aimed at must be within the spell's reach (see spellrange.go).
	if ally != nil && ally != me && aimsAtAlly(spell, buff) {
		if err := checkSpellReach(spell, name, me.Pos, ally.Pos, ally.Name); err != nil {
			return nil, err
		}
	} else if monster != nil && hostile {
		if err := checkSpellReach(spell, name, me.Pos, monster.Pos, monster.Name); err != nil {
			return nil, err
		}
	}
	// An area spell catches every enemy in its area (spared: allies).
	monster, caught, err := aimArea(cs, me, spell, name, monster)
	if err != nil {
		return nil, err
	}

	// Action economy — validated up front, before the engine spends any cost.
	// A spell's combat cost is its action_cost ("action" | "bonus_action");
	// reactions fire on their own, and anything longer than an action can't be
	// cast as a combat turn.
	actionCost, _ := spell["action_cost"].(string)
	if actionCost == "" {
		actionCost = "action"
	}
	switch actionCost {
	case "bonus_action":
		if state.BonusActionUsed {
			return nil, fmt.Errorf("bonus action already used this turn")
		}
	case "action":
		if state.ActionUsed {
			return nil, fmt.Errorf("action already used this turn")
		}
	case "reaction":
		return nil, fmt.Errorf("%s is a reaction: keep it prepared and it casts itself when its moment comes", name)
	default:
		return nil, fmt.Errorf("%s takes too long to cast in combat", name)
	}

	level := character.GetLevelFromXP(save.Experience, advancement)

	deps := spells.Deps{
		RollD20:              RollD20,
		RollDice:             RollDice,
		ResolveMonsterDamage: ResolveDamageToMonster,
		InCombat:             true,
		Area:                 caught,
	}
	// Elemental Adept: for the chosen damage type, the player's spells ignore the
	// target's resistance and treat any damage die that rolls 1 as a 2.
	if elem := character.FeatChoice(save, "elemental-adept"); elem != "" {
		deps.ResolveMonsterDamage = func(dice string, mod int, dtype string, crit bool, m *types.MonsterInstance) int {
			if strings.EqualFold(dtype, elem) {
				return resolveElementalAdeptDamage(dice, mod, dtype, crit, m)
			}
			return ResolveDamageToMonster(dice, mod, dtype, crit, m)
		}
	}
	// Offensive spells need a line to the target (one centred on the caster
	// doesn't); cover raises its AC against spell attacks for the duration of
	// the cast.
	var preLog []string
	if monster != nil && spells.TargetsEnemy(db, spellID) && !selfCentred(spell) {
		restore, lines, err := applySpellCover(cs, monster)
		if err != nil {
			return nil, err
		}
		defer restore()
		preLog = lines
	}
	// A spell attack takes what the fight gives a roll: buff dice (bless),
	// advantage (true strike, a restrained or faerie-fired target). One-shot
	// buffs are restored if the cast then fails.
	buffsBefore := append([]types.SpellBuff(nil), state.SpellBuffs...)
	if monster != nil && spells.ShapeOf(spell) == "attack" {
		b, a, lines := buffAttackRoll(state, false)
		deps.AttackBonus = b
		deps.AttackAdvantage = a + ConditionAttackAdvantage(state.Conditions, monster.Conditions)
		preLog = append(preLog, lines...)
		if inDarkness(cs, save) {
			deps.AttackAdvantage--
			preLog = append(preLog, "  🌑 It's dark — disadvantage (no darkvision, no light in hand).")
		}
	}
	// Components must be at hand (general slots, or a pouch in one). Ones left
	// in the backpack can be dug out — it costs the bonus action.
	deps.ComponentsAtHand = true
	res, err := spells.Cast(db, deps, save, spellID, level, monster)
	dugOut := false
	if errors.Is(err, spells.ErrComponentsInPack) {
		if actionCost == "bonus_action" || state.BonusActionUsed {
			state.SpellBuffs = buffsBefore
			return nil, fmt.Errorf("the components for that are in your pack, and digging them out takes your bonus action (keep them in your general slots or a pouch)")
		}
		deps.ComponentsAtHand = false
		res, err = spells.Cast(db, deps, save, spellID, level, monster)
		dugOut = err == nil
	}
	if err != nil {
		state.SpellBuffs = buffsBefore
		return nil, err
	}

	log := append(preLog, res.Log...)
	if dugOut {
		state.BonusActionUsed = true
		log = append([]string{"  You dig the components out of your pack (bonus action)."}, log...)
	}
	if hostile {
		log = append(log, breakOnAttack(state)...)
	}

	// Spend the action economy.
	if actionCost == "bonus_action" {
		state.BonusActionUsed = true
	} else {
		state.ActionUsed = true
	}

	return append(log, castConsequences(cs, me, save, res, ally, monster, level, advancement)...), nil
}

// hostileSpell reports whether a spell is aimed at an enemy: an attack, auto-hit
// or save spell, a control spell, or a mark (hex) on a foe.
func hostileSpell(db *sql.DB, spellID string, spell map[string]interface{}) bool {
	b := combatBuffOf(spell)
	return spells.TargetsEnemy(db, spellID) || spellConditionOf(spell) != nil || (b != nil && b.Target == "enemy")
}

// castConsequences owns what a resolved cast does to the fight, shared by a
// normal cast and a scroll: damage (with on-hit buffs like hex) and its XP and
// kill, healing, cures, the spell's combat buff and condition rider, and
// concentration (a new concentration spell ends the old one first).
func castConsequences(cs *types.CombatSession, me *types.PartyCombatant, save *types.SaveFile, res *spells.CastResult, ally *types.PartyCombatant, monster *types.MonsterInstance, level int, advancement []types.AdvancementEntry) []string {
	var log []string
	if res.Concentration && me.Concentration != nil {
		old := me.Concentration.SpellName
		dropConcentration(cs, me, save, res.EffectID)
		log = append(log, fmt.Sprintf("  You stop concentrating on %s.", old))
	}

	// Damage → monster HP + XP, on the target or everyone an area spell
	// caught. A spell attack that hits carries buffs that ride spell hits (hex).
	hits := castHits(res, monster)
	xp := 0
	for i := range hits {
		h := &hits[i]
		if res.Shape == "attack" && res.Hit {
			extra, lines := buffHitDamage(cs, me, h.Monster, false, true, res.Crit, res.DamageType)
			h.Damage += extra
			log = append(log, lines...)
		}
		if h.Damage > 0 {
			applyDamageToMonster(h.Monster, h.Damage)
			xp += awardDamageXP(cs, h.Monster, h.Damage, save.TimeOfDay, level, advancement)
		}
	}
	if xp > 0 {
		log = append(log, fmt.Sprintf("  +%d XP", xp))
	}

	// Healing → combat HP pool (not save.HP; combat owns HP until it ends):
	// yours, or the ally you aimed it at. Healing the hurt earns XP.
	target := me
	if ally != nil {
		target = ally
	}
	if res.Heal > 0 {
		healed, healLog := HealMember(cs, me, target, res.Heal, save.TimeOfDay, level, advancement)
		if target != me {
			log = append(log, fmt.Sprintf("  💚 %s recovers %d HP.", target.Name, healed))
		}
		log = append(log, healLog...)
	}

	// Cures end the conditions on whoever it was cast on, and their status
	// effects (poison) with them.
	if len(res.Cures) > 0 {
		cures := make([]interface{}, len(res.Cures))
		for i, c := range res.Cures {
			cures[i] = c
		}
		cured := cureConditions(&target.CombatState.Conditions, cures)
		gaminventory.CureEffects(saveOf(target, save), res.Cures)
		for _, c := range cured {
			log = append(log, fmt.Sprintf("  %s is no longer %s.", target.Name, c))
		}
		if len(cured) == 0 && res.Heal == 0 && combatBuffOf(res.Spell) == nil {
			log = append(log, fmt.Sprintf("  Nothing ails %s.", target.Name))
		}
	}

	log = append(log, applySpellBuff(cs, me, res, ally, monster, spellDC(save, level), castingMod(save))...)

	// Control spells impose their condition on each target: on a failed save,
	// or on a hit for an attack spell (guiding bolt's glow).
	for _, h := range hits {
		if h.Monster.IsAlive {
			log = append(log, applySpellCondition(res, h.Monster, h.SaveMade, me.ID, spellDC(save, level))...)
		}
	}

	// Concentration — a maintained buff/control the caster must now hold.
	if res.Concentration {
		me.Concentration = &types.ConcentrationState{
			SpellID:   res.SpellID,
			SpellName: res.SpellName,
			EffectID:  res.EffectID,
		}
	}

	// Every kill pays out, then one check for victory.
	killed := false
	for _, h := range hits {
		if !h.Monster.IsAlive {
			log = append(log, killRewards(cs, h.Monster, save, advancement)...)
			killed = true
		}
	}
	if killed {
		log = append(log, checkVictory(cs)...)
	}
	return log
}

// castHits lists who a cast landed on: everyone an area spell caught, or the
// one target (none for a spell on yourself or an ally).
func castHits(res *spells.CastResult, monster *types.MonsterInstance) []spells.TargetOutcome {
	if len(res.Targets) > 0 {
		return append([]spells.TargetOutcome(nil), res.Targets...)
	}
	if monster == nil {
		return nil
	}
	return []spells.TargetOutcome{{Monster: monster, Damage: res.Damage, SaveMade: res.SaveMade}}
}

// applySpellCondition lands a spell's applies_condition on a target. A save
// spell lands it only when that target failed its save (saveMade), an attack
// spell only on a hit. The recurring save-to-end DC is the caster's spell save DC.
func applySpellCondition(res *spells.CastResult, monster *types.MonsterInstance, saveMade bool, casterID string, dc int) []string {
	c := spellConditionOf(res.Spell)
	if c == nil {
		return nil
	}
	if (res.Shape == "save" && saveMade) || (res.Shape == "attack" && !res.Hit) {
		return nil
	}
	if res.SaveDC > 0 {
		dc = res.SaveDC
	}
	line, _ := landCondition(monster, c, res.SpellID, casterID, dc)
	if line == "" {
		return nil
	}
	return []string{line}
}

// applySpellCover checks the player can see the spell's target and, if it has
// cover, raises its AC until the returned restore runs.
func applySpellCover(cs *types.CombatSession, monster *types.MonsterInstance) (func(), []string, error) {
	visible, cover := Sight(cs, actor(cs).Pos, monster.Pos)
	if !visible {
		return nil, nil, fmt.Errorf("you have no line of sight to %s", monster.Name)
	}
	if cover == 0 {
		return func() {}, nil, nil
	}
	monster.ArmorClass += cover
	return func() { monster.ArmorClass -= cover },
		[]string{fmt.Sprintf("  🛡 %s has %s (+%d AC).", monster.Name, coverLabel(cover), cover)}, nil
}

// checkConcentrationOnDamage runs a Constitution save (DC = max(10, ½ damage))
// to keep a maintained spell after the player takes damage. On a failure the
// linked ActiveEffect is dropped and concentration clears. Returns log lines.
// Lives here (not combat.go) so the effects import stays local to the cast path.
func checkConcentrationOnDamage(cs *types.CombatSession, save *types.SaveFile, dmg int) []string {
	if cs == nil || actor(cs).Concentration == nil || dmg <= 0 {
		return nil
	}
	me := actor(cs)
	name := me.Concentration.SpellName
	// Dropping unconscious ends it outright.
	if me.CombatState.IsUnconscious {
		dropConcentration(cs, me, save, "")
		return []string{fmt.Sprintf("  ✘ Your concentration on %s breaks as you fall.", name)}
	}
	dc := 10
	if half := dmg / 2; half > dc {
		dc = half
	}
	conMod := character.AbilityMod(character.AbilityScore(effectiveStats(save), "constitution"))
	roll := RollD20()
	advNote := ""
	if character.HasFeat(save, "war-caster") { // advantage on concentration saves
		r, r1, r2 := RollAdvantage()
		roll = r
		advNote = fmt.Sprintf(" [War Caster adv %d/%d]", r1, r2)
	}
	// It's a saving throw, so bless and resistance help.
	if b := buffSaveBonus(&me.CombatState); b > 0 {
		conMod += b
		advNote += fmt.Sprintf(" [+%d spell]", b)
	}
	if roll+conMod >= dc {
		return []string{fmt.Sprintf("  You hold concentration on %s (CON save %d%s vs DC %d%s).",
			name, roll, formatModifier(conMod), dc, advNote)}
	}
	dropConcentration(cs, me, save, "")
	return []string{fmt.Sprintf("  ✘ You lose concentration on %s! (CON save %d%s vs DC %d)",
		name, roll, formatModifier(conMod), dc)}
}

// ─── Combat consumables (M4 Phase C) ─────────────────────────────────────────

// ProcessPlayerUseItem drinks a potion / eats food / uses a consumable mid-fight.
// Consumables are reached from the general slots — either loose in a slot or
// stashed in a pouch/sack container occupying one (the equipped backpack itself
// isn't reachable in a fight). Healing and mana route through the shared item
// effect path, bridged onto the combat HP pool so the heal lands on the live
// combatant rather than the resting save HP. Uses the player's action.
// targetID aims a spell scroll ("" = nearest); other consumables ignore it.
func ProcessPlayerUseItem(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, itemID, targetID string) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot use an item: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	state := &actor(cs).CombatState
	if state.ActionUsed {
		return nil, fmt.Errorf("action already used this turn")
	}

	item, err := gamedata.LoadItemByID(db, itemID)
	if err != nil {
		return nil, fmt.Errorf("unknown item %q", itemID)
	}
	name := itemID
	if n, _ := item["name"].(string); n != "" {
		name = n
	}

	// A spell scroll casts the spell it carries (bypassing prepared/known/components).
	if spellID, _ := item["spell_id"].(string); spellID != "" {
		return processScrollUse(db, cs, save, itemID, spellID, name, targetID)
	}

	if !hasTag(item["tags"], "consumable") {
		return nil, fmt.Errorf("%s can't be used in combat", name)
	}

	// Locate a stack of the item at hand (loose general slot or a general-slot
	// container) — or in the backpack, which costs the bonus action to dig out.
	slot, fromPack, err := reachItem(save.Inventory, state, itemID, name)
	if err != nil {
		return nil, err
	}

	// Who it's for: you, or an ally standing next to you (a potion to the lips
	// of a fallen friend gets them back up).
	me := actor(cs)
	target, targetSave := me, save
	if ally := FindMember(cs, targetID); ally != nil && ally != me {
		if ally.Fled {
			return nil, fmt.Errorf("%s has left the fight", ally.Name)
		}
		if chebyshev(me.Pos, ally.Pos) > 1 {
			return nil, fmt.Errorf("move next to %s to hand them that", ally.Name)
		}
		target, targetSave = ally, saveOf(ally, save)
	}
	tstate := &target.CombatState

	// Bridge: point save.HP at the combat HP pool so ApplyItemEffects (which heals
	// save.HP and clamps to MaxHP) mends the live combatant. MaxHP is identical in
	// both (combat snapshots save.MaxHP), so the clamp is correct.
	before := tstate.CurrentHP
	prevHP := targetSave.HP
	targetSave.HP = tstate.CurrentHP
	msgs := gaminventory.ApplyItemEffects(targetSave, itemID)
	tstate.CurrentHP = targetSave.HP
	targetSave.HP = prevHP // combat owns HP until the fight ends
	// A coating applied mid-fight takes effect at once.
	for _, b := range takeCoatings(targetSave) {
		addBuff(tstate, b)
	}

	// Consume one from the located stack (a reusable tool, like a whetstone, stays).
	if reusable, _ := item["reusable"].(bool); !reusable {
		decrementSlotStack(slot)
	}
	state.ActionUsed = true
	if fromPack {
		state.BonusActionUsed = true
	}

	// A smoke bomb ("escape": true) gets you out, no roll.
	if escape, _ := item["escape"].(bool); escape {
		return escapeFight(cs, fmt.Sprintf("  You smash the %s at your feet and slip away in the smoke!", name)), nil
	}
	// An antidote ("cures": ["poisoned"]) ends those conditions in the fight
	// too (ApplyItemEffects above already lifted the matching status effects).
	cured := cureConditions(&tstate.Conditions, item["cures"])

	line := fmt.Sprintf("  You use %s.", name)
	if target != me {
		line = fmt.Sprintf("  %s gives %s a %s.", me.Name, target.Name, name)
	} else if PartyFight(cs) {
		line = fmt.Sprintf("  %s uses %s.", me.Name, name)
	}
	if len(msgs) > 0 {
		line = strings.TrimSuffix(line, ".") + " — " + strings.Join(msgs, ", ") + "."
	}
	log := []string{line}
	if fromPack {
		log = append([]string{fmt.Sprintf("  You dig the %s out of your pack (bonus action).", name)}, log...)
	}
	for _, c := range cured {
		log = append(log, fmt.Sprintf("  %s is no longer %s.", target.Name, c))
	}

	// Healing the hurt wakes the fallen and earns the healer XP.
	if healed := tstate.CurrentHP - before; healed > 0 {
		if tstate.IsUnconscious {
			tstate.IsUnconscious = false
			tstate.DeathSaveFailures, tstate.DeathSaveSuccesses = 0, 0
			log = append(log, fmt.Sprintf("  ✨ %s is back on their feet!", target.Name))
		}
		adv, _ := character.LoadAdvancement(db)
		level := character.GetLevelFromXP(save.Experience, adv)
		if xp := awardHealXP(cs, me, target, healed, save.TimeOfDay, level, adv); xp > 0 {
			log = append(log, fmt.Sprintf("  +%d XP (healing)", xp))
		}
	}
	return log, nil
}

// reachItem finds an item to use mid-fight. At hand means the four general
// slots and containers in them (a pouch, a sack); the backpack can be reached
// too, but rummaging in it takes the bonus action. fromPack reports which.
func reachItem(inv map[string]interface{}, state *types.PlayerCombatState, itemID, name string) (slot map[string]interface{}, fromPack bool, err error) {
	if slot = findReachableConsumable(inv, itemID); slot != nil {
		return slot, false, nil
	}
	if slot = findPackItem(inv, itemID); slot == nil {
		return nil, false, fmt.Errorf("you aren't carrying a %s", name)
	}
	if state.BonusActionUsed {
		return nil, false, fmt.Errorf("your %s is in your pack, and digging it out takes your bonus action (keep it in your general slots or a pouch)", name)
	}
	return slot, true, nil
}

// findPackItem finds a stack of itemID in the equipped backpack.
func findPackItem(inv map[string]interface{}, itemID string) map[string]interface{} {
	gear, _ := inv["gear_slots"].(map[string]interface{})
	bag, _ := gear["bag"].(map[string]interface{})
	contents, _ := bag["contents"].([]interface{})
	for _, raw := range contents {
		if s, ok := raw.(map[string]interface{}); ok {
			if id, _ := s["item"].(string); id == itemID && slotQty(s, "quantity") > 0 {
				return s
			}
		}
	}
	return nil
}

// cureConditions removes the conditions an item cures (its "cures" list) and
// returns the ones that were actually there.
func cureConditions(conds *[]types.CombatCondition, cures interface{}) []string {
	list, _ := cures.([]interface{})
	if len(list) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, c := range list {
		if s, ok := c.(string); ok {
			want[strings.ToLower(s)] = true
		}
	}
	var kept []types.CombatCondition
	var cured []string
	for _, c := range *conds {
		if want[strings.ToLower(c.Name)] {
			cured = append(cured, c.Name)
			continue
		}
		kept = append(kept, c)
	}
	*conds = kept
	return cured
}

// processScrollUse resolves a spell scroll in combat: it casts the scroll's spell
// at the target (bypassing prepared/known/components; mana still applies), applies
// the same consequences as a normal cast, then consumes one scroll. Uses the action.
func processScrollUse(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, itemID, spellID, itemName, targetID string) ([]string, error) {
	state := &actor(cs).CombatState
	slot, fromPack, err := reachItem(save.Inventory, state, itemID, itemName)
	if err != nil {
		return nil, err
	}
	monster, err := ResolveTarget(cs, targetID)
	if err != nil {
		return nil, err
	}
	spell, err := gamedata.LoadSpellByID(db, spellID)
	if err != nil {
		return nil, err
	}
	if hostileSpell(db, spellID, spell) {
		if err := checkSpellReach(spell, itemName, actor(cs).Pos, monster.Pos, monster.Name); err != nil {
			return nil, err
		}
	}
	monster, caught, err := aimArea(cs, actor(cs), spell, itemName, monster)
	if err != nil {
		return nil, err
	}
	adv, _ := character.LoadAdvancement(db)
	level := character.GetLevelFromXP(save.Experience, adv)

	var coverLog []string
	if spells.TargetsEnemy(db, spellID) && !selfCentred(spell) {
		restore, lines, err := applySpellCover(cs, monster)
		if err != nil {
			return nil, err
		}
		defer restore()
		coverLog = lines
	}
	deps := spells.Deps{RollD20: RollD20, RollDice: RollDice, ResolveMonsterDamage: ResolveDamageToMonster, InCombat: true, Area: caught}
	res, err := spells.CastFromScroll(db, deps, save, spellID, level, monster)
	if err != nil {
		return nil, err
	}

	log := append([]string{fmt.Sprintf("  You read the %s.", itemName)}, coverLog...)
	log = append(log, res.Log...)
	decrementSlotStack(slot)
	state.ActionUsed = true
	if fromPack {
		state.BonusActionUsed = true
		log = append([]string{fmt.Sprintf("  You dig the %s out of your pack (bonus action).", itemName)}, log...)
	}
	return append(log, castConsequences(cs, actor(cs), save, res, nil, monster, level, adv)...), nil
}

// findReachableConsumable returns the slot map (a live reference) holding itemID,
// searching loose general slots first, then the contents of any general-slot
// container. Returns nil if none is at hand. The equipped backpack is excluded
// on purpose — rummaging the pack mid-fight costs the bonus action (reachItem).
func findReachableConsumable(inv map[string]interface{}, itemID string) map[string]interface{} {
	gen, ok := inv["general_slots"].([]interface{})
	if !ok {
		return nil
	}
	for _, raw := range gen {
		slot, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := slot["item"].(string); id == itemID && slotQty(slot, "quantity") > 0 {
			return slot
		}
	}
	// Second pass: look inside general-slot containers (component pouch, sack, …).
	for _, raw := range gen {
		slot, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		contents, ok := slot["contents"].([]interface{})
		if !ok {
			continue
		}
		for _, craw := range contents {
			cslot, ok := craw.(map[string]interface{})
			if !ok {
				continue
			}
			if id, _ := cslot["item"].(string); id == itemID && slotQty(cslot, "quantity") > 0 {
				return cslot
			}
		}
	}
	return nil
}

// decrementSlotStack removes one unit from a slot, clearing it to the empty shape
// ({item:null, quantity:0}) when the last one is consumed.
func decrementSlotStack(slot map[string]interface{}) {
	q := slotQty(slot, "quantity")
	if q <= 1 {
		slot["item"] = nil
		slot["quantity"] = 0
		return
	}
	slot["quantity"] = q - 1
}

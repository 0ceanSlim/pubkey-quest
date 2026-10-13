package combat

import (
	"database/sql"
	"fmt"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
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
	// A spell can be aimed at an ally (heals, buffs) or an enemy.
	ally := FindMember(cs, targetID)
	var monster *types.MonsterInstance
	if ally != nil {
		if ally.Fled {
			return nil, fmt.Errorf("%s has left the fight", ally.Name)
		}
		if spells.TargetsEnemy(db, spellID) {
			return nil, fmt.Errorf("aim that spell at an enemy")
		}
	} else {
		m, err := ResolveTarget(cs, targetID)
		if err != nil {
			return nil, err
		}
		monster = m
	}
	state := &actor(cs).CombatState
	if IsIncapacitated(state.Conditions) {
		return nil, fmt.Errorf("you are incapacitated and can't cast")
	}

	// Action economy — validated up front, before the engine spends any cost.
	// A spell's combat cost is its action_cost ("action" | "bonus_action");
	// anything longer than an action can't be cast as a combat turn.
	actionCost := spellActionCost(db, spellID)
	switch actionCost {
	case "bonus_action":
		if state.BonusActionUsed {
			return nil, fmt.Errorf("bonus action already used this turn")
		}
	case "action":
		if state.ActionUsed {
			return nil, fmt.Errorf("action already used this turn")
		}
	default:
		return nil, fmt.Errorf("%s takes too long to cast in combat", spellID)
	}

	level := character.GetLevelFromXP(save.Experience, advancement)

	deps := spells.Deps{
		RollD20:              RollD20,
		RollDice:             RollDice,
		ResolveMonsterDamage: ResolveDamageToMonster,
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
	// Offensive spells need a line to the target; cover raises its AC against
	// spell attacks for the duration of the cast.
	var coverLog []string
	if monster != nil && spells.TargetsEnemy(db, spellID) {
		restore, lines, err := applySpellCover(cs, monster)
		if err != nil {
			return nil, err
		}
		defer restore()
		coverLog = lines
	}
	res, err := spells.Cast(db, deps, save, spellID, level, monster)
	if err != nil {
		return nil, err
	}

	log := append(coverLog, res.Log...)

	// Damage → monster HP + XP + kill.
	if res.Damage > 0 && monster != nil {
		applyDamageToMonster(monster, res.Damage)
		if xp := awardDamageXP(cs, monster, res.Damage, save.TimeOfDay, level, advancement); xp > 0 {
			log = append(log, fmt.Sprintf("  +%d XP", xp))
		}
	}

	// Healing → combat HP pool (not save.HP; combat owns HP until it ends):
	// yours, or the ally you aimed it at. Healing the hurt earns XP.
	if res.Heal > 0 {
		target := actor(cs)
		if ally != nil {
			target = ally
		}
		healed, healLog := HealMember(cs, actor(cs), target, res.Heal, save.TimeOfDay, level, advancement)
		if ally != nil && ally != actor(cs) {
			log = append(log, fmt.Sprintf("  💚 %s recovers %d HP.", ally.Name, healed))
		}
		log = append(log, healLog...)
	}

	// Control spells impose their condition on the target on a failed save
	// (M5 §15): entangle → restrained, faerie-fire → outlined, …
	if monster != nil && monster.IsAlive {
		log = append(log, applySpellCondition(spellID, res, monster)...)
	}

	// Concentration — a maintained buff/control the player must now hold.
	if res.Concentration {
		actor(cs).Concentration = &types.ConcentrationState{
			SpellID:   res.SpellID,
			SpellName: res.SpellName,
			EffectID:  res.EffectID,
		}
	}

	// Spend the action economy.
	if actionCost == "bonus_action" {
		state.BonusActionUsed = true
	} else {
		state.ActionUsed = true
	}

	if monster != nil && !monster.IsAlive {
		log = append(log, handleMonsterKill(cs, monster, save, advancement)...)
	}

	return log, nil
}

// spellConditionRider maps a control spell to the condition it imposes. saveStat
// != "" means the condition re-saves each turn on that stat (entangle's STR check
// to break free); "" means it lasts its full duration with no re-save (faerie-fire
// is decided by the DEX save at cast time). More riders plug in here as the M5
// conditions land more spells (docs/draft/spell-mechanics-proposals.md).
type spellConditionRider struct {
	condition string
	saveStat  string
	rounds    int
}

var spellConditionRiders = map[string]spellConditionRider{
	"entangle":     {condition: "restrained", saveStat: "strength", rounds: 10}, // STR save each turn to break free
	"faerie-fire":  {condition: "outlined", saveStat: "", rounds: 10},           // DEX save at cast; no re-save
	"command":      {condition: "prone", saveStat: "", rounds: 1},               // WIS save at cast; "Grovel" → prone for 1 round
	"color-spray":  {condition: "blinded", saveStat: "", rounds: 1},             // no save (HP-pool auto-blind), 1 round
	"sleep":        {condition: "unconscious", saveStat: "wisdom", rounds: 5},   // WIS save each turn to wake
	"charm-person": {condition: "charmed", saveStat: "", rounds: 10},            // WIS save at cast; no re-save, won't attack you
}

// applySpellCondition applies a control spell's condition to the target. Save-
// shaped spells land it only on a FAILED save (SaveMade == false); the recurring
// save-to-end DC is the spell's own save DC. Returns a log line, or nil.
func applySpellCondition(spellID string, res *spells.CastResult, monster *types.MonsterInstance) []string {
	rider, ok := spellConditionRiders[spellID]
	if !ok {
		return nil
	}
	if res.Shape == "save" && res.SaveMade {
		return nil // target resisted the spell
	}
	dc := res.SaveDC
	if dc <= 0 {
		dc = 10
	}
	saveDC := 0
	if rider.saveStat != "" {
		saveDC = dc
	}
	ApplyCondition(&monster.Conditions, types.CombatCondition{
		Name:           rider.condition,
		DurationRounds: rider.rounds,
		SaveDC:         saveDC,
		SaveStat:       rider.saveStat,
	})
	return []string{fmt.Sprintf("  %s is %s!", monster.Name, rider.condition)}
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

// spellActionCost reads a spell's combat action cost, defaulting to "action".
func spellActionCost(db *sql.DB, spellID string) string {
	spell, err := gamedata.LoadSpellByID(db, spellID)
	if err != nil {
		return "action"
	}
	if ac, _ := spell["action_cost"].(string); ac != "" {
		return ac
	}
	return "action"
}

// applyHealToPlayer adds healing to the combat HP pool, capped at MaxHP.
func applyHealToPlayer(cs *types.CombatSession, heal int) {
	if len(cs.Party) == 0 {
		return
	}
	st := &actor(cs).CombatState
	st.CurrentHP += heal
	if st.CurrentHP > st.MaxHP {
		st.CurrentHP = st.MaxHP
	}
}

// checkConcentrationOnDamage runs a Constitution save (DC = max(10, ½ damage))
// to keep a maintained spell after the player takes damage. On a failure the
// linked ActiveEffect is dropped and concentration clears. Returns log lines.
// Lives here (not combat.go) so the effects import stays local to the cast path.
func checkConcentrationOnDamage(cs *types.CombatSession, save *types.SaveFile, dmg int) []string {
	if cs == nil || actor(cs).Concentration == nil || dmg <= 0 {
		return nil
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
	if roll+conMod >= dc {
		return []string{fmt.Sprintf("  You hold concentration on %s (CON save %d%s vs DC %d%s).",
			actor(cs).Concentration.SpellName, roll, formatModifier(conMod), dc, advNote)}
	}
	name := actor(cs).Concentration.SpellName
	if actor(cs).Concentration.EffectID != "" {
		effects.RemoveEffect(save, actor(cs).Concentration.EffectID)
	}
	actor(cs).Concentration = nil
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

	// Locate a stack of the item within reach (loose general slot or a general-
	// slot container). Nil means the player isn't carrying one where they can grab
	// it mid-fight.
	slot := findReachableConsumable(save.Inventory, itemID)
	if slot == nil {
		return nil, fmt.Errorf("no %s within reach", name)
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

	// Consume one from the located stack.
	decrementSlotStack(slot)
	state.ActionUsed = true

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
	slot := findReachableConsumable(save.Inventory, itemID)
	if slot == nil {
		return nil, fmt.Errorf("no %s within reach", itemName)
	}
	monster, err := ResolveTarget(cs, targetID)
	if err != nil {
		return nil, err
	}
	adv, _ := character.LoadAdvancement(db)
	level := character.GetLevelFromXP(save.Experience, adv)

	var coverLog []string
	if spells.TargetsEnemy(db, spellID) {
		restore, lines, err := applySpellCover(cs, monster)
		if err != nil {
			return nil, err
		}
		defer restore()
		coverLog = lines
	}
	deps := spells.Deps{RollD20: RollD20, RollDice: RollDice, ResolveMonsterDamage: ResolveDamageToMonster}
	res, err := spells.CastFromScroll(db, deps, save, spellID, level, monster)
	if err != nil {
		return nil, err
	}

	log := append([]string{fmt.Sprintf("  You read the %s.", itemName)}, coverLog...)
	log = append(log, res.Log...)
	if res.Damage > 0 {
		applyDamageToMonster(monster, res.Damage)
		if xp := awardDamageXP(cs, monster, res.Damage, save.TimeOfDay, level, adv); xp > 0 {
			log = append(log, fmt.Sprintf("  +%d XP", xp))
		}
	}
	if res.Heal > 0 {
		applyHealToPlayer(cs, res.Heal)
	}
	if monster.IsAlive {
		log = append(log, applySpellCondition(res.SpellID, res, monster)...)
	}
	if res.Concentration {
		actor(cs).Concentration = &types.ConcentrationState{SpellID: res.SpellID, SpellName: res.SpellName, EffectID: res.EffectID}
	}

	decrementSlotStack(slot)
	state.ActionUsed = true

	if !monster.IsAlive {
		log = append(log, handleMonsterKill(cs, monster, save, adv)...)
	}
	return log, nil
}

// findReachableConsumable returns the slot map (a live reference) holding itemID,
// searching loose general slots first, then the contents of any general-slot
// container. Returns nil if none is reachable. The equipped backpack is excluded
// on purpose — you can't rummage the pack mid-fight.
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

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

// ProcessPlayerCast resolves the player casting spellID at the (single) monster.
// Does NOT run the monster's response — the caller ends the turn like any action.
func ProcessPlayerCast(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, spellID string, advancement []types.AdvancementEntry) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot cast: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	if len(cs.Monsters) == 0 || !cs.Monsters[0].IsAlive {
		return nil, fmt.Errorf("no living target to cast at")
	}
	state := &cs.Party[0].CombatState
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

	monster := &cs.Monsters[0]
	level := character.GetLevelFromXP(save.Experience, advancement)

	deps := spells.Deps{
		RollD20:              RollD20,
		RollDice:             RollDice,
		ResolveMonsterDamage: ResolveDamageToMonster,
	}
	res, err := spells.Cast(db, deps, save, spellID, level, monster)
	if err != nil {
		return nil, err
	}

	log := res.Log

	// Damage → monster HP + XP + kill.
	if res.Damage > 0 {
		applyDamageToMonster(monster, res.Damage)
		if xp := awardDamageXP(cs, monster, res.Damage, save.TimeOfDay, level, advancement); xp > 0 {
			log = append(log, fmt.Sprintf("  +%d XP", xp))
		}
	}

	// Healing → combat HP pool (not save.HP; combat owns HP until it ends).
	if res.Heal > 0 {
		applyHealToPlayer(cs, res.Heal)
	}

	// Control spells impose their condition on the target on a failed save
	// (M5 §15): entangle → restrained, faerie-fire → outlined, …
	if monster.IsAlive {
		log = append(log, applySpellCondition(spellID, res, monster)...)
	}

	// Concentration — a maintained buff/control the player must now hold.
	if res.Concentration {
		cs.Concentration = &types.ConcentrationState{
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

	if !monster.IsAlive {
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
	"entangle":    {condition: "restrained", saveStat: "strength", rounds: 10}, // STR save each turn to break free
	"faerie-fire": {condition: "outlined", saveStat: "", rounds: 10},           // DEX save at cast; no re-save
	"command":     {condition: "prone", saveStat: "", rounds: 1},               // WIS save at cast; "Grovel" → prone for 1 round
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
	st := &cs.Party[0].CombatState
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
	if cs == nil || cs.Concentration == nil || dmg <= 0 {
		return nil
	}
	dc := 10
	if half := dmg / 2; half > dc {
		dc = half
	}
	conMod := character.AbilityMod(character.AbilityScore(save.Stats, "constitution"))
	roll := RollD20()
	if roll+conMod >= dc {
		return []string{fmt.Sprintf("  You hold concentration on %s (CON save %d%s vs DC %d).",
			cs.Concentration.SpellName, roll, formatModifier(conMod), dc)}
	}
	name := cs.Concentration.SpellName
	if cs.Concentration.EffectID != "" {
		effects.RemoveEffect(save, cs.Concentration.EffectID)
	}
	cs.Concentration = nil
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
func ProcessPlayerUseItem(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, itemID string) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot use an item: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	state := &cs.Party[0].CombatState
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

	// Bridge: point save.HP at the combat HP pool so ApplyItemEffects (which heals
	// save.HP and clamps to MaxHP) mends the live combatant. MaxHP is identical in
	// both (combat snapshots save.MaxHP), so the clamp is correct.
	prevHP := save.HP
	save.HP = state.CurrentHP
	msgs := gaminventory.ApplyItemEffects(save, itemID)
	state.CurrentHP = save.HP
	save.HP = prevHP // combat owns HP until the fight ends

	// Consume one from the located stack.
	decrementSlotStack(slot)
	state.ActionUsed = true

	line := fmt.Sprintf("  You use %s.", name)
	if len(msgs) > 0 {
		line = fmt.Sprintf("  You use %s — %s.", name, strings.Join(msgs, ", "))
	}
	return []string{line}, nil
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

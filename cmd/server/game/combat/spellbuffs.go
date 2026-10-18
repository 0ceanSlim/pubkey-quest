package combat

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/spells"
	"pubkey-quest/types"
)

// ─── Spell buffs, riders and reactions in combat ─────────────────────────────
//
// The casting engine (game/spells) spends a spell's costs and resolves its
// damage, healing and lasting effect. Everything a spell does to the fight
// itself is data on the spell, applied here:
//
//   combat_buff        → a types.SpellBuff on the caster, an ally, the party, or
//                        on the caster marking a foe (bless, shield, smites, hex)
//   applies_condition  → a condition on the target (entangle, command, hold person)
//   action_cost        → "reaction" spells aren't cast on your turn: kept
//                        prepared, they fire on their trigger (shield on a hit
//                        it would turn aside, hellish rebuke when you're hurt)
//
// Buffs live on PlayerCombatState.SpellBuffs; a concentration spell's buffs and
// conditions end when the concentration does (dropConcentration).

// combatBuffOf reads a spell's combat_buff block, or nil if it has none.
func combatBuffOf(spell map[string]interface{}) *types.SpellBuff {
	raw, ok := spell["combat_buff"].(map[string]interface{})
	if !ok {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var buff types.SpellBuff
	if json.Unmarshal(b, &buff) != nil {
		return nil
	}
	return &buff
}

// spellConditionOf reads a spell's applies_condition block, or nil.
func spellConditionOf(spell map[string]interface{}) *types.SpellCondition {
	raw, ok := spell["applies_condition"].(map[string]interface{})
	if !ok {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var c types.SpellCondition
	if json.Unmarshal(b, &c) != nil || c.Condition == "" {
		return nil
	}
	return &c
}

// castingMod is the caster's spellcasting ability modifier (0 for non-casters).
func castingMod(save *types.SaveFile) int {
	ability, ok := character.SpellcastingAbility(save.Class)
	if !ok {
		return 0
	}
	return character.AbilityMod(character.AbilityScore(effectiveStats(save), ability))
}

// spellDC is the caster's spell save DC: 8 + proficiency + casting modifier.
func spellDC(save *types.SaveFile, level int) int {
	return 8 + character.ProficiencyBonus(level) + castingMod(save)
}

// buffTargets picks who a buff lands on: the caster, the chosen ally, or (for
// "party") the caster and the two nearest standing allies.
func buffTargets(cs *types.CombatSession, caster *types.PartyCombatant, b *types.SpellBuff, ally *types.PartyCombatant) []*types.PartyCombatant {
	switch b.Target {
	case "ally":
		if ally != nil {
			return []*types.PartyCombatant{ally}
		}
	case "party":
		var others []*types.PartyCombatant
		for _, p := range standingMembers(cs) {
			if p != caster {
				others = append(others, p)
			}
		}
		sort.SliceStable(others, func(i, j int) bool {
			return chebyshev(caster.Pos, others[i].Pos) < chebyshev(caster.Pos, others[j].Pos)
		})
		if len(others) > 2 {
			others = others[:2]
		}
		return append([]*types.PartyCombatant{caster}, others...)
	}
	return []*types.PartyCombatant{caster}
}

// applySpellBuff lands a cast spell's combat_buff. monster is the foe a mark
// ("enemy" target) goes on; ally the friend an "ally" buff goes on (nil = the
// caster). dc and mod are the caster's spell save DC and casting modifier.
func applySpellBuff(cs *types.CombatSession, caster *types.PartyCombatant, res *spells.CastResult, ally *types.PartyCombatant, monster *types.MonsterInstance, dc, mod int) []string {
	def := combatBuffOf(res.Spell)
	if def == nil {
		return nil
	}
	def.SpellID, def.Name, def.CasterID, def.SaveDC = res.SpellID, res.SpellName, caster.ID, dc
	if def.Target == "enemy" && monster != nil {
		def.MarkedID = monster.InstanceID
	}
	if def.TempHPEachTurn > 0 {
		def.TempHPEachTurn = max(1, mod)
	}

	var log []string
	for _, t := range buffTargets(cs, caster, def, ally) {
		b := *def
		st := &t.CombatState
		who := t.Name
		if t == caster && !PartyFight(cs) {
			who = "You"
		}
		if b.Revive && st.IsUnconscious {
			reviveAtOneHP(st)
			log = append(log, fmt.Sprintf("  ✨ %s draws breath and gets back up at 1 HP!", t.Name))
		}
		if b.TempHP != "" {
			if n := RollDice(b.TempHP, false); n > st.TempHP {
				st.TempHP = n
				log = append(log, fmt.Sprintf("  ✨ %s %s %d temporary HP.", who, verb(who, "gain", "gains"), n))
			}
		}
		for _, c := range b.Immune {
			if HasCondition(st.Conditions, c) {
				RemoveCondition(&st.Conditions, c)
				log = append(log, fmt.Sprintf("  %s %s no longer %s.", who, verb(who, "are", "is"), c))
			}
		}
		if hasBuffMechanics(&b) {
			addBuff(st, b)
			if line := buffSummary(&b, monster); line != "" {
				log = append(log, fmt.Sprintf("  ✨ %s: %s.", who, line))
			}
		}
	}
	return log
}

// verb picks the second-person form for "You" and the third-person otherwise.
func verb(who, you, other string) string {
	if who == "You" {
		return you
	}
	return other
}

// addBuff puts a buff on a combatant, replacing the same spell's earlier one.
func addBuff(st *types.PlayerCombatState, b types.SpellBuff) {
	out := st.SpellBuffs[:0]
	for _, old := range st.SpellBuffs {
		if old.SpellID != b.SpellID || old.CasterID != b.CasterID {
			out = append(out, old)
		}
	}
	st.SpellBuffs = append(out, b)
}

// removeBuffs drops every buff matching drop, returning their names.
func removeBuffs(st *types.PlayerCombatState, drop func(*types.SpellBuff) bool) []string {
	var names []string
	out := st.SpellBuffs[:0]
	for i := range st.SpellBuffs {
		if drop(&st.SpellBuffs[i]) {
			names = append(names, st.SpellBuffs[i].Name)
			continue
		}
		out = append(out, st.SpellBuffs[i])
	}
	st.SpellBuffs = out
	return names
}

// hasBuffMechanics reports whether a buff still does anything (one-shot parts
// like a smite's damage or true strike's advantage are cleared once spent).
func hasBuffMechanics(b *types.SpellBuff) bool {
	return b.ACBonus != 0 || b.ACFloor != 0 || b.AttackBonus != 0 || b.AttackDice != "" || b.SaveDice != "" ||
		b.HitDice != "" || b.HitBonus != 0 || b.HitCondition != nil ||
		len(b.Resist) > 0 || b.AttackersDisadvantage || b.RangedDefense || len(b.WardTypes) > 0 ||
		len(b.Immune) > 0 || b.Decoys > 0 || b.Sanctuary || b.DeathWard || b.EasyEscape ||
		b.TempHPEachTurn > 0 || b.Retaliate != "" || b.NextAttackAdvantage ||
		b.MeleeStat != "" || b.WeaponDice != "" || b.ExtraMove != 0
}

// dropSpent clears buffs whose last one-shot part has been used.
func dropSpent(st *types.PlayerCombatState) {
	removeBuffs(st, func(b *types.SpellBuff) bool { return !hasBuffMechanics(b) })
}

// DescribeBuff says what a buff does, for the combat HUD.
func DescribeBuff(cs *types.CombatSession, b *types.SpellBuff) string {
	var marked *types.MonsterInstance
	if b.MarkedID != "" {
		marked = FindMonster(cs, b.MarkedID)
	}
	return buffSummary(b, marked)
}

// buffSummary is a short line for the log saying what a buff does.
func buffSummary(b *types.SpellBuff, marked *types.MonsterInstance) string {
	var parts []string
	add := func(format string, a ...interface{}) { parts = append(parts, fmt.Sprintf(format, a...)) }
	if b.ACBonus != 0 {
		add("%+d AC", b.ACBonus)
	}
	if b.ACFloor != 0 {
		add("AC at least %d", b.ACFloor)
	}
	switch {
	case b.AttackDice != "" && b.SaveDice != "":
		add("+%s to attacks and saves", b.AttackDice)
	case b.AttackDice != "":
		add("+%s to attacks", b.AttackDice)
	case b.SaveDice != "":
		add("+%s to saving throws", b.SaveDice)
	}
	if b.AttackBonus != 0 {
		add("%+d to weapon attacks", b.AttackBonus)
	}
	if b.HitDice != "" || b.HitBonus != 0 {
		dmg := b.HitDice
		if dmg == "" {
			dmg = fmt.Sprintf("%d", b.HitBonus)
		} else if b.HitBonus != 0 {
			dmg = fmt.Sprintf("%s%+d", b.HitDice, b.HitBonus)
		}
		if b.HitType != "" {
			dmg += " " + b.HitType
		}
		when := "on every hit"
		if b.HitOnce {
			when = "on the next hit"
		}
		if b.MarkedID != "" && marked != nil {
			when += " against " + marked.Name
		}
		add("%s damage %s", dmg, when)
	} else if b.HitCondition != nil {
		add("the next hit can leave the target %s", b.HitCondition.Condition)
	}
	if len(b.Resist) > 0 {
		add("resists %s damage", strings.Join(b.Resist, ", "))
	}
	if b.AttackersDisadvantage {
		add("attacks against you have disadvantage")
	}
	if b.RangedDefense {
		add("ranged attacks against you have disadvantage")
	}
	if len(b.WardTypes) > 0 {
		add("warded against %s", strings.Join(b.WardTypes, ", "))
	}
	if b.Decoys > 0 {
		add("%d illusory double(s)", b.Decoys)
	}
	if b.Sanctuary {
		add("foes must pass a Wisdom save (DC %d) to attack", b.SaveDC)
	}
	if b.DeathWard {
		add("the next blow that would drop you leaves you at 1 HP")
	}
	if b.EasyEscape {
		add("you can slip away at will")
	}
	if b.TempHPEachTurn > 0 {
		add("%d temporary HP each turn", b.TempHPEachTurn)
	}
	if b.Retaliate != "" {
		add("melee attackers that hit take %s %s", b.Retaliate, b.RetaliateType)
	}
	if b.NextAttackAdvantage {
		add("advantage on the next attack")
	}
	if b.MeleeStat != "" {
		add("melee attacks use your spellcasting ability")
	}
	if b.WeaponDice != "" {
		add("weapon damage becomes %s", b.WeaponDice)
	}
	if b.ExtraMove != 0 {
		add("%+d movement", b.ExtraMove)
	}
	if b.Rounds > 0 {
		add("%d round(s)", b.Rounds)
	}
	return strings.Join(parts, ", ")
}

// ─── Hooks the turn loop calls ───────────────────────────────────────────────

// tickBuffs runs at the start of the holder's turn: timed buffs count down and
// expire, and heroism's temporary hit points come back.
func tickBuffs(st *types.PlayerCombatState) {
	for i := range st.SpellBuffs {
		b := &st.SpellBuffs[i]
		if b.TempHPEachTurn > st.TempHP {
			st.TempHP = b.TempHPEachTurn
		}
		if b.Rounds > 0 {
			b.Rounds--
			if b.Rounds == 0 {
				b.Rounds = -1 // expired; swept below
			}
		}
	}
	removeBuffs(st, func(b *types.SpellBuff) bool { return b.Rounds < 0 })
}

// buffMovement is the extra grid cells a combatant's buffs give each turn.
func buffMovement(st *types.PlayerCombatState) int {
	n := 0
	for _, b := range st.SpellBuffs {
		n += b.ExtraMove
	}
	return n
}

// buffedAC adds in-fight spell buffs to a combatant's AC.
func buffedAC(st *types.PlayerCombatState, ac int) int {
	floor := 0
	for _, b := range st.SpellBuffs {
		ac += b.ACBonus
		floor = max(floor, b.ACFloor)
	}
	return max(ac, floor)
}

// buffAttackRoll is what the holder's buffs add to an attack roll: flat bonuses
// (weapon attacks only), rolled dice (bless) and advantage (true strike, spent).
func buffAttackRoll(st *types.PlayerCombatState, weapon bool) (bonus, adv int, log []string) {
	for i := range st.SpellBuffs {
		b := &st.SpellBuffs[i]
		if weapon && b.AttackBonus != 0 {
			bonus += b.AttackBonus
		}
		if b.AttackDice != "" {
			r := RollDice(b.AttackDice, false)
			bonus += r
			log = append(log, fmt.Sprintf("  ✨ %s: +%d to the roll.", b.Name, r))
		}
		if b.NextAttackAdvantage {
			adv++
			b.NextAttackAdvantage = false
			log = append(log, fmt.Sprintf("  ✨ %s: advantage!", b.Name))
		}
	}
	dropSpent(st)
	return bonus, adv, log
}

// buffSaveBonus rolls what the holder's buffs add to a saving throw (bless,
// resistance).
func buffSaveBonus(st *types.PlayerCombatState) int {
	if st == nil {
		return 0
	}
	n := 0
	for _, b := range st.SpellBuffs {
		if b.SaveDice != "" {
			n += RollDice(b.SaveDice, false)
		}
	}
	return n
}

// breakOnAttack ends the holder's buffs that only last until they attack
// (sanctuary, invisibility).
func breakOnAttack(st *types.PlayerCombatState) []string {
	var log []string
	for _, name := range removeBuffs(st, func(b *types.SpellBuff) bool { return b.EndsOnAttack }) {
		log = append(log, fmt.Sprintf("  %s ends as you strike.", name))
	}
	return log
}

// buffHitDamage rolls the holder's on-hit buffs for a hit on m: extra damage
// (divine favor, hex, a smite) and a smite's condition rider. melee/spell say
// what kind of hit it was; weaponType types damage a buff doesn't name.
func buffHitDamage(cs *types.CombatSession, holder *types.PartyCombatant, m *types.MonsterInstance, melee, spell, crit bool, weaponType string) (int, []string) {
	st := &holder.CombatState
	total := 0
	var log []string
	for i := range st.SpellBuffs {
		b := &st.SpellBuffs[i]
		if b.HitDice == "" && b.HitBonus == 0 && b.HitCondition == nil {
			continue
		}
		if b.MarkedID != "" && b.MarkedID != m.InstanceID {
			continue
		}
		if (spell && !b.HitSpells) || (b.MeleeOnly && !melee) {
			continue
		}
		dtype := b.HitType
		if dtype == "" {
			dtype = weaponType
		}
		if b.HitDice != "" || b.HitBonus != 0 {
			d := b.HitBonus
			if b.HitDice != "" {
				d = ResolveDamageToMonster(b.HitDice, b.HitBonus, dtype, crit, m)
			}
			if d > 0 {
				total += d
				log = append(log, fmt.Sprintf("  ✨ %s: +%d %s damage.", b.Name, d, dtype))
			}
		}
		if c := b.HitCondition; c != nil {
			landed := true
			if c.Save != "" {
				saveTotal := monsterSaveTotal(m, c.Save)
				if saveTotal >= b.SaveDC {
					landed = false
					log = append(log, fmt.Sprintf("  %s resists %s (%s save %d vs DC %d).", m.Name, b.Name, c.Save, saveTotal, b.SaveDC))
				}
			}
			if landed {
				if line, _ := landCondition(m, c, b.SpellID, b.CasterID, b.SaveDC); line != "" {
					log = append(log, line)
				}
			}
		}
		if b.HitOnce {
			b.HitDice, b.HitBonus, b.HitCondition, b.HitOnce = "", 0, nil, false
		}
	}
	dropSpent(st)
	return total, log
}

// meleeOverride applies shillelagh-style buffs to a melee weapon attack: the
// attack and damage use the caster's spellcasting ability, and the weapon's
// damage die is replaced. Returns the (possibly copied) item and stats.
func meleeOverride(st *types.PlayerCombatState, save *types.SaveFile, item map[string]interface{}, stats map[string]interface{}, itemID string) (map[string]interface{}, map[string]interface{}, []string) {
	for _, b := range st.SpellBuffs {
		if b.MeleeStat == "" && b.WeaponDice == "" {
			continue
		}
		if len(b.Weapons) > 0 && !containsFold(b.Weapons, itemID) {
			continue
		}
		var log []string
		if b.WeaponDice != "" && item != nil {
			cp := make(map[string]interface{}, len(item))
			for k, v := range item {
				cp[k] = v
			}
			cp["damage"] = b.WeaponDice
			item = cp
		}
		if b.MeleeStat != "" {
			if ability, ok := character.SpellcastingAbility(save.Class); ok {
				cp := make(map[string]interface{}, len(stats))
				for k, v := range stats {
					cp[k] = v
				}
				score := character.AbilityScore(stats, ability)
				for k := range cp {
					if strings.EqualFold(k, "strength") {
						cp[k] = score
					}
				}
				stats = cp
			}
		}
		log = append(log, fmt.Sprintf("  ✨ %s guides the blow.", b.Name))
		return item, stats, log
	}
	return item, stats, nil
}

// containsFold reports whether list holds s, ignoring case.
func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// conditionPhrase words a spell-applied condition for the log.
var conditionPhrase = map[string]string{
	"baned":      "baned: −1d4 to its attacks and saves",
	"distracted": "distracted",
	"dueled":     "locked in the duel",
	"burning":    "burning",
	"moonlit":    "caught in the moonbeam",
	"beset":      "beset by spirits",
	"laughing":   "helpless with laughter",
	"exhausted":  "exhausted: its attacks have disadvantage",
}

// landCondition puts a spell's condition on a monster unless its type isn't
// one the spell affects or it's immune. Returns the log line and whether it
// landed.
func landCondition(m *types.MonsterInstance, c *types.SpellCondition, spellID, casterID string, dc int) (string, bool) {
	if len(c.CreatureTypes) > 0 && !containsFold(c.CreatureTypes, m.Data.Type) {
		return fmt.Sprintf("  The spell finds no hold on %s.", m.Name), false
	}
	if containsFold(m.Data.ConditionImmunities, c.Condition) {
		return fmt.Sprintf("  %s can't be %s.", m.Name, c.Condition), false
	}
	rounds := c.Rounds
	if rounds <= 0 {
		rounds = -1
	}
	saveDC := 0
	if c.SaveEachTurn != "" {
		saveDC = dc
	}
	ApplyCondition(&m.Conditions, types.CombatCondition{
		Name: c.Condition, DurationRounds: rounds, SaveDC: saveDC, SaveStat: c.SaveEachTurn,
		Source: spellID, CasterID: casterID, DamageDice: c.DamageDice, DamageType: c.DamageType,
	})
	phrase := conditionPhrase[c.Condition]
	if phrase == "" {
		phrase = c.Condition
	}
	return fmt.Sprintf("  %s is %s!", m.Name, phrase), true
}

// ─── Defence: what buffs do when a monster attacks ──────────────────────────

// attackDefense works out what the target's buffs (and the monster's spell
// conditions) do to a monster's attack before it's rolled: advantage shifts,
// a penalty to the roll, or the attack lost outright (sanctuary).
func attackDefense(cs *types.CombatSession, target *types.PartyCombatant, m *types.MonsterInstance, ranged bool) (adv, penalty int, lost bool, log []string) {
	st := &target.CombatState
	for _, b := range st.SpellBuffs {
		if b.AttackersDisadvantage {
			adv--
		}
		if b.RangedDefense && ranged {
			adv--
		}
		if len(b.WardTypes) > 0 && containsFold(b.WardTypes, m.Data.Type) {
			adv--
		}
	}
	for _, c := range m.Conditions {
		switch strings.ToLower(c.Name) {
		case "baned":
			p := RollDice("1d4", false)
			penalty += p
			log = append(log, fmt.Sprintf("  Bane weighs on %s (−%d).", m.Name, p))
		case "dueled":
			if c.CasterID != "" && c.CasterID != target.ID {
				adv--
			}
		}
	}
	for _, b := range st.SpellBuffs {
		if !b.Sanctuary {
			continue
		}
		total := monsterSaveTotal(m, "wisdom")
		if total < b.SaveDC {
			return adv, penalty, true, append(log, fmt.Sprintf("  %s can't bring itself to strike %s (Wisdom save %d vs DC %d).",
				m.Name, target.Name, total, b.SaveDC))
		}
		log = append(log, fmt.Sprintf("  %s pushes past the sanctuary (Wisdom save %d vs DC %d).", m.Name, total, b.SaveDC))
		break
	}
	return adv, penalty, false, log
}

// deflectHit gives the target's illusions and reactions a chance to turn a hit
// aside: an illusory double takes it, or a prepared shield spell fires when +AC
// would make it miss. Returns true when the hit becomes a miss.
func deflectHit(cs *types.CombatSession, target *types.PartyCombatant, save *types.SaveFile, m *types.MonsterInstance, result AttackResult, playerAC int) (bool, []string) {
	st := &target.CombatState
	for i := range st.SpellBuffs {
		b := &st.SpellBuffs[i]
		if b.Decoys > 0 {
			b.Decoys--
			dropSpent(st)
			return true, []string{fmt.Sprintf("  %s's blow passes through an illusory double, which vanishes.", m.Name)}
		}
	}
	if result.IsCrit {
		return false, nil
	}
	return tryShieldReaction(cs, target, save, result.Total, playerAC)
}

// resistDamage halves damage of a type the target's buffs resist.
func resistDamage(st *types.PlayerCombatState, dtype string, dmg int) (int, []string) {
	for _, b := range st.SpellBuffs {
		if containsFold(b.Resist, dtype) {
			cut := dmg - dmg/2
			return dmg / 2, []string{fmt.Sprintf("  ✨ %s turns aside %d %s damage.", b.Name, cut, dtype)}
		}
	}
	return dmg, nil
}

// soakDamage takes damage out of temporary hit points first.
func soakDamage(st *types.PlayerCombatState, dmg int) (int, []string) {
	if st.TempHP <= 0 || dmg <= 0 {
		return dmg, nil
	}
	soaked := min(st.TempHP, dmg)
	st.TempHP -= soaked
	return dmg - soaked, []string{fmt.Sprintf("  Temporary HP absorbs %d damage.", soaked)}
}

// deathWard keeps a combatant on their feet at 1 HP the first time a blow
// would drop them, spending the ward.
func deathWard(st *types.PlayerCombatState, who string) []string {
	var line string
	removed := false
	removeBuffs(st, func(b *types.SpellBuff) bool {
		if b.DeathWard && !removed {
			removed = true
			line = fmt.Sprintf("  ✨ %s holds %s on their feet at 1 HP!", b.Name, who)
			return true
		}
		return false
	})
	if !removed {
		return nil
	}
	st.CurrentHP = 1
	return []string{line}
}

// retaliate deals the target's retaliation buffs back to a melee attacker that
// hit them (armor of agathys' cold). Returns log lines; may kill the attacker.
func retaliate(cs *types.CombatSession, target *types.PartyCombatant, save *types.SaveFile, m *types.MonsterInstance) []string {
	st := &target.CombatState
	var log []string
	for _, b := range st.SpellBuffs {
		if b.Retaliate == "" || (b.RetaliateWhileTempHP && st.TempHP <= 0) || !m.IsAlive {
			continue
		}
		d := ResolveDamageToMonster(b.Retaliate, 0, b.RetaliateType, false, m)
		log = append(log, fmt.Sprintf("  ✨ %s lashes %s for %d %s damage.", b.Name, m.Name, d, b.RetaliateType))
		log = append(log, spellDamageToMonster(cs, target, save, m, d)...)
	}
	return log
}

// spellDamageToMonster applies damage a spell dealt outside its caster's own
// action (a reaction, retaliation, a burn): HP, damage XP to the caster, and
// the kill if it drops.
func spellDamageToMonster(cs *types.CombatSession, caster *types.PartyCombatant, save *types.SaveFile, m *types.MonsterInstance, dmg int) []string {
	if dmg <= 0 || !m.IsAlive {
		return nil
	}
	prev := cs.Actor
	if idx := MemberIndex(cs, caster.ID); idx >= 0 {
		cs.Actor = idx
	}
	defer func() { cs.Actor = prev }()

	adv, _ := character.LoadAdvancement(serverdb.GetDB())
	level := character.GetLevelFromXP(save.Experience, adv)
	applyDamageToMonster(m, dmg)
	var log []string
	if xp := awardDamageXP(cs, m, dmg, save.TimeOfDay, level, adv); xp > 0 {
		log = append(log, fmt.Sprintf("  +%d XP", xp))
	}
	if !m.IsAlive {
		log = append(log, handleMonsterKill(cs, m, save, adv)...)
	}
	return log
}

// conditionDamage burns a monster at the start of its turn for each damaging
// condition on it (searing smite, acid arrow). Returns log lines; the monster
// may die.
func conditionDamage(cs *types.CombatSession, save *types.SaveFile, m *types.MonsterInstance) []string {
	var log []string
	for _, c := range m.Conditions {
		if c.DamageDice == "" || !m.IsAlive {
			continue
		}
		d := ResolveDamageToMonster(c.DamageDice, 0, c.DamageType, false, m)
		what := conditionPhrase[c.Name]
		if what == "" {
			what = c.Name
		}
		log = append(log, fmt.Sprintf("  %s takes %d %s damage (%s).", m.Name, d, c.DamageType, what))
		caster := FindMember(cs, c.CasterID)
		if caster == nil {
			caster = actor(cs)
		}
		log = append(log, spellDamageToMonster(cs, caster, saveOf(caster, save), m, d)...)
	}
	return log
}

// ─── Concentration ───────────────────────────────────────────────────────────

// dropConcentration ends a member's concentration: the spell's lasting effect
// (unless it is keep, the effect a new cast just applied), the buffs it gave
// anyone, and the conditions it holds on foes.
func dropConcentration(cs *types.CombatSession, member *types.PartyCombatant, save *types.SaveFile, keep string) {
	c := member.Concentration
	if c == nil {
		return
	}
	if c.EffectID != "" && c.EffectID != keep {
		effects.RemoveEffect(save, c.EffectID)
	}
	for i := range cs.Party {
		removeBuffs(&cs.Party[i].CombatState, func(b *types.SpellBuff) bool {
			return b.SpellID == c.SpellID && b.CasterID == member.ID
		})
	}
	for i := range cs.Monsters {
		m := &cs.Monsters[i]
		kept := m.Conditions[:0]
		for _, cond := range m.Conditions {
			if cond.Source == c.SpellID && cond.CasterID == member.ID {
				continue
			}
			kept = append(kept, cond)
		}
		m.Conditions = kept
	}
	member.Concentration = nil
}

// ─── Reaction spells ─────────────────────────────────────────────────────────

// preparedReactions lists a member's prepared reaction spells for a trigger
// ("hit": an attack is about to hit you; "damaged": a creature just hurt you),
// with their data.
func preparedReactions(db *sql.DB, save *types.SaveFile, trigger string) []struct {
	id    string
	spell map[string]interface{}
} {
	var out []struct {
		id    string
		spell map[string]interface{}
	}
	seen := map[string]bool{}
	for _, raw := range save.SpellSlots {
		list, _ := raw.([]interface{})
		for _, e := range list {
			m, _ := e.(map[string]interface{})
			id, _ := m["spell"].(string)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			sp, err := gamedata.LoadSpellByID(db, id)
			if err != nil || !spells.IsReaction(sp) {
				continue
			}
			if t, _ := sp["reaction_trigger"].(string); t == trigger {
				out = append(out, struct {
					id    string
					spell map[string]interface{}
				}{id, sp})
			}
		}
	}
	return out
}

// castReaction runs a reaction spell through the casting engine (known,
// prepared, mana, components at hand) as a combat cast.
func castReaction(db *sql.DB, save *types.SaveFile, spellID string, target *types.MonsterInstance) (*spells.CastResult, int, error) {
	adv, _ := character.LoadAdvancement(db)
	level := character.GetLevelFromXP(save.Experience, adv)
	deps := spells.Deps{
		RollD20: RollD20, RollDice: RollDice, ResolveMonsterDamage: ResolveDamageToMonster,
		ComponentsAtHand: true, InCombat: true,
	}
	res, err := spells.Cast(db, deps, save, spellID, level, target)
	return res, level, err
}

// tryShieldReaction casts a prepared AC reaction (shield) when it would turn a
// hit (attack total `total` vs `ac`) into a miss. Spending the mana only when
// it matters is the point: a hit it can't stop doesn't trigger it.
func tryShieldReaction(cs *types.CombatSession, target *types.PartyCombatant, save *types.SaveFile, total, ac int) (bool, []string) {
	st := &target.CombatState
	if st.ReactionUsed || st.IsUnconscious || save == nil {
		return false, nil
	}
	db := serverdb.GetDB()
	if db == nil {
		return false, nil
	}
	for _, r := range preparedReactions(db, save, "hit") {
		def := combatBuffOf(r.spell)
		if def == nil || def.ACBonus <= 0 || total >= ac+def.ACBonus {
			continue
		}
		res, level, err := castReaction(db, save, r.id, nil)
		if err != nil {
			continue
		}
		st.ReactionUsed = true
		log := []string{fmt.Sprintf("  🛡 %s casts %s as the blow comes in (%d vs AC %d)!", whoName(cs, target), res.SpellName, total, ac+def.ACBonus)}
		log = append(log, applySpellBuff(cs, target, res, nil, nil, spellDC(save, level), castingMod(save))...)
		return true, append(log, "  The attack glances off.")
	}
	return false, nil
}

// tryDamagedReaction casts a prepared retaliation reaction (hellish rebuke) at
// the monster that just hurt the target, if it's in the spell's range.
func tryDamagedReaction(cs *types.CombatSession, target *types.PartyCombatant, save *types.SaveFile, m *types.MonsterInstance) []string {
	st := &target.CombatState
	if st.ReactionUsed || st.IsUnconscious || !m.IsAlive || save == nil {
		return nil
	}
	db := serverdb.GetDB()
	if db == nil {
		return nil
	}
	for _, r := range preparedReactions(db, save, "damaged") {
		if rng := spellRange(r.spell); rng > 0 && chebyshev(target.Pos, m.Pos) > rng {
			continue
		}
		res, _, err := castReaction(db, save, r.id, m)
		if err != nil {
			continue
		}
		st.ReactionUsed = true
		log := append([]string{fmt.Sprintf("  🔥 %s answers with %s!", whoName(cs, target), res.SpellName)}, res.Log...)
		return append(log, spellDamageToMonster(cs, target, save, m, res.Damage)...)
	}
	return nil
}

// spellRange reads a spell's range in grid cells (0 = self/touch or unset).
func spellRange(spell map[string]interface{}) int {
	switch v := spell["range"].(type) {
	case string:
		n := 0
		fmt.Sscanf(v, "%d", &n)
		return n
	case float64:
		return int(v)
	}
	return 0
}

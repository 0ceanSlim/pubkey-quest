package combat

import (
	"fmt"
	"strings"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// ─── Conditions engine (M5 §15) ──────────────────────────────────────────────
//
// Conditions live on the combat state as `[]types.CombatCondition`
// ({Name, DurationRounds, SaveDC, SaveStat}) — one list on the player combat
// state, one per monster instance. Each named condition has a fixed mechanical
// footprint (advantage/disadvantage on attacks, incapacitation, speed) applied by
// the resolvers here. Sources: player save-spells (entangle → restrained,
// faerie-fire → outlined, …) and, later, monster abilities.

// ConditionSpec is a condition's mechanical footprint (D&D 5e, §15 of the combat
// plan). Only the combat-relevant flags are modelled for alpha.
type ConditionSpec struct {
	AttacksAgainstAdvantage bool // attackers targeting an afflicted creature roll with advantage
	OwnAttacksDisadvantage  bool // the afflicted creature's own attacks roll with disadvantage
	Incapacitated           bool // cannot take actions or reactions
	Speed0                  bool // cannot move (informational for now; movement gating is a later slice)
}

// conditionRegistry maps a lowercase condition name to its mechanics. Unknown
// names resolve to the zero spec (no effect) so authored data can't crash combat.
var conditionRegistry = map[string]ConditionSpec{
	"blinded":    {AttacksAgainstAdvantage: true, OwnAttacksDisadvantage: true},
	"poisoned":   {OwnAttacksDisadvantage: true},
	"frightened": {OwnAttacksDisadvantage: true}, // "can't move closer to source" deferred (movement slice)
	"prone":      {OwnAttacksDisadvantage: true}, // melee-adv / ranged-disadv nuance deferred
	"restrained": {AttacksAgainstAdvantage: true, OwnAttacksDisadvantage: true, Speed0: true},
	"grappled":   {Speed0: true},
	"stunned":    {AttacksAgainstAdvantage: true, Incapacitated: true},
	"paralyzed":   {AttacksAgainstAdvantage: true, Incapacitated: true}, // auto-crit-in-melee nuance deferred
	"unconscious": {AttacksAgainstAdvantage: true, Incapacitated: true, Speed0: true}, // sleep etc.; wakes on the save-to-end
	"outlined":    {AttacksAgainstAdvantage: true}, // faerie-fire: attackers see you clearly
	"charmed":     {}, // no roll modifier; the "won't attack the charmer" rule is enforced in ApplyMonsterAction
}

func specFor(name string) ConditionSpec { return conditionRegistry[strings.ToLower(name)] }

// HasCondition reports whether the list holds a (case-insensitive) condition.
func HasCondition(conds []types.CombatCondition, name string) bool {
	for _, c := range conds {
		if strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

// IsIncapacitated reports whether any active condition prevents actions/reactions.
func IsIncapacitated(conds []types.CombatCondition) bool {
	for _, c := range conds {
		if specFor(c.Name).Incapacitated {
			return true
		}
	}
	return false
}

// incapacitatingConditionName returns the name of the first incapacitating
// condition (for log lines), or "incapacitated" as a fallback.
func incapacitatingConditionName(conds []types.CombatCondition) string {
	for _, c := range conds {
		if specFor(c.Name).Incapacitated {
			return strings.ToLower(c.Name)
		}
	}
	return "incapacitated"
}

// IsSpeedZero reports whether any active condition roots the creature in place.
func IsSpeedZero(conds []types.CombatCondition) bool {
	for _, c := range conds {
		if specFor(c.Name).Speed0 {
			return true
		}
	}
	return false
}

// ConditionAttackAdvantage returns the advantage delta for an attacker holding
// attackerConds striking a target holding targetConds: +1 per advantage source,
// −1 per disadvantage source. ResolveAttackRoll uses the sign of the total, so
// this composes with the existing weapon/range advantage the callers already sum.
func ConditionAttackAdvantage(attackerConds, targetConds []types.CombatCondition) int {
	adv := 0
	for _, c := range targetConds {
		if specFor(c.Name).AttacksAgainstAdvantage {
			adv++
		}
	}
	for _, c := range attackerConds {
		if specFor(c.Name).OwnAttacksDisadvantage {
			adv--
		}
	}
	return adv
}

// ApplyCondition adds a condition, replacing any existing one of the same name
// (re-applying refreshes duration/DC). Empty names are ignored.
func ApplyCondition(conds *[]types.CombatCondition, cond types.CombatCondition) {
	if conds == nil || strings.TrimSpace(cond.Name) == "" {
		return
	}
	for i, c := range *conds {
		if strings.EqualFold(c.Name, cond.Name) {
			(*conds)[i] = cond
			return
		}
	}
	*conds = append(*conds, cond)
}

// RemoveCondition drops a named condition if present.
func RemoveCondition(conds *[]types.CombatCondition, name string) {
	if conds == nil {
		return
	}
	out := (*conds)[:0]
	for _, c := range *conds {
		if !strings.EqualFold(c.Name, name) {
			out = append(out, c)
		}
	}
	*conds = out
}

// TickCreatureConditions resolves a creature's conditions at the end of its turn:
// a condition carrying a SaveDC lets the creature attempt a save to shake it
// (saveRoll returns d20 + the creature's total bonus for the given stat); on a
// success it ends. Otherwise its DurationRounds decrements and it ends at 0.
// Conditions with neither a save nor a positive remaining duration are permanent
// until removed by another effect. Returns log lines for anything that changed.
//
// `who` is the creature's display name for the log; `saveRoll(stat)` yields the
// full save total (roll + modifier). Caller supplies the RNG-backed roller so the
// engine stays testable.
func TickCreatureConditions(who string, conds *[]types.CombatCondition, saveRoll func(stat string) int) []string {
	if conds == nil || len(*conds) == 0 {
		return nil
	}
	var log []string
	kept := (*conds)[:0]
	for _, c := range *conds {
		// Save-to-end: a condition with a DC + stat gets a save at end of turn.
		if c.SaveDC > 0 && c.SaveStat != "" {
			total := saveRoll(c.SaveStat)
			if total >= c.SaveDC {
				log = append(log, fmt.Sprintf("  %s shakes off %s (%s save %d vs DC %d).",
					who, c.Name, c.SaveStat, total, c.SaveDC))
				continue // ended
			}
		}
		// Timed durations count down; -1 (or 0 with no save) means "until removed".
		if c.DurationRounds > 0 {
			c.DurationRounds--
			if c.DurationRounds == 0 {
				log = append(log, fmt.Sprintf("  %s is no longer %s.", who, c.Name))
				continue
			}
		}
		kept = append(kept, c)
	}
	*conds = kept
	return log
}

// monsterSaveTotal rolls a monster's saving throw for a stat: d20 + the listed
// saving_throws bonus, else the raw ability modifier.
func monsterSaveTotal(m *types.MonsterInstance, stat string) int {
	roll := RollD20()
	if v, ok := m.Data.SavingThrows[strings.ToLower(stat)]; ok {
		return roll + v
	}
	return roll + StatMod(monsterAbilityScore(m.Data.Stats, stat))
}

// playerSaveTotal rolls a player's saving throw: d20 + the ability modifier for
// the given stat.
func playerSaveTotal(save *types.SaveFile, stat string) int {
	return RollD20() + character.AbilityMod(character.AbilityScore(effectiveStats(save), stat))
}

// monsterEffectCondition maps a monster hit-special `effect` string to a combat
// condition name. Non-condition effects (max_hp_reduction, life_steal, …) are absent.
var monsterEffectCondition = map[string]string{
	"knocked_prone": "prone",
	"prone":         "prone",
	"paralyzed":     "paralyzed",
	"restrained":    "restrained",
	"poisoned":      "poisoned",
	"blinded":       "blinded",
	"frightened":    "frightened",
	"stunned":       "stunned",
}

// applyMonsterConditionRider resolves a monster attack's on-hit rider from its
// authored hit.special (§15/§23): the player saves vs the special's DC using the
// named ability; on a failure they gain the mapped condition. A "save"-type
// special names the condition in Effect; "poison_save" always inflicts poisoned.
// restrained/paralyzed re-save each of the player's turns; prone is 1 round.
// Non-condition specials (pull, life_steal, …) are ignored. Returns log lines.
func applyMonsterConditionRider(cs *types.CombatSession, save *types.SaveFile, action types.MonsterAction) []string {
	sp := action.Hit.Special
	if sp == nil || save == nil || len(cs.Party) == 0 {
		return nil
	}

	var cond, stat string
	switch strings.ToLower(sp.Type) {
	case "poison_save":
		cond, stat = "poisoned", "constitution"
	case "save":
		cond, stat = monsterEffectCondition[strings.ToLower(sp.Effect)], sp.Ability
	default:
		return nil // pull / life_steal / max_hp_reduction / … not a simple condition
	}
	if cond == "" {
		return nil
	}
	if stat == "" {
		stat = "constitution"
	}
	dc := sp.DC
	if dc <= 0 {
		dc = 11
	}

	total := playerSaveTotal(save, stat)
	if total >= dc {
		return []string{fmt.Sprintf("  You resist %s (%s save %d vs DC %d).", cond, stat, total, dc)}
	}

	rounds, reSaveDC, reSaveStat := 3, 0, ""
	switch cond {
	case "prone":
		rounds = 1
	case "restrained", "paralyzed":
		rounds, reSaveDC, reSaveStat = -1, dc, stat // save at the end of each of your turns
	}
	ApplyCondition(&cs.Party[0].CombatState.Conditions, types.CombatCondition{
		Name: cond, DurationRounds: rounds, SaveDC: reSaveDC, SaveStat: reSaveStat,
	})
	return []string{fmt.Sprintf("  ✘ You are %s! (%s save %d vs DC %d)", cond, stat, total, dc)}
}

// monsterAbilityScore reads a named ability score off a monster stat block.
func monsterAbilityScore(s types.MonsterStats, stat string) int {
	switch strings.ToLower(stat) {
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

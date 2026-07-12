package combat

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/types"
)

// ─── Class abilities (M5 §12) ────────────────────────────────────────────────
//
// Martial classes (barbarian/fighter/monk/rogue) spend a combat-scoped resource
// pool (Rage/Stamina/Ki/Cunning) on abilities via POST /api/combat/ability. The
// pool is seeded at combat start from class + level, regenerates during the fight
// (per turn / per hit taken / per crit — see class-resources.json), and is never
// persisted. Ability definitions (unlock level, cost, cooldown, scaling tiers)
// load from the migrated `abilities` table; the mechanical effect of each is a
// Go handler in abilityMechanics, keyed by ability id. Abilities without a handler
// return a graceful "not usable in combat yet" rather than a no-op button.

// classResourceCfg mirrors game-data/systems/class-resources.json for the four
// martial classes. Kept in Go so combat has no file dependency at runtime; the
// two must stay in sync (there are only four rows).
type classResourceCfg struct {
	Type        string
	Label       string
	Max         int    // fixed pool; 0 → use maxFormula
	MaxFormula  string // "wisdom_mod + level" (monk ki)
	PerTurn     int
	PerHitTaken int
	PerCrit     int
	StartAtMax  bool // combat_start "max" vs 0
}

var classResources = map[string]classResourceCfg{
	"fighter":   {Type: "stamina", Label: "Stamina", Max: 10, PerTurn: 2, StartAtMax: true},
	"barbarian": {Type: "rage", Label: "Rage", Max: 100, PerHitTaken: 10, StartAtMax: false},
	"monk":      {Type: "ki", Label: "Ki", MaxFormula: "wisdom_mod + level", PerTurn: 1, StartAtMax: true},
	"rogue":     {Type: "cunning", Label: "Cunning", Max: 10, PerTurn: 2, PerCrit: 1, StartAtMax: true},
}

// InitResourcePool seeds a martial class's combat resource pool on the player's
// combat state. Casters (no entry) get no pool. Called once at combat start.
func InitResourcePool(state *types.PlayerCombatState, class string, level int, stats map[string]interface{}) {
	cfg, ok := classResources[strings.ToLower(class)]
	if !ok {
		return // caster — surfaces mana instead, no ability pool
	}
	max := cfg.Max
	if cfg.MaxFormula == "wisdom_mod + level" {
		max = StatMod(GetStatFromMap(stats, "wisdom")) + level
		if max < 1 {
			max = 1
		}
	}
	cur := 0
	if cfg.StartAtMax {
		cur = max
	}
	state.Resource = &types.ResourcePool{
		Type: cfg.Type, Label: cfg.Label, Current: cur, Max: max,
		PerTurn: cfg.PerTurn, PerHitTaken: cfg.PerHitTaken, PerCrit: cfg.PerCrit,
	}
}

// regenResource adds to a pool, clamped to its max. Nil-safe.
func regenResource(p *types.ResourcePool, amount int) {
	if p == nil || amount <= 0 {
		return
	}
	p.Current += amount
	if p.Current > p.Max {
		p.Current = p.Max
	}
}

// tickPlayerAbilities runs at the end of the player's turn: per-turn resource
// regen, and the rage duration countdown (rage clears when it hits 0). Returns
// log lines for anything the player should see.
func tickPlayerAbilities(state *types.PlayerCombatState) []string {
	var log []string
	if state.Resource != nil {
		regenResource(state.Resource, state.Resource.PerTurn)
	}
	if state.RageTurnsLeft > 0 {
		state.RageTurnsLeft--
		if state.RageTurnsLeft == 0 {
			state.RageDamageBonusPct = 0
			state.RageResistPct = 0
			log = append(log, "  Your rage subsides.")
		}
	}
	return log
}

// consumePlayerAction marks the player's action as spent — unless an Action Surge
// / Flurry granted an extra action, in which case it burns one of those and leaves
// the action available so the next attack still lands this turn.
func consumePlayerAction(state *types.PlayerCombatState) {
	if state.ExtraActions > 0 {
		state.ExtraActions--
		return
	}
	state.ActionUsed = true
}

// applyPlayerDamageRiders folds the ability-driven damage riders into a weapon
// hit: barbarian rage's % bonus and the rogue's readied Sneak Attack dice (once,
// consumed on the first hit). Returns the modified damage and log lines.
func applyPlayerDamageRiders(state *types.PlayerCombatState, dmg int, isCrit bool) (int, []string) {
	var log []string
	if state.RageTurnsLeft > 0 && state.RageDamageBonusPct > 0 {
		bonus := dmg * state.RageDamageBonusPct / 100
		if bonus > 0 {
			dmg += bonus
			log = append(log, fmt.Sprintf("  🪓 Rage +%d damage.", bonus))
		}
	}
	if state.PendingSneakDice != "" {
		sneak := RollDice(state.PendingSneakDice, isCrit)
		dmg += sneak
		log = append(log, fmt.Sprintf("  🗡️ Sneak Attack +%d (%s).", sneak, state.PendingSneakDice))
		state.PendingSneakDice = "" // consumed on the first hit of the turn
	}
	return dmg, log
}

// ─── Ability definitions loaded from the DB ──────────────────────────────────

type abilityTier struct {
	MinLevel     int  `json:"min_level"`
	MaxLevel     int  `json:"max_level"`
	OverrideCost *int `json:"override_cost"`
}

type abilityDef struct {
	ID           string
	Name         string
	Class        string
	UnlockLevel  int
	ResourceCost int
	Cooldown     string
	Tiers        []abilityTier
}

// loadAbility reads one ability (with its scaling tiers) from the abilities table.
func loadAbility(db *sql.DB, id string) (*abilityDef, error) {
	var a abilityDef
	var props string
	err := db.QueryRow(
		"SELECT id, name, class, unlock_level, resource_cost, COALESCE(cooldown, ''), COALESCE(properties, '') FROM abilities WHERE id = ?",
		id,
	).Scan(&a.ID, &a.Name, &a.Class, &a.UnlockLevel, &a.ResourceCost, &a.Cooldown, &props)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("unknown ability %q", id)
	}
	if err != nil {
		return nil, fmt.Errorf("loadAbility %s: %w", id, err)
	}
	if props != "" {
		var full struct {
			Tiers []abilityTier `json:"scaling_tiers"`
		}
		if json.Unmarshal([]byte(props), &full) == nil {
			a.Tiers = full.Tiers
		}
	}
	return &a, nil
}

// tierIndexFor returns the 0-based index of the scaling tier that applies at the
// given level (clamped to the last tier once the character is past all bands).
func tierIndexFor(a *abilityDef, level int) int {
	for i, t := range a.Tiers {
		if level >= t.MinLevel && level <= t.MaxLevel {
			return i
		}
	}
	if len(a.Tiers) > 0 {
		return len(a.Tiers) - 1
	}
	return 0
}

// clampIdx keeps a tier index inside a magnitude table.
func clampIdx(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// ─── Ability mechanics ───────────────────────────────────────────────────────

// abilityMechanic is the combat effect of an ability. action is "action",
// "bonus", or "" (a free feature that consumes neither). apply mutates combat
// state and returns log lines.
type abilityMechanic struct {
	action string
	apply  func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string
}

// abilityMechanics is the alpha set — two per martial class (M5: "pick 2–3 per
// class"). Every entry maps to real combat state so no button is a no-op.
var abilityMechanics = map[string]abilityMechanic{
	// ── Barbarian ──
	"enter-rage": {action: "bonus", apply: func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string {
		i := clampIdx(tierIdx, 4)
		dmg := []int{50, 60, 75, 100}[i]
		resist := []int{25, 35, 45, 50}[i]
		turns := []int{4, 5, 6, 8}[i]
		state.RageDamageBonusPct = dmg
		state.RageResistPct = resist
		state.RageTurnsLeft = turns
		return []string{fmt.Sprintf("  You fly into a rage — +%d%% damage, -%d%% damage taken for %d turns.", dmg, resist, turns)}
	}},
	"intimidating-roar": {action: "action", apply: func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string {
		if len(cs.Monsters) == 0 || !cs.Monsters[0].IsAlive {
			return []string{"  You roar, but there's nothing left to frighten."}
		}
		m := &cs.Monsters[0]
		dc := 8 + proficiencyBonus(level) + StatMod(GetStatFromMap(effectiveStats(save), "strength"))
		total := monsterSaveTotal(m, "wisdom")
		if total >= dc {
			return []string{fmt.Sprintf("  You roar! %s holds its nerve (WIS save %d vs DC %d).", m.Name, total, dc)}
		}
		rounds := []int{2, 2, 3, 3}[clampIdx(tierIdx, 4)]
		ApplyCondition(&m.Conditions, types.CombatCondition{Name: "frightened", DurationRounds: rounds})
		return []string{fmt.Sprintf("  ✘ You roar! %s is frightened for %d rounds (WIS save %d vs DC %d).", m.Name, rounds, total, dc)}
	}},

	// ── Fighter ──
	"second-wind": {action: "bonus", apply: func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string {
		i := clampIdx(tierIdx, 4)
		pct := []int{25, 40, 60, 80}[i]
		heal := state.MaxHP * pct / 100
		if heal < 1 {
			heal = 1
		}
		before := state.CurrentHP
		state.CurrentHP += heal
		if state.CurrentHP > state.MaxHP {
			state.CurrentHP = state.MaxHP
		}
		log := []string{fmt.Sprintf("  You catch your breath and recover %d HP.", state.CurrentHP-before)}
		if i >= 2 && len(state.Conditions) > 0 { // t3+: shrug off a debuff
			removed := state.Conditions[0].Name
			state.Conditions = state.Conditions[1:]
			log = append(log, fmt.Sprintf("  You shrug off %s.", removed))
		}
		return log
	}},
	"action-surge": {action: "", apply: func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string {
		extra := []int{1, 1, 2, 2}[clampIdx(tierIdx, 4)]
		state.ExtraActions += extra
		return []string{fmt.Sprintf("  You surge with energy — %d extra action(s) this turn.", extra)}
	}},

	// ── Monk ──
	"flurry-of-blows": {action: "bonus", apply: func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string {
		extra := []int{1, 2, 3, 4}[clampIdx(tierIdx, 4)]
		state.ExtraActions += extra
		return []string{fmt.Sprintf("  A flurry of blows — %d extra strike(s) this turn.", extra)}
	}},
	"patient-defense": {action: "bonus", apply: func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string {
		state.Dodging = true
		return []string{"  You take a patient, defensive stance — attacks against you have disadvantage."}
	}},

	// ── Rogue ──
	"sneak-attack": {action: "", apply: func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string {
		dice := []string{"2d6", "4d6", "6d6", "10d6"}[clampIdx(tierIdx, 4)]
		state.PendingSneakDice = dice
		return []string{fmt.Sprintf("  You ready a sneak attack — your next hit deals +%s.", dice)}
	}},
	"shadow-step": {action: "bonus", apply: func(cs *types.CombatSession, state *types.PlayerCombatState, save *types.SaveFile, level, tierIdx int) []string {
		state.Disengaged = true
		state.Dodging = true
		return []string{"  You melt into the shadows — no opportunity attacks, and strikes against you have disadvantage."}
	}},
}

// ProcessPlayerAbility resolves the player activating a class ability during combat.
// Validates class / unlock level / resource / cooldown / action economy, applies the
// ability's mechanic, then spends the resource. Does NOT run the monster turn — the
// caller ends the turn (surge/flurry leave the action open on purpose).
func ProcessPlayerAbility(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, abilityID string, advancement []types.AdvancementEntry) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot use ability: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	state := &cs.Party[0].CombatState
	if IsIncapacitated(state.Conditions) {
		return nil, fmt.Errorf("you are %s and can't act", incapacitatingConditionName(state.Conditions))
	}

	a, err := loadAbility(db, abilityID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(a.Class, save.Class) {
		return nil, fmt.Errorf("%s is a %s ability — your class can't use it", a.Name, a.Class)
	}

	level := character.GetLevelFromXP(save.Experience, advancement)
	if level < a.UnlockLevel {
		return nil, fmt.Errorf("%s unlocks at level %d", a.Name, a.UnlockLevel)
	}

	if a.Cooldown == "once_per_combat" && containsString(state.AbilitiesUsed, a.ID) {
		return nil, fmt.Errorf("%s can only be used once per fight", a.Name)
	}

	mech, ok := abilityMechanics[a.ID]
	if !ok {
		return nil, fmt.Errorf("%s isn't usable in combat yet", a.Name)
	}

	// Action economy — check before spending anything.
	switch mech.action {
	case "action":
		if state.ActionUsed && state.ExtraActions == 0 {
			return nil, fmt.Errorf("you've already taken your action this turn")
		}
	case "bonus":
		if state.BonusActionUsed {
			return nil, fmt.Errorf("you've already used your bonus action this turn")
		}
	}

	// Resource cost (tier can override).
	tierIdx := tierIndexFor(a, level)
	cost := a.ResourceCost
	if len(a.Tiers) > 0 && a.Tiers[clampIdx(tierIdx, len(a.Tiers))].OverrideCost != nil {
		cost = *a.Tiers[clampIdx(tierIdx, len(a.Tiers))].OverrideCost
	}
	if state.Resource == nil {
		return nil, fmt.Errorf("your class has no ability resource")
	}
	if state.Resource.Current < cost {
		return nil, fmt.Errorf("not enough %s (%d/%d needed)", state.Resource.Label, state.Resource.Current, cost)
	}

	// Apply, then spend.
	log := []string{fmt.Sprintf("✦ %s!", a.Name)}
	log = append(log, mech.apply(cs, state, save, level, tierIdx)...)

	state.Resource.Current -= cost
	if a.Cooldown == "once_per_combat" {
		state.AbilitiesUsed = append(state.AbilitiesUsed, a.ID)
	}
	switch mech.action {
	case "action":
		consumePlayerAction(state)
	case "bonus":
		state.BonusActionUsed = true
	}

	return log, nil
}

// containsString reports whether s is in the slice (case-insensitive).
func containsString(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

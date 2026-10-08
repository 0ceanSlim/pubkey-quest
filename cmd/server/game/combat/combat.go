package combat

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/encounter"
	"pubkey-quest/cmd/server/game/events"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

const combatGridWidth  = 9
const combatGridHeight = 7

// effectiveStats returns the player's ability scores with active-effect modifiers
// (buffs, plus fatigue/exhaustion penalties) folded in — the stat block every
// combat roll reads from, so being tired or buffed actually changes attack, AC,
// saving throws, and ability DCs. This mirrors the effective stats the
// out-of-combat skill checks already use (effects.EffectiveStats).
func effectiveStats(save *types.SaveFile) map[string]interface{} {
	return effects.EffectiveStats(save)
}

// chebyshev returns the Chebyshev distance between two grid positions.
// This maps directly to D&D range: 0 = contact, 1 = adjacent, 2+ = ranged.
func chebyshev(a, b types.Position) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// ChebyshevExported is the exported version of chebyshev for use by the API layer.
func ChebyshevExported(a, b types.Position) int {
	return chebyshev(a, b)
}

// raceSpeed returns the base movement speed (in feet) for a D&D race.
// 5e canonical values: 25 ft for Small/stout races, 30 ft for Medium.
func raceSpeed(race string) int {
	switch strings.ToLower(race) {
	case "halfling", "gnome", "dwarf":
		return 25
	default:
		return 30
	}
}

// playerMovementBudget returns the number of grid cells the player can move per turn.
// Each cell = 5 ft. Budget = speed / 5 (floor).
func playerMovementBudget(race string) int {
	return raceSpeed(race) / 5
}

// ─── StartCombat ─────────────────────────────────────────────────────────────

// EncounterSpec describes the hostile side of a fight about to start.
type EncounterSpec struct {
	MonsterIDs    []string // one entry per monster; repeats mean several of that kind
	EnvironmentID string   // drives the starting range
	Surprise      bool     // the party is caught off guard: every monster acts first in round 1
}

// StartCombat initialises a new CombatSession against a single monster.
// Kept as the common-case entry point; see StartEncounter for groups.
func StartCombat(db *sql.DB, save *types.SaveFile, npub, monsterID, environmentID string, advancement []types.AdvancementEntry) (*types.CombatSession, error) {
	return StartEncounter(db, save, npub, EncounterSpec{MonsterIDs: []string{monsterID}, EnvironmentID: environmentID}, advancement)
}

// MonsterIDsForCount expands a monster id and an authored count into the id list
// StartEncounter takes. A count below 1 means one.
func MonsterIDsForCount(monsterID string, count int) []string {
	if count < 1 {
		count = 1
	}
	if count > maxEncounterMonsters {
		count = maxEncounterMonsters
	}
	ids := make([]string, count)
	for i := range ids {
		ids[i] = monsterID
	}
	return ids
}

// StartEncounter initialises a new CombatSession for one or more monsters.
// Initiative is rolled for every combatant; any monsters ahead of the player in
// the order act immediately, so the returned session is always parked on the
// player's turn (or already resolved if the opening went badly).
// The session lives in server memory only — it is never written to the save file.
func StartEncounter(db *sql.DB, save *types.SaveFile, npub string, spec EncounterSpec, advancement []types.AdvancementEntry) (*types.CombatSession, error) {
	if len(spec.MonsterIDs) == 0 {
		return nil, fmt.Errorf("StartEncounter: no monsters")
	}
	if len(spec.MonsterIDs) > maxEncounterMonsters {
		spec.MonsterIDs = spec.MonsterIDs[:maxEncounterMonsters]
	}

	monsters := make([]types.MonsterInstance, 0, len(spec.MonsterIDs))
	crs := make([]float64, 0, len(spec.MonsterIDs))
	for _, id := range spec.MonsterIDs {
		data, err := LoadMonsterByID(db, id)
		if err != nil {
			return nil, fmt.Errorf("StartEncounter: %w", err)
		}
		monsters = append(monsters, newMonsterInstance(data))
		crs = append(crs, data.ChallengeRating)
	}
	assignInstanceIDs(monsters)

	cs := &types.CombatSession{
		Party:         []types.PartyCombatant{newPlayerCombatant(npub, save)},
		Monsters:      monsters,
		Round:         1,
		GridWidth:     combatGridWidth,
		GridHeight:    combatGridHeight,
		EnvironmentID: spec.EnvironmentID,
		IsSurprised:   spec.Surprise,
		Phase:         "active",
	}
	placeMonsters(cs, startingRange(spec.EnvironmentID))

	level := character.GetLevelFromXP(save.Experience, advancement)
	// Seed the martial class resource pool (Rage/Stamina/Ki/Cunning) for the fight.
	InitResourcePool(&cs.Party[0].CombatState, save.Class, level, save.Stats)
	// Rate the whole group against the player's level band (M5 §22 difficulty
	// guardrail): several weak monsters can add up to a deadly fight.
	cs.Difficulty = encounter.GroupDifficulty(crs, level, len(cs.Party))

	cs.Log = append(cs.Log, encounterOpeningLine(cs))
	switch cs.Difficulty {
	case "deadly":
		cs.Log = append(cs.Log, fmt.Sprintf("  ⚠️ %s looks deadly — you may want to flee.", foeLabel(cs)))
	case "tough":
		cs.Log = append(cs.Log, fmt.Sprintf("  ⚠️ %s looks like a tough fight.", foeLabel(cs)))
	}

	cs.Log = append(cs.Log, rollEncounterInitiative(cs, save)...)
	if spec.Surprise {
		cs.Log = append(cs.Log, "⚡ You're caught off guard!")
	}

	if cs.Initiative[0].Type == "monster" {
		cs.Log = append(cs.Log, fmt.Sprintf("⚡ %s goes first!", cs.Initiative[0].Name))
		cs.Log = append(cs.Log, runOpeningTurns(db, cs, save)...)
	} else {
		cs.CurrentTurnIndex = 0
		startPlayerTurn(cs, save)
		cs.Log = append(cs.Log, "⚡ You go first!")
	}

	return cs, nil
}

// encounterOpeningLine announces the foes and how far off the nearest one is.
func encounterOpeningLine(cs *types.CombatSession) string {
	if len(cs.Monsters) == 1 {
		return fmt.Sprintf("⚔️  Combat begins! %s appears at range %d.", cs.Monsters[0].Name, nearestRange(cs))
	}
	return fmt.Sprintf("⚔️  Combat begins! %s appear — the nearest at range %d.", foeLabel(cs), nearestRange(cs))
}

// foeLabel names the opposition for log lines: "Goblin" for one, "3 Goblins" for
// a pack of one kind, "4 enemies" for a mixed group.
func foeLabel(cs *types.CombatSession) string {
	if len(cs.Monsters) == 1 {
		return cs.Monsters[0].Name
	}
	first := cs.Monsters[0].TemplateID
	for _, m := range cs.Monsters[1:] {
		if m.TemplateID != first {
			return fmt.Sprintf("%d enemies", len(cs.Monsters))
		}
	}
	return fmt.Sprintf("%d %ss", len(cs.Monsters), cs.Monsters[0].Data.Name)
}

// rollEncounterInitiative rolls every combatant's initiative, sorts the order
// (DEX breaks ties) and returns the log lines. A surprised party is moved behind
// every monster for the opening round.
func rollEncounterInitiative(cs *types.CombatSession, save *types.SaveFile) []string {
	effStats := effectiveStats(save)
	playerDEX := GetStatFromMap(effStats, "dexterity")
	playerInit := rollInitiative(StatMod(playerDEX))

	entries := []types.InitiativeEntry{
		{ID: cs.Party[0].ID, Type: "player", Name: "You", Initiative: playerInit.Total, DEXScore: playerDEX},
	}
	log := []string{
		"⚡ Rolling initiative…",
		fmt.Sprintf("  You rolled %d%s", playerInit.Face, formatModifier(playerInit.Mod)),
	}
	for i := range cs.Monsters {
		m := &cs.Monsters[i]
		roll := rollInitiative(StatMod(m.Data.Stats.Dexterity))
		m.Initiative = roll.Total
		entries = append(entries, types.InitiativeEntry{
			ID: m.InstanceID, Type: "monster", Name: m.Name, Initiative: roll.Total, DEXScore: m.Data.Stats.Dexterity,
		})
		log = append(log, fmt.Sprintf("  %s rolled %d%s", m.Name, roll.Face, formatModifier(roll.Mod)))
	}
	cs.Initiative = sortInitiative(entries, cs.IsSurprised)
	return log
}

// newPlayerCombatant snapshots the player's current HP into combat state.
func newPlayerCombatant(npub string, save *types.SaveFile) types.PartyCombatant {
	return types.PartyCombatant{
		Type:               "player",
		ID:                 npub,
		IsPlayerControlled: true,
		Pos:                types.Position{X: 1, Y: combatGridHeight / 2},
		CombatState: types.PlayerCombatState{
			CurrentHP:      save.HP,
			MaxHP:          save.MaxHP,
			MovementBudget: playerMovementBudget(save.Race),
		},
	}
}

// newMonsterInstance builds a live monster from its stat block, rolling HP.
func newMonsterInstance(data *types.MonsterData) types.MonsterInstance {
	hp := rollMonsterHP(data.HPDice, data.HitPoints)
	return types.MonsterInstance{
		TemplateID: data.ID,
		InstanceID: data.ID,
		Name:       data.Name,
		CurrentHP:  hp,
		MaxHP:      hp,
		ArmorClass: data.ArmorClass,
		IsAlive:    true,
		Data:       *data,
	}
}

// rollMonsterHP rolls the monster's HP dice, falling back to the fixed average.
func rollMonsterHP(hpDice string, fixedHP int) int {
	if hpDice == "" || fixedHP <= 0 {
		return fixedHP
	}
	hp := RollDice(hpDice, false)
	if hp < 1 {
		return fixedHP
	}
	return hp
}

// initRoll holds a single initiative roll: the d20 face, the modifier, and the total.
type initRoll struct {
	Face  int // d20 face value (1–20) — what the dice animation shows
	Mod   int // ability modifier added to the roll
	Total int // Face + Mod — what feeds into initiative ordering
}

// rollInitiative makes one initiative throw and returns face / mod / total.
func rollInitiative(mod int) initRoll {
	f := RollD20()
	return initRoll{Face: f, Mod: mod, Total: f + mod}
}

// sortInitiative orders combatants by initiative, with DEX as the tiebreaker.
// When surprised, the party is placed after every monster (stable within each side).
func sortInitiative(entries []types.InitiativeEntry, surprised bool) []types.InitiativeEntry {
	sort.SliceStable(entries, func(i, j int) bool {
		if surprised && (entries[i].Type == "player") != (entries[j].Type == "player") {
			return entries[j].Type == "player"
		}
		if entries[i].Initiative != entries[j].Initiative {
			return entries[i].Initiative > entries[j].Initiative
		}
		return entries[i].DEXScore > entries[j].DEXScore
	})
	return entries
}

// startingRange returns the encounter's initial range based on environment type.
func startingRange(environmentID string) int {
	switch environmentID {
	case "dungeon", "cave", "cellar", "ruins-interior", "crypt":
		return RollRange(0, 1)
	case "forest", "jungle", "swamp", "thicket":
		return RollRange(1, 2)
	case "grassland", "plains", "desert", "coast", "road":
		return RollRange(2, 4)
	default:
		return 2
	}
}

// ─── ProcessPlayerMove ───────────────────────────────────────────────────────

// ProcessPlayerMove moves the player to the target grid cell.
// Can be called any time during the player's turn while movement budget remains.
// Does NOT trigger the monsters' turns — the player must call ProcessEndTurn.
//
// Every monster whose melee reach the player leaves (without Disengage) gets an
// opportunity attack, if it still has its reaction.
func ProcessPlayerMove(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, targetX, targetY int) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot move: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}

	target := types.Position{X: targetX, Y: targetY}
	if !inGrid(cs, target) {
		return nil, fmt.Errorf("target (%d,%d) is outside the grid", targetX, targetY)
	}
	me := &cs.Party[0]
	if occupied(cs, target, me.ID) {
		return nil, fmt.Errorf("that space is occupied")
	}

	state := &me.CombatState
	dist := chebyshev(me.Pos, target)
	if dist == 0 {
		return nil, fmt.Errorf("already at that position")
	}
	remaining := state.MovementBudget - state.MovementSpent
	if dist > remaining {
		return nil, fmt.Errorf("not enough movement — need %d cells, have %d remaining", dist, remaining)
	}

	// Ranges to every active monster before the move, for the OA check below.
	before := map[string]int{}
	for _, m := range activeMonsters(cs) {
		before[m.InstanceID] = rangeTo(cs, m)
	}
	prevRange := nearestRange(cs)
	me.Pos = target
	state.MovementSpent += dist
	newRange := nearestRange(cs)

	var dir string
	switch {
	case newRange < prevRange:
		dir = "forward"
	case newRange > prevRange:
		dir = "back"
	default:
		dir = "sideways"
	}
	log := []string{fmt.Sprintf("  You move %s. (range: %d, movement: %d/%d)",
		dir, newRange, state.MovementSpent, state.MovementBudget)}

	// Opportunity attacks: every monster whose reach the player just left.
	for _, m := range activeMonsters(cs) {
		reach := MonsterMeleeReach(m)
		if reach > 0 && before[m.InstanceID] <= reach && rangeTo(cs, m) > reach &&
			!state.Disengaged && !m.ReactionUsed && !IsIncapacitated(m.Conditions) {
			log = append(log, executeMonsterOA(cs, m, save, db)...)
			if cs.Phase != "active" {
				break // the OA dropped the player
			}
		}
	}

	return log, nil
}

// executeMonsterOA resolves an opportunity attack from the monster against the fleeing player.
// Uses the first available melee action. Marks monster.ReactionUsed.
func executeMonsterOA(cs *types.CombatSession, monster *types.MonsterInstance, save *types.SaveFile, db *sql.DB) []string {
	actionIdx := -1
	for i, a := range monster.Data.Actions {
		if a.Type == "melee_attack" {
			actionIdx = i
			break
		}
	}
	if actionIdx < 0 {
		return nil
	}
	monster.ReactionUsed = true
	action := monster.Data.Actions[actionIdx]
	playerAC := computePlayerAC(db, save)

	result := ResolveAttackRoll(action.AttackBonus, playerAC, 0)
	log := []string{
		fmt.Sprintf(
			"  ⚡ %s takes an opportunity attack with %s: rolled %d%s",
			monster.Name, action.Name, result.Roll,
			formatModifier(action.AttackBonus),
		),
		outcomeLine(result),
	}
	if !result.IsHit {
		return log
	}
	dmg := ResolveDamageToPlayer(action.Hit.Dice, action.Hit.Mod, result.IsCrit)
	crit := ""
	if result.IsCrit {
		crit = " CRITICAL HIT!"
	}
	log = append(log, fmt.Sprintf("  %s deals %d %s damage.%s", monster.Name, dmg, action.Hit.Type, crit))
	log = append(log, applyDamageToPlayer(cs, dmg)...)
	return log
}

// ─── ProcessPlayerDisengage ──────────────────────────────────────────────────

// ProcessPlayerDisengage spends the player's action to disengage — no opportunity
// attacks will be provoked by movement for the rest of this turn.
func ProcessPlayerDisengage(cs *types.CombatSession) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot disengage: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	state := &cs.Party[0].CombatState
	if state.ActionUsed {
		return nil, fmt.Errorf("action already used this turn")
	}
	state.ActionUsed = true
	state.Disengaged = true
	return []string{"  You disengage — your movement no longer provokes opportunity attacks."}, nil
}

// ─── ProcessPlayerHold ───────────────────────────────────────────────────────

// ProcessPlayerHold uses the player's action to brace into a readied stance.
// Consumes all remaining movement (footwork burned into the brace) and sets
// HeldPosition=true so the readied counter-attack fires if the monster closes
// into melee reach on its next turn. Requires the action to be unused AND at
// least one point of movement remaining (you can't brace if you've already run).
func ProcessPlayerHold(cs *types.CombatSession) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot hold: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	state := &cs.Party[0].CombatState
	if state.ActionUsed {
		return nil, fmt.Errorf("action already used this turn")
	}
	if state.MovementSpent >= state.MovementBudget {
		return nil, fmt.Errorf("need at least one movement point to brace")
	}
	state.ActionUsed = true
	state.MovementSpent = state.MovementBudget
	state.HeldPosition = true
	return []string{"  You brace yourself, readying a counter-strike."}, nil
}


// ─── ProcessPlayerFlee ───────────────────────────────────────────────────────

// ProcessPlayerFlee attempts to escape combat.
// Requires every enemy to be at range ≥ 3. Uses the player's full action.
// The chance is set by the nearest enemy's range and the fastest pursuer.
// On success: phase → "loot" (bodies left behind — no loot; XP already accumulated). Combat ends.
// On failure: returns log, caller should prompt player to End Turn.
func ProcessPlayerFlee(cs *types.CombatSession, save *types.SaveFile) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot flee: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	foes := activeMonsters(cs)
	if len(foes) == 0 {
		return nil, fmt.Errorf("no living enemy to flee from")
	}
	r := nearestRange(cs)
	if r < 3 {
		return nil, fmt.Errorf("too close to flee — get every enemy to range 3 or more first")
	}

	// The fastest pursuer decides the chase; any relentless one makes it harder.
	monster := foes[0]
	monsterAth := 0.0
	relentless := false
	for _, m := range foes {
		ath := athleticsScore(m.Data.Stats.Strength, m.Data.Stats.Constitution, m.Data.Stats.Dexterity)
		if ath > monsterAth {
			monster, monsterAth = m, ath
		}
		relentless = relentless || m.Data.Behavior.Relentless
	}

	fleeStats := effectiveStats(save)
	playerAth := athleticsScore(
		GetStatFromMap(fleeStats, "strength"),
		GetStatFromMap(fleeStats, "constitution"),
		GetStatFromMap(fleeStats, "dexterity"),
	)

	speedAdv := playerAth - monsterAth
	speedMod := speedAdv * 0.03

	baseChance := float64(r-2) * 0.25 // range 3=25%, 4=50%, 5=75%, 6→capped
	if baseChance > 0.90 {
		baseChance = 0.90
	}

	relentlessPenalty := 0.0
	if relentless {
		relentlessPenalty = 0.20
	}

	chance := baseChance + speedMod - relentlessPenalty
	if chance < 0.05 {
		chance = 0.05
	}
	if chance > 0.95 {
		chance = 0.95
	}

	roll := RollRange(1, 100)
	pct := int(chance * 100)

	log := []string{fmt.Sprintf(
		"  You attempt to flee! Escape chance: %d%% (rolled %d).", pct, roll,
	)}

	if roll <= pct {
		// Escape successful — end combat without a kill (XP already in cs.XPEarnedThisFight)
		cs.Phase = "loot"
		cs.LootRolled = nil
		log = append(log, "  You manage to put enough distance between you and the enemy to escape!")
		return log, nil
	}

	// Escape failed — player's action is spent; monster responds when player ends turn
	state := &cs.Party[0].CombatState
	state.ActionUsed = true
	log = append(log, fmt.Sprintf("  %s cuts off your escape! You're still in combat.", monster.Name))
	return log, nil
}

// athleticsScore computes the athletics speed value from three raw ability scores.
// Formula from plan §2: (STR × 0.5) + (CON × 0.35) + (DEX × 0.15).
func athleticsScore(str, con, dex int) float64 {
	return float64(str)*0.5 + float64(con)*0.35 + float64(dex)*0.15
}

// ─── ProcessPlayerAttack ─────────────────────────────────────────────────────

// ProcessPlayerAttack resolves the player's attack action.
// Can be called any time during the player's turn while action is available.
// Does NOT trigger the monster's response — the player must call ProcessEndTurn.
//
// targetID: the monster instance to attack ("" = nearest).
// hand: "main" (default) or "off" for two-weapon bonus attack.
// thrown: true to treat a melee weapon with the "thrown" tag as a ranged attack.
func ProcessPlayerAttack(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, targetID, weaponSlot string, hand string, thrown bool, advancement []types.AdvancementEntry) ([]string, error) {
	if cs.Phase != "active" {
		return nil, fmt.Errorf("cannot attack: combat phase is %q", cs.Phase)
	}
	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}

	var log []string
	state := &cs.Party[0].CombatState
	if IsIncapacitated(state.Conditions) {
		return nil, fmt.Errorf("you are incapacitated and can't act")
	}
	monster, err := ResolveTarget(cs, targetID)
	if err != nil {
		return nil, err
	}
	r := rangeTo(cs, monster)
	isOffHand := hand == "off"

	if isOffHand {
		// Validate two-weapon fighting conditions before doing anything
		if err := validateTwoWeaponFighting(db, save, cs); err != nil {
			return nil, err
		}
		// Bonus attack always uses the off-hand slot
		weaponSlot = "offhand"
	}

	item, isUnarmed, err := loadWeaponItem(db, save.Inventory, weaponSlot)
	if err != nil {
		return nil, err
	}

	// Validate that the attack can reach the target at the current range
	if err := validateAttackRange(r, item, isUnarmed, thrown); err != nil {
		return nil, err
	}

	// Ammo consumption — ranged weapons with the "ammunition" tag require ammo
	if !isUnarmed && item != nil && !thrown {
		if hasTag(item["tags"], "ammunition") {
			if err := consumeAmmo(save, cs, item); err != nil {
				return nil, err
			}
		}
	}

	// Thrown weapon — consume one instance of the thrown item from its gear slot
	if thrown && !isUnarmed && item != nil {
		if !hasTag(item["tags"], "thrown") {
			return nil, fmt.Errorf("this weapon cannot be thrown")
		}
		if err := consumeFromGearSlot(save.Inventory, weaponSlot); err != nil {
			return nil, fmt.Errorf("could not consume thrown weapon: %w", err)
		}
	}

	level := character.GetLevelFromXP(save.Experience, advancement)

	attackBonus := resolveAttackBonus(item, effectiveStats(save), save.Class, level, isUnarmed, thrown)
	advantage := resolveAttackAdvantage(r, item, isUnarmed, save.Race, thrown)
	// Conditions: the player's own conditions (poisoned/prone/…) impose disadvantage;
	// the target monster's (restrained/blinded/outlined/…) grant advantage.
	advantage += ConditionAttackAdvantage(state.Conditions, monster.Conditions)
	result := ResolveAttackRoll(attackBonus, monster.ArmorClass, advantage)

	log = append(log, formatAttackRoll(save.D, item, isUnarmed, result), outcomeLine(result))

	if !result.IsHit {
		if isOffHand {
			state.BonusActionUsed = true
		} else {
			consumePlayerAction(state)
		}
		return log, nil
	}

	// Landing a crit builds the rogue's Cunning pool.
	if result.IsCrit && state.Resource != nil {
		regenResource(state.Resource, state.Resource.PerCrit)
	}

	offhandEmpty := isOffhandEmpty(save.Inventory)
	var dmg int
	if isOffHand {
		// Two-weapon fighting bonus attack: no ability score modifier to damage
		dmg = resolvePlayerDamageNoMod(item, monster, offhandEmpty, result.IsCrit)
	} else {
		dmg = resolvePlayerDamage(item, effectiveStats(save), monster, isUnarmed, offhandEmpty, result.IsCrit, thrown)
	}
	log = append(log, formatDamage(item, isUnarmed, dmg, result.IsCrit))

	// Ability riders: rage % bonus + a readied Sneak Attack (consumed on first hit).
	dmg, riderLog := applyPlayerDamageRiders(state, dmg, result.IsCrit)
	log = append(log, riderLog...)

	applyDamageToMonster(monster, dmg)

	xp := awardDamageXP(cs, monster, dmg, save.TimeOfDay, level, advancement)
	if xp > 0 {
		log = append(log, fmt.Sprintf("  +%d XP", xp))
	}

	if isOffHand {
		state.BonusActionUsed = true
	} else {
		consumePlayerAction(state)
	}

	if !monster.IsAlive {
		return append(log, handleMonsterKill(cs, monster, save, advancement)...), nil
	}

	return log, nil
}

// loadWeaponItem fetches item properties for the chosen weapon slot.
// Returns nil + isUnarmed=true when the slot is empty or weaponSlot is "unarmed".
func loadWeaponItem(db *sql.DB, inventory map[string]interface{}, weaponSlot string) (map[string]interface{}, bool, error) {
	if weaponSlot == "unarmed" {
		return nil, true, nil
	}
	// gear_slots uses lowercase keys ("mainhand", "offhand"); normalise camelCase input.
	itemID := gaminventory.GetEquippedItemID(inventory, strings.ToLower(weaponSlot))
	if itemID == "" {
		return nil, true, nil
	}
	item, err := gamedata.LoadItemByID(db, itemID)
	if err != nil {
		return nil, false, fmt.Errorf("loadWeaponItem %s: %w", itemID, err)
	}
	// A non-weapon in hand (spellbook, torch, focus) is not a legal attack source —
	// you strike unarmed instead. Fixes the "spellbook shows as an attack that
	// no-ops" bug (M5 interaction matrix).
	if !isWeaponItem(item) {
		return nil, true, nil
	}
	return item, false, nil
}

// isWeaponItem reports whether an item is a real weapon (by type or tag) and thus
// a legal attack source. Everything else held in hand falls back to Unarmed.
func isWeaponItem(item map[string]interface{}) bool {
	if item == nil {
		return false
	}
	if t, ok := item["type"].(string); ok && strings.Contains(strings.ToLower(t), "weapon") {
		return true
	}
	return hasTag(item["tags"], "weapon")
}

// resolveAttackBonus computes the player's total attack roll modifier.
// When thrown is true the weapon is used as a ranged throw and always uses DEX.
func resolveAttackBonus(item map[string]interface{}, stats map[string]interface{}, class string, level int, isUnarmed, thrown bool) int {
	if isUnarmed {
		return UnarmedAttackBonus(stats, class, level)
	}
	if thrown {
		dexMod := StatMod(GetStatFromMap(stats, "dexterity"))
		weaponType, _ := item["type"].(string)
		weaponID, _ := item["id"].(string)
		prof := 0
		if IsProficientWith(class, weaponType, weaponID) {
			prof = proficiencyBonus(level)
		}
		return dexMod + prof
	}
	return WeaponAttackBonus(item, stats, class, level)
}

// resolveAttackAdvantage returns >0 (advantage), <0 (disadvantage), or 0 (normal).
// Phase 2: ranged-at-melee-range, long-range, heavy weapon + small race.
// r is the range to the target.
func resolveAttackAdvantage(r int, item map[string]interface{}, isUnarmed bool, race string, thrown bool) int {
	if isUnarmed || item == nil {
		return 0
	}

	advantage := 0
	weaponType, _ := item["type"].(string)
	actingAsRanged := IsRangedAction(weaponType) || thrown

	if actingAsRanged {
		// Disadvantage when firing at an adjacent target (within 5 ft)
		if r <= 1 {
			advantage--
		}
		// Disadvantage when beyond normal range (but still within long range)
		normalRange, _ := getRangedReach(item)
		if r > normalRange {
			advantage--
		}
	}

	// Heavy weapons impose disadvantage for small races
	if hasTag(item["tags"], "heavy") {
		if strings.EqualFold(race, "halfling") || strings.EqualFold(race, "gnome") {
			advantage--
		}
	}

	return advantage
}

// validateAttackRange returns an error if the range to the target (r) prevents this attack.
func validateAttackRange(r int, item map[string]interface{}, isUnarmed, thrown bool) error {
	if isUnarmed || item == nil {
		if r > 1 {
			return fmt.Errorf("enemy is out of melee range — move closer or use a ranged weapon")
		}
		return nil
	}

	weaponType, _ := item["type"].(string)
	isRanged := IsRangedAction(weaponType)

	if isRanged || thrown {
		normalRange, longRange := getRangedReach(item)
		maxRange := longRange
		if maxRange == 0 {
			maxRange = normalRange
		}
		if r > maxRange {
			return fmt.Errorf("target is beyond maximum range (%d)", maxRange)
		}
		return nil
	}

	// Melee range gate
	reach := getMeleeReach(item)
	if r > reach {
		return fmt.Errorf("enemy is out of melee range (weapon reach: %d, current range: %d) — move closer", reach, r)
	}
	return nil
}

// validateTwoWeaponFighting checks conditions for a bonus action off-hand attack.
func validateTwoWeaponFighting(db *sql.DB, save *types.SaveFile, cs *types.CombatSession) error {
	if len(cs.Party) == 0 {
		return fmt.Errorf("no player in combat")
	}
	if cs.Party[0].CombatState.BonusActionUsed {
		return fmt.Errorf("bonus action already used this turn")
	}

	// Load main-hand item (try lowercase then camelCase slot names)
	mainItem, mainUnarmed, _ := loadWeaponItem(db, save.Inventory, "mainhand")
	if mainUnarmed || mainItem == nil {
		mainItem, mainUnarmed, _ = loadWeaponItem(db, save.Inventory, "mainHand")
	}
	if mainUnarmed || mainItem == nil {
		return fmt.Errorf("no weapon in main hand for two-weapon fighting")
	}

	// Load off-hand item
	offItem, offUnarmed, _ := loadWeaponItem(db, save.Inventory, "offhand")
	if offUnarmed || offItem == nil {
		return fmt.Errorf("no weapon in off hand for two-weapon fighting")
	}

	if !hasTag(mainItem["tags"], "light") {
		return fmt.Errorf("main hand weapon must be light for two-weapon fighting")
	}
	if !hasTag(offItem["tags"], "light") {
		return fmt.Errorf("off hand weapon must be light for two-weapon fighting")
	}
	if hasTag(mainItem["tags"], "loading") {
		return fmt.Errorf("cannot use two-weapon fighting with a loading weapon")
	}
	return nil
}

// consumeAmmo removes one unit of ammo from the ammo gear slot and increments
// the session's ammo-used counter. Returns an error if no ammo is equipped.
func consumeAmmo(save *types.SaveFile, cs *types.CombatSession, weapon map[string]interface{}) error {
	gearSlots, ok := save.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return errNoAmmo()
	}

	for _, key := range []string{"ammunition", "ammo"} {
		slotMap, ok := gearSlots[key].(map[string]interface{})
		if !ok {
			continue
		}
		itemID, _ := slotMap["item"].(string)
		if itemID == "" {
			continue
		}

		// A quiver (or any container) in the ammo slot: draw one round from its
		// contents rather than consuming the quiver itself.
		if contents, ok := slotMap["contents"].([]interface{}); ok {
			if consumeFromAmmoContents(contents, weapon) {
				cs.AmmoUsedThisCombat++
				return nil
			}
			continue // container present but has no usable ammo inside
		}

		// Raw ammo sitting directly in the slot.
		if ammoMatchesWeapon(itemID, weapon) {
			qty := slotQty(slotMap, "quantity")
			if qty <= 1 {
				gearSlots[key] = map[string]interface{}{"item": nil, "quantity": 0}
			} else {
				slotMap["quantity"] = qty - 1
				gearSlots[key] = slotMap
			}
			cs.AmmoUsedThisCombat++
			return nil
		}
	}
	return errNoAmmo()
}

func errNoAmmo() error {
	return fmt.Errorf("no ammunition — equip a quiver with ammo (or ammo) in the ammo slot")
}

// consumeFromAmmoContents removes one round from a container's contents: a first
// pass prefers ammo matching the weapon, a second pass takes any ammo so a
// mismatched-but-present round still fires. Returns false if the container is empty.
func consumeFromAmmoContents(contents []interface{}, weapon map[string]interface{}) bool {
	for _, matchOnly := range []bool{true, false} {
		for _, c := range contents {
			slotMap, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			id, _ := slotMap["item"].(string)
			if id == "" {
				continue
			}
			qty := slotQty(slotMap, "quantity")
			if qty <= 0 {
				continue
			}
			if matchOnly && !ammoMatchesWeapon(id, weapon) {
				continue
			}
			if qty <= 1 {
				slotMap["item"] = nil
				slotMap["quantity"] = 0
			} else {
				slotMap["quantity"] = qty - 1
			}
			return true
		}
	}
	return false
}

// ammoMatchesWeapon loosely matches an ammo item id against the weapon's
// "ammunition" label (bows→"arrows", crossbows→"bolts", …). That label is
// inconsistent in the data, so both are normalized and substring-matched; a
// weapon with no label accepts any ammo.
func ammoMatchesWeapon(ammoID string, weapon map[string]interface{}) bool {
	if weapon == nil {
		return true
	}
	want, _ := weapon["ammunition"].(string)
	w := normalizeAmmoToken(want)
	a := normalizeAmmoToken(ammoID)
	if w == "" || a == "" {
		return true
	}
	return strings.Contains(a, w) || strings.Contains(w, a)
}

func normalizeAmmoToken(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return strings.TrimSuffix(b.String(), "s")
}

// consumeFromGearSlot decrements a gear slot item by 1, clearing the slot when
// quantity reaches zero. Tries both the exact slot name and its lowercase form.
func consumeFromGearSlot(inventory map[string]interface{}, slot string) error {
	gearSlots, ok := inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid gear_slots structure")
	}

	var slotData interface{}
	var slotKey string
	for _, key := range []string{slot, strings.ToLower(slot)} {
		if sd, exists := gearSlots[key]; exists && sd != nil {
			slotData = sd
			slotKey = key
			break
		}
	}

	if slotData == nil {
		return fmt.Errorf("no item in slot %s to consume", slot)
	}

	slotMap, ok := slotData.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid slot data for %s", slot)
	}

	qty := slotQty(slotMap, "quantity")
	if qty <= 1 {
		gearSlots[slotKey] = map[string]interface{}{"item": nil, "quantity": 0}
	} else {
		slotMap["quantity"] = qty - 1
		gearSlots[slotKey] = slotMap
	}
	return nil
}

// slotQty safely reads an integer quantity from an inventory slot map.
func slotQty(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// formatAttackRoll returns the roll-only line — "X attacks with Y: rolled N+M".
// Callers should append outcomeLine(result) immediately after to surface the
// hit/miss/crit verdict as a separate beat (drives the dice animation pacing
// on the frontend).
func formatAttackRoll(playerName string, item map[string]interface{}, isUnarmed bool, result AttackResult) string {
	weapon := "Unarmed Strike"
	if !isUnarmed && item != nil {
		if n, ok := item["name"].(string); ok {
			weapon = n
		}
	}
	return fmt.Sprintf("  %s attacks with %s: rolled %d%s",
		playerName, weapon, result.Roll, formatModifier(result.Modifier))
}

// outcomeLine renders the hit/miss/crit verdict as its own log entry.
func outcomeLine(r AttackResult) string {
	switch {
	case r.IsCrit:
		return "  💥 CRITICAL HIT!"
	case r.IsCritMiss:
		return "  ✘ Critical miss!"
	case r.IsHit:
		return fmt.Sprintf("  ⚔ HIT! (vs AC %d)", r.Total)
	default:
		return "  ✘ MISS"
	}
}

// resolvePlayerDamage rolls damage and applies monster resistances/immunities/vulnerabilities.
// When thrown is true the weapon uses DEX modifier for damage instead of the normal ability.
func resolvePlayerDamage(item map[string]interface{}, stats map[string]interface{}, monster *types.MonsterInstance, isUnarmed, offhandEmpty, isCrit, thrown bool) int {
	if isUnarmed {
		return ResolveDamageToMonster("1d4", StatMod(GetStatFromMap(stats, "strength")), "bludgeoning", isCrit, monster)
	}
	abilityMod := WeaponDamageBonus(item, stats)
	if thrown {
		abilityMod = StatMod(GetStatFromMap(stats, "dexterity"))
	}
	return ResolveDamageToMonster(
		WeaponDamageDice(item, offhandEmpty),
		abilityMod,
		WeaponDamageType(item),
		isCrit, monster,
	)
}

// resolvePlayerDamageNoMod rolls weapon damage without any ability score modifier.
// Used for two-weapon fighting off-hand attacks.
func resolvePlayerDamageNoMod(item map[string]interface{}, monster *types.MonsterInstance, offhandEmpty, isCrit bool) int {
	if item == nil {
		return ResolveDamageToMonster("1d4", 0, "bludgeoning", isCrit, monster)
	}
	return ResolveDamageToMonster(
		WeaponDamageDice(item, offhandEmpty),
		0,
		WeaponDamageType(item),
		isCrit, monster,
	)
}

// formatDamage returns a narrative log line describing damage dealt.
func formatDamage(item map[string]interface{}, isUnarmed bool, dmg int, isCrit bool) string {
	dmgType := "bludgeoning"
	if !isUnarmed && item != nil {
		dmgType = WeaponDamageType(item)
	}
	crit := ""
	if isCrit {
		crit = " Critical hit!"
	}
	return fmt.Sprintf("  You deal %d %s damage.%s", dmg, dmgType, crit)
}

// isOffhandEmpty returns true when nothing is equipped in the offhand slot.
func isOffhandEmpty(inventory map[string]interface{}) bool {
	return gaminventory.GetEquippedItemID(inventory, "offhand") == ""
}

// applyDamageToMonster reduces monster HP and marks it dead when HP reaches zero.
func applyDamageToMonster(monster *types.MonsterInstance, dmg int) {
	monster.CurrentHP -= dmg
	if monster.CurrentHP <= 0 {
		monster.CurrentHP = 0
		monster.IsAlive = false
	}
}

// awardDamageXP computes XP for the hit and adds it to the session total. The
// reward is per hit (kept even on a flee): base XP from damage, scaled by the
// player's level multiplier through the single character.BonusXP path.
func awardDamageXP(cs *types.CombatSession, monster *types.MonsterInstance, dmg, timeOfDay, level int, advancement []types.AdvancementEntry) int {
	xp := character.BonusXP(level, XPForDamage(monster, dmg, NightMultiplier(timeOfDay)), advancement)
	cs.XPEarnedThisFight += xp
	return xp
}

// handleMonsterKill processes a monster's death: rolls its loot onto the fight's
// pile, awards the kill bonus, checks for a level-up, and ends the fight if it
// was the last enemy standing.
func handleMonsterKill(cs *types.CombatSession, monster *types.MonsterInstance, save *types.SaveFile, advancement []types.AdvancementEntry) []string {
	log := []string{fmt.Sprintf("  %s is defeated!", monster.Name)}

	// Feed the kill to the event recorder so "slay" quest objectives advance.
	// No-op until a consumer is subscribed at startup.
	events.Record(save, events.MonsterKilled, monster.Data.ID, 1)

	cs.LootRolled = append(cs.LootRolled, RollLoot(monster.Data.LootTable)...)

	// Kill bonus: flat XP for the kill itself (set on tougher monsters, and on
	// POI/dungeon steps via the node walker in M3), on top of the proportional
	// damage XP accrued during the fight.
	killLevel := character.GetLevelFromXP(save.Experience, advancement)
	if bonus := character.BonusXP(killLevel, KillBonusXP(&monster.Data), advancement); bonus > 0 {
		cs.XPEarnedThisFight += bonus
		log = append(log, fmt.Sprintf("  +%d bonus XP for slaying %s!", bonus, monster.Name))
	}

	if !cs.LevelUpPending && character.WillLevelUp(save.Experience, cs.XPEarnedThisFight, advancement) {
		cs.LevelUpPending = true
		log = append(log, "  Level up!")
	}

	return append(log, checkVictory(cs)...)
}

// checkVictory ends the fight once no enemy is left in it (all slain or fled).
// Loot already rolled from the kills stays on the pile. Returns the closing log
// line, or nil while the fight goes on.
func checkVictory(cs *types.CombatSession) []string {
	if cs.Phase != "active" && cs.Phase != "death_saves" {
		return nil
	}
	if len(activeMonsters(cs)) > 0 {
		return nil
	}
	cs.Phase = "loot"
	for _, m := range cs.Monsters {
		if !m.IsAlive {
			return []string{fmt.Sprintf("  Victory! +%d XP this fight.", cs.XPEarnedThisFight)}
		}
	}
	return []string{"  The battlefield falls quiet — your foes have fled."}
}

// executePlayerOA resolves the player's opportunity attack against the monster
// as it leaves their reach. Uses the main-hand weapon at normal attack bonus,
// no advantage (unlike the readied counter-attack). Marks ReactionUsed.
func executePlayerOA(cs *types.CombatSession, save *types.SaveFile, db *sql.DB, monster *types.MonsterInstance) []string {
	if len(cs.Party) == 0 {
		return nil
	}
	state := &cs.Party[0].CombatState
	if state.ReactionUsed {
		return nil
	}
	state.ReactionUsed = true

	advancement, err := character.LoadAdvancement(db)
	if err != nil {
		return []string{"  (Opportunity attack failed — could not load data.)"}
	}

	item, isUnarmed, _ := loadWeaponItem(db, save.Inventory, "mainHand")
	if item == nil && !isUnarmed {
		item, isUnarmed, _ = loadWeaponItem(db, save.Inventory, "mainhand")
	}
	// Ranged weapons don't get OAs (must be melee)
	if !isUnarmed && item != nil {
		weaponType, _ := item["type"].(string)
		if IsRangedAction(weaponType) {
			return []string{"  (You swing wide as it retreats — can't make an opportunity attack with a ranged weapon.)"}
		}
	}

	level := character.GetLevelFromXP(save.Experience, advancement)
	attackBonus := resolveAttackBonus(item, effectiveStats(save), save.Class, level, isUnarmed, false)
	result := ResolveAttackRoll(attackBonus, monster.ArmorClass, 0)

	weaponName := "Unarmed Strike"
	if !isUnarmed && item != nil {
		if n, ok := item["name"].(string); ok {
			weaponName = n
		}
	}
	log := []string{
		fmt.Sprintf(
			"  ⚡ Opportunity attack! You strike with %s: rolled %d%s",
			weaponName, result.Roll, formatModifier(result.Modifier),
		),
		outcomeLine(result),
	}
	if !result.IsHit {
		return log
	}
	offhandEmpty := isOffhandEmpty(save.Inventory)
	dmg := resolvePlayerDamage(item, effectiveStats(save), monster, isUnarmed, offhandEmpty, result.IsCrit, false)
	log = append(log, formatDamage(item, isUnarmed, dmg, result.IsCrit))
	applyDamageToMonster(monster, dmg)

	xp := awardDamageXP(cs, monster, dmg, save.TimeOfDay, level, advancement)
	if xp > 0 {
		log = append(log, fmt.Sprintf("  +%d XP", xp))
	}

	if !monster.IsAlive {
		log = append(log, handleMonsterKill(cs, monster, save, advancement)...)
	}
	return log
}

// PlayerMeleeReachForSave is an exported helper for the API layer to report the
// player's current melee reach in state responses.
func PlayerMeleeReachForSave(db *sql.DB, save *types.SaveFile) int {
	return getPlayerMeleeReach(db, save)
}

// getPlayerMeleeReach returns the melee reach of the player's main-hand weapon.
func getPlayerMeleeReach(db *sql.DB, save *types.SaveFile) int {
	item, isUnarmed, _ := loadWeaponItem(db, save.Inventory, "mainHand")
	if item == nil {
		item, isUnarmed, _ = loadWeaponItem(db, save.Inventory, "mainhand")
	}
	if isUnarmed || item == nil {
		return 1 // Unarmed melee reach
	}
	return getMeleeReach(item)
}

// executeReadiedAttack fires the player's counter-attack when their readied stance triggers.
// The attack is made with advantage (they were braced and waiting).
// Returns log entries and true if the monster was killed.
func executeReadiedAttack(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, monster *types.MonsterInstance) ([]string, bool) {
	advancement, err := character.LoadAdvancement(db)
	if err != nil {
		return []string{"  (Readied attack failed — could not load data.)"}, false
	}

	item, isUnarmed, _ := loadWeaponItem(db, save.Inventory, "mainHand")
	if item == nil && !isUnarmed {
		item, isUnarmed, _ = loadWeaponItem(db, save.Inventory, "mainhand")
	}

	level := character.GetLevelFromXP(save.Experience, advancement)
	attackBonus := resolveAttackBonus(item, effectiveStats(save), save.Class, level, isUnarmed, false)
	result := ResolveAttackRoll(attackBonus, monster.ArmorClass, 1) // Advantage: player was ready

	log := []string{formatAttackRoll(save.D, item, isUnarmed, result), outcomeLine(result)}
	if !result.IsHit {
		return log, false
	}

	offhandEmpty := isOffhandEmpty(save.Inventory)
	dmg := resolvePlayerDamage(item, effectiveStats(save), monster, isUnarmed, offhandEmpty, result.IsCrit, false)
	log = append(log, formatDamage(item, isUnarmed, dmg, result.IsCrit))
	applyDamageToMonster(monster, dmg)

	xp := awardDamageXP(cs, monster, dmg, save.TimeOfDay, level, advancement)
	if xp > 0 {
		log = append(log, fmt.Sprintf("  +%d XP", xp))
	}

	if !monster.IsAlive {
		log = append(log, handleMonsterKill(cs, monster, save, advancement)...)
		return log, true
	}
	return log, false
}

// computePlayerAC queries the player's current AC from equipped items.
func computePlayerAC(db *sql.DB, save *types.SaveFile) int {
	return CalculatePlayerAC(db, save.Inventory, effectiveStats(save))
}

// applyDamageToPlayer deducts HP and transitions to death_saves if HP reaches zero.
// Returns any resulting log lines (e.g., the player going unconscious).
func applyDamageToPlayer(cs *types.CombatSession, dmg int) []string {
	if len(cs.Party) == 0 {
		return nil
	}
	state := &cs.Party[0].CombatState

	var log []string
	// Rage soaks a slice of incoming damage.
	if state.RageResistPct > 0 && dmg > 0 {
		if reduced := dmg * state.RageResistPct / 100; reduced > 0 {
			dmg -= reduced
			log = append(log, fmt.Sprintf("  🪓 Rage absorbs %d damage.", reduced))
		}
	}
	// Taking a hit fuels the barbarian's Rage pool.
	if dmg > 0 && state.Resource != nil {
		regenResource(state.Resource, state.Resource.PerHitTaken)
	}

	state.CurrentHP -= dmg
	if state.CurrentHP <= 0 {
		state.CurrentHP = 0
		state.IsUnconscious = true
		cs.Phase = "death_saves"
		return []string{"  You fall unconscious. Make death saving throws."}
	}
	return nil
}

// addDeathSaveFailures adds N failures and transitions to defeat when total reaches 3.
func addDeathSaveFailures(state *types.PlayerCombatState, cs *types.CombatSession, n int) {
	state.DeathSaveFailures += n
	if state.DeathSaveFailures >= 3 {
		cs.Phase = "defeat"
	}
}

// resolveDeathSaveRoll applies the roll result to death save counters.
func resolveDeathSaveRoll(state *types.PlayerCombatState, cs *types.CombatSession, roll int) string {
	switch {
	case roll == 20:
		return reviveFromDeathSave(state, cs)
	case roll == 1:
		return twoDeathSaveFailures(state, cs)
	case roll >= 10:
		return oneDeathSaveSuccess(state, cs)
	default:
		return oneDeathSaveFailure(state, cs)
	}
}

// reviveFromDeathSave handles a natural 20: player regains 1 HP and consciousness.
func reviveFromDeathSave(state *types.PlayerCombatState, cs *types.CombatSession) string {
	state.CurrentHP = 1
	state.IsUnconscious = false
	state.DeathSaveSuccesses = 0
	state.DeathSaveFailures = 0
	cs.Phase = "active"
	return "  Natural 20! You regain consciousness with 1 HP."
}

// twoDeathSaveFailures handles a natural 1: counts as two failures.
func twoDeathSaveFailures(state *types.PlayerCombatState, cs *types.CombatSession) string {
	addDeathSaveFailures(state, cs, 2)
	if cs.Phase == "defeat" {
		return "  Natural 1 — two failures. You have died."
	}
	return fmt.Sprintf("  Natural 1 — two failures. (%d/3 failures)", state.DeathSaveFailures)
}

// oneDeathSaveSuccess handles a roll of 10+: one success, stable at three.
func oneDeathSaveSuccess(state *types.PlayerCombatState, cs *types.CombatSession) string {
	state.DeathSaveSuccesses++
	if state.DeathSaveSuccesses >= 3 {
		state.IsStable = true
		cs.Phase = "victory"
		return "  Success! You are now stable."
	}
	return fmt.Sprintf("  Success. (%d/3 successes)", state.DeathSaveSuccesses)
}

// oneDeathSaveFailure handles a roll of 2–9: one failure, dead at three.
func oneDeathSaveFailure(state *types.PlayerCombatState, cs *types.CombatSession) string {
	addDeathSaveFailures(state, cs, 1)
	if cs.Phase == "defeat" {
		return "  Failure. You have died."
	}
	return fmt.Sprintf("  Failure. (%d/3 failures)", state.DeathSaveFailures)
}

// applyDeathSaveHit records 1 (hit) or 2 (crit) automatic death save failures.
// Returns a description of the outcome.
func applyDeathSaveHit(cs *types.CombatSession, isCrit bool) string {
	state := &cs.Party[0].CombatState
	n := 1
	if isCrit {
		n = 2
	}
	addDeathSaveFailures(state, cs, n)
	if cs.Phase == "defeat" {
		return fmt.Sprintf("  Hit — %d failure(s). You have died.", n)
	}
	return fmt.Sprintf("  Hit — %d failure(s). (%d/3 failures)", n, state.DeathSaveFailures)
}

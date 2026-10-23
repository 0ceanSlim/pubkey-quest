package combat

import (
	"database/sql"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/encounter"
	"pubkey-quest/cmd/server/game/events"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)


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
	EnvironmentID string   // where the fight is (for the log / analytics)
	Surprise      bool     // the party is caught off guard: every monster acts first in round 1
	// MonstersSurprised: the party crept up unseen and attacked (a sneak
	// attack). The party acts first, every monster loses its first turn, and
	// the party's attacks that first turn have advantage.
	MonstersSurprised bool
	// Elite toughens the first monster for a bigger party: the number of
	// extra players (0 = a normal monster).
	Elite int
	// Arena is an authored terrain layout (rows of combat-terrain arena_key
	// characters) used instead of generating terrain for the battlefield.
	Arena []string
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
	return StartPartyEncounter(db, []FightMember{{Npub: npub, SaveID: save.InternalID, Save: save}}, spec, advancement)
}

// FightMember is one player joining a fight: who, and their live save.
type FightMember struct {
	Npub   string
	SaveID string
	Save   *types.SaveFile
}

// StartPartyEncounter starts a fight for one or more players (the first is the
// party leader). Players line up on the left, monsters on the right; everyone
// rolls initiative and keeps their own place in the order.
func StartPartyEncounter(db *sql.DB, members []FightMember, spec EncounterSpec, advancement []types.AdvancementEntry) (*types.CombatSession, error) {
	if len(members) == 0 {
		return nil, fmt.Errorf("StartEncounter: no players")
	}
	if len(spec.MonsterIDs) == 0 {
		return nil, fmt.Errorf("StartEncounter: no monsters")
	}
	if len(spec.MonsterIDs) > maxEncounterMonsters {
		spec.MonsterIDs = spec.MonsterIDs[:maxEncounterMonsters]
	}
	leader := members[0].Save

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
	if spec.Elite > 0 {
		makeElite(&monsters[0], spec.Elite)
	}

	party := make([]types.PartyCombatant, 0, len(members))
	for _, m := range members {
		p := newPlayerCombatant(m.Npub, m.Save)
		p.SaveID = m.SaveID
		p.Name = m.Save.D
		party = append(party, p)
	}
	width, height := GridSizeFor(len(party) + len(monsters))
	// Players stand in the left column, fanning out from the middle row.
	mid := height / 2
	for i := range party {
		off := (i + 1) / 2
		if i%2 == 1 {
			off = -off
		}
		y := mid + off
		x := 1
		if y < 0 || y >= height {
			x, y = 0, mid+off/2
		}
		party[i].Pos = types.Position{X: x, Y: y}
	}

	cs := &types.CombatSession{
		Party:         party,
		Monsters:      monsters,
		Round:         1,
		GridWidth:     width,
		GridHeight:    height,
		EnvironmentID: spec.EnvironmentID,
		IsSurprised:   spec.Surprise,
		Phase:         "active",
	}
	// Terrain: an authored arena, or scattered from where the fight is.
	loadTerrainConfig(db)
	cs.Battlefield = BattlefieldFor(db, leader)
	if len(spec.Arena) > 0 {
		applyArena(cs, spec.Arena)
	} else {
		generateTerrain(cs, rand.New(rand.NewSource(time.Now().UnixNano())))
	}
	for i := range cs.Party { // an authored arena can't wall a player in
		p := cs.Party[i].Pos
		if len(cs.Terrain) == width*height {
			cs.Terrain[p.Y*width+p.X] = ""
		}
	}
	placeMonsters(cs, scaleRangeToGrid(startingRangeFor(cs.Battlefield), width))

	// Seed each martial class's resource pool (Rage/Stamina/Ki/Cunning).
	levelSum := 0
	for i, m := range members {
		level := character.GetLevelFromXP(m.Save.Experience, advancement)
		levelSum += level
		InitResourcePool(&cs.Party[i].CombatState, m.Save.Class, level, m.Save.Stats)
	}
	// Rate the whole group against the party's level band (M5 §22 difficulty
	// guardrail): several weak monsters can add up to a deadly fight.
	cs.Difficulty = encounter.GroupDifficulty(crs, (levelSum+len(members)/2)/len(members), len(cs.Party))

	cs.Log = append(cs.Log, encounterOpeningLine(cs))
	switch cs.Difficulty {
	case "deadly":
		cs.Log = append(cs.Log, fmt.Sprintf("  ⚠️ %s looks deadly — you may want to flee.", foeLabel(cs)))
	case "tough":
		cs.Log = append(cs.Log, fmt.Sprintf("  ⚠️ %s looks like a tough fight.", foeLabel(cs)))
	}

	// Initiative is still rolled for the order of later rounds, but a sneak
	// attack doesn't show the dice: you already have the first move.
	sneak := spec.MonstersSurprised && !spec.Surprise
	initLog := rollEncounterInitiative(cs, members)
	if !sneak {
		cs.Log = append(cs.Log, initLog...)
	}
	if spec.Surprise {
		cs.Log = append(cs.Log, "⚡ You're caught off guard!")
	}
	if sneak {
		for i := range cs.Monsters {
			cs.Monsters[i].Surprised = true
		}
		cs.Initiative = partyFirst(cs.Initiative)
		cs.Log = append(cs.Log, "⚡ They never saw you coming — you strike first!")
		for i := range cs.Party {
			cs.Party[i].CombatState.Ambushing = true
		}
	}

	if cs.Initiative[0].Type == "monster" {
		cs.Log = append(cs.Log, fmt.Sprintf("⚡ %s goes first!", cs.Initiative[0].Name))
		cs.Log = append(cs.Log, runOpeningTurns(db, cs, leader)...)
	} else {
		cs.CurrentTurnIndex = 0
		cs.Actor = MemberIndex(cs, cs.Initiative[0].ID)
		first := actor(cs)
		ambushing := first.CombatState.Ambushing
		startPlayerTurn(cs, saveOf(first, members[cs.Actor].Save))
		first.CombatState.Ambushing = ambushing
		if PartyFight(cs) {
			cs.Log = append(cs.Log, fmt.Sprintf("⚡ %s goes first!", first.Name))
		} else {
			cs.Log = append(cs.Log, "⚡ You go first!")
		}
	}

	return cs, nil
}

// makeElite toughens a monster for a bigger party (party-combat-plan §1): +50%
// HP and +1 to hit per extra player.
func makeElite(m *types.MonsterInstance, extraPlayers int) {
	m.MaxHP += m.MaxHP * extraPlayers / 2
	m.CurrentHP = m.MaxHP
	for i := range m.Data.Actions {
		m.Data.Actions[i].AttackBonus += extraPlayers
	}
	m.Name = "Elite " + m.Name
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
func rollEncounterInitiative(cs *types.CombatSession, members []FightMember) []string {
	log := []string{"⚡ Rolling initiative…"}
	var entries []types.InitiativeEntry
	for i, m := range members {
		dex := GetStatFromMap(effectiveStats(m.Save), "dexterity")
		roll := rollInitiative(StatMod(dex))
		name := cs.Party[i].Name
		who := "You"
		if PartyFight(cs) {
			who = name
		} else {
			name = "You"
		}
		entries = append(entries, types.InitiativeEntry{ID: cs.Party[i].ID, Type: "player", Name: name, Initiative: roll.Total, DEXScore: dex})
		log = append(log, fmt.Sprintf("  %s rolled %d%s", who, roll.Face, formatModifier(roll.Mod)))
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

// partyFirst reorders initiative so every party entry acts before any monster
// (the order within each side is kept) — used when the monsters are surprised.
func partyFirst(entries []types.InitiativeEntry) []types.InitiativeEntry {
	out := make([]types.InitiativeEntry, 0, len(entries))
	for _, e := range entries {
		if e.Type == "player" {
			out = append(out, e)
		}
	}
	for _, e := range entries {
		if e.Type != "player" {
			out = append(out, e)
		}
	}
	return out
}

// newPlayerCombatant snapshots the player's current HP into combat state. The
// caller places it once the grid size is known.
func newPlayerCombatant(npub string, save *types.SaveFile) types.PartyCombatant {
	return types.PartyCombatant{
		Type:               "player",
		ID:                 npub,
		IsPlayerControlled: true,
		CombatState: types.PlayerCombatState{
			CurrentHP:      save.HP,
			MaxHP:          save.MaxHP,
			MovementBudget: playerMovementBudget(save.Race) + effects.ModifierTotal(save, "combat_move"),
			// Spells cast before the fight carry in: false life's temporary
			// HP, a familiar's help on your first attack.
			TempHP: effects.ModifierTotal(save, "temp_hp"),
			Helped: effects.ModifierTotal(save, "combat_help") > 0,
			// Weapon coatings applied before the fight (basic poison, a honed edge).
			SpellBuffs: takeCoatings(save),
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
	me := actor(cs)
	if occupied(cs, target, me.ID) {
		return nil, fmt.Errorf("that space is occupied")
	}
	if !passable(cs, target) {
		return nil, fmt.Errorf("you can't stand there (%s)", terrainName(cs, terrainAt(cs, target)))
	}

	state := &me.CombatState
	if IsSpeedZero(state.Conditions) {
		return nil, fmt.Errorf("you're held fast and can't move")
	}
	if me.Pos == target {
		return nil, fmt.Errorf("already at that position")
	}
	// Terrain makes some cells cost more than one movement: find the cheapest
	// route there (around cover and through no one).
	pm := shortestPaths(cs, me.Pos, moveRules{moverID: me.ID, isPlayer: true, planCost: playerPlanCost(cs)})
	path := pm.pathTo(target)
	if path == nil {
		return nil, fmt.Errorf("there's no way through to that space")
	}
	dist := pm.cost[target]
	remaining := state.MovementBudget - state.MovementSpent
	if dist > remaining {
		return nil, fmt.Errorf("not enough movement — that costs %d, you have %d left", dist, remaining)
	}

	// Traps along the way (caltrops, ball bearings): a failed save can end the
	// move on the trap, with the rest of this turn's movement lost.
	var trapLog []string
	trapDmg, trapped := 0, false
	for i, p := range path {
		l, d, stop := playerTrap(cs, save, p)
		trapLog = append(trapLog, l...)
		trapDmg += d
		if stop {
			path, target, trapped = path[:i+1], p, true
			dist = pm.cost[p]
			break
		}
	}

	// Ranges to every active monster before the move, for the OA check below.
	before := map[string]int{}
	for _, m := range activeMonsters(cs) {
		before[m.InstanceID] = rangeTo(cs, m)
	}
	prevRange := nearestRange(cs)
	me.Pos = target
	state.MovementSpent += dist
	if trapped {
		state.MovementSpent = state.MovementBudget
	}
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

	log = append(log, trapLog...)
	if trapDmg > 0 {
		log = append(log, damagePlayer(cs, save, trapDmg)...)
		if cs.Phase != "active" {
			return log, nil
		}
	}

	// Hazards burn on the way through.
	for _, p := range path {
		log = append(log, playerHazard(cs, save, p, "step into")...)
		if cs.Phase != "active" {
			return log, nil
		}
	}

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
	playerAC := computePlayerAC(db, cs, save)

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
	log = append(log, fmt.Sprintf("  %s deals %d %s damage (%s).%s", monster.Name, dmg, action.Hit.Type, DiceLabel(action.Hit.Dice, action.Hit.Mod, result.IsCrit), crit))
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
	state := &actor(cs).CombatState
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
	state := &actor(cs).CombatState
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
	for _, b := range actor(cs).CombatState.SpellBuffs {
		if b.EasyEscape {
			return escapeFight(cs, fmt.Sprintf("  Under cover of %s, you slip away unseen!", b.Name)), nil
		}
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
		return append(log, escapeFight(cs, "  You manage to put enough distance between you and the enemy to escape!")...), nil
	}

	// Escape failed — player's action is spent; monster responds when player ends turn
	state := &actor(cs).CombatState
	state.ActionUsed = true
	log = append(log, fmt.Sprintf("  %s cuts off your escape! You're still in combat.", monster.Name))
	return log, nil
}

// escapeFight takes the acting player out of the fight. Solo, the fight ends
// without a kill (XP already earned is kept, no loot); in a party they slip
// away alone and the others fight on. Shared by a successful flee and a smoke
// bomb.
func escapeFight(cs *types.CombatSession, line string) []string {
	if PartyFight(cs) {
		me := actor(cs)
		me.Fled = true
		me.Loot = nil
		log := []string{fmt.Sprintf("  %s breaks away and escapes the fight!", me.Name)}
		return append(log, partyAfterDeparture(cs)...)
	}
	cs.Phase = "loot"
	actor(cs).Loot = nil
	return []string{line}
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
	state := &actor(cs).CombatState
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
	// …and that there's a line to it. Cover raises its AC.
	visible, cover := Sight(cs, actor(cs).Pos, monster.Pos)
	if !visible {
		return nil, fmt.Errorf("you have no line of sight to %s — move to get a clear shot", monster.Name)
	}
	isRangedAttack := thrown || (!isUnarmed && item != nil && IsRangedAction(stringOf(item["type"])))

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

	// Shillelagh and the like: a melee weapon swung with the caster's
	// spellcasting ability, its damage die replaced.
	stats := effectiveStats(save)
	if !isRangedAttack && !isUnarmed {
		var overLog []string
		item, stats, overLog = meleeOverride(state, save, item, stats, gaminventory.GetEquippedItemID(save.Inventory, weaponSlot))
		log = append(log, overLog...)
	}

	attackBonus := resolveAttackBonus(item, stats, save.Class, level, isUnarmed, thrown)
	advantage := resolveAttackAdvantage(r, item, isUnarmed, save.Race, thrown)
	// Spell buffs: bless's d4, magic weapon's +1, true strike's advantage.
	buffBonus, buffAdv, buffLog := buffAttackRoll(state, true)
	attackBonus += buffBonus
	advantage += buffAdv
	log = append(log, buffLog...)
	// Conditions: the player's own conditions (poisoned/prone/…) impose disadvantage;
	// the target monster's (restrained/blinded/outlined/…) grant advantage.
	advantage += ConditionAttackAdvantage(state.Conditions, monster.Conditions)
	// Night outdoors: without darkvision or a light in hand, you swing blind.
	if inDarkness(cs, save) {
		advantage--
		log = append(log, "  🌑 It's dark — disadvantage (no darkvision, no light in hand).")
	}
	// A sneak attack's opening turn: they don't see it coming.
	if state.Ambushing {
		advantage++
	}
	// Allies: a Help from a friend, or a friend flanking the target in melee.
	if state.Helped {
		advantage++
		state.Helped = false
	}
	if !isRangedAttack && flanking(cs, actor(cs), monster) {
		advantage++
		log = append(log, "  ⚔ Flanking — advantage!")
	}
	// Terrain: shooting down from high ground helps; swinging while swimming doesn't.
	if isRangedAttack {
		attackBonus += highGroundBonus(cs, actor(cs).Pos, monster.Pos)
	} else if inDeepWater(cs, actor(cs).Pos) {
		advantage--
	}
	if cover > 0 {
		log = append(log, fmt.Sprintf("  🛡 %s has %s (+%d AC).", monster.Name, coverLabel(cover), cover))
	}
	result := ResolveAttackRoll(attackBonus, monster.ArmorClass+cover, advantage)

	log = append(log, formatAttackRoll(save.D, item, isUnarmed, result), outcomeLine(result))
	// Attacking ends sanctuary and invisibility, hit or miss.
	log = append(log, breakOnAttack(state)...)

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
		dmg = resolvePlayerDamage(item, stats, monster, isUnarmed, offhandEmpty, result.IsCrit, thrown)
	}
	log = append(log, formatDamage(item, isUnarmed, dmg, result.IsCrit, offhandEmpty))

	// Spell buffs that ride a weapon hit: divine favor, hunter's mark, a smite.
	weaponType := "bludgeoning"
	if !isUnarmed && item != nil {
		weaponType = WeaponDamageType(item)
	}
	extra, hitLog := buffHitDamage(cs, actor(cs), monster, !isRangedAttack, false, result.IsCrit, weaponType)
	dmg += extra
	log = append(log, hitLog...)

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
	if actor(cs).CombatState.BonusActionUsed {
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
				actor(cs).AmmoUsed++
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
			actor(cs).AmmoUsed++
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
	return fmt.Sprintf("  %s attacks with %s: rolled %d%s%s",
		playerName, weapon, result.Roll, formatModifier(result.Modifier), AdvNote(result))
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

// formatDamage returns a narrative log line describing damage dealt, naming
// the dice rolled ("You deal 9 slashing damage (1d8).") so the log and the
// dice animation can show them.
func formatDamage(item map[string]interface{}, isUnarmed bool, dmg int, isCrit, offhandEmpty bool) string {
	dmgType := "bludgeoning"
	dice := "1d4"
	if !isUnarmed && item != nil {
		dmgType = WeaponDamageType(item)
		dice = WeaponDamageDice(item, offhandEmpty)
	}
	crit := ""
	if isCrit {
		crit = " Critical hit!"
	}
	return fmt.Sprintf("  You deal %d %s damage (%s).%s", dmg, dmgType, DiceLabel(dice, 0, isCrit), crit)
}

// DiceLabel names a damage roll for the log: "1d8", "2d6+3", and doubled
// dice on a critical hit ("2d8"), as 5e rolls them.
func DiceLabel(dice string, mod int, isCrit bool) string {
	label := dice
	var n, sides int
	if _, err := fmt.Sscanf(dice, "%dd%d", &n, &sides); err == nil && n > 0 && sides > 0 {
		if isCrit {
			n *= 2
		}
		label = fmt.Sprintf("%dd%d", n, sides)
	}
	if mod != 0 {
		label += formatModifier(mod)
	}
	return label
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
	actor(cs).XPEarned += xp
	// Whoever hurts a monster most draws its attention.
	if monster.Threat == nil {
		monster.Threat = map[string]int{}
	}
	monster.Threat[actor(cs).ID] += dmg
	return xp
}

// handleMonsterKill processes a monster's death: rolls its loot onto the fight's
// pile, awards the kill bonus, checks for a level-up, and ends the fight if it
// was the last enemy standing.
func handleMonsterKill(cs *types.CombatSession, monster *types.MonsterInstance, save *types.SaveFile, advancement []types.AdvancementEntry) []string {
	return append(killRewards(cs, monster, save, advancement), checkVictory(cs)...)
}

// killRewards is a kill without the victory check: the "defeated" line, quest
// credit, loot and kill-bonus XP. An area spell that drops several monsters
// pays each kill, then checks for victory once.
func killRewards(cs *types.CombatSession, monster *types.MonsterInstance, save *types.SaveFile, advancement []types.AdvancementEntry) []string {
	log := []string{fmt.Sprintf("  %s is defeated!", monster.Name)}
	members := inFightMembers(cs)
	if len(members) == 0 {
		members = []*types.PartyCombatant{actor(cs)}
	}
	party := len(members) > 1

	// Everyone in the fight gets the kill for "slay" quest objectives.
	for _, m := range members {
		events.Record(saveOf(m, save), events.MonsterKilled, monster.Data.ID, 1)
	}

	// Loot: a solo kill pays the whole table; a party shares one table's worth.
	if party {
		distributePartyLoot(cs, monster, members)
	} else {
		members[0].Loot = append(members[0].Loot, RollLoot(monster.Data.LootTable)...)
	}

	// Kill bonus: flat XP for the kill itself (set on tougher monsters, and on
	// POI/dungeon steps via the node walker in M3), on top of the proportional
	// damage XP accrued during the fight. A party shares it.
	base := KillBonusXP(&monster.Data)
	if party && base > 0 {
		base = (base + len(members) - 1) / len(members)
	}
	for _, m := range members {
		ms := saveOf(m, save)
		level := character.GetLevelFromXP(ms.Experience, advancement)
		if bonus := character.BonusXP(level, base, advancement); bonus > 0 {
			m.XPEarned += bonus
			if party {
				log = append(log, fmt.Sprintf("  %s: +%d bonus XP.", m.Name, bonus))
			} else {
				log = append(log, fmt.Sprintf("  +%d bonus XP for slaying %s!", bonus, monster.Name))
			}
		}
		if !m.LevelUpPending && character.WillLevelUp(ms.Experience, m.XPEarned, advancement) {
			m.LevelUpPending = true
			if party {
				log = append(log, fmt.Sprintf("  %s is ready to level up!", m.Name))
			} else {
				log = append(log, "  Level up!")
			}
		}
	}
	return log
}

// distributePartyLoot shares one kill's loot across the party (party-combat-plan
// §4) without paying out a full table per member:
//   - guaranteed drops go to whoever did the most to the monster, except gold,
//     which is split evenly (the remainder to that top contributor);
//   - each member makes the table's random rolls at 1/N odds, so the party
//     collects about one table per kill between them.
func distributePartyLoot(cs *types.CombatSession, monster *types.MonsterInstance, members []*types.PartyCombatant) {
	table := monster.Data.LootTable
	n := len(members)
	top := members[0]
	for _, m := range members[1:] {
		if monster.Threat[m.ID] > monster.Threat[top.ID] {
			top = m
		}
	}
	for _, g := range table.Guaranteed {
		qty := RollRange(g.Quantity[0], g.Quantity[1])
		if qty <= 0 {
			continue
		}
		if g.Item == "gold-piece" {
			share, rest := qty/n, qty%n
			for _, m := range members {
				q := share
				if m == top {
					q += rest
				}
				if q > 0 {
					m.Loot = appendOrStack(m.Loot, types.LootDrop{Item: g.Item, Quantity: q})
				}
			}
			continue
		}
		top.Loot = appendOrStack(top.Loot, types.LootDrop{Item: g.Item, Quantity: qty})
	}
	rolls := table.Rolls
	if rolls <= 0 {
		rolls = 1
	}
	for _, m := range members {
		for i := 0; i < rolls; i++ {
			if rand.Intn(n) != 0 { // a 1-in-N chance at each roll
				continue
			}
			if drop := rollOneTier(table.Tiers); drop != nil {
				m.Loot = appendOrStack(m.Loot, *drop)
			}
		}
	}
}

// HealXPMultiplier scales healing XP against damage XP (party-combat-plan §5).
const HealXPMultiplier = 1.0

// awardHealXP pays a healer for HP restored to someone hurt in this fight, at
// the fight's average monster XP per HP. Only damage taken this fight counts
// (no overheal), and healing yourself pays half.
func awardHealXP(cs *types.CombatSession, healer, target *types.PartyCombatant, healed, timeOfDay, level int, advancement []types.AdvancementEntry) int {
	if healed <= 0 || healer == nil || target == nil {
		return 0
	}
	if healed > target.DamageTaken {
		healed = target.DamageTaken
	}
	target.DamageTaken -= healed
	totalXP, totalHP := 0, 0
	for i := range cs.Monsters {
		totalXP += cs.Monsters[i].Data.XP
		totalHP += cs.Monsters[i].MaxHP
	}
	if healed <= 0 || totalHP == 0 || totalXP == 0 {
		return 0
	}
	raw := float64(healed) * float64(totalXP) / float64(totalHP) * HealXPMultiplier * NightMultiplier(timeOfDay)
	if healer == target {
		raw /= 2
	}
	if raw < 1 {
		raw = 1
	}
	xp := character.BonusXP(level, int(raw), advancement)
	healer.XPEarned += xp
	return xp
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
			if PartyFight(cs) {
				return []string{"  Victory!"}
			}
			return []string{fmt.Sprintf("  Victory! +%d XP this fight.", actor(cs).XPEarned)}
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
	state := &actor(cs).CombatState
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
			"  ⚡ Opportunity attack! You strike with %s: rolled %d%s%s",
			weaponName, result.Roll, formatModifier(result.Modifier), AdvNote(result),
		),
		outcomeLine(result),
	}
	if !result.IsHit {
		return log
	}
	offhandEmpty := isOffhandEmpty(save.Inventory)
	dmg := resolvePlayerDamage(item, effectiveStats(save), monster, isUnarmed, offhandEmpty, result.IsCrit, false)
	log = append(log, formatDamage(item, isUnarmed, dmg, result.IsCrit, offhandEmpty))
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
	log = append(log, formatDamage(item, isUnarmed, dmg, result.IsCrit, offhandEmpty))
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

// computePlayerAC is the AC a monster rolls against: gear and lasting spells
// (PlayerAC), plus the current actor's in-fight spell buffs (shield, shield of
// faith, barkskin).
func computePlayerAC(db *sql.DB, cs *types.CombatSession, save *types.SaveFile) int {
	ac := PlayerAC(db, save)
	if cs != nil && len(cs.Party) > 0 {
		ac = buffedAC(&actor(cs).CombatState, ac)
	}
	return ac
}

// applyDamageToPlayer deducts HP and transitions to death_saves if HP reaches zero.
// Returns any resulting log lines (e.g., the player going unconscious).
func applyDamageToPlayer(cs *types.CombatSession, dmg int) []string {
	if len(cs.Party) == 0 {
		return nil
	}
	state := &actor(cs).CombatState

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
	// Temporary hit points (false life, armor of agathys) go first.
	dmg, soakLog := soakDamage(state, dmg)
	log = append(log, soakLog...)

	if dmg > state.CurrentHP {
		actor(cs).DamageTaken += state.CurrentHP
	} else if dmg > 0 {
		actor(cs).DamageTaken += dmg
	}
	state.CurrentHP -= dmg
	if state.CurrentHP <= 0 {
		if ward := deathWard(state, whoName(cs, actor(cs))); ward != nil {
			return append(log, ward...)
		}
		state.CurrentHP = 0
		state.IsUnconscious = true
		if PartyFight(cs) {
			// In a party, a downed member waits for help; only a wipe ends it.
			if len(standingMembers(cs)) == 0 {
				cs.Phase = "defeat"
				return []string{fmt.Sprintf("  %s falls. The whole party is down…", actor(cs).Name)}
			}
			return []string{fmt.Sprintf("  💀 %s falls unconscious! Heal them or help them up.", actor(cs).Name)}
		}
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
	reviveAtOneHP(state)
	cs.Phase = "active"
	return "  Natural 20! You regain consciousness with 1 HP."
}

// reviveAtOneHP puts a downed player back on their feet at 1 HP with fresh
// death saves.
func reviveAtOneHP(state *types.PlayerCombatState) {
	state.CurrentHP = 1
	state.IsUnconscious = false
	state.IsStable = false
	state.DeathSaveSuccesses = 0
	state.DeathSaveFailures = 0
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
		// Back on your feet at 1 HP; whatever's still here is still here.
		reviveAtOneHP(state)
		cs.Phase = "active"
		return "  Success! You drag yourself back to your feet with 1 HP."
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
	state := &actor(cs).CombatState
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

// stringOf reads a string out of a loosely-typed item field ("" otherwise).
func stringOf(v interface{}) string {
	s, _ := v.(string)
	return s
}

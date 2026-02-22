package combat

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/game/character"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// ─── StartCombat ─────────────────────────────────────────────────────────────

// StartCombat initialises a new CombatSession for a single-monster encounter.
// The session lives in server memory only — it is never written to the save file.
func StartCombat(db *sql.DB, save *types.SaveFile, npub, monsterID, environmentID string, advancement []types.AdvancementEntry) (*types.CombatSession, error) {
	monsterData, err := LoadMonsterByID(db, monsterID)
	if err != nil {
		return nil, fmt.Errorf("StartCombat: %w", err)
	}

	cs := initCombatSession(npub, save, monsterData, environmentID)

	playerDEXMod := StatMod(GetStatFromMap(save.Stats, "dexterity"))
	monsterDEXMod := StatMod(monsterData.Stats.Dexterity)
	playerInit, monsterInit := rollInitiatives(playerDEXMod, monsterDEXMod)

	playerDEX := GetStatFromMap(save.Stats, "dexterity")
	cs.Initiative = buildInitiativeOrder(npub, playerDEX, playerInit, monsterData.Stats.Dexterity, monsterInit, cs.Monsters[0])

	cs.Log = append(cs.Log,
		fmt.Sprintf("⚔️  Combat begins! %s appears. Starting range: %d.", cs.Monsters[0].Name, cs.Range),
	)

	if monsterHasFirstTurn(cs) {
		cs.Log = append(cs.Log, execMonsterOpeningTurn(db, cs, save)...)
	}

	return cs, nil
}

// initCombatSession constructs the initial CombatSession with one player and one monster.
func initCombatSession(npub string, save *types.SaveFile, monster *types.MonsterData, environmentID string) *types.CombatSession {
	return &types.CombatSession{
		Party:         []types.PartyCombatant{newPlayerCombatant(npub, save)},
		Monsters:      []types.MonsterInstance{newMonsterInstance(monster)},
		Round:         1,
		Range:         startingRange(environmentID),
		EnvironmentID: environmentID,
		Phase:         "active",
	}
}

// newPlayerCombatant snapshots the player's current HP into combat state.
func newPlayerCombatant(npub string, save *types.SaveFile) types.PartyCombatant {
	return types.PartyCombatant{
		Type:               "player",
		ID:                 npub,
		IsPlayerControlled: true,
		CombatState: types.PlayerCombatState{
			CurrentHP: save.HP,
			MaxHP:     save.MaxHP,
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

// rollInitiatives returns d20+DEX for the player and monster.
func rollInitiatives(playerDEXMod, monsterDEXMod int) (int, int) {
	return RollD20() + playerDEXMod, RollD20() + monsterDEXMod
}

// buildInitiativeOrder returns combatants sorted by initiative, with DEX as the tiebreaker.
func buildInitiativeOrder(playerID string, playerDEX, playerInit, monsterDEX, monsterInit int, monster types.MonsterInstance) []types.InitiativeEntry {
	entries := []types.InitiativeEntry{
		{ID: playerID, Type: "player", Initiative: playerInit, DEXScore: playerDEX},
		{ID: monster.InstanceID, Type: "monster", Initiative: monsterInit, DEXScore: monsterDEX},
	}
	sort.Slice(entries, func(i, j int) bool {
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

// monsterHasFirstTurn returns true when the monster leads the initiative order.
func monsterHasFirstTurn(cs *types.CombatSession) bool {
	return len(cs.Initiative) > 0 && cs.Initiative[0].Type == "monster"
}

// execMonsterOpeningTurn runs the monster's turn when it wins initiative at combat start.
func execMonsterOpeningTurn(db *sql.DB, cs *types.CombatSession, save *types.SaveFile) []string {
	playerAC := computePlayerAC(db, save)
	dmg, log := ExecuteMonsterTurn(cs, &cs.Monsters[0], playerAC)
	if dmg > 0 {
		log = append(log, applyDamageToPlayer(cs, dmg)...)
	}
	return log
}

// ─── ProcessPlayerAttack ─────────────────────────────────────────────────────

// ProcessPlayerAttack resolves one full attack round: player movement, attack roll,
// damage, XP, and the monster's response turn.
//
// hand: "main" (default) or "off" for two-weapon bonus attack.
// thrown: true to treat a melee weapon with the "thrown" tag as a ranged attack.
//
// Returns log entries for this round. Caller appends them to cs.Log.
func ProcessPlayerAttack(db *sql.DB, cs *types.CombatSession, save *types.SaveFile, weaponSlot string, moveDir int, hand string, thrown bool, advancement []types.AdvancementEntry) ([]string, error) {
	var log []string

	if len(cs.Party) == 0 {
		return nil, fmt.Errorf("no player in combat")
	}
	state := &cs.Party[0].CombatState
	isOffHand := hand == "off"

	if isOffHand {
		// Validate two-weapon fighting conditions before doing anything
		if err := validateTwoWeaponFighting(db, save, cs); err != nil {
			return nil, err
		}
		// Bonus attack always uses the off-hand slot
		weaponSlot = "offhand"
	} else {
		// New main action: reset per-turn flags
		state.ActionUsed = false
		state.BonusActionUsed = false
	}

	log = append(log, applyPlayerMove(cs, moveDir)...)

	item, isUnarmed, err := loadWeaponItem(db, save.Inventory, weaponSlot)
	if err != nil {
		return nil, err
	}

	// Validate that the attack can reach the target at this range
	if err := validateAttackRange(cs, item, isUnarmed, thrown); err != nil {
		return nil, err
	}

	// Ammo consumption — ranged weapons with the "ammunition" tag require ammo
	if !isUnarmed && item != nil && !thrown {
		if hasTag(item["tags"], "ammunition") {
			if err := consumeAmmo(save, cs); err != nil {
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

	monster := &cs.Monsters[0]
	level := character.GetLevelFromXP(save.Experience, advancement)

	attackBonus := resolveAttackBonus(item, save.Stats, save.Class, level, isUnarmed, thrown)
	advantage := resolveAttackAdvantage(cs, item, isUnarmed, save.Race, thrown)
	result := ResolveAttackRoll(attackBonus, monster.ArmorClass, advantage)

	log = append(log, formatAttackRoll(save.D, item, isUnarmed, result))

	if !result.IsHit {
		if isOffHand {
			state.BonusActionUsed = true
		} else {
			state.ActionUsed = true
		}
		return append(log, runMonsterResponseTurn(db, cs, save)...), nil
	}

	offhandEmpty := isOffhandEmpty(save.Inventory)
	var dmg int
	if isOffHand {
		// Two-weapon fighting bonus attack: no ability score modifier to damage
		dmg = resolvePlayerDamageNoMod(item, monster, offhandEmpty, result.IsCrit)
	} else {
		dmg = resolvePlayerDamage(item, save.Stats, monster, isUnarmed, offhandEmpty, result.IsCrit, thrown)
	}
	log = append(log, formatDamage(item, isUnarmed, dmg, result.IsCrit))

	applyDamageToMonster(monster, dmg)

	xp := awardDamageXP(cs, monster, dmg, save.TimeOfDay, level, advancement)
	if xp > 0 {
		log = append(log, fmt.Sprintf("  +%d XP", xp))
	}

	if isOffHand {
		state.BonusActionUsed = true
	} else {
		state.ActionUsed = true
	}

	if !monster.IsAlive {
		return append(log, handleMonsterKill(cs, monster, save.Experience, advancement)...), nil
	}

	return append(log, runMonsterResponseTurn(db, cs, save)...), nil
}

// applyPlayerMove adjusts range and returns a log line when the player moves.
func applyPlayerMove(cs *types.CombatSession, dir int) []string {
	if dir == 0 {
		return nil
	}
	cs.Range += dir
	if cs.Range < 0 {
		cs.Range = 0
	}
	direction := "closer"
	if dir > 0 {
		direction = "back"
	}
	return []string{fmt.Sprintf("  You move %s (range: %d).", direction, cs.Range)}
}

// loadWeaponItem fetches item properties for the chosen weapon slot.
// Returns nil + isUnarmed=true when the slot is empty or weaponSlot is "unarmed".
func loadWeaponItem(db *sql.DB, inventory map[string]interface{}, weaponSlot string) (map[string]interface{}, bool, error) {
	if weaponSlot == "unarmed" {
		return nil, true, nil
	}
	itemID := gaminventory.GetEquippedItemID(inventory, weaponSlot)
	if itemID == "" {
		return nil, true, nil
	}
	item, err := gamedata.LoadItemByID(db, itemID)
	if err != nil {
		return nil, false, fmt.Errorf("loadWeaponItem %s: %w", itemID, err)
	}
	return item, false, nil
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
func resolveAttackAdvantage(cs *types.CombatSession, item map[string]interface{}, isUnarmed bool, race string, thrown bool) int {
	if isUnarmed || item == nil {
		return 0
	}

	advantage := 0
	weaponType, _ := item["type"].(string)
	actingAsRanged := IsRangedAction(weaponType) || thrown

	if actingAsRanged {
		// Disadvantage when firing at melee range
		if cs.Range == 0 {
			advantage--
		}
		// Disadvantage when beyond normal range (but still within long range)
		normalRange, _ := getRangedReach(item)
		if cs.Range > normalRange {
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

// validateAttackRange returns an error if the current combat range prevents this attack.
func validateAttackRange(cs *types.CombatSession, item map[string]interface{}, isUnarmed, thrown bool) error {
	if isUnarmed || item == nil {
		if cs.Range > 0 {
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
		if cs.Range > maxRange {
			return fmt.Errorf("target is beyond maximum range (%d)", maxRange)
		}
		return nil
	}

	// Melee range gate
	reach := getMeleeReach(item)
	if cs.Range > reach {
		return fmt.Errorf("enemy is out of melee range (weapon reach: %d, current range: %d) — move closer", reach, cs.Range)
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
func consumeAmmo(save *types.SaveFile, cs *types.CombatSession) error {
	gearSlots, ok := save.Inventory["gear_slots"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no ammunition equipped — equip arrows or bolts in the ammo slot")
	}

	var ammoMap map[string]interface{}
	var ammoKey string
	for _, key := range []string{"ammunition", "ammo"} {
		if slotData, exists := gearSlots[key]; exists && slotData != nil {
			if slotMap, ok := slotData.(map[string]interface{}); ok {
				if itemID, _ := slotMap["item"].(string); itemID != "" {
					ammoMap = slotMap
					ammoKey = key
					break
				}
			}
		}
	}

	if ammoMap == nil {
		return fmt.Errorf("no ammunition equipped — equip arrows or bolts in the ammo slot")
	}

	qty := slotQty(ammoMap, "quantity")
	if qty <= 0 {
		qty = 1
	}
	if qty <= 1 {
		gearSlots[ammoKey] = map[string]interface{}{"item": nil, "quantity": 0}
	} else {
		ammoMap["quantity"] = qty - 1
		gearSlots[ammoKey] = ammoMap
	}

	cs.AmmoUsedThisCombat++
	return nil
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

// formatAttackRoll returns a narrative log line describing the attack roll.
func formatAttackRoll(playerName string, item map[string]interface{}, isUnarmed bool, result AttackResult) string {
	weapon := "Unarmed Strike"
	if !isUnarmed && item != nil {
		if n, ok := item["name"].(string); ok {
			weapon = n
		}
	}
	return fmt.Sprintf("  %s attacks with %s: rolled %d%s%s",
		playerName, weapon, result.Roll, formatModifier(result.Modifier), attackQualifier(result))
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

// awardDamageXP computes XP for the hit and adds it to the session total.
func awardDamageXP(cs *types.CombatSession, monster *types.MonsterInstance, dmg, timeOfDay, level int, advancement []types.AdvancementEntry) int {
	xpMult := character.GetXPMultiplierForLevel(level, advancement)
	xp := XPForDamage(monster, dmg, NightMultiplier(timeOfDay), xpMult)
	cs.XPEarnedThisFight += xp
	return xp
}

// handleMonsterKill processes monster death: rolls loot and checks for a level-up.
func handleMonsterKill(cs *types.CombatSession, monster *types.MonsterInstance, playerXP int, advancement []types.AdvancementEntry) []string {
	log := []string{fmt.Sprintf("  %s is defeated!", monster.Name)}

	cs.LootRolled = RollLoot(monster.Data.LootTable)
	cs.Phase = "loot"

	if character.WillLevelUp(playerXP, cs.XPEarnedThisFight, advancement) {
		cs.LevelUpPending = true
		log = append(log, "  Level up!")
	}

	log = append(log, fmt.Sprintf("  Victory! +%d XP this fight.", cs.XPEarnedThisFight))
	return log
}

// runMonsterResponseTurn runs the monster's turn after a player action.
func runMonsterResponseTurn(db *sql.DB, cs *types.CombatSession, save *types.SaveFile) []string {
	if cs.Phase != "active" || len(cs.Monsters) == 0 || !cs.Monsters[0].IsAlive {
		return nil
	}
	playerAC := computePlayerAC(db, save)
	dmg, log := ExecuteMonsterTurn(cs, &cs.Monsters[0], playerAC)
	if dmg > 0 {
		log = append(log, applyDamageToPlayer(cs, dmg)...)
	}
	return log
}

// computePlayerAC queries the player's current AC from equipped items.
func computePlayerAC(db *sql.DB, save *types.SaveFile) int {
	return CalculatePlayerAC(db, save.Inventory, save.Stats)
}

// applyDamageToPlayer deducts HP and transitions to death_saves if HP reaches zero.
// Returns any resulting log lines (e.g., the player going unconscious).
func applyDamageToPlayer(cs *types.CombatSession, dmg int) []string {
	if len(cs.Party) == 0 {
		return nil
	}
	state := &cs.Party[0].CombatState
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

// ─── ProcessDeathSave ────────────────────────────────────────────────────────

// ProcessDeathSave rolls one death saving throw and runs the monster's follow-up turn.
// Returns log entries for this round. Caller appends them to cs.Log.
func ProcessDeathSave(cs *types.CombatSession, save *types.SaveFile) []string {
	if len(cs.Party) == 0 || cs.Phase != "death_saves" {
		return nil
	}

	roll := RollD20()
	log := []string{fmt.Sprintf("  Death saving throw: rolled %d.", roll)}
	log = append(log, resolveDeathSaveRoll(&cs.Party[0].CombatState, cs, roll))

	if cs.Phase == "death_saves" {
		log = append(log, runMonsterDeathSaveTurn(cs, save)...)
	}

	return log
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

// runMonsterDeathSaveTurn runs the monster's attack against an unconscious player.
// Hits apply death save failures rather than HP damage.
func runMonsterDeathSaveTurn(cs *types.CombatSession, save *types.SaveFile) []string {
	if len(cs.Monsters) == 0 || !cs.Monsters[0].IsAlive {
		return nil
	}
	monster := &cs.Monsters[0]
	decision := DecideMonsterAction(cs, monster)

	if decision.Action == "flee" {
		cs.Phase = "victory"
		return []string{fmt.Sprintf("  %s flees! You are safe.", monster.Name)}
	}
	if decision.Action != "attack" {
		return nil
	}

	action := monster.Data.Actions[decision.ActionIndex]
	isMeleeAtContact := action.Type == "melee_attack" && cs.Range == 0
	playerAC := 10 + StatMod(GetStatFromMap(save.Stats, "dexterity"))
	result := resolveDeathSaveAttack(action, playerAC, isMeleeAtContact)

	log := []string{fmt.Sprintf("  %s attacks: rolled %d%s%s",
		monster.Name, result.Roll, formatModifier(action.AttackBonus), attackQualifier(result))}

	if result.IsHit {
		log = append(log, applyDeathSaveHit(cs, result.IsCrit || isMeleeAtContact))
	}
	return log
}

// resolveDeathSaveAttack rolls the monster's attack against an unconscious player.
// All attacks have advantage; adjacent melee attacks auto-crit.
func resolveDeathSaveAttack(action types.MonsterAction, playerAC int, isMeleeAtContact bool) AttackResult {
	if isMeleeAtContact {
		return AttackResult{Roll: 20, Total: 20, IsCrit: true, IsHit: true}
	}
	return ResolveAttackRoll(action.AttackBonus, playerAC, 1) // advantage=1
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

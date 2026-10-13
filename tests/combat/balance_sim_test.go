package combat_test

// Combat balance simulation harness.
//
// This is a measuring tool, not a regression test: it plays thousands of solo
// fights through the REAL combat engine (combat.StartEncounter, the player
// Process* calls, and the monster turns the engine runs inside ProcessEndTurn)
// and prints win-rate tables. It is skipped unless explicitly requested:
//
//	PQ_BALANCE_SIM=1 go test ./tests/combat/ -run TestBalanceSim -v -count=1 -timeout 60m
//
// Optional knobs (environment variables):
//
//	PQ_SIM_N          fights per (scenario, level, build, opponent) pairing (default 200)
//	PQ_SIM_SCENARIOS  comma list of scenario names to run (default: all)
//	PQ_SIM_LEVELS     comma list of levels (default: 1,2,3)
//	PQ_SIM_MAXCR      highest monster CR simulated (default 2)
//	PQ_SIM_ONLY       comma list of opponent-label substrings to keep
//	PQ_SIM_WHATIF     1 adds what-if quest variants (1–2 rats, a weaker Rat King)
//	PQ_SIM_OUT        also write the report to this file
//
// What it models
//
//   - Characters are built the way the game builds them: ability scores from
//     character.GenerateStats with a random 32-byte key per fight (so every fight
//     samples the real stat distribution), capitalised stat keys exactly as
//     saves store them, MaxHP from character.DeriveMaxHP, and gear placed in the
//     slots the creation auto-equipper (cmd/server/api/character/helpers.go
//     createInventoryStructure) actually puts it in for the default choices.
//     Levels 2–3 use the XP thresholds from advancement.json, the derived HP
//     progression, and spend the level-2 ability point on the class's main stat.
//   - Opponents: every monster the travel roller can pick (any monster whose
//     environment list contains a travel biome, i.e. an environment_type used by
//     a location) up to PQ_SIM_MAXCR, fought on that biome's battlefield, plus the
//     Rats of Goldenhaven cellar fight (3× giant-rat, authored arena) and the
//     quest's Rat King boss.
//   - Player policy (weapon attacks only — no spells, abilities, items, dodge or
//     flee): walk toward the nearest monster and hit it with the main-hand weapon
//     (or shoot it, for a ranged main hand), take a two-weapon off-hand swing when
//     the engine allows one, end the turn; roll death saves when down.
//
// Scenarios
//
// A scenario toggles test-side stand-ins for engine fixes, so the effect of each
// proposed fix can be measured without touching game code:
//
//	shipped     the engine and data as they are today
//	stats-fix   the save also carries lowercase stat keys, which is what a
//	            case-insensitive combat.GetStatFromMap would see
//	all-fixes   stats-fix + monster HP re-rolled from hp_dice parsed properly
//	            ("2d8+2" today falls back to 1 HP — see rollHPDice)
//
// Each scenario runs the as-generated builds plus two "re-equipped" builds
// (Wizard with the quarterstaff in hand instead of the spellbook, Barbarian with
// the greataxe instead of a javelin) to isolate the starting-gear auto-equip.
//
// Randomness: the engine rolls with the global math/rand source, which Go 1.20+
// seeds randomly per process, and terrain with a time-seeded source — so every
// run is a fresh sample. Hundreds of fights per pairing keep the noise to a few
// percentage points.

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	gamedata "pubkey-quest/cmd/server/api/data"
	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/encounter"
	gaminventory "pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// ─── Configuration ───────────────────────────────────────────────────────────

const (
	simPlayerName = "Hero"
	simMaxRounds  = 60 // a fight still running after this many rounds is a timeout
)

type simScenario struct {
	Name      string
	FixStats  bool // lowercase stat keys alongside the real capitalised ones
	FixHPDice bool // monster HP rolled from "NdS±M" correctly
}

var simScenarios = []simScenario{
	{Name: "shipped"},
	{Name: "stats-fix", FixStats: true},
	{Name: "all-fixes", FixStats: true, FixHPDice: true},
}

// simBuild is one character build: a class plus what is in its hands.
type simBuild struct {
	Label     string
	Class     string
	MainStat  string             // where the level-2 ability point goes
	Gear      map[string]simItem // gear slot → item
	RangedPos bool               // main hand is a ranged weapon: shoot, don't close
}

type simItem struct {
	ID  string
	Qty int
}

// simBuilds mirrors what createInventoryStructure equips for each class's
// default (first-listed) starting-gear choices. Notes on the quirks are inline.
var simBuilds = []simBuild{
	{Label: "Fighter", Class: "Fighter", MainStat: "Strength", Gear: map[string]simItem{
		// chainmail-set + (longsword + shield) + light crossbow/bolts
		"chest": {"chainmail-hauberk", 1}, "legs": {"chainmail-chausses", 1},
		"mainhand": {"longsword", 1}, "offhand": {"shield", 1}, "ammo": {"crossbow-bolts", 20},
	}},
	{Label: "Rogue", Class: "Rogue", MainStat: "Dexterity", Gear: map[string]simItem{
		// The two given daggers (stack 1) are equipped before the rapier is
		// reached, so a new rogue dual-wields daggers; rapier + shortbow go to the bag.
		"chest": {"leather-vest", 1}, "legs": {"leather-leggings", 1},
		"mainhand": {"dagger", 1}, "offhand": {"dagger", 1}, "ammo": {"arrows", 20},
	}},
	{Label: "Wizard", Class: "Wizard", MainStat: "Intelligence", Gear: map[string]simItem{
		// The given spellbook has gear_slot "hands" and is equipped first, so it
		// takes the main hand and the quarterstaff lands in the off hand. A
		// non-weapon in the main hand means every attack is an unarmed strike.
		"mainhand": {"spellbook", 1}, "offhand": {"quarterstaff", 1},
	}},
	{Label: "Wizard(staff)", Class: "Wizard", MainStat: "Intelligence", Gear: map[string]simItem{
		// Re-equipped: quarterstaff two-handed (versatile 1d8).
		"mainhand": {"quarterstaff", 1},
	}},
	{Label: "Cleric", Class: "Cleric", MainStat: "Wisdom", Gear: map[string]simItem{
		// shield (given) + mace + scale mail + light crossbow (bag) + amulet
		"chest": {"scalemail-cuirass", 1}, "legs": {"scalemail-greaves", 1},
		"mainhand": {"mace", 1}, "offhand": {"shield", 1}, "neck": {"amulet", 1},
		"ammo": {"crossbow-bolts", 20},
	}},
	{Label: "Ranger", Class: "Ranger", MainStat: "Dexterity", RangedPos: true, Gear: map[string]simItem{
		// The given longbow (two-handed) takes the main hand first; both
		// shortswords end up in the bag.
		"chest": {"scalemail-cuirass", 1}, "legs": {"scalemail-greaves", 1},
		"mainhand": {"longbow", 1}, "ammo": {"arrows", 20},
	}},
	{Label: "Barbarian", Class: "Barbarian", MainStat: "Strength", Gear: map[string]simItem{
		// The given javelins (gear_slot "hands") are equipped before the greataxe
		// (gear_slot "mainhand"), which then finds the main hand full; one handaxe
		// takes the off hand. No armour, and no Unarmored Defense in the AC calc.
		"mainhand": {"javelin", 4}, "offhand": {"handaxe", 1},
	}},
	{Label: "Barbarian(axe)", Class: "Barbarian", MainStat: "Strength", Gear: map[string]simItem{
		// Re-equipped: greataxe.
		"mainhand": {"greataxe", 1},
	}},
}

// levelXP is the experience that puts a character at each level (advancement.json).
var levelXP = map[int]int{1: 0, 2: 250, 3: 650}

// ─── Opponents ───────────────────────────────────────────────────────────────

type simOpponent struct {
	Label      string
	MonsterIDs []string
	CR         float64  // for a group, the single monster's CR
	Biomes     []string // travel biomes it can be rolled in (empty = quest only)
	Location   string   // save.Location for the battlefield profile
	Building   string
	Room       string
	Arena      []string
	Travel     bool // part of the random travel pool
	// Tweak, when set, edits each monster after it spawns (what-if tuning
	// proposals measured without touching game data).
	Tweak func(m *types.MonsterInstance)
}

// loadOpponents lists every monster the travel roller could pick (up to maxCR),
// plus the authored Rats of Goldenhaven fights.
func loadOpponents(t *testing.T, maxCR float64) []simOpponent {
	t.Helper()
	database := db.GetDB()

	// Travel biomes are the environment_type values locations actually use;
	// remember one location per biome so the fight gets that biome's battlefield.
	biomeLoc := map[string]string{}
	rows, err := database.Query(`SELECT id, properties FROM locations`)
	if err != nil {
		t.Fatalf("query locations: %v", err)
	}
	for rows.Next() {
		var id, props string
		if rows.Scan(&id, &props) != nil {
			continue
		}
		var loc struct {
			EnvironmentType string `json:"environment_type"`
		}
		if json.Unmarshal([]byte(props), &loc) == nil && loc.EnvironmentType != "" {
			if _, seen := biomeLoc[loc.EnvironmentType]; !seen {
				biomeLoc[loc.EnvironmentType] = id
			}
		}
	}
	rows.Close()

	var biomes []string
	for b := range biomeLoc {
		biomes = append(biomes, b)
	}
	sort.Strings(biomes)

	byID := map[string]*simOpponent{}
	for _, b := range biomes {
		refs, err := db.GetMonstersByBiome(b)
		if err != nil {
			t.Fatalf("monsters for %s: %v", b, err)
		}
		for _, r := range refs {
			if r.CR > maxCR {
				continue
			}
			o := byID[r.ID]
			if o == nil {
				o = &simOpponent{Label: r.ID, MonsterIDs: []string{r.ID}, CR: r.CR, Location: biomeLoc[b], Travel: true}
				byID[r.ID] = o
			}
			o.Biomes = append(o.Biomes, b)
		}
	}
	var out []simOpponent
	for _, o := range byID {
		out = append(out, *o)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CR != out[j].CR {
			return out[i].CR < out[j].CR
		}
		return out[i].Label < out[j].Label
	})

	// The Rats of Goldenhaven: the cellar wave and the Rat King, from the quest file.
	var quest struct {
		Stages []struct {
			Encounter *struct {
				Location  string   `json:"location"`
				Building  string   `json:"building"`
				Room      string   `json:"room"`
				MonsterID string   `json:"monster_id"`
				Count     int      `json:"count"`
				Arena     []string `json:"arena"`
			} `json:"encounter"`
		} `json:"stages"`
	}
	raw, err := os.ReadFile("game-data/quests-drafts/side/rats-of-goldenhaven.json")
	if err != nil {
		t.Fatalf("read rats quest: %v", err)
	}
	if err := json.Unmarshal(raw, &quest); err != nil {
		t.Fatalf("parse rats quest: %v", err)
	}
	var cellar simOpponent
	for _, st := range quest.Stages {
		if e := st.Encounter; e != nil && e.MonsterID == "giant-rat" {
			cellar = simOpponent{
				Label: fmt.Sprintf("quest:%dx%s(cellar)", e.Count, e.MonsterID), CR: 0.125,
				MonsterIDs: combat.MonsterIDsForCount(e.MonsterID, e.Count),
				Location:   e.Location, Building: e.Building, Room: e.Room, Arena: e.Arena,
			}
			out = append(out, cellar)
			break
		}
	}
	boss := simOpponent{
		Label: "quest:rat-king-boss", MonsterIDs: []string{"rat-king-boss"}, CR: 1,
		Location: "goldenhaven", Building: "sailors_tavern", Room: "cellar",
	}
	out = append(out, boss)

	// What-if variants of the quest fights (PQ_SIM_WHATIF=1): smaller rat waves,
	// and a Rat King cut to a CR-1/2-ish stat line (half HP, one die of damage).
	if os.Getenv("PQ_SIM_WHATIF") == "1" && cellar.Label != "" {
		for _, n := range []int{1, 2} {
			w := cellar
			w.Label = fmt.Sprintf("whatif:%dxgiant-rat(cellar)", n)
			w.MonsterIDs = combat.MonsterIDsForCount("giant-rat", n)
			out = append(out, w)
		}
		k := boss
		k.Label = "whatif:rat-king(18hp,1d6+2)"
		k.Tweak = func(m *types.MonsterInstance) {
			m.MaxHP, m.CurrentHP = 18, 18
			acts := append([]types.MonsterAction(nil), m.Data.Actions...)
			for i := range acts {
				if acts[i].Hit.Dice == "2d6" {
					acts[i].Hit.Dice = "1d6"
				}
			}
			m.Data.Actions = acts
		}
		out = append(out, k)
	}

	// PQ_SIM_ONLY: comma list of substrings; keep only matching opponents.
	if v := os.Getenv("PQ_SIM_ONLY"); v != "" {
		var keep []simOpponent
		for _, o := range out {
			for _, sub := range strings.Split(v, ",") {
				if sub = strings.TrimSpace(sub); sub != "" && strings.Contains(o.Label, sub) {
					keep = append(keep, o)
					break
				}
			}
		}
		out = keep
	}
	return out
}

// ─── Character construction ──────────────────────────────────────────────────

// newSimSave builds a save the way character creation would, at the given level.
func newSimSave(rng *rand.Rand, b simBuild, level int, sc simScenario, opp simOpponent) *types.SaveFile {
	key := make([]byte, 32)
	rng.Read(key)
	gen := character.GenerateStats(hex.EncodeToString(key), b.Class)
	if level >= 2 && gen[b.MainStat] < character.AbilityScoreMax { // level-2 ability point
		gen[b.MainStat]++
	}

	stats := map[string]interface{}{}
	for k, v := range gen {
		stats[k] = float64(v) // capitalised keys, float64 values — as a loaded save has them
		if sc.FixStats {
			stats[strings.ToLower(k)] = float64(v)
		}
	}

	gear := map[string]interface{}{}
	for _, slot := range []string{"neck", "head", "ammo", "mainhand", "chest", "offhand", "ring1", "legs", "ring2", "gloves", "boots", "bag"} {
		gear[slot] = map[string]interface{}{"item": nil, "quantity": 0}
	}
	for slot, it := range b.Gear {
		gear[slot] = map[string]interface{}{"item": it.ID, "quantity": it.Qty}
	}

	hp := character.DeriveMaxHP(b.Class, level, stats)
	return &types.SaveFile{
		D: simPlayerName, Race: "Human", Class: b.Class,
		Experience: levelXP[level], HP: hp, MaxHP: hp,
		Stats:     stats,
		Inventory: map[string]interface{}{"general_slots": []interface{}{}, "gear_slots": gear},
		Location:  opp.Location, Building: opp.Building, Room: opp.Room,
		TimeOfDay: 720, CurrentDay: 1,
	}
}

// rollHPDice rolls an "NdS", "NdS+M" or "NdS-M" expression. It stands in for a
// fixed combat.RollDice: today ParseDice hands strconv.Atoi("8+2"), fails, and
// RollDice returns 1, so rollMonsterHP gives every "+M"/"-M" monster 1 HP.
func rollHPDice(expr string, fallback int) int {
	m := regexp.MustCompile(`^\s*(\d+)d(\d+)\s*([+-]\s*\d+)?\s*$`).FindStringSubmatch(expr)
	if m == nil {
		return fallback
	}
	n, _ := strconv.Atoi(m[1])
	s, _ := strconv.Atoi(m[2])
	mod := 0
	if m[3] != "" {
		mod, _ = strconv.Atoi(strings.ReplaceAll(m[3], " ", ""))
	}
	total := mod
	for i := 0; i < n; i++ {
		total += rand.Intn(s) + 1
	}
	if total < 1 {
		total = 1
	}
	return total
}

// ─── One fight ───────────────────────────────────────────────────────────────

type fightResult struct {
	Outcome      string // "win", "monster_fled", "loss", "stable", "timeout"
	Rounds       int
	HPLeftFrac   float64
	MonsterHP    int // total starting monster HP (after any scenario fix)
	PAttacks     int
	PHits        int
	MAttacks     int
	MHits        int
	PDamage      int
	MDamage      int
	MonsterFirst bool
	DroppedOnce  bool
}

var (
	reMonsterAttack = regexp.MustCompile(`^\s+(?:⚡ )?(.+?) (?:attacks with .+: rolled|takes an opportunity attack with)`)
	rePlayerDamage  = regexp.MustCompile(`^\s+You deal (\d+) `)
	reMonsterDamage = regexp.MustCompile(`^\s+(.+?) deals (\d+) `)
)

// tally folds a batch of engine log lines into the fight's attack/damage counts.
// An attack line is followed by its verdict line (see combat.outcomeLine); a
// "twist away" reflex save turns a monster hit into a miss.
func (r *fightResult) tally(lines []string) {
	pending := "" // "p" or "m": who made the attack awaiting its verdict
	for _, l := range lines {
		switch {
		case strings.Contains(l, simPlayerName+" attacks with") || strings.Contains(l, "Opportunity attack! You strike"):
			pending = "p"
			r.PAttacks++
			continue
		case reMonsterAttack.MatchString(l):
			pending = "m"
			r.MAttacks++
			continue
		}
		hit := strings.HasPrefix(l, "  💥 CRITICAL HIT!") || strings.HasPrefix(l, "  ⚔ HIT!")
		verdict := hit || strings.HasPrefix(l, "  ✘ MISS") || strings.HasPrefix(l, "  ✘ Critical miss!")
		if verdict && pending != "" {
			if hit && pending == "p" {
				r.PHits++
			} else if hit && pending == "m" {
				r.MHits++
			}
			pending = ""
			continue
		}
		if strings.Contains(l, "You twist away just in time") {
			r.MHits--
		}
		if m := rePlayerDamage.FindStringSubmatch(l); m != nil {
			n, _ := strconv.Atoi(m[1])
			r.PDamage += n
		} else if m := reMonsterDamage.FindStringSubmatch(l); m != nil && m[1] != "You" {
			n, _ := strconv.Atoi(m[2])
			r.MDamage += n
		}
	}
}

// simWeapon is what the policy needs to know about the main-hand weapon.
type simWeapon struct {
	ranged      bool
	normalRange int
	longRange   int
	reach       int
	twoWeapon   bool // a light weapon in each hand: try the off-hand bonus attack
}

func loadSimWeapon(t *testing.T, save *types.SaveFile) simWeapon {
	database := db.GetDB()
	w := simWeapon{reach: combat.PlayerMeleeReachForSave(database, save)}
	main, _ := gamedata.LoadItemByID(database, gaminventory.GetEquippedItemID(save.Inventory, "mainhand"))
	off, _ := gamedata.LoadItemByID(database, gaminventory.GetEquippedItemID(save.Inventory, "offhand"))
	if main != nil && strings.Contains(strings.ToLower(fmt.Sprint(main["type"])), "ranged") {
		w.ranged = true
		w.normalRange = intOf(main["range"])
		w.longRange = intOf(main["range_long"])
		if w.normalRange == 0 {
			w.normalRange = 3
		}
		if w.longRange == 0 {
			w.longRange = w.normalRange
		}
	}
	w.twoWeapon = isLight(main) && isLight(off)
	return w
}

func isLight(item map[string]interface{}) bool {
	tags, _ := item["tags"].([]interface{})
	for _, tg := range tags {
		if s, _ := tg.(string); s == "light" {
			return true
		}
	}
	return false
}

func intOf(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func cheb(a, b types.Position) int {
	dx, dy := a.X-b.X, a.Y-b.Y
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}

// nearestActive returns the active monster nearest the player (fewest HP on ties).
func nearestActive(cs *types.CombatSession) *types.MonsterInstance {
	me := combat.PlayerPos(cs)
	var best *types.MonsterInstance
	for i := range cs.Monsters {
		m := &cs.Monsters[i]
		if !m.Active() {
			continue
		}
		if best == nil || cheb(me, m.Pos) < cheb(me, best.Pos) ||
			(cheb(me, m.Pos) == cheb(me, best.Pos) && m.CurrentHP < best.CurrentHP) {
			best = m
		}
	}
	return best
}

// moveToward walks to the reachable cell that best serves the goal: for melee,
// as close to the target as possible; for ranged, inside normal range without
// stepping adjacent. Ties go to the cheaper move. No move if nothing improves.
func moveToward(cs *types.CombatSession, save *types.SaveFile, target *types.MonsterInstance, w simWeapon, log *[]string) {
	score := func(d int) int {
		if !w.ranged {
			return d
		}
		s := 0
		if d > w.normalRange {
			s += d - w.normalRange
		}
		if d <= 1 {
			s += 2
		}
		return s
	}
	here := cheb(combat.PlayerPos(cs), target.Pos)
	bestScore, bestCost := score(here), 0
	var best *combat.ReachCell
	for _, c := range combat.PlayerReachable(cs) {
		c := c
		s := score(cheb(types.Position{X: c.X, Y: c.Y}, target.Pos))
		if s < bestScore || (s == bestScore && best != nil && c.Cost < bestCost) {
			best, bestScore, bestCost = &c, s, c.Cost
		}
	}
	if best == nil {
		return
	}
	if l, err := combat.ProcessPlayerMove(db.GetDB(), cs, save, best.X, best.Y); err == nil {
		*log = append(*log, l...)
	}
}

// playerTurn runs the simple policy for one turn and ends it.
func playerTurn(cs *types.CombatSession, save *types.SaveFile, w simWeapon, adv []types.AdvancementEntry) []string {
	database := db.GetDB()
	var log []string
	target := nearestActive(cs)
	if target == nil {
		return log
	}
	inReach := func(m *types.MonsterInstance) bool {
		d := cheb(combat.PlayerPos(cs), m.Pos)
		if w.ranged {
			return d <= w.longRange
		}
		return d <= w.reach
	}
	if !inReach(target) || (w.ranged && cheb(combat.PlayerPos(cs), target.Pos) > w.normalRange) {
		moveToward(cs, save, target, w, &log)
	}
	if cs.Phase != "active" {
		return log
	}
	// Whatever is in reach now (the target, or another monster that closed in).
	if !inReach(target) {
		target = nil
		for i := range cs.Monsters {
			if m := &cs.Monsters[i]; m.Active() && inReach(m) {
				target = m
				break
			}
		}
	}
	if target != nil {
		l, err := combat.ProcessPlayerAttack(database, cs, save, target.InstanceID, "mainHand", "main", false, adv)
		if err != nil && cheb(combat.PlayerPos(cs), target.Pos) <= 1 {
			// e.g. out of ammunition with a monster in your face: punch it.
			l, err = combat.ProcessPlayerAttack(database, cs, save, target.InstanceID, "unarmed", "main", false, adv)
		}
		if err == nil {
			log = append(log, l...)
		}
		if w.twoWeapon && cs.Phase == "active" && target.Active() && cheb(combat.PlayerPos(cs), target.Pos) <= 1 {
			if l, err := combat.ProcessPlayerAttack(database, cs, save, target.InstanceID, "offhand", "off", false, adv); err == nil {
				log = append(log, l...)
			}
		}
	}
	if cs.Phase == "active" {
		if l, err := combat.ProcessEndTurn(database, cs, save); err == nil {
			log = append(log, l...)
		}
	}
	return log
}

// runFight plays one encounter to the end.
func runFight(t *testing.T, b simBuild, level int, sc simScenario, opp simOpponent, adv []types.AdvancementEntry, rng *rand.Rand) fightResult {
	database := db.GetDB()
	save := newSimSave(rng, b, level, sc, opp)
	spec := combat.EncounterSpec{MonsterIDs: opp.MonsterIDs, EnvironmentID: opp.Location, Arena: opp.Arena}
	cs, err := combat.StartEncounter(database, save, "npub_balance_sim", spec, adv)
	if err != nil { // runs on a worker goroutine: report, don't Fatal
		t.Errorf("StartEncounter %v: %v", opp.MonsterIDs, err)
		return fightResult{Outcome: "timeout"}
	}
	var r fightResult
	r.MonsterFirst = len(cs.Initiative) > 0 && cs.Initiative[0].Type == "monster"
	r.tally(cs.Log)

	for i := range cs.Monsters {
		m := &cs.Monsters[i]
		if sc.FixHPDice {
			hp := rollHPDice(m.Data.HPDice, m.Data.HitPoints)
			m.MaxHP, m.CurrentHP = hp, hp
		}
		if opp.Tweak != nil {
			opp.Tweak(m)
		}
		r.MonsterHP += m.MaxHP
	}

	w := loadSimWeapon(t, save)
play:
	for steps := 0; steps < 10*simMaxRounds && cs.Round <= simMaxRounds; steps++ {
		switch cs.Phase {
		case "active":
			r.tally(playerTurn(cs, save, w, adv))
		case "death_saves":
			r.DroppedOnce = true
			combat.ProcessDeathSave(cs, save) // death-save swings are left out of the hit rates
		default:
			break play // loot / defeat / victory: the fight is over
		}
		if cs.Party[0].CombatState.IsUnconscious {
			r.DroppedOnce = true
		}
	}

	r.Rounds = cs.Round
	switch cs.Phase {
	case "loot":
		r.Outcome = "monster_fled"
		for _, m := range cs.Monsters {
			if !m.IsAlive {
				r.Outcome = "win"
				break
			}
		}
		st := cs.Party[0].CombatState
		r.HPLeftFrac = float64(st.CurrentHP) / float64(st.MaxHP)
	case "defeat":
		r.Outcome = "loss"
	case "victory": // three death-save successes: stable but the monster still stands
		r.Outcome = "stable"
	default:
		r.Outcome = "timeout"
	}
	return r
}

// ─── Aggregation ─────────────────────────────────────────────────────────────

type pairKey struct {
	Scenario string
	Level    int
	Build    string
	Opp      string
}

type pairStats struct {
	N, Wins, Fled, Losses, Stable, Timeouts int
	Rounds, HPLeftOnWins                    float64
	MonsterHP                               float64
	PAtt, PHit, MAtt, MHit, PDmg, MDmg      int
	RoundsAll                               int
	MonsterFirst, Dropped                   int
}

func (p *pairStats) add(r fightResult) {
	p.N++
	switch r.Outcome {
	case "win":
		p.Wins++
		p.HPLeftOnWins += r.HPLeftFrac
	case "monster_fled":
		p.Fled++
	case "loss":
		p.Losses++
	case "stable":
		p.Stable++
	default:
		p.Timeouts++
	}
	p.Rounds += float64(r.Rounds)
	p.RoundsAll += r.Rounds
	p.MonsterHP += float64(r.MonsterHP)
	p.PAtt += r.PAttacks
	p.PHit += r.PHits
	p.MAtt += r.MAttacks
	p.MHit += r.MHits
	p.PDmg += r.PDamage
	p.MDmg += r.MDamage
	if r.MonsterFirst {
		p.MonsterFirst++
	}
	if r.DroppedOnce {
		p.Dropped++
	}
}

func (p *pairStats) merge(o *pairStats) {
	p.N += o.N
	p.Wins += o.Wins
	p.Fled += o.Fled
	p.Losses += o.Losses
	p.Stable += o.Stable
	p.Timeouts += o.Timeouts
	p.Rounds += o.Rounds
	p.RoundsAll += o.RoundsAll
	p.HPLeftOnWins += o.HPLeftOnWins
	p.MonsterHP += o.MonsterHP
	p.PAtt += o.PAtt
	p.PHit += o.PHit
	p.MAtt += o.MAtt
	p.MHit += o.MHit
	p.PDmg += o.PDmg
	p.MDmg += o.MDmg
	p.MonsterFirst += o.MonsterFirst
	p.Dropped += o.Dropped
}

// survive counts a fight the player walked away from on their feet (a kill, or
// the monster ran off the board).
func (p *pairStats) survive() float64 { return pct(p.Wins+p.Fled, p.N) }

func pct(a, b int) float64 {
	if b == 0 {
		return math.NaN()
	}
	return 100 * float64(a) / float64(b)
}

func div(a, b float64) float64 {
	if b == 0 {
		return math.NaN()
	}
	return a / b
}

// ─── The harness ─────────────────────────────────────────────────────────────

func TestBalanceSim(t *testing.T) {
	if os.Getenv("PQ_BALANCE_SIM") != "1" {
		t.Skip("balance simulation: set PQ_BALANCE_SIM=1 to run")
	}
	combatSetup(t)
	adv, err := character.LoadAdvancement(db.GetDB())
	if err != nil {
		t.Fatalf("load advancement: %v", err)
	}

	n := envInt("PQ_SIM_N", 200)
	maxCR := 2.0
	if v, err := strconv.ParseFloat(os.Getenv("PQ_SIM_MAXCR"), 64); err == nil {
		maxCR = v
	}
	levels := []int{1, 2, 3}
	if v := os.Getenv("PQ_SIM_LEVELS"); v != "" {
		levels = nil
		for _, s := range strings.Split(v, ",") {
			if l, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				levels = append(levels, l)
			}
		}
	}
	scenarios := simScenarios
	if v := os.Getenv("PQ_SIM_SCENARIOS"); v != "" {
		want := map[string]bool{}
		for _, s := range strings.Split(v, ",") {
			want[strings.TrimSpace(s)] = true
		}
		scenarios = nil
		for _, sc := range simScenarios {
			if want[sc.Name] {
				scenarios = append(scenarios, sc)
			}
		}
	}
	opps := loadOpponents(t, maxCR)

	// One job per pairing; a worker pool plays them in parallel.
	type job struct {
		key   pairKey
		sc    simScenario
		level int
		build simBuild
		opp   simOpponent
	}
	var jobs []job
	for _, sc := range scenarios {
		for _, lv := range levels {
			for _, b := range simBuilds {
				for _, o := range opps {
					jobs = append(jobs, job{pairKey{sc.Name, lv, b.Label, o.Label}, sc, lv, b, o})
				}
			}
		}
	}
	t.Logf("balance sim: %d pairings × %d fights (%d scenarios, levels %v, %d builds, %d opponents)",
		len(jobs), n, len(scenarios), levels, len(simBuilds), len(opps))

	start := time.Now()
	results := map[pairKey]*pairStats{}
	var mu sync.Mutex
	ch := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for j := range ch {
				ps := &pairStats{}
				for i := 0; i < n; i++ {
					ps.add(runFight(t, j.build, j.level, j.sc, j.opp, adv, rng))
				}
				mu.Lock()
				results[j.key] = ps
				mu.Unlock()
			}
		}(time.Now().UnixNano() + int64(w))
	}
	for _, j := range jobs {
		ch <- j
	}
	close(ch)
	wg.Wait()
	t.Logf("balance sim: done in %s", time.Since(start).Round(time.Second))

	var out strings.Builder
	for _, sc := range scenarios {
		for _, lv := range levels {
			reportWinTable(&out, sc.Name, lv, opps, results)
		}
		reportDetail(&out, sc.Name, levels, opps, results)
	}
	for _, sc := range scenarios {
		reportBands(&out, sc.Name, levels, opps, results)
	}
	fmt.Print(out.String())
	if path := os.Getenv("PQ_SIM_OUT"); path != "" {
		_ = os.WriteFile(path, []byte(out.String()), 0o644)
	}
}

func envInt(name string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return def
}

// reportWinTable prints survival % (kill or monster fled) per opponent × build.
func reportWinTable(out *strings.Builder, scenario string, level int, opps []simOpponent, res map[pairKey]*pairStats) {
	fmt.Fprintf(out, "\n=== [%s] Level %d — survival %% (win or monster fled), CRCap=%.2f ===\n", scenario, level, encounter.CRCap(level))
	fmt.Fprintf(out, "%-28s %5s %6s", "opponent", "CR", "avgHP")
	for _, b := range simBuilds {
		fmt.Fprintf(out, " %14s", b.Label)
	}
	fmt.Fprintf(out, " %7s\n", "ALL")
	for _, o := range opps {
		all := &pairStats{}
		var cells []string
		hp := 0.0
		for _, b := range simBuilds {
			ps := res[pairKey{scenario, level, b.Label, o.Label}]
			if ps == nil {
				cells = append(cells, fmt.Sprintf(" %14s", "-"))
				continue
			}
			all.merge(ps)
			cells = append(cells, fmt.Sprintf(" %14.0f", ps.survive()))
			hp = ps.MonsterHP / float64(ps.N)
		}
		mark := " "
		if o.Travel && o.CR <= encounter.CRCap(level) {
			mark = "*" // in this level's travel pool today
		}
		fmt.Fprintf(out, "%s%-27s %5.3g %6.1f%s %7.0f\n", mark, o.Label, o.CR, hp, strings.Join(cells, ""), all.survive())
	}
	fmt.Fprintln(out, "  (* = can be rolled by the travel roller at this level today)")
}

// reportDetail prints the per-level fight metrics, pooled over builds, per opponent.
func reportDetail(out *strings.Builder, scenario string, levels []int, opps []simOpponent, res map[pairKey]*pairStats) {
	fmt.Fprintf(out, "\n=== [%s] fight metrics (pooled over builds) ===\n", scenario)
	fmt.Fprintf(out, "%-3s %-27s %5s %6s %6s %6s %6s %7s %7s %6s %6s %6s %6s\n",
		"lvl", "opponent", "win%", "fled%", "loss%", "rnds", "hpL%", "pHit%", "mHit%", "pDPR", "mDPR", "mFst%", "down%")
	for _, lv := range levels {
		for _, o := range opps {
			all := &pairStats{}
			for _, b := range simBuilds {
				if ps := res[pairKey{scenario, lv, b.Label, o.Label}]; ps != nil {
					all.merge(ps)
				}
			}
			if all.N == 0 {
				continue
			}
			fmt.Fprintf(out, "%-3d %-27s %5.0f %6.0f %6.0f %6.1f %6.0f %7.0f %7.0f %6.1f %6.1f %6.0f %6.0f\n",
				lv, o.Label, pct(all.Wins, all.N), pct(all.Fled, all.N), pct(all.Losses+all.Stable+all.Timeouts, all.N),
				div(all.Rounds, float64(all.N)), 100*div(all.HPLeftOnWins, float64(all.Wins)),
				pct(all.PHit, all.PAtt), pct(all.MHit, all.MAtt),
				div(float64(all.PDmg), float64(all.RoundsAll)), div(float64(all.MDmg), float64(all.RoundsAll)),
				pct(all.MonsterFirst, all.N), pct(all.Dropped, all.N))
		}
	}
	// Per-build summary over the level's current travel pool.
	fmt.Fprintf(out, "\n--- [%s] per build, pooled over the travel pool each level can roll today ---\n", scenario)
	fmt.Fprintf(out, "%-3s %-15s %5s %6s %7s %7s %6s %6s\n", "lvl", "build", "surv%", "rnds", "pHit%", "mHit%", "pDPR", "mDPR")
	for _, lv := range levels {
		for _, b := range simBuilds {
			all := &pairStats{}
			for _, o := range opps {
				if !o.Travel || o.CR > encounter.CRCap(lv) {
					continue
				}
				if ps := res[pairKey{scenario, lv, b.Label, o.Label}]; ps != nil {
					all.merge(ps)
				}
			}
			if all.N == 0 {
				continue
			}
			fmt.Fprintf(out, "%-3d %-15s %5.0f %6.1f %7.0f %7.0f %6.1f %6.1f\n", lv, b.Label, all.survive(),
				div(all.Rounds, float64(all.N)), pct(all.PHit, all.PAtt), pct(all.MHit, all.MAtt),
				div(float64(all.PDmg), float64(all.RoundsAll)), div(float64(all.MDmg), float64(all.RoundsAll)))
		}
	}
}

// reportBands evaluates CR-cap rules: for each level and cap, the expected
// survival of a random travel fight — each biome rolls uniformly among its
// eligible monsters (as encounter.Roll does), averaged over builds, then over
// biomes. Also reports the worst biome.
func reportBands(out *strings.Builder, scenario string, levels []int, opps []simOpponent, res map[pairKey]*pairStats) {
	fmt.Fprintf(out, "\n=== [%s] expected survival of a random travel fight, by CR cap ===\n", scenario)
	caps := []float64{0.125, 0.25, 0.5, 1, 2}
	biomes := map[string]bool{}
	for _, o := range opps {
		for _, b := range o.Biomes {
			biomes[b] = true
		}
	}
	fmt.Fprintf(out, "%-3s %-10s %8s %8s %8s\n", "lvl", "cap", "mean%", "worst%", "worstBiome")
	for _, lv := range levels {
		rows := append([]float64{encounter.CRCap(lv)}, caps...)
		for i, cap := range rows {
			var means []float64
			worst, worstBiome := 101.0, ""
			for bio := range biomes {
				var surv []float64
				for _, o := range opps {
					if o.CR > cap || !contains(o.Biomes, bio) {
						continue
					}
					all := &pairStats{}
					for _, b := range simBuilds {
						if ps := res[pairKey{scenario, lv, b.Label, o.Label}]; ps != nil {
							all.merge(ps)
						}
					}
					if all.N > 0 {
						surv = append(surv, all.survive())
					}
				}
				if len(surv) == 0 {
					continue
				}
				m := mean(surv)
				means = append(means, m)
				if m < worst {
					worst, worstBiome = m, bio
				}
			}
			label := fmt.Sprintf("%.3g", cap)
			if i == 0 {
				label += " (now)"
			}
			fmt.Fprintf(out, "%-3d %-10s %8.0f %8.0f %8s\n", lv, label, mean(means), worst, worstBiome)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return math.NaN()
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

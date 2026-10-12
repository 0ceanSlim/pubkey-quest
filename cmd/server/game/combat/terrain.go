package combat

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"strings"
	"sync"

	"pubkey-quest/types"
)

// ─── Terrain (party-combat-plan §2) ──────────────────────────────────────────
//
// Every grid cell has a terrain type: open ground, rough ground, water, cover,
// high ground or a hazard. What each type does, and how each kind of
// battlefield scatters it, is data (game-data/systems/combat-terrain.json, read
// from the systems table). cs.Terrain holds the generated board, row-major;
// "" means open ground.

// TerrainType is one cell type's rules.
type TerrainType struct {
	Name           string `json:"name"`
	Glyph          string `json:"glyph"`
	MoveCost       int    `json:"move_cost"`       // movement spent to enter (min 1)
	ClimbCost      int    `json:"climb_cost"`      // extra to enter from a lower cell
	BlocksMovement bool   `json:"blocks_movement"` // can't stand on it
	BlocksSight    bool   `json:"blocks_sight"`    // stops a line of sight through it
	CoverAC        int    `json:"cover_ac"`        // AC a target gains when a line passes it
	RangedBonus    int    `json:"ranged_bonus"`    // to hit for ranged attacks fired down from it
	Swim           bool   `json:"swim"`            // non-swimmers fight badly here
	Damage         string `json:"damage"`          // hazard dice on entering / ending a turn here
	DamageType     string `json:"damage_type"`
}

// TerrainLook renames a type for one battlefield ("Tree" for a forest's high cover).
type TerrainLook struct {
	Name  string `json:"name"`
	Glyph string `json:"glyph"`
}

// TerrainFeature scatters one type: a number of clusters, each a blob of cells.
type TerrainFeature struct {
	Terrain  string `json:"terrain"`
	Clusters [2]int `json:"clusters"` // min, max clusters on a base-size board
	Size     [2]int `json:"size"`     // min, max cells per cluster
}

// Battlefield is how one kind of place looks when a fight breaks out there.
type Battlefield struct {
	Name          string                 `json:"name"`
	StartingRange [2]int                 `json:"starting_range"`
	Look          map[string]TerrainLook `json:"look"`
	Features      []TerrainFeature       `json:"features"`
}

// TerrainConfig mirrors combat-terrain.json.
type TerrainConfig struct {
	KeepClearColumns int                    `json:"keep_clear_columns"`
	BaseCells        int                    `json:"base_cells"`
	Types            map[string]TerrainType `json:"types"`
	ArenaKey         map[string]string      `json:"arena_key"`
	Battlefields     map[string]Battlefield `json:"battlefields"`
}

var (
	terrainMu  sync.Mutex
	terrainCfg *TerrainConfig
)

// fallbackTerrain is used when the systems table has no combat-terrain row
// (database not re-migrated yet): every board is open ground.
var fallbackTerrain = &TerrainConfig{
	KeepClearColumns: 2,
	BaseCells:        88,
	Types:            map[string]TerrainType{"open": {Name: "Open ground", MoveCost: 1}},
	Battlefields:     map[string]Battlefield{"default": {Name: "Open ground", StartingRange: [2]int{2, 2}}},
}

// loadTerrainConfig reads the terrain config once per process.
func loadTerrainConfig(db *sql.DB) *TerrainConfig {
	terrainMu.Lock()
	defer terrainMu.Unlock()
	if terrainCfg != nil {
		return terrainCfg
	}
	if db == nil {
		return fallbackTerrain
	}
	var props string
	if err := db.QueryRow("SELECT properties FROM systems WHERE id = 'combat-terrain'").Scan(&props); err != nil {
		log.Printf("⚠️ combat terrain config missing (run codex --migrate): %v — boards will be open ground", err)
		terrainCfg = fallbackTerrain
		return terrainCfg
	}
	var cfg TerrainConfig
	if err := json.Unmarshal([]byte(props), &cfg); err != nil {
		log.Printf("❌ combat terrain config unreadable: %v — boards will be open ground", err)
		terrainCfg = fallbackTerrain
		return terrainCfg
	}
	if cfg.BaseCells <= 0 {
		cfg.BaseCells = 88
	}
	terrainCfg = &cfg
	return terrainCfg
}

// activeTerrain is the config used for rules lookups once a fight is running.
func activeTerrain() *TerrainConfig {
	terrainMu.Lock()
	defer terrainMu.Unlock()
	if terrainCfg == nil {
		return fallbackTerrain
	}
	return terrainCfg
}

// SetTerrainConfig replaces the loaded config (tests).
func SetTerrainConfig(cfg *TerrainConfig) {
	terrainMu.Lock()
	terrainCfg = cfg
	terrainMu.Unlock()
}

// ActiveTerrainConfig exposes the rules for the API layer's legend.
func ActiveTerrainConfig() *TerrainConfig {
	return activeTerrain()
}

// ─── Cell lookups ────────────────────────────────────────────────────────────

// terrainAt is the terrain id at pos ("open" off the board or when unset).
func terrainAt(cs *types.CombatSession, pos types.Position) string {
	if len(cs.Terrain) != cs.GridWidth*cs.GridHeight || !inGrid(cs, pos) {
		return "open"
	}
	if t := cs.Terrain[pos.Y*cs.GridWidth+pos.X]; t != "" {
		return t
	}
	return "open"
}

func terrainTypeAt(cs *types.CombatSession, pos types.Position) TerrainType {
	t, ok := activeTerrain().Types[terrainAt(cs, pos)]
	if !ok {
		return TerrainType{Name: "Open ground", MoveCost: 1}
	}
	return t
}

// passable reports whether a creature can stand on pos (ignoring occupants).
func passable(cs *types.CombatSession, pos types.Position) bool {
	return inGrid(cs, pos) && !terrainTypeAt(cs, pos).BlocksMovement
}

// stepCost is the movement spent stepping from one cell to a neighbour, and
// whether the step is allowed at all. A diagonal can't squeeze between two
// blocking cells.
func stepCost(cs *types.CombatSession, from, to types.Position) (int, bool) {
	if !passable(cs, to) {
		return 0, false
	}
	dx, dy := to.X-from.X, to.Y-from.Y
	if dx != 0 && dy != 0 {
		if !passable(cs, types.Position{X: from.X + dx, Y: from.Y}) &&
			!passable(cs, types.Position{X: from.X, Y: from.Y + dy}) {
			return 0, false
		}
	}
	t := terrainTypeAt(cs, to)
	cost := t.MoveCost
	if cost < 1 {
		cost = 1
	}
	if t.ClimbCost > 0 && terrainAt(cs, from) != terrainAt(cs, to) {
		cost += t.ClimbCost
	}
	return cost, true
}

// ─── Movement: Dijkstra over the board ───────────────────────────────────────

// moveRules says who is moving, for occupancy: the player can't pass through
// monsters; monsters can pass through each other but never through a player.
// Nobody may end on an occupied cell.
type moveRules struct {
	moverID  string
	isPlayer bool
	// planCost adds an AI preference on top of the real cost (avoid hazards,
	// deep water for non-swimmers). nil for the player.
	planCost func(pos types.Position) int
}

func (r moveRules) canPass(cs *types.CombatSession, pos types.Position) bool {
	for i := range cs.Party {
		if cs.Party[i].ID != r.moverID && cs.Party[i].Pos == pos {
			return false
		}
	}
	if r.isPlayer {
		for i := range cs.Monsters {
			if cs.Monsters[i].Active() && cs.Monsters[i].Pos == pos {
				return false
			}
		}
	}
	return true
}

// pathMap is a shortest-path tree from one cell: real movement cost, the
// planning cost used to pick routes, and each cell's predecessor.
type pathMap struct {
	cost map[types.Position]int
	plan map[types.Position]int
	prev map[types.Position]types.Position
	from types.Position
}

var neighbourOffsets = [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}

// shortestPaths runs Dijkstra (on planning cost) from `from` across the whole
// board. The board is at most 17×11, so a simple O(n²) scan is plenty.
func shortestPaths(cs *types.CombatSession, from types.Position, rules moveRules) pathMap {
	pm := pathMap{
		cost: map[types.Position]int{from: 0},
		plan: map[types.Position]int{from: 0},
		prev: map[types.Position]types.Position{},
		from: from,
	}
	done := map[types.Position]bool{}
	for {
		var cur types.Position
		best := -1
		for p, c := range pm.plan {
			if !done[p] && (best < 0 || c < best || (c == best && (p.Y < cur.Y || (p.Y == cur.Y && p.X < cur.X)))) {
				cur, best = p, c
			}
		}
		if best < 0 {
			return pm
		}
		done[cur] = true
		for _, d := range neighbourOffsets {
			next := types.Position{X: cur.X + d[0], Y: cur.Y + d[1]}
			if done[next] || !rules.canPass(cs, next) {
				continue
			}
			c, ok := stepCost(cs, cur, next)
			if !ok {
				continue
			}
			plan := best + c
			if rules.planCost != nil {
				plan += rules.planCost(next)
			}
			if old, seen := pm.plan[next]; !seen || plan < old {
				pm.plan[next] = plan
				pm.cost[next] = pm.cost[cur] + c
				pm.prev[next] = cur
			}
		}
	}
}

// pathTo reconstructs the steps from the tree's origin to `to` (excluding the
// origin). nil when unreachable.
func (pm pathMap) pathTo(to types.Position) []types.Position {
	if _, ok := pm.cost[to]; !ok || to == pm.from {
		return nil
	}
	var rev []types.Position
	for p := to; p != pm.from; p = pm.prev[p] {
		rev = append(rev, p)
	}
	out := make([]types.Position, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
	}
	return out
}

// endable reports whether a mover may finish its move on pos.
func endable(cs *types.CombatSession, pos types.Position, moverID string) bool {
	return passable(cs, pos) && !occupied(cs, pos, moverID)
}

// ReachCell is one cell the player can move to this turn, with its cost.
type ReachCell struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Cost int `json:"cost"`
}

// PlayerReachable lists every cell the player can end a move on with the
// movement they have left (for the client's movement preview).
func PlayerReachable(cs *types.CombatSession) []ReachCell {
	if len(cs.Party) == 0 {
		return nil
	}
	me := actor(cs)
	remaining := me.CombatState.MovementBudget - me.CombatState.MovementSpent
	if remaining <= 0 {
		return nil
	}
	pm := shortestPaths(cs, me.Pos, moveRules{moverID: me.ID, isPlayer: true})
	var out []ReachCell
	for p, c := range pm.cost {
		if c == 0 || c > remaining || !endable(cs, p, me.ID) {
			continue
		}
		out = append(out, ReachCell{X: p.X, Y: p.Y, Cost: c})
	}
	return out
}

// ─── Line of sight and cover ─────────────────────────────────────────────────

// lineCells walks a Bresenham line from a to b, excluding both ends.
func lineCells(a, b types.Position) []types.Position {
	var out []types.Position
	dx, dy := absInt(b.X-a.X), -absInt(b.Y-a.Y)
	sx, sy := 1, 1
	if a.X > b.X {
		sx = -1
	}
	if a.Y > b.Y {
		sy = -1
	}
	err := dx + dy
	x, y := a.X, a.Y
	for {
		if x == b.X && y == b.Y {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x += sx
		}
		if e2 <= dx {
			err += dx
			y += sy
		}
		if x == b.X && y == b.Y {
			break
		}
		out = append(out, types.Position{X: x, Y: y})
	}
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// lineCover follows one line: blocked if any cell on it blocks sight, else the
// best cover any cell on it gives.
func lineCover(cs *types.CombatSession, cells []types.Position) (blocked bool, cover int) {
	for _, c := range cells {
		t := terrainTypeAt(cs, c)
		if t.BlocksSight {
			return true, 0
		}
		if t.CoverAC > cover {
			cover = t.CoverAC
		}
	}
	return false, cover
}

// Sight reports whether `from` can see `to`, and the AC bonus cover gives the
// target. Lines are traced both ways (Bresenham isn't symmetric):
//   - both clear: visible, with any low cover the line passes;
//   - one clear: visible around an edge — three-quarters cover;
//   - neither: no line of sight.
//
// Adjacent creatures always see each other, except diagonally past a corner:
// blocked if both side cells block sight, half cover if one does.
func Sight(cs *types.CombatSession, from, to types.Position) (bool, int) {
	if from == to {
		return true, 0
	}
	if chebyshev(from, to) == 1 {
		if from.X == to.X || from.Y == to.Y {
			return true, 0
		}
		a := terrainTypeAt(cs, types.Position{X: to.X, Y: from.Y}).BlocksSight
		b := terrainTypeAt(cs, types.Position{X: from.X, Y: to.Y}).BlocksSight
		switch {
		case a && b:
			return false, 0
		case a || b:
			return true, 2
		}
		return true, 0
	}
	blockedAB, coverAB := lineCover(cs, lineCells(from, to))
	blockedBA, coverBA := lineCover(cs, lineCells(to, from))
	switch {
	case !blockedAB && !blockedBA:
		if coverBA > coverAB {
			coverAB = coverBA
		}
		return true, coverAB
	case !blockedAB || !blockedBA:
		return true, 5
	}
	return false, 0
}

// highGroundBonus is the to-hit bonus for a ranged attack fired from high
// ground at a target below it.
func highGroundBonus(cs *types.CombatSession, from, to types.Position) int {
	t := terrainTypeAt(cs, from)
	if t.RangedBonus > 0 && terrainAt(cs, to) != terrainAt(cs, from) {
		return t.RangedBonus
	}
	return 0
}

// inDeepWater reports whether pos is swimming terrain.
func inDeepWater(cs *types.CombatSession, pos types.Position) bool {
	return terrainTypeAt(cs, pos).Swim
}

// coverLabel describes a cover bonus for the log.
func coverLabel(ac int) string {
	switch {
	case ac >= 5:
		return "three-quarters cover"
	case ac > 0:
		return "half cover"
	}
	return ""
}

// ─── Hazards ─────────────────────────────────────────────────────────────────

// hazardAt returns the hazard's damage dice and type at pos ("" when none).
func hazardAt(cs *types.CombatSession, pos types.Position) (string, string, string) {
	t := terrainTypeAt(cs, pos)
	if t.Damage == "" {
		return "", "", ""
	}
	return t.Damage, t.DamageType, terrainName(cs, terrainAt(cs, pos))
}

// terrainName is a type's display name on this battlefield ("Tree", "Hearth").
func terrainName(cs *types.CombatSession, id string) string {
	cfg := activeTerrain()
	if bf, ok := cfg.Battlefields[cs.Battlefield]; ok {
		if look, ok := bf.Look[id]; ok && look.Name != "" {
			return look.Name
		}
	}
	if t, ok := cfg.Types[id]; ok {
		return t.Name
	}
	return id
}

// ─── Generation ──────────────────────────────────────────────────────────────

// BattlefieldFor picks the terrain profile for where the fight happens: a
// building's cellar-like room, any other room or building, a city street, or
// the surrounding environment's biome.
func BattlefieldFor(db *sql.DB, save *types.SaveFile) string {
	cfg := loadTerrainConfig(db)
	room := strings.ToLower(save.Room)
	if save.Building != "" {
		for _, k := range []string{"cellar", "crypt", "cave", "sewer", "dungeon", "vault"} {
			if strings.Contains(room, k) {
				return "cellar"
			}
		}
		return "interior"
	}
	if db != nil && save.Location != "" {
		var props string
		if err := db.QueryRow(`SELECT properties FROM locations WHERE id = ?`, save.Location).Scan(&props); err == nil {
			var loc struct {
				Type            string `json:"type"`
				EnvironmentType string `json:"environment_type"`
			}
			if json.Unmarshal([]byte(props), &loc) == nil {
				if loc.EnvironmentType != "" {
					if _, ok := cfg.Battlefields[loc.EnvironmentType]; ok {
						return loc.EnvironmentType
					}
				}
				if loc.Type != "" && loc.Type != "environment" {
					return "street" // a city, town or village, outdoors
				}
			}
		}
	}
	return "default"
}

// battlefieldProfile returns the named profile, falling back to "default".
func battlefieldProfile(name string) Battlefield {
	cfg := activeTerrain()
	if bf, ok := cfg.Battlefields[name]; ok {
		return bf
	}
	return cfg.Battlefields["default"]
}

// startingRangeFor rolls the opening distance for a battlefield (before it's
// stretched to the board width).
func startingRangeFor(battlefield string) int {
	r := battlefieldProfile(battlefield).StartingRange
	if r[1] < r[0] {
		r[1] = r[0]
	}
	if r == [2]int{} {
		return 2
	}
	return RollRange(r[0], r[1])
}

func rollBetween(rng *rand.Rand, lo, hi int) int {
	if hi <= lo {
		return lo
	}
	return lo + rng.Intn(hi-lo+1)
}

// generateTerrain scatters the battlefield's features over the board. The
// outer keep-clear columns on each side stay open so nobody spawns stuck, and
// every open cell must be reachable from the player's side — otherwise it
// re-rolls (and gives up to open ground after a few tries).
func generateTerrain(cs *types.CombatSession, rng *rand.Rand) {
	cfg := activeTerrain()
	bf := battlefieldProfile(cs.Battlefield)
	n := cs.GridWidth * cs.GridHeight
	if len(bf.Features) == 0 {
		cs.Terrain = make([]string, n)
		return
	}
	keep := cfg.KeepClearColumns
	scale := float64(n) / float64(cfg.BaseCells)

	for attempt := 0; attempt < 12; attempt++ {
		cs.Terrain = make([]string, n)
		for _, f := range bf.Features {
			if _, ok := cfg.Types[f.Terrain]; !ok {
				continue
			}
			clusters := int(float64(rollBetween(rng, f.Clusters[0], f.Clusters[1]))*scale + 0.5)
			for c := 0; c < clusters; c++ {
				size := rollBetween(rng, f.Size[0], f.Size[1])
				growCluster(cs, rng, f.Terrain, size, keep)
			}
		}
		if boardConnected(cs) {
			return
		}
	}
	cs.Terrain = make([]string, n)
}

// growCluster plants a blob of `size` cells of one terrain type on open cells
// between the keep-clear columns.
func growCluster(cs *types.CombatSession, rng *rand.Rand, terrain string, size, keep int) {
	minX, maxX := keep, cs.GridWidth-1-keep
	if maxX < minX {
		return
	}
	free := func(p types.Position) bool {
		return p.X >= minX && p.X <= maxX && p.Y >= 0 && p.Y < cs.GridHeight &&
			cs.Terrain[p.Y*cs.GridWidth+p.X] == ""
	}
	var seed types.Position
	found := false
	for try := 0; try < 20; try++ {
		seed = types.Position{X: rollBetween(rng, minX, maxX), Y: rng.Intn(cs.GridHeight)}
		if free(seed) {
			found = true
			break
		}
	}
	if !found {
		return
	}
	blob := []types.Position{seed}
	cs.Terrain[seed.Y*cs.GridWidth+seed.X] = terrain
	for len(blob) < size {
		from := blob[rng.Intn(len(blob))]
		d := neighbourOffsets[rng.Intn(4)] // orthogonal growth keeps blobs compact
		next := types.Position{X: from.X + d[0], Y: from.Y + d[1]}
		grown := false
		for try := 0; try < 8 && !grown; try++ {
			if free(next) {
				cs.Terrain[next.Y*cs.GridWidth+next.X] = terrain
				blob = append(blob, next)
				grown = true
				break
			}
			from = blob[rng.Intn(len(blob))]
			d = neighbourOffsets[rng.Intn(4)]
			next = types.Position{X: from.X + d[0], Y: from.Y + d[1]}
		}
		if !grown {
			return
		}
	}
}

// boardConnected reports whether every passable cell can be walked to from the
// player's spawn cell (column 1, middle row).
func boardConnected(cs *types.CombatSession) bool {
	start := types.Position{X: 1, Y: cs.GridHeight / 2}
	if !passable(cs, start) {
		return false
	}
	seen := map[types.Position]bool{start: true}
	queue := []types.Position{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range neighbourOffsets {
			next := types.Position{X: cur.X + d[0], Y: cur.Y + d[1]}
			if seen[next] {
				continue
			}
			if _, ok := stepCost(cs, cur, next); !ok {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	for y := 0; y < cs.GridHeight; y++ {
		for x := 0; x < cs.GridWidth; x++ {
			p := types.Position{X: x, Y: y}
			if passable(cs, p) && !seen[p] {
				return false
			}
		}
	}
	return true
}

// applyArena lays an authored layout over the board, centred. Each row is a
// string of arena_key characters ('.' open, '#' high cover, 'W' deep water, …);
// cells the layout doesn't cover stay open.
func applyArena(cs *types.CombatSession, rows []string) {
	cfg := activeTerrain()
	cs.Terrain = make([]string, cs.GridWidth*cs.GridHeight)
	if len(rows) == 0 {
		return
	}
	width := 0
	for _, r := range rows {
		if l := len([]rune(r)); l > width {
			width = l
		}
	}
	offX := (cs.GridWidth - width) / 2
	offY := (cs.GridHeight - len(rows)) / 2
	for y, row := range rows {
		for x, ch := range []rune(row) {
			p := types.Position{X: x + offX, Y: y + offY}
			if !inGrid(cs, p) {
				continue
			}
			if t := cfg.ArenaKey[string(ch)]; t != "" && t != "open" {
				cs.Terrain[p.Y*cs.GridWidth+p.X] = t
			}
		}
	}
	// The player's spawn must be standable.
	start := types.Position{X: 1, Y: cs.GridHeight / 2}
	cs.Terrain[start.Y*cs.GridWidth+start.X] = ""
}

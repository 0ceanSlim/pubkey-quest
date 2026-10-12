package combat

import (
	"encoding/json"
	"math/rand"
	"os"
	"testing"

	"pubkey-quest/types"
)

// loadTestTerrain installs the real combat-terrain.json for a test and restores
// the previous config afterwards.
func loadTestTerrain(t *testing.T) *TerrainConfig {
	t.Helper()
	data, err := os.ReadFile("../../../../game-data/systems/combat-terrain.json")
	if err != nil {
		t.Fatalf("read terrain config: %v", err)
	}
	var cfg TerrainConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse terrain config: %v", err)
	}
	prev := terrainCfg
	SetTerrainConfig(&cfg)
	t.Cleanup(func() { SetTerrainConfig(prev) })
	return &cfg
}

// arenaBoard builds a 9×7 board with one goblin and the given arena.
func arenaBoard(t *testing.T, rows ...string) *types.CombatSession {
	t.Helper()
	ms := goblins(1)
	cs := boardWith(ms)
	applyArena(cs, rows)
	return cs
}

func TestTerrainConfigBattlefieldsUseKnownTypes(t *testing.T) {
	cfg := loadTestTerrain(t)
	if _, ok := cfg.Battlefields["default"]; !ok {
		t.Fatal("no default battlefield")
	}
	for name, bf := range cfg.Battlefields {
		for _, f := range bf.Features {
			if _, ok := cfg.Types[f.Terrain]; !ok {
				t.Errorf("battlefield %s: unknown terrain %q", name, f.Terrain)
			}
		}
		for id := range bf.Look {
			if _, ok := cfg.Types[id]; !ok {
				t.Errorf("battlefield %s: look for unknown terrain %q", name, id)
			}
		}
	}
	for ch, id := range cfg.ArenaKey {
		if _, ok := cfg.Types[id]; !ok {
			t.Errorf("arena key %q → unknown terrain %q", ch, id)
		}
	}
}

func TestGeneratedBoardsStayConnectedAndClearAtTheSides(t *testing.T) {
	cfg := loadTestTerrain(t)
	rng := rand.New(rand.NewSource(7))
	for name := range cfg.Battlefields {
		for _, size := range [][2]int{{11, 8}, {13, 9}, {17, 11}} {
			for i := 0; i < 25; i++ {
				cs := &types.CombatSession{GridWidth: size[0], GridHeight: size[1], Battlefield: name}
				generateTerrain(cs, rng)
				if !boardConnected(cs) {
					t.Fatalf("%s %dx%d: disconnected board", name, size[0], size[1])
				}
				for y := 0; y < cs.GridHeight; y++ {
					for _, x := range []int{0, 1, cs.GridWidth - 2, cs.GridWidth - 1} {
						if terrainAt(cs, types.Position{X: x, Y: y}) != "open" {
							t.Fatalf("%s: keep-clear column %d has %s", name, x, terrainAt(cs, types.Position{X: x, Y: y}))
						}
					}
				}
			}
		}
	}
}

func TestMoveCostsFollowTerrain(t *testing.T) {
	loadTestTerrain(t)
	cs := arenaBoard(t,
		".........",
		".........",
		".........",
		"..,^.....", // (2,3) rough ground, (3,3) high ground
		".........",
		".........",
		".........",
	)
	from := types.Position{X: 1, Y: 3}
	if c, ok := stepCost(cs, from, types.Position{X: 2, Y: 3}); !ok || c != 2 {
		t.Errorf("rough ground: cost %d ok %v, want 2", c, ok)
	}
	if c, ok := stepCost(cs, types.Position{X: 2, Y: 3}, types.Position{X: 3, Y: 3}); !ok || c != 2 {
		t.Errorf("climbing onto high ground: cost %d ok %v, want 2", c, ok)
	}
	if c, ok := stepCost(cs, types.Position{X: 3, Y: 3}, types.Position{X: 4, Y: 3}); !ok || c != 1 {
		t.Errorf("stepping down: cost %d ok %v, want 1", c, ok)
	}
}

func TestPlayerRoutesAroundAWall(t *testing.T) {
	loadTestTerrain(t)
	cs := arenaBoard(t,
		".........",
		"..#......",
		"..#......",
		"..#......",
		"..#......",
		"..#......",
		".........",
	)
	cs.Monsters[0].Pos = types.Position{X: 8, Y: 0}
	cs.Party[0].CombatState.MovementBudget = 5
	// (3,3) is 2 cells away as the crow flies, but the wall forces a 6-move detour.
	if _, err := ProcessPlayerMove(nil, cs, &types.SaveFile{}, 3, 3); err == nil {
		t.Fatal("expected not enough movement to get round the wall")
	}
	cs.Party[0].CombatState.MovementBudget = 10
	if _, err := ProcessPlayerMove(nil, cs, &types.SaveFile{}, 3, 3); err != nil {
		t.Fatalf("detour should fit in 10: %v", err)
	}
	if spent := cs.Party[0].CombatState.MovementSpent; spent < 4 {
		t.Errorf("spent %d, the detour costs more than the straight line", spent)
	}
	if _, err := ProcessPlayerMove(nil, cs, &types.SaveFile{}, 2, 2); err == nil {
		t.Error("moved onto a wall")
	}
}

func TestSightAndCover(t *testing.T) {
	loadTestTerrain(t)
	cs := arenaBoard(t,
		".........",
		".........",
		".........",
		"...#.=...", // (3,3) high cover, (5,3) low cover
		".........",
		".........",
		".........",
	)
	if visible, _ := Sight(cs, types.Position{X: 1, Y: 3}, types.Position{X: 4, Y: 3}); visible {
		t.Error("saw straight through a wall")
	}
	if visible, cover := Sight(cs, types.Position{X: 4, Y: 3}, types.Position{X: 7, Y: 3}); !visible || cover != 2 {
		t.Errorf("past low cover: visible %v cover %d, want true / 2", visible, cover)
	}
	if visible, cover := Sight(cs, types.Position{X: 1, Y: 0}, types.Position{X: 7, Y: 0}); !visible || cover != 0 {
		t.Errorf("open row: visible %v cover %d, want true / 0", visible, cover)
	}
	// Adjacent across a corner of one pillar: half cover.
	if visible, cover := Sight(cs, types.Position{X: 3, Y: 2}, types.Position{X: 4, Y: 3}); !visible || cover != 2 {
		t.Errorf("round a corner: visible %v cover %d, want true / 2", visible, cover)
	}
}

func TestPlayerCannotShootThroughWalls(t *testing.T) {
	loadTestTerrain(t)
	cs := arenaBoard(t,
		".........",
		".........",
		".........",
		"...#.....",
		".........",
		".........",
		".........",
	)
	cs.Monsters[0].Pos = types.Position{X: 5, Y: 3}
	save := &types.SaveFile{Inventory: map[string]interface{}{}}
	if _, err := ProcessPlayerAttack(nil, cs, save, cs.Monsters[0].InstanceID, "unarmed", "main", false, nil); err == nil {
		t.Fatal("expected an error attacking out of range / sight")
	}
}

func TestMonsterWalksAroundAWall(t *testing.T) {
	loadTestTerrain(t)
	cs := arenaBoard(t,
		".........",
		"...#.....",
		"...#.....",
		"...#.....",
		"...#.....",
		"...#.....",
		".........",
	)
	g := &cs.Monsters[0]
	g.Data.Speed.Walk = 30
	g.Data.Actions = []types.MonsterAction{{Type: "melee_attack", Name: "Scimitar"}}
	g.Pos = types.Position{X: 5, Y: 3}

	for turn := 0; turn < 3 && chebyshev(g.Pos, playerPos(cs)) > 1; turn++ {
		d := DecideMonsterAction(cs, g)
		ApplyMonsterMove(cs, g, d, 0, nil)
		if !passable(cs, g.Pos) {
			t.Fatalf("goblin stood in a wall at %+v", g.Pos)
		}
	}
	if chebyshev(g.Pos, playerPos(cs)) != 1 {
		t.Errorf("goblin never got round the wall, ended at %+v", g.Pos)
	}
}

func TestRangedMonsterSeeksALine(t *testing.T) {
	loadTestTerrain(t)
	cs := arenaBoard(t,
		".........",
		".........",
		".........",
		"...#.....",
		".........",
		".........",
		".........",
	)
	g := &cs.Monsters[0]
	long := 16
	g.Data.Speed.Walk = 30
	g.Data.Behavior.PreferredRange = 4
	g.Data.Actions = []types.MonsterAction{{Type: "ranged_attack", Name: "Shortbow", RangeLong: &long}}
	g.Pos = types.Position{X: 5, Y: 3} // range 4, but the pillar blocks the shot

	if selectActionFor(cs, g) >= 0 {
		t.Fatal("shouldn't be able to shoot through the pillar")
	}
	d := DecideMonsterAction(cs, g)
	ApplyMonsterMove(cs, g, d, 0, nil)
	if visible, _ := Sight(cs, g.Pos, playerPos(cs)); !visible {
		t.Errorf("archer moved to %+v and still has no shot", g.Pos)
	}
}

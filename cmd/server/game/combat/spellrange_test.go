package combat

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"pubkey-quest/types"
)

// Spell range: how far a spell reaches the one it's aimed at, in grid cells.

func rangedSpell(rng, long string) map[string]interface{} {
	return map[string]interface{}{"range": rng, "range_long": long}
}

func TestSpellReach(t *testing.T) {
	cases := []struct {
		name      string
		rng, long string
		want      int
	}{
		{"ranged", "4", "4", 4},
		{"range_long is no long band", "4", "6", 4},
		{"touch", "0", "0", 1},
		{"thrown (produce flame)", "0", "2", 2},
		{"short", "1", "1", 1},
	}
	for _, c := range cases {
		if got := spellReach(rangedSpell(c.rng, c.long)); got != c.want {
			t.Errorf("%s: reach %d, want %d", c.name, got, c.want)
		}
	}
	if got := spellReach(map[string]interface{}{"range": float64(3)}); got != 3 {
		t.Errorf("numeric range: reach %d, want 3", got)
	}
}

func TestCheckSpellReachRefusesBeyondRange(t *testing.T) {
	me := types.Position{X: 1, Y: 3}
	bolt := rangedSpell("4", "6")
	if err := checkSpellReach(bolt, "Fire Bolt", me, types.Position{X: 5, Y: 7}, "Goblin"); err != nil {
		t.Errorf("4 cells (diagonal) should be in reach: %v", err)
	}
	err := checkSpellReach(bolt, "Fire Bolt", me, types.Position{X: 6, Y: 3}, "Goblin")
	if err == nil || !strings.Contains(err.Error(), "25 ft away") || !strings.Contains(err.Error(), "20 ft") {
		t.Errorf("5 cells off: got %v, want a refusal naming 25 ft and 20 ft", err)
	}
}

func TestTouchSpellNeedsAdjacency(t *testing.T) {
	me := types.Position{X: 1, Y: 3}
	grasp := rangedSpell("0", "0")
	if err := checkSpellReach(grasp, "Shocking Grasp", me, types.Position{X: 2, Y: 4}, "Goblin"); err != nil {
		t.Errorf("diagonally adjacent should be touchable: %v", err)
	}
	err := checkSpellReach(grasp, "Shocking Grasp", me, types.Position{X: 3, Y: 3}, "Goblin")
	if err == nil || !strings.Contains(err.Error(), "touch") {
		t.Errorf("2 cells off: got %v, want a touch refusal", err)
	}
	// Thrown: range 0, but range_long carries the throw.
	flame := rangedSpell("0", "2")
	if err := checkSpellReach(flame, "Produce Flame", me, types.Position{X: 3, Y: 3}, "Goblin"); err != nil {
		t.Errorf("a 2-cell throw should reach: %v", err)
	}
	if err := checkSpellReach(flame, "Produce Flame", me, types.Position{X: 4, Y: 3}, "Goblin"); err == nil || strings.Contains(err.Error(), "touch") {
		t.Errorf("3 cells off: got %v, want a plain range refusal", err)
	}
}

func TestAreaReach(t *testing.T) {
	me := types.Position{X: 1, Y: 3}
	far := types.Position{X: 8, Y: 3} // 7 cells off
	withArea := func(rng, shape string, radius int) map[string]interface{} {
		s := rangedSpell(rng, rng)
		s["aoe"] = map[string]interface{}{"shape": shape, "radius": float64(radius)}
		return s
	}
	// A sphere is placed at range: its centre must be in reach.
	if err := checkSpellReach(withArea("5", "sphere", 4), "Fireball", me, far, "Goblin"); err == nil {
		t.Error("fireball centred 7 cells off with range 5 should be refused")
	}
	if err := checkSpellReach(withArea("5", "sphere", 4), "Fireball", me, types.Position{X: 6, Y: 3}, "Goblin"); err != nil {
		t.Errorf("fireball centred 5 cells off should be in reach: %v", err)
	}
	// Areas spreading from the caster reach as far as their own size (aimArea's
	// job), whatever the range field says.
	for _, shape := range []string{"self_radius", "cone", "line"} {
		if err := checkSpellReach(withArea("1", shape, 2), "Spell", me, far, "Goblin"); err != nil {
			t.Errorf("%s: range shouldn't apply, got %v", shape, err)
		}
	}
}

func TestAimsAtAlly(t *testing.T) {
	cases := []struct {
		name  string
		spell map[string]interface{}
		buff  *types.SpellBuff
		want  bool
	}{
		{"heal", map[string]interface{}{"heal": "1d8+3"}, nil, true},
		{"cure", map[string]interface{}{"cures": []interface{}{"poisoned"}}, nil, true},
		{"ally buff", map[string]interface{}{}, &types.SpellBuff{Target: "ally"}, true},
		{"self buff", map[string]interface{}{"heal": nil}, &types.SpellBuff{}, false},
		{"party buff", map[string]interface{}{}, &types.SpellBuff{Target: "party"}, false},
	}
	for _, c := range cases {
		if got := aimsAtAlly(c.spell, c.buff); got != c.want {
			t.Errorf("%s: aimsAtAlly = %v, want %v", c.name, got, c.want)
		}
	}
}

// The shipped spells read the way the range rules intend.
func TestSpellDataReach(t *testing.T) {
	cases := []struct {
		id    string
		reach int
		needs bool
	}{
		{"fire-bolt", 4, true},
		{"shocking-grasp", 1, true},
		{"inflict-wounds", 1, true},
		{"produce-flame", 2, true},
		{"fireball", 5, true},
		{"cure-wounds", 1, true},
		{"healing-word", 2, true},
		{"burning-hands", 0, false},
		{"thunderwave", 0, false},
	}
	for _, c := range cases {
		data, err := os.ReadFile("../../../../game-data/magic/spells/" + c.id + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var spell map[string]interface{}
		if err := json.Unmarshal(data, &spell); err != nil {
			t.Fatalf("%s: %v", c.id, err)
		}
		if got := spellNeedsReach(spell); got != c.needs {
			t.Errorf("%s: needs reach = %v, want %v", c.id, got, c.needs)
			continue
		}
		if c.needs {
			if got := spellReach(spell); got != c.reach {
				t.Errorf("%s: reach %d, want %d", c.id, got, c.reach)
			}
		}
	}
}

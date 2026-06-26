package requirement_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/requirement"
	"pubkey-quest/types"
)

// fakeContext is an in-memory Context so the pure evaluator can be exercised
// without loading skill tables, advancement, or quest data.
type fakeContext struct {
	skills    map[string]int
	stats     map[string]int
	level     int
	qp        int
	items     map[string]bool
	class     string
	race      string
	alignment string
	completed map[string]bool
}

func (f fakeContext) SkillValue(id string) int        { return f.skills[id] }
func (f fakeContext) StatValue(id string) int         { return f.stats[id] }
func (f fakeContext) Level() int                      { return f.level }
func (f fakeContext) QuestPoints() int                { return f.qp }
func (f fakeContext) HasItem(id string) bool          { return f.items[id] }
func (f fakeContext) Class() string                   { return f.class }
func (f fakeContext) Race() string                    { return f.race }
func (f fakeContext) Alignment() string               { return f.alignment }
func (f fakeContext) IsQuestCompleted(id string) bool { return f.completed[id] }

func baseCtx() fakeContext {
	return fakeContext{
		skills:    map[string]int{"athletics": 14, "thieving": 8},
		stats:     map[string]int{"strength": 12, "dexterity": 16},
		level:     5,
		qp:        10,
		items:     map[string]bool{"silver-key": true},
		class:     "Druid",
		race:      "Elf",
		alignment: "neutral_good",
		completed: map[string]bool{"the-rising-shadow": true},
	}
}

func TestEvaluateOne(t *testing.T) {
	ctx := baseCtx()
	cases := []struct {
		name string
		req  types.POIRequirement
		want bool
	}{
		{"skill met", types.POIRequirement{Type: "skill", ID: "athletics", Min: 14}, true},
		{"skill short", types.POIRequirement{Type: "skill", ID: "athletics", Min: 15}, false},
		{"skill missing id reads zero", types.POIRequirement{Type: "skill", ID: "survival", Min: 1}, false},
		{"stat met", types.POIRequirement{Type: "stat", ID: "dexterity", Min: 16}, true},
		{"stat short", types.POIRequirement{Type: "stat", ID: "strength", Min: 13}, false},
		{"level met", types.POIRequirement{Type: "level", Min: 5}, true},
		{"level short", types.POIRequirement{Type: "level", Min: 6}, false},
		{"quest points met", types.POIRequirement{Type: "quest_points", Min: 10}, true},
		{"quest points short", types.POIRequirement{Type: "quest_points", Min: 11}, false},
		{"item present", types.POIRequirement{Type: "item", ID: "silver-key"}, true},
		{"item absent", types.POIRequirement{Type: "item", ID: "iron-key"}, false},
		{"class in list", types.POIRequirement{Type: "class", Values: []string{"Druid", "Ranger"}}, true},
		{"class case-insensitive", types.POIRequirement{Type: "class", Values: []string{"druid"}}, true},
		{"class not in list", types.POIRequirement{Type: "class", Values: []string{"Wizard"}}, false},
		{"race in list", types.POIRequirement{Type: "race", Values: []string{"Elf", "Half-Elf"}}, true},
		{"alignment in list", types.POIRequirement{Type: "alignment", Values: []string{"good", "neutral_good"}}, true},
		{"alignment not in list", types.POIRequirement{Type: "alignment", Values: []string{"lawful_evil"}}, false},
		{"quest completed", types.POIRequirement{Type: "quest_completed", ID: "the-rising-shadow"}, true},
		{"quest not completed", types.POIRequirement{Type: "quest_completed", ID: "the-shadows-source"}, false},
		{"unknown type fails closed", types.POIRequirement{Type: "phase-of-moon"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := requirement.EvaluateOne(c.req, ctx); got != c.want {
				t.Errorf("EvaluateOne(%+v) = %v, want %v", c.req, got, c.want)
			}
		})
	}
}

func TestEvaluate_EmptyPasses(t *testing.T) {
	if res := requirement.Evaluate(nil, baseCtx()); !res.OK || len(res.Missing) != 0 {
		t.Errorf("empty requirement set should pass clean, got %+v", res)
	}
}

func TestEvaluate_AndSemantics(t *testing.T) {
	ctx := baseCtx()
	reqs := []types.POIRequirement{
		{Type: "level", Min: 5},                    // pass
		{Type: "skill", ID: "athletics", Min: 14},  // pass
		{Type: "item", ID: "iron-key"},             // FAIL
		{Type: "quest_points", Min: 99},            // FAIL
	}
	res := requirement.Evaluate(reqs, ctx)
	if res.OK {
		t.Fatal("set with two unmet requirements should not pass")
	}
	if len(res.Missing) != 2 {
		t.Fatalf("expected 2 missing requirements, got %d: %+v", len(res.Missing), res.Missing)
	}
	// All-met set passes.
	if res := requirement.Evaluate(reqs[:2], ctx); !res.OK {
		t.Errorf("all-met set should pass, got %+v", res)
	}
}

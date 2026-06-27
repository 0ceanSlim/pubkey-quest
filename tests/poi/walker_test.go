package poi_test

import (
	"math/rand"
	"testing"

	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/types"
)

// fakeCtx implements requirement.Context for the walker's choice gating + checks.
type fakeCtx struct {
	skills map[string]int
	items  map[string]bool
}

func (f fakeCtx) SkillValue(id string) int        { return f.skills[id] }
func (f fakeCtx) StatValue(id string) int         { return 0 }
func (f fakeCtx) Level() int                      { return 1 }
func (f fakeCtx) QuestPoints() int                { return 0 }
func (f fakeCtx) HasItem(id string) bool          { return f.items[id] }
func (f fakeCtx) Class() string                   { return "Fighter" }
func (f fakeCtx) Race() string                    { return "Human" }
func (f fakeCtx) Alignment() string               { return "neutral" }
func (f fakeCtx) IsQuestCompleted(id string) bool { return false }

func rng() *rand.Rand { return rand.New(rand.NewSource(1)) }

func TestNarrative(t *testing.T) {
	res := poi.Resolve(
		types.POIStep{Type: types.POIStepNarrative, Text: "A dark hall.", Next: "n2"},
		"n1", &types.SaveFile{}, poi.Deps{Ctx: fakeCtx{}, Rng: rng()})
	if res.Text != "A dark hall." || res.Next != "n2" || res.Terminal {
		t.Errorf("narrative: %+v", res)
	}
}

func TestChoiceFiltersByRequirement(t *testing.T) {
	node := types.POIStep{Type: types.POIStepChoice, Choices: []types.POIChoice{
		{Label: "Open it", Next: "open"},
		{Label: "Pick the lock", Next: "pick", Requirements: []types.POIRequirement{{Type: "item", ID: "thieves-kit"}}},
	}}
	got := poi.Resolve(node, "c1", &types.SaveFile{}, poi.Deps{Ctx: fakeCtx{}, Rng: rng()})
	if len(got.Choices) != 1 || got.Choices[0].Label != "Open it" {
		t.Errorf("without the kit only the ungated choice should show, got %+v", got.Choices)
	}
	withKit := poi.Resolve(node, "c1", &types.SaveFile{},
		poi.Deps{Ctx: fakeCtx{items: map[string]bool{"thieves-kit": true}}, Rng: rng()})
	if len(withKit.Choices) != 2 {
		t.Errorf("with the kit both choices should show, got %d", len(withKit.Choices))
	}
}

func TestCheckBranches(t *testing.T) {
	node := types.POIStep{Type: types.POIStepCheck, Skill: "perception", DC: 5,
		SuccessNext: "win", FailureNext: "lose", SuccessText: "You spot it."}
	// skill 20 (mod +5) vs DC 5 → always succeeds → success branch.
	win := poi.Resolve(node, "ck", &types.SaveFile{},
		poi.Deps{Ctx: fakeCtx{skills: map[string]int{"perception": 20}}, Rng: rng()})
	if win.Next != "win" {
		t.Errorf("high skill vs low DC should take success branch, got %s", win.Next)
	}
	// skill 4 (mod -3) vs DC 25 → always fails → failure branch.
	node.DC = 25
	lose := poi.Resolve(node, "ck", &types.SaveFile{},
		poi.Deps{Ctx: fakeCtx{skills: map[string]int{"perception": 4}}, Rng: rng()})
	if lose.Next != "lose" {
		t.Errorf("low skill vs high DC should take failure branch, got %s", lose.Next)
	}
}

func TestRewardGranted(t *testing.T) {
	var granted *types.POIReward
	deps := poi.Deps{Ctx: fakeCtx{}, Rng: rng(),
		GrantReward: func(_ *types.SaveFile, r *types.POIReward) { granted = r }}
	res := poi.Resolve(
		types.POIStep{Type: types.POIStepReward, Reward: &types.POIReward{XP: 50, Gold: 10}, Next: "n2"},
		"rw", &types.SaveFile{}, deps)
	if granted == nil || granted.XP != 50 {
		t.Fatal("reward was not granted")
	}
	if res.Next != "n2" || len(res.Outcome) == 0 {
		t.Errorf("reward result: %+v", res)
	}
}

func TestDamage(t *testing.T) {
	save := &types.SaveFile{HP: 20}
	poi.Resolve(
		types.POIStep{Type: types.POIStepDamage, Damage: &types.POIDamage{Type: "fire", Amount: 8}, Next: "n2"},
		"dmg", save, poi.Deps{Ctx: fakeCtx{}, Rng: rng()})
	if save.HP != 12 {
		t.Errorf("HP = %d, want 12", save.HP)
	}
}

func TestEffectLootMonsterExit(t *testing.T) {
	var effectApplied string
	var itemsAdded []string
	deps := poi.Deps{Ctx: fakeCtx{}, Rng: rng(),
		ApplyEffect: func(_ *types.SaveFile, id string) { effectApplied = id },
		AddItem:     func(_ *types.SaveFile, id string, _ int) { itemsAdded = append(itemsAdded, id) },
	}

	poi.Resolve(types.POIStep{Type: types.POIStepEffect, Effect: &types.POIEffect{ID: "blessed"}, Next: "x"}, "e", &types.SaveFile{}, deps)
	if effectApplied != "blessed" {
		t.Error("effect not applied")
	}

	loot := types.POIStep{Type: types.POIStepLoot, LootTable: &types.POILootTable{
		Guaranteed: []types.POILootEntry{{Item: "gold-piece", Quantity: float64(5)}}}}
	poi.Resolve(loot, "l", &types.SaveFile{}, deps)
	if len(itemsAdded) != 1 || itemsAdded[0] != "gold-piece" {
		t.Errorf("guaranteed loot not granted: %v", itemsAdded)
	}

	mon := poi.Resolve(types.POIStep{Type: types.POIStepMonster, MonsterID: "wolf", Next: "after"}, "m", &types.SaveFile{}, deps)
	if mon.Combat != "wolf" || mon.Next != "after" {
		t.Errorf("monster node: %+v", mon)
	}

	ex := poi.Resolve(types.POIStep{Type: types.POIStepExit, Text: "You leave."}, "ex", &types.SaveFile{}, deps)
	if !ex.Terminal {
		t.Error("exit should be terminal")
	}
}

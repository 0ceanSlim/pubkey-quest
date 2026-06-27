package quest_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/types"
)

// ── fakes ──────────────────────────────────────────────────────────────────

// fakeCtx is a stand-in requirement.Context (the real one is built at the API
// layer from skills/level/etc.; here we set the facts directly).
type fakeCtx struct {
	level, qp                 int
	class, race, alignment    string
	skills, stats             map[string]int
	items, completed          map[string]bool
}

func (f fakeCtx) SkillValue(id string) int        { return f.skills[id] }
func (f fakeCtx) StatValue(id string) int         { return f.stats[id] }
func (f fakeCtx) Level() int                      { return f.level }
func (f fakeCtx) QuestPoints() int                { return f.qp }
func (f fakeCtx) HasItem(id string) bool          { return f.items[id] }
func (f fakeCtx) Class() string                   { return f.class }
func (f fakeCtx) Race() string                    { return f.race }
func (f fakeCtx) Alignment() string               { return f.alignment }
func (f fakeCtx) IsQuestCompleted(id string) bool { return f.completed[id] }

func testQuests() map[string]*types.QuestData {
	return map[string]*types.QuestData{
		"wolf-hunt": {
			ID: "wolf-hunt", Name: "Wolf Hunt", Category: types.QuestSide, TotalQP: 3,
			Stages: []types.QuestStage{{
				ID: "s1",
				Objectives: []types.QuestObjective{
					{Type: types.ObjectiveSlay, Target: "wolf", Count: 2, Description: "Slay 2 wolves"},
				},
				Rewards: &types.POIReward{XP: 50, Gold: 100},
			}},
		},
		"two-step": {
			ID: "two-step", Name: "Two Step", Category: types.QuestSide, TotalQP: 5,
			Stages: []types.QuestStage{
				{ID: "a", Objectives: []types.QuestObjective{{Type: types.ObjectiveTalk, Target: "npc-a", Description: "Talk"}}},
				{ID: "b", Objectives: []types.QuestObjective{{Type: types.ObjectiveFetch, Target: "item-x", Count: 1, Description: "Fetch"}},
					Rewards: &types.POIReward{Gold: 20}},
			},
		},
		"warrior-only": {
			ID: "warrior-only", Name: "Warrior Only", Category: types.QuestClass,
			Requirements: []types.POIRequirement{{Type: "class", Values: []string{"Fighter"}}},
			Stages:       []types.QuestStage{{ID: "s1", Objectives: []types.QuestObjective{{Type: types.ObjectiveTalk, Target: "x"}}}},
		},
		"sequel": {
			ID: "sequel", Name: "Sequel", Category: types.QuestSide,
			Prerequisites: []string{"wolf-hunt"},
			Stages:        []types.QuestStage{{ID: "s1", Objectives: []types.QuestObjective{{Type: types.ObjectiveTalk, Target: "x"}}}},
		},
	}
}

func lookupFrom(qs map[string]*types.QuestData) quest.QuestLookup {
	return func(id string) (*types.QuestData, error) {
		if q, ok := qs[id]; ok {
			return q, nil
		}
		return nil, nil
	}
}

func newSave() *types.SaveFile {
	// A couple of empty general slots so gold rewards (stored as a gold-piece
	// item) have somewhere to land, mirroring a real save's inventory shape.
	return &types.SaveFile{
		Inventory: map[string]interface{}{
			"general_slots": []interface{}{
				map[string]interface{}{"item": "", "quantity": 0},
				map[string]interface{}{"item": "", "quantity": 0},
			},
		},
		Stats: map[string]interface{}{},
	}
}

// goldOf reads the player's gold (the gold-piece stack in general_slots).
func goldOf(save *types.SaveFile) int {
	slots, _ := save.Inventory["general_slots"].([]interface{})
	for _, s := range slots {
		slot, _ := s.(map[string]interface{})
		if slot["item"] == "gold-piece" {
			switch q := slot["quantity"].(type) {
			case int:
				return q
			case float64:
				return int(q)
			}
		}
	}
	return 0
}

// ── tests ──────────────────────────────────────────────────────────────────

func TestAcceptThenSlayCompletesAndRewards(t *testing.T) {
	qs := testQuests()
	save := newSave()
	ctx := fakeCtx{class: "Wizard"}

	if err := quest.Accept(*qs["wolf-hunt"], save, ctx); err != nil {
		t.Fatalf("Accept: %v", err)
	}
	if !quest.IsActive(save, "wolf-hunt") {
		t.Fatal("quest should be active after Accept")
	}

	var r events.Recorder
	r.Subscribe(quest.Consumer(lookupFrom(qs), nil))

	r.Record(save, events.MonsterKilled, "wolf", 1)
	if quest.IsCompleted(save, "wolf-hunt") {
		t.Fatal("one kill should not complete a slay-2 objective")
	}
	if got := save.QuestsActive[0].ObjectiveCounts[0]; got != 1 {
		t.Fatalf("objective count = %d, want 1", got)
	}

	r.Record(save, events.MonsterKilled, "wolf", 1)
	if !quest.IsCompleted(save, "wolf-hunt") {
		t.Fatal("second kill should complete the quest")
	}
	if quest.IsActive(save, "wolf-hunt") {
		t.Error("completed quest should leave the active list")
	}
	if save.Experience != 50 {
		t.Errorf("XP reward not granted: Experience = %d, want 50", save.Experience)
	}
	if g := goldOf(save); g != 100 {
		t.Errorf("gold reward not granted: gold = %d, want 100", g)
	}
}

func TestUnrelatedEventsDoNotProgress(t *testing.T) {
	qs := testQuests()
	save := newSave()
	_ = quest.Accept(*qs["wolf-hunt"], save, fakeCtx{})

	var r events.Recorder
	r.Subscribe(quest.Consumer(lookupFrom(qs), nil))
	r.Record(save, events.MonsterKilled, "goblin", 3) // wrong target
	r.Record(save, events.ItemAcquired, "wolf", 1)    // wrong kind

	if c := save.QuestsActive[0].ObjectiveCounts[0]; c != 0 {
		t.Errorf("unrelated events advanced the objective to %d", c)
	}
}

func TestMultiStageProgression(t *testing.T) {
	qs := testQuests()
	save := newSave()
	_ = quest.Accept(*qs["two-step"], save, fakeCtx{})

	var r events.Recorder
	r.Subscribe(quest.Consumer(lookupFrom(qs), nil))

	r.Record(save, events.NPCTalked, "npc-a", 1)
	if quest.IsCompleted(save, "two-step") {
		t.Fatal("first stage should not complete the quest")
	}
	if save.QuestsActive[0].Stage != 1 {
		t.Fatalf("should have advanced to stage 1, got %d", save.QuestsActive[0].Stage)
	}

	r.Record(save, events.ItemAcquired, "item-x", 1)
	if !quest.IsCompleted(save, "two-step") {
		t.Fatal("second stage should complete the quest")
	}
	if g := goldOf(save); g != 20 {
		t.Errorf("final-stage gold not granted, got %d", g)
	}
}

func TestAvailabilityGating(t *testing.T) {
	qs := testQuests()
	all := []types.QuestData{*qs["wolf-hunt"], *qs["two-step"], *qs["warrior-only"], *qs["sequel"]}

	save := newSave()
	wizard := fakeCtx{class: "Wizard"}

	avail := quest.Available(all, save, wizard)
	if hasQuest(avail, "warrior-only") {
		t.Error("a Wizard should not see a Fighter-only quest")
	}
	if hasQuest(avail, "sequel") {
		t.Error("sequel should be hidden until its prerequisite is done")
	}
	if !hasQuest(avail, "wolf-hunt") {
		t.Error("wolf-hunt should be available")
	}

	// Complete the prerequisite and the sequel opens up; a Fighter sees the
	// class quest.
	save.QuestsCompleted = []string{"wolf-hunt"}
	fighter := fakeCtx{class: "Fighter"}
	avail = quest.Available(all, save, fighter)
	if !hasQuest(avail, "sequel") {
		t.Error("sequel should be available once wolf-hunt is completed")
	}
	if !hasQuest(avail, "warrior-only") {
		t.Error("a Fighter should see the Fighter-only quest")
	}
	if hasQuest(avail, "wolf-hunt") {
		t.Error("a completed quest should not be available")
	}
}

func TestAcceptRejectsUnqualified(t *testing.T) {
	qs := testQuests()
	save := newSave()
	if err := quest.Accept(*qs["warrior-only"], save, fakeCtx{class: "Wizard"}); err == nil {
		t.Error("a Wizard accepting a Fighter-only quest should error")
	}
	if quest.IsActive(save, "warrior-only") {
		t.Error("a rejected quest must not be added to the active list")
	}
}

func TestAbandon(t *testing.T) {
	qs := testQuests()
	save := newSave()
	_ = quest.Accept(*qs["wolf-hunt"], save, fakeCtx{})
	if err := quest.Abandon(save, "wolf-hunt"); err != nil {
		t.Fatalf("Abandon: %v", err)
	}
	if quest.IsActive(save, "wolf-hunt") {
		t.Error("quest should not be active after Abandon")
	}
	if err := quest.Abandon(save, "wolf-hunt"); err == nil {
		t.Error("abandoning a non-active quest should error")
	}
}

func TestQuestPointsDerive(t *testing.T) {
	qs := testQuests()
	save := newSave()
	save.QuestsCompleted = []string{"wolf-hunt", "two-step"} // 3 + 5
	if got := quest.QuestPoints(save, lookupFrom(qs)); got != 8 {
		t.Errorf("quest points = %d, want 8", got)
	}
}

func hasQuest(list []types.QuestData, id string) bool {
	for _, q := range list {
		if q.ID == id {
			return true
		}
	}
	return false
}

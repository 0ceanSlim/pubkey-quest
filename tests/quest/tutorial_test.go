package quest_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/types"
)

// The tutorial ("The Caretaker's Letter"): started at creation, never offered,
// driven by "do" objectives, with a casters-only stage and a skip.

func tutorialQuest() *types.QuestData {
	return &types.QuestData{
		ID: "caretakers-letter-test", Name: "The Caretaker's Letter", Category: types.QuestMain,
		StartCondition: types.QuestStartCondition{Type: quest.StartAtCreation, Target: "keeper"},
		Stages: []types.QuestStage{
			{ID: "keeper", Objectives: []types.QuestObjective{{Type: types.ObjectiveTalk, Target: "keeper"}}},
			{ID: "gear", Objectives: []types.QuestObjective{{Type: types.ObjectiveDo, Target: "claim-starting-gear"}}},
			{ID: "spell", Classes: []string{"Wizard"}, Objectives: []types.QuestObjective{{Type: types.ObjectiveDo, Target: "prepare-spell"}}},
			{ID: "bed", Objectives: []types.QuestObjective{
				{Type: types.ObjectiveDo, Target: "sleep-inn"},
				{Type: types.ObjectiveDo, Target: "save-game"},
			}},
			{ID: "back", Objectives: []types.QuestObjective{{Type: types.ObjectiveTalk, Target: "keeper"}},
				Rewards: &types.POIReward{Gold: 100}},
		},
	}
}

func tutorialRecorder(q *types.QuestData) *events.Recorder {
	r := &events.Recorder{}
	r.Subscribe(quest.Consumer(func(id string) (*types.QuestData, error) { return q, nil }, nil))
	return r
}

func TestCreationQuestIsNeverOffered(t *testing.T) {
	q := tutorialQuest()
	save := &types.SaveFile{Class: "Fighter"}
	if quest.CanStart(*q, save, fakeCtx{}) {
		t.Fatal("a creation quest must not be offered")
	}
	quest.StartForNewCharacter(*q, save)
	if !quest.IsActive(save, q.ID) || quest.StageOf(save, q.ID) != 0 {
		t.Fatalf("started at stage 0: active=%v stage=%d", quest.IsActive(save, q.ID), quest.StageOf(save, q.ID))
	}
	quest.StartForNewCharacter(*q, save) // a second start does nothing
	if len(save.QuestsActive) != 1 {
		t.Fatalf("started twice: %d active", len(save.QuestsActive))
	}
}

func TestTutorialStepsAndCasterStage(t *testing.T) {
	for _, tc := range []struct {
		class     string
		wantSpell bool
	}{{"Wizard", true}, {"Fighter", false}} {
		q := tutorialQuest()
		save := &types.SaveFile{Class: tc.class}
		r := tutorialRecorder(q)
		quest.StartForNewCharacter(*q, save)

		r.Record(save, events.NPCTalked, "keeper", 1)
		r.Record(save, events.Did, "claim-starting-gear", 1)
		want := 3 // the bed stage
		if tc.wantSpell {
			want = 2
		}
		if got := quest.StageOf(save, q.ID); got != want {
			t.Fatalf("%s: after the gear, stage %d, want %d", tc.class, got, want)
		}
		if tc.wantSpell {
			r.Record(save, events.Did, "save-game", 1) // not this stage's: ignored
			if quest.StageOf(save, q.ID) != 2 {
				t.Fatalf("a save mustn't move the spell stage")
			}
			r.Record(save, events.Did, "prepare-spell", 1)
		}
		r.Record(save, events.Did, "sleep-inn", 1)
		if quest.StageOf(save, q.ID) != 3 {
			t.Fatalf("%s: both bed objectives are needed", tc.class)
		}
		r.Record(save, events.Did, "save-game", 1)
		r.Record(save, events.NPCTalked, "keeper", 1)
		if quest.IsCompleted(save, q.ID) {
			t.Fatalf("%s: talking to the keeper mustn't finish it: it's turned in", tc.class)
		}
		r.Record(save, events.TurnedIn, q.ID, 1)
		if !quest.IsCompleted(save, q.ID) {
			t.Fatalf("%s: the tutorial should be complete (stage %d)", tc.class, quest.StageOf(save, q.ID))
		}
	}
}

func TestSkipFinishesWithTheWelcomeOnly(t *testing.T) {
	q := tutorialQuest()
	other := &types.QuestData{ID: "other"}
	save := &types.SaveFile{Class: "Fighter"}
	quest.StartForNewCharacter(*q, save)
	save.QuestsActive = append(save.QuestsActive, types.QuestProgress{QuestID: other.ID})

	quest.FinishNow(save, q, nil)
	if !quest.IsCompleted(save, q.ID) || quest.IsActive(save, q.ID) {
		t.Fatal("skipping should complete the tutorial")
	}
	if !quest.IsActive(save, other.ID) {
		t.Fatal("skipping the tutorial mustn't touch another quest")
	}
}

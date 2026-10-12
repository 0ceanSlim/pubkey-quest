package content_test

import (
	"math/rand"
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/poi"
	"pubkey-quest/cmd/server/game/quest"
	"pubkey-quest/types"
)

// siteCtx is a requirement context over a live save: quest stages are real,
// every skill is high enough to pass its check.
type siteCtx struct{ save *types.SaveFile }

func (c siteCtx) SkillValue(string) int           { return 30 }
func (c siteCtx) StatValue(string) int            { return 10 }
func (c siteCtx) Level() int                      { return 1 }
func (c siteCtx) QuestPoints() int                { return 0 }
func (c siteCtx) HasItem(string) bool             { return false }
func (c siteCtx) Class() string                   { return "Fighter" }
func (c siteCtx) Race() string                    { return "Human" }
func (c siteCtx) Alignment() string               { return "neutral" }
func (c siteCtx) IsQuestCompleted(id string) bool { return quest.IsCompleted(c.save, id) }
func (c siteCtx) QuestStage(id string) int        { return quest.StageOf(c.save, id) }

// TestRatsSewersPlaythrough walks the Rats of Goldenhaven sewers the way a
// player would — from the breach after the cellar is cleared to the Rat King —
// feeding what happens to the quest engine, and checks it finishes stages 1–3
// and only Bob's conversation is left.
func TestRatsSewersPlaythrough(t *testing.T) {
	setup(t)
	qd, err := db.GetQuestByID("rats-of-goldenhaven")
	if err != nil {
		t.Fatal(err)
	}
	site, ok := qd.Sites["goldenhaven-sewers"]
	if !ok {
		t.Fatal("rats quest has no sewers site")
	}
	for i := 1; i <= 3; i++ {
		if qd.Stages[i].Site != "goldenhaven-sewers" {
			t.Fatalf("stage %d doesn't open the sewers", i)
		}
	}

	save := &types.SaveFile{
		Inventory:    map[string]interface{}{"general_slots": []interface{}{}},
		Stats:        map[string]interface{}{},
		HP:           50,
		MaxHP:        50,
		QuestsActive: []types.QuestProgress{{QuestID: qd.ID, Stage: 1, ObjectiveCounts: make([]int, len(qd.Stages[1].Objectives))}},
	}
	var rec events.Recorder
	rec.Subscribe(quest.Consumer(db.GetQuestByID, nil))

	var looted []string
	deps := poi.Deps{
		Ctx:     siteCtx{save},
		Rng:     rand.New(rand.NewSource(1)),
		AddItem: func(_ *types.SaveFile, id string, _ int) { looted = append(looted, id) },
	}
	// The route a player takes: always the forward choice.
	prefer := []string{"Examine the breach", "Climb through into the tunnels", "Follow the scratching", "Head for the drainage chamber"}

	node := site.StartNode
	for steps := 0; steps < 50; steps++ {
		res := poi.Resolve(site.Nodes[node], node, save, deps)
		if res.CheckSkill != "" && res.CheckSuccess {
			rec.Record(save, events.SkillCheckPassed, res.CheckSkill, 1)
		}
		if res.Waypoint != "" {
			rec.Record(save, events.LocationDiscovered, res.Waypoint, 1)
		}
		if res.Combat != "" {
			rec.Record(save, events.MonsterKilled, res.Combat, max(res.CombatCount, 1)) // the fight is won
		}
		if res.Terminal {
			break
		}
		next := res.Next
		if len(res.Choices) > 0 {
			next = ""
			for _, want := range prefer {
				for _, ch := range res.Choices {
					if ch.Label == want && next == "" {
						next = ch.Next
					}
				}
			}
			if next == "" {
				t.Fatalf("at %q (stage %d) no forward choice among %+v", node, quest.StageOf(save, qd.ID), res.Choices)
			}
		}
		node = next
	}

	if got := quest.StageOf(save, qd.ID); got != 4 {
		t.Fatalf("after the sewers the quest is at stage %d, want 4 (talk to Bob)", got)
	}
	if len(looted) != 1 || looted[0] != "rat-tail-trophy" {
		t.Errorf("looted %v, want the Rat King's knot", looted)
	}

	rec.Record(save, events.NPCTalked, "tavern-owner-bob", 1)
	if !quest.IsCompleted(save, qd.ID) {
		t.Error("talking to Bob should finish the quest")
	}
}

package content_test

import (
	"math/rand"
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/events"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/inventory"
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
func (c siteCtx) HasItem(id string) bool          { return gameutil.CountItem(c.save, id) > 0 }
func (c siteCtx) Class() string                   { return "Fighter" }
func (c siteCtx) Race() string                    { return "Human" }
func (c siteCtx) Alignment() string               { return "neutral" }
func (c siteCtx) IsQuestCompleted(id string) bool { return quest.IsCompleted(c.save, id) }
func (c siteCtx) QuestStage(id string) int        { return quest.StageOf(c.save, id) }

// player is a save plus the quest engine listening to it, for walking real
// content the way the game does.
type player struct {
	save *types.SaveFile
	rec  events.Recorder
}

func newPlayer() *player {
	slots := []interface{}{}
	for i := 0; i < 6; i++ {
		slots = append(slots, map[string]interface{}{"item": "", "quantity": 0})
	}
	p := &player{save: &types.SaveFile{
		Inventory: map[string]interface{}{"general_slots": slots, "gear_slots": map[string]interface{}{}},
		Stats:     map[string]interface{}{},
		HP:        50,
		MaxHP:     50,
	}}
	p.rec.Subscribe(quest.Consumer(db.GetQuestByID, nil))
	return p
}

// walk plays a node graph from start, always taking the first choice in
// prefer that's on offer, feeding checks, waypoints, kills and loot to the
// quest engine as the server does. It fails if a choice node offers none.
func (p *player) walk(t *testing.T, nodes map[string]types.POIStep, start string, prefer ...string) {
	t.Helper()
	deps := poi.Deps{
		Ctx: siteCtx{p.save},
		Rng: rand.New(rand.NewSource(1)),
		AddItem: func(s *types.SaveFile, id string, qty int) {
			if n, _ := inventory.AddItemToInventory(s, id, qty); n > 0 {
				p.rec.Record(s, events.ItemAcquired, id, 1)
			}
		},
	}
	node := start
	for steps := 0; steps < 60; steps++ {
		res := poi.Resolve(nodes[node], node, p.save, deps)
		if res.CheckSkill != "" && res.CheckSuccess {
			p.rec.Record(p.save, events.SkillCheckPassed, res.CheckSkill, 1)
		}
		if res.Waypoint != "" {
			p.rec.Record(p.save, events.LocationDiscovered, res.Waypoint, 1)
		}
		if res.Combat != "" {
			p.rec.Record(p.save, events.MonsterKilled, res.Combat, max(res.CombatCount, 1)) // the fight is won
		}
		if res.Terminal {
			return
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
				t.Fatalf("at %q no wanted choice among %+v", node, res.Choices)
			}
		}
		node = next
	}
	t.Fatalf("walk from %q never ended", start)
}

// TestRatsSewersPlaythrough walks the Rats of Goldenhaven sewers the way a
// player would — from the breach after the cellar is cleared to the Rat King —
// and checks it finishes stages 1–3 and only Bob's conversation is left.
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

	p := newPlayer()
	p.save.QuestsActive = []types.QuestProgress{{QuestID: qd.ID, Stage: 1, ObjectiveCounts: make([]int, len(qd.Stages[1].Objectives))}}
	p.walk(t, site.Nodes, site.StartNode,
		"Examine the breach", "Climb through into the tunnels", "Follow the scratching", "Head for the drainage chamber")

	if got := quest.StageOf(p.save, qd.ID); got != 4 {
		t.Fatalf("after the sewers the quest is at stage %d, want 4 (talk to Bob)", got)
	}
	if gameutil.CountItem(p.save, "rat-tail-trophy") != 1 {
		t.Error("didn't come out with the Rat King's knot")
	}
	p.rec.Record(p.save, events.NPCTalked, "tavern-owner-bob", 1)
	if quest.IsCompleted(p.save, qd.ID) {
		t.Error("just talking to Bob mustn't finish the quest: it's handed in")
	}
	if !quest.ReadyToTurnIn(p.save, qd, "tavern-owner-bob", false) {
		t.Fatal("Bob should take the turn-in")
	}
	p.rec.Record(p.save, events.TurnedIn, qd.ID, 1)
	if !quest.IsCompleted(p.save, qd.ID) {
		t.Error("turning in to Bob should finish the quest")
	}
}

// TestRisingShadowPlaythrough plays the main quest's opening: accepting from
// Seraphina, the fountain site, Thistle, the ghost lilies on the Forgotten
// Battlefield (a quest branch in a POI that's visitable on its own), the
// hand-ins, and the ritual.
func TestRisingShadowPlaythrough(t *testing.T) {
	setup(t)
	qd, err := db.GetQuestByID("the-rising-shadow")
	if err != nil {
		t.Fatal(err)
	}
	p := newPlayer()
	if err := quest.Accept(*qd, p.save, siteCtx{p.save}); err != nil {
		t.Fatal(err)
	}
	stageIs := func(want int, when string) {
		t.Helper()
		if got := quest.StageOf(p.save, qd.ID); got != want {
			t.Fatalf("%s: quest at stage %d, want %d", when, got, want)
		}
	}

	p.rec.Record(p.save, events.NPCTalked, "oracle-seraphina", 1) // accepting is a conversation
	stageIs(1, "after accepting")

	fountain := qd.Sites["golden-fountain"]
	p.walk(t, fountain.Nodes, fountain.StartNode, "Study the seven mouths")
	stageIs(2, "after the fountain")

	p.rec.Record(p.save, events.NPCTalked, "thistle-goldworthy", 1)
	stageIs(3, "after meeting Thistle")

	bf, err := db.GetPOIByID("forgotten-battlefield")
	if err != nil {
		t.Fatal(err)
	}
	p.walk(t, bf.Nodes, bf.StartNode, "Look for the ghost lilies Thistle described.")
	stageIs(4, "after the battlefield")

	p.rec.Record(p.save, events.NPCTalked, "thistle-goldworthy", 1)
	stageIs(5, "after trading the lilies")
	if gameutil.CountItem(p.save, "ghost-lily") != 0 || gameutil.CountItem(p.save, "incense-of-sight") != 1 {
		t.Fatalf("trade: lilies=%d incense=%d", gameutil.CountItem(p.save, "ghost-lily"), gameutil.CountItem(p.save, "incense-of-sight"))
	}

	ritual := qd.Sites["scrying-bowl"]
	p.walk(t, ritual.Nodes, ritual.StartNode, "Breathe in and look into the bowl")
	stageIs(6, "after the ritual")
	if gameutil.CountItem(p.save, "incense-of-sight") != 0 || gameutil.CountItem(p.save, "vial-of-pure-water") != 0 {
		t.Error("the ritual should use up the incense and the water")
	}

	p.rec.Record(p.save, events.TurnedIn, qd.ID, 1)
	if !quest.IsCompleted(p.save, qd.ID) {
		t.Error("turning in to Seraphina after the vision should finish the quest")
	}
}

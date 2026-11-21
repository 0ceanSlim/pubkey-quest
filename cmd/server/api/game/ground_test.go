package game

import (
	"testing"

	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/session"
	"pubkey-quest/cmd/server/world"
)

// groundOf is what a player sees on the ground where they stand.
func groundOf(sess *session.GameSession) map[string]int {
	out := map[string]int{}
	for _, d := range groundHere(sess) {
		out[d.Item] += d.Quantity
	}
	return out
}

func sharedGround(t *testing.T) {
	t.Helper()
	old := world.World
	world.World = world.NewSharedGround()
	t.Cleanup(func() { world.World = old })
}

// A drop is the dropper's alone until they save; then anyone at that spot can
// take it (docs/draft/shared-ground.md).
func TestDropsGoPublicOnSave(t *testing.T) {
	sharedGround(t)
	a := wharfSession(t, "torch")
	a.Ground = world.NewGroundStore()
	b := wharfSession(t)
	b.Npub, b.SaveID, b.Ground = "npub_other", "s2", world.NewGroundStore()

	if r := fishAction(t, a, "drop_item", map[string]any{"item_id": "torch", "from_slot": 0.0, "from_slot_type": "general", "quantity": 2.0}); !r.Success {
		t.Fatalf("drop: %+v", r)
	}
	if groundOf(a)["torch"] != 2 {
		t.Fatalf("the dropper sees %v", groundOf(a))
	}
	if len(groundOf(b)) != 0 {
		t.Fatalf("someone else sees an unsaved drop: %v", groundOf(b))
	}
	// Not saved: it's still the dropper's to pick back up.
	if r := fishAction(t, a, "pickup_item", map[string]any{"item_id": "torch", "quantity": 1.0}); !r.Success || gameutil.CountItem(&a.SaveData, "torch") != 4 {
		t.Fatalf("picking my own drop back up: %+v", r)
	}

	publishGround(a, a.Ground.Mark()) // the save is kept
	if groundOf(b)["torch"] != 1 {
		t.Fatalf("after the save, the other player sees %v", groundOf(b))
	}
	pile := groundHere(b)[0]
	if pile.Yours {
		t.Fatal("a saved drop is still marked yours")
	}
	if r := fishAction(t, b, "pickup_item", map[string]any{"pile": pile.ID}); !r.Success || gameutil.CountItem(&b.SaveData, "torch") != 1 {
		t.Fatalf("the other player's pickup: %+v", r)
	}
	if len(groundOf(a)) != 0 || len(groundOf(b)) != 0 {
		t.Fatalf("still on the ground: %v / %v", groundOf(a), groundOf(b))
	}
	if r := fishAction(t, a, "pickup_item", map[string]any{"pile": pile.ID}); r.Success {
		t.Fatal("picked up a pile that's gone")
	}
}

// A quest item that reaches the ground stays its owner's, saved or not.
func TestQuestItemsStayYours(t *testing.T) {
	sharedGround(t)
	a := wharfSession(t)
	a.Ground = world.NewGroundStore()
	b := wharfSession(t)
	b.Npub, b.Ground = "npub_other", world.NewGroundStore()

	a.Ground.Add(groundSpot(a), "caretakers-letter", 1) // a full pack overflowed
	publishGround(a, a.Ground.Mark())
	if len(groundOf(b)) != 0 {
		t.Fatalf("someone else sees my quest item: %v", groundOf(b))
	}
	if groundOf(a)["caretakers-letter"] != 1 {
		t.Fatalf("I lost sight of my quest item: %v", groundOf(a))
	}
}

// On a road you see what was left near you, whichever way you're heading:
// a traveller's District is the road end they set out from.
func TestRoadDropsAreWhereTheyFell(t *testing.T) {
	sharedGround(t)
	a := wharfSession(t, "torch")
	a.Ground = world.NewGroundStore()
	a.SaveData.Location, a.SaveData.District, a.SaveData.TravelProgress = "greywind-plains", "kingdom-north", 0.3
	fishAction(t, a, "drop_item", map[string]any{"item_id": "torch", "from_slot": 0.0, "from_slot_type": "general", "quantity": 1.0})
	publishGround(a, a.Ground.Mark())

	b := wharfSession(t)
	b.Npub, b.Ground = "npub_other", world.NewGroundStore()
	b.SaveData.Location, b.SaveData.District, b.SaveData.TravelProgress = "greywind-plains", "frosthold-south", 0.3
	if len(groundOf(b)) != 0 {
		t.Fatalf("0.3 from the far end (0.7 along) sees %v", groundOf(b))
	}
	b.SaveData.TravelProgress = 0.68 // 0.32 from the first end
	if groundOf(b)["torch"] != 1 {
		t.Fatalf("heading the other way, right by it, sees %v", groundOf(b))
	}
}

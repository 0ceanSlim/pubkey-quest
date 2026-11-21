package world

import (
	"path/filepath"
	"testing"
	"time"

	"pubkey-quest/types"
)

func TestGroundStoreAddTakeList(t *testing.T) {
	g := NewGroundStore()
	here := Spot{Key: "kingdom|center||"}

	// Drops of the same item stack into one pile.
	g.Add(here, "longsword", 1)
	g.Add(here, "rations", 3)
	g.Add(here, "rations", 2)

	list := g.List(here, "me")
	if len(list) != 2 {
		t.Fatalf("expected 2 piles, got %d: %+v", len(list), list)
	}
	var rations int
	for _, d := range list {
		if d.Item == "rations" {
			rations = d.Quantity
		}
		if !d.Yours {
			t.Errorf("a session's own drop should be marked yours: %+v", d)
		}
	}
	if rations != 5 {
		t.Errorf("rations should stack to 5, got %d", rations)
	}

	// Take part of a stack — the remainder stays.
	if _, got := g.Take(here, "me", "", "rations", 2); got != 2 {
		t.Errorf("take 2 rations: got %d, want 2", got)
	}
	// Take more than remains — clamps to what's there and clears the pile.
	if _, got := g.Take(here, "me", "", "rations", 99); got != 3 {
		t.Errorf("take remaining rations: got %d, want 3", got)
	}
	for _, d := range g.List(here, "me") {
		if d.Item == "rations" {
			t.Errorf("rations pile should be removed once emptied, got %+v", d)
		}
	}

	// Taking something not on the ground yields 0 (pickup will reject).
	if _, got := g.Take(here, "me", "", "not-here", 1); got != 0 {
		t.Errorf("take absent item: got %d, want 0", got)
	}

	// Different spots are isolated.
	if len(g.List(Spot{Key: "other|place||"}, "me")) != 0 {
		t.Error("a different spot should have no ground items")
	}
}

// On a road you see what lies within RoadReach of you.
func TestGroundAlongARoad(t *testing.T) {
	g := newStore("g")
	g.Add(Spot{Key: "road|darkwood-forest", Road: true, Pos: 0.40}, "torch", 1)
	near := Spot{Key: "road|darkwood-forest", Road: true, Pos: 0.43}
	far := Spot{Key: "road|darkwood-forest", Road: true, Pos: 0.50}
	if len(g.List(near, "")) != 1 || len(g.List(far, "")) != 0 {
		t.Fatalf("near sees %d, far sees %d; want 1 and 0", len(g.List(near, "")), len(g.List(far, "")))
	}
	if _, n := g.Take(far, "", "", "torch", 1); n != 0 {
		t.Fatal("took a torch lying out of reach")
	}
}

// A save publishes the drops it covers: the save in hand (Mark) covers what
// was dropped before it, and a later drop doesn't join those piles.
func TestMarkDrainPublish(t *testing.T) {
	mine, world := NewGroundStore(), newStore("g")
	here := Spot{Key: "kingdom|center||"}
	mine.Add(here, "rations", 2)
	mark := mine.Mark() // a relay save prepared here
	mine.Add(here, "rations", 3)
	world.Publish(mine.Drain(mark))

	if l := world.List(here, "you"); len(l) != 1 || l[0].Quantity != 2 || l[0].Yours {
		t.Fatalf("published %+v, want 2 rations, not marked yours", l)
	}
	if l := mine.List(here, "me"); len(l) != 1 || l[0].Quantity != 3 {
		t.Fatalf("kept %+v, want the 3 dropped after the save", l)
	}
}

// A quest item's pile shows only to its owner.
func TestOwnedPiles(t *testing.T) {
	world := newStore("g")
	here := Spot{Key: "kingdom|center||"}
	world.Publish([]Pile{{GroundDrop: GroundDrop{Item: "letter", Quantity: 1}, Key: here.Key, Owner: "me"}})
	if len(world.List(here, "you")) != 0 {
		t.Fatal("someone else sees my quest item")
	}
	if _, n := world.Take(here, "you", "", "letter", 1); n != 0 {
		t.Fatal("someone else took my quest item")
	}
	if p, n := world.Take(here, "me", "", "letter", 1); n != 1 || p.Owner != "me" {
		t.Fatalf("I couldn't take my own quest item back: %d", n)
	}
}

// Piles despawn, and a crowded spot loses its oldest.
func TestDespawnAndCrowding(t *testing.T) {
	world := newStore("g")
	here := Spot{Key: "kingdom|center||"}
	world.Publish([]Pile{{GroundDrop: GroundDrop{Item: "old", Quantity: 1}, Key: here.Key}})
	world.piles[0].At = time.Now().Add(-DespawnAfter - time.Minute)
	if len(world.List(here, "")) != 0 {
		t.Fatal("an expired pile is still there")
	}

	for i := 0; i < MaxPilesPerSpot+5; i++ {
		world.Restore([]Pile{{GroundDrop: GroundDrop{Item: "x", Quantity: 1, State: map[string]interface{}{"t": i}},
			Key: here.Key, At: time.Now().Add(time.Duration(i) * time.Second)}})
	}
	l := world.List(here, "")
	if len(l) != MaxPilesPerSpot {
		t.Fatalf("%d piles at a spot, want %d", len(l), MaxPilesPerSpot)
	}
	if l[0].State["t"] != 5 {
		t.Fatalf("oldest kept pile is %v, want the 6th (5 oldest crowded out)", l[0].State["t"])
	}
}

// The world's ground is kept across a restart.
func TestWorldGroundPersists(t *testing.T) {
	old, oldFile := World, GroundFile
	t.Cleanup(func() { World, GroundFile = old, oldFile })
	World, GroundFile = newStore("g"), filepath.Join(t.TempDir(), "ground.json")

	here := Spot{Key: "kingdom|center||"}
	World.Publish([]Pile{{GroundDrop: GroundDrop{Item: "rope", Quantity: 1}, Key: here.Key}})
	SaveWorldGround()
	World = newStore("g")
	LoadWorldGround()
	if l := World.List(here, ""); len(l) != 1 || l[0].Item != "rope" {
		t.Fatalf("after a restart: %+v", l)
	}
}

func TestGroundKey(t *testing.T) {
	s := &types.SaveFile{Location: "kingdom", District: "market", Building: "inn", Room: "common"}
	if got := GroundKey(s); got != "kingdom|market|inn|common" {
		t.Errorf("GroundKey = %q", got)
	}
	if GroundKey(nil) != "" {
		t.Error("nil save should yield an empty key")
	}
}

func TestGroundStoreNilSafe(t *testing.T) {
	var g *GroundStore
	g.Add(Spot{Key: "k"}, "x", 1) // must not panic
	if _, n := g.Take(Spot{Key: "k"}, "", "", "x", 1); n != 0 {
		t.Error("nil store Take should return 0")
	}
	if g.List(Spot{Key: "k"}, "") != nil {
		t.Error("nil store List should return nil")
	}
}

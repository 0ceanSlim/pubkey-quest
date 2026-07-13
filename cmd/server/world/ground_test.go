package world

import (
	"testing"

	"pubkey-quest/types"
)

func TestGroundStoreAddTakeList(t *testing.T) {
	g := NewGroundStore()
	const key = "kingdom|center||"

	// Drops of the same item stack into one pile.
	g.Add(key, "longsword", 1)
	g.Add(key, "rations", 3)
	g.Add(key, "rations", 2)

	list := g.List(key)
	if len(list) != 2 {
		t.Fatalf("expected 2 piles, got %d: %+v", len(list), list)
	}
	var rations int
	for _, d := range list {
		if d.Item == "rations" {
			rations = d.Quantity
		}
	}
	if rations != 5 {
		t.Errorf("rations should stack to 5, got %d", rations)
	}

	// Take part of a stack — the remainder stays.
	if got := g.Take(key, "rations", 2); got != 2 {
		t.Errorf("take 2 rations: got %d, want 2", got)
	}
	// Take more than remains — clamps to what's there and clears the pile.
	if got := g.Take(key, "rations", 99); got != 3 {
		t.Errorf("take remaining rations: got %d, want 3", got)
	}
	// The empty pile is gone.
	for _, d := range g.List(key) {
		if d.Item == "rations" {
			t.Errorf("rations pile should be removed once emptied, got %+v", d)
		}
	}

	// Taking something not on the ground yields 0 (pickup will reject).
	if got := g.Take(key, "not-here", 1); got != 0 {
		t.Errorf("take absent item: got %d, want 0", got)
	}

	// Different location keys are isolated.
	if len(g.List("other|place||")) != 0 {
		t.Error("a different location should have no ground items")
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
	g.Add("k", "x", 1)             // must not panic
	if g.Take("k", "x", 1) != 0 {  // nil store takes nothing
		t.Error("nil store Take should return 0")
	}
	if g.List("k") != nil {
		t.Error("nil store List should return nil")
	}
}

package inventory_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/gameutil"
)

// Keepers accept goods rather than coin, so CountItem has to read quantities
// (PlayerHasItem only answers yes/no) and ConsumeItem has to spend across
// stacks — and, crucially, take nothing at all when the player is short.

func TestCountItemSumsAcrossStacksAndBackpack(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "iron-ore", 2)
	general(s)[1] = slot(1, "iron-ore", 1)
	backpack(s)[0] = slot(0, "iron-ore", 4)

	if got := gameutil.CountItem(s, "iron-ore"); got != 7 {
		t.Errorf("CountItem = %d, want 7 across general slots and backpack", got)
	}
	if got := gameutil.CountItem(s, "whetstone"); got != 0 {
		t.Errorf("CountItem for an item not held = %d, want 0", got)
	}
}

func TestConsumeItemSpendsAcrossStacks(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "iron-ore", 2)
	backpack(s)[0] = slot(0, "iron-ore", 4)

	if !gameutil.ConsumeItem(s, "iron-ore", 3) {
		t.Fatal("ConsumeItem returned false with 6 ore held")
	}
	if got := gameutil.CountItem(s, "iron-ore"); got != 3 {
		t.Errorf("%d ore left, want 3 after spending 3 of 6", got)
	}
}

// The important one: a turn-in the player can't afford must not eat the part
// they do have.
func TestConsumeItemTakesNothingWhenShort(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "iron-ore", 2)

	if gameutil.ConsumeItem(s, "iron-ore", 3) {
		t.Fatal("ConsumeItem returned true with only 2 of 3 held")
	}
	if got := gameutil.CountItem(s, "iron-ore"); got != 2 {
		t.Errorf("%d ore left, want the original 2 untouched", got)
	}
}

func TestConsumeItemExactStackClearsSlot(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "whetstone", 1)

	if !gameutil.ConsumeItem(s, "whetstone", 1) {
		t.Fatal("ConsumeItem returned false for an exact stack")
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want an empty slot", got)
	}
}

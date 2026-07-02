package inventory_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/types"
)

// pouchSlot builds a general slot holding a component-pouch, optionally
// pre-filled with `fill` spell-component entries in its contents.
func pouchSlot(index, fill int) map[string]interface{} {
	contents := make([]interface{}, 4) // component-pouch has container_slots: 4
	for i := range contents {
		if i < fill {
			contents[i] = map[string]interface{}{"item": "arcane-powder", "quantity": float64(1)}
		} else {
			contents[i] = map[string]interface{}{"item": nil, "quantity": float64(0)}
		}
	}
	return map[string]interface{}{"slot": float64(index), "item": "component-pouch", "quantity": float64(1), "contents": contents}
}

func pouchContents(s *types.SaveFile, generalIndex int) []interface{} {
	m := general(s)[generalIndex].(map[string]interface{})
	c, _ := m["contents"].([]interface{})
	return c
}

func addToContainer(s *types.SaveFile, itemID string, fromSlot, containerSlot int) error {
	_, err := inventory.HandleAddToContainerAction(s, p(map[string]interface{}{
		"item_id": itemID, "from_slot": float64(fromSlot), "from_slot_type": "general",
		"container_slot": float64(containerSlot), "container_slot_type": "general",
	}))
	return err
}

func TestAddSpellComponentToPouchThenRemove(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "arcane-powder", 1)
	general(s)[1] = pouchSlot(1, 0)

	if err := addToContainer(s, "arcane-powder", 0, 1); err != nil {
		t.Fatalf("add to container: %v", err)
	}
	if got := slotItem(general(s), 0); got != "" {
		t.Errorf("general[0] = %q, want empty after moving into pouch", got)
	}
	if got, _ := pouchContents(s, 1)[0].(map[string]interface{})["item"].(string); got != "arcane-powder" {
		t.Errorf("pouch contents[0] = %q, want arcane-powder", got)
	}

	// Remove it back to inventory.
	_, err := inventory.HandleRemoveFromContainerAction(s, p(map[string]interface{}{
		"container_slot": float64(1), "container_slot_type": "general", "from_container_slot": float64(0),
	}))
	if err != nil {
		t.Fatalf("remove from container: %v", err)
	}
	// Lands in the first free general slot (slot 0 is now empty).
	if got := slotItem(general(s), 0); got != "arcane-powder" {
		t.Errorf("general[0] = %q, want arcane-powder back after remove", got)
	}
}

// Regression: removing an item from a container returns it to the backpack with
// the slot index written as a Go int. A subsequent add must still find it — a
// float64-only slot read made the just-removed item invisible ("item not found
// in source inventory"), which broke all container interaction after one remove.
func TestAddAfterRemoveFindsIntSlot(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	// General slots full → a removed item falls back to the backpack, where the
	// remove handler stores the slot as an int.
	general(s)[0] = pouchSlot(0, 0)
	general(s)[1] = slot(1, "rations", 1)
	general(s)[2] = slot(2, "healing", 1)
	general(s)[3] = slot(3, "dagger", 1)
	backpack(s)[0] = slot(0, "arcane-powder", 1)

	add := func(from int, fromType string) error {
		_, err := inventory.HandleAddToContainerAction(s, p(map[string]interface{}{
			"item_id": "arcane-powder", "from_slot": float64(from), "from_slot_type": fromType,
			"container_slot": float64(0), "container_slot_type": "general",
		}))
		return err
	}

	if err := add(0, "inventory"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := inventory.HandleRemoveFromContainerAction(s, p(map[string]interface{}{
		"container_slot": float64(0), "container_slot_type": "general", "from_container_slot": float64(0),
	})); err != nil {
		t.Fatalf("remove: %v", err)
	}

	from := -1
	for i := range backpack(s) {
		if slotItem(backpack(s), i) == "arcane-powder" {
			from = i
			break
		}
	}
	if from < 0 {
		t.Fatal("arcane-powder not returned to the backpack after remove")
	}
	if err := add(from, "inventory"); err != nil {
		t.Fatalf("second add after remove failed (int-slot regression): %v", err)
	}
}

// Removing from a container must not lose the item when there's nowhere to put
// it: general slots full and no backpack equipped (e.g. you took your bag off,
// it's sitting in a general slot, and you're emptying it). The item stays inside
// and the action reports no room — it must not vanish into a phantom bag slot.
func TestRemoveFromContainerNoRoomKeepsItem(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	gearSlots(s)["bag"] = emptyGear() // backpack is NOT equipped
	general(s)[0] = map[string]interface{}{
		"item": "backpack", "quantity": float64(1), "slot": float64(0),
		"contents": []interface{}{
			map[string]interface{}{"item": "torch", "quantity": float64(1), "slot": float64(0)},
		},
	}
	general(s)[1] = slot(1, "rations", 1)
	general(s)[2] = slot(2, "healing", 1)
	general(s)[3] = slot(3, "dagger", 1)

	if _, err := inventory.HandleRemoveFromContainerAction(s, p(map[string]interface{}{
		"container_slot": float64(0), "container_slot_type": "general", "from_container_slot": float64(0),
	})); err == nil {
		t.Fatal("expected a 'no room' error with a full general row and no bag equipped")
	}
	bp := general(s)[0].(map[string]interface{})["contents"].([]interface{})
	if got, _ := bp[0].(map[string]interface{})["item"].(string); got != "torch" {
		t.Errorf("container item = %q, want torch still inside (not lost)", got)
	}
}

// A container cannot be nested inside another container. This is enforced today.
func TestPouchRejectsNestedContainer(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = pouchSlot(0, 0)
	general(s)[1] = pouchSlot(1, 0)

	if err := addToContainer(s, "component-pouch", 0, 1); err == nil {
		t.Error("expected nesting a container inside a container to be rejected")
	}
}

// A full container rejects further additions.
func TestPouchFullRejectsAdd(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "arcane-powder", 1)
	general(s)[1] = pouchSlot(1, 4) // all 4 slots used

	if err := addToContainer(s, "arcane-powder", 0, 1); err == nil {
		t.Error("expected a full container to reject the add")
	}
}

// The component pouch declares allowed_types ["Spell Component"]; a longsword
// is not a spell component and must be rejected.
//
// RED until Phase 2: the handler reads allowed_types as a string, but the JSON
// stores an array, so the assertion fails and the gate silently degrades to
// "any" (everything allowed). Fixing the reader (Phase 2) makes this pass.
func TestPouchRejectsDisallowedType(t *testing.T) {
	setup(t)
	s := newSave(4, 20)
	general(s)[0] = slot(0, "longsword", 1)
	general(s)[1] = pouchSlot(1, 0)

	if err := addToContainer(s, "longsword", 0, 1); err == nil {
		t.Error("expected a non-component item to be rejected by the component pouch")
	}
}

package combat

import (
	"testing"

	"pubkey-quest/types"
)

// quiverSave builds a save with a quiver equipped in the ammo slot holding
// `qty` arrows.
func quiverSave(qty int) *types.SaveFile {
	return &types.SaveFile{Inventory: map[string]interface{}{
		"gear_slots": map[string]interface{}{
			"ammo": map[string]interface{}{
				"item": "quiver", "quantity": float64(1),
				"contents": []interface{}{
					map[string]interface{}{"item": "arrows", "quantity": float64(qty), "slot": float64(0)},
				},
			},
		},
	}}
}

func ammoContents(save *types.SaveFile) map[string]interface{} {
	return save.Inventory["gear_slots"].(map[string]interface{})["ammo"].(map[string]interface{})["contents"].([]interface{})[0].(map[string]interface{})
}

// A ranged attack draws a round from the quiver's contents, leaving the quiver
// itself equipped (the pre-fix bug consumed the quiver's own quantity).
func TestConsumeAmmoFromQuiver(t *testing.T) {
	save := quiverSave(3)
	cs := &types.CombatSession{}
	weapon := map[string]interface{}{"ammunition": "arrows"}

	if err := consumeAmmo(save, cs, weapon); err != nil {
		t.Fatalf("consume: %v", err)
	}
	ammoSlot := save.Inventory["gear_slots"].(map[string]interface{})["ammo"].(map[string]interface{})
	if ammoSlot["item"] != "quiver" {
		t.Errorf("quiver should stay equipped, got item=%v", ammoSlot["item"])
	}
	round := ammoContents(save)
	if round["item"] != "arrows" || slotQty(round, "quantity") != 2 {
		t.Errorf("quiver arrows = %v x%d, want arrows x2", round["item"], slotQty(round, "quantity"))
	}
	if cs.AmmoUsedThisCombat != 1 {
		t.Errorf("AmmoUsedThisCombat = %d, want 1", cs.AmmoUsedThisCombat)
	}
}

// Firing the last round empties the contents slot but keeps the quiver.
func TestConsumeLastAmmoFromQuiver(t *testing.T) {
	save := quiverSave(1)
	if err := consumeAmmo(save, &types.CombatSession{}, map[string]interface{}{"ammunition": "arrows"}); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if round := ammoContents(save); round["item"] != nil {
		t.Errorf("emptied round should be nil, got %v", round["item"])
	}
}

// An empty quiver blocks the shot.
func TestConsumeAmmoEmptyQuiverErrors(t *testing.T) {
	save := quiverSave(0)
	// A qty-0 round is not usable ammo.
	if err := consumeAmmo(save, &types.CombatSession{}, map[string]interface{}{"ammunition": "arrows"}); err == nil {
		t.Error("expected an error firing with an empty quiver")
	}
}

// Loose ammo↔weapon matching across the inconsistent weapon "ammunition" labels.
func TestAmmoMatchesWeaponLoose(t *testing.T) {
	cases := []struct {
		ammo, label string
		want        bool
	}{
		{"arrows", "arrows", true},
		{"crossbow-bolts", "bolts", true},
		{"sling-bullet", "sling bullets", true},
		{"blowgun-needle", "blowgun-needle", true},
		{"arrows", "bolts", false},
	}
	for _, c := range cases {
		if got := ammoMatchesWeapon(c.ammo, map[string]interface{}{"ammunition": c.label}); got != c.want {
			t.Errorf("ammoMatchesWeapon(%q, label=%q) = %v, want %v", c.ammo, c.label, got, c.want)
		}
	}
}

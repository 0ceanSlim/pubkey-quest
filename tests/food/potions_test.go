package food_test

import (
	"testing"

	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/inventory"
)

// Mana draughts restore a flat amount, never past the max.
func TestManaDraughts(t *testing.T) {
	setup(t)
	s := player(2)
	s.Mana, s.MaxMana = 1, 6
	inventory.ApplyItemEffects(s, "mana-draught")
	if s.Mana != 3 {
		t.Errorf("mana draught: %d, want 3", s.Mana)
	}
	inventory.ApplyItemEffects(s, "superior-mana-draught")
	if s.Mana != 6 {
		t.Errorf("superior draught past the max: %d, want 6", s.Mana)
	}
}

// Settling tea ends food poisoning (the tea isn't food, so it goes down).
func TestSettlingTeaCuresFoodPoisoning(t *testing.T) {
	setup(t)
	s := player(2)
	if err := effects.ApplyEffect(s, inventory.FoodPoisoning); err != nil {
		t.Fatal(err)
	}
	if msg := inventory.WhyCantEat(s, "settling-tea"); msg != "" {
		t.Fatalf("refused: %s", msg)
	}
	inventory.ApplyItemEffects(s, "settling-tea")
	if effects.HasActiveEffect(s, inventory.FoodPoisoning) {
		t.Error("still food poisoned")
	}
}

// Owl's Eye gives the darkvision stat the night-combat rule reads.
func TestOwlsEyeDarkvision(t *testing.T) {
	setup(t)
	s := player(2)
	if effects.ModifierTotal(s, "darkvision") != 0 {
		t.Fatal("darkvision before drinking")
	}
	inventory.ApplyItemEffects(s, "owls-eye")
	if effects.ModifierTotal(s, "darkvision") <= 0 {
		t.Error("no darkvision after Owl's Eye")
	}
}

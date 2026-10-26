package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/tests/helpers"
)

// The innkeepers are found by their rooms, and the menu reads cooking.json.
func TestInnKitchenConfig(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]bool{"kingdom-inn-keeper": true, "millhaven-barkeep": true, "general-merchant": false} {
		n, err := serverdb.GetNPCByID(id)
		if err != nil || n == nil {
			t.Fatalf("%s: %v", id, err)
		}
		if isInnkeeper(n) != want {
			t.Errorf("%s innkeeper = %v", id, !want)
		}
	}
	cfg := cooking()
	if cfg.Inn.CookCost != 2 || len(cfg.Inn.Meals) == 0 || cfg.Inn.Meals[0].Effect != "meal-hearty" {
		t.Fatalf("cooking config: %+v", cfg)
	}
	if _, err := effects.LoadEffectData(cfg.Inn.Meals[0].Effect); err != nil {
		t.Errorf("meal effect: %v", err)
	}
}

package building_test

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/building"
	"pubkey-quest/tests/helpers"
)

// Every starting city must expose exactly one building typed "vault" — that is
// the keeper's door, and character creation resolves a new player's home keeper
// through it (cmd/server/api/character/create.go). If a city's vault building
// loses its type, new characters there start with no vault access at all, so
// this guards the content rather than the lookup.
func TestEveryStartingCityHasATypedVaultBuilding(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatalf("init db: %v", err)
	}
	db := serverdb.GetDB()

	// The six starting cities, each with the keeper's building we expect.
	want := map[string]string{
		"kingdom":     "vault_of_crowns",
		"verdant":     "glade_of_safekeeping",
		"ironpeak":    "stone_vault",
		"millhaven":   "burrow_lock",
		"marshlight":  "war_hoard",
		"goldenhaven": "ember_vault",
	}

	for city, expected := range want {
		got, err := building.FindBuildingIDByType(db, city, "vault")
		if err != nil {
			t.Errorf("%s: no vault building found: %v", city, err)
			continue
		}
		if got != expected {
			t.Errorf("%s: vault building = %q, want %q", city, got, expected)
		}
		// The type must also resolve through the normal path, since that's what
		// building_type encounters and room logic read.
		if resolved, err := building.GetBuildingType(db, city, got); err != nil {
			t.Errorf("%s: GetBuildingType(%s): %v", city, got, err)
		} else if resolved != "vault" {
			t.Errorf("%s: GetBuildingType(%s) = %q, want \"vault\"", city, got, resolved)
		}
	}
}

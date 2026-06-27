package content_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
)

// TestEveryBiomeHasMonsterPool walks every environment, resolves its biome, and
// asserts the biome has at least one monster — the runtime equivalent of the
// --check-connections encounter-coverage check, and a prerequisite for biome
// travel encounters (an empty pool means nothing can spawn there).
func TestEveryBiomeHasMonsterPool(t *testing.T) {
	setup(t)

	rows, err := db.GetDB().Query(`SELECT id FROM locations WHERE location_type = 'environment'`)
	if err != nil {
		t.Fatal(err)
	}
	var envs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			envs = append(envs, id)
		}
	}
	rows.Close()
	if len(envs) == 0 {
		t.Fatal("no environments loaded")
	}

	for _, env := range envs {
		biome, err := db.GetEnvironmentType(env)
		if err != nil {
			t.Errorf("GetEnvironmentType(%s): %v", env, err)
			continue
		}
		if biome == "" {
			continue // an environment may legitimately omit a biome
		}
		mons, err := db.GetMonstersByBiome(biome)
		if err != nil {
			t.Errorf("GetMonstersByBiome(%s): %v", biome, err)
			continue
		}
		if len(mons) == 0 {
			t.Errorf("environment %s (biome %q) has an empty monster pool", env, biome)
		}
		for _, m := range mons {
			if m.ID == "" {
				t.Errorf("biome %q returned a monster with no id", biome)
			}
		}
	}
}

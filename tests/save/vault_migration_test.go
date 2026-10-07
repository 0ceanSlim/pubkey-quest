package save_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// Schema v4 replaced the per-building vault grids with one shared, infinitely
// stacking pool. Loading a pre-v4 save must pour every town's grid into that one
// pool — nothing lost, duplicates merged — and turn each grid's building into a
// keeper registration, so a player keeps the doors they had already earned.
func TestVaultMigrationMergesPerBuildingGrids(t *testing.T) {
	npubDir := filepath.Join(t.TempDir(), "npub1test")
	if err := os.MkdirAll(npubDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Two registered vaults in different towns, both holding rations, plus a
	// pouch that still has something inside it (legal before v4).
	v3 := `{
	  "d": "Hero", "schema_version": 3,
	  "vaults": [
	    {"building": "vault_of_crowns", "slots": [
	      {"slot": 0, "item": "rations", "quantity": 12},
	      {"slot": 1, "item": "longsword", "quantity": 1},
	      {"slot": 2, "item": null, "quantity": 0}
	    ]},
	    {"building": "stone_vault", "slots": [
	      {"slot": 0, "item": "rations", "quantity": 7},
	      {"slot": 1, "item": "pouch", "quantity": 1, "contents": [
	        {"slot": 0, "item": "rough-gem", "quantity": 3}
	      ]}
	    ]}
	  ]
	}`
	path := filepath.Join(npubDir, "save_1.json")
	if err := os.WriteFile(path, []byte(v3), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := session.LoadSaveFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if loaded.SchemaVersion != types.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", loaded.SchemaVersion, types.CurrentSchemaVersion)
	}
	if len(loaded.LegacyVaults) != 0 {
		t.Errorf("LegacyVaults should be cleared after migration, got %+v", loaded.LegacyVaults)
	}

	want := map[string]int{
		"rations":   19, // 12 + 7, merged across two towns
		"longsword": 1,
		"pouch":     1,
		"rough-gem": 3, // poured out of the pouch, which must now be empty
	}
	got := map[string]int{}
	for _, e := range loaded.Vault {
		if _, dup := got[e.ItemID]; dup {
			t.Errorf("item %q appears in more than one entry; entries must merge", e.ItemID)
		}
		got[e.ItemID] = e.Quantity
	}
	for id, qty := range want {
		if got[id] != qty {
			t.Errorf("vault holds %d %s, want %d", got[id], id, qty)
		}
	}
	for id := range got {
		if _, expected := want[id]; !expected {
			t.Errorf("unexpected item %q in migrated vault", id)
		}
	}

	if len(loaded.VaultKeepers) != 2 {
		t.Fatalf("VaultKeepers = %v, want both buildings", loaded.VaultKeepers)
	}
	for _, b := range []string{"vault_of_crowns", "stone_vault"} {
		found := false
		for _, k := range loaded.VaultKeepers {
			if k == b {
				found = true
			}
		}
		if !found {
			t.Errorf("keeper %q lost in migration (got %v)", b, loaded.VaultKeepers)
		}
	}

	// The migrated save must round-trip without resurrecting the legacy field.
	out := filepath.Join(npubDir, "save_2.json")
	if err := session.WriteSaveFile(out, loaded); err != nil {
		t.Fatalf("write: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"vaults"`) {
		t.Error("written save still contains the legacy \"vaults\" field")
	}
	reloaded, err := session.LoadSaveFile(out)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(reloaded.Vault) != len(loaded.Vault) || len(reloaded.VaultKeepers) != 2 {
		t.Errorf("round-trip changed the vault: %+v", reloaded)
	}
}

// A save with no vaults at all migrates to an empty shared vault rather than
// failing or inventing one.
func TestVaultMigrationHandlesNoVaults(t *testing.T) {
	npubDir := filepath.Join(t.TempDir(), "npub1test")
	if err := os.MkdirAll(npubDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(npubDir, "save_1.json")
	if err := os.WriteFile(path, []byte(`{"d":"Hero"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := session.LoadSaveFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Vault) != 0 || len(loaded.VaultKeepers) != 0 {
		t.Errorf("want an empty shared vault, got %+v / %+v", loaded.Vault, loaded.VaultKeepers)
	}
}

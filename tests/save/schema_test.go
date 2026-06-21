package save_test

import (
	"os"
	"path/filepath"
	"testing"

	"pubkey-quest/cmd/server/session"
	"pubkey-quest/types"
)

// A v1 save (no schema_version) loads as schema-v2 with zero-valued new fields,
// and a write→read round-trip preserves the new fields.
func TestSchemaMigrationAndRoundTrip(t *testing.T) {
	npubDir := filepath.Join(t.TempDir(), "npub1test")
	if err := os.MkdirAll(npubDir, 0o755); err != nil {
		t.Fatal(err)
	}

	v1 := `{"d":"Hero","class":"Wizard","experience":300,"hp":8,"max_hp":8,"stats":{"constitution":14}}`
	path := filepath.Join(npubDir, "save_1.json")
	if err := os.WriteFile(path, []byte(v1), 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := session.LoadSaveFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.SchemaVersion != types.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", loaded.SchemaVersion, types.CurrentSchemaVersion)
	}
	if loaded.Room != "" || len(loaded.QuestsCompleted) != 0 || len(loaded.Rentals) != 0 {
		t.Errorf("v1 save should load with zero-valued schema-v2 fields, got %+v", loaded)
	}

	// Populate the new fields and confirm a write→read round-trip preserves them.
	loaded.Room = "common-room"
	loaded.QuestsCompleted = []string{"intro-quest"}
	loaded.QuestsActive = []types.QuestProgress{{QuestID: "side-1", Stage: 2, ObjectiveCounts: []int{1, 3}}}
	loaded.Rentals = []types.Rental{{Building: "inn", ExpiresDay: 5, ExpiresMin: 720}}

	out := filepath.Join(npubDir, "save_2.json")
	if err := session.WriteSaveFile(out, loaded); err != nil {
		t.Fatalf("write: %v", err)
	}
	reloaded, err := session.LoadSaveFile(out)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Room != "common-room" ||
		len(reloaded.QuestsCompleted) != 1 ||
		len(reloaded.QuestsActive) != 1 || reloaded.QuestsActive[0].Stage != 2 ||
		len(reloaded.Rentals) != 1 {
		t.Errorf("round-trip lost schema-v2 fields: %+v", reloaded)
	}
}

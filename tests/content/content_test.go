package content_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/tests/helpers"
)

// setup points the working dir at the project root and opens the migrated
// game.db, so these tests exercise the real loaded content (run --migrate first).
func setup(t *testing.T) {
	t.Helper()
	helpers.SetupTestEnvironment(t)
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("init db (did you run `go run ./cmd/codex --migrate`?): %v", err)
	}
	t.Cleanup(func() { db.Close() })
}

func TestQuestsLoaded(t *testing.T) {
	setup(t)

	quests, err := db.GetAllQuests()
	if err != nil {
		t.Fatal(err)
	}
	if len(quests) == 0 {
		t.Fatal("no quests loaded from drafts")
	}
	for _, q := range quests {
		if q.ID == "" {
			t.Errorf("quest with empty id: %+v", q)
		}
		if len(q.Stages) == 0 {
			t.Errorf("quest %s has no stages", q.ID)
		}
	}

	// The full definition round-trips through the by-id read path.
	first := quests[0]
	got, err := db.GetQuestByID(first.ID)
	if err != nil {
		t.Fatalf("GetQuestByID(%s): %v", first.ID, err)
	}
	if got.Name != first.Name || len(got.Stages) != len(first.Stages) {
		t.Errorf("round-trip mismatch for %s", first.ID)
	}
}

func TestQuestsByCategory(t *testing.T) {
	setup(t)

	dailies, err := db.GetQuestsByCategory("daily")
	if err != nil {
		t.Fatal(err)
	}
	if len(dailies) == 0 {
		t.Fatal("expected at least one daily quest")
	}
	for _, q := range dailies {
		if q.Category != "daily" {
			t.Errorf("quest %s category = %q, want daily", q.ID, q.Category)
		}
	}
}

func TestPOIsLoaded(t *testing.T) {
	setup(t)

	// Derive a real environment rather than hard-coding one.
	var env string
	if err := db.GetDB().QueryRow(`SELECT parent_environment FROM pois WHERE parent_environment != '' LIMIT 1`).Scan(&env); err != nil {
		t.Fatalf("no POI has a parent_environment: %v", err)
	}

	pois, err := db.GetPOIsByEnvironment(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(pois) == 0 {
		t.Fatalf("no POIs loaded for environment %q", env)
	}
	for _, p := range pois {
		if p.StartNode == "" {
			t.Errorf("POI %s has no start_node", p.ID)
		}
		if len(p.Nodes) == 0 {
			t.Errorf("POI %s has no nodes", p.ID)
		}
		if _, ok := p.Nodes[p.StartNode]; !ok {
			t.Errorf("POI %s start_node %q is not among its nodes", p.ID, p.StartNode)
		}
	}

	if _, err := db.GetPOIByID(pois[0].ID); err != nil {
		t.Errorf("GetPOIByID(%s): %v", pois[0].ID, err)
	}
}

func TestEncountersLoaded(t *testing.T) {
	setup(t)

	var trigger string
	if err := db.GetDB().QueryRow(`SELECT trigger FROM encounters WHERE trigger != '' LIMIT 1`).Scan(&trigger); err != nil {
		t.Fatalf("no encounter has a trigger: %v", err)
	}

	encs, err := db.GetEncountersByTrigger(trigger)
	if err != nil {
		t.Fatal(err)
	}
	if len(encs) == 0 {
		t.Fatalf("no encounters loaded for trigger %q", trigger)
	}
	for _, e := range encs {
		if e.StartNode == "" {
			t.Errorf("encounter %s has no start_node", e.ID)
		}
		if len(e.Nodes) == 0 {
			t.Errorf("encounter %s has no nodes", e.ID)
		}
	}

	if _, err := db.GetEncounterByID(encs[0].ID); err != nil {
		t.Errorf("GetEncounterByID(%s): %v", encs[0].ID, err)
	}
}

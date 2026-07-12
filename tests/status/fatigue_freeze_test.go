// Package status_test covers the survival loop (fatigue accrual + starvation) at
// the gametime/effects/status seam, against the migrated www/game.db (effect
// templates load from SQLite). Skips if the DB is missing — run
// `go run ./cmd/codex --migrate` first.
package status_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

func setup(t *testing.T) {
	t.Helper()
	helpers.SetupTestEnvironment(t)
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
}

func baseStats() map[string]interface{} {
	return map[string]interface{}{
		"strength": 10, "dexterity": 10, "constitution": 10,
		"intelligence": 10, "wisdom": 10, "charisma": 10,
	}
}

// Waiting / resting must freeze fatigue while time still passes; normal time must
// still accrue it.
func TestFatigueFreezeOnWaitRest(t *testing.T) {
	setup(t)

	// Frozen path (accrueFatigue = false): a long span passes, fatigue stays put.
	frozen := &types.SaveFile{HP: 20, MaxHP: 20, Hunger: 2, Fatigue: 0, Stats: baseStats()}
	if err := status.EnsureFatigueAccumulation(frozen); err != nil {
		t.Fatalf("seed fatigue-accumulation: %v", err)
	}
	gametime.AdvanceTime(frozen, 600, false) // 10 hours of waiting/resting
	if frozen.Fatigue != 0 {
		t.Errorf("fatigue should be frozen while waiting/resting, got %d after 600 min", frozen.Fatigue)
	}

	// Normal path (accrueFatigue = true): the same span builds fatigue.
	normal := &types.SaveFile{HP: 20, MaxHP: 20, Hunger: 2, Fatigue: 0, Stats: baseStats()}
	if err := status.EnsureFatigueAccumulation(normal); err != nil {
		t.Fatalf("seed fatigue-accumulation: %v", err)
	}
	gametime.AdvanceTime(normal, 600, true)
	if normal.Fatigue == 0 {
		t.Errorf("fatigue should accrue during normal time passing, still 0 after 600 min")
	}
}

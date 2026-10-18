package spellbook

import (
	"os"
	"sync"
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// setup opens the migrated database once (SetupTestEnvironment moves to the
// repo root; come back so other tests' relative paths still work).
var dbOnce sync.Once

func setup(t *testing.T) {
	t.Helper()
	dbOnce.Do(func() {
		wd, _ := os.Getwd()
		defer os.Chdir(wd)
		helpers.SetupTestEnvironment(t)
		if err := serverdb.InitDatabase(); err != nil {
			t.Fatalf("init database: %v", err)
		}
		load()
	})
}

func xpFor(t *testing.T, level int) int {
	t.Helper()
	for _, e := range load().advanced {
		if e.Level == level {
			return e.ExperiencePoints
		}
	}
	t.Fatalf("no advancement row for level %d", level)
	return 0
}

func slotLen(save *types.SaveFile, k string) int {
	l, _ := save.SpellSlots[k].([]interface{})
	return len(l)
}

// A level-3 druid knows the druid level-2 spells and has slots to prepare them.
func TestDruidLevelThree(t *testing.T) {
	setup(t)
	druid := &types.SaveFile{Class: "Druid", KnownSpells: []string{"druidcraft", "cure-wounds"}}
	if Knows(druid, "moonbeam") {
		t.Fatal("a level-1 druid shouldn't know moonbeam yet")
	}
	if !Knows(druid, "entangle") {
		t.Error("a druid knows every druid level-1 spell")
	}
	druid.Experience = xpFor(t, 3)
	for _, id := range []string{"moonbeam", "barkskin", "hold-person", "lesser-restoration", "pass-without-trace"} {
		if !Knows(druid, id) {
			t.Errorf("a level-3 druid should know %s", id)
		}
	}
	if Knows(druid, "fireball") {
		t.Error("not a druid spell")
	}

	// Slots from creation (level-1 counts) grow to the level-3 row, keeping what's prepared.
	druid.SpellSlots = map[string]interface{}{
		"level_1": []map[string]interface{}{{"slot": 0, "spell": "cure-wounds", "quantity": 1}, {"slot": 1, "spell": nil, "quantity": 0}},
	}
	if !SyncSlots(druid) {
		t.Fatal("slots should grow")
	}
	want := SlotCounts("Druid", 3)
	for k, n := range want {
		if got := slotLen(druid, k); got != n {
			t.Errorf("%s: %d slots, want %d", k, got, n)
		}
	}
	first := druid.SpellSlots["level_1"].([]interface{})[0].(map[string]interface{})
	if first["spell"] != "cure-wounds" {
		t.Errorf("a prepared spell must survive the sync: %v", first)
	}
	if SyncSlots(druid) {
		t.Error("a second sync should change nothing")
	}
}

// Known casters (a bard) don't get the whole list: picks are their own system.
func TestBardKeepsItsList(t *testing.T) {
	setup(t)
	bard := &types.SaveFile{Class: "Bard", KnownSpells: []string{"vicious-mockery"}, Experience: xpFor(t, 3)}
	if got := Known(bard); len(got) != 1 {
		t.Errorf("a bard's known spells are its own list: %v", got)
	}
}

// Leveling up through GrantXP grows the slots straight away.
func TestLevelUpGrowsSlots(t *testing.T) {
	setup(t)
	s := &types.SaveFile{Class: "Druid", SpellSlots: map[string]interface{}{}}
	character.GrantXP(s, xpFor(t, 3), load().advanced)
	if slotLen(s, "level_2") != SlotCounts("Druid", 3)["level_2"] {
		t.Errorf("level-2 slots after leveling: %d", slotLen(s, "level_2"))
	}
}

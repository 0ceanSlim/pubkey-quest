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

// Known casters learn a spell for each step up spells_known since level 1.
func TestBardPicks(t *testing.T) {
	setup(t)
	bard := &types.SaveFile{Class: "Bard", KnownSpells: []string{"vicious-mockery", "healing-word"}}
	if Picks(bard) != 0 {
		t.Fatalf("a level-1 bard has no picks yet, got %d", Picks(bard))
	}
	bard.Experience = xpFor(t, 3)
	sk := load().known["bard"]
	want := sk[2] - sk[0]
	if got := Picks(bard); got != want || want <= 0 {
		t.Fatalf("level-3 bard picks: got %d, want %d", got, want)
	}
	opts := PickOptions(bard)
	hasL2 := false
	for _, id := range opts {
		if id == "healing-word" {
			t.Error("a known spell isn't an option")
		}
		if load().levelOf[id] == 2 {
			hasL2 = true
		}
		if load().levelOf[id] > MaxSpellLevel("Bard", 3) {
			t.Errorf("%s is above the bard's slot level", id)
		}
	}
	if !hasL2 {
		t.Error("a level-3 bard should be offered level-2 spells")
	}
	if err := Learn(bard, "fireball"); err == nil {
		t.Error("fireball isn't a bard spell")
	}
	if err := Learn(bard, opts[0]); err != nil {
		t.Fatal(err)
	}
	if !Knows(bard, opts[0]) || Picks(bard) != want-1 {
		t.Errorf("learning spends a pick: knows=%v picks=%d", Knows(bard, opts[0]), Picks(bard))
	}
	for Picks(bard) > 0 {
		if err := Learn(bard, PickOptions(bard)[0]); err != nil {
			t.Fatal(err)
		}
	}
	if err := Learn(bard, PickOptions(bard)[0]); err == nil {
		t.Error("no picks left should refuse")
	}
}

// Wizards get two a level; whole-list casters get none (they know everything).
func TestPickCounts(t *testing.T) {
	setup(t)
	wiz := &types.SaveFile{Class: "Wizard", Experience: xpFor(t, 3)}
	if got := Picks(wiz); got != 4 {
		t.Errorf("level-3 wizard: %d picks, want 4 (2 per level)", got)
	}
	if got := Picks(&types.SaveFile{Class: "Druid", Experience: xpFor(t, 3)}); got != 0 {
		t.Errorf("druids know their whole list: %d picks", got)
	}
	if got := Picks(&types.SaveFile{Class: "Fighter", Experience: xpFor(t, 3)}); got != 0 {
		t.Errorf("fighters learn no spells: %d picks", got)
	}
}

// Paladins and rangers follow 5e: nothing at level 1; at level 2 a paladin
// knows the whole paladin list and a ranger picks two.
func TestPaladinAndRangerAre5e(t *testing.T) {
	setup(t)
	pal := &types.SaveFile{Class: "Paladin"}
	if len(Known(pal)) != 0 || MaxSpellLevel("Paladin", 1) != 0 {
		t.Errorf("a level-1 paladin has no spells or slots: %v", Known(pal))
	}
	pal.Experience = xpFor(t, 2)
	if !Knows(pal, "bless") || Picks(pal) != 0 {
		t.Errorf("a level-2 paladin knows the paladin list (bless) and has no picks: knows=%v picks=%d", Knows(pal, "bless"), Picks(pal))
	}
	ranger := &types.SaveFile{Class: "Ranger", Experience: xpFor(t, 2)}
	if MaxSpellLevel("Ranger", 1) != 0 {
		t.Error("a level-1 ranger has no spell slots")
	}
	if got := Picks(ranger); got != 2 {
		t.Errorf("a level-2 ranger picks 2 spells, got %d", got)
	}
	// An old ranger save still carrying the pre-5e starting grants loses them
	// on load, so they don't eat its picks.
	old := &types.SaveFile{Class: "Ranger", Experience: xpFor(t, 2), KnownSpells: []string{"cure-wounds", "goodberry", "hunters-mark"}}
	if !DropLegacyStartingSpells(old) || len(old.KnownSpells) != 0 || Picks(old) != 2 {
		t.Errorf("legacy grants should drop and leave 2 picks: %v, picks %d", old.KnownSpells, Picks(old))
	}
}

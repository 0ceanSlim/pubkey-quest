package spells

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/cmd/server/game/spellbook"
	"pubkey-quest/types"
)

// A level-3 wizard with a scroll of web in their spellbook.
func wizardWithScroll(t *testing.T, scroll string) *types.SaveFile {
	adv, _ := character.LoadAdvancement(serverdb.GetDB())
	xp := 0
	for _, e := range adv {
		if e.Level == 3 {
			xp = e.ExperiencePoints
		}
	}
	book := slot(0, "spellbook", 1)
	book["contents"] = []interface{}{slot(0, scroll, 1), slot(1, "", 0)}
	return &types.SaveFile{
		Class: "Wizard", Experience: xp,
		Inventory: map[string]interface{}{
			"general_slots": []interface{}{book, slot(1, "gold-piece", 20000)},
			"gear_slots":    map[string]interface{}{},
		},
	}
}

func TestMemorizeScroll(t *testing.T) {
	scribeSetup(t)
	db := serverdb.GetDB()
	save := wizardWithScroll(t, "scroll-of-web")
	if spellbook.Knows(save, "web") {
		t.Fatal("the wizard doesn't know web yet")
	}
	picks := spellbook.Picks(save)

	preview, err := Memorize(db, save, 0, "general", 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Gold != 10000 || preview.Minutes != 240 || spellbook.Knows(save, "web") {
		t.Fatalf("a dry run reports the cost (level 2: 10,000 gold, 4 hours) and spends nothing: %+v", preview)
	}

	if _, err := Memorize(db, save, 0, "general", 0, false); err != nil {
		t.Fatal(err)
	}
	if !spellbook.Knows(save, "web") {
		t.Error("web should now be known")
	}
	if gameutil.GetGoldQuantity(save) != 10000 {
		t.Errorf("gold: %d, want 10000", gameutil.GetGoldQuantity(save))
	}
	if gameutil.CountItem(save, "scroll-of-web") != 0 {
		t.Error("the scroll is used up")
	}
	if spellbook.Picks(save) != picks {
		t.Errorf("a memorized spell doesn't use a level-up pick: %d → %d", picks, spellbook.Picks(save))
	}
}

func TestMemorizeRefusals(t *testing.T) {
	scribeSetup(t)
	db := serverdb.GetDB()
	cleric := wizardWithScroll(t, "scroll-of-web")
	cleric.Class = "Cleric"
	if _, err := Memorize(db, cleric, 0, "general", 0, true); err == nil {
		t.Error("only wizards commit scrolls to memory")
	}
	if _, err := Memorize(db, wizardWithScroll(t, "scroll-of-cure-wounds"), 0, "general", 0, true); err == nil {
		t.Error("cure wounds isn't a wizard spell")
	}
	if _, err := Memorize(db, wizardWithScroll(t, "scroll-of-fireball"), 0, "general", 0, true); err == nil {
		t.Error("a level-3 wizard has no level-3 slots for fireball")
	}
	loose := wizardWithScroll(t, "scroll-of-web")
	if _, err := Memorize(db, loose, 1, "general", 0, true); err == nil {
		t.Error("the scroll has to be in a spellbook")
	}
}

// Random loot scrolls honour the filter and favour low levels.
func TestRandomScroll(t *testing.T) {
	scribeSetup(t)
	seq := 0
	rnd := func(n int) int { seq = (seq*1103515245 + 12345) & 0x7fffffff; return seq % n }
	byLevel := map[int]int{}
	for i := 0; i < 2000; i++ {
		id := spellbook.RandomScroll(&types.ScrollFilter{MaxLevel: 2, Classes: []string{"cleric"}}, rnd)
		if id == "" {
			t.Fatal("no scroll for a cleric filter")
		}
		spell := id[len("scroll-of-"):]
		if !spellbook.OnClassList("cleric", spell) {
			t.Fatalf("%s isn't a cleric spell", spell)
		}
		byLevel[spellbook.SpellLevel(spell)]++
	}
	if byLevel[3] != 0 {
		t.Error("max_level 2 must not drop level-3 scrolls")
	}
	if byLevel[1] <= byLevel[2] {
		t.Errorf("level-1 scrolls should drop far more than level-2: %v", byLevel)
	}
	if id := spellbook.RandomScroll(&types.ScrollFilter{Schools: []string{"necromancy"}}, rnd); id == "" {
		t.Error("a necromancy filter should find a scroll")
	}
}

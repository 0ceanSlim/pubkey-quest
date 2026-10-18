package spells

import (
	"os"
	"sync"
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/character"
	"pubkey-quest/cmd/server/game/gameutil"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

var scribeDBOnce sync.Once

func scribeSetup(t *testing.T) {
	t.Helper()
	scribeDBOnce.Do(func() {
		wd, _ := os.Getwd()
		defer os.Chdir(wd)
		helpers.SetupTestEnvironment(t)
		if err := serverdb.InitDatabase(); err != nil {
			t.Fatalf("init database: %v", err)
		}
	})
}

func slot(i int, item string, qty int) map[string]interface{} {
	if item == "" {
		return map[string]interface{}{"slot": float64(i), "item": nil, "quantity": float64(0)}
	}
	return map[string]interface{}{"slot": float64(i), "item": item, "quantity": float64(qty)}
}

// A level-3 druid at the Green Mortar with parchment, ink, a pen, bark and gold.
func druidAtHerbary(t *testing.T) *types.SaveFile {
	adv, _ := character.LoadAdvancement(serverdb.GetDB())
	xp := 0
	for _, e := range adv {
		if e.Level == 3 {
			xp = e.ExperiencePoints
		}
	}
	return &types.SaveFile{
		Class: "Druid", Experience: xp, Location: "verdant", Building: "green_mortar",
		Stats: map[string]interface{}{"wisdom": float64(16)},
		Inventory: map[string]interface{}{
			"general_slots": []interface{}{
				slot(0, "parchment", 2), slot(1, "ink", 1), slot(2, "ink-pen", 1),
				slot(3, "bark-shavings", 2), slot(4, "gold-piece", 30000), slot(5, "", 0),
			},
			"gear_slots": map[string]interface{}{},
		},
	}
}

func TestScribeBarkskin(t *testing.T) {
	scribeSetup(t)
	db := serverdb.GetDB()
	save := druidAtHerbary(t)
	st, err := Scribable(db, save)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Here {
		t.Fatal("the Green Mortar is a nature station")
	}
	var bark *ScribeOption
	for i := range st.Options {
		if st.Options[i].SpellID == "barkskin" {
			bark = &st.Options[i]
		}
	}
	if bark == nil || !bark.CanScribe {
		t.Fatalf("a level-3 druid with the materials can scribe barkskin: %+v", bark)
	}
	if bark.Chance != 75+5*3 {
		t.Errorf("chance: %d, want 90 (75 + 5 × WIS mod 3)", bark.Chance)
	}

	prev := scribeRoll
	t.Cleanup(func() { scribeRoll = prev })
	scribeRoll = func() int { return 1 }
	res, err := Scribe(db, save, "barkskin")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Scribed || gameutil.CountItem(save, "scroll-of-barkskin") != 1 {
		t.Fatalf("should have scribed a scroll: %+v", res)
	}
	if gameutil.CountItem(save, "parchment") != 1 || gameutil.CountItem(save, "bark-shavings") != 1 {
		t.Error("parchment and bark are spent")
	}
	if gameutil.CountItem(save, "ink") != 1 || gameutil.CountItem(save, "ink-pen") != 1 {
		t.Error("ink and pen are carried, not spent")
	}
	if gameutil.GetGoldQuantity(save) != 30000-bark.Gold {
		t.Errorf("gold: %d", gameutil.GetGoldQuantity(save))
	}

	// A failed roll still spends everything.
	scribeRoll = func() int { return 100 }
	res, err = Scribe(db, save, "barkskin")
	if err != nil {
		t.Fatal(err)
	}
	if res.Scribed || gameutil.CountItem(save, "parchment") != 0 || gameutil.CountItem(save, "scroll-of-barkskin") != 1 {
		t.Errorf("a failure spends the materials and makes nothing: %+v", res)
	}
	// Out of parchment: refused, nothing spent.
	gold := gameutil.GetGoldQuantity(save)
	if _, err := Scribe(db, save, "barkskin"); err == nil || gameutil.GetGoldQuantity(save) != gold {
		t.Error("no parchment should refuse before spending")
	}
}

func TestScribeNeedsTheRightStationAndSpell(t *testing.T) {
	scribeSetup(t)
	db := serverdb.GetDB()
	save := druidAtHerbary(t)
	if _, err := Scribe(db, save, "fireball"); err == nil {
		t.Error("a druid doesn't know fireball")
	}
	save.Location, save.Building = "kingdom", "mage_tower"
	if StationHere(db, save) {
		t.Error("a mage tower isn't a druid's station")
	}
	if _, err := Scribe(db, save, "barkskin"); err == nil {
		t.Error("scribing away from a nature station should be refused")
	}
}

package status_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/gametime"
	"pubkey-quest/cmd/server/game/survival"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// The wild's weather (docs/draft/survival-gear.md): an hourly Constitution
// save in the Frozen Wastes' cold or the Sunscorch Desert's daytime heat.

func survivalSetup(t *testing.T) {
	t.Helper()
	helpers.SetupTestEnvironment(t)
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
	survival.SetConfig(nil)
}

// rolls makes the d20 give these in turn (the last one after that).
func rolls(t *testing.T, r ...int) {
	t.Helper()
	old := survival.Roll
	i := 0
	survival.Roll = func() int {
		v := r[min(i, len(r)-1)]
		i++
		return v
	}
	t.Cleanup(func() { survival.Roll = old })
}

// traveller is out in a place at an hour, CON 10, carrying the items.
func traveller(location string, hour int, items ...string) *types.SaveFile {
	slots := []interface{}{}
	for _, id := range items {
		slots = append(slots, map[string]interface{}{"item": id, "quantity": 1})
	}
	return &types.SaveFile{
		Race: "Human", Class: "Fighter", HP: 20, MaxHP: 20, CurrentDay: 2, TimeOfDay: hour * 60,
		Stats: map[string]interface{}{"strength": 10, "dexterity": 10, "constitution": 10,
			"intelligence": 10, "wisdom": 10, "charisma": 10},
		Inventory: map[string]interface{}{"general_slots": slots, "gear_slots": map[string]interface{}{}},
		Location:  location,
	}
}

func TestColdCostsFatigueUnlessWarm(t *testing.T) {
	survivalSetup(t)
	rolls(t, 1)

	bare := traveller("frozen-wastes", 10)
	gametime.AdvanceTime(bare, 180, false)
	if bare.Fatigue != 3 {
		t.Fatalf("three cold hours, every save failed: fatigue %d, want 3", bare.Fatigue)
	}
	if !effects.HasActiveEffect(bare, "bitter-cold") {
		t.Fatal("no Bitter Cold shown in the Wastes without furs")
	}

	furs := traveller("frozen-wastes", 10, "winter-furs")
	gametime.AdvanceTime(furs, 180, false)
	if furs.Fatigue != 0 || effects.HasActiveEffect(furs, "bitter-cold") {
		t.Fatalf("winter furs: fatigue %d, cold shown %v", furs.Fatigue, effects.HasActiveEffect(furs, "bitter-cold"))
	}

	town := traveller("frosthold", 10)
	gametime.AdvanceTime(town, 180, false)
	if town.Fatigue != 0 {
		t.Fatalf("cold in town: fatigue %d", town.Fatigue)
	}
}

// A blanket gives advantage: the better of two d20s.
func TestBlanketGivesAdvantage(t *testing.T) {
	survivalSetup(t)
	rolls(t, 1, 15)
	s := traveller("frozen-wastes", 10, "blanket")
	gametime.AdvanceTime(s, 60, false)
	if s.Fatigue != 0 {
		t.Fatalf("1 and 15 with advantage against DC 10: fatigue %d", s.Fatigue)
	}
}

// Asleep in the Wastes, a tinderbox's fire keeps the cold off all night; a
// party's fire warms everyone.
func TestCampfireInTheWastes(t *testing.T) {
	survivalSetup(t)
	rolls(t, 1)
	from, to := 2*1440+22*60, 3*1440+6*60

	cold := traveller("frozen-wastes", 6)
	survival.Sleep(cold, from, to, false)
	if cold.Fatigue != 8 {
		t.Fatalf("a night in the Wastes without a fire: fatigue %d, want 8", cold.Fatigue)
	}
	own := traveller("frozen-wastes", 6, "tinderbox")
	survival.Sleep(own, from, to, false)
	party := traveller("frozen-wastes", 6)
	survival.Sleep(party, from, to, true) // someone else's fire
	if own.Fatigue != 0 || party.Fatigue != 0 {
		t.Fatalf("a night by the fire: fatigue %d (own tinderbox), %d (the party's)", own.Fatigue, party.Fatigue)
	}
}

// The desert's day: a full waterskin is drunk from by itself, a drink every two
// hours, and lasts the ten hours of heat with nothing to spare.
func TestWaterskinSeesYouThroughTheDay(t *testing.T) {
	survivalSetup(t)
	rolls(t, 1)
	s := traveller("sunscorch-desert", 7, "waterskin")
	s.TimeOfDay = 7*60 + 30
	gametime.AdvanceTime(s, 11*60, false) // 07:30 → 18:30
	if s.Fatigue != 0 {
		t.Fatalf("a day's heat with a full skin: fatigue %d", s.Fatigue)
	}
	general := s.Inventory["general_slots"].([]interface{})
	if left := condition.ChargesLeft(general[0].(map[string]interface{})); left != 0 {
		t.Fatalf("%d drinks left after the day, want 0", left)
	}
	if effects.HasActiveEffect(s, "desert-heat") {
		t.Fatal("still shown the heat at 18:30")
	}
}

// Without water every hour is a save, one DC harder each hour.
func TestHeatClimbsWithoutWater(t *testing.T) {
	survivalSetup(t)
	rolls(t, 10) // CON 10: a 10 passes DC 5 to 10, fails from DC 11
	s := traveller("sunscorch-desert", 8)
	s.TimeOfDay = 8*60 + 30
	gametime.AdvanceTime(s, 9*60, false) // marks 9:00 … 17:00
	// DCs 5,6,…,13: the hours at DC 11, 12 and 13 fail.
	if s.Fatigue != 3 {
		t.Fatalf("nine dry hours rolling 10: fatigue %d, want 3", s.Fatigue)
	}
	if s.Lim().Dry != 9 {
		t.Fatalf("dry hours %d, want 9", s.Lim().Dry)
	}
	// Night: you cool off.
	gametime.AdvanceTime(s, 6*60, false)
	if s.Lim().Dry != 0 {
		t.Fatalf("dry hours %d after dark", s.Lim().Dry)
	}
}

// A drink keeps the heat off for two hours; medium armour makes the save
// harder (the worse of two d20s).
func TestDrinkAndArmour(t *testing.T) {
	survivalSetup(t)
	rolls(t, 1)
	s := traveller("sunscorch-desert", 10)
	effects.Drink(s, 0)
	gametime.AdvanceTime(s, 120, false) // 11:00 and 12:00: watered till 12:00
	if s.Fatigue != 1 {
		t.Fatalf("two hours after a drink: fatigue %d, want 1 (the 12:00 save)", s.Fatigue)
	}

	rolls(t, 20, 1)
	armoured := traveller("sunscorch-desert", 10)
	armoured.Inventory["gear_slots"] = map[string]interface{}{"chest": map[string]interface{}{"item": "chain-shirt"}}
	gametime.AdvanceTime(armoured, 60, false)
	if armoured.Fatigue != 1 {
		t.Fatalf("20 and 1 with disadvantage in a chain shirt: fatigue %d, want 1", armoured.Fatigue)
	}
}

// Resistance keeps a weather off (Rimeheart, the huge sapphire, resists cold);
// Create Water's fresh water is a drink; Unbroken halves the fatigue.
func TestResistanceWaterAndUnbroken(t *testing.T) {
	survivalSetup(t)
	rolls(t, 1)

	warded := traveller("frozen-wastes", 10)
	warded.ActiveEffects = []types.ActiveEffect{{EffectID: "jewel-sapphire-huge"}}
	gametime.AdvanceTime(warded, 120, false)
	if warded.Fatigue != 0 {
		t.Fatalf("Rimeheart in the Wastes: fatigue %d", warded.Fatigue)
	}

	spring := traveller("sunscorch-desert", 10)
	if err := effects.ApplyEffect(spring, "fresh-water"); err != nil {
		t.Fatal(err)
	}
	if !effects.HasActiveEffect(spring, effects.WateredEffect) || spring.Lim().Drank != 2*1440+600 {
		t.Fatalf("Create Water isn't a drink: drank at %d", spring.Lim().Drank)
	}
	gametime.AdvanceTime(spring, 60, false)
	if spring.Fatigue != 0 {
		t.Fatalf("an hour after fresh water: fatigue %d", spring.Fatigue)
	}

	steady := traveller("frozen-wastes", 10)
	steady.ActiveEffects = []types.ActiveEffect{{EffectID: "jewel-jasper-huge", EffectIndex: 3}} // fatigue_slow
	gametime.AdvanceTime(steady, 240, false)
	if steady.Fatigue != 2 {
		t.Fatalf("four cold hours with Unbroken: fatigue %d, want 2", steady.Fatigue)
	}
}

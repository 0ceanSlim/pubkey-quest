package teleport

import (
	"os"
	"sync"
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

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
	})
	if len(Load().Cities) != 6 {
		t.Fatalf("teleportation.json not migrated: %d cities", len(Load().Cities))
	}
}

func slot(i int, item string, qty int) map[string]interface{} {
	if item == "" {
		return map[string]interface{}{"slot": float64(i), "item": nil, "quantity": float64(0)}
	}
	return map[string]interface{}{"slot": float64(i), "item": item, "quantity": float64(qty)}
}

// A Human from the Kingdom, out on the Merchant's Highway with their stone.
func traveller(items ...string) *types.SaveFile {
	slots := []interface{}{}
	for i, it := range items {
		slots = append(slots, slot(i, it, 1))
	}
	slots = append(slots, slot(len(items), "", 0))
	return &types.SaveFile{
		Race: "Human", Location: "merchants-highway", District: "kingdom-east", TravelProgress: 0.4,
		LocationsDiscovered: []string{"kingdom"},
		Inventory:           map[string]interface{}{"general_slots": slots, "gear_slots": map[string]interface{}{}},
	}
}

func TestStoneForSale(t *testing.T) {
	setup(t)
	s := traveller()
	if ok, _ := StoneForSale(s, "kingdom"); !ok {
		t.Error("your home city's stone is always for sale to you")
	}
	if ok, _ := StoneForSale(s, "verdant"); ok {
		t.Error("Verdant's stone needs Verdant's stone quest")
	}
	s.QuestsCompleted = []string{"stone-verdant"}
	if ok, _ := StoneForSale(s, "verdant"); !ok {
		t.Error("after the stone quest Verdant's keeper sells you one")
	}
	held := traveller("homeward-stone-kingdom")
	if ok, _ := StoneForSale(held, "kingdom"); ok {
		t.Error("nobody can buy a stone they already carry")
	}
	vaulted := traveller()
	vaulted.Vault = []types.VaultEntry{{ItemID: "homeward-stone-kingdom", Quantity: 1}}
	if ok, _ := StoneForSale(vaulted, "kingdom"); ok {
		t.Error("…or one in their vault")
	}
}

func TestUseStoneAndCooldown(t *testing.T) {
	setup(t)
	s := traveller("homeward-stone-kingdom")
	day, minute := s.CurrentDay, s.TimeOfDay
	res, err := UseStone(s, "homeward-stone-kingdom")
	if err != nil {
		t.Fatal(err)
	}
	if s.Location != "kingdom" || s.District != "center" || s.TravelProgress != 0 || s.Building != "" {
		t.Errorf("should stand at the Kingdom's waystone, off the road: %s/%s %.2f", s.Location, s.District, s.TravelProgress)
	}
	if res.City != "kingdom" || s.CurrentDay != day || s.TimeOfDay != minute {
		t.Error("teleporting takes no time")
	}
	if gotWait := CooldownLeft(s, Load().StoneCooldownEffect); gotWait != 10080 {
		t.Errorf("a week's cooldown, got %d minutes", gotWait)
	}
	s.Location = "merchants-highway"
	if _, err := UseStone(s, "homeward-stone-kingdom"); err == nil {
		t.Error("the stones share one charge: a second use within the week fails")
	}
	// A week later — exactly — the charge is back (the timer used to stick at 0).
	effects.TickEffects(s, 10080, false)
	if left := CooldownLeft(s, Load().StoneCooldownEffect); left != 0 {
		t.Fatalf("cooldown should be over, %d left", left)
	}
	for _, ae := range s.ActiveEffects {
		if ae.EffectID == Load().StoneCooldownEffect {
			t.Fatal("an expired cooldown must be removed, not kept as permanent")
		}
	}
	if _, err := UseStone(s, "homeward-stone-kingdom"); err != nil {
		t.Errorf("after a week the stone works again: %v", err)
	}
	if _, err := UseStone(traveller(), "homeward-stone-kingdom"); err == nil {
		t.Error("you have to carry the stone")
	}
}

func TestWaystoneHop(t *testing.T) {
	setup(t)
	s := traveller()
	if _, err := Hop(s, "millhaven"); err == nil {
		t.Error("a hop starts at a waystone")
	}
	s.Location, s.District, s.TravelProgress = "kingdom", "center", 0
	if WaystoneHere(s) != "kingdom" {
		t.Fatal("the Kingdom's town square has a waystone")
	}
	if _, err := Hop(s, "millhaven"); err == nil {
		t.Error("Millhaven's waystone is dormant until its waystone quest is done")
	}
	s.QuestsCompleted = []string{"waystone-millhaven"}
	if _, err := Hop(s, "millhaven"); err != nil {
		t.Fatal(err)
	}
	if s.Location != "millhaven" || s.District != "center" {
		t.Errorf("should be at Millhaven's waystone: %s/%s", s.Location, s.District)
	}
	s.QuestsCompleted = append(s.QuestsCompleted, "waystone-kingdom")
	if _, err := Hop(s, "kingdom"); err == nil {
		t.Error("hops have their own week-long cooldown")
	}
	s.Building = "village_inn"
	if WaystoneHere(s) != "" {
		t.Error("inside a building you aren't at the waystone")
	}
}

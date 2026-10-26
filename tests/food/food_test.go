package food_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/cmd/server/game/effects"
	"pubkey-quest/cmd/server/game/inventory"
	"pubkey-quest/cmd/server/game/status"
	"pubkey-quest/cmd/server/game/vault"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// Food (docs/draft/food-system.md, degradation §3): hours of fullness, spoiling
// from a made-time, and that state travelling with the item.

func setup(t *testing.T) {
	t.Helper()
	helpers.SetupTestEnvironment(t)
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
	condition.ResetCache()
}

func player(hunger int) *types.SaveFile {
	s := &types.SaveFile{
		HP: 20, MaxHP: 20, Hunger: hunger, CurrentDay: 3, TimeOfDay: 600,
		Stats: map[string]interface{}{"constitution": 30}, // always makes the save
		Inventory: map[string]interface{}{
			"general_slots": []interface{}{
				map[string]interface{}{"slot": 0, "item": nil, "quantity": 0},
				map[string]interface{}{"slot": 1, "item": nil, "quantity": 0},
				map[string]interface{}{"slot": 2, "item": nil, "quantity": 0},
				map[string]interface{}{"slot": 3, "item": nil, "quantity": 0},
			},
			"gear_slots": map[string]interface{}{"bag": map[string]interface{}{"item": "backpack", "quantity": 1, "contents": bagSlots()}},
		},
	}
	status.HandleHungerChange(s)
	return s
}

// bagSlots is an empty 20-slot backpack.
func bagSlots() []interface{} {
	out := make([]interface{}, 20)
	for i := range out {
		out[i] = map[string]interface{}{"slot": i, "item": nil, "quantity": 0}
	}
	return out
}

func slots(s *types.SaveFile) []interface{} {
	return s.Inventory["general_slots"].([]interface{})
}

func slot(s *types.SaveFile, i int) map[string]interface{} {
	return slots(s)[i].(map[string]interface{})
}

func bag(s *types.SaveFile) []interface{} {
	g := s.Inventory["gear_slots"].(map[string]interface{})
	return g["bag"].(map[string]interface{})["contents"].([]interface{})
}

// find returns the first slot holding itemID (belt, then backpack) and where it is.
func find(s *types.SaveFile, itemID string) (map[string]interface{}, inventory.SlotRef) {
	for _, kind := range []string{"general", "inventory"} {
		list := slots(s)
		if kind == "inventory" {
			list = bag(s)
		}
		for i, raw := range list {
			if sl, ok := raw.(map[string]interface{}); ok && sl["item"] == itemID {
				return sl, inventory.SlotRef{Type: kind, Index: i}
			}
		}
	}
	return nil, inventory.SlotRef{}
}

// all returns every slot holding itemID.
func all(s *types.SaveFile, itemID string) []map[string]interface{} {
	var out []map[string]interface{}
	for _, list := range [][]interface{}{slots(s), bag(s)} {
		for _, raw := range list {
			if sl, ok := raw.(map[string]interface{}); ok && sl["item"] == itemID {
				out = append(out, sl)
			}
		}
	}
	return out
}

// Fullness is time until starving; eating spills up through the levels and
// stops at Stuffed with a full bar, wasting the rest.
func TestFeedFillsUpThroughTheLevels(t *testing.T) {
	setup(t)
	s := player(1) // Hungry, a fresh bar
	if got := status.Fullness(s); got != 240 {
		t.Fatalf("Hungry with a full bar = %d minutes, want 240", got)
	}
	gained, wasted := status.Feed(s, 6*60) // a ration
	// 4h left + 6h = 10h: past Well Fed's 8h, so Stuffed.
	if gained != 360 || wasted != 0 || s.Hunger != 3 {
		t.Fatalf("after a ration: gained %d wasted %d, hunger %d", gained, wasted, s.Hunger)
	}
	if got := status.Fullness(s); got != 600 {
		t.Errorf("fullness %d, want 600", got)
	}
	status.Feed(s, 24*60)
	if !status.IsFull(s) || s.Hunger != 3 {
		t.Errorf("a feast should leave you Stuffed and full (hunger %d, fullness %d)", s.Hunger, status.Fullness(s))
	}

	starving := player(0)
	status.Feed(starving, 60)
	if starving.Hunger != 1 || status.Fullness(starving) != 60 {
		t.Errorf("starving + 1h: hunger %d, fullness %d", starving.Hunger, status.Fullness(starving))
	}
}

// Rations keep you fed 6 hours; you can't eat when full; a stale fish is half
// as filling; and while food-poisoned you can't eat at all.
func TestEatingRulesThroughUseItem(t *testing.T) {
	setup(t)
	s := player(1)
	if _, err := inventory.AddItemToInventory(s, "rations", 2); err != nil {
		t.Fatal(err)
	}
	before := status.Fullness(s)
	if r, err := inventory.HandleUseItemAction(s, map[string]interface{}{"item_id": "rations"}); err != nil || !r.Success {
		t.Fatalf("eat rations: %v %+v", err, r)
	}
	if got := status.Fullness(s) - before; got != 360 {
		t.Errorf("a ration gave %d minutes, want 360", got)
	}

	status.Feed(s, 24*60)
	if r, _ := inventory.HandleUseItemAction(s, map[string]interface{}{"item_id": "rations"}); r == nil || r.Success {
		t.Error("eating when full should be refused")
	}

	hungry := player(1)
	if _, err := inventory.AddItemToInventory(hungry, "raw-fish", 1); err != nil {
		t.Fatal(err)
	}
	fish, _ := find(hungry, "raw-fish")
	fish[condition.KeyMade] = condition.Now(hungry) - 400 // past half its 12 hours: stale
	before = status.Fullness(hungry)
	inventory.HandleUseItemAction(hungry, map[string]interface{}{"item_id": "raw-fish"})
	if got := status.Fullness(hungry) - before; got != 60 {
		t.Errorf("a stale raw fish gave %d minutes, want 60 (half of 2 hours)", got)
	}

	sick := player(1)
	inventory.AddItemToInventory(sick, "rations", 1)
	if err := effects.ApplyEffect(sick, inventory.FoodPoisoning); err != nil {
		t.Fatal(err)
	}
	if r, _ := inventory.HandleUseItemAction(sick, map[string]interface{}{"item_id": "rations"}); r == nil || r.Success {
		t.Error("food poisoning should stop you eating")
	}
}

// Spoiling food never stacks; each gets its own made-time; past its shelf
// life it becomes spoiled food, which does stack; goodberries vanish.
func TestSpoilingAndStacking(t *testing.T) {
	setup(t)
	s := player(2)
	if n, _ := inventory.AddItemToInventory(s, "raw-fish", 2); n != 2 {
		t.Fatalf("added %d fish", n)
	}
	fish := all(s, "raw-fish")
	if len(fish) != 2 || fish[0]["quantity"] != 1 || fish[1]["quantity"] != 1 {
		t.Fatalf("fish should go one per slot: %+v", fish)
	}
	if fish[0][condition.KeyMade] != condition.Now(s) {
		t.Errorf("a new fish is made now: %+v", fish[0])
	}

	s.TimeOfDay += 721
	msgs := condition.SpoilSlots([][]interface{}{slots(s), bag(s)}, condition.Now(s))
	if len(msgs) != 2 {
		t.Errorf("both fish should spoil: %v", msgs)
	}
	spoiled := all(s, condition.SpoiledFood)
	if len(spoiled) != 1 || spoiled[0]["quantity"] != 2 || len(all(s, "raw-fish")) != 0 {
		t.Errorf("two spoiled fish should be one stack of 2: %+v", spoiled)
	}
	if condition.HasState(spoiled[0]) {
		t.Error("spoiled food carries no state")
	}

	inventory.AddItemToInventory(s, "goodberry", 10)
	berries, _ := find(s, "goodberry")
	if berries == nil || berries["quantity"] != 10 {
		t.Fatalf("goodberries share a stack: %+v", berries)
	}
	s.TimeOfDay += 1441
	condition.SpoilSlots([][]interface{}{slots(s), bag(s)}, condition.Now(s))
	if berries["item"] != nil {
		t.Errorf("goodberries should vanish after a day: %+v", berries)
	}
}

// Banking keeps a food's state, and it comes back out with it; selling pays by
// freshness.
func TestVaultKeepsFreshnessAndSaleShares(t *testing.T) {
	setup(t)
	s := player(2)
	made := condition.Now(s) - 500
	if err := vault.DepositWithState(s, "raw-fish", 1, map[string]interface{}{condition.KeyMade: made}); err != nil {
		t.Fatal(err)
	}
	vault.Deposit(s, "rations", 3)
	vault.Deposit(s, "rations", 2)
	if len(s.Vault) != 2 || vault.Quantity(s, "rations") != 5 {
		t.Fatalf("rations merge, the fish keeps its own entry: %+v", s.Vault)
	}
	if r, err := inventory.HandleVaultWithdraw(s, "raw-fish", 1); err != nil || !r.Success {
		t.Fatalf("withdraw: %v %+v", err, r)
	}
	if got, _ := find(s, "raw-fish"); got == nil || got[condition.KeyMade] != made {
		t.Errorf("the fish came out as %+v, want made at %d (it kept aging in the vault)", got, made)
	}

	now := condition.Now(s)
	for _, tc := range []struct {
		age, share int
	}{{0, 100}, {400, 50}, {800, 0}} {
		if got := condition.SellShare("raw-fish", map[string]interface{}{condition.KeyMade: now - tc.age}, now); got != tc.share {
			t.Errorf("a fish %d minutes old sells at %d%%, want %d%%", tc.age, got, tc.share)
		}
	}
	if got := condition.SellShare("rations", nil, now); got != 100 {
		t.Errorf("rations sell at %d%%", got)
	}
}

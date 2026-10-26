package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// A waterskin's drinks: drink it down, fill it in town or at the coast, not in
// the desert.
func TestWaterskin(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	condition.ResetCache()
	skin := map[string]interface{}{"slot": 0, "item": "waterskin", "quantity": 1}
	s := &types.SaveFile{Location: "sunscorch-desert", Inventory: map[string]interface{}{
		"general_slots": []interface{}{skin}}}
	ref := map[string]any{"slot_type": "general", "slot": float64(0)}

	for i := 0; i < 5; i++ {
		if r, _ := handleDrink(s, ref); !r.Success {
			t.Fatalf("drink %d: %s", i+1, r.Message)
		}
	}
	if r, _ := handleDrink(s, ref); r.Success {
		t.Error("drank from an empty waterskin")
	}
	if r, _ := handleFillWater(s); r.Success {
		t.Error("filled it in the desert")
	}
	s.Location = "suncrest-coastlands"
	if r, _ := handleFillWater(s); !r.Success || condition.ChargesLeft(skin) != 5 {
		t.Errorf("fill at the coast: %s, %d left", r.Message, condition.ChargesLeft(skin))
	}
	s.Location = "kingdom"
	if r, _ := handleFillWater(s); r.Success {
		t.Error("filled an already full waterskin")
	}
}

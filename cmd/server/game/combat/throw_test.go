package combat

import (
	"strings"
	"testing"

	"pubkey-quest/types"
)

func netItem() map[string]interface{} {
	return map[string]interface{}{
		"id": "net", "name": "Net", "range": "2", "range_long": "3",
		"throw": map[string]interface{}{
			"condition": map[string]interface{}{"condition": "restrained", "save_each_turn": "strength"},
			"dc":        float64(10),
		},
	}
}

func TestThrowItemOutOfRange(t *testing.T) {
	ms := goblinsAt(types.Position{X: 7, Y: 3}) // 6 squares away; the net reaches 3
	cs := boardWith(ms)
	save := &types.SaveFile{}
	slot := map[string]interface{}{"item": "net", "quantity": 1}
	item := netItem()
	if _, err := throwItem(cs, save, item, item["throw"].(map[string]interface{}), "net", "Net", slot, false, ms[0].InstanceID); err == nil {
		t.Fatal("throwing past long range should fail")
	}
	if slot["quantity"] != 1 {
		t.Errorf("a refused throw must not use the item up: quantity %v", slot["quantity"])
	}
}

func TestThrowItemNetRestrains(t *testing.T) {
	// AC 0: everything but a natural 1 hits, so a few throws must land the net.
	for i := 0; i < 50; i++ {
		ms := goblinsAt(types.Position{X: 3, Y: 3})
		ms[0].ArmorClass = 0
		cs := boardWith(ms)
		save := &types.SaveFile{}
		slot := map[string]interface{}{"item": "net", "quantity": 2}
		item := netItem()
		log, err := throwItem(cs, save, item, item["throw"].(map[string]interface{}), "net", "Net", slot, false, ms[0].InstanceID)
		if err != nil {
			t.Fatal(err)
		}
		if slot["quantity"] != 1 {
			t.Fatalf("one net should be used up: quantity %v", slot["quantity"])
		}
		if !cs.Party[0].CombatState.ActionUsed {
			t.Fatal("a throw costs the action")
		}
		if !strings.Contains(strings.Join(log, "\n"), "You throw the Net") {
			t.Fatalf("missing throw line: %v", log)
		}
		for _, c := range cs.Monsters[0].Conditions {
			if c.Name == "restrained" {
				if c.SaveStat != "strength" || c.SaveDC != 10 {
					t.Errorf("restrained should end on a DC 10 Strength save, got %s/%d", c.SaveStat, c.SaveDC)
				}
				return
			}
		}
	}
	t.Fatal("the net never landed in 50 throws at AC 0")
}

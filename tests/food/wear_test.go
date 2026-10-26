package food_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/combat"
	"pubkey-quest/cmd/server/game/condition"
)

// Wear (degradation §4): category durability, tiers, AC and sale shares.

func TestDurabilityByCategory(t *testing.T) {
	setup(t)
	for id, want := range map[string]int{
		"longsword": 300, "dagger": 200, "longbow": 300, "shield": 300, "chainmail-hauberk": 400,
		"leather-vest": 250, "dart": 0, "arrows": 0, "rations": 0,
	} {
		if got := condition.MaxDurability(id); got != want {
			t.Errorf("%s: durability %d, want %d", id, got, want)
		}
	}
}

func TestWearTiers(t *testing.T) {
	setup(t)
	sword := map[string]interface{}{"item": "longsword", "quantity": 1}
	for _, tc := range []struct {
		wear int
		tier condition.Tier
	}{{0, condition.Pristine}, {75, condition.Pristine}, {76, condition.Worn}, {151, condition.Damaged}, {226, condition.Ruined}, {999, condition.Ruined}} {
		sword[condition.KeyWear] = tc.wear
		if got, _ := condition.WearTier(sword); got != tc.tier {
			t.Errorf("longsword with %d wear: %s, want %s", tc.wear, got, tc.tier)
		}
	}
	delete(sword, condition.KeyWear)
	before, after := condition.AddWear(sword, 80)
	if before != condition.Pristine || after != condition.Worn {
		t.Errorf("80 wear on a fresh longsword: %s → %s", before, after)
	}
	condition.AddWear(sword, 10000)
	if condition.WearOf(sword) != 300 {
		t.Errorf("wear caps at durability: %d", condition.WearOf(sword))
	}

	if got := condition.SellShare("longsword", map[string]interface{}{condition.KeyWear: 160}, 0); got != 50 {
		t.Errorf("a damaged longsword sells at %d%%, want 50%%", got)
	}
}

// Worn armour protects less: −1 worn, −2 damaged, half its value ruined.
func TestWornArmourAC(t *testing.T) {
	setup(t)
	stats := map[string]interface{}{"dexterity": 10}
	inv := func(wear int) map[string]interface{} {
		return map[string]interface{}{"gear_slots": map[string]interface{}{
			"chest": map[string]interface{}{"item": "chainmail-hauberk", "quantity": 1, condition.KeyWear: wear},
		}}
	}
	base := combat.CalculatePlayerAC(db.GetDB(), inv(0), stats)
	for _, tc := range []struct{ wear, loss int }{{150, 1}, {250, 2}, {350, 5}} {
		if got := base - combat.CalculatePlayerAC(db.GetDB(), inv(tc.wear), stats); got != tc.loss {
			t.Errorf("hauberk with %d wear loses %d AC, want %d (base %d)", tc.wear, got, tc.loss, base)
		}
	}
}

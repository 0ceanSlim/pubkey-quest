package game

import (
	"testing"

	serverdb "pubkey-quest/cmd/server/db"
	"pubkey-quest/cmd/server/game/condition"
	"pubkey-quest/tests/helpers"
	"pubkey-quest/types"
)

// A light shining at night in the wild gives you away; a bullseye lantern's
// beam helps you see; a hooded lantern with its shutter closed hides you. By
// day, in town, or with the light out, nothing changes.
func TestLightAtNight(t *testing.T) {
	helpers.SetupTestEnvironment(t)
	if err := serverdb.InitDatabase(); err != nil {
		t.Fatal(err)
	}
	condition.ResetCache()

	holding := func(item string, slot map[string]interface{}, location string, minute int) *types.SaveFile {
		slot["item"], slot["quantity"] = item, 1
		return &types.SaveFile{
			Location: location, TimeOfDay: minute,
			Inventory: map[string]interface{}{"gear_slots": map[string]interface{}{"offhand": slot}},
		}
	}
	lit := func() map[string]interface{} { return map[string]interface{}{"l": 1, "b": 300} }
	const night, noon = 23 * 60, 12 * 60

	for _, tc := range []struct {
		name              string
		save              *types.SaveFile
		stealth, percepts int
	}{
		{"lit torch, night, wild", holding("torch", lit(), "darkwood-forest", night), -5, 0},
		{"bullseye, night, wild", holding("lantern-bullseye", lit(), "darkwood-forest", night), -5, 2},
		{"hooded, shutter closed", holding("lantern-hooded", map[string]interface{}{"l": 1, "s": 1, "b": 300}, "darkwood-forest", night), 0, 0},
		{"unlit torch", holding("torch", map[string]interface{}{}, "darkwood-forest", night), 0, 0},
		{"lit torch by day", holding("torch", lit(), "darkwood-forest", noon), 0, 0},
		{"lit torch in town", holding("torch", lit(), "kingdom", night), 0, 0},
	} {
		s, p, notes := lightAtNight(tc.save)
		if s != tc.stealth || p != tc.percepts {
			t.Errorf("%s: stealth %d perception %d (%v), want %d %d", tc.name, s, p, notes, tc.stealth, tc.percepts)
		}
	}

	// And it reaches the skill the noticing checks read: −5 Stealth is 10 points.
	day := holding("torch", lit(), "darkwood-forest", noon)
	dark := holding("torch", lit(), "darkwood-forest", night)
	day.Stats, dark.Stats = map[string]interface{}{"dexterity": 14}, map[string]interface{}{"dexterity": 14}
	if a, b := buildTravelContext(day).SkillValue("thieving"), buildTravelContext(dark).SkillValue("thieving"); a-b != 10 {
		t.Errorf("thieving by day %d, by night with a torch %d: want 10 apart", a, b)
	}
}

// Package inventory_test exercises the inventory mutation handlers
// (cmd/server/game/inventory) against realistic loaded-save shapes.
//
// These handlers operate on state.Inventory, an untyped map[string]interface{}
// mirroring the on-disk JSON: general_slots is a []interface{} of slot maps,
// gear_slots is a map keyed by slot name, and the "bag" gear slot carries a
// contents []interface{} (the backpack). Numbers arrive as float64 after JSON
// decode, so fixtures build slots that way and action params are passed as
// float64 — the same types the live HTTP layer hands these handlers.
package inventory_test

import (
	"testing"

	"pubkey-quest/cmd/server/db"
	"pubkey-quest/types"
	"pubkey-quest/tests/helpers"
)

// setup chdirs to the project root and opens the migrated www/game.db. The
// handlers look items up in SQLite, so a DB is required (the suite skips if
// www/game.db is missing — run `go run ./cmd/codex --migrate` first).
func setup(t *testing.T) {
	t.Helper()
	helpers.SetupTestEnvironment(t)
	if err := db.InitDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}
}

// slot builds one inventory slot map in the loaded-JSON shape. An empty item
// (item == "") yields a nil-item slot.
func slot(index int, item string, qty int) map[string]interface{} {
	m := map[string]interface{}{"slot": float64(index)}
	if item == "" {
		m["item"] = nil
		m["quantity"] = float64(0)
	} else {
		m["item"] = item
		m["quantity"] = float64(qty)
	}
	return m
}

func emptyGear() map[string]interface{} {
	return map[string]interface{}{"item": nil, "quantity": float64(0)}
}

// newSave returns a SaveFile with `general` empty general slots and a backpack
// of `backpack` empty slots, plus the standard empty gear slots.
func newSave(general, backpack int) *types.SaveFile {
	gen := make([]interface{}, general)
	for i := range gen {
		gen[i] = slot(i, "", 0)
	}
	bp := make([]interface{}, backpack)
	for i := range bp {
		bp[i] = slot(i, "", 0)
	}
	return &types.SaveFile{
		HP: 5, MaxHP: 20, Mana: 0, MaxMana: 10, Hunger: 1, Fatigue: 0,
		Class: "Fighter",
		Stats: map[string]interface{}{
			"strength": float64(16), "dexterity": float64(14), "constitution": float64(14),
			"intelligence": float64(10), "wisdom": float64(12), "charisma": float64(10),
		},
		Inventory: map[string]interface{}{
			"general_slots": gen,
			"gear_slots": map[string]interface{}{
				"bag":        map[string]interface{}{"item": "backpack", "quantity": float64(1), "contents": bp},
				"mainhand":   emptyGear(),
				"offhand":    emptyGear(),
				"chest":      emptyGear(),
				"armor":      emptyGear(),
				"ammunition": emptyGear(),
			},
		},
	}
}

// --- state readers -------------------------------------------------------

func general(s *types.SaveFile) []interface{} {
	return s.Inventory["general_slots"].([]interface{})
}

func gearSlots(s *types.SaveFile) map[string]interface{} {
	return s.Inventory["gear_slots"].(map[string]interface{})
}

func backpack(s *types.SaveFile) []interface{} {
	bag := gearSlots(s)["bag"].(map[string]interface{})
	return bag["contents"].([]interface{})
}

// gearItem returns the item id in a named gear slot, or "".
func gearItem(s *types.SaveFile, name string) string {
	gs, ok := gearSlots(s)[name].(map[string]interface{})
	if !ok {
		return ""
	}
	id, _ := gs["item"].(string)
	return id
}

// slotItem returns the item id at slots[i], or "" if empty/out of range.
func slotItem(slots []interface{}, i int) string {
	if i < 0 || i >= len(slots) {
		return ""
	}
	m, ok := slots[i].(map[string]interface{})
	if !ok {
		return ""
	}
	id, _ := m["item"].(string)
	return id
}

// slotQty reads a slot's quantity tolerating int or float64 (handlers store
// both — a split writes int, JSON decode produces float64).
func slotQty(slots []interface{}, i int) int {
	if i < 0 || i >= len(slots) {
		return 0
	}
	m, ok := slots[i].(map[string]interface{})
	if !ok {
		return 0
	}
	switch v := m["quantity"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// makeVault appends a registered vault (loaded shape: slots as []interface{})
// to the save and returns the building id.
func makeVault(s *types.SaveFile, building string, slots int) {
	vs := make([]interface{}, slots)
	for i := range vs {
		vs[i] = slot(i, "", 0)
	}
	if s.Vaults == nil {
		s.Vaults = []map[string]interface{}{}
	}
	s.Vaults = append(s.Vaults, map[string]interface{}{"building": building, "slots": vs})
}

func vaultSlots(s *types.SaveFile, building string) []interface{} {
	for _, v := range s.Vaults {
		if b, _ := v["building"].(string); b == building {
			return v["slots"].([]interface{})
		}
	}
	return nil
}

// p is a tiny constructor for action param maps.
func p(kv map[string]interface{}) map[string]interface{} { return kv }
